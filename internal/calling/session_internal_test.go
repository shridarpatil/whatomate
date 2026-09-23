package calling

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/crypto"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/pkg/whatsapp"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// authCapture records the Authorization header of the last request.
type authCapture struct {
	mu   sync.Mutex
	auth string
	hits int
}

func (a *authCapture) server(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a.mu.Lock()
		a.auth = r.Header.Get("Authorization")
		a.hits++
		a.mu.Unlock()
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// Calls that skip the IVR (voice_call button / sticky routing) have no
// handler-supplied account in scope when the agent hangs up, so termination
// runs off the account captured on the session. Reloading it from the DB here
// would send Meta the encrypted token and leave the caller's leg up.
func TestTerminateCallBySession_UsesSessionAccount(t *testing.T) {
	ac := &authCapture{}
	srv := ac.server(t)

	m := &Manager{
		log:      testutil.NopLogger(),
		whatsapp: whatsapp.NewWithBaseURL(testutil.NopLogger(), srv.URL),
	}

	m.terminateCallBySession(&CallSession{
		ID:             "wacid.test",
		OrganizationID: uuid.New(),
		AccountName:    "acct",
		WAAccount: &whatsapp.Account{
			PhoneID:     "phone-1",
			APIVersion:  "v18.0",
			AccessToken: "plaintext-access-token",
		},
	})

	ac.mu.Lock()
	defer ac.mu.Unlock()
	assert.Equal(t, 1, ac.hits, "terminate must reach the API")
	assert.Equal(t, "Bearer plaintext-access-token", ac.auth)
}

func TestTerminateCallBySession_NoAccountDoesNotCallAPI(t *testing.T) {
	ac := &authCapture{}
	srv := ac.server(t)

	m := &Manager{
		log:      testutil.NopLogger(),
		whatsapp: whatsapp.NewWithBaseURL(testutil.NopLogger(), srv.URL),
	}

	m.terminateCallBySession(&CallSession{ID: "wacid.test", AccountName: "acct"})

	ac.mu.Lock()
	defer ac.mu.Unlock()
	assert.Zero(t, ac.hits)
}

// The session's account must be the decrypted one. If a caller ever stores the
// raw DB record's token, Meta receives ciphertext and the call is never ended.
func TestSessionAccount_IsDecryptedBeforeUse(t *testing.T) {
	const key = "test-encryption-key"
	const plaintext = "plaintext-access-token"

	enc, err := crypto.Encrypt(plaintext, key)
	require.NoError(t, err)
	require.True(t, crypto.IsEncrypted(enc))

	account := &models.WhatsAppAccount{Name: "acct", AccessToken: enc}
	account.DecryptSecrets(key)

	assert.Equal(t, plaintext, account.ToWAAccount().AccessToken)
}
