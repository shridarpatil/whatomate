package handlers

import (
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/internal/queue"
	"github.com/shridarpatil/whatomate/internal/websocket"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
	"gorm.io/gorm"
)

// CampaignRequest represents campaign create/update request
type CampaignRequest struct {
	Name            string     `json:"name" validate:"required"`
	WhatsAppAccount string     `json:"whatsapp_account" validate:"required"`
	TemplateID      string     `json:"template_id" validate:"required"`
	ScheduledAt     *time.Time `json:"scheduled_at"`
}

// CampaignResponse represents campaign in API responses
type CampaignResponse struct {
	ID              uuid.UUID            `json:"id"`
	Name            string               `json:"name"`
	WhatsAppAccount string               `json:"whatsapp_account"`
	TemplateID      uuid.UUID            `json:"template_id"`
	TemplateName    string               `json:"template_name,omitempty"`
	Status          models.CampaignStatus `json:"status"`
	TotalRecipients int                  `json:"total_recipients"`
	SentCount       int                  `json:"sent_count"`
	DeliveredCount  int                  `json:"delivered_count"`
	ReadCount       int                  `json:"read_count"`
	FailedCount     int                  `json:"failed_count"`
	ScheduledAt     *time.Time           `json:"scheduled_at,omitempty"`
	StartedAt       *time.Time           `json:"started_at,omitempty"`
	CompletedAt     *time.Time           `json:"completed_at,omitempty"`
	CreatedAt       time.Time            `json:"created_at"`
	UpdatedAt       time.Time            `json:"updated_at"`
}

// RecipientRequest represents recipient import request
type RecipientRequest struct {
	PhoneNumber    string                 `json:"phone_number" validate:"required"`
	RecipientName  string                 `json:"recipient_name"`
	TemplateParams map[string]interface{} `json:"template_params"`
}

// ListCampaigns implements campaign listing
func (a *App) ListCampaigns(r *fastglue.Request) error {
	orgID, err := a.getOrgIDFromContext(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	// Get query params
	status := string(r.RequestCtx.QueryArgs().Peek("status"))
	whatsappAccount := string(r.RequestCtx.QueryArgs().Peek("whatsapp_account"))
	fromDate := string(r.RequestCtx.QueryArgs().Peek("from"))
	toDate := string(r.RequestCtx.QueryArgs().Peek("to"))

	var campaigns []models.BulkMessageCampaign
	query := a.DB.Where("organization_id = ?", orgID).
		Preload("Template").
		Order("created_at DESC")

	if status != "" {
		query = query.Where("status = ?", status)
	}
	if whatsappAccount != "" {
		query = query.Where("whats_app_account = ?", whatsappAccount)
	}
	if fromDate != "" {
		if parsedFrom, err := time.Parse("2006-01-02", fromDate); err == nil {
			query = query.Where("created_at >= ?", parsedFrom)
		}
	}
	if toDate != "" {
		if parsedTo, err := time.Parse("2006-01-02", toDate); err == nil {
			// End of day
			endOfDay := parsedTo.Add(24*time.Hour - time.Nanosecond)
			query = query.Where("created_at <= ?", endOfDay)
		}
	}

	if err := query.Find(&campaigns).Error; err != nil {
		a.Log.Error("Failed to list campaigns", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to list campaigns", nil, "")
	}

	// Convert to response format
	response := make([]CampaignResponse, len(campaigns))
	for i, c := range campaigns {
		response[i] = CampaignResponse{
			ID:              c.ID,
			Name:            c.Name,
			WhatsAppAccount: c.WhatsAppAccount,
			TemplateID:      c.TemplateID,
			Status:          c.Status,
			TotalRecipients: c.TotalRecipients,
			SentCount:       c.SentCount,
			DeliveredCount:  c.DeliveredCount,
			ReadCount:       c.ReadCount,
			FailedCount:     c.FailedCount,
			ScheduledAt:     c.ScheduledAt,
			StartedAt:       c.StartedAt,
			CompletedAt:     c.CompletedAt,
			CreatedAt:       c.CreatedAt,
			UpdatedAt:       c.UpdatedAt,
		}
		if c.Template != nil {
			response[i].TemplateName = c.Template.Name
		}
	}

	return r.SendEnvelope(map[string]interface{}{
		"campaigns": response,
		"total":     len(response),
	})
}

// CreateCampaign implements campaign creation
func (a *App) CreateCampaign(r *fastglue.Request) error {
	orgID, err := a.getOrgIDFromContext(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	userID, err := a.getUserIDFromContext(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	var req CampaignRequest
	if err := r.Decode(&req, "json"); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid request body", nil, "")
	}

	// Validate template exists
	templateID, err := uuid.Parse(req.TemplateID)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid template ID", nil, "")
	}

	var template models.Template
	if err := a.DB.Where("id = ? AND organization_id = ?", templateID, orgID).First(&template).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Template not found", nil, "")
	}

	// Validate WhatsApp account exists
	var account models.WhatsAppAccount
	if err := a.DB.Where("name = ? AND organization_id = ?", req.WhatsAppAccount, orgID).First(&account).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "WhatsApp account not found", nil, "")
	}

	campaign := models.BulkMessageCampaign{
		OrganizationID:  orgID,
		WhatsAppAccount: req.WhatsAppAccount,
		Name:            req.Name,
		TemplateID:      templateID,
		Status:          models.CampaignStatusDraft,
		ScheduledAt:     req.ScheduledAt,
		CreatedBy:       userID,
	}

	if err := a.DB.Create(&campaign).Error; err != nil {
		a.Log.Error("Failed to create campaign", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to create campaign", nil, "")
	}

	a.Log.Info("Campaign created", "campaign_id", campaign.ID, "name", campaign.Name)

	return r.SendEnvelope(CampaignResponse{
		ID:              campaign.ID,
		Name:            campaign.Name,
		WhatsAppAccount: campaign.WhatsAppAccount,
		TemplateID:      campaign.TemplateID,
		TemplateName:    template.Name,
		Status:          campaign.Status,
		TotalRecipients: campaign.TotalRecipients,
		SentCount:       campaign.SentCount,
		DeliveredCount:  campaign.DeliveredCount,
		FailedCount:     campaign.FailedCount,
		ScheduledAt:     campaign.ScheduledAt,
		CreatedAt:       campaign.CreatedAt,
		UpdatedAt:       campaign.UpdatedAt,
	})
}

