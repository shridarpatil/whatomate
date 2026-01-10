package worker

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/shridarpatil/whatomate/internal/config"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/internal/queue"
	"github.com/shridarpatil/whatomate/pkg/whatsapp"
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
	if campaign.Status == "paused" || campaign.Status == "cancelled" {
		w.Log.Info("Campaign not active, skipping recipient", "campaign_id", job.CampaignID, "status", campaign.Status, "recipient_id", job.RecipientID)
		return nil // Not an error, just skip
	}

	// Get WhatsApp account
	var account models.WhatsAppAccount
	if err := w.DB.Where("name = ? AND organization_id = ?", campaign.WhatsAppAccount, job.OrganizationID).First(&account).Error; err != nil {
		w.Log.Error("Failed to load WhatsApp account", "error", err, "account_name", campaign.WhatsAppAccount)
		w.updateRecipientStatus(job.RecipientID, "failed", "", "WhatsApp account not found")
		w.incrementCampaignCount(job.CampaignID, "failed_count")
		return nil // Don't retry, mark as failed
	}

	// Get or create contact for this recipient
	contact, err := w.getOrCreateContact(job.OrganizationID, job.PhoneNumber, job.RecipientName)
	if err != nil || contact == nil {
		w.Log.Error("Failed to get or create contact", "error", err, "phone", job.PhoneNumber)
		w.updateRecipientStatus(job.RecipientID, "failed", "", "Failed to create contact")
		w.incrementCampaignCount(job.CampaignID, "failed_count")
		return nil // Don't retry
	}

	// Build recipient for sending
	recipient := &models.BulkMessageRecipient{
		PhoneNumber:    job.PhoneNumber,
		RecipientName:  job.RecipientName,
		TemplateParams: job.TemplateParams,
	}

	// Send template message
	waMessageID, err := w.sendTemplateMessage(ctx, &account, campaign.Template, recipient)

	// Create Message record
	message := models.Message{
		OrganizationID:    job.OrganizationID,
		WhatsAppAccount:   campaign.WhatsAppAccount,
		ContactID:         contact.ID,
		WhatsAppMessageID: waMessageID,
		Direction:         "outgoing",
		MessageType:       "template",
		TemplateParams:    job.TemplateParams,
		Metadata: models.JSONB{
			"campaign_id":    job.CampaignID.String(),
			"recipient_name": job.RecipientName,
		},
	}
	if campaign.Template != nil {
		message.TemplateName = campaign.Template.Name
		content := campaign.Template.BodyContent
		if job.TemplateParams != nil {
			for i := 1; i <= 10; i++ {
				key := fmt.Sprintf("%d", i)
				if val, ok := job.TemplateParams[key]; ok {
					placeholder := fmt.Sprintf("{{%d}}", i)
					content = strings.ReplaceAll(content, placeholder, fmt.Sprintf("%v", val))
				}
			}
		}
		message.Content = content
	}

	if err != nil {
		w.Log.Error("Failed to send message", "error", err, "recipient", job.PhoneNumber)
		message.Status = "failed"
		message.ErrorMessage = err.Error()
		w.updateRecipientStatus(job.RecipientID, "failed", "", err.Error())
		w.incrementCampaignCount(job.CampaignID, "failed_count")
	} else {
		w.Log.Info("Message sent", "recipient", job.PhoneNumber, "message_id", waMessageID)
		message.Status = "sent"
		w.updateRecipientStatus(job.RecipientID, "sent", waMessageID, "")
		w.incrementCampaignCount(job.CampaignID, "sent_count")
	}

	// Save message record
	if err := w.DB.Create(&message).Error; err != nil {
		w.Log.Error("Failed to save message", "error", err, "recipient", job.PhoneNumber)
	}

	// Check if campaign is complete (all recipients processed)
	w.checkCampaignCompletion(ctx, job.CampaignID, job.OrganizationID)

	return nil
}

// updateRecipientStatus updates the recipient's status in the database
func (w *Worker) updateRecipientStatus(recipientID uuid.UUID, status, waMessageID, errorMsg string) {
	updates := map[string]interface{}{
		"status":               status,
		"whats_app_message_id": waMessageID,
	}
	if status == "sent" {
		updates["sent_at"] = time.Now()
	}
	if errorMsg != "" {
		updates["error_message"] = errorMsg
	}
	w.DB.Model(&models.BulkMessageRecipient{}).Where("id = ?", recipientID).Updates(updates)
}

// incrementCampaignCount increments a campaign counter atomically
func (w *Worker) incrementCampaignCount(campaignID uuid.UUID, column string) {
	w.DB.Model(&models.BulkMessageCampaign{}).
		Where("id = ?", campaignID).
		Update(column, gorm.Expr(column+" + 1"))
}

