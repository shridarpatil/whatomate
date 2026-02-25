package queue_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/internal/queue"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// skipIfNoRedis returns a Redis client or skips the test if Redis is unavailable.
func skipIfNoRedis(t *testing.T) *redis.Client {
	t.Helper()
	client := testutil.SetupTestRedis(t)
	if client == nil {
		t.Skip("Redis not available, skipping test")
	}
	return client
}

// cleanStream deletes the Redis stream used by tests so each test starts fresh.
func cleanStream(t *testing.T, client *redis.Client) {
	t.Helper()
	ctx := context.Background()
	client.Del(ctx, queue.StreamName)
	t.Cleanup(func() {
		client.Del(ctx, queue.StreamName)
		// Also clean up the consumer group; ignore errors if it doesn't exist.
		client.XGroupDestroy(ctx, queue.StreamName, queue.ConsumerGroup)
	})
}

// makeRecipientJob creates a RecipientJob with random IDs for testing.
func makeRecipientJob() *queue.RecipientJob {
	return &queue.RecipientJob{
		CampaignID:     uuid.New(),
		RecipientID:    uuid.New(),
		OrganizationID: uuid.New(),
		PhoneNumber:    "1234567890",
		RecipientName:  "Test User",
		TemplateParams: models.JSONB{"1": "Hello", "2": "World"},
	}
}

// mockHandler implements queue.JobHandler for testing.
type mockHandler struct {
	mu   sync.Mutex
	jobs []*queue.RecipientJob
	err  error // if set, HandleRecipientJob returns this error
}

func (h *mockHandler) HandleRecipientJob(_ context.Context, job *queue.RecipientJob) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.jobs = append(h.jobs, job)
	return h.err
}

func (h *mockHandler) getJobs() []*queue.RecipientJob {
	h.mu.Lock()
	defer h.mu.Unlock()
	dst := make([]*queue.RecipientJob, len(h.jobs))
	copy(dst, h.jobs)
	return dst
}

// --- NewRedisQueue tests ---

func TestNewRedisQueue(t *testing.T) {
	t.Parallel()
	client := skipIfNoRedis(t)
	log := testutil.NopLogger()

	q := queue.NewRedisQueue(client, log)
	require.NotNil(t, q)

	err := q.Close()
	assert.NoError(t, err)
}

// --- EnqueueRecipient tests ---

func TestEnqueueRecipient_Single(t *testing.T) {
	client := skipIfNoRedis(t)
	cleanStream(t, client)
	log := testutil.NopLogger()
	ctx := testutil.TestContext(t)

	q := queue.NewRedisQueue(client, log)
	job := makeRecipientJob()

	err := q.EnqueueRecipient(ctx, job)
	require.NoError(t, err)

	// Verify the job landed in the stream.
	msgs, err := client.XRange(ctx, queue.StreamName, "-", "+").Result()
	require.NoError(t, err)
	require.Len(t, msgs, 1)

	assert.Equal(t, string(queue.JobTypeRecipient), msgs[0].Values["type"])

	var decoded queue.RecipientJob
	err = json.Unmarshal([]byte(msgs[0].Values["payload"].(string)), &decoded)
	require.NoError(t, err)
	assert.Equal(t, job.CampaignID, decoded.CampaignID)
	assert.Equal(t, job.RecipientID, decoded.RecipientID)
	assert.Equal(t, job.PhoneNumber, decoded.PhoneNumber)
}

func TestEnqueueRecipient_SetsEnqueuedAt(t *testing.T) {
	client := skipIfNoRedis(t)
	cleanStream(t, client)
	log := testutil.NopLogger()
	ctx := testutil.TestContext(t)

	q := queue.NewRedisQueue(client, log)
	job := makeRecipientJob()
	// Leave EnqueuedAt as zero so the queue sets it.
	assert.True(t, job.EnqueuedAt.IsZero())

	err := q.EnqueueRecipient(ctx, job)
	require.NoError(t, err)

	// The job should now have a non-zero EnqueuedAt.
	assert.False(t, job.EnqueuedAt.IsZero())
}

