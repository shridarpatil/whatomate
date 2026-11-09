package mcp

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/valyala/fasthttp"
)

// callTool runs one tools/call. Unknown tools are a protocol error; everything
// else (bad arguments, API errors) is an isError tool result.
func (s *Server) callTool(ctx *fasthttp.RequestCtx, c *Caller, name string, args map[string]any) (any, *rpcError) {
	switch name {
	case toolWhoami:
		return s.toolWhoami(ctx, c), nil
	case toolListOrgs:
		return s.toolListOrganizations(ctx, c), nil
	case toolSwitchOrg:
		return s.toolSwitchOrganization(c, args), nil
	case toolListAreas:
		return toolListAreasResult(), nil
	case toolDescribeArea:
		return toolDescribeAreaResult(scalarString(args["area"])), nil
	case toolCall:
		return s.toolCallAPI(ctx, c, args), nil
	case toolUpload:
		return s.toolUploadFile(ctx, c, args), nil
	}
	if ft, ok := findFeatured(name); ok {
		path, query, body, err := buildFeaturedCall(ft.entry, args)
		if err != nil {
			return errorResult("%v", err), nil
		}
		return s.callJSON(ctx, c, ft.entry.Method, path, query, body), nil
	}
	return nil, &rpcError{rpcInvalidParams, "unknown tool: " + name}
}

// toolCallAPI implements whatomate_call.
func (s *Server) toolCallAPI(ctx *fasthttp.RequestCtx, c *Caller, args map[string]any) *toolResult {
	method := strings.ToUpper(scalarString(args["method"]))
	path := scalarString(args["path"])
	query := url.Values{}
	if q, ok := args["query"].(map[string]any); ok {
		for k, v := range q {
			addQueryValue(query, k, v)
		}
	} else if args["query"] != nil {
		return errorResult("query must be an object of parameter names to values")
	}
	return s.callJSON(ctx, c, method, path, query, args["body"])
}

// callJSON validates the path against the catalog and dispatches a JSON call.
func (s *Server) callJSON(ctx *fasthttp.RequestCtx, c *Caller, method, path string, query url.Values, body any) *toolResult {
	entry, internalPath, err := resolveCallPath(method, path, s.app.Config.Server.BasePath)
	if err != nil {
		return errorResult("%v", err)
	}
	if entry.Multipart != nil {
		return errorResult("%s %s takes a file upload; use whatomate_upload", entry.Method, entry.Path)
	}
	req := apiRequest{Method: entry.Method, Path: internalPath, Query: query}
	if body != nil {
		if entry.Method == "GET" {
			return errorResult("GET requests take no body; pass filters in `query`")
		}
		b, err := marshalNoEscape(body)
		if err != nil {
			return errorResult("body is not valid JSON: %v", err)
		}
		req.Body, req.ContentType = b, "application/json"
	}
	resp, err := s.dispatch(ctx, c, req)
	if err != nil {
		return errorResult("%v", err)
	}
	return shapeResponse(resp)
}

// scalarString renders a JSON scalar argument as a string.
func scalarString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(t)
	case json.Number:
		return t.String()
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(t)
	default:
		return strings.TrimSpace(fmt.Sprint(t))
	}
}

// addQueryValue adds a JSON argument to query: arrays become repeated values,
// objects are JSON-encoded, scalars are stringified.
func addQueryValue(q url.Values, key string, v any) {
	switch t := v.(type) {
	case nil:
		return
	case []any:
		for _, item := range t {
			addQueryValue(q, key, item)
		}
	case map[string]any:
		b, _ := marshalNoEscape(t)
		q.Add(key, string(b))
	default:
		q.Add(key, scalarString(t))
	}
}
