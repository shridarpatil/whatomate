package handlers_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/shridarpatil/whatomate/internal/config"
	"github.com/shridarpatil/whatomate/internal/handlers"
	"github.com/shridarpatil/whatomate/internal/queue"
	"github.com/shridarpatil/whatomate/internal/websocket"
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

// withRedis sets the Redis client on the test App.
func withRedis(rdb *redis.Client) appOption {
	return func(a *handlers.App) {
		a.Redis = rdb
	}
}

// withWhatsApp sets the WhatsApp client on the test App.
func withWhatsApp(wa *whatsapp.Client) appOption {
	return func(a *handlers.App) {
		a.WhatsApp = wa
	}
}

// withWSHub sets the WebSocket hub on the test App.
func withWSHub(hub *websocket.Hub) appOption {
	return func(a *handlers.App) {
		a.WSHub = hub
	}
}

// withHTTPClient sets the HTTP client on the test App.
func withHTTPClient(client *http.Client) appOption {
	return func(a *handlers.App) {
		a.HTTPClient = client
	}
}

// newTestApp creates an App instance for testing with a test database and default config.
func newTestApp(t *testing.T, opts ...appOption) *handlers.App {
	t.Helper()

	db := testutil.SetupTestDB(t)
	log := testutil.NopLogger()

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
	}

	for _, opt := range opts {
		opt(app)
	}

	return app
}

// newTestAppWithRedis creates an App with DB and Redis.
// Skips the test if TEST_REDIS_URL is not set.
func newTestAppWithRedis(t *testing.T, opts ...appOption) *handlers.App {
	t.Helper()

	redisClient := testutil.SetupTestRedis(t)
	if redisClient == nil {
		t.Skip("TEST_REDIS_URL not set, skipping Redis test")
	}

	allOpts := append([]appOption{withRedis(redisClient)}, opts...)
	app := newTestApp(t, allOpts...)

	// Add HTTP client with timeout if not already set
	if app.HTTPClient == nil {
		app.HTTPClient = &http.Client{
			Timeout: 30 * time.Second,
		}
	}

	return app
}
