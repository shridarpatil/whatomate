package mcp

import (
	"encoding/json"
	"testing"

	"github.com/nikyjain/whatomate/internal/config"
	"github.com/nikyjain/whatomate/internal/handlers"
	"github.com/nikyjain/whatomate/test/testutil"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

// withTestEntries temporarily replaces the catalog with the given areas and
// entries (restored on cleanup). Tests using it must not run in parallel.
func withTestEntries(t *testing.T, as []Area, es ...Entry) {
	t.Helper()
	// The test catalog replaces the real one so unit tests stay independent of
	// the production catalog files.
	savedAreas, savedEntries := areas, entries
	areas = map[string]Area{}
	entries = append([]Entry(nil), es...)
	for _, a := range as {
		registerArea(a)
	}
	t.Cleanup(func() { areas, entries = savedAreas, savedEntries })
}

var testArea = Area{Key: "zz_test", Title: "Test area", Description: "test-only endpoints"}

// testEntries are test-only catalog entries used across the unit tests.
var testEntries = []Entry{
	{Area: "zz_test", Method: "GET", Path: "/api/contacts", Summary: "List contacts", Permission: "contacts:read",
		Query: []Param{{Name: "search", Type: "string"}, {Name: "page", Type: "integer"}}, Tool: "whatomate_search_contacts"},
	{Area: "zz_test", Method: "GET", Path: "/api/contacts/{id}", Summary: "Get contact"},
	{Area: "zz_test", Method: "PUT", Path: "/api/contacts/{id}", Summary: "Update contact",
		Body: map[string]any{"type": "object", "properties": map[string]any{"profile_name": map[string]any{"type": "string"}}}},
	{Area: "zz_test", Method: "POST", Path: "/api/contacts/import", Summary: "Import contacts CSV",
		Multipart: &MultipartSpec{FileField: "file", Accept: "text/csv"}},
	{Area: "zz_test", Method: "GET", Path: "/api/users", Summary: "List users", Permission: "users:read"},
	{Area: "zz_test", Method: "POST", Path: "/api/messages", Summary: "Send text", Tool: "whatomate_send_text",
		Body: map[string]any{"type": "object", "required": []string{"contact_id"}}, SideEffects: "sends WhatsApp messages"},
}

func useTestCatalog(t *testing.T) { withTestEntries(t, []Area{testArea}, testEntries...) }

// newUnitServer builds a Server with config only (no DB/Redis).
func newUnitServer(t *testing.T) *Server {
	t.Helper()
	app := &handlers.App{
		Config: &config.Config{
			JWT: config.JWTConfig{Secret: testutil.TestJWTSecret, AccessExpiryMins: 15},
			MCP: config.MCPConfig{Enabled: true},
		},
		Log: testutil.NopLogger(),
	}
	return New(app)
}

func newRequest(method, path string, body []byte) *fastglue.Request {
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(method)
	ctx.Request.SetRequestURI(path)
	ctx.Request.Header.SetHost("wa.example.com")
	if body != nil {
		ctx.Request.Header.SetContentType("application/json")
		ctx.Request.SetBody(body)
	}
	return &fastglue.Request{RequestCtx: ctx}
}

// rpc runs one JSON-RPC message through handleRPC and decodes the response.
func rpc(t *testing.T, s *Server, c *Caller, msg string) (int, map[string]any) {
	t.Helper()
	status, body := s.handleRPC(&fasthttp.RequestCtx{}, c, []byte(msg))
	if body == nil {
		return status, nil
	}
	var out map[string]any
	require.NoError(t, json.Unmarshal(body, &out), string(body))
	return status, out
}

// toolText extracts text + isError from a tools/call JSON-RPC response.
func toolText(t *testing.T, resp map[string]any) (string, bool) {
	t.Helper()
	res, ok := resp["result"].(map[string]any)
	require.True(t, ok, "no result in %v", resp)
	content := res["content"].([]any)
	require.NotEmpty(t, content)
	return content[0].(map[string]any)["text"].(string), res["isError"].(bool)
}

// fakeHandler records the last dispatched request and answers with status/body.
type fakeHandler struct {
	last        fasthttp.Request
	remote      string
	status      int
	contentType string
	body        string
	calls       int
}

func (f *fakeHandler) handle(ctx *fasthttp.RequestCtx) {
	f.calls++
	ctx.Request.CopyTo(&f.last)
	f.remote = ctx.RemoteAddr().String()
	if f.status == 0 {
		f.status = 200
	}
	ctx.SetStatusCode(f.status)
	ct := f.contentType
	if ct == "" {
		ct = "application/json"
	}
	ctx.SetContentType(ct)
	ctx.SetBodyString(f.body)
}
