package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	truncationNote        = "\n\n[truncated - result exceeded 60000 characters; use page/limit/search filters to narrow it]"
	responseTruncatedNote = "\n\n[response truncated at 2 MB]"
)

// toolResult is an MCP tools/call result.
type toolResult struct {
	Content []textContent `json:"content"`
	IsError bool          `json:"isError"`
}

type textContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func textResult(text string, isError bool) *toolResult {
	return &toolResult{Content: []textContent{{Type: "text", Text: truncate(text)}}, IsError: isError}
}

func errorResult(format string, args ...any) *toolResult {
	return textResult(fmt.Sprintf(format, args...), true)
}

// jsonResult renders v as compact JSON.
func jsonResult(v any) *toolResult {
	b, err := marshalNoEscape(v)
	if err != nil {
		return errorResult("failed to encode result: %v", err)
	}
	return textResult(string(b), false)
}

// truncate caps s at maxResultChars characters (rune-safe) with a note.
func truncate(s string) string {
	if utf8.RuneCountInString(s) <= maxResultChars {
		return s
	}
	cut := 0
	for i := range s {
		if cut == maxResultChars {
			return s[:i] + truncationNote
		}
		cut++
	}
	return s
}

// shapeResponse turns a captured API response into a tool result:
// HTTP >= 400 -> isError with status and the error envelope's message; JSON ->
// compact JSON (a {"status":"success","data":X} envelope is unwrapped to X);
// text/CSV -> as text; anything else -> a short description of the binary.
func shapeResponse(resp *apiResponse) *toolResult {
	res := shapeResponseBody(resp)
	if resp.Truncated && len(res.Content) > 0 {
		res.Content[0].Text += responseTruncatedNote
	}
	return res
}

func shapeResponseBody(resp *apiResponse) *toolResult {
	ct := strings.ToLower(resp.ContentType)
	isJSON := strings.Contains(ct, "json") || (ct == "" && json.Valid(resp.Body))

	if resp.Status >= 400 {
		msg := strings.TrimSpace(string(resp.Body))
		if isJSON {
			var env struct {
				Message string `json:"message"`
				Error   string `json:"error"`
			}
			if json.Unmarshal(resp.Body, &env) == nil {
				if env.Message != "" {
					msg = env.Message
				} else if env.Error != "" {
					msg = env.Error
				}
			}
		}
		if msg == "" {
			msg = "request failed"
		}
		return errorResult("HTTP %d: %s", resp.Status, msg)
	}

	switch {
	case isJSON:
		return textResult(compactJSON(resp.Body), false)
	case isTextual(ct):
		return textResult(string(resp.Body), false)
	case len(resp.Body) == 0:
		return textResult(fmt.Sprintf("HTTP %d (no content)", resp.Status), false)
	default:
		return jsonResult(map[string]any{
			"content_type": resp.ContentType,
			"bytes":        len(resp.Body),
			"note":         "binary content is not returned through the connector; download it from the Whatomate web app",
		})
	}
}

func isTextual(ct string) bool {
	return strings.HasPrefix(ct, "text/") || strings.Contains(ct, "csv") || strings.Contains(ct, "xml")
}

// compactJSON unwraps the fastglue success envelope and re-encodes compactly.
func compactJSON(body []byte) string {
	d := json.NewDecoder(bytes.NewReader(body))
	d.UseNumber()
	var v any
	if err := d.Decode(&v); err != nil {
		return string(body)
	}
	if m, ok := v.(map[string]any); ok {
		if status, _ := m["status"].(string); status == "success" {
			if data, has := m["data"]; has {
				v = data
			}
		}
	}
	out, err := marshalNoEscape(v)
	if err != nil {
		return string(body)
	}
	return string(out)
}

// decodeData returns the "data" of a successful JSON envelope.
func decodeData(resp *apiResponse) (any, error) {
	d := json.NewDecoder(bytes.NewReader(resp.Body))
	d.UseNumber()
	var env struct {
		Data any `json:"data"`
	}
	if err := d.Decode(&env); err != nil {
		return nil, err
	}
	return env.Data, nil
}
