package worker

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nikyjain/whatomate/internal/config"
	"github.com/nikyjain/whatomate/internal/contactutil"
	"github.com/nikyjain/whatomate/internal/models"
	"github.com/nikyjain/whatomate/internal/queue"
	"github.com/nikyjain/whatomate/internal/templateutil"
	"github.com/nikyjain/whatomate/pkg/whatsapp"
	"github.com/redis/go-redis/v9"
	"github.com/zerodha/logf"
	"gorm.io/gorm"
)

// Worker processes jobs from the queue
type Worker struct {
	Config    *config.Config
	DB        *gorm.DB
	Redis     *redis.Client
	Log       logf.Logger
	WhatsApp  *whatsapp.Client
	Consumer  *queue.RedisConsumer
	Publisher *queue.Publisher
}

// Ensure Worker implements JobHandler interface
var _ queue.JobHandler = (*Worker)(nil)

// New creates a new Worker instance
func New(cfg *config.Config, db *gorm.DB, rdb *redis.Client, log logf.Logger) (*Worker, error) {
	consumer, err := queue.NewRedisConsumer(rdb, log)
	if err != nil {
		return nil, fmt.Errorf("failed to create consumer: %w", err)
	}

	publisher := queue.NewPublisher(rdb, log)

	return &Worker{
		Config:    cfg,
		DB:        db,
		Redis:     rdb,
		Log:       log,
		WhatsApp:  whatsapp.New(log),
		Consumer:  consumer,
		Publisher: publisher,
	}, nil
}

// Run starts the worker and processes jobs until context is cancelled
func (w *Worker) Run(ctx context.Context) error {
	w.Log.Info("Worker starting")

	err := w.Consumer.Consume(ctx, w)
	if err != nil && ctx.Err() == nil {
		return fmt.Errorf("consumer error: %w", err)
	}

	w.Log.Info("Worker stopped")
	return nil
}

