package handlers

import (
	"context"
	"strings"
	"time"

	"github.com/nikyjain/whatomate/internal/models"
	"github.com/nikyjain/whatomate/internal/webhookhealth"
)

// recordWebhookHealth notes, for every phone number ID in the payload, whether
// Meta's webhook was accepted or rejected. Campaign sending consults this so it
// never sends while delivery reports are being lost.
func (a *App) recordWebhookHealth(payload *WebhookPayload, accepted bool) {
	if a.Redis == nil || payload == nil {
		return
	}
	ctx := context.Background()
	now := time.Now()
	seen := map[string]bool{}
	for _, entry := range payload.Entry {
		for _, change := range entry.Changes {
			id := strings.TrimSpace(change.Value.Metadata.PhoneNumberID)
			if id == "" || seen[id] {
				continue
			}
			seen[id] = true
			if accepted {
				webhookhealth.RecordAccepted(ctx, a.Redis, id, now)
			} else {
				webhookhealth.RecordRejected(ctx, a.Redis, id, now)
			}
		}
	}
}

// campaignDeliveryBlockReason returns why the campaign's WhatsApp number must
// not send right now (its delivery reports are not reaching the app), or "".
func (a *App) campaignDeliveryBlockReason(ctx context.Context, campaign *models.BulkMessageCampaign) string {
	if a.Redis == nil || campaign == nil {
		return ""
	}
	var account models.WhatsAppAccount
	if err := a.DB.Select("phone_id").
		Where("name = ? AND organization_id = ?", campaign.WhatsAppAccount, campaign.OrganizationID).
		First(&account).Error; err != nil {
		return ""
	}
	return webhookhealth.BlockReason(ctx, a.Redis, account.PhoneID, time.Now())
}

// pauseCampaignForDelivery pauses a campaign and stores the reason shown to
// users. Recipients stay pending so resuming sends to them.
func (a *App) pauseCampaignForDelivery(campaign *models.BulkMessageCampaign, reason string) {
	if err := a.DB.Model(&models.BulkMessageCampaign{}).Where("id = ?", campaign.ID).
		Updates(map[string]any{
			"status":       models.CampaignStatusPaused,
			"pause_reason": reason,
		}).Error; err != nil {
		a.Log.Error("Failed to pause campaign for broken delivery reports", "error", err, "campaign_id", campaign.ID)
		return
	}
	campaign.Status = models.CampaignStatusPaused
	campaign.PauseReason = reason
	a.Log.Warn("Campaign paused: delivery reports not reaching the app", "campaign_id", campaign.ID, "reason", reason)
}
