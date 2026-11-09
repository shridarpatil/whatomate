package mcp

import (
	"errors"

	"github.com/google/uuid"
	"github.com/nikyjain/whatomate/internal/handlers"
	"github.com/nikyjain/whatomate/internal/models"
	"github.com/valyala/fasthttp"
)

// getData dispatches a GET and returns the envelope data, or an error result.
func (s *Server) getData(ctx *fasthttp.RequestCtx, c *Caller, path string) (any, *toolResult) {
	resp, err := s.dispatch(ctx, c, apiRequest{Method: "GET", Path: path})
	if err != nil {
		return nil, errorResult("%v", err)
	}
	if resp.Status >= 400 {
		return nil, shapeResponse(resp)
	}
	data, err := decodeData(resp)
	if err != nil {
		return nil, errorResult("unexpected response from %s", path)
	}
	return data, nil
}

func connectionInfo(c *Caller) map[string]any {
	info := map[string]any{"type": c.Kind}
	if c.OrgID != uuid.Nil {
		info["organization_id"] = c.OrgID.String()
	}
	if c.Grant != nil {
		info["app"] = c.Grant.ClientName
		info["can_switch_organization"] = true
	} else {
		info["can_switch_organization"] = false
	}
	return info
}

// toolWhoami implements whatomate_whoami via GET /api/me.
func (s *Server) toolWhoami(ctx *fasthttp.RequestCtx, c *Caller) *toolResult {
	data, errRes := s.getData(ctx, c, "/api/me")
	if errRes != nil {
		return errRes
	}
	return jsonResult(map[string]any{"connection": connectionInfo(c), "user": data})
}

// toolListOrganizations implements whatomate_list_organizations via
// GET /api/me/organizations.
func (s *Server) toolListOrganizations(ctx *fasthttp.RequestCtx, c *Caller) *toolResult {
	data, errRes := s.getData(ctx, c, "/api/me/organizations")
	if errRes != nil {
		return errRes
	}
	out := map[string]any{"connection": connectionInfo(c)}
	if m, ok := data.(map[string]any); ok {
		for k, v := range m {
			out[k] = v
		}
	} else {
		out["organizations"] = data
	}
	return jsonResult(out)
}

// toolSwitchOrganization implements whatomate_switch_organization: OAuth
// grants only, membership checked with the same rules as SwitchOrg.
func (s *Server) toolSwitchOrganization(c *Caller, args map[string]any) *toolResult {
	if c.Kind != CallerOAuth || c.Grant == nil {
		return errorResult("This connection uses an API key, and API keys are fixed to one organisation. Create an API key in the other organisation, or connect through OAuth to switch organisations.")
	}
	orgID, err := uuid.Parse(scalarString(args["organization_id"]))
	if err != nil {
		return errorResult("organization_id must be a UUID from whatomate_list_organizations")
	}
	user, err := s.app.ResolveOrgUser(c.UserID, orgID)
	if err != nil {
		switch {
		case errors.Is(err, handlers.ErrNotOrgMember):
			return errorResult("You are not a member of this organization")
		case errors.Is(err, handlers.ErrOrgNotFound):
			return errorResult("Organization not found")
		default:
			return errorResult("Cannot switch organisation: %v", err)
		}
	}
	if err := s.app.DB.Model(&models.MCPGrant{}).Where("id = ? AND revoked_at IS NULL", c.Grant.ID).
		Update("organization_id", orgID).Error; err != nil {
		return errorResult("Failed to switch organisation")
	}
	c.Grant.OrganizationID, c.OrgID, c.User = orgID, orgID, user

	var org models.Organization
	_ = s.app.DB.Select("id", "name").Where("id = ?", orgID).First(&org).Error
	return jsonResult(map[string]any{
		"organization_id":   orgID.String(),
		"organization_name": org.Name,
		"message":           "Switched. Later calls act in this organisation with your role there.",
	})
}

// toolListAreasResult implements whatomate_list_areas.
func toolListAreasResult() *toolResult {
	counts := map[string]int{}
	for _, e := range Entries() {
		counts[e.Area]++
	}
	type areaOut struct {
		Key           string `json:"key"`
		Title         string `json:"title"`
		Description   string `json:"description"`
		EndpointCount int    `json:"endpoint_count"`
	}
	out := []areaOut{}
	for _, a := range Areas() {
		out = append(out, areaOut{a.Key, a.Title, a.Description, counts[a.Key]})
	}
	return jsonResult(map[string]any{"areas": out})
}

// toolDescribeAreaResult implements whatomate_describe_area.
func toolDescribeAreaResult(key string) *toolResult {
	a, ok := areas[key]
	if !ok {
		return errorResult("unknown area %q; call whatomate_list_areas for valid keys", key)
	}
	es := EntriesForArea(key)
	if es == nil {
		es = []Entry{}
	}
	return jsonResult(map[string]any{"area": a, "endpoints": es})
}