func TestEnqueueRecipient_PreservesExistingEnqueuedAt(t *testing.T) {
	client := skipIfNoRedis(t)
	cleanStream(t, client)
	log := testutil.NopLogger()
	ctx := testutil.TestContext(t)

	q := queue.NewRedisQueue(client, log)
	job := makeRecipientJob()
	fixedTime := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	job.EnqueuedAt = fixedTime

	err := q.EnqueueRecipient(ctx, job)
	require.NoError(t, err)

	// EnqueuedAt should remain unchanged.
	assert.Equal(t, fixedTime, job.EnqueuedAt)

	// Verify in Redis payload as well.
	msgs, err := client.XRange(ctx, queue.StreamName, "-", "+").Result()
	require.NoError(t, err)
	require.Len(t, msgs, 1)

	var decoded queue.RecipientJob
	err = json.Unmarshal([]byte(msgs[0].Values["payload"].(string)), &decoded)
	require.NoError(t, err)
	assert.True(t, fixedTime.Equal(decoded.EnqueuedAt))
}

// --- EnqueueRecipients (batch) tests ---

func TestEnqueueRecipients_Batch(t *testing.T) {
	client := skipIfNoRedis(t)
	cleanStream(t, client)
	log := testutil.NopLogger()
	ctx := testutil.TestContext(t)

	q := queue.NewRedisQueue(client, log)

	jobs := make([]*queue.RecipientJob, 5)
	for i := range jobs {
		jobs[i] = makeRecipientJob()
	}

	err := q.EnqueueRecipients(ctx, jobs)
	require.NoError(t, err)

	// All 5 jobs should be in the stream.
	msgs, err := client.XRange(ctx, queue.StreamName, "-", "+").Result()
	require.NoError(t, err)
	assert.Len(t, msgs, 5)

	// Verify each message has the correct type.
	for _, msg := range msgs {
		assert.Equal(t, string(queue.JobTypeRecipient), msg.Values["type"])
	}
}

func TestEnqueueRecipients_Empty(t *testing.T) {
	client := skipIfNoRedis(t)
	cleanStream(t, client)
	log := testutil.NopLogger()
	ctx := testutil.TestContext(t)

	q := queue.NewRedisQueue(client, log)

	// Enqueuing an empty slice should be a no-op.
	err := q.EnqueueRecipients(ctx, []*queue.RecipientJob{})
	require.NoError(t, err)

	msgs, err := client.XRange(ctx, queue.StreamName, "-", "+").Result()
	require.NoError(t, err)
	assert.Empty(t, msgs)
}

func TestEnqueueRecipients_SetsEnqueuedAt(t *testing.T) {
	client := skipIfNoRedis(t)
	cleanStream(t, client)
	log := testutil.NopLogger()
	ctx := testutil.TestContext(t)

	q := queue.NewRedisQueue(client, log)

	jobs := []*queue.RecipientJob{makeRecipientJob(), makeRecipientJob()}
	for _, j := range jobs {
		assert.True(t, j.EnqueuedAt.IsZero())
	}

	err := q.EnqueueRecipients(ctx, jobs)
	require.NoError(t, err)

	// All jobs should now have EnqueuedAt set.
	for _, j := range jobs {
		assert.False(t, j.EnqueuedAt.IsZero())
	}
}

// --- Consumer tests ---

func TestNewRedisConsumer(t *testing.T) {
	client := skipIfNoRedis(t)
	cleanStream(t, client)
	log := testutil.NopLogger()

	consumer, err := queue.NewRedisConsumer(client, log)
	require.NoError(t, err)
	require.NotNil(t, consumer)

	err = consumer.Close()
	assert.NoError(t, err)
}

