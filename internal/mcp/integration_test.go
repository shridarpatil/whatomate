package mcp

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/nikyjain/whatomate/internal/models"
	"github.com/nikyjain/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func TestOAuthCall_ScopedToGrantOrgAndPermissions(t *testing.T) {
	s := newDBServer(t)
	a, b := newOrgFixture(t, s), newOrgFixture(t, s)
	access, _ := connect(t, s, a.User.ID, a.Org.ID)
	bearer := "Bearer " + access

	_, resp := mcpCall(t, s, "Authorization", bearer, callToolMsg("whatomate_call", `{"method":"GET","path":"/api/contacts"}`))
	text, isErr := toolText(t, resp)
	require.False(t, isErr, text)
	assert.Contains(t, text, a.Contact.PhoneNumber)
	assert.NotContains(t, text, b.Contact.PhoneNumber)

	// The role lacks users:read: the real handler answers 403.
	_, resp = mcpCall(t, s, "Authorization", bearer, callToolMsg("whatomate_call", `{"method":"GET","path":"/api/users"}`))
	text, isErr = toolText(t, resp)
	assert.True(t, isErr)
	assert.Contains(t, text, "HTTP 403")

	_, resp = mcpCall(t, s, "Authorization", bearer, callToolMsg("whatomate_whoami", `{}`))
	text, isErr = toolText(t, resp)
	require.False(t, isErr, text)
	assert.Contains(t, text, a.User.Email)
	assert.Contains(t, text, `"type":"oauth"`)

	// Switching to an org the user does not belong to is refused...
	_, resp = mcpCall(t, s, "Authorization", bearer, callToolMsg("whatomate_switch_organization", `{"organization_id":"`+b.Org.ID.String()+`"}`))
	text, isErr = toolText(t, resp)
	assert.True(t, isErr)
	assert.Contains(t, text, "not a member")

	// ...and allowed once they are, after which calls act in that org.
	addMembership(t, s, a.User.ID, b.Org.ID, &b.Role.ID)
	_, resp = mcpCall(t, s, "Authorization", bearer, callToolMsg("whatomate_switch_organization", `{"organization_id":"`+b.Org.ID.String()+`"}`))
	_, isErr = toolText(t, resp)
	require.False(t, isErr)
	_, resp = mcpCall(t, s, "Authorization", bearer, callToolMsg("whatomate_call", `{"method":"GET","path":"/api/contacts"}`))
	text, _ = toolText(t, resp)
	assert.Contains(t, text, b.Contact.PhoneNumber)
	assert.NotContains(t, text, a.Contact.PhoneNumber)
}

func TestGrantValidity_CheckedEveryRequest(t *testing.T) {
	s := newDBServer(t)
	ping := `{"jsonrpc":"2.0","id":1,"method":"ping"}`

	// Membership removed -> 401.
	a := newOrgFixture(t, s)
	other := newOrgFixture(t, s)
	addMembership(t, s, a.User.ID, other.Org.ID, &other.Role.ID)
	access, _ := connect(t, s, a.User.ID, other.Org.ID)
	status, _ := mcpCall(t, s, "Authorization", "Bearer "+access, ping)
	require.Equal(t, 200, status)
	require.NoError(t, s.app.DB.Where("user_id = ? AND organization_id = ?", a.User.ID, other.Org.ID).Delete(&models.UserOrganization{}).Error)
	status, _ = mcpCall(t, s, "Authorization", "Bearer "+access, ping)
	assert.Equal(t, 401, status)

	// User deactivated -> 401.
	c := newOrgFixture(t, s)
	access, _ = connect(t, s, c.User.ID, c.Org.ID)
	require.NoError(t, s.app.DB.Model(&models.User{}).Where("id = ?", c.User.ID).Update("is_active", false).Error)
	status, _ = mcpCall(t, s, "Authorization", "Bearer "+access, ping)
	assert.Equal(t, 401, status)

	// Revoked from the settings page -> 401; another user cannot revoke it.
	d := newOrgFixture(t, s)
	access, _ = connect(t, s, d.User.ID, d.Org.ID)
	var grant models.MCPGrant
	require.NoError(t, s.app.DB.Where("user_id = ?", d.User.ID).First(&grant).Error)

	list := newRequest("GET", "/api/mcp/grants", nil)
	list.RequestCtx.SetUserValue("user_id", d.User.ID)
	require.NoError(t, s.ListGrants(list))
	assert.Contains(t, string(list.RequestCtx.Response.Body()), d.Org.Name)
	assert.Contains(t, string(list.RequestCtx.Response.Body()), `"client_name":"Claude"`)

	revoke := func(userID uuid.UUID) int {
		r := newRequest("DELETE", "/api/mcp/grants/"+grant.ID.String(), nil)
		r.RequestCtx.SetUserValue("user_id", userID)
		r.RequestCtx.SetUserValue("id", grant.ID.String())
		require.NoError(t, s.RevokeGrant(r))
		return r.RequestCtx.Response.StatusCode()
	}
	assert.Equal(t, 404, revoke(a.User.ID))
	assert.Equal(t, 200, revoke(d.User.ID))
	status, _ = mcpCall(t, s, "Authorization", "Bearer "+access, ping)
	assert.Equal(t, 401, status)
	var tokens int64
	s.app.DB.Model(&models.MCPToken{}).Where("grant_id = ?", grant.ID).Count(&tokens)
	assert.Zero(t, tokens)
}

func TestAPIKeyPath(t *testing.T) {
	s := newDBServer(t)
	a := newOrgFixture(t, s)
	key := "whm_" + uuid.NewString()[:8] + "0123456789abcdef01234567"
	hash, err := bcrypt.GenerateFromPassword([]byte(key), bcrypt.MinCost)
	require.NoError(t, err)
	testutil.CreateTestAPIKeyFixture(t, s.app.DB, &testutil.APIKeyFixture{
		ID: uuid.New(), OrganizationID: &a.Org.ID, UserID: a.User.ID, Name: "mcp-test",
		KeyPrefix: key[4:20], KeyHash: string(hash), IsActive: true,
	})

	status, _ := mcpCall(t, s, "X-API-Key", "whm_"+uuid.NewString()[:32], `{"jsonrpc":"2.0","id":1,"method":"ping"}`)
	assert.Equal(t, 401, status)

	_, resp := mcpCall(t, s, "X-API-Key", key, callToolMsg("whatomate_call", `{"method":"GET","path":"/api/contacts"}`))
	text, isErr := toolText(t, resp)
	require.False(t, isErr, text)
	assert.Contains(t, text, a.Contact.PhoneNumber)

	_, resp = mcpCall(t, s, "X-API-Key", key, callToolMsg("whatomate_list_organizations", `{}`))
	text, isErr = toolText(t, resp)
	require.False(t, isErr, text)
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(text), &out))
	assert.Equal(t, "api_key", out["connection"].(map[string]any)["type"])
}
