package mcp

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// adminCatalogAreas are the areas defined in catalog_{profile,users,roles,
// api_keys,organizations,teams,settings,admin,analytics,audit_logs,calls}.go.
var adminCatalogAreas = []string{
	"profile", "users", "roles", "api_keys", "organizations", "teams",
	"settings", "admin", "analytics", "audit_logs", "calls",
}

// adminCatalogPrefixes are the /api prefixes those areas own.
var adminCatalogPrefixes = []string{
	"/api/me", "/api/users", "/api/roles", "/api/permissions", "/api/api-keys",
	"/api/organizations", "/api/org", "/api/teams", "/api/settings", "/api/admin",
	"/api/analytics", "/api/widgets", "/api/audit-logs", "/api/calls",
	"/api/call-logs", "/api/call-transfers", "/api/ivr-flows",
}

// adminCatalogExcluded are routes under those prefixes deliberately left out.
var adminCatalogExcluded = map[string]bool{
	"PUT /api/me/password": true,
}

func adminEntries() []Entry {
	var out []Entry
	for _, a := range adminCatalogAreas {
		out = append(out, EntriesForArea(a)...)
	}
	return out
}

func TestAdminCatalogEntriesWellFormed(t *testing.T) {
	validMethods := map[string]bool{"GET": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true}
	known := map[string]bool{}
	for _, a := range Areas() {
		known[a.Key] = true
	}
	for _, a := range adminCatalogAreas {
		if !known[a] {
			t.Errorf("area %q not registered", a)
		}
		if len(EntriesForArea(a)) == 0 {
			t.Errorf("area %q has no entries", a)
		}
	}
	seen := map[string]bool{}
	for _, e := range adminEntries() {
		key := e.Method + " " + e.Path
		if seen[key] {
			t.Errorf("duplicate entry %s", key)
		}
		seen[key] = true
		if strings.TrimSpace(e.Summary) == "" {
			t.Errorf("%s: empty Summary", key)
		}
		if !validMethods[e.Method] {
			t.Errorf("%s: invalid method", key)
		}
		if !strings.HasPrefix(e.Path, "/api/") {
			t.Errorf("%s: path must start with /api/", key)
		}
		if e.Body != nil && e.Body["type"] != "object" {
			t.Errorf("%s: body schema must have type object", key)
		}
		if e.Body != nil && e.Multipart != nil {
			t.Errorf("%s: multipart entries must not have a JSON body", key)
		}
		if e.Body != nil && (e.Method == "GET" || e.Method == "DELETE") {
			t.Errorf("%s: GET/DELETE entries should not have a body", key)
		}
	}
}

func TestAdminCatalogFeaturedToolOnce(t *testing.T) {
	n := 0
	for _, e := range Entries() {
		if e.Tool == "whatomate_dashboard_analytics" {
			n++
			if e.Method != "GET" || e.Path != "/api/analytics/dashboard" {
				t.Errorf("whatomate_dashboard_analytics on %s %s", e.Method, e.Path)
			}
		}
	}
	if n != 1 {
		t.Fatalf("whatomate_dashboard_analytics set %d times, want 1", n)
	}
}

// TestAdminCatalogCoversMainRoutes checks that every route registered in
// main.go under the owned prefixes is catalogued (or explicitly excluded), and
// that no owned entry points at a route that does not exist.
func TestAdminCatalogCoversMainRoutes(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("..", "..", "cmd", "whatomate", "main.go"))
	if err != nil {
		t.Skipf("main.go not readable: %v", err)
	}
	re := regexp.MustCompile(`g\.(GET|POST|PUT|PATCH|DELETE)\("(/api/[^"]*)"`)
	routes := map[string]bool{}
	for _, m := range re.FindAllStringSubmatch(string(src), -1) {
		if adminOwnsPath(m[2]) {
			routes[m[1]+" "+m[2]] = true
		}
	}
	catalogued := map[string]bool{}
	for _, e := range adminEntries() {
		catalogued[e.Method+" "+e.Path] = true
	}
	for r := range routes {
		if !catalogued[r] && !adminCatalogExcluded[r] {
			t.Errorf("route %s not catalogued", r)
		}
	}
	for c := range catalogued {
		if !routes[c] {
			t.Errorf("catalog entry %s has no route in main.go", c)
		}
	}
}

func adminOwnsPath(p string) bool {
	for _, pre := range adminCatalogPrefixes {
		if p == pre || strings.HasPrefix(p, pre+"/") {
			return true
		}
	}
	return false
}
