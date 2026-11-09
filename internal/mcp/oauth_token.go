package mcp

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"

	"github.com/nikyjain/whatomate/internal/models"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
	"gorm.io/gorm"
)

// tokenParams reads the token request (form-encoded per RFC 6749; JSON is
// accepted too). client_id may also come from HTTP Basic auth.
func tokenParams(ctx *fasthttp.RequestCtx) map[string]string {
	out := map[string]string{}
	if strings.Contains(strings.ToLower(string(ctx.Request.Header.ContentType())), "json") {
		var m map[string]any
		if json.Unmarshal(ctx.PostBody(), &m) == nil {
			for k, v := range m {
				out[k] = scalarString(v)
			}
		}
	} else {
		for k, v := range ctx.PostArgs().All() {
			out[string(k)] = string(v)
		}
	}
	if out["client_id"] == "" {
		if user, _, ok := basicAuth(string(ctx.Request.Header.Peek("Authorization"))); ok {
			out["client_id"] = user
		}
	}
	return out
}

func basicAuth(h string) (string, string, bool) {
	scheme, rest, ok := strings.Cut(h, " ")
	if !ok || !strings.EqualFold(scheme, "Basic") {
		return "", "", false
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(rest))
	if err != nil {
		return "", "", false
	}
	u, p, ok := strings.Cut(string(raw), ":")
	return u, p, ok
}

// verifyPKCE checks BASE64URL(SHA256(verifier)) == challenge (RFC 7636 S256).
func verifyPKCE(verifier, challenge string) bool {
	if l := len(verifier); l < 43 || l > 128 || !codeChallengeRe.MatchString(verifier) {
		return false
	}
	sum := sha256.Sum256([]byte(verifier))
	got := base64.RawURLEncoding.EncodeToString(sum[:])
	return subtle.ConstantTimeCompare([]byte(got), []byte(challenge)) == 1
}

func tokenResponse(ctx *fasthttp.RequestCtx, access, refresh string) error {
	ctx.Response.Header.Set("Pragma", "no-cache")
	return writeJSON(ctx, fasthttp.StatusOK, map[string]any{
		"access_token":  access,
		"token_type":    "Bearer",
		"expires_in":    int(accessTokenTTL.Seconds()),
		"refresh_token": refresh,
	})
}

// Token serves POST /oauth/token for authorization_code and refresh_token.
func (s *Server) Token(r *fastglue.Request) error {
	if !s.Enabled() {
		return notFound(r)
	}
	ctx := r.RequestCtx
	p := tokenParams(ctx)
	switch p["grant_type"] {
	case "authorization_code":
		return s.exchangeCode(ctx, p)
	case "refresh_token":
		return s.refreshTokens(ctx, p)
	default:
		return oauthError(ctx, fasthttp.StatusBadRequest, "unsupported_grant_type", "grant_type must be authorization_code or refresh_token")
	}
}

func (s *Server) exchangeCode(ctx *fasthttp.RequestCtx, p map[string]string) error {
	ac, ok := s.consumeAuthCode(p["code"])
	if !ok {
		return oauthError(ctx, fasthttp.StatusBadRequest, "invalid_grant", "authorization code is invalid, expired or already used")
	}
	if ac.ClientID != p["client_id"] || ac.RedirectURI != p["redirect_uri"] {
		return oauthError(ctx, fasthttp.StatusBadRequest, "invalid_grant", "client_id or redirect_uri does not match the authorization request")
	}
	if !verifyPKCE(p["code_verifier"], ac.CodeChallenge) {
		return oauthError(ctx, fasthttp.StatusBadRequest, "invalid_grant", "PKCE code_verifier does not match")
	}
	client, ok := s.findClient(ac.ClientID)
	if !ok {
		return oauthError(ctx, fasthttp.StatusUnauthorized, "invalid_client", "unknown client")
	}
	if _, err := s.app.ResolveOrgUser(ac.UserID, ac.OrgID); err != nil {
		return oauthError(ctx, fasthttp.StatusBadRequest, "invalid_grant", "the user can no longer access this organization")
	}

	var access, refresh string
	err := s.app.DB.Transaction(func(tx *gorm.DB) error {
		grant := models.MCPGrant{
			UserID: ac.UserID, OrganizationID: ac.OrgID,
			ClientID: client.ClientID, ClientName: client.ClientName, Resource: ac.Resource,
		}
		if err := tx.Create(&grant).Error; err != nil {
			return err
		}
		var err error
		access, refresh, err = issueTokens(tx, grant.ID)
		return err
	})
	if err != nil {
		s.app.Log.Error("Failed to create MCP grant", "error", err)
		return oauthError(ctx, fasthttp.StatusInternalServerError, "server_error", "failed to issue tokens")
	}
	return tokenResponse(ctx, access, refresh)
}

// refreshTokens rotates a refresh token. Presenting a refresh token that was
// already used revokes the whole grant (token theft / replay detection).
func (s *Server) refreshTokens(ctx *fasthttp.RequestCtx, p map[string]string) error {
	invalid := func() error {
		return oauthError(ctx, fasthttp.StatusBadRequest, "invalid_grant", "refresh token is invalid, expired or revoked")
	}
	raw := p["refresh_token"]
	if !strings.HasPrefix(raw, refreshTokenPrefix) {
		return invalid()
	}
	var tok models.MCPToken
	if err := s.app.DB.Where("token_hash = ? AND kind = ?", hashToken(raw), models.MCPTokenRefresh).First(&tok).Error; err != nil {
		return invalid()
	}
	var grant models.MCPGrant
	if err := s.app.DB.Where("id = ?", tok.GrantID).First(&grant).Error; err != nil || grant.RevokedAt != nil {
		return invalid()
	}
	if tok.UsedAt != nil {
		s.app.Log.Warn("MCP refresh token reuse detected; revoking grant", "grant_id", grant.ID)
		_ = s.revokeGrant(grant.ID)
		return invalid()
	}
	if time.Now().After(tok.ExpiresAt) {
		return invalid()
	}
	if p["client_id"] != "" && p["client_id"] != grant.ClientID {
		return oauthError(ctx, fasthttp.StatusBadRequest, "invalid_grant", "refresh token was issued to another client")
	}
	if _, err := s.checkGrant(&grant); err != nil {
		_ = s.revokeGrant(grant.ID)
		return invalid()
	}

	var access, refresh string
	reused := false
	err := s.app.DB.Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&models.MCPToken{}).Where("id = ? AND used_at IS NULL", tok.ID).Update("used_at", time.Now())
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			reused = true
			return nil
		}
		// Old access tokens of this grant end with the rotation.
		if err := tx.Where("grant_id = ? AND kind = ?", grant.ID, models.MCPTokenAccess).Delete(&models.MCPToken{}).Error; err != nil {
			return err
		}
		var err error
		access, refresh, err = issueTokens(tx, grant.ID)
		return err
	})
	if reused {
		_ = s.revokeGrant(grant.ID)
		return invalid()
	}
	if err != nil {
		s.app.Log.Error("Failed to rotate MCP refresh token", "error", err)
		return oauthError(ctx, fasthttp.StatusInternalServerError, "server_error", "failed to issue tokens")
	}
	return tokenResponse(ctx, access, refresh)
}
