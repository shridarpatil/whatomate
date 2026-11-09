package mcp

import (
	"os"
	"regexp"
	"testing"
)

var messagingAreas = map[string]bool{
	"rsvp": true, "campaigns": true, "contacts": true, "messages": true, "templates": true,
	"tags": true, "canned_responses": true, "import_export": true, "flow_media": true,
}

var messagingFeaturedTools = []string{
	"whatomate_search_contacts",
	"whatomate_list_conversations",
	"whatomate_get_messages",
	"whatomate_send_text",
	"whatomate_send_template",
	"whatomate_list_templates",
	"whatomate_list_campaigns",
	"whatomate_get_campaign",
	"whatomate_list_rsvp_events",
	"whatomate_get_rsvp_results",
}

func TestMessagingCatalogEntriesWellFormed(t *testing.T) {
	validMethods := map[string]bool{"GET": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true}
	count := 0
	for _, e := range Entries() {
		if !messagingAreas[e.Area] {
			continue
		}
		count++
		if _, ok := areas[e.Area]; !ok {
			t.Errorf("%s %s: area %q is not registered", e.Method, e.Path, e.Area)
		}
		if e.Summary == "" {
			t.Errorf("%s %s: empty summary", e.Method, e.Path)
		}
		if !validMethods[e.Method] {
			t.Errorf("%s %s: invalid method", e.Method, e.Path)
		}
		if len(e.Path) < 5 || e.Path[:5] != "/api/" {
			t.Errorf("%s %s: path must start with /api/", e.Method, e.Path)
		}
		if e.Body != nil && e.Body["type"] != "object" {
			t.Errorf("%s %s: body schema must have type object", e.Method, e.Path)
		}
		if e.Body != nil && e.Multipart != nil {
			t.Errorf("%s %s: multipart entries must not also declare a JSON body", e.Method, e.Path)
		}
		if e.Multipart != nil && e.Multipart.FileField == "" {
			t.Errorf("%s %s: multipart entry without file field", e.Method, e.Path)
		}
	}
	if count == 0 {
		t.Fatal("no messaging catalog entries registered")
	}
}

func TestMessagingFeaturedToolsSetOnce(t *testing.T) {
	seen := map[string]int{}
	for _, e := range Entries() {
		if e.Tool != "" {
			seen[e.Tool]++
		}
	}
	for _, name := range messagingFeaturedTools {
		if seen[name] != 1 {
			t.Errorf("featured tool %s set %d times, want exactly 1", name, seen[name])
		}
	}
}

// TestMessagingRoutesCatalogued checks that every route main.go registers under
// the prefixes owned by these catalog files has a catalog entry.
func TestMessagingRoutesCatalogued(t *testing.T) {
	src, err := os.ReadFile("../../cmd/whatomate/main.go")
	if err != nil {
		t.Skipf("main.go not readable: %v", err)
	}
	re := regexp.MustCompile(`g\.(GET|POST|PUT|PATCH|DELETE)\("(/api/(?:rsvp-events|campaigns|contacts|messages|templates|tags|canned-responses|chat|media|import|export|flow-media)(?:/[^"]*)?)"`)
	have := map[string]bool{}
	for _, e := range Entries() {
		have[e.Method+" "+e.Path] = true
	}
	matches := re.FindAllStringSubmatch(string(src), -1)
	if len(matches) == 0 {
		t.Fatal("no routes found in main.go")
	}
	for _, m := range matches {
		if key := m[1] + " " + m[2]; !have[key] {
			t.Errorf("route %s is not catalogued", key)
		}
	}
}
