package handlers_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/shridarpatil/whatomate/internal/config"
	"github.com/shridarpatil/whatomate/internal/handlers"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// setupWebhookTestApp creates an App with DB and Redis for webhook tests.
// Skips test if TEST_DATABASE_URL or TEST_REDIS_URL is not set.
func setupWebhookTestApp(t *testing.T) (*handlers.App, *gorm.DB, *redis.Client) {
	t.Helper()

	db := testutil.SetupTestDB(t)
	redisClient := testutil.SetupTestRedis(t)
	if redisClient == nil {
		t.Skip("TEST_REDIS_URL not set, skipping Redis test")
	}
	log := testutil.NopLogger()

	app := &handlers.App{
		Config: &config.Config{},
		DB:     db,
		Redis:  redisClient,
		Log:    log,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	return app, db, redisClient
}

// clearWebhookCache clears the webhook cache for a specific organization.
// This ensures test isolation when using shared Redis.
func clearWebhookCache(t *testing.T, redisClient *redis.Client, orgID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	cacheKey := fmt.Sprintf("webhooks:%s", orgID.String())
	redisClient.Del(ctx, cacheKey)
}

func TestApp_WaitForBackgroundTasks(t *testing.T) {
	t.Parallel()

	t.Run("returns immediately when no background tasks", func(t *testing.T) {
		t.Parallel()

		app := &handlers.App{
			Log: testutil.NopLogger(),
		}

		done := make(chan struct{})
		go func() {
			app.WaitForBackgroundTasks()
			close(done)
		}()

		select {
		case <-done:
			// Expected - should return immediately when no background tasks
		case <-time.After(time.Second):
			t.Fatal("WaitForBackgroundTasks should return immediately when no tasks are running")
		}
	})
}

func TestApp_DispatchWebhook_CompletesSuccessfully(t *testing.T) {
	t.Parallel()

	app, db, redisClient := setupWebhookTestApp(t)

	// Create test organization
	org := &models.Organization{
		BaseModel: models.BaseModel{ID: uuid.New()},
		Name:      "test-org-webhook",
		Slug:      "test-org-webhook-" + uuid.New().String()[:8],
	}
	require.NoError(t, db.Create(org).Error)

	// Clear cache and cleanup after test
	clearWebhookCache(t, redisClient, org.ID)
	t.Cleanup(func() { clearWebhookCache(t, redisClient, org.ID) })

	// Create a test server that responds successfully
	var requestCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create webhook
	webhook := &models.Webhook{
		BaseModel:      models.BaseModel{ID: uuid.New()},
		OrganizationID: org.ID,
		Name:           "test-webhook",
		URL:            server.URL,
		Events:         models.StringArray{"message.incoming"},
		IsActive:       true,
	}
	require.NoError(t, db.Create(webhook).Error)

	// Dispatch webhook
	app.DispatchWebhook(org.ID, models.WebhookEventMessageIncoming, map[string]string{"test": "data"})

	// Wait for background tasks
	app.WaitForBackgroundTasks()

	// Verify webhook was called
	assert.Equal(t, int32(1), requestCount.Load(), "webhook should have been called once")
}

func TestApp_DispatchWebhook_ConcurrencyLimit(t *testing.T) {
	t.Parallel()

	app, db, redisClient := setupWebhookTestApp(t)

	// Create test organization
	org := &models.Organization{
		BaseModel: models.BaseModel{ID: uuid.New()},
		Name:      "test-org-concurrency",
		Slug:      "test-org-concurrency-" + uuid.New().String()[:8],
	}
	require.NoError(t, db.Create(org).Error)

	// Clear cache and cleanup after test
	clearWebhookCache(t, redisClient, org.ID)
	t.Cleanup(func() { clearWebhookCache(t, redisClient, org.ID) })

	// Track concurrent requests
	var currentConcurrent atomic.Int32
	var maxConcurrent atomic.Int32
	var totalRequests atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := currentConcurrent.Add(1)
		totalRequests.Add(1)

		// Track max concurrent
		for {
			max := maxConcurrent.Load()
			if current <= max || maxConcurrent.CompareAndSwap(max, current) {
				break
			}
		}

		time.Sleep(50 * time.Millisecond)
		currentConcurrent.Add(-1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create 15 webhooks (more than the max concurrent limit of 10)
	for i := 0; i < 15; i++ {
		webhook := &models.Webhook{
			BaseModel:      models.BaseModel{ID: uuid.New()},
			OrganizationID: org.ID,
			Name:           "test-webhook-" + uuid.New().String()[:8],
			URL:            server.URL,
			Events:         models.StringArray{"message.incoming"},
			IsActive:       true,
		}
		require.NoError(t, db.Create(webhook).Error)
	}

	// Dispatch webhook (should trigger all 15 webhooks)
	app.DispatchWebhook(org.ID, models.WebhookEventMessageIncoming, map[string]string{"test": "data"})

	// Wait for all background tasks
	app.WaitForBackgroundTasks()

	// Verify concurrency was limited to 10
	assert.LessOrEqual(t, maxConcurrent.Load(), int32(10), "max concurrent webhooks should be limited to 10")
	assert.Equal(t, int32(15), totalRequests.Load(), "all 15 webhooks should have been called")
}

func TestApp_DispatchWebhook_NoWebhooks(t *testing.T) {
	t.Parallel()

	app, db, redisClient := setupWebhookTestApp(t)

	// Create test organization with no webhooks
	org := &models.Organization{
		BaseModel: models.BaseModel{ID: uuid.New()},
		Name:      "test-org-no-webhooks",
		Slug:      "test-org-no-webhooks-" + uuid.New().String()[:8],
	}
	require.NoError(t, db.Create(org).Error)

	// Clear cache and cleanup after test
	clearWebhookCache(t, redisClient, org.ID)
	t.Cleanup(func() { clearWebhookCache(t, redisClient, org.ID) })

	// Should not panic when no webhooks exist
	app.DispatchWebhook(org.ID, models.WebhookEventMessageIncoming, map[string]string{"test": "data"})

	// Wait should complete quickly
	done := make(chan struct{})
	go func() {
		app.WaitForBackgroundTasks()
		close(done)
	}()

	select {
	case <-done:
		// Expected
	case <-time.After(2 * time.Second):
		t.Fatal("WaitForBackgroundTasks should complete quickly when no webhooks to dispatch")
	}
}

func TestApp_DispatchWebhook_InactiveWebhook(t *testing.T) {
	t.Parallel()

	app, db, redisClient := setupWebhookTestApp(t)

	// Create test organization
	org := &models.Organization{
		BaseModel: models.BaseModel{ID: uuid.New()},
		Name:      "test-org-inactive",
		Slug:      "test-org-inactive-" + uuid.New().String()[:8],
	}
	require.NoError(t, db.Create(org).Error)

	// Ensure clean state: delete any webhooks and clear cache
	db.Where("organization_id = ?", org.ID).Delete(&models.Webhook{})
	clearWebhookCache(t, redisClient, org.ID)
	t.Cleanup(func() {
		db.Where("organization_id = ?", org.ID).Delete(&models.Webhook{})
		clearWebhookCache(t, redisClient, org.ID)
	})

	var requestCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create webhook (GORM default:true sets IsActive=true)
	webhook := &models.Webhook{
		BaseModel:      models.BaseModel{ID: uuid.New()},
		OrganizationID: org.ID,
		Name:           "test-inactive-webhook",
		URL:            server.URL,
		Events:         models.StringArray{"message.incoming"},
	}
	require.NoError(t, db.Create(webhook).Error)

	// Explicitly set inactive (GORM default:true prevents setting false during create)
	require.NoError(t, db.Model(webhook).Update("is_active", false).Error)

	// Verify the webhook is actually inactive in DB
	var savedWebhook models.Webhook
	require.NoError(t, db.Where("id = ?", webhook.ID).First(&savedWebhook).Error)
	require.False(t, savedWebhook.IsActive, "webhook should be saved as inactive")

	// Clear cache after updating webhook to inactive
	clearWebhookCache(t, redisClient, org.ID)

	app.DispatchWebhook(org.ID, models.WebhookEventMessageIncoming, map[string]string{"test": "data"})
	app.WaitForBackgroundTasks()

	// Inactive webhook should not be called
	assert.Equal(t, int32(0), requestCount.Load(), "inactive webhook should not be called")
}

func TestApp_DispatchWebhook_EventFiltering(t *testing.T) {
	t.Parallel()

	app, db, redisClient := setupWebhookTestApp(t)

	// Create test organization
	org := &models.Organization{
		BaseModel: models.BaseModel{ID: uuid.New()},
		Name:      "test-org-filtering",
		Slug:      "test-org-filtering-" + uuid.New().String()[:8],
	}
	require.NoError(t, db.Create(org).Error)

	// Clear cache and cleanup after test
	clearWebhookCache(t, redisClient, org.ID)
	t.Cleanup(func() { clearWebhookCache(t, redisClient, org.ID) })

	var requestCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create webhook that only listens to message.outgoing
	webhook := &models.Webhook{
		BaseModel:      models.BaseModel{ID: uuid.New()},
		OrganizationID: org.ID,
		Name:           "test-filtering-webhook",
		URL:            server.URL,
		Events:         models.StringArray{"message.outgoing"}, // Not message.incoming
		IsActive:       true,
	}
	require.NoError(t, db.Create(webhook).Error)

	// Dispatch message.incoming event
	app.DispatchWebhook(org.ID, models.WebhookEventMessageIncoming, map[string]string{"test": "data"})
	app.WaitForBackgroundTasks()

	// Webhook should not be called because it doesn't subscribe to message.incoming
	assert.Equal(t, int32(0), requestCount.Load(), "webhook should not be called for non-subscribed events")
}

func TestApp_DispatchWebhook_RetryOnFailure(t *testing.T) {
	t.Parallel()

	app, db, redisClient := setupWebhookTestApp(t)

	// Create test organization
	org := &models.Organization{
		BaseModel: models.BaseModel{ID: uuid.New()},
		Name:      "test-org-retry",
		Slug:      "test-org-retry-" + uuid.New().String()[:8],
	}
	require.NoError(t, db.Create(org).Error)

	// Clear cache and cleanup after test
	clearWebhookCache(t, redisClient, org.ID)
	t.Cleanup(func() { clearWebhookCache(t, redisClient, org.ID) })

	var requestCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)
		if count < 3 {
			// Fail first 2 requests
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	webhook := &models.Webhook{
		BaseModel:      models.BaseModel{ID: uuid.New()},
		OrganizationID: org.ID,
		Name:           "test-retry-webhook",
		URL:            server.URL,
		Events:         models.StringArray{"message.incoming"},
		IsActive:       true,
	}
	require.NoError(t, db.Create(webhook).Error)

	app.DispatchWebhook(org.ID, models.WebhookEventMessageIncoming, map[string]string{"test": "data"})
	app.WaitForBackgroundTasks()

	// Should have retried (3 attempts total)
	assert.Equal(t, int32(3), requestCount.Load(), "should retry on failure")
}

func TestApp_DispatchWebhook_HTTPTimeout(t *testing.T) {
	t.Parallel()

	app, db, redisClient := setupWebhookTestApp(t)

	// Create test organization
	org := &models.Organization{
		BaseModel: models.BaseModel{ID: uuid.New()},
		Name:      "test-org-timeout",
		Slug:      "test-org-timeout-" + uuid.New().String()[:8],
	}
	require.NoError(t, db.Create(org).Error)

	// Clear cache and cleanup after test
	clearWebhookCache(t, redisClient, org.ID)
	t.Cleanup(func() { clearWebhookCache(t, redisClient, org.ID) })

	var requestStarted atomic.Bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestStarted.Store(true)
		// Delay longer than HTTP client timeout (10s)
		select {
		case <-r.Context().Done():
			// Request was cancelled
			return
		case <-time.After(15 * time.Second):
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	webhook := &models.Webhook{
		BaseModel:      models.BaseModel{ID: uuid.New()},
		OrganizationID: org.ID,
		Name:           "test-timeout-webhook",
		URL:            server.URL,
		Events:         models.StringArray{"message.incoming"},
		IsActive:       true,
	}
	require.NoError(t, db.Create(webhook).Error)

	app.DispatchWebhook(org.ID, models.WebhookEventMessageIncoming, map[string]string{"test": "data"})

	// Wait with timeout (the HTTP client has 10s timeout, with 3 retries = ~30s max)
	done := make(chan struct{})
	go func() {
		app.WaitForBackgroundTasks()
		close(done)
	}()

	select {
	case <-done:
		// Expected - should complete due to HTTP timeout after retries
	case <-time.After(2 * time.Minute):
		t.Fatal("dispatch should timeout and complete within reasonable time")
	}

	assert.True(t, requestStarted.Load(), "request should have started")
}

func TestApp_DispatchWebhook_MultipleEvents(t *testing.T) {
	t.Parallel()

	app, db, redisClient := setupWebhookTestApp(t)

	// Create test organization
	org := &models.Organization{
		BaseModel: models.BaseModel{ID: uuid.New()},
		Name:      "test-org-multi-events",
		Slug:      "test-org-multi-events-" + uuid.New().String()[:8],
	}
	require.NoError(t, db.Create(org).Error)

	// Clear cache and cleanup after test
	clearWebhookCache(t, redisClient, org.ID)
	t.Cleanup(func() { clearWebhookCache(t, redisClient, org.ID) })

	var incomingCount atomic.Int32
	var outgoingCount atomic.Int32

	// Create a separate counter server for each event type
	incomingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		incomingCount.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer incomingServer.Close()

	outgoingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		outgoingCount.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer outgoingServer.Close()

	// Create webhooks for different events
	webhook1 := &models.Webhook{
		BaseModel:      models.BaseModel{ID: uuid.New()},
		OrganizationID: org.ID,
		Name:           "test-incoming-webhook",
		URL:            incomingServer.URL,
		Events:         models.StringArray{"message.incoming"},
		IsActive:       true,
	}
	require.NoError(t, db.Create(webhook1).Error)

	webhook2 := &models.Webhook{
		BaseModel:      models.BaseModel{ID: uuid.New()},
		OrganizationID: org.ID,
		Name:           "test-outgoing-webhook",
		URL:            outgoingServer.URL,
		Events:         models.StringArray{"message.outgoing"},
		IsActive:       true,
	}
	require.NoError(t, db.Create(webhook2).Error)

	// Dispatch incoming event
	app.DispatchWebhook(org.ID, models.WebhookEventMessageIncoming, map[string]string{"test": "incoming"})
	app.WaitForBackgroundTasks()

	// Dispatch outgoing event
	app.DispatchWebhook(org.ID, models.WebhookEventMessageOutgoing, map[string]string{"test": "outgoing"})
	app.WaitForBackgroundTasks()

	// Verify each webhook was called for its event
	assert.Equal(t, int32(1), incomingCount.Load(), "incoming webhook should be called once")
	assert.Equal(t, int32(1), outgoingCount.Load(), "outgoing webhook should be called once")
}