// HandleRecipientJob processes a single recipient message job
func (w *Worker) HandleRecipientJob(ctx context.Context, job *queue.RecipientJob) error {
	// Check if campaign is still active before sending
	var campaign models.BulkMessageCampaign
	if err := w.DB.Where("id = ?", job.CampaignID).Preload("Template").First(&campaign).Error; err != nil {
		w.Log.Error("Failed to load campaign", "error", err, "campaign_id", job.CampaignID)
		return fmt.Errorf("failed to load campaign: %w", err)
	}

	// Skip if campaign is paused or cancelled
	if campaign.Status == models.CampaignStatusPaused || campaign.Status == models.CampaignStatusCancelled {
		w.Log.Info("Campaign not active, skipping recipient", "campaign_id", job.CampaignID, "status", campaign.Status, "recipient_id", job.RecipientID)
		return nil // Not an error, just skip
	}
	if w.skipIneligibleRSVPReminder(ctx, job) {
		return nil
	}

	// Get WhatsApp account
	var account models.WhatsAppAccount
	if err := w.DB.Where("name = ? AND organization_id = ?", campaign.WhatsAppAccount, job.OrganizationID).First(&account).Error; err != nil {
		w.Log.Error("Failed to load WhatsApp account", "error", err, "account_name", campaign.WhatsAppAccount)
		w.updateRecipientStatus(job.RecipientID, models.MessageStatusFailed, "", "WhatsApp account not found")
		w.incrementCampaignCount(job.CampaignID, "failed_count")
		w.checkCampaignCompletion(ctx, job.CampaignID, job.OrganizationID)
		return nil // Don't retry, mark as failed
	}
	w.decryptAccountSecrets(&account)

	// Get or create contact for this recipient
	contact, _, err := contactutil.GetOrCreateContact(w.DB, job.OrganizationID, job.PhoneNumber, job.RecipientName)
	if err != nil || contact == nil {
		w.Log.Error("Failed to get or create contact", "error", err, "phone", job.PhoneNumber)
		w.updateRecipientStatus(job.RecipientID, models.MessageStatusFailed, "", "Failed to create contact")
		w.incrementCampaignCount(job.CampaignID, "failed_count")
		w.checkCampaignCompletion(ctx, job.CampaignID, job.OrganizationID)
		return nil // Don't retry
	}
	if !contact.IsActive {
		w.Log.Info("Skipping inactive contact for campaign recipient", "campaign_id", job.CampaignID, "contact_id", contact.ID, "recipient_id", job.RecipientID)
		w.updateRecipientStatus(job.RecipientID, models.MessageStatusFailed, "", "Contact is inactive")
		w.incrementCampaignCount(job.CampaignID, "failed_count")
		w.checkCampaignCompletion(ctx, job.CampaignID, job.OrganizationID)
		return nil
	}

	// Check marketing opt-out
	if contact.MarketingOptOut && campaign.Template != nil && strings.EqualFold(campaign.Template.Category, "MARKETING") {
		w.Log.Info("Skipping marketing message for opted-out contact", "contact_id", contact.ID, "phone", job.PhoneNumber)
		w.updateRecipientStatus(job.RecipientID, models.MessageStatusFailed, "", "Contact opted out of marketing messages")
		w.incrementCampaignCount(job.CampaignID, "failed_count")
		w.checkCampaignCompletion(ctx, job.CampaignID, job.OrganizationID)
		return nil
	}

	// Build recipient for sending
	recipient := &models.BulkMessageRecipient{
		PhoneNumber:    job.PhoneNumber,
		RecipientName:  job.RecipientName,
		TemplateParams: job.TemplateParams,
		HeaderParams:   job.HeaderParams,
	}

	// Send template message
	waMessageID, err := w.sendTemplateMessage(ctx, &account, campaign.Template, recipient, campaign.HeaderMediaID, campaignHeaderMediaRef(&campaign))

	// Create Message record
	message := models.Message{
		OrganizationID:    job.OrganizationID,
		WhatsAppAccount:   campaign.WhatsAppAccount,
		ContactID:         contact.ID,
		WhatsAppMessageID: waMessageID,
		Direction:         models.DirectionOutgoing,
		MessageType:       models.MessageTypeTemplate,
		TemplateParams:    job.TemplateParams,
		Metadata: models.JSONB{
			"campaign_id":    job.CampaignID.String(),
			"recipient_name": job.RecipientName,
		},
	}
	if campaign.Template != nil {
		message.Metadata["delivery_route"] = string(models.ResolveTemplateDeliveryRoute(&account, campaign.Template))
		message.TemplateName = campaign.Template.Name
		content := templateutil.ReplaceWithJSONBParams(campaign.Template.BodyContent, campaign.Template.BodyContent, job.TemplateParams)
		message.Content = content
		// Store campaign header media so it renders in the chat bubble
		if campaign.HeaderMediaLocalPath != "" {
			message.MediaURL = campaign.HeaderMediaLocalPath
			message.MediaMimeType = campaign.HeaderMediaMimeType
			message.MediaFilename = campaign.HeaderMediaFilename
		}
	}

	if err != nil {
		w.Log.Error("Failed to send message", "error", err, "recipient", job.PhoneNumber)
		message.Status = models.MessageStatusFailed
		message.ErrorMessage = err.Error()
		w.markContactInactiveIfUndeliverable(contact, err)
		w.updateRecipientStatus(job.RecipientID, models.MessageStatusFailed, "", err.Error())
		w.incrementCampaignCount(job.CampaignID, "failed_count")
	} else {
		w.Log.Info("Message sent", "recipient", job.PhoneNumber, "message_id", waMessageID)
		message.Status = models.MessageStatusSent
		w.updateRecipientStatus(job.RecipientID, models.MessageStatusSent, waMessageID, "")
		w.incrementCampaignCount(job.CampaignID, "sent_count")
	}

	// Save message record
	if err := w.DB.Create(&message).Error; err != nil {
		w.Log.Error("Failed to save message", "error", err, "recipient", job.PhoneNumber)
	}

	// Update contact's last message time and preview so conversation sorts to top in /chat
	if err == nil {
		now := time.Now()
		preview := message.Content
		if len(preview) > 100 {
			preview = preview[:100]
		}
		if preview == "" && campaign.Template != nil {
			preview = campaign.Template.DisplayName
		}
		if dbErr := w.DB.Model(contact).Updates(map[string]any{
			"last_message_at":      now,
			"last_message_preview": preview,
		}).Error; dbErr != nil {
			w.Log.Error("Failed to update contact last message", "error", dbErr, "contact_id", contact.ID)
		}
	}

	// Check if campaign is complete (all recipients processed)
	w.checkCampaignCompletion(ctx, job.CampaignID, job.OrganizationID)

	return nil
}

