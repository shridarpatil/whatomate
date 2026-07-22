package queue_test

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/shridarpatil/whatomate/internal/queue"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pendingCount returns how many messages are currently pending (delivered but not ACKed)
// in the consumer group.
func pendingCount(t *testing.T, client *redis.Client) int64 {
	t.Helper()
	res, err := client.XPending(context.Background(), queue.StreamName, queue.ConsumerGroup).Result()
	if err != nil {
		// Group may not exist yet — treat as zero.
		return 0
	}
	return res.Count
}

func TestConsume_HandlerErrorLeavesMessagePending(t *testing.T) {
	client := skipIfNoRedis(t)
	cleanStream(t, client)
	log := testutil.NopLogger()
	ctx := testutil.TestContextWithTimeout(t, 10*time.Second)

	q := queue.NewRedisQueue(client, log)
	require.NoError(t, q.EnqueueRecipient(ctx, makeRecipientJob()))

	consumer, err := queue.NewRedisConsumer(client, log)
	require.NoError(t, err)
	defer func() { _ = consumer.Close() }()

	handler := &mockHandler{err: assert.AnError} // returns error every time

	consumeCtx, cancel := context.WithCancel(ctx)
	go func() { _ = consumer.Consume(consumeCtx, handler) }()

	// Wait for the handler to attempt the job.
	testutil.AssertEventually(t, func() bool {
		return len(handler.getJobs()) >= 1
	}, 8*time.Second, "handler should have been invoked at least once")

	cancel()
	// Give the consumer a moment to exit.
	time.Sleep(200 * time.Millisecond)

	// Failed processing must NOT ACK the message — it stays pending so a fresh
	// worker can claim it later via the XPending recovery loop.
	assert.GreaterOrEqual(t, pendingCount(t, client), int64(1),
		"handler error must leave the message pending (un-ACKed)")
}

func TestConsume_MalformedMessage_MissingType(t *testing.T) {
	client := skipIfNoRedis(t)
	cleanStream(t, client)
	log := testutil.NopLogger()
	ctx := testutil.TestContextWithTimeout(t, 10*time.Second)

	// Add a message directly with no "type" field.
	_, err := client.XAdd(ctx, &redis.XAddArgs{
		Stream: queue.StreamName,
		Values: map[string]any{"payload": "{}"},
	}).Result()
	require.NoError(t, err)

	consumer, err := queue.NewRedisConsumer(client, log)
	require.NoError(t, err)
	defer func() { _ = consumer.Close() }()

	handler := &mockHandler{}
	consumeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	_ = consumer.Consume(consumeCtx, handler)

	assert.Empty(t, handler.getJobs(), "malformed message must not invoke the handler")
	assert.GreaterOrEqual(t, pendingCount(t, client), int64(1),
		"malformed message must remain pending so it can be reviewed manually")
}

func TestConsume_UnknownJobType(t *testing.T) {
	client := skipIfNoRedis(t)
	cleanStream(t, client)
	log := testutil.NopLogger()
	ctx := testutil.TestContextWithTimeout(t, 10*time.Second)

	_, err := client.XAdd(ctx, &redis.XAddArgs{
		Stream: queue.StreamName,
		Values: map[string]any{"type": "future-job-type", "payload": "{}"},
	}).Result()
	require.NoError(t, err)

	consumer, err := queue.NewRedisConsumer(client, log)
	require.NoError(t, err)
	defer func() { _ = consumer.Close() }()

	handler := &mockHandler{}
	consumeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	_ = consumer.Consume(consumeCtx, handler)

	assert.Empty(t, handler.getJobs())
	// Unknown type should also leave the message pending (not ACKed) so it doesn't get silently dropped.
	assert.GreaterOrEqual(t, pendingCount(t, client), int64(1))
}

func TestConsume_MalformedPayloadJSON(t *testing.T) {
	client := skipIfNoRedis(t)
	cleanStream(t, client)
	log := testutil.NopLogger()
	ctx := testutil.TestContextWithTimeout(t, 10*time.Second)

	_, err := client.XAdd(ctx, &redis.XAddArgs{
		Stream: queue.StreamName,
		Values: map[string]any{"type": "recipient", "payload": "not valid json"},
	}).Result()
	require.NoError(t, err)

	consumer, err := queue.NewRedisConsumer(client, log)
	require.NoError(t, err)
	defer func() { _ = consumer.Close() }()

	handler := &mockHandler{}
	consumeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	_ = consumer.Consume(consumeCtx, handler)

	assert.Empty(t, handler.getJobs())
	assert.GreaterOrEqual(t, pendingCount(t, client), int64(1))
}

func TestConsume_SuccessfulJobIsAckedAndCleared(t *testing.T) {
	client := skipIfNoRedis(t)
	cleanStream(t, client)
	log := testutil.NopLogger()
	ctx := testutil.TestContextWithTimeout(t, 10*time.Second)

	q := queue.NewRedisQueue(client, log)
	require.NoError(t, q.EnqueueRecipient(ctx, makeRecipientJob()))

	consumer, err := queue.NewRedisConsumer(client, log)
	require.NoError(t, err)
	defer func() { _ = consumer.Close() }()

	handler := &mockHandler{}
	consumeCtx, cancel := context.WithCancel(ctx)
	go func() { _ = consumer.Consume(consumeCtx, handler) }()

	testutil.AssertEventually(t, func() bool {
		return len(handler.getJobs()) >= 1
	}, 8*time.Second, "handler should be invoked")

	// Give it a moment to ACK after the handler returns success.
	time.Sleep(200 * time.Millisecond)
	cancel()

	assert.Equal(t, int64(0), pendingCount(t, client),
		"successful jobs must be ACKed and cleared from pending")
}
