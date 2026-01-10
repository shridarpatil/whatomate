package testutil

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

// NewJSONRequest creates a fastglue request with a JSON body for testing.
func NewJSONRequest(t *testing.T, body any) *fastglue.Request {
	t.Helper()

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetContentType("application/json")
	ctx.Request.Header.SetMethod("POST")

	if body != nil {
		jsonData, err := json.Marshal(body)
		require.NoError(t, err, "failed to marshal request body")
		ctx.Request.SetBody(jsonData)
	}

	return &fastglue.Request{RequestCtx: ctx}
}

// NewGETRequest creates a fastglue GET request for testing.
func NewGETRequest(t *testing.T) *fastglue.Request {
	t.Helper()

	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod("GET")

	return &fastglue.Request{RequestCtx: ctx}
}

// NewRequest creates an empty fastglue request for testing.
func NewRequest(t *testing.T) *fastglue.Request {
	t.Helper()

	ctx := &fasthttp.RequestCtx{}
	return &fastglue.Request{RequestCtx: ctx}
}

// SetAuthHeader sets a Bearer token Authorization header on the request.
func SetAuthHeader(req *fastglue.Request, token string) {
	req.RequestCtx.Request.Header.Set("Authorization", "Bearer "+token)
}

// SetHeader sets a header on the request.
func SetHeader(req *fastglue.Request, key, value string) {
	req.RequestCtx.Request.Header.Set(key, value)
}

// SetQueryParam sets a query parameter on the request.
func SetQueryParam(req *fastglue.Request, key, value string) {
	req.RequestCtx.QueryArgs().Set(key, value)
}

// SetPathParam sets a path parameter (user value) on the request.
func SetPathParam(req *fastglue.Request, key string, value any) {
	req.RequestCtx.SetUserValue(key, value)
}

// GetResponseBody returns the response body as bytes.
func GetResponseBody(req *fastglue.Request) []byte {
	return req.RequestCtx.Response.Body()
}

// GetResponseStatusCode returns the response status code.
func GetResponseStatusCode(req *fastglue.Request) int {
	return req.RequestCtx.Response.StatusCode()
}

// ParseJSONResponse parses the response body as JSON into the given target.
func ParseJSONResponse(t *testing.T, req *fastglue.Request, target any) {
	t.Helper()

	body := GetResponseBody(req)
	err := json.Unmarshal(body, target)
	require.NoError(t, err, "failed to parse JSON response: %s", string(body))
}

// APIEnvelope represents the standard API response envelope.
type APIEnvelope struct {
	Status string          `json:"status"`
	Data   json.RawMessage `json:"data,omitempty"`
	Error  *APIError       `json:"error,omitempty"`
}

// APIError represents an error in the API envelope.
type APIError struct {
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}

// ParseEnvelopeResponse parses the response as an API envelope and returns the data.
func ParseEnvelopeResponse(t *testing.T, req *fastglue.Request, target any) {
	t.Helper()

	var envelope APIEnvelope
	ParseJSONResponse(t, req, &envelope)

	if target != nil && envelope.Data != nil {
		err := json.Unmarshal(envelope.Data, target)
		require.NoError(t, err, "failed to parse envelope data")
	}
}

// AssertErrorResponse asserts that the response is an error with the expected message.
func AssertErrorResponse(t *testing.T, req *fastglue.Request, expectedStatus int, expectedMessage string) {
	t.Helper()

	require.Equal(t, expectedStatus, GetResponseStatusCode(req), "unexpected status code")

	var envelope APIEnvelope
	ParseJSONResponse(t, req, &envelope)

	require.Equal(t, "error", envelope.Status, "expected error status")
	require.NotNil(t, envelope.Error, "expected error in envelope")
	require.Contains(t, envelope.Error.Message, expectedMessage, "error message mismatch")
}
