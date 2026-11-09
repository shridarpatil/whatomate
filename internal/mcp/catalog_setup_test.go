package mcp

import (
	"strings"
	"testing"
)

// setupAreaRoutes is the number of routes main.go registers under the setup
// prefixes (accounts, chatbot, flows, lead ads, catalogs, webhooks, custom
// actions), minus the excluded /api/custom-actions/redirect/{token}.
var setupAreaRoutes = map[string]int{
	"accounts":       27,
	"chatbot":        24,
	"whatsapp_flows": 10,
	"lead_ads":       20,
	"catalogs":       10,
	"webhooks":       6,
	"custom_actions": 6,
}

func TestSetupCatalogEntriesWellFormed(t *testing.T) {
	validMethods := map[string]bool{"GET": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true}
	seen := map[string]bool{}
	counts := map[string]int{}

	for _, e := range Entries() {
		if _, mine := setupAreaRoutes[e.Area]; !mine {
			continue
		}
		counts[e.Area]++
		key := e.Method + " " + e.Path
		if seen[key] {
			t.Errorf("duplicate entry %s", key)
		}
		seen[key] = true
		if strings.TrimSpace(e.Summary) == "" {
			t.Errorf("%s: empty summary", key)
		}
		if !validMethods[e.Method] {
			t.Errorf("%s: invalid method", key)
		}
		if !strings.HasPrefix(e.Path, "/api/") {
			t.Errorf("%s: path must start with /api/", key)
		}
		if strings.HasPrefix(e.Path, "/api/custom-actions/redirect") {
			t.Errorf("%s: redirect route is excluded from the catalog", key)
		}
		if e.Body != nil && e.Body["type"] != "object" {
			t.Errorf("%s: body schema must have type object", key)
		}
		if e.Body != nil && e.Multipart != nil {
			t.Errorf("%s: multipart entries must not set Body", key)
		}
	}

	for area, want := range setupAreaRoutes {
		if _, ok := areas[area]; !ok {
			t.Errorf("area %q not registered", area)
		}
		if counts[area] != want {
			t.Errorf("area %q: %d entries, want %d", area, counts[area], want)
		}
	}
}

func TestSetupCatalogFeaturedTools(t *testing.T) {
	want := map[string]string{
		"whatomate_list_accounts":      "GET /api/accounts",
		"whatomate_list_chatbot_flows": "GET /api/chatbot/flows",
	}
	got := map[string][]string{}
	for _, e := range Entries() {
		if _, ok := want[e.Tool]; ok {
			got[e.Tool] = append(got[e.Tool], e.Method+" "+e.Path)
		}
	}
	for tool, route := range want {
		if len(got[tool]) != 1 {
			t.Errorf("featured tool %s set %d times, want once", tool, len(got[tool]))
			continue
		}
		if got[tool][0] != route {
			t.Errorf("featured tool %s on %s, want %s", tool, got[tool][0], route)
		}
	}
}

func TestSetupCatalogMatchPrefersLiterals(t *testing.T) {
	e, params, ok := Match("POST", "/api/catalogs/sync")
	if !ok || e.Path != "/api/catalogs/sync" || len(params) != 0 {
		t.Fatalf("POST /api/catalogs/sync matched %+v %v", e, params)
	}
	e, params, ok = Match("GET", "/api/leads/stats")
	if !ok || e.Path != "/api/leads/stats" {
		t.Fatalf("GET /api/leads/stats matched %+v %v", e, params)
	}
	e, params, ok = Match("PUT", "/api/chatbot/transfers/abc/assign")
	if !ok || params["id"] != "abc" {
		t.Fatalf("assign transfer matched %+v %v", e, params)
	}
}
