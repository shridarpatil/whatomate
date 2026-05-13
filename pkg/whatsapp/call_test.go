package whatsapp_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/shridarpatil/whatomate/pkg/whatsapp"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// callServer captures the last request body for assertion and lets the test
// override the response.
type callServer struct {
	mu        sync.Mutex
	server    *httptest.Server
	LastPath  string
	LastBody  map[string]any
	LastQuery string
	Response  func(w http.ResponseWriter, r *http.Request)
}

func newCallServer(t *testing.T) *callServer {
	t.Helper()
	cs := &callServer{}
	cs.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cs.mu.Lock()
		cs.LastPath = r.URL.Path
		cs.LastQuery = r.URL.RawQuery
		if r.Body != nil {
			body := map[string]any{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			cs.LastBody = body
		}
		fn := cs.Response
		cs.mu.Unlock()
		if fn == nil {
			_, _ = w.Write([]byte(`{"success":true}`))
			return
		}
		fn(w, r)
	}))
	t.Cleanup(cs.server.Close)
	return cs
}

func newCallTestClient(serverURL string) *whatsapp.Client {
	return whatsapp.NewWithBaseURL(testutil.NopLogger(), serverURL)
}

// --- PreAcceptCall ---

func TestClient_PreAcceptCall_SuccessfulRequest(t *testing.T) {
	srv := newCallServer(t)
	client := newCallTestClient(srv.server.URL)

	err := client.PreAcceptCall(context.Background(), &whatsapp.Account{
		PhoneID: "phone-1", APIVersion: "v18.0", AccessToken: "tok",
	}, "wacid.123", "v=0\r\no=- 1234 ...")
	require.NoError(t, err)

	assert.Equal(t, "/v18.0/phone-1/calls", srv.LastPath)
	assert.Equal(t, "whatsapp", srv.LastBody["messaging_product"])
	assert.Equal(t, "wacid.123", srv.LastBody["call_id"])
	assert.Equal(t, "pre_accept", srv.LastBody["action"])
	session := srv.LastBody["session"].(map[string]any)
	assert.Equal(t, "answer", session["sdp_type"])
	assert.Contains(t, session["sdp"].(string), "v=0")
}

func TestClient_PreAcceptCall_APIErrorWrapped(t *testing.T) {
	srv := newCallServer(t)
	srv.Response = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"bad sdp","code":100}}`))
	}
	client := newCallTestClient(srv.server.URL)

	err := client.PreAcceptCall(context.Background(), &whatsapp.Account{
		PhoneID: "phone-1", APIVersion: "v18.0", AccessToken: "tok",
	}, "wacid.123", "sdp")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to pre-accept call")
	assert.Contains(t, err.Error(), "bad sdp")
}

// --- AcceptCall ---

func TestClient_AcceptCall_PostsCorrectAction(t *testing.T) {
	srv := newCallServer(t)
	client := newCallTestClient(srv.server.URL)

	err := client.AcceptCall(context.Background(), &whatsapp.Account{
		PhoneID: "p", APIVersion: "v18.0", AccessToken: "tok",
	}, "wacid.123", "answer-sdp")
	require.NoError(t, err)
	assert.Equal(t, "accept", srv.LastBody["action"])
	session := srv.LastBody["session"].(map[string]any)
	assert.Equal(t, "answer-sdp", session["sdp"])
}

// --- RejectCall ---

func TestClient_RejectCall_NoSessionInBody(t *testing.T) {
	srv := newCallServer(t)
	client := newCallTestClient(srv.server.URL)

	err := client.RejectCall(context.Background(), &whatsapp.Account{
		PhoneID: "p", APIVersion: "v18.0", AccessToken: "tok",
	}, "wacid.123")
	require.NoError(t, err)
	assert.Equal(t, "reject", srv.LastBody["action"])
	_, hasSession := srv.LastBody["session"]
	assert.False(t, hasSession, "reject must not include a session/SDP")
}

// --- TerminateCall ---

func TestClient_TerminateCall_PostsTerminate(t *testing.T) {
	srv := newCallServer(t)
	client := newCallTestClient(srv.server.URL)

	err := client.TerminateCall(context.Background(), &whatsapp.Account{
		PhoneID: "p", APIVersion: "v18.0", AccessToken: "tok",
	}, "wacid.999")
	require.NoError(t, err)
	assert.Equal(t, "terminate", srv.LastBody["action"])
	assert.Equal(t, "wacid.999", srv.LastBody["call_id"])
}

// --- InitiateCall ---

func TestClient_InitiateCall_ParsesCallID(t *testing.T) {
	srv := newCallServer(t)
	srv.Response = func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"calls":[{"id":"wacid.outgoing-1"}]}`))
	}
	client := newCallTestClient(srv.server.URL)

	id, err := client.InitiateCall(context.Background(), &whatsapp.Account{
		PhoneID: "p", APIVersion: "v18.0", AccessToken: "tok",
	}, whatsapp.Recipient{Phone: "1234567890"}, "v=0\r\no=- offer")
	require.NoError(t, err)
	assert.Equal(t, "wacid.outgoing-1", id)

	assert.Equal(t, "connect", srv.LastBody["action"])
	assert.Equal(t, "1234567890", srv.LastBody["to"])
	session := srv.LastBody["session"].(map[string]any)
	assert.Equal(t, "offer", session["sdp_type"])
}