// GetCampaign implements getting a single campaign
func (a *App) GetCampaign(r *fastglue.Request) error {
	orgID, err := a.getOrgIDFromContext(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	campaignID := r.RequestCtx.UserValue("id").(string)
	id, err := uuid.Parse(campaignID)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid campaign ID", nil, "")
	}

	var campaign models.BulkMessageCampaign
	if err := a.DB.Where("id = ? AND organization_id = ?", id, orgID).
		Preload("Template").
		First(&campaign).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Campaign not found", nil, "")
	}

	response := CampaignResponse{
		ID:              campaign.ID,
		Name:            campaign.Name,
		WhatsAppAccount: campaign.WhatsAppAccount,
		TemplateID:      campaign.TemplateID,
		Status:          campaign.Status,
		TotalRecipients: campaign.TotalRecipients,
		SentCount:       campaign.SentCount,
		DeliveredCount:  campaign.DeliveredCount,
		FailedCount:     campaign.FailedCount,
		ScheduledAt:     campaign.ScheduledAt,
		StartedAt:       campaign.StartedAt,
		CompletedAt:     campaign.CompletedAt,
		CreatedAt:       campaign.CreatedAt,
		UpdatedAt:       campaign.UpdatedAt,
	}
	if campaign.Template != nil {
		response.TemplateName = campaign.Template.Name
	}

	return r.SendEnvelope(response)
}

