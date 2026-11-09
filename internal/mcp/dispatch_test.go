package mcp

import (
	"bytes"
	"encoding/base64"
	"io"
	"mime"
	"mime/multipart"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/nikyjain/whatomate/internal/middleware"
	"github.com/nikyjain/whatomate/internal/models"
	"github.com/nikyjain/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCall_DispatchesWithAPIKeyAndQuery(t *testing.T) {
	useTestCatalog(t)
	s := newUnitServer(t)
	f := &fakeHandler{body: `{"status":"success","data":{"contacts":[{"id":"c1"}],"total":1}}`}
	s.SetHandler(f.handle)

	c := &Caller{Kind: CallerAPIKey, APIKey: "whm_0123456789abcdef0123456789abcdef"}
	_, resp := rpc(t, s, c, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"whatomate_call",
		"arguments":{"method":"GET","path":"/api/contacts","query":{"search":"ravi","page":2,"tags":["a","b"]}}}}`)
	text, isErr := toolText(t, resp)
	require.False(t, isErr, text)
	assert.Equal(t, `{"contacts":[{"id":"c1"}],"total":1}`, text)

	assert.Equal(t, "GET", string(f.last.Header.Method()))
	assert.Equal(t, "/api/contacts", string(f.last.URI().Path()))
	args := f.last.URI().QueryArgs()
	assert.Equal(t, "ravi", string(args.Peek("search")))
	assert.Equal(t, "2", string(args.Peek("page")))
	assert.Len(t, args.PeekMulti("tags"), 2)
	assert.Equal(t, c.APIKey, string(f.last.Header.Peek("X-API-Key")))
	assert.Equal(t, "mcp", string(f.last.Header.Peek("X-Whatomate-Client")))
	assert.Empty(t, f.last.Header.Peek("Authorization"))
	assert.True(t, strings.HasPrefix(f.remote, "127.0.0.1"))
}

func TestCall_BodyAndErrors(t *testing.T) {
	useTestCatalog(t)
	s := newUnitServer(t)
	f := &fakeHandler{status: 403, body: `{"status":"error","message":"Insufficient permissions"}`}
	s.SetHandler(f.handle)

	_, resp := rpc(t, s, apiKeyCaller, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"whatomate_call",
		"arguments":{"method":"PUT","path":"/api/contacts/c1","body":{"profile_name":"Asha & Co"}}}}`)
	text, isErr := toolText(t, resp)
	assert.True(t, isErr)
	assert.Equal(t, "HTTP 403: Insufficient permissions", text)
	assert.Equal(t, `{"profile_name":"Asha & Co"}`, string(f.last.Body()))
	assert.Equal(t, "application/json", string(f.last.Header.ContentType()))

	// Rejected paths never reach the handler.
	calls := f.calls
	for _, args := range []string{
		`{"method":"POST","path":"/api/auth/switch-org","body":{}}`,
		`{"method":"GET","path":"/api/contacts/../users"}`,
		`{"method":"GET","path":"/api/contacts","body":{"x":1}}`,
		`{"method":"POST","path":"/api/contacts/import","body":{}}`,
	} {
		_, resp = rpc(t, s, apiKeyCaller, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"whatomate_call","arguments":`+args+`}}`)
		_, isErr = toolText(t, resp)
		assert.True(t, isErr, args)
	}
	assert.Equal(t, calls, f.calls)
}

func TestFeaturedTool_BuildsCall(t *testing.T) {
	useTestCatalog(t)
	s := newUnitServer(t)
	f := &fakeHandler{body: `{"status":"success","data":[]}`}
	s.SetHandler(f.handle)
	_, resp := rpc(t, s, apiKeyCaller, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"whatomate_search_contacts","arguments":{"search":"x y","ignored":"z"}}}`)
	_, isErr := toolText(t, resp)
	require.False(t, isErr)
	assert.Equal(t, "x y", string(f.last.URI().QueryArgs().Peek("search")))
	assert.Empty(t, f.last.URI().QueryArgs().Peek("ignored"))
}

