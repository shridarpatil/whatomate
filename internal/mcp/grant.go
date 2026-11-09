package mcp

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/nikyjain/whatomate/internal/models"
	"gorm.io/gorm"
)

var errGrantRevoked = errors.New("grant revoked")

// hashToken returns the SHA-256 hex of a token; only hashes are stored.
func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// newSecret returns prefix + 32 random bytes as hex.
func newSecret(prefix string) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(b), nil
}

// lookupAccessToken finds the live grant behind an access token.
func (s *Server) lookupAccessToken(raw string) (*models.MCPGrant, error) {
	var tok models.MCPToken
	err := s.app.DB.Where("token_hash = ? AND kind = ? AND expires_at > ?",
		hashToken(raw), models.MCPTokenAccess, time.Now()).First(&tok).Error
	if err != nil {
		return nil, err
	}
	var grant models.MCPGrant
	if err := s.app.DB.Where("id = ?", tok.GrantID).First(&grant).Error; err != nil {
		return nil, err
	}
	return &grant, nil
}

// checkGrant enforces, on every request, that the grant is not revoked, the
// user is still active and (unless super admin) still a member of the org.
// It returns the user resolved into the grant's organisation.
func (s *Server) checkGrant(g *models.MCPGrant) (*models.User, error) {
	if g.RevokedAt != nil {
		return nil, errGrantRevoked
	}
	return s.app.ResolveOrgUser(g.UserID, g.OrganizationID)
}

// touchGrant records use of a grant, at most once a minute.
func (s *Server) touchGrant(g *models.MCPGrant) {
	now := time.Now()
	if g.LastUsedAt != nil && now.Sub(*g.LastUsedAt) < time.Minute {
		return
	}
	g.LastUsedAt = &now
	if err := s.app.DB.Model(&models.MCPGrant{}).Where("id = ?", g.ID).
		Update("last_used_at", now).Error; err != nil {
		s.app.Log.Error("Failed to update MCP grant last_used_at", "error", err)
	}
}

// revokeGrant marks a grant revoked and deletes all its tokens.
func (s *Server) revokeGrant(id uuid.UUID) error {
	return s.app.DB.Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		if err := tx.Model(&models.MCPGrant{}).Where("id = ? AND revoked_at IS NULL", id).
			Update("revoked_at", now).Error; err != nil {
			return err
		}
		return tx.Where("grant_id = ?", id).Delete(&models.MCPToken{}).Error
	})
}

// issueTokens creates a fresh access + refresh token pair for a grant.
func issueTokens(tx *gorm.DB, grantID uuid.UUID) (access, refresh string, err error) {
	if access, err = newSecret(accessTokenPrefix); err != nil {
		return "", "", err
	}
	if refresh, err = newSecret(refreshTokenPrefix); err != nil {
		return "", "", err
	}
	now := time.Now()
	rows := []models.MCPToken{
		{GrantID: grantID, Kind: models.MCPTokenAccess, TokenHash: hashToken(access), ExpiresAt: now.Add(accessTokenTTL)},
		{GrantID: grantID, Kind: models.MCPTokenRefresh, TokenHash: hashToken(refresh), ExpiresAt: now.Add(refreshTokenTTL)},
	}
	if err := tx.Create(&rows).Error; err != nil {
		return "", "", err
	}
	return access, refresh, nil
}