// updateRecipientStatus updates the recipient's status in the database
func (w *Worker) updateRecipientStatus(recipientID uuid.UUID, status models.MessageStatus, waMessageID, errorMsg string) {
	now := time.Now()
	updates := map[string]interface{}{
		"status":               status,
		"whats_app_message_id": waMessageID,
	}
	if status == models.MessageStatusSent {
		updates["sent_at"] = now
		updates["error_message"] = ""
	}
	if errorMsg != "" {
		updates["error_message"] = errorMsg
	}
	w.DB.Model(&models.BulkMessageRecipient{}).Where("id = ?", recipientID).Updates(updates)

	deliveryUpdates := map[string]interface{}{"attempted_at": now.UTC()}
	switch status {
	case models.MessageStatusSent:
		deliveryUpdates["status"] = models.RSVPReminderDeliverySent
		deliveryUpdates["message_id"] = waMessageID
		deliveryUpdates["error_message"] = ""
	case models.MessageStatusFailed:
		deliveryUpdates["status"] = models.RSVPReminderDeliveryFailed
		deliveryUpdates["error_message"] = errorMsg
	}
	if len(deliveryUpdates) > 1 {
		w.DB.Model(&models.RSVPReminderDelivery{}).
			Where("campaign_recipient_id = ?", recipientID).
			Updates(deliveryUpdates)
	}
}

func (w *Worker) skipIneligibleRSVPReminder(ctx context.Context, job *queue.RecipientJob) bool {
	var delivery models.RSVPReminderDelivery
	if err := w.DB.Where("campaign_recipient_id = ?", job.RecipientID).First(&delivery).Error; err != nil {
		return false
	}
	if delivery.Status == models.RSVPReminderDeliverySent || delivery.Status == models.RSVPReminderDeliverySkipped {
		return true
	}

	var eligible int64
	w.DB.Model(&models.RSVPResponse{}).
		Where("id = ? AND organization_id = ? AND rsvp_started_at IS NULL AND responded_at IS NULL", delivery.RSVPResponseID, job.OrganizationID).
		Count(&eligible)
	if eligible > 0 {
		return false
	}

	now := time.Now().UTC()
	reason := "RSVP already started or completed; reminder skipped"
	updated := w.DB.Model(&models.BulkMessageRecipient{}).Where("id = ? AND status = ?", job.RecipientID, models.MessageStatusPending).Updates(map[string]interface{}{
		"status":        models.MessageStatusFailed,
		"error_message": reason,
	})
	if updated.Error != nil || updated.RowsAffected == 0 {
		return true
	}
	w.DB.Model(&delivery).Updates(map[string]interface{}{
		"status":        models.RSVPReminderDeliverySkipped,
		"error_message": reason,
		"attempted_at":  now,
	})
	w.incrementCampaignCount(job.CampaignID, "failed_count")
	w.checkCampaignCompletion(ctx, job.CampaignID, job.OrganizationID)
	return true
}

func (w *Worker) markContactInactiveIfUndeliverable(contact *models.Contact, err error) {
	if contact == nil || err == nil || !isUndeliverableMessageError(err) {
		return
	}

	if dbErr := w.DB.Model(contact).Update("is_active", false).Error; dbErr != nil {
		w.Log.Error("Failed to mark undeliverable contact inactive", "error", dbErr, "contact_id", contact.ID)
		return
	}
	contact.IsActive = false
	w.Log.Info("Marked contact inactive after undeliverable message", "contact_id", contact.ID, "phone", contact.PhoneNumber)
}