func TestClient_InitiateCall_EmptyResponseRejected(t *testing.T) {
	srv := newCallServer(t)
	srv.Response = func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	}
	client := newCallTestClient(srv.server.URL)

	_, err := client.InitiateCall(context.Background(), &whatsapp.Account{
		PhoneID: "p", APIVersion: "v18.0", AccessToken: "tok",
	}, whatsapp.Recipient{Phone: "1234567890"}, "sdp")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse call_id")
}

// --- SendCallPermissionRequest ---

func TestClient_SendCallPermissionRequest_DefaultBody(t *testing.T) {
	srv := newCallServer(t)
	srv.Response = func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"messages":[{"id":"wamid.perm-1"}]}`))
	}
	client := newCallTestClient(srv.server.URL)

	id, err := client.SendCallPermissionRequest(context.Background(), &whatsapp.Account{
		PhoneID: "p", APIVersion: "v18.0", AccessToken: "tok",
	}, whatsapp.Recipient{Phone: "1234567890"}, "")
	require.NoError(t, err)
	assert.Equal(t, "wamid.perm-1", id)

	interactive := srv.LastBody["interactive"].(map[string]any)
	body := interactive["body"].(map[string]any)
	assert.Contains(t, body["text"], "We'd like to call", "default body should be filled in when caller passes \"\"")
	assert.Equal(t, "call_permission_request", interactive["type"])
}

func TestClient_SendCallPermissionRequest_CustomBody(t *testing.T) {
	srv := newCallServer(t)
	srv.Response = func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"messages":[{"id":"wamid.perm-2"}]}`))
	}
	client := newCallTestClient(srv.server.URL)

	id, err := client.SendCallPermissionRequest(context.Background(), &whatsapp.Account{
		PhoneID: "p", APIVersion: "v18.0", AccessToken: "tok",
	}, whatsapp.Recipient{Phone: "555"}, "Custom prompt")
	require.NoError(t, err)
	assert.Equal(t, "wamid.perm-2", id)

	interactive := srv.LastBody["interactive"].(map[string]any)
	body := interactive["body"].(map[string]any)
	assert.Equal(t, "Custom prompt", body["text"])
}

// --- GetCallPermission ---

func TestClient_GetCallPermission_ParsesStatus(t *testing.T) {
	srv := newCallServer(t)
	srv.Response = func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"permission":{"status":"temporary"}}`))
	}
	client := newCallTestClient(srv.server.URL)

	status, err := client.GetCallPermission(context.Background(), &whatsapp.Account{
		PhoneID: "p", APIVersion: "v18.0", AccessToken: "tok",
	}, "1234567890")
	require.NoError(t, err)
	assert.Equal(t, "temporary", status)
	assert.Contains(t, srv.LastQuery, "user_wa_id=1234567890")
	assert.Equal(t, "/v18.0/p/call_permissions", srv.LastPath)
}

func TestClient_GetCallPermission_MalformedResponse(t *testing.T) {
	srv := newCallServer(t)
	srv.Response = func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`not json`))
	}
	client := newCallTestClient(srv.server.URL)

	_, err := client.GetCallPermission(context.Background(), &whatsapp.Account{
		PhoneID: "p", APIVersion: "v18.0", AccessToken: "tok",
	}, "1234567890")
	require.Error(t, err)
}

// --- Auth header passed through ---

func TestClient_PreAcceptCall_PassesBearerToken(t *testing.T) {
	srv := newCallServer(t)
	srv.Response = func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer the-token", r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`{"success":true}`))
	}
	client := newCallTestClient(srv.server.URL)

	err := client.PreAcceptCall(context.Background(), &whatsapp.Account{
		PhoneID: "p", APIVersion: "v18.0", AccessToken: "the-token",
	}, "wacid.x", "sdp")
	require.NoError(t, err)
	// Touch strings package so the assertion above doesn't false-imply unused.
	_ = strings.TrimSpace("")
}
