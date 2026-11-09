package handlers

import (
	"context"
	"time"

	"github.com/nikyjain/whatomate/internal/models"
)

// StartScheduledCampaignProcessor starts a background loop that promotes due scheduled campaigns
// into the queue for worker processing.
func (a *App) StartScheduledCampaignProcessor(interval time.Duration) {
	if interval <= 0 {
		interval = time.Minute
	}

	ctx, cancel := context.WithCancel(context.Background())
	a.ScheduledCampaignCancel = cancel

	a.wg.Add(1)
	go func() {
		defer a.wg.Done()

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		a.ProcessDueCampaigns(ctx)

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				a.ProcessDueCampaigns(ctx)
			}
		}
	}()
}

// StopScheduledCampaignProcessor stops the background scheduled campaign loop.
func (a *App) StopScheduledCampaignProcessor() {
	if a.ScheduledCampaignCancel != nil {
		a.ScheduledCampaignCancel()
	}
}

// ProcessDueCampaigns finds scheduled campaigns whose send time has arrived and enqueues them.
func (a *App) ProcessDueCampaigns(ctx context.Context) {
	var campaigns []models.BulkMessageCampaign
	if err := a.DB.Where("status = ? AND scheduled_at IS NOT NULL AND scheduled_at <= ?",
		models.CampaignStatusScheduled, time.Now()).
		Preload("Template").
		Find(&campaigns).Error; err != nil {
		a.Log.Error("Failed to load scheduled campaigns", "error", err)
		return
	}

	for i := range campaigns {
		campaign := &campaigns[i]

		if err := a.validateCampaignReadyForStart(campaign); err != nil {
			a.Log.Error("Scheduled campaign is not ready to start", "campaign_id", campaign.ID, "error", err)
			_ = a.DB.Model(campaign).Update("status", models.CampaignStatusFailed).Error
			continue
		}

		recipients, err := a.loadPendingCampaignRecipients(campaign.ID)
		if err != nil {
			a.Log.Error("Failed to load scheduled campaign recipients", "campaign_id", campaign.ID, "error", err)
			continue
		}
		if len(recipients) == 0 {
			a.Log.Warn("Scheduled campaign has no pending recipients", "campaign_id", campaign.ID)
			_ = a.DB.Model(campaign).Update("status", models.CampaignStatusFailed).Error
			continue
		}

		if reason := a.campaignDeliveryBlockReason(ctx, campaign); reason != "" {
			a.pauseCampaignForDelivery(campaign, reason)
			continue
		}

		if err := a.enqueueCampaignRecipients(ctx, campaign, recipients, time.Now(), models.CampaignStatusScheduled); err != nil {
			a.Log.Error("Failed to enqueue scheduled campaign", "campaign_id", campaign.ID, "error", err)
			continue
		}

		a.Log.Info("Scheduled campaign started", "campaign_id", campaign.ID, "recipients", len(recipients))
	}
}