// UpdateCampaign implements campaign update
func (a *App) UpdateCampaign(r *fastglue.Request) error {
	orgID, err := a.getOrgIDFromContext(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	campaignID := r.RequestCtx.UserValue("id").(string)
	id, err := uuid.Parse(campaignID)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid campaign ID", nil, "")
	}

	var campaign models.BulkMessageCampaign
	if err := a.DB.Where("id = ? AND organization_id = ?", id, orgID).First(&campaign).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Campaign not found", nil, "")
	}

	// Only allow updates to draft campaigns
	if campaign.Status != models.CampaignStatusDraft {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Can only update draft campaigns", nil, "")
	}

	var req CampaignRequest
	if err := r.Decode(&req, "json"); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid request body", nil, "")
	}

	// Update fields
	updates := map[string]interface{}{
		"name":         req.Name,
		"scheduled_at": req.ScheduledAt,
	}

	if req.TemplateID != "" {
		templateID, err := uuid.Parse(req.TemplateID)
		if err != nil {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid template ID", nil, "")
		}
		updates["template_id"] = templateID
	}

	if req.WhatsAppAccount != "" {
		updates["whats_app_account"] = req.WhatsAppAccount
	}

	if err := a.DB.Model(&campaign).Updates(updates).Error; err != nil {
		a.Log.Error("Failed to update campaign", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to update campaign", nil, "")
	}

	// Reload campaign
	a.DB.Where("id = ?", id).Preload("Template").First(&campaign)

	response := CampaignResponse{
		ID:              campaign.ID,
		Name:            campaign.Name,
		WhatsAppAccount: campaign.WhatsAppAccount,
		TemplateID:      campaign.TemplateID,
		Status:          campaign.Status,
		TotalRecipients: campaign.TotalRecipients,
		SentCount:       campaign.SentCount,
		DeliveredCount:  campaign.DeliveredCount,
		FailedCount:     campaign.FailedCount,
		ScheduledAt:     campaign.ScheduledAt,
		CreatedAt:       campaign.CreatedAt,
		UpdatedAt:       campaign.UpdatedAt,
	}
	if campaign.Template != nil {
		response.TemplateName = campaign.Template.Name
	}

	return r.SendEnvelope(response)
}