// publishCampaignStats publishes campaign stats for real-time updates
func (w *Worker) publishCampaignStats(ctx context.Context, campaignID, organizationID uuid.UUID) {
	var campaign models.BulkMessageCampaign
	if err := w.DB.Where("id = ?", campaignID).First(&campaign).Error; err != nil {
		return
	}

	_ = w.Publisher.PublishCampaignStats(ctx, &queue.CampaignStatsUpdate{
		CampaignID:     campaignID.String(),
		OrganizationID: organizationID,
		Status:         campaign.Status,
		SentCount:      campaign.SentCount,
		DeliveredCount: campaign.DeliveredCount,
		ReadCount:      campaign.ReadCount,
		FailedCount:    campaign.FailedCount,
	})
}

// checkCampaignCompletion checks if all recipients are processed and marks campaign as completed
func (w *Worker) checkCampaignCompletion(ctx context.Context, campaignID, organizationID uuid.UUID) {
	// Count pending recipients
	var pendingCount int64
	w.DB.Model(&models.BulkMessageRecipient{}).
		Where("campaign_id = ? AND status = ?", campaignID, "pending").
		Count(&pendingCount)

	// If no pending recipients, mark campaign as completed
	if pendingCount == 0 {
		var campaign models.BulkMessageCampaign
		if err := w.DB.Where("id = ?", campaignID).First(&campaign).Error; err != nil {
			return
		}

		// Only complete if currently processing
		if campaign.Status != "processing" {
			return
		}

		now := time.Now()
		w.DB.Model(&campaign).Updates(map[string]interface{}{
			"status":       "completed",
			"completed_at": now,
		})

		w.Log.Info("Campaign completed", "campaign_id", campaignID, "sent", campaign.SentCount, "failed", campaign.FailedCount)

		// Publish completion status
		_ = w.Publisher.PublishCampaignStats(ctx, &queue.CampaignStatsUpdate{
			CampaignID:     campaignID.String(),
			OrganizationID: organizationID,
			Status:         "completed",
			SentCount:      campaign.SentCount,
			DeliveredCount: campaign.DeliveredCount,
			ReadCount:      campaign.ReadCount,
			FailedCount:    campaign.FailedCount,
		})
	} else {
		// Publish current stats
		w.publishCampaignStats(ctx, campaignID, organizationID)
	}
}

// sendTemplateMessage sends a template message via WhatsApp Cloud API
func (w *Worker) sendTemplateMessage(ctx context.Context, account *models.WhatsAppAccount, template *models.Template, recipient *models.BulkMessageRecipient) (string, error) {
	waAccount := &whatsapp.Account{
		PhoneID:     account.PhoneID,
		BusinessID:  account.BusinessID,
		APIVersion:  account.APIVersion,
		AccessToken: account.AccessToken,
	}

	// Build template components with parameters
	var components []map[string]interface{}

	// Add body parameters if template has variables
	if len(recipient.TemplateParams) > 0 {
		bodyParams := []map[string]interface{}{}
		for i := 1; i <= 10; i++ {
			key := fmt.Sprintf("%d", i)
			if val, ok := recipient.TemplateParams[key]; ok {
				bodyParams = append(bodyParams, map[string]interface{}{
					"type": "text",
					"text": val,
				})
			}
		}
		if len(bodyParams) > 0 {
			components = append(components, map[string]interface{}{
				"type":       "body",
				"parameters": bodyParams,
			})
		}
	}

	return w.WhatsApp.SendTemplateMessageWithComponents(ctx, waAccount, recipient.PhoneNumber, template.Name, template.Language, components)
}

// Close cleans up worker resources
func (w *Worker) Close() error {
	if w.Consumer != nil {
		return w.Consumer.Close()
	}
	return nil
}

// getOrCreateContact finds or creates a contact for a phone number
func (w *Worker) getOrCreateContact(orgID uuid.UUID, phoneNumber, name string) (*models.Contact, error) {
	// Normalize phone number (remove + prefix if present)
	normalizedPhone := phoneNumber
	if len(normalizedPhone) > 0 && normalizedPhone[0] == '+' {
		normalizedPhone = normalizedPhone[1:]
	}

	// Try to find existing contact
	var contact models.Contact
	err := w.DB.Where("organization_id = ? AND phone_number = ?", orgID, normalizedPhone).First(&contact).Error
	if err == nil {
		return &contact, nil
	}

	// Also try with + prefix
	err = w.DB.Where("organization_id = ? AND phone_number = ?", orgID, "+"+normalizedPhone).First(&contact).Error
	if err == nil {
		return &contact, nil
	}

	// Create new contact
	contact = models.Contact{
		OrganizationID: orgID,
		PhoneNumber:    normalizedPhone,
		ProfileName:    name,
	}
	if err := w.DB.Create(&contact).Error; err != nil {
		return nil, fmt.Errorf("failed to create contact: %w", err)
	}

	w.Log.Info("Created new contact for campaign recipient", "phone", normalizedPhone, "name", name)
	return &contact, nil
}
