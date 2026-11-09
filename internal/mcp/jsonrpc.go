package mcp

import (
	"bytes"
	"encoding/json"

	"github.com/valyala/fasthttp"
)

// JSON-RPC 2.0 error codes.
const (
	rpcParseError     = -32700
	rpcInvalidRequest = -32600
	rpcMethodNotFound = -32601
	rpcInvalidParams  = -32602
	rpcInternalError  = -32603
)

// supportedProtocolVersions lists the MCP protocol versions this server speaks,
// newest first.
var supportedProtocolVersions = []string{"2025-06-18", "2025-03-26", "2024-11-05"}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

const serverInstructions = `Whatomate is a WhatsApp Business platform. You act as the connected user, inside one organisation, with exactly that user's role and permissions; every call goes through the same checks as the web app.
Start with whatomate_whoami to see the user, organisation and permissions. For anything not covered by a featured tool: whatomate_list_areas -> whatomate_describe_area(area) -> whatomate_call(method, path, query, body). Use whatomate_upload for file uploads (media, CSV imports).
Dates are ISO 8601 in the API; when showing dates to people, format them as dd/mm/yyyy (dd/mm/yyyy HH:mm with time).
Adding or importing RSVP guests never messages anyone; sending invitations, reminders, templates and campaigns are explicit actions that message real people - confirm with the user before sending.`

// negotiateVersion echoes the client's requested protocol version when it is
// supported, otherwise answers the newest supported version.
func negotiateVersion(requested string) string {
	for _, v := range supportedProtocolVersions {
		if v == requested {
			return v
		}
	}
	return supportedProtocolVersions[0]
}

// handleRPC processes one JSON-RPC message and returns the HTTP status and body
// (nil body for notifications).
func (s *Server) handleRPC(ctx *fasthttp.RequestCtx, c *Caller, raw []byte) (int, []byte) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) > 0 && trimmed[0] == '[' {
		return fasthttp.StatusOK, encodeRPC(nil, nil, &rpcError{rpcInvalidRequest, "batch requests are not supported"})
	}
	var req rpcRequest
	if err := json.Unmarshal(trimmed, &req); err != nil {
		return fasthttp.StatusOK, encodeRPC(nil, nil, &rpcError{rpcParseError, "parse error"})
	}
	if req.JSONRPC != "2.0" || req.Method == "" {
		return fasthttp.StatusOK, encodeRPC(req.ID, nil, &rpcError{rpcInvalidRequest, "invalid JSON-RPC 2.0 request"})
	}
	// Notifications (no id) get 202 and no body, whatever the method.
	if len(req.ID) == 0 {
		return fasthttp.StatusAccepted, nil
	}

	result, rerr := s.route(ctx, c, req)
	return fasthttp.StatusOK, encodeRPC(req.ID, result, rerr)
}

func (s *Server) route(ctx *fasthttp.RequestCtx, c *Caller, req rpcRequest) (any, *rpcError) {
	switch req.Method {
	case "initialize":
		var p struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		_ = json.Unmarshal(req.Params, &p)
		return map[string]any{
			"protocolVersion": negotiateVersion(p.ProtocolVersion),
			"capabilities":    map[string]any{"tools": map[string]any{"listChanged": false}},
			"serverInfo":      map[string]any{"name": "whatomate", "title": "Whatomate", "version": ServerVersion},
			"instructions":    serverInstructions,
		}, nil
	case "ping":
		return map[string]any{}, nil
	case "tools/list":
		return map[string]any{"tools": allTools()}, nil
	case "tools/call":
		var p struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := decodeParams(req.Params, &p); err != nil || p.Name == "" {
			return nil, &rpcError{rpcInvalidParams, "tools/call needs params.name"}
		}
		if p.Arguments == nil {
			p.Arguments = map[string]any{}
		}
		return s.callTool(ctx, c, p.Name, p.Arguments)
	default:
		return nil, &rpcError{rpcMethodNotFound, "method not found: " + req.Method}
	}
}

func decodeParams(raw json.RawMessage, v any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	return d.Decode(v)
}

func encodeRPC(id json.RawMessage, result any, rerr *rpcError) []byte {
	if len(id) == 0 {
		id = json.RawMessage("null")
	}
	resp := rpcResponse{JSONRPC: "2.0", ID: id}
	if rerr != nil {
		resp.Error = rerr
	} else {
		if result == nil {
			result = map[string]any{}
		}
		resp.Result = result
	}
	b, err := marshalNoEscape(resp)
	if err != nil {
		b, _ = json.Marshal(rpcResponse{JSONRPC: "2.0", ID: id, Error: &rpcError{rpcInternalError, "failed to encode result"}})
	}
	return b
}

// marshalNoEscape marshals v without HTML escaping (<, >, & stay readable).
func marshalNoEscape(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}