func TestConsume_ProcessesJob(t *testing.T) {
	client := skipIfNoRedis(t)
	cleanStream(t, client)
	log := testutil.NopLogger()
	ctx := testutil.TestContextWithTimeout(t, 10*time.Second)

	q := queue.NewRedisQueue(client, log)

	// Enqueue a job first.
	job := makeRecipientJob()
	err := q.EnqueueRecipient(ctx, job)
	require.NoError(t, err)

	// Create consumer.
	consumer, err := queue.NewRedisConsumer(client, log)
	require.NoError(t, err)
	defer consumer.Close() //nolint:errcheck

	handler := &mockHandler{}

	// Run the consumer in a goroutine with a cancellable context.
	consumeCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		_ = consumer.Consume(consumeCtx, handler)
	}()

	// Wait for the handler to receive the job.
	testutil.AssertEventually(t, func() bool {
		return len(handler.getJobs()) >= 1
	}, 8*time.Second, "handler should have received at least 1 job")

	cancel()

	received := handler.getJobs()
	require.Len(t, received, 1)
	assert.Equal(t, job.CampaignID, received[0].CampaignID)
	assert.Equal(t, job.RecipientID, received[0].RecipientID)
	assert.Equal(t, job.PhoneNumber, received[0].PhoneNumber)
	assert.Equal(t, job.RecipientName, received[0].RecipientName)
}

func TestConsume_EmptyQueue(t *testing.T) {
	client := skipIfNoRedis(t)
	cleanStream(t, client)
	log := testutil.NopLogger()
	ctx := testutil.TestContextWithTimeout(t, 10*time.Second)

	consumer, err := queue.NewRedisConsumer(client, log)
	require.NoError(t, err)
	defer consumer.Close() //nolint:errcheck

	handler := &mockHandler{}

	// Cancel quickly -- the consumer should handle an empty queue gracefully.
	consumeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	err = consumer.Consume(consumeCtx, handler)
	// Should return context error (deadline exceeded or cancelled), not a crash.
	assert.ErrorIs(t, err, context.DeadlineExceeded)

	// No jobs should have been processed.
	assert.Empty(t, handler.getJobs())
}

func TestConsume_MultipleJobs(t *testing.T) {
	client := skipIfNoRedis(t)
	cleanStream(t, client)
	log := testutil.NopLogger()
	ctx := testutil.TestContextWithTimeout(t, 15*time.Second)

	q := queue.NewRedisQueue(client, log)

	// Enqueue 3 jobs.
	jobs := make([]*queue.RecipientJob, 3)
	for i := range jobs {
		jobs[i] = makeRecipientJob()
	}
	err := q.EnqueueRecipients(ctx, jobs)
	require.NoError(t, err)

	consumer, err := queue.NewRedisConsumer(client, log)
	require.NoError(t, err)
	defer consumer.Close() //nolint:errcheck

	handler := &mockHandler{}

	consumeCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		_ = consumer.Consume(consumeCtx, handler)
	}()

	testutil.AssertEventually(t, func() bool {
		return len(handler.getJobs()) >= 3
	}, 12*time.Second, "handler should have received all 3 jobs")

	cancel()

	received := handler.getJobs()
	assert.Len(t, received, 3)

	// Verify all campaign IDs were received (order may vary).
	receivedIDs := make(map[uuid.UUID]bool)
	for _, r := range received {
		receivedIDs[r.CampaignID] = true
	}
	for _, j := range jobs {
		assert.True(t, receivedIDs[j.CampaignID], "expected campaign ID %s to be received", j.CampaignID)
	}
}

// --- Pub/Sub tests ---

func TestPublishCampaignStats(t *testing.T) {
	t.Parallel()
	client := skipIfNoRedis(t)
	log := testutil.NopLogger()
	ctx := testutil.TestContext(t)

	pub := queue.NewPublisher(client, log)

	update := &queue.CampaignStatsUpdate{
		CampaignID:     uuid.New().String(),
		OrganizationID: uuid.New(),
		Status:         models.CampaignStatusProcessing,
		SentCount:      10,
		DeliveredCount: 8,
		ReadCount:      5,
		FailedCount:    2,
	}

	// Publishing without any subscriber should not error.
	err := pub.PublishCampaignStats(ctx, update)
	assert.NoError(t, err)
}