func isUndeliverableMessageError(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "message undeliverable") || strings.Contains(message, "131026")
}

// incrementCampaignCount increments a campaign counter atomically
func (w *Worker) incrementCampaignCount(campaignID uuid.UUID, column string) {
	w.DB.Model(&models.BulkMessageCampaign{}).
		Where("id = ?", campaignID).
		Update(column, gorm.Expr(column+" + 1"))
}

// publishCampaignStats publishes campaign stats for real-time updates
func (w *Worker) publishCampaignStats(ctx context.Context, campaignID, organizationID uuid.UUID) {
	if w.Publisher == nil {
		return
	}
	var campaign models.BulkMessageCampaign
	if err := w.DB.Where("id = ?", campaignID).First(&campaign).Error; err != nil {
		return
	}

	_ = w.Publisher.PublishCampaignStats(ctx, &queue.CampaignStatsUpdate{
		CampaignID:      campaignID.String(),
		OrganizationID:  organizationID,
		Status:          campaign.Status,
		TotalRecipients: campaign.TotalRecipients,
		SentCount:       campaign.SentCount,
		DeliveredCount:  campaign.DeliveredCount,
		ReadCount:       campaign.ReadCount,
		FailedCount:     campaign.FailedCount,
		StartedAt:       campaign.StartedAt,
		CompletedAt:     campaign.CompletedAt,
	})
}

// checkCampaignCompletion checks if all recipients are processed and marks campaign as completed
func (w *Worker) checkCampaignCompletion(ctx context.Context, campaignID, organizationID uuid.UUID) {
	// Count pending recipients
	var pendingCount int64
	w.DB.Model(&models.BulkMessageRecipient{}).
		Where("campaign_id = ? AND status = ?", campaignID, models.MessageStatusPending).
		Count(&pendingCount)

	// If no pending recipients, mark campaign as completed
	if pendingCount == 0 {
		var campaign models.BulkMessageCampaign
		if err := w.DB.Where("id = ?", campaignID).First(&campaign).Error; err != nil {
			return
		}

		// Only complete if currently processing
		if campaign.Status != models.CampaignStatusProcessing {
			return
		}

		now := time.Now()
		w.DB.Model(&campaign).Updates(map[string]any{
			"status":       models.CampaignStatusCompleted,
			"completed_at": now,
		})
		campaign.Status = models.CampaignStatusCompleted
		campaign.CompletedAt = &now

		var reminderCounts struct {
			Sent   int64
			Failed int64
		}
		w.DB.Model(&models.RSVPReminderDelivery{}).
			Where("campaign_id = ?", campaignID).
			Select(`
				COUNT(CASE WHEN status = 'sent' THEN 1 END) AS sent,
				COUNT(CASE WHEN status = 'failed' THEN 1 END) AS failed
			`).Scan(&reminderCounts)
		scheduleStatus := models.RSVPReminderScheduleCompleted
		if reminderCounts.Failed > 0 {
			scheduleStatus = models.RSVPReminderScheduleCompletedWithErrors
		}
		w.DB.Model(&models.RSVPReminderSchedule{}).
			Where("campaign_id = ? AND status = ?", campaignID, models.RSVPReminderScheduleProcessing).
			Updates(map[string]interface{}{
				"status":       scheduleStatus,
				"sent_count":   reminderCounts.Sent,
				"failed_count": reminderCounts.Failed,
				"processed_at": now.UTC(),
			})

		w.Log.Info("Campaign completed", "campaign_id", campaignID, "sent", campaign.SentCount, "failed", campaign.FailedCount)

		// Publish completion status
		if w.Publisher != nil {
			_ = w.Publisher.PublishCampaignStats(ctx, &queue.CampaignStatsUpdate{
				CampaignID:      campaignID.String(),
				OrganizationID:  organizationID,
				Status:          campaign.Status,
				TotalRecipients: campaign.TotalRecipients,
				SentCount:       campaign.SentCount,
				DeliveredCount:  campaign.DeliveredCount,
				ReadCount:       campaign.ReadCount,
				FailedCount:     campaign.FailedCount,
				StartedAt:       campaign.StartedAt,
				CompletedAt:     campaign.CompletedAt,
			})
		}
	} else {
		// Publish current stats
		w.publishCampaignStats(ctx, campaignID, organizationID)
	}
}

