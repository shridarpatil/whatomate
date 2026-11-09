package models

import (
	"time"

	"github.com/google/uuid"
)

// MCPOAuthClient is an OAuth 2.1 public client registered through dynamic
// client registration (RFC 7591) by an MCP client such as claude.ai.
type MCPOAuthClient struct {
	ID           uuid.UUID   `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ClientID     string      `gorm:"size:64;uniqueIndex;not null" json:"client_id"`
	ClientName   string      `gorm:"size:255" json:"client_name"`
	RedirectURIs StringArray `gorm:"type:jsonb;not null;default:'[]'" json:"redirect_uris"`
	CreatedAt    time.Time   `gorm:"autoCreateTime" json:"created_at"`
}

func (MCPOAuthClient) TableName() string { return "mcp_oauth_clients" }

// HasRedirectURI reports whether uri exactly matches a registered redirect URI.
func (c *MCPOAuthClient) HasRedirectURI(uri string) bool {
	for _, u := range c.RedirectURIs {
		if u == uri {
			return true
		}
	}
	return false
}

// MCPGrant is one approved connection of an AI app to Whatomate: the app acts
// as UserID inside OrganizationID with the user's role in that organisation.
type MCPGrant struct {
	ID             uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	UserID         uuid.UUID  `gorm:"type:uuid;index;not null" json:"user_id"`
	OrganizationID uuid.UUID  `gorm:"type:uuid;index;not null" json:"organization_id"`
	ClientID       string     `gorm:"size:64;index;not null" json:"client_id"`
	ClientName     string     `gorm:"size:255" json:"client_name"`
	Resource       string     `gorm:"size:1024" json:"resource,omitempty"`
	CreatedAt      time.Time  `gorm:"autoCreateTime" json:"created_at"`
	LastUsedAt     *time.Time `json:"last_used_at,omitempty"`
	RevokedAt      *time.Time `gorm:"index" json:"revoked_at,omitempty"`
}

func (MCPGrant) TableName() string { return "mcp_grants" }

// MCPToken kinds.
const (
	MCPTokenAccess  = "access"
	MCPTokenRefresh = "refresh"
)

// MCPToken is an access or refresh token of a grant. Only the SHA-256 hex of
// the token is stored; the token itself is shown to the client once.
type MCPToken struct {
	ID        uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	GrantID   uuid.UUID  `gorm:"type:uuid;index;not null" json:"grant_id"`
	Kind      string     `gorm:"size:16;not null" json:"kind"`
	TokenHash string     `gorm:"size:64;uniqueIndex;not null" json:"-"`
	ExpiresAt time.Time  `gorm:"index;not null" json:"expires_at"`
	UsedAt    *time.Time `json:"used_at,omitempty"`
	CreatedAt time.Time  `gorm:"autoCreateTime" json:"created_at"`
}

func (MCPToken) TableName() string { return "mcp_tokens" }
