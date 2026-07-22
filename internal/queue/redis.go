package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/zerodha/logf"
)

const (
	// StreamName is the Redis stream for campaign jobs
	StreamName = "whatomate:campaigns"

	// ConsumerGroup is the consumer group name for workers
	ConsumerGroup = "campaign-workers"

	// BlockTimeout is how long to block waiting for new messages
	BlockTimeout = 5 * time.Second

	// ClaimMinIdleTime is the minimum idle time before claiming a pending message
	ClaimMinIdleTime = 5 * time.Minute
)

// RedisQueue implements the Queue interface using Redis Streams
type RedisQueue struct {
	client *redis.Client
	log    logf.Logger
}

// NewRedisQueue creates a new Redis queue
func NewRedisQueue(client *redis.Client, log logf.Logger) *RedisQueue {
	return &RedisQueue{
		client: client,
		log:    log,
	}
}

// EnqueueRecipient adds a single recipient job to the queue
func (q *RedisQueue) EnqueueRecipient(ctx context.Context, job *RecipientJob) error {
	if job.EnqueuedAt.IsZero() {
		job.EnqueuedAt = time.Now()
	}

	payload, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to marshal recipient job: %w", err)
	}

	_, err = q.client.XAdd(ctx, &redis.XAddArgs{
		Stream: StreamName,
		Values: map[string]any{
			"type":    string(JobTypeRecipient),
			"payload": string(payload),
		},
	}).Result()

	if err != nil {
		return fmt.Errorf("failed to enqueue recipient job: %w", err)
	}

	return nil
}

// EnqueueRecipients adds multiple recipient jobs to the queue using pipeline
func (q *RedisQueue) EnqueueRecipients(ctx context.Context, jobs []*RecipientJob) error {
	if len(jobs) == 0 {
		return nil
	}

	pipe := q.client.Pipeline()
	now := time.Now()

	for _, job := range jobs {
		if job.EnqueuedAt.IsZero() {
			job.EnqueuedAt = now
		}

		payload, err := json.Marshal(job)
		if err != nil {
			return fmt.Errorf("failed to marshal recipient job: %w", err)
		}

		pipe.XAdd(ctx, &redis.XAddArgs{
			Stream: StreamName,
			Values: map[string]any{
				"type":    string(JobTypeRecipient),
				"payload": string(payload),
			},
		})
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to enqueue recipient jobs: %w", err)
	}

	q.log.Info("Recipient jobs enqueued", "count", len(jobs), "campaign_id", jobs[0].CampaignID)
	return nil
}

// Close closes the queue connection
func (q *RedisQueue) Close() error {
	return nil // Redis client is managed externally
}

// RedisConsumer implements the Consumer interface using Redis Streams
type RedisConsumer struct {
	client     *redis.Client
	log        logf.Logger
	consumerID string
}

// NewRedisConsumer creates a new Redis consumer
func NewRedisConsumer(client *redis.Client, log logf.Logger) (*RedisConsumer, error) {
	// Generate unique consumer ID
	hostname, _ := os.Hostname()
	consumerID := fmt.Sprintf("worker-%s-%d", hostname, os.Getpid())

	consumer := &RedisConsumer{
		client:     client,
		log:        log,
		consumerID: consumerID,
	}

	// Create consumer group if it doesn't exist
	ctx := context.Background()
	err := client.XGroupCreateMkStream(ctx, StreamName, ConsumerGroup, "0").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		return nil, fmt.Errorf("failed to create consumer group: %w", err)
	}

	log.Info("Redis consumer initialized", "consumer_id", consumerID)
	return consumer, nil
}

