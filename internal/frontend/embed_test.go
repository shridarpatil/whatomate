package frontend

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func serve(t *testing.T, h fasthttp.RequestHandler, uri string) *fasthttp.RequestCtx {
	t.Helper()
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(fasthttp.MethodGet)
	ctx.Request.SetRequestURI(uri)
	h(ctx)
	return ctx
}

// The frontend is only embedded after `make build-prod`; a plain checkout ships
// an empty dist, where Handler serves the "not embedded" notice instead.
func requireEmbedded(t *testing.T) {
	t.Helper()
	if !IsEmbedded() {
		t.Skip("frontend not embedded — run 'make build-prod' first")
	}
}

func TestBuildJSON(t *testing.T) {
	requireEmbedded(t)

	ctx := serve(t, Handler(""), "/build.json")

	require.Equal(t, fasthttp.StatusOK, ctx.Response.StatusCode())
	assert.Equal(t, "application/json", string(ctx.Response.Header.Peek("Content-Type")))
	// A cached build id would report the old build forever, which is exactly
	// the failure this endpoint exists to detect.
	assert.Equal(t, "no-store", string(ctx.Response.Header.Peek("Cache-Control")))

	var body struct {
		Build string `json:"build"`
	}
	require.NoError(t, json.Unmarshal(ctx.Response.Body(), &body))
	assert.Len(t, body.Build, 12)
	assert.Regexp(t, "^[0-9a-f]{12}$", body.Build)
}

func TestBuildIDIsStableAcrossCalls(t *testing.T) {
	requireEmbedded(t)

	h := Handler("")
	first := string(serve(t, h, "/build.json").Response.Body())
	second := string(serve(t, Handler(""), "/build.json").Response.Body())

	// Same binary, same answer: clients compare this value against the one they
	// booted with, so any drift between requests would fake an update and
	// nag every agent into reloading on a loop.
	assert.Equal(t, first, second)
}

func TestBuildJSONIsNotSwallowedBySPAFallback(t *testing.T) {
	requireEmbedded(t)

	h := Handler("")
	spa := serve(t, h, "/chat/123")
	build := serve(t, h, "/build.json")

	// An unknown route renders the SPA shell; /build.json must not, or the
	// client would parse index.html as JSON and never detect anything.
	assert.Contains(t, string(spa.Response.Header.Peek("Content-Type")), "text/html")
	assert.Equal(t, "application/json", string(build.Response.Header.Peek("Content-Type")))
}
