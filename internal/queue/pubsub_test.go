package queue_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/internal/queue"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPublisher_PublishCampaignStats_NoSubscribersIsNotAnError(t *testing.T) {
	rdb := testutil.SetupTestRedis(t)
	if rdb == nil {
		t.Skip("TEST_REDIS_URL not set")
	}
	pub := queue.NewPublisher(rdb, testutil.NopLogger())

	err := pub.PublishCampaignStats(context.Background(), &queue.CampaignStatsUpdate{
		CampaignID:     "abc",
		OrganizationID: uuid.New(),
		Status:         models.CampaignStatusProcessing,
		SentCount:      5,
	})
	require.NoError(t, err, "publishing without subscribers must not error")
}

func TestSubscriber_ReceivesPublishedUpdate(t *testing.T) {
	rdb := testutil.SetupTestRedis(t)
	if rdb == nil {
		t.Skip("TEST_REDIS_URL not set")
	}
	pub := queue.NewPublisher(rdb, testutil.NopLogger())
	sub := queue.NewSubscriber(rdb, testutil.NopLogger())
	t.Cleanup(func() { _ = sub.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	var (
		mu       sync.Mutex
		received []*queue.CampaignStatsUpdate
	)
	require.NoError(t, sub.SubscribeCampaignStats(ctx, func(u *queue.CampaignStatsUpdate) {
		mu.Lock()
		received = append(received, u)
		mu.Unlock()
	}))

	orgID := uuid.New()
	want := &queue.CampaignStatsUpdate{
		CampaignID:     "campaign-1",
		OrganizationID: orgID,
		Status:         models.CampaignStatusProcessing,
		SentCount:      10,
		DeliveredCount: 8,
		ReadCount:      4,
		FailedCount:    2,
	}
	require.NoError(t, pub.PublishCampaignStats(context.Background(), want))

	testutil.AssertEventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(received) == 1
	}, 2*time.Second, "subscriber should receive one update")

	mu.Lock()
	got := received[0]
	mu.Unlock()
	assert.Equal(t, want.CampaignID, got.CampaignID)
	assert.Equal(t, want.OrganizationID, got.OrganizationID)
	assert.Equal(t, want.Status, got.Status)
	assert.Equal(t, want.SentCount, got.SentCount)
	assert.Equal(t, want.DeliveredCount, got.DeliveredCount)
	assert.Equal(t, want.ReadCount, got.ReadCount)
	assert.Equal(t, want.FailedCount, got.FailedCount)
}

func TestSubscriber_FanoutToAllSubscribers(t *testing.T) {
	rdb := testutil.SetupTestRedis(t)
	if rdb == nil {
		t.Skip("TEST_REDIS_URL not set")
	}
	pub := queue.NewPublisher(rdb, testutil.NopLogger())

	subA := queue.NewSubscriber(rdb, testutil.NopLogger())
	subB := queue.NewSubscriber(rdb, testutil.NopLogger())
	t.Cleanup(func() { _ = subA.Close() })
	t.Cleanup(func() { _ = subB.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	var (
		muA, muB sync.Mutex
		gotA     int
		gotB     int
	)
	require.NoError(t, subA.SubscribeCampaignStats(ctx, func(u *queue.CampaignStatsUpdate) {
		muA.Lock()
		gotA++
		muA.Unlock()
	}))
	require.NoError(t, subB.SubscribeCampaignStats(ctx, func(u *queue.CampaignStatsUpdate) {
		muB.Lock()
		gotB++
		muB.Unlock()
	}))

	for range 3 {
		require.NoError(t, pub.PublishCampaignStats(context.Background(), &queue.CampaignStatsUpdate{
			CampaignID:     "x",
			OrganizationID: uuid.New(),
			Status:         models.CampaignStatusProcessing,
		}))
	}

	testutil.AssertEventually(t, func() bool {
		muA.Lock()
		defer muA.Unlock()
		return gotA == 3
	}, 2*time.Second, "subscriber A should receive 3 messages")

	testutil.AssertEventually(t, func() bool {
		muB.Lock()
		defer muB.Unlock()
		return gotB == 3
	}, 2*time.Second, "subscriber B should receive 3 messages")
}

func TestSubscriber_ContextCancelStopsHandler(t *testing.T) {
	rdb := testutil.SetupTestRedis(t)
	if rdb == nil {
		t.Skip("TEST_REDIS_URL not set")
	}
	pub := queue.NewPublisher(rdb, testutil.NopLogger())
	sub := queue.NewSubscriber(rdb, testutil.NopLogger())
	t.Cleanup(func() { _ = sub.Close() })

	ctx, cancel := context.WithCancel(context.Background())

	var (
		mu       sync.Mutex
		received int
	)
	require.NoError(t, sub.SubscribeCampaignStats(ctx, func(u *queue.CampaignStatsUpdate) {
		mu.Lock()
		received++
		mu.Unlock()
	}))

	// Receive one to verify the channel is alive.
	require.NoError(t, pub.PublishCampaignStats(context.Background(), &queue.CampaignStatsUpdate{
		CampaignID: "c1", OrganizationID: uuid.New(), Status: models.CampaignStatusProcessing,
	}))
	testutil.AssertEventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return received == 1
	}, 2*time.Second, "expected first message")

	// Cancel the context and close the subscriber to stop receiving further messages.
	cancel()
	require.NoError(t, sub.Close())

	// Give the goroutine a moment to wind down before publishing again.
	time.Sleep(100 * time.Millisecond)
	require.NoError(t, pub.PublishCampaignStats(context.Background(), &queue.CampaignStatsUpdate{
		CampaignID: "c2", OrganizationID: uuid.New(), Status: models.CampaignStatusProcessing,
	}))

	// Wait briefly to ensure no further deliveries.
	time.Sleep(200 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, 1, received, "no messages must be delivered after Close")
}