// Consume starts consuming jobs from the queue
func (c *RedisConsumer) Consume(ctx context.Context, handler JobHandler) error {
	c.log.Info("Starting to consume jobs", "consumer_id", c.consumerID)

	// First, try to claim any stale pending messages from crashed workers
	if err := c.claimPendingMessages(ctx, handler); err != nil {
		c.log.Warn("Failed to claim pending messages", "error", err)
	}

	for {
		select {
		case <-ctx.Done():
			c.log.Info("Consumer shutting down")
			return ctx.Err()
		default:
		}

		// Read new messages from the stream
		streams, err := c.client.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    ConsumerGroup,
			Consumer: c.consumerID,
			Streams:  []string{StreamName, ">"},
			Count:    1,
			Block:    BlockTimeout,
		}).Result()

		if err != nil {
			if err == redis.Nil {
				// No messages available, continue waiting
				continue
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			c.log.Error("Failed to read from stream", "error", err)
			time.Sleep(time.Second) // Back off on error
			continue
		}

		for _, stream := range streams {
			for _, msg := range stream.Messages {
				if err := c.processMessage(ctx, msg, handler); err != nil {
					c.log.Error("Failed to process message", "error", err, "message_id", msg.ID)
					// Don't ACK failed messages - they'll be reclaimed later
					continue
				}

				// Acknowledge the message
				if err := c.client.XAck(ctx, StreamName, ConsumerGroup, msg.ID).Err(); err != nil {
					c.log.Error("Failed to ACK message", "error", err, "message_id", msg.ID)
				}
			}
		}
	}
}

// claimPendingMessages claims stale pending messages from crashed workers
func (c *RedisConsumer) claimPendingMessages(ctx context.Context, handler JobHandler) error {
	// Get pending messages that have been idle for too long
	pending, err := c.client.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: StreamName,
		Group:  ConsumerGroup,
		Start:  "-",
		End:    "+",
		Count:  100,
		Idle:   ClaimMinIdleTime,
	}).Result()

	if err != nil {
		return fmt.Errorf("failed to get pending messages: %w", err)
	}

	if len(pending) == 0 {
		return nil
	}

	c.log.Info("Found stale pending messages to claim", "count", len(pending))

	// Claim and process each pending message
	for _, p := range pending {
		// Claim the message
		messages, err := c.client.XClaim(ctx, &redis.XClaimArgs{
			Stream:   StreamName,
			Group:    ConsumerGroup,
			Consumer: c.consumerID,
			MinIdle:  ClaimMinIdleTime,
			Messages: []string{p.ID},
		}).Result()

		if err != nil {
			c.log.Error("Failed to claim message", "error", err, "message_id", p.ID)
			continue
		}

		for _, msg := range messages {
			if err := c.processMessage(ctx, msg, handler); err != nil {
				c.log.Error("Failed to process claimed message", "error", err, "message_id", msg.ID)
				continue
			}

			// Acknowledge the message
			if err := c.client.XAck(ctx, StreamName, ConsumerGroup, msg.ID).Err(); err != nil {
				c.log.Error("Failed to ACK claimed message", "error", err, "message_id", msg.ID)
			}
		}
	}

	return nil
}

// processMessage processes a single message from the stream
func (c *RedisConsumer) processMessage(ctx context.Context, msg redis.XMessage, handler JobHandler) error {
	jobType, ok := msg.Values["type"].(string)
	if !ok {
		return fmt.Errorf("invalid message: missing type")
	}

	payload, ok := msg.Values["payload"].(string)
	if !ok {
		return fmt.Errorf("invalid message: missing payload")
	}

	switch JobType(jobType) {
	case JobTypeRecipient:
		var job RecipientJob
		if err := json.Unmarshal([]byte(payload), &job); err != nil {
			return fmt.Errorf("failed to unmarshal recipient job: %w", err)
		}
		c.log.Debug("Processing recipient job", "campaign_id", job.CampaignID, "recipient_id", job.RecipientID, "message_id", msg.ID)
		return handler.HandleRecipientJob(ctx, &job)

	default:
		return fmt.Errorf("unknown job type: %s", jobType)
	}
}

// Close closes the consumer connection
func (c *RedisConsumer) Close() error {
	return nil // Redis client is managed externally
}