// DeleteCampaign implements campaign deletion
func (a *App) DeleteCampaign(r *fastglue.Request) error {
	orgID, err := a.getOrgIDFromContext(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	campaignID := r.RequestCtx.UserValue("id").(string)
	id, err := uuid.Parse(campaignID)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid campaign ID", nil, "")
	}

	var campaign models.BulkMessageCampaign
	if err := a.DB.Where("id = ? AND organization_id = ?", id, orgID).First(&campaign).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Campaign not found", nil, "")
	}

	// Don't allow deletion of running campaigns
	if campaign.Status == models.CampaignStatusProcessing || campaign.Status == models.CampaignStatusQueued {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Cannot delete running campaign", nil, "")
	}

	// Delete recipients first
	if err := a.DB.Where("campaign_id = ?", id).Delete(&models.BulkMessageRecipient{}).Error; err != nil {
		a.Log.Error("Failed to delete campaign recipients", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to delete campaign", nil, "")
	}

	// Delete campaign
	if err := a.DB.Delete(&campaign).Error; err != nil {
		a.Log.Error("Failed to delete campaign", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to delete campaign", nil, "")
	}

	a.Log.Info("Campaign deleted", "campaign_id", id)

	return r.SendEnvelope(map[string]interface{}{
		"message": "Campaign deleted successfully",
	})
}

// StartCampaign implements starting a campaign
func (a *App) StartCampaign(r *fastglue.Request) error {
	orgID, err := a.getOrgIDFromContext(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	campaignID := r.RequestCtx.UserValue("id").(string)
	id, err := uuid.Parse(campaignID)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid campaign ID", nil, "")
	}

	var campaign models.BulkMessageCampaign
	if err := a.DB.Where("id = ? AND organization_id = ?", id, orgID).First(&campaign).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Campaign not found", nil, "")
	}

	// Check if campaign can be started
	if campaign.Status != models.CampaignStatusDraft && campaign.Status != models.CampaignStatusScheduled && campaign.Status != models.CampaignStatusPaused {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Campaign cannot be started in current state", nil, "")
	}

	// Get all pending recipients
	var recipients []models.BulkMessageRecipient
	if err := a.DB.Where("campaign_id = ? AND status = ?", id, models.MessageStatusPending).Find(&recipients).Error; err != nil {
		a.Log.Error("Failed to load recipients", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to load recipients", nil, "")
	}

	if len(recipients) == 0 {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Campaign has no pending recipients", nil, "")
	}

	// Update status to processing
	now := time.Now()
	updates := map[string]interface{}{
		"status":     models.CampaignStatusProcessing,
		"started_at": now,
	}

	if err := a.DB.Model(&campaign).Updates(updates).Error; err != nil {
		a.Log.Error("Failed to start campaign", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to start campaign", nil, "")
	}

	a.Log.Info("Campaign started", "campaign_id", id, "recipients", len(recipients))

	// Enqueue all recipients as individual jobs for parallel processing
	jobs := make([]*queue.RecipientJob, len(recipients))
	for i, recipient := range recipients {
		jobs[i] = &queue.RecipientJob{
			CampaignID:     id,
			RecipientID:    recipient.ID,
			OrganizationID: orgID,
			PhoneNumber:    recipient.PhoneNumber,
			RecipientName:  recipient.RecipientName,
			TemplateParams: recipient.TemplateParams,
		}
	}

	if err := a.Queue.EnqueueRecipients(r.RequestCtx, jobs); err != nil {
		a.Log.Error("Failed to enqueue recipients", "error", err)
		// Revert status on failure
		a.DB.Model(&campaign).Update("status", models.CampaignStatusDraft)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to queue recipients", nil, "")
	}

	a.Log.Info("Recipients enqueued for processing", "campaign_id", id, "count", len(jobs))

	return r.SendEnvelope(map[string]interface{}{
		"message": "Campaign started",
		"status":  models.CampaignStatusProcessing,
	})
}

// PauseCampaign implements pausing a campaign
func (a *App) PauseCampaign(r *fastglue.Request) error {
	orgID, err := a.getOrgIDFromContext(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	campaignID := r.RequestCtx.UserValue("id").(string)
	id, err := uuid.Parse(campaignID)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid campaign ID", nil, "")
	}

	var campaign models.BulkMessageCampaign
	if err := a.DB.Where("id = ? AND organization_id = ?", id, orgID).First(&campaign).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Campaign not found", nil, "")
	}

	if campaign.Status != models.CampaignStatusProcessing && campaign.Status != models.CampaignStatusQueued {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Campaign is not running", nil, "")
	}

	if err := a.DB.Model(&campaign).Update("status", models.CampaignStatusPaused).Error; err != nil {
		a.Log.Error("Failed to pause campaign", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to pause campaign", nil, "")
	}

	a.Log.Info("Campaign paused", "campaign_id", id)

	return r.SendEnvelope(map[string]interface{}{
		"message": "Campaign paused",
		"status":  models.CampaignStatusPaused,
	})
}

// CancelCampaign implements cancelling a campaign
func (a *App) CancelCampaign(r *fastglue.Request) error {
	orgID, err := a.getOrgIDFromContext(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	campaignID := r.RequestCtx.UserValue("id").(string)
	id, err := uuid.Parse(campaignID)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid campaign ID", nil, "")
	}

	var campaign models.BulkMessageCampaign
	if err := a.DB.Where("id = ? AND organization_id = ?", id, orgID).First(&campaign).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Campaign not found", nil, "")
	}

	if campaign.Status == models.CampaignStatusCompleted || campaign.Status == models.CampaignStatusCancelled {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Campaign already finished", nil, "")
	}

	if err := a.DB.Model(&campaign).Update("status", models.CampaignStatusCancelled).Error; err != nil {
		a.Log.Error("Failed to cancel campaign", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to cancel campaign", nil, "")
	}

	a.Log.Info("Campaign cancelled", "campaign_id", id)

	return r.SendEnvelope(map[string]interface{}{
		"message": "Campaign cancelled",
		"status":  models.CampaignStatusCancelled,
	})
}

// RetryFailed retries sending to all failed recipients
func (a *App) RetryFailed(r *fastglue.Request) error {
	orgID, err := a.getOrgIDFromContext(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	campaignID := r.RequestCtx.UserValue("id").(string)
	id, err := uuid.Parse(campaignID)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid campaign ID", nil, "")
	}

	var campaign models.BulkMessageCampaign
	if err := a.DB.Where("id = ? AND organization_id = ?", id, orgID).First(&campaign).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Campaign not found", nil, "")
	}

	// Only allow retry on completed or paused campaigns
	if campaign.Status != models.CampaignStatusCompleted && campaign.Status != models.CampaignStatusPaused && campaign.Status != models.CampaignStatusFailed {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Can only retry failed messages on completed, paused, or failed campaigns", nil, "")
	}

	// Get failed recipients
	var failedRecipients []models.BulkMessageRecipient
	if err := a.DB.Where("campaign_id = ? AND status = ?", id, models.MessageStatusFailed).Find(&failedRecipients).Error; err != nil {
		a.Log.Error("Failed to load failed recipients", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to load failed recipients", nil, "")
	}

	if len(failedRecipients) == 0 {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "No failed messages to retry", nil, "")
	}

	// Reset failed recipients to pending
	if err := a.DB.Model(&models.BulkMessageRecipient{}).
		Where("campaign_id = ? AND status = ?", id, models.MessageStatusFailed).
		Updates(map[string]interface{}{
			"status":        models.MessageStatusPending,
			"error_message": "",
		}).Error; err != nil {
		a.Log.Error("Failed to reset failed recipients", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to reset failed recipients", nil, "")
	}

	// Reset failed messages in messages table to pending
	if err := a.DB.Model(&models.Message{}).
		Where("metadata->>'campaign_id' = ? AND status = ?", id.String(), models.MessageStatusFailed).
		Updates(map[string]interface{}{
			"status":        models.MessageStatusPending,
			"error_message": "",
		}).Error; err != nil {
		a.Log.Error("Failed to reset failed messages", "error", err)
	}

	// Recalculate campaign stats from messages table
	a.recalculateCampaignStats(id)

	// Update campaign status to processing
	if err := a.DB.Model(&campaign).Update("status", models.CampaignStatusProcessing).Error; err != nil {
		a.Log.Error("Failed to update campaign status", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to update campaign", nil, "")
	}

	a.Log.Info("Retrying failed messages", "campaign_id", id, "failed_count", len(failedRecipients))

	// Enqueue failed recipients as individual jobs for parallel processing
	jobs := make([]*queue.RecipientJob, len(failedRecipients))
	for i, recipient := range failedRecipients {
		jobs[i] = &queue.RecipientJob{
			CampaignID:     id,
			RecipientID:    recipient.ID,
			OrganizationID: orgID,
			PhoneNumber:    recipient.PhoneNumber,
			RecipientName:  recipient.RecipientName,
			TemplateParams: recipient.TemplateParams,
		}
	}

	if err := a.Queue.EnqueueRecipients(r.RequestCtx, jobs); err != nil {
		a.Log.Error("Failed to enqueue recipients for retry", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to queue recipients", nil, "")
	}

	a.Log.Info("Failed recipients enqueued for retry", "campaign_id", id, "count", len(jobs))

	return r.SendEnvelope(map[string]interface{}{
		"message":     "Retrying failed messages",
		"retry_count": len(failedRecipients),
		"status":      models.CampaignStatusProcessing,
	})
}

// ImportRecipients implements adding recipients to a campaign
func (a *App) ImportRecipients(r *fastglue.Request) error {
	orgID, err := a.getOrgIDFromContext(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	campaignID := r.RequestCtx.UserValue("id").(string)
	id, err := uuid.Parse(campaignID)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid campaign ID", nil, "")
	}

	var campaign models.BulkMessageCampaign
	if err := a.DB.Where("id = ? AND organization_id = ?", id, orgID).First(&campaign).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Campaign not found", nil, "")
	}

	if campaign.Status != models.CampaignStatusDraft {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Can only add recipients to draft campaigns", nil, "")
	}

	var req struct {
		Recipients []RecipientRequest `json:"recipients" validate:"required"`
	}
	if err := r.Decode(&req, "json"); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid request body", nil, "")
	}

	// Create recipients
	recipients := make([]models.BulkMessageRecipient, len(req.Recipients))
	for i, rec := range req.Recipients {
		recipients[i] = models.BulkMessageRecipient{
			CampaignID:     id,
			PhoneNumber:    rec.PhoneNumber,
			RecipientName:  rec.RecipientName,
			TemplateParams: models.JSONB(rec.TemplateParams),
			Status:         models.MessageStatusPending,
		}
	}

	if err := a.DB.Create(&recipients).Error; err != nil {
		a.Log.Error("Failed to add recipients", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to add recipients", nil, "")
	}

	// Update total recipients count
	var totalCount int64
	a.DB.Model(&models.BulkMessageRecipient{}).Where("campaign_id = ?", id).Count(&totalCount)
	a.DB.Model(&campaign).Update("total_recipients", totalCount)

	a.Log.Info("Recipients added to campaign", "campaign_id", id, "count", len(req.Recipients))

	return r.SendEnvelope(map[string]interface{}{
		"message":          "Recipients added successfully",
		"added_count":      len(req.Recipients),
		"total_recipients": totalCount,
	})
}

// GetCampaignRecipients implements listing campaign recipients
func (a *App) GetCampaignRecipients(r *fastglue.Request) error {
	orgID, err := a.getOrgIDFromContext(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	campaignID := r.RequestCtx.UserValue("id").(string)
	id, err := uuid.Parse(campaignID)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid campaign ID", nil, "")
	}

	// Verify campaign belongs to org
	var campaign models.BulkMessageCampaign
	if err := a.DB.Where("id = ? AND organization_id = ?", id, orgID).First(&campaign).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Campaign not found", nil, "")
	}

	var recipients []models.BulkMessageRecipient
	if err := a.DB.Where("campaign_id = ?", id).Order("created_at ASC").Find(&recipients).Error; err != nil {
		a.Log.Error("Failed to list recipients", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to list recipients", nil, "")
	}

	return r.SendEnvelope(map[string]interface{}{
		"recipients": recipients,
		"total":      len(recipients),
	})
}

// getUserIDFromContext extracts user ID from request context (set by auth middleware)
func (a *App) getUserIDFromContext(r *fastglue.Request) (uuid.UUID, error) {
	userIDVal := r.RequestCtx.UserValue("user_id")
	if userIDVal == nil {
		return uuid.Nil, fasthttp.ErrNoMultipartForm
	}
	// The middleware stores uuid.UUID directly, not as string
	userID, ok := userIDVal.(uuid.UUID)
	if !ok {
		return uuid.Nil, fasthttp.ErrNoMultipartForm
	}
	return userID, nil
}

// incrementCampaignStat increments the appropriate campaign counter based on status
func (a *App) incrementCampaignStat(campaignID string, status string) {
	campaignUUID, err := uuid.Parse(campaignID)
	if err != nil {
		a.Log.Error("Invalid campaign ID for stats update", "campaign_id", campaignID)
		return
	}

	var column string
	switch models.MessageStatus(status) {
	case models.MessageStatusDelivered:
		column = "delivered_count"
	case models.MessageStatusRead:
		column = "read_count"
	case models.MessageStatusFailed:
		column = "failed_count"
	default:
		// sent is already counted during processCampaign
		return
	}

	if err := a.DB.Model(&models.BulkMessageCampaign{}).
		Where("id = ?", campaignUUID).
		Update(column, gorm.Expr(column+" + 1")).Error; err != nil {
		a.Log.Error("Failed to increment campaign stat", "error", err, "campaign_id", campaignID, "column", column)
		return
	}

	// Broadcast stats update via WebSocket
	if a.WSHub != nil {
		var campaign models.BulkMessageCampaign
		if err := a.DB.Where("id = ?", campaignUUID).First(&campaign).Error; err == nil {
			a.WSHub.BroadcastToOrg(campaign.OrganizationID, websocket.WSMessage{
				Type: websocket.TypeCampaignStatsUpdate,
				Payload: map[string]interface{}{
					"campaign_id":     campaignID,
					"sent_count":      campaign.SentCount,
					"delivered_count": campaign.DeliveredCount,
					"read_count":      campaign.ReadCount,
					"failed_count":    campaign.FailedCount,
				},
			})
		}
	}
}

// recalculateCampaignStats recalculates all campaign stats from messages table
func (a *App) recalculateCampaignStats(campaignID uuid.UUID) {
	var stats struct {
		Sent      int64
		Delivered int64
		Read      int64
		Failed    int64
	}

	a.DB.Model(&models.Message{}).
		Where("metadata->>'campaign_id' = ?", campaignID.String()).
		Select(`
			COUNT(CASE WHEN status IN ('sent','delivered','read') THEN 1 END) as sent,
			COUNT(CASE WHEN status IN ('delivered','read') THEN 1 END) as delivered,
			COUNT(CASE WHEN status = 'read' THEN 1 END) as read,
			COUNT(CASE WHEN status = 'failed' THEN 1 END) as failed
		`).Scan(&stats)

	if err := a.DB.Model(&models.BulkMessageCampaign{}).Where("id = ?", campaignID).
		Updates(map[string]interface{}{
			"sent_count":      stats.Sent,
			"delivered_count": stats.Delivered,
			"read_count":      stats.Read,
			"failed_count":    stats.Failed,
		}).Error; err != nil {
		a.Log.Error("Failed to recalculate campaign stats", "error", err, "campaign_id", campaignID)
	}
}

