package mcp

import (
	"time"

	"github.com/google/uuid"
	"github.com/nikyjain/whatomate/internal/models"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

// grantView is one connected AI app on the settings page.
type grantView struct {
	ID               uuid.UUID  `json:"id"`
	ClientName       string     `json:"client_name"`
	OrganizationID   uuid.UUID  `json:"organization_id"`
	OrganizationName string     `json:"organization_name"`
	CreatedAt        time.Time  `json:"created_at"`
	LastUsedAt       *time.Time `json:"last_used_at,omitempty"`
}

func requestUserID(r *fastglue.Request) (uuid.UUID, bool) {
	id, ok := r.RequestCtx.UserValue("user_id").(uuid.UUID)
	return id, ok && id != uuid.Nil
}

// Info serves GET /api/mcp/info: the connector URL for the settings page.
func (s *Server) Info(r *fastglue.Request) error {
	url := ""
	if s.Enabled() {
		url = s.baseURL(r.RequestCtx) + "/mcp"
	}
	return r.SendEnvelope(map[string]any{"enabled": s.Enabled(), "connector_url": url})
}

// ListGrants serves GET /api/mcp/grants: the current user's connected apps.
func (s *Server) ListGrants(r *fastglue.Request) error {
	userID, ok := requestUserID(r)
	if !ok {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}
	var grants []models.MCPGrant
	if err := s.app.DB.Where("user_id = ? AND revoked_at IS NULL", userID).
		Order("created_at DESC").Find(&grants).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to list connected apps", nil, "")
	}
	orgIDs := make([]uuid.UUID, 0, len(grants))
	for _, g := range grants {
		orgIDs = append(orgIDs, g.OrganizationID)
	}
	names := map[uuid.UUID]string{}
	if len(orgIDs) > 0 {
		var orgs []models.Organization
		_ = s.app.DB.Select("id", "name").Where("id IN ?", orgIDs).Find(&orgs).Error
		for _, o := range orgs {
			names[o.ID] = o.Name
		}
	}
	out := make([]grantView, 0, len(grants))
	for _, g := range grants {
		out = append(out, grantView{
			ID: g.ID, ClientName: g.ClientName, OrganizationID: g.OrganizationID,
			OrganizationName: names[g.OrganizationID], CreatedAt: g.CreatedAt, LastUsedAt: g.LastUsedAt,
		})
	}
	return r.SendEnvelope(map[string]any{"grants": out})
}

// RevokeGrant serves DELETE /api/mcp/grants/{id}: revoke one of the current
// user's own grants and delete its tokens.
func (s *Server) RevokeGrant(r *fastglue.Request) error {
	userID, ok := requestUserID(r)
	if !ok {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}
	rawID, _ := r.RequestCtx.UserValue("id").(string)
	id, err := uuid.Parse(rawID)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid grant ID", nil, "")
	}
	var grant models.MCPGrant
	if err := s.app.DB.Where("id = ? AND user_id = ? AND revoked_at IS NULL", id, userID).First(&grant).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Connected app not found", nil, "")
	}
	if err := s.revokeGrant(grant.ID); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to revoke", nil, "")
	}
	return r.SendEnvelope(map[string]any{"message": "Disconnected"})
}