func TestUpload_BuildsMultipart(t *testing.T) {
	useTestCatalog(t)
	s := newUnitServer(t)
	f := &fakeHandler{body: `{"status":"success","data":{"imported":2}}`}
	s.SetHandler(f.handle)
	csv := "phone,name\n919876543210,Asha\n"
	b64 := base64.StdEncoding.EncodeToString([]byte(csv))
	_, resp := rpc(t, s, apiKeyCaller, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"whatomate_upload",
		"arguments":{"path":"/api/contacts/import","file_base64":"`+b64+`","filename":"guests.csv","fields":{"tag":"vip"}}}}`)
	text, isErr := toolText(t, resp)
	require.False(t, isErr, text)

	_, params, err := mime.ParseMediaType(string(f.last.Header.ContentType()))
	require.NoError(t, err)
	mr := multipart.NewReader(bytes.NewReader(f.last.Body()), params["boundary"])
	got := map[string]string{}
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		data, _ := io.ReadAll(p)
		got[p.FormName()] = string(data)
		if p.FormName() == "file" {
			assert.Equal(t, "guests.csv", p.FileName())
			assert.Contains(t, p.Header.Get("Content-Type"), "csv")
		}
	}
	assert.Equal(t, csv, got["file"])
	assert.Equal(t, "vip", got["tag"])

	// JSON endpoints and bad base64 are refused.
	_, resp = rpc(t, s, apiKeyCaller, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"whatomate_upload","arguments":{"path":"/api/contacts","file_base64":"`+b64+`","filename":"a.csv"}}}`)
	_, isErr = toolText(t, resp)
	assert.True(t, isErr)
	_, resp = rpc(t, s, apiKeyCaller, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"whatomate_upload","arguments":{"path":"/api/contacts/import","file_base64":"%%%","filename":"a.csv"}}}`)
	_, isErr = toolText(t, resp)
	assert.True(t, isErr)
}

func TestDispatch_OAuthMintsShortLivedSessionJWT(t *testing.T) {
	useTestCatalog(t)
	s := newUnitServer(t)
	f := &fakeHandler{body: `{"status":"success","data":{}}`}
	s.SetHandler(f.handle)
	roleID := uuid.New()
	user := &models.User{BaseModel: models.BaseModel{ID: uuid.New()}, OrganizationID: uuid.New(), Email: "a@b.c", RoleID: &roleID}
	c := &Caller{Kind: CallerOAuth, UserID: user.ID, OrgID: user.OrganizationID, User: user, Grant: &models.MCPGrant{}}

	_, err := s.dispatch(nil, c, apiRequest{Method: "GET", Path: "/api/contacts"})
	require.NoError(t, err)
	auth := string(f.last.Header.Peek("Authorization"))
	require.True(t, strings.HasPrefix(auth, "Bearer "))
	assert.Empty(t, f.last.Header.Peek("X-API-Key"))

	claims := &middleware.JWTClaims{}
	_, err = jwt.ParseWithClaims(strings.TrimPrefix(auth, "Bearer "), claims, func(*jwt.Token) (any, error) {
		return []byte(testutil.TestJWTSecret), nil
	})
	require.NoError(t, err)
	assert.Equal(t, user.ID, claims.UserID)
	assert.Equal(t, user.OrganizationID, claims.OrganizationID)
	assert.Equal(t, roleID, *claims.RoleID)
	assert.Equal(t, "whatomate", claims.Issuer)
	assert.WithinDuration(t, time.Now().Add(internalTokenTTL), claims.ExpiresAt.Time, 5*time.Second)
}

func TestDispatch_NoHandler(t *testing.T) {
	s := newUnitServer(t)
	_, err := s.dispatch(nil, apiKeyCaller, apiRequest{Method: "GET", Path: "/api/contacts"})
	assert.Error(t, err)
}
