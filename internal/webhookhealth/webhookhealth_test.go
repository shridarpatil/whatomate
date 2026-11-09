package webhookhealth

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nikyjain/whatomate/test/testutil"
)

func ms(t time.Time) string { return strconv.FormatInt(t.UnixMilli(), 10) }

func TestBlockReason_Rules(t *testing.T) {
	now := time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC)
	cases := []struct {
		name    string
		vals    map[string]string
		blocked bool
	}{
		{"unknown state allows", map[string]string{}, false},
		{"only accepted allows", map[string]string{fieldOK: ms(now.Add(-time.Hour))}, false},
		{"rejected after accepted blocks", map[string]string{fieldOK: ms(now.Add(-2 * time.Hour)), fieldRejected: ms(now.Add(-time.Hour))}, true},
		{"rejected with no accepted blocks", map[string]string{fieldRejected: ms(now.Add(-time.Minute))}, true},
		{"accepted after rejected allows", map[string]string{fieldRejected: ms(now.Add(-2 * time.Hour)), fieldOK: ms(now.Add(-time.Hour))}, false},
		{"fresh unconfirmed send allows", map[string]string{fieldUnconfirmed: ms(now.Add(-time.Minute))}, false},
		{"stale unconfirmed send blocks", map[string]string{fieldUnconfirmed: ms(now.Add(-UnconfirmedGrace))}, true},
		{"garbage values allow", map[string]string{fieldRejected: "x", fieldUnconfirmed: "y"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := blockReason(tc.vals, now)
			if (got != "") != tc.blocked {
				t.Fatalf("blocked=%v, want %v (reason %q)", got != "", tc.blocked, got)
			}
		})
	}
}

func TestNilRedisNeverBlocks(t *testing.T) {
	ctx := context.Background()
	RecordRejected(ctx, nil, "123", time.Now())
	if r := BlockReason(ctx, nil, "123", time.Now()); r != "" {
		t.Fatalf("nil redis must not block, got %q", r)
	}
}

func TestRedisLifecycle(t *testing.T) {
	rdb := testutil.SetupTestRedis(t)
	if rdb == nil {
		t.Skip("TEST_REDIS_URL not set")
	}
	ctx := context.Background()
	phone := "test-" + uuid.NewString()
	t.Cleanup(func() { rdb.Del(ctx, key(phone)) })
	now := time.Now()

	RecordAccepted(ctx, rdb, phone, now.Add(-time.Hour))
	RecordRejected(ctx, rdb, phone, now.Add(-time.Minute))
	if BlockReason(ctx, rdb, phone, now) == "" {
		t.Fatal("expected block after rejection")
	}
	RecordAccepted(ctx, rdb, phone, now)
	if r := BlockReason(ctx, rdb, phone, now); r != "" {
		t.Fatalf("accepted webhook must clear block, got %q", r)
	}

	RecordCampaignSend(ctx, rdb, phone, now.Add(-UnconfirmedGrace-time.Minute))
	RecordCampaignSend(ctx, rdb, phone, now) // must not move the marker forward
	if BlockReason(ctx, rdb, phone, now) == "" {
		t.Fatal("expected block for unconfirmed campaign send")
	}
	RecordAccepted(ctx, rdb, phone, now)
	if r := BlockReason(ctx, rdb, phone, now); r != "" {
		t.Fatalf("accepted webhook must clear unconfirmed send, got %q", r)
	}
}
