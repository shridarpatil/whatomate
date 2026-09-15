package whatsapp_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shridarpatil/whatomate/pkg/whatsapp"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_GetBusinessAccountInfo(t *testing.T) {
	var gotPath, gotQuery, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.Query().Get("fields")
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{
			"id": "107455662167439",
			"name": "Test WhatsApp Business Account",
			"currency": "INR",
			"timezone_id": "71"
		}`))
	}))
	t.Cleanup(srv.Close)
	client := whatsapp.NewWithBaseURL(testutil.NopLogger(), srv.URL)

	info, err := client.GetBusinessAccountInfo(context.Background(), &whatsapp.Account{
		BusinessID: "107455662167439", APIVersion: "v21.0", AccessToken: "tok",
	})
	require.NoError(t, err)
	assert.Equal(t, "107455662167439", info.ID)
	assert.Equal(t, "Test WhatsApp Business Account", info.Name)
	assert.Equal(t, "INR", info.Currency)
	assert.Equal(t, "71", info.TimezoneID)

	assert.Equal(t, "/v21.0/107455662167439", gotPath)
	assert.Equal(t, "currency,timezone_id,name", gotQuery)
	// The token travels in the header, never in the query string.
	assert.Equal(t, "Bearer tok", gotAuth)
}

func TestClient_GetBusinessAccountInfo_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"Invalid OAuth access token","code":190}}`))
	}))
	t.Cleanup(srv.Close)
	client := whatsapp.NewWithBaseURL(testutil.NopLogger(), srv.URL)

	info, err := client.GetBusinessAccountInfo(context.Background(), &whatsapp.Account{
		BusinessID: "WABA-1", APIVersion: "v21.0", AccessToken: "bad",
	})
	require.Error(t, err)
	assert.Nil(t, info)
}
