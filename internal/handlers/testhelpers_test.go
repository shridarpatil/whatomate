package handlers_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/shridarpatil/whatomate/internal/config"
	"github.com/shridarpatil/whatomate/internal/handlers"
	"github.com/shridarpatil/whatomate/internal/queue"
	"github.com/shridarpatil/whatomate/pkg/whatsapp"
	"github.com/shridarpatil/whatomate/test/testutil"
)

// appOption configures an App for testing.
type appOption func(*handlers.App)

// withQueue sets the queue on the test App.
func withQueue(q queue.Queue) appOption {
	return func(a *handlers.App) {
		a.Queue = q
	}
}

// withWhatsApp sets the WhatsApp client on the test App.
func withWhatsApp(wa *whatsapp.Client) appOption {
	return func(a *handlers.App) {
		a.WhatsApp = wa
	}
}

// withHTTPClient sets the HTTP client on the test App.
func withHTTPClient(client *http.Client) appOption {
	return func(a *handlers.App) {
		a.HTTPClient = client
	}
}

// newTestApp creates an App instance for testing with a test database, Redis, and default config.
// Skips the test if TEST_REDIS_URL is not set.
func newTestApp(t *testing.T, opts ...appOption) *handlers.App {
	t.Helper()

	db := testutil.SetupTestDB(t)
	log := testutil.NopLogger()

	redisClient := testutil.SetupTestRedis(t)
	if redisClient == nil {
		t.Skip("TEST_REDIS_URL not set, skipping test")
	}

	cfg := &config.Config{
		JWT: config.JWTConfig{
			Secret:            testutil.TestJWTSecret,
			AccessExpiryMins:  15,
			RefreshExpiryDays: 7,
		},
	}

	app := &handlers.App{
		Config: cfg,
		DB:     db,
		Log:    log,
		Redis:  redisClient,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	for _, opt := range opts {
		opt(app)
	}

	return app
}
