package mcp

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

var routeRe = regexp.MustCompile(`\bg\.(GET|POST|PUT|PATCH|DELETE)\(\s*"(/api/[^"]*)"`)

// registeredAPIRoutes parses cmd/whatomate/main.go for every
// g.GET/POST/PUT/PATCH/DELETE("/api/...") registration ("METHOD /path").
func registeredAPIRoutes(t *testing.T) map[string]bool {
	t.Helper()
	src, err := os.ReadFile(filepath.Join("..", "..", "cmd", "whatomate", "main.go"))
	require.NoError(t, err)
	routes := map[string]bool{}
	for _, m := range routeRe.FindAllStringSubmatch(string(src), -1) {
		routes[m[1]+" "+m[2]] = true
	}
	require.NotEmpty(t, routes, "no /api routes found in main.go; did the registration style change?")
	return routes
}

// TestCatalogParity guards against silent drift: every /api route must be
// catalogued or explicitly excluded, and every catalog entry must point at a
// real route. It is skipped only while the catalog is completely empty.
func TestCatalogParity(t *testing.T) {
	if len(Entries()) == 0 {
		t.Skip("catalog incomplete")
	}
	for _, p := range parityProblems(registeredAPIRoutes(t), Entries()) {
		t.Error(p)
	}
}

// parityProblems lists every mismatch between the registered routes and the
// catalog entries.
func parityProblems(routes map[string]bool, es []Entry) []string {
	catalogued := map[string]int{}
	tools := map[string]string{}
	var problems []string
	for _, e := range es {
		key := e.Method + " " + e.Path
		catalogued[key]++
		// A route may be listed twice only to expose it as a second featured
		// tool (GET /api/contacts is both the contact search and the inbox list).
		if catalogued[key] == 2 && e.Tool == "" {
			problems = append(problems, "duplicate catalog entry: "+key)
		}
		if !routes[key] {
			problems = append(problems, "catalog entry has no matching route in main.go: "+key)
		}
		if IsExcluded(e.Method, e.Path) {
			problems = append(problems, "catalog entry is an excluded route: "+key)
		}
		if _, ok := areas[e.Area]; !ok {
			problems = append(problems, "catalog entry uses an unregistered area "+e.Area+": "+key)
		}
		if e.Summary == "" {
			problems = append(problems, "catalog entry has no summary: "+key)
		}
		if e.Tool != "" {
			if prev, dup := tools[e.Tool]; dup {
				problems = append(problems, "featured tool "+e.Tool+" used by both "+prev+" and "+key)
			}
			tools[e.Tool] = key
		}
	}
	for route := range routes {
		if catalogued[route] > 0 {
			continue
		}
		method, path := splitRouteKey(route)
		if !IsExcluded(method, path) {
			problems = append(problems, "route is neither catalogued nor excluded: "+route)
		}
	}
	sort.Strings(problems)
	return problems
}

// TestParityProblems_DetectsDrift proves the guard bites.
func TestParityProblems_DetectsDrift(t *testing.T) {
	useTestCatalog(t)
	routes := map[string]bool{
		"GET /api/contacts": true, "GET /api/new-thing": true, "POST /api/auth/login": true,
	}
	es := []Entry{
		{Area: "zz_test", Method: "GET", Path: "/api/contacts", Summary: "x"},
		{Area: "zz_test", Method: "GET", Path: "/api/contacts", Summary: "x"},
		{Area: "zz_test", Method: "GET", Path: "/api/gone", Summary: "x"},
		{Area: "nope", Method: "POST", Path: "/api/auth/login"},
	}
	got := strings.Join(parityProblems(routes, es), "|")
	for _, want := range []string{
		"route is neither catalogued nor excluded: GET /api/new-thing",
		"catalog entry has no matching route in main.go: GET /api/gone",
		"duplicate catalog entry: GET /api/contacts",
		"catalog entry is an excluded route: POST /api/auth/login",
		"unregistered area nope",
		"has no summary: POST /api/auth/login",
	} {
		require.Contains(t, got, want)
	}
	require.NotContains(t, got, "neither catalogued nor excluded: POST /api/auth/login")
}

func splitRouteKey(k string) (string, string) {
	for i := 0; i < len(k); i++ {
		if k[i] == ' ' {
			return k[:i], k[i+1:]
		}
	}
	return k, ""
}

// TestExcludedRoutesExist keeps the exclusion list honest: every /api
// exclusion must still match at least one registered route.
func TestExcludedRoutesExist(t *testing.T) {
	routes := registeredAPIRoutes(t)
	for _, ex := range ExcludedRoutes {
		if len(ex.Path) < 5 || ex.Path[:5] != "/api/" {
			continue
		}
		found := false
		for route := range routes {
			method, path := splitRouteKey(route)
			if (ex.Method == "*" || ex.Method == method) && excludedPathMatches(ex.Path, path) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("excluded route %s %s matches no registered route", ex.Method, ex.Path)
		}
	}
}