func TestSubscribeCampaignStats_ReceivesUpdate(t *testing.T) {
	t.Parallel()
	client := skipIfNoRedis(t)
	log := testutil.NopLogger()
	ctx := testutil.TestContextWithTimeout(t, 10*time.Second)

	pub := queue.NewPublisher(client, log)
	sub := queue.NewSubscriber(client, log)
	defer sub.Close() //nolint:errcheck

	// Use a unique campaign ID to filter out messages from parallel tests
	// sharing the same pub/sub channel.
	targetCampaignID := uuid.New().String()

	var mu sync.Mutex
	var matched *queue.CampaignStatsUpdate

	subCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	err := sub.SubscribeCampaignStats(subCtx, func(update *queue.CampaignStatsUpdate) {
		mu.Lock()
		defer mu.Unlock()
		if update.CampaignID == targetCampaignID {
			matched = update
		}
	})
	require.NoError(t, err)

	// Give the subscriber a moment to fully establish.
	time.Sleep(100 * time.Millisecond)

	update := &queue.CampaignStatsUpdate{
		CampaignID:     targetCampaignID,
		OrganizationID: uuid.New(),
		Status:         models.CampaignStatusCompleted,
		SentCount:      50,
		DeliveredCount: 45,
		ReadCount:      30,
		FailedCount:    5,
	}

	err = pub.PublishCampaignStats(ctx, update)
	require.NoError(t, err)

	testutil.AssertEventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return matched != nil
	}, 5*time.Second, "subscriber should have received the stats update")

	mu.Lock()
	defer mu.Unlock()
	require.NotNil(t, matched)
	assert.Equal(t, update.CampaignID, matched.CampaignID)
	assert.Equal(t, update.OrganizationID, matched.OrganizationID)
	assert.Equal(t, update.Status, matched.Status)
	assert.Equal(t, update.SentCount, matched.SentCount)
	assert.Equal(t, update.DeliveredCount, matched.DeliveredCount)
	assert.Equal(t, update.ReadCount, matched.ReadCount)
	assert.Equal(t, update.FailedCount, matched.FailedCount)
}

func TestSubscriber_Close(t *testing.T) {
	t.Parallel()
	client := skipIfNoRedis(t)
	log := testutil.NopLogger()

	sub := queue.NewSubscriber(client, log)

	// Close before subscribing should not error.
	err := sub.Close()
	assert.NoError(t, err)
}

// --- Error handling tests ---

func TestEnqueueRecipient_InvalidRedis(t *testing.T) {
	t.Parallel()
	log := testutil.NopLogger()

	// Create a client pointing to an invalid address.
	badClient := redis.NewClient(&redis.Options{
		Addr:        "localhost:1", // Invalid port
		DialTimeout: 100 * time.Millisecond,
	})
	defer badClient.Close() //nolint:errcheck

	q := queue.NewRedisQueue(badClient, log)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	job := makeRecipientJob()
	err := q.EnqueueRecipient(ctx, job)
	assert.Error(t, err)
}

func TestEnqueueRecipients_InvalidRedis(t *testing.T) {
	t.Parallel()
	log := testutil.NopLogger()

	badClient := redis.NewClient(&redis.Options{
		Addr:        "localhost:1",
		DialTimeout: 100 * time.Millisecond,
	})
	defer badClient.Close() //nolint:errcheck

	q := queue.NewRedisQueue(badClient, log)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	jobs := []*queue.RecipientJob{makeRecipientJob()}
	err := q.EnqueueRecipients(ctx, jobs)
	assert.Error(t, err)
}

func TestNewRedisConsumer_InvalidRedis(t *testing.T) {
	t.Parallel()
	log := testutil.NopLogger()

	badClient := redis.NewClient(&redis.Options{
		Addr:        "localhost:1",
		DialTimeout: 100 * time.Millisecond,
	})
	defer badClient.Close() //nolint:errcheck

	_, err := queue.NewRedisConsumer(badClient, log)
	assert.Error(t, err)
}

func TestPublishCampaignStats_InvalidRedis(t *testing.T) {
	t.Parallel()
	log := testutil.NopLogger()

	badClient := redis.NewClient(&redis.Options{
		Addr:        "localhost:1",
		DialTimeout: 100 * time.Millisecond,
	})
	defer badClient.Close() //nolint:errcheck

	pub := queue.NewPublisher(badClient, log)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	update := &queue.CampaignStatsUpdate{
		CampaignID: uuid.New().String(),
		Status:     models.CampaignStatusProcessing,
	}
	err := pub.PublishCampaignStats(ctx, update)
	assert.Error(t, err)
}
