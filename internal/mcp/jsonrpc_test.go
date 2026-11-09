package mcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

var apiKeyCaller = &Caller{Kind: CallerAPIKey, APIKey: "whm_x"}

func TestInitialize_NegotiatesProtocolVersion(t *testing.T) {
	s := newUnitServer(t)
	for requested, want := range map[string]string{
		"2025-06-18": "2025-06-18",
		"2025-03-26": "2025-03-26",
		"2024-11-05": "2024-11-05",
		"1999-01-01": "2025-06-18",
		"":           "2025-06-18",
	} {
		_, resp := rpc(t, s, apiKeyCaller, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"`+requested+`"}}`)
		res := resp["result"].(map[string]any)
		assert.Equal(t, want, res["protocolVersion"], "requested %q", requested)
		assert.Equal(t, "whatomate", res["serverInfo"].(map[string]any)["name"])
		assert.Contains(t, res["instructions"], "dd/mm/yyyy")
		assert.Contains(t, res["capabilities"], "tools")
	}
}

func TestRPC_PingNotificationsAndErrors(t *testing.T) {
	s := newUnitServer(t)

	status, resp := rpc(t, s, apiKeyCaller, `{"jsonrpc":"2.0","id":"a","method":"ping"}`)
	assert.Equal(t, fasthttp.StatusOK, status)
	assert.Equal(t, "a", resp["id"])
	assert.Equal(t, map[string]any{}, resp["result"])

	status, resp = rpc(t, s, apiKeyCaller, `{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	assert.Equal(t, fasthttp.StatusAccepted, status)
	assert.Nil(t, resp)

	_, resp = rpc(t, s, apiKeyCaller, `{"jsonrpc":"2.0","id":2,"method":"resources/list"}`)
	assert.Equal(t, float64(rpcMethodNotFound), resp["error"].(map[string]any)["code"])

	_, resp = rpc(t, s, apiKeyCaller, `{not json`)
	assert.Equal(t, float64(rpcParseError), resp["error"].(map[string]any)["code"])

	_, resp = rpc(t, s, apiKeyCaller, `[{"jsonrpc":"2.0","id":1,"method":"ping"}]`)
	assert.Equal(t, float64(rpcInvalidRequest), resp["error"].(map[string]any)["code"])

	_, resp = rpc(t, s, apiKeyCaller, `{"jsonrpc":"1.0","id":1,"method":"ping"}`)
	assert.Equal(t, float64(rpcInvalidRequest), resp["error"].(map[string]any)["code"])

	_, resp = rpc(t, s, apiKeyCaller, `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"nope"}}`)
	assert.Equal(t, float64(rpcInvalidParams), resp["error"].(map[string]any)["code"])
}

func TestToolsList_StableAndAnnotated(t *testing.T) {
	useTestCatalog(t)
	s := newUnitServer(t)
	_, resp := rpc(t, s, apiKeyCaller, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	tools := resp["result"].(map[string]any)["tools"].([]any)

	var names []string
	byName := map[string]map[string]any{}
	for _, raw := range tools {
		tool := raw.(map[string]any)
		names = append(names, tool["name"].(string))
		byName[tool["name"].(string)] = tool
	}
	assert.Equal(t, []string{
		toolWhoami, toolListOrgs, toolSwitchOrg, toolListAreas, toolDescribeArea, toolCall, toolUpload,
		"whatomate_search_contacts", "whatomate_send_text",
	}, names)

	ann := func(name string) map[string]any { return byName[name]["annotations"].(map[string]any) }
	assert.Equal(t, true, ann(toolWhoami)["readOnlyHint"])
	assert.Equal(t, true, ann(toolCall)["destructiveHint"])
	assert.Equal(t, true, ann(toolCall)["openWorldHint"])
	assert.Equal(t, true, ann(toolUpload)["destructiveHint"])
	assert.Equal(t, true, ann("whatomate_search_contacts")["readOnlyHint"])
	assert.Equal(t, true, ann("whatomate_send_text")["destructiveHint"])

	schema := byName["whatomate_search_contacts"]["inputSchema"].(map[string]any)
	props := schema["properties"].(map[string]any)
	assert.Contains(t, props, "search")
	assert.Contains(t, props, "page")
	send := byName["whatomate_send_text"]["inputSchema"].(map[string]any)
	assert.Equal(t, []any{"body"}, send["required"])
	assert.Contains(t, byName["whatomate_send_text"]["description"], "sends WhatsApp messages")

	// The list is deterministic.
	_, again := rpc(t, s, apiKeyCaller, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	require.Equal(t, resp["result"], again["result"])
}

func TestListAndDescribeAreas(t *testing.T) {
	useTestCatalog(t)
	s := newUnitServer(t)
	_, resp := rpc(t, s, apiKeyCaller, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"whatomate_list_areas"}}`)
	text, isErr := toolText(t, resp)
	assert.False(t, isErr)
	assert.Contains(t, text, `"key":"zz_test"`)
	assert.Contains(t, text, `"endpoint_count":6`)

	_, resp = rpc(t, s, apiKeyCaller, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"whatomate_describe_area","arguments":{"area":"zz_test"}}}`)
	text, isErr = toolText(t, resp)
	assert.False(t, isErr)
	assert.Contains(t, text, `/api/contacts/{id}`)

	_, resp = rpc(t, s, apiKeyCaller, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"whatomate_describe_area","arguments":{"area":"nope"}}}`)
	_, isErr = toolText(t, resp)
	assert.True(t, isErr)
}

func TestSwitchOrganization_RejectsAPIKey(t *testing.T) {
	s := newUnitServer(t)
	_, resp := rpc(t, s, apiKeyCaller, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"whatomate_switch_organization","arguments":{"organization_id":"0b6c1d1e-0000-4000-8000-000000000001"}}}`)
	text, isErr := toolText(t, resp)
	assert.True(t, isErr)
	assert.Contains(t, text, "API keys are fixed to one organisation")
}