// sendTemplateMessage sends a template message via WhatsApp Cloud API
func (w *Worker) sendTemplateMessage(ctx context.Context, account *models.WhatsAppAccount, template *models.Template, recipient *models.BulkMessageRecipient, campaignHeaderMediaID, campaignHeaderMediaURL string) (string, error) {
	waAccount := account.ToWAAccount()

	// Resolve body and button parameters for template sending.
	bodyParams := templateutil.ResolveParamsMap(template.BodyContent, recipient.TemplateParams)
	buttonParams, _ := templateutil.ResolveURLButtonParams(template.Buttons, recipient.TemplateParams)

	// Resolve the header text parameter (if any) into its own map. Prefer
	// recipient.HeaderParams (new path, populated by AddRecipients) and fall
	// back to a TemplateParams lookup for legacy recipient rows persisted
	// before HeaderParams existed.
	var headerParams map[string]string
	if template.HeaderType == "TEXT" {
		if hNames := templateutil.ExtParamNames(template.HeaderContent); len(hNames) == 1 {
			name := hNames[0]
			if raw, ok := recipient.HeaderParams[name]; ok {
				headerParams = map[string]string{name: fmt.Sprintf("%v", raw)}
			} else if raw, ok := recipient.TemplateParams[name]; ok {
				headerParams = map[string]string{name: fmt.Sprintf("%v", raw)}
			}
		}
	}

	// Use the shared component builder (same as chat template sending).
	components, err := whatsapp.BuildTemplateComponentsWithHeaderText(
		bodyParams, buttonParams, nil, template.Buttons,
		template.HeaderType, template.HeaderContent, headerParams,
		campaignHeaderMediaID, campaignHeaderMediaURL,
	)
	if err != nil {
		return "", fmt.Errorf("failed to build template components: %w", err)
	}
	// Add auto-generated button components (Flow needs flow_token)
	components = append(components, whatsapp.AutoButtonComponents(template.Buttons)...)

	rcpt := whatsapp.Recipient{Phone: recipient.PhoneNumber}
	route := models.ResolveTemplateDeliveryRoute(account, template)
	if route == models.TemplateDeliveryRouteMarketingMessagesLite {
		return w.WhatsApp.SendMarketingTemplateMessage(ctx, waAccount, rcpt, template.Name, template.Language, components)
	}

	return w.WhatsApp.SendTemplateMessage(ctx, waAccount, rcpt, template.Name, template.Language, components)
}

// decryptAccountSecrets decrypts the encrypted secrets on a WhatsApp account.
func (w *Worker) decryptAccountSecrets(account *models.WhatsAppAccount) {
	var key string
	if w.Config != nil {
		key = w.Config.App.EncryptionKey
	}
	account.DecryptSecrets(key)
}

// Close cleans up worker resources
func (w *Worker) Close() error {
	if w.Consumer != nil {
		return w.Consumer.Close()
	}
	return nil
}

// campaignHeaderMediaRef returns the value passed to the template component
// builder alongside the campaign's header media ID: with an uploaded media ID it
// is the document filename (Meta requires one for DOCUMENT headers), otherwise
// it is the public media link.
func campaignHeaderMediaRef(campaign *models.BulkMessageCampaign) string {
	if campaign.HeaderMediaID != "" && strings.TrimSpace(campaign.HeaderMediaFilename) != "" {
		return campaign.HeaderMediaFilename
	}
	return campaign.HeaderMediaURL
}
