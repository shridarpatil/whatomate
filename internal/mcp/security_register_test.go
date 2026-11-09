package mcp

import (
	"encoding/json"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nikyjain/whatomate/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func postRegister(t *testing.T, s *Server) int {
	t.Helper()
	r := newRequest("POST", "/oauth/register", []byte(`{"client_name":"x","redirect_uris":["`+testRedirectURI+`"]}`))
	require.NoError(t, s.Register(r))
	return r.RequestCtx.Response.StatusCode()
}

// Fix 4: without Redis, dynamic client registration fails closed.
func TestRegister_FailsClosedWithoutRedis(t *testing.T) {
	s := newUnitServer(t)
	assert.Equal(t, fasthttp.StatusServiceUnavailable, postRegister(t, s))
}

// Fix 4: a global cap applies across all client IPs.
func TestRegister_GlobalCap(t *testing.T) {
	s := newDBServer(t)
	s.globalRegisterMax = 2
	assert.Equal(t, fasthttp.StatusCreated, postRegister(t, s))
	assert.Equal(t, fasthttp.StatusCreated, postRegister(t, s))
	assert.Equal(t, fasthttp.StatusTooManyRequests, postRegister(t, s))
}

// Fix 4: each registration prunes clients older than 24h that never got a grant.
func TestRegister_PrunesUnusedClients(t *testing.T) {
	s := newDBServer(t)
	db := s.app.DB
	mk := func(age time.Duration) string {
		c := models.MCPOAuthClient{ClientID: "wmcp_c_test" + uuid.NewString()[:20], ClientName: "old",
			RedirectURIs: models.StringArray{testRedirectURI}}
		require.NoError(t, db.Create(&c).Error)
		require.NoError(t, db.Model(&c).Update("created_at", time.Now().Add(-age)).Error)
		return c.ClientID
	}
	stale, fresh, granted := mk(20*365*24*time.Hour), mk(time.Hour), mk(48*time.Hour)
	fx := newOrgFixture(t, s)
	require.NoError(t, db.Create(&models.MCPGrant{UserID: fx.User.ID, OrganizationID: fx.Org.ID, ClientID: granted, ClientName: "old"}).Error)

	assert.Equal(t, fasthttp.StatusCreated, postRegister(t, s))
	exists := func(id string) bool {
		var n int64
		require.NoError(t, db.Model(&models.MCPOAuthClient{}).Where("client_id = ?", id).Count(&n).Error)
		return n == 1
	}
	assert.False(t, exists(stale), "stale client without a grant is pruned")
	assert.True(t, exists(fresh), "recent client is kept")
	assert.True(t, exists(granted), "client with a grant is kept")
}

// Fix 3: the consent page learns where access goes and whether the app is a
// recognised Claude app.
func TestClientInfo_RedirectHostAndVerified(t *testing.T) {
	s := newDBServer(t)
	fx := newOrgFixture(t, s)
	r := newRequest("POST", "/oauth/register", []byte(`{"client_name":"Claude","redirect_uris":["`+testRedirectURI+`","https://evil.example/cb"]}`))
	require.NoError(t, s.Register(r))
	var reg map[string]any
	require.NoError(t, json.Unmarshal(r.RequestCtx.Response.Body(), &reg))
	clientID := reg["client_id"].(string)

	info := func(redirect string) map[string]any {
		r := newRequest("GET", "/api/oauth/client?client_id="+clientID+"&redirect_uri="+url.QueryEscape(redirect), nil)
		r.RequestCtx.SetUserValue("user_id", fx.User.ID)
		require.NoError(t, s.ClientInfo(r))
		var env struct {
			Data map[string]any `json:"data"`
		}
		require.NoError(t, json.Unmarshal(r.RequestCtx.Response.Body(), &env))
		return env.Data
	}
	d := info(testRedirectURI)
	assert.Equal(t, true, d["verified"])
	assert.Equal(t, "https://claude.ai", d["redirect_host"])
	d = info("https://evil.example/cb")
	assert.Equal(t, false, d["verified"])
	assert.Equal(t, true, d["redirect_uri_valid"])
	assert.Equal(t, "https://evil.example", d["redirect_host"])
}
