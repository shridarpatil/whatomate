// Package webhookhealth tracks, per WhatsApp phone number ID, whether Meta's
// webhook callbacks (incoming messages and delivery statuses) are actually
// reaching and being accepted by this app.
//
// Campaign delivery/read statistics and incoming messages depend entirely on
// those callbacks. When they are rejected (wrong app secret) or never arrive
// (webhook subscription removed, wrong callback URL), campaigns still send but
// every delivery report is lost. Campaign sending consults BlockReason and
// refuses or pauses while a number's callbacks are known to be broken.
//
// State lives in one Redis hash per phone number ID:
//
//	ok_at              last accepted webhook (unix ms)
//	rejected_at        last webhook rejected for its signature (unix ms)
//	unconfirmed_since  first campaign send not yet followed by any accepted
//	                   webhook (unix ms); cleared by every accepted webhook
//
// Missing state means "unknown" and never blocks. Every function is a no-op
// when the Redis client is nil or the phone number ID is empty.
package webhookhealth

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	keyPrefix = "whatsapp:webhook_health:"
	keyTTL    = 30 * 24 * time.Hour

	fieldOK          = "ok_at"
	fieldRejected    = "rejected_at"
	fieldUnconfirmed = "unconfirmed_since"

	// UnconfirmedGrace is how long a campaign send may go without any accepted
	// webhook before the number is treated as not receiving callbacks. Meta
	// reports "sent" within seconds of every message, so silence this long
	// means callbacks are not arriving.
	UnconfirmedGrace = 10 * time.Minute
)

func key(phoneID string) string { return keyPrefix + strings.TrimSpace(phoneID) }

func usable(rdb *redis.Client, phoneID string) bool {
	return rdb != nil && strings.TrimSpace(phoneID) != ""
}

// RecordAccepted notes that a webhook for phoneID passed verification. It
// clears any pending unconfirmed-send marker.
func RecordAccepted(ctx context.Context, rdb *redis.Client, phoneID string, now time.Time) {
	if !usable(rdb, phoneID) {
		return
	}
	k := key(phoneID)
	pipe := rdb.Pipeline()
	pipe.HSet(ctx, k, fieldOK, now.UnixMilli())
	pipe.HDel(ctx, k, fieldUnconfirmed)
	pipe.Expire(ctx, k, keyTTL)
	_, _ = pipe.Exec(ctx)
}

// RecordRejected notes that a webhook for phoneID was rejected because its
// signature could not be verified.
func RecordRejected(ctx context.Context, rdb *redis.Client, phoneID string, now time.Time) {
	if !usable(rdb, phoneID) {
		return
	}
	k := key(phoneID)
	pipe := rdb.Pipeline()
	pipe.HSet(ctx, k, fieldRejected, now.UnixMilli())
	pipe.Expire(ctx, k, keyTTL)
	_, _ = pipe.Exec(ctx)
}

// RecordCampaignSend notes a campaign message sent from phoneID. Only the
// first send after the last accepted webhook is kept.
func RecordCampaignSend(ctx context.Context, rdb *redis.Client, phoneID string, now time.Time) {
	if !usable(rdb, phoneID) {
		return
	}
	k := key(phoneID)
	pipe := rdb.Pipeline()
	pipe.HSetNX(ctx, k, fieldUnconfirmed, now.UnixMilli())
	pipe.Expire(ctx, k, keyTTL)
	_, _ = pipe.Exec(ctx)
}

// BlockReason returns a user-facing explanation when campaigns from phoneID
// must not be sent, or "" when sending is allowed (including when the state is
// unknown or Redis is unavailable).
func BlockReason(ctx context.Context, rdb *redis.Client, phoneID string, now time.Time) string {
	if !usable(rdb, phoneID) {
		return ""
	}
	vals, err := rdb.HGetAll(ctx, key(phoneID)).Result()
	if err != nil {
		return ""
	}
	return blockReason(vals, now)
}

func blockReason(vals map[string]string, now time.Time) string {
	okAt := parseMillis(vals[fieldOK])
	rejectedAt := parseMillis(vals[fieldRejected])
	unconfirmed := parseMillis(vals[fieldUnconfirmed])

	if rejectedAt > 0 && rejectedAt > okAt {
		return fmt.Sprintf(
			"WhatsApp delivery reports are being rejected for this number since %s because the Meta App Secret saved in Whatomate does not match. "+
				"Campaigns are blocked so delivery data is not lost. Save the correct App Secret (Settings → Accounts), then send a WhatsApp message to this number to confirm, and start the campaign again.",
			formatTime(rejectedAt))
	}
	if unconfirmed > 0 && now.Sub(time.UnixMilli(unconfirmed)) >= UnconfirmedGrace {
		return fmt.Sprintf(
			"No delivery reports have arrived from WhatsApp for this number since a campaign message was sent at %s. "+
				"Campaigns are blocked so delivery data is not lost. Check the webhook setup in Meta (callback URL and \"messages\" subscription), then send a WhatsApp message to this number to confirm, and start the campaign again.",
			formatTime(unconfirmed))
	}
	return ""
}

func parseMillis(s string) int64 {
	v, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return 0
	}
	return v
}

// formatTime renders dd/mm/yyyy HH:mm in UTC, matching the app's date display rule.
func formatTime(ms int64) string {
	return time.UnixMilli(ms).UTC().Format("02/01/2006 15:04") + " UTC"
}
