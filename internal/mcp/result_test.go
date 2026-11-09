package mcp

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
)

func TestShapeResponse(t *testing.T) {
	r := shapeResponse(&apiResponse{Status: 200, ContentType: "application/json",
		Body: []byte(`{"status":"success","data":{"name":"A <b> & c","n":12345678901234567890}}`)})
	assert.False(t, r.IsError)
	assert.Equal(t, `{"n":12345678901234567890,"name":"A <b> & c"}`, r.Content[0].Text)

	r = shapeResponse(&apiResponse{Status: 403, ContentType: "application/json",
		Body: []byte(`{"status":"error","message":"Insufficient permissions","data":null}`)})
	assert.True(t, r.IsError)
	assert.Equal(t, "HTTP 403: Insufficient permissions", r.Content[0].Text)

	r = shapeResponse(&apiResponse{Status: 500, ContentType: "text/plain", Body: nil})
	assert.True(t, r.IsError)
	assert.Equal(t, "HTTP 500: request failed", r.Content[0].Text)

	r = shapeResponse(&apiResponse{Status: 200, ContentType: "text/csv", Body: []byte("a,b\n1,2\n")})
	assert.Equal(t, "a,b\n1,2\n", r.Content[0].Text)

	r = shapeResponse(&apiResponse{Status: 200, ContentType: "application/pdf", Body: []byte{0x25, 0x50, 0x44, 0x46}})
	assert.False(t, r.IsError)
	assert.Contains(t, r.Content[0].Text, `"bytes":4`)
	assert.Contains(t, r.Content[0].Text, `"content_type":"application/pdf"`)

	r = shapeResponse(&apiResponse{Status: 204})
	assert.Equal(t, "HTTP 204 (no content)", r.Content[0].Text)
}

func TestTruncate(t *testing.T) {
	short := strings.Repeat("x", maxResultChars)
	assert.Equal(t, short, truncate(short))

	long := strings.Repeat("é", maxResultChars+10)
	out := truncate(long)
	assert.True(t, utf8.ValidString(out))
	assert.True(t, strings.HasSuffix(out, truncationNote))
	assert.Equal(t, maxResultChars, utf8.RuneCountInString(strings.TrimSuffix(out, truncationNote)))

	r := shapeResponse(&apiResponse{Status: 200, ContentType: "text/plain", Body: []byte(strings.Repeat("y", maxResultChars*2))})
	assert.Contains(t, r.Content[0].Text, "use page/limit/search filters")
}
