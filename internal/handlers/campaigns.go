package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nikyjain/whatomate/internal/contactutil"
	"github.com/nikyjain/whatomate/internal/models"
	"github.com/nikyjain/whatomate/internal/queue"
	"github.com/nikyjain/whatomate/internal/utils"
	"github.com/nikyjain/whatomate/internal/websocket"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CampaignRequest represents campaign create/update request
type CampaignRequest struct {
	Name            string     `json:"name" validate:"required"`
	WhatsAppAccount string     `json:"whatsapp_account" validate:"required"`
	TemplateID      string     `json:"template_id" validate:"required"`
	HeaderMediaURL  string     `json:"header_media_url"`
	HeaderMediaID   string     `json:"header_media_id"`
	ScheduledAt     *time.Time `json:"scheduled_at"`
}

// CampaignResponse represents campaign in API responses
type CampaignResponse struct {
	ID                  uuid.UUID             `json:"id"`
	Name                string                `json:"name"`
	WhatsAppAccount     string                `json:"whatsapp_account"`
	TemplateID          uuid.UUID             `json:"template_id"`
	TemplateName        string                `json:"template_name,omitempty"`
	SourceType          string                `json:"source_type,omitempty"`
	SourceID            *uuid.UUID            `json:"source_id,omitempty"`
	HeaderMediaURL      string                `json:"header_media_url,omitempty"`
	HeaderMediaID       string                `json:"header_media_id,omitempty"`
	HeaderMediaFilename string                `json:"header_media_filename,omitempty"`
	HeaderMediaMimeType string                `json:"header_media_mime_type,omitempty"`
	Status              models.CampaignStatus `json:"status"`
	TotalRecipients     int                   `json:"total_recipients"`
	SentCount           int                   `json:"sent_count"`
	DeliveredCount      int                   `json:"delivered_count"`
	ReadCount           int                   `json:"read_count"`
	FailedCount         int                   `json:"failed_count"`
	ScheduledAt         *time.Time            `json:"scheduled_at,omitempty"`
	StartedAt           *time.Time            `json:"started_at,omitempty"`
	CompletedAt         *time.Time            `json:"completed_at,omitempty"`
	CreatedByName       string                `json:"created_by_name,omitempty"`
	UpdatedByName       string                `json:"updated_by_name,omitempty"`
	CreatedAt           time.Time             `json:"created_at"`
	UpdatedAt           time.Time             `json:"updated_at"`
}

// RecipientRequest represents recipient import request
type RecipientRequest struct {
	PhoneNumber    string         `json:"phone_number" validate:"required"`
	RecipientName  string         `json:"recipient_name"`
	TemplateParams map[string]any `json:"template_params"`
	// HeaderParams carries the value for a TEXT-header variable (max 1 per
	// Meta), keyed by the variable's name. Kept separate from TemplateParams
	// so a positional header {{1}} doesn't collide with body {{1}}.
	HeaderParams map[string]any `json:"header_params"`
}

type CampaignRecipientImportRequest struct {
	Recipients []RecipientRequest `json:"recipients"`
	ContactIDs []string           `json:"contact_ids"`
	TagNames   []string           `json:"tag_names"`
}

const campaignRecipientCreateBatchSize = 500

// ListCampaigns implements campaign listing
func (a *App) ListCampaigns(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	pg := parsePagination(r)

	// Get query params
	status := string(r.RequestCtx.QueryArgs().Peek("status"))
	whatsappAccount := string(r.RequestCtx.QueryArgs().Peek("whatsapp_account"))
	search := string(r.RequestCtx.QueryArgs().Peek("search"))

	baseQuery := a.DB.Where("organization_id = ?", orgID)

	if search != "" {
		baseQuery = baseQuery.Where("name ILIKE ?", "%"+search+"%")
	}

	if status != "" {
		baseQuery = baseQuery.Where("status = ?", status)
	}
	if whatsappAccount != "" {
		baseQuery = baseQuery.Where("whats_app_account = ?", whatsappAccount)
	}
	if from, ok := parseDateParam(r, "from"); ok {
		baseQuery = baseQuery.Where("created_at >= ?", from)
	}
	if to, ok := parseDateParam(r, "to"); ok {
		baseQuery = baseQuery.Where("created_at <= ?", endOfDay(to))
	}

	// Get total count
	var total int64
	baseQuery.Model(&models.BulkMessageCampaign{}).Count(&total)

	var campaigns []models.BulkMessageCampaign
	if err := pg.Apply(baseQuery.
		Preload("Template").
		Order("created_at DESC")).
		Find(&campaigns).Error; err != nil {
		a.Log.Error("Failed to list campaigns", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to list campaigns", nil, "")
	}

	// Convert to response format
	response := make([]CampaignResponse, len(campaigns))
	for i, c := range campaigns {
		response[i] = CampaignResponse{
			ID:                  c.ID,
			Name:                c.Name,
			WhatsAppAccount:     c.WhatsAppAccount,
			TemplateID:          c.TemplateID,
			SourceType:          c.SourceType,
			SourceID:            c.SourceID,
			HeaderMediaURL:      c.HeaderMediaURL,
			HeaderMediaID:       c.HeaderMediaID,
			HeaderMediaFilename: c.HeaderMediaFilename,
			HeaderMediaMimeType: c.HeaderMediaMimeType,
			Status:              c.Status,
			TotalRecipients:     c.TotalRecipients,
			SentCount:           c.SentCount,
			DeliveredCount:      c.DeliveredCount,
			ReadCount:           c.ReadCount,
			FailedCount:         c.FailedCount,
			ScheduledAt:         c.ScheduledAt,
			StartedAt:           c.StartedAt,
			CompletedAt:         c.CompletedAt,
			CreatedAt:           c.CreatedAt,
			UpdatedAt:           c.UpdatedAt,
		}
		if c.Template != nil {
			response[i].TemplateName = c.Template.Name
		}
	}

	return r.SendEnvelope(listEnvelope("campaigns", response, total, pg))
}

// CreateCampaign implements campaign creation
func (a *App) CreateCampaign(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	var req CampaignRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}

	// Validate template exists
	templateID, err := uuid.Parse(req.TemplateID)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid template ID", nil, "")
	}

	template, err := findByIDAndOrg[models.Template](a.DB, r, templateID, orgID, "Template")
	if err != nil {
		return nil
	}

	if _, err := a.resolveWhatsAppAccount(orgID, req.WhatsAppAccount); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "WhatsApp account not found", nil, "")
	}

	campaign := models.BulkMessageCampaign{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  orgID,
		WhatsAppAccount: req.WhatsAppAccount,
		Name:            req.Name,
		TemplateID:      templateID,
		HeaderMediaURL:  strings.TrimSpace(req.HeaderMediaURL),
		HeaderMediaID:   req.HeaderMediaID,
		Status:          models.CampaignStatusDraft,
		ScheduledAt:     req.ScheduledAt,
		CreatedBy:       userID,
		UpdatedByID:     &userID,
	}

	if campaign.HeaderMediaURL != "" {
		if !campaignTemplateNeedsMedia(template) {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Template does not have a media header", nil, "")
		}
		if err := a.populateCampaignHeaderMediaFromURL(r.RequestCtx, &campaign); err != nil {
			return r.SendErrorEnvelope(fasthttp.StatusBadGateway, err.Error(), nil, "")
		}
	}

	if err := a.DB.Create(&campaign).Error; err != nil {
		a.Log.Error("Failed to create campaign", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to create campaign", nil, "")
	}

	a.logAudit(orgID, userID,
		"campaign", campaign.ID, models.AuditActionCreated, nil, &campaign)

	a.Log.Info("Campaign created", "campaign_id", campaign.ID, "name", campaign.Name)

	return r.SendEnvelope(CampaignResponse{
		ID:                  campaign.ID,
		Name:                campaign.Name,
		WhatsAppAccount:     campaign.WhatsAppAccount,
		TemplateID:          campaign.TemplateID,
		SourceType:          campaign.SourceType,
		SourceID:            campaign.SourceID,
		TemplateName:        template.Name,
		HeaderMediaURL:      campaign.HeaderMediaURL,
		HeaderMediaID:       campaign.HeaderMediaID,
		HeaderMediaFilename: campaign.HeaderMediaFilename,
		HeaderMediaMimeType: campaign.HeaderMediaMimeType,
		Status:              campaign.Status,
		TotalRecipients:     campaign.TotalRecipients,
		SentCount:           campaign.SentCount,
		DeliveredCount:      campaign.DeliveredCount,
		ReadCount:           campaign.ReadCount,
		FailedCount:         campaign.FailedCount,
		ScheduledAt:         campaign.ScheduledAt,
		CreatedAt:           campaign.CreatedAt,
		UpdatedAt:           campaign.UpdatedAt,
	})
}

// GetCampaign implements getting a single campaign
func (a *App) GetCampaign(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	id, err := parsePathUUID(r, "id", "campaign")
	if err != nil {
		return nil
	}

	var campaign models.BulkMessageCampaign
	if err := a.DB.Where("id = ? AND organization_id = ?", id, orgID).
		Preload("Template").
		Preload("Creator").
		Preload("UpdatedBy").
		First(&campaign).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Campaign not found", nil, "")
	}

	response := CampaignResponse{
		ID:                  campaign.ID,
		Name:                campaign.Name,
		WhatsAppAccount:     campaign.WhatsAppAccount,
		TemplateID:          campaign.TemplateID,
		SourceType:          campaign.SourceType,
		SourceID:            campaign.SourceID,
		HeaderMediaURL:      campaign.HeaderMediaURL,
		HeaderMediaID:       campaign.HeaderMediaID,
		HeaderMediaFilename: campaign.HeaderMediaFilename,
		HeaderMediaMimeType: campaign.HeaderMediaMimeType,
		Status:              campaign.Status,
		TotalRecipients:     campaign.TotalRecipients,
		SentCount:           campaign.SentCount,
		DeliveredCount:      campaign.DeliveredCount,
		ReadCount:           campaign.ReadCount,
		FailedCount:         campaign.FailedCount,
		ScheduledAt:         campaign.ScheduledAt,
		StartedAt:           campaign.StartedAt,
		CompletedAt:         campaign.CompletedAt,
		CreatedAt:           campaign.CreatedAt,
		UpdatedAt:           campaign.UpdatedAt,
	}
	if campaign.Template != nil {
		response.TemplateName = campaign.Template.Name
	}
	if campaign.Creator != nil {
		response.CreatedByName = campaign.Creator.FullName
	}
	if campaign.UpdatedBy != nil {
		response.UpdatedByName = campaign.UpdatedBy.FullName
	}

	return r.SendEnvelope(response)
}

// UpdateCampaign implements campaign update
func (a *App) UpdateCampaign(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	id, err := parsePathUUID(r, "id", "campaign")
	if err != nil {
		return nil
	}

	campaign, err := findByIDAndOrg[models.BulkMessageCampaign](a.DB, r, id, orgID, "Campaign")
	if err != nil {
		return nil
	}
	if campaign.SourceType == models.CampaignSourceRSVPReminder {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "RSVP reminder campaigns cannot be edited", nil, "")
	}

	// Only allow updates to draft campaigns
	if campaign.Status != models.CampaignStatusDraft {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Can only update draft campaigns", nil, "")
	}

	oldCampaign := *campaign

	var req CampaignRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}

	templateID := campaign.TemplateID
	if req.TemplateID != "" {
		parsedTemplateID, err := uuid.Parse(req.TemplateID)
		if err != nil {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid template ID", nil, "")
		}
		templateID = parsedTemplateID
	}

	template, err := findByIDAndOrg[models.Template](a.DB, r, templateID, orgID, "Template")
	if err != nil {
		return nil
	}

	accountName := campaign.WhatsAppAccount
	if req.WhatsAppAccount != "" {
		accountName = req.WhatsAppAccount
	}
	if _, err := a.resolveWhatsAppAccount(orgID, accountName); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "WhatsApp account not found", nil, "")
	}

	// Update fields
	updates := map[string]any{
		"name":          req.Name,
		"scheduled_at":  req.ScheduledAt,
		"updated_by_id": userID,
	}

	if req.TemplateID != "" {
		updates["template_id"] = templateID
	}

	if req.WhatsAppAccount != "" {
		updates["whats_app_account"] = req.WhatsAppAccount
	}

	if trimmedURL := strings.TrimSpace(req.HeaderMediaURL); trimmedURL != "" {
		if !campaignTemplateNeedsMedia(template) {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Template does not have a media header", nil, "")
		}

		previewCampaign := *campaign
		previewCampaign.TemplateID = templateID
		previewCampaign.WhatsAppAccount = accountName
		previewCampaign.HeaderMediaURL = trimmedURL
		if err := a.populateCampaignHeaderMediaFromURL(r.RequestCtx, &previewCampaign); err != nil {
			return r.SendErrorEnvelope(fasthttp.StatusBadGateway, err.Error(), nil, "")
		}

		updates["header_media_url"] = previewCampaign.HeaderMediaURL
		updates["header_media_id"] = previewCampaign.HeaderMediaID
		updates["header_media_filename"] = previewCampaign.HeaderMediaFilename
		updates["header_media_mime_type"] = previewCampaign.HeaderMediaMimeType
		updates["header_media_local_path"] = previewCampaign.HeaderMediaLocalPath
	}

	if err := a.DB.Model(campaign).Updates(updates).Error; err != nil {
		a.Log.Error("Failed to update campaign", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to update campaign", nil, "")
	}

	// Reload campaign
	a.DB.Where("id = ?", id).Preload("Template").Preload("Creator").Preload("UpdatedBy").First(campaign)

	a.logAudit(orgID, userID,
		"campaign", campaign.ID, models.AuditActionUpdated, &oldCampaign, campaign)

	response := CampaignResponse{
		ID:                  campaign.ID,
		Name:                campaign.Name,
		WhatsAppAccount:     campaign.WhatsAppAccount,
		TemplateID:          campaign.TemplateID,
		SourceType:          campaign.SourceType,
		SourceID:            campaign.SourceID,
		HeaderMediaURL:      campaign.HeaderMediaURL,
		HeaderMediaID:       campaign.HeaderMediaID,
		HeaderMediaFilename: campaign.HeaderMediaFilename,
		HeaderMediaMimeType: campaign.HeaderMediaMimeType,
		Status:              campaign.Status,
		TotalRecipients:     campaign.TotalRecipients,
		SentCount:           campaign.SentCount,
		DeliveredCount:      campaign.DeliveredCount,
		ReadCount:           campaign.ReadCount,
		FailedCount:         campaign.FailedCount,
		ScheduledAt:         campaign.ScheduledAt,
		CreatedAt:           campaign.CreatedAt,
		UpdatedAt:           campaign.UpdatedAt,
	}
	if campaign.Template != nil {
		response.TemplateName = campaign.Template.Name
	}
	if campaign.Creator != nil {
		response.CreatedByName = campaign.Creator.FullName
	}
	if campaign.UpdatedBy != nil {
		response.UpdatedByName = campaign.UpdatedBy.FullName
	}

	return r.SendEnvelope(response)
}

func campaignTemplateNeedsMedia(template *models.Template) bool {
	if template == nil {
		return false
	}
	switch template.HeaderType {
	case "IMAGE", "VIDEO", "DOCUMENT":
		return true
	default:
		return false
	}
}

func (a *App) populateCampaignHeaderMediaFromURL(ctx context.Context, campaign *models.BulkMessageCampaign) error {
	headerMediaURL := strings.TrimSpace(campaign.HeaderMediaURL)
	if headerMediaURL == "" {
		return nil
	}
	data, filename, mimeType, err := a.downloadCampaignHeaderMedia(ctx, headerMediaURL)
	if err != nil {
		return err
	}

	localPath, err := a.saveCampaignMedia(campaign.ID.String(), data, mimeType)
	if err != nil {
		a.Log.Error("Failed to save campaign media locally", "error", err, "campaign_id", campaign.ID, "header_media_url", headerMediaURL)
	}

	campaign.HeaderMediaID = ""
	campaign.HeaderMediaFilename = filename
	campaign.HeaderMediaMimeType = mimeType
	campaign.HeaderMediaLocalPath = localPath

	return nil
}

func (a *App) downloadCampaignHeaderMedia(ctx context.Context, headerMediaURL string) ([]byte, string, string, error) {
	if _, err := url.ParseRequestURI(headerMediaURL); err != nil {
		return nil, "", "", fmt.Errorf("invalid header media url")
	}

	httpClient := a.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	// Callers may pass a *fasthttp.RequestCtx, whose Done() is only valid
	// while a live server owns it (it nil-derefs in tests and after the
	// handler returns). Detach from its cancellation and bound the download
	// with our own timeout instead.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 60*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, headerMediaURL, nil)
	if err != nil {
		return nil, "", "", fmt.Errorf("prepare header media download: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, "", "", fmt.Errorf("download header media: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, "", "", fmt.Errorf("header media url returned status %d", resp.StatusCode)
	}

	const maxMediaSize = 16 << 20
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxMediaSize+1))
	if err != nil {
		return nil, "", "", fmt.Errorf("read header media: %w", err)
	}
	if len(data) > maxMediaSize {
		return nil, "", "", fmt.Errorf("header media file too large. Maximum size is 16MB")
	}

	filename := campaignHeaderMediaFilename(headerMediaURL)
	mimeType := detectCampaignMediaMimeType(filename, resp.Header.Get("Content-Type"), data)
	if !isAllowedCampaignMediaMimeType(mimeType) {
		return nil, "", "", fmt.Errorf("unsupported header media type: %s", mimeType)
	}
	if filepath.Ext(filename) == "" {
		if ext := getExtensionFromMimeType(mimeType); ext != "" {
			filename += ext
		}
	}

	return data, sanitizeFilename(filename), mimeType, nil
}

func campaignHeaderMediaFilename(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err == nil {
		name := path.Base(parsed.Path)
		if name != "" && name != "." && name != "/" {
			return name
		}
	}
	return "header-media"
}

// DeleteCampaign implements campaign deletion
func (a *App) DeleteCampaign(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	id, err := parsePathUUID(r, "id", "campaign")
	if err != nil {
		return nil
	}

	campaign, err := findByIDAndOrg[models.BulkMessageCampaign](a.DB, r, id, orgID, "Campaign")
	if err != nil {
		return nil
	}
	if campaign.SourceType == models.CampaignSourceRSVPReminder {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "RSVP reminder campaigns are retained for audit", nil, "")
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
	if err := a.DB.Delete(campaign).Error; err != nil {
		a.Log.Error("Failed to delete campaign", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to delete campaign", nil, "")
	}

	a.logAudit(orgID, userID,
		"campaign", id, models.AuditActionDeleted, campaign, nil)

	a.Log.Info("Campaign deleted", "campaign_id", id)

	return r.SendEnvelope(map[string]any{
		"message": "Campaign deleted successfully",
	})
}

// StartCampaign implements starting a campaign
func (a *App) StartCampaign(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	id, err := parsePathUUID(r, "id", "campaign")
	if err != nil {
		return nil
	}

	var campaign models.BulkMessageCampaign
	if err := a.DB.Where("id = ? AND organization_id = ?", id, orgID).
		Preload("Template").
		First(&campaign).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Campaign not found", nil, "")
	}

	// Check if campaign can be started
	if campaign.Status != models.CampaignStatusDraft && campaign.Status != models.CampaignStatusScheduled && campaign.Status != models.CampaignStatusPaused {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Campaign cannot be started in current state", nil, "")
	}

	if err := a.validateCampaignReadyForStart(&campaign); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, err.Error(), nil, "")
	}

	recipients, err := a.loadPendingCampaignRecipients(campaign.ID)
	if err != nil {
		a.Log.Error("Failed to load recipients", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to load recipients", nil, "")
	}
	if len(recipients) == 0 {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Campaign has no pending recipients", nil, "")
	}

	now := time.Now()
	if campaign.ScheduledAt != nil && campaign.ScheduledAt.After(now) {
		if err := a.DB.Model(&campaign).Updates(map[string]interface{}{
			"status": models.CampaignStatusScheduled,
		}).Error; err != nil {
			a.Log.Error("Failed to schedule campaign", "error", err)
			return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to schedule campaign", nil, "")
		}

		a.Log.Info("Campaign scheduled", "campaign_id", id, "scheduled_at", campaign.ScheduledAt)
		return r.SendEnvelope(map[string]interface{}{
			"message":      "Campaign scheduled",
			"status":       models.CampaignStatusScheduled,
			"scheduled_at": campaign.ScheduledAt,
		})
	}

	if reason := a.campaignDeliveryBlockReason(r.RequestCtx, &campaign); reason != "" {
		return r.SendErrorEnvelope(fasthttp.StatusConflict, reason, nil, "")
	}

	if err := a.enqueueCampaignRecipients(r.RequestCtx, &campaign, recipients, now, campaign.Status); err != nil {
		a.Log.Error("Failed to start campaign", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to queue recipients", nil, "")
	}

	a.Log.Info("Campaign started", "campaign_id", id, "recipients", len(recipients))

	return r.SendEnvelope(map[string]interface{}{
		"message": "Campaign started",
		"status":  models.CampaignStatusProcessing,
	})
}

// campaignMediaRequiredError builds the "template requires <type> header
// media" message shared by validateCampaignReadyForStart (below) and
// rsvpReminderMediaValidationError (rsvp_reminder_campaign.go) so the two
// call sites - one gating StartCampaign, the other gating an RSVP reminder
// send - cannot drift apart, since they were previously copy-pasted verbatim.
func campaignMediaRequiredError(headerType string) error {
	return fmt.Errorf("template requires %s header media. Configure campaign media before starting", strings.ToLower(headerType))
}

func (a *App) validateCampaignReadyForStart(campaign *models.BulkMessageCampaign) error {
	if campaign.Template == nil && campaign.TemplateID != uuid.Nil {
		var template models.Template
		if err := a.DB.Where("id = ? AND organization_id = ?", campaign.TemplateID, campaign.OrganizationID).First(&template).Error; err != nil {
			return fmt.Errorf("campaign template no longer exists")
		}
		campaign.Template = &template
	}
	if campaign.Template == nil {
		return fmt.Errorf("campaign template no longer exists")
	}

	switch campaign.Template.HeaderType {
	case "IMAGE", "VIDEO", "DOCUMENT":
		if strings.TrimSpace(campaign.HeaderMediaID) == "" &&
			strings.TrimSpace(campaign.HeaderMediaURL) == "" {
			return campaignMediaRequiredError(campaign.Template.HeaderType)
		}
	}

	return nil
}

func (a *App) loadPendingCampaignRecipients(campaignID uuid.UUID) ([]models.BulkMessageRecipient, error) {
	var recipients []models.BulkMessageRecipient
	err := a.DB.Where("campaign_id = ? AND status = ?", campaignID, models.MessageStatusPending).Find(&recipients).Error
	return recipients, err
}

func (a *App) enqueueCampaignRecipients(ctx context.Context, campaign *models.BulkMessageCampaign, recipients []models.BulkMessageRecipient, now time.Time, fallbackStatus models.CampaignStatus) error {
	previousStartedAt := campaign.StartedAt
	previousCompletedAt := campaign.CompletedAt
	startedAt := now
	if previousStartedAt != nil {
		startedAt = *previousStartedAt
	}

	updates := map[string]interface{}{
		"status":       models.CampaignStatusProcessing,
		"started_at":   startedAt,
		"completed_at": nil,
		"pause_reason": "",
	}
	if err := a.DB.Model(campaign).Updates(updates).Error; err != nil {
		return err
	}

	jobs := make([]*queue.RecipientJob, len(recipients))
	for i, recipient := range recipients {
		jobs[i] = &queue.RecipientJob{
			CampaignID:     campaign.ID,
			RecipientID:    recipient.ID,
			OrganizationID: campaign.OrganizationID,
			PhoneNumber:    recipient.PhoneNumber,
			RecipientName:  recipient.RecipientName,
			TemplateParams: recipient.TemplateParams,
			HeaderParams:   recipient.HeaderParams,
		}
	}

	if err := a.Queue.EnqueueRecipients(ctx, jobs); err != nil {
		revert := map[string]interface{}{
			"status": fallbackStatus,
		}
		if previousStartedAt != nil {
			revert["started_at"] = *previousStartedAt
		} else {
			revert["started_at"] = nil
		}
		if previousCompletedAt != nil {
			revert["completed_at"] = *previousCompletedAt
		} else {
			revert["completed_at"] = nil
		}
		a.DB.Model(campaign).Updates(revert)
		return err
	}

	return nil
}

// PauseCampaign implements pausing a campaign
func (a *App) PauseCampaign(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	id, err := parsePathUUID(r, "id", "campaign")
	if err != nil {
		return nil
	}

	campaign, err := findByIDAndOrg[models.BulkMessageCampaign](a.DB, r, id, orgID, "Campaign")
	if err != nil {
		return nil
	}

	if campaign.Status != models.CampaignStatusProcessing && campaign.Status != models.CampaignStatusQueued {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Campaign is not running", nil, "")
	}

	if err := a.DB.Model(campaign).Updates(map[string]any{
		"status":       models.CampaignStatusPaused,
		"pause_reason": "",
	}).Error; err != nil {
		a.Log.Error("Failed to pause campaign", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to pause campaign", nil, "")
	}

	a.Log.Info("Campaign paused", "campaign_id", id)

	return r.SendEnvelope(map[string]any{
		"message": "Campaign paused",
		"status":  models.CampaignStatusPaused,
	})
}

// CancelCampaign implements cancelling a campaign
func (a *App) CancelCampaign(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	id, err := parsePathUUID(r, "id", "campaign")
	if err != nil {
		return nil
	}

	campaign, err := findByIDAndOrg[models.BulkMessageCampaign](a.DB, r, id, orgID, "Campaign")
	if err != nil {
		return nil
	}

	if campaign.Status == models.CampaignStatusCompleted || campaign.Status == models.CampaignStatusCancelled {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Campaign already finished", nil, "")
	}

	if err := a.DB.Model(campaign).Update("status", models.CampaignStatusCancelled).Error; err != nil {
		a.Log.Error("Failed to cancel campaign", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to cancel campaign", nil, "")
	}
	now := time.Now().UTC()
	a.DB.Model(&models.RSVPReminderDelivery{}).
		Where("campaign_id = ? AND status = ?", id, models.RSVPReminderDeliveryQueued).
		Updates(map[string]interface{}{"status": models.RSVPReminderDeliverySkipped, "error_message": "Campaign cancelled", "attempted_at": now})
	a.DB.Model(&models.RSVPReminderSchedule{}).
		Where("campaign_id = ? AND status = ?", id, models.RSVPReminderScheduleProcessing).
		Updates(map[string]interface{}{"status": models.RSVPReminderScheduleCancelled, "processed_at": now})

	a.Log.Info("Campaign cancelled", "campaign_id", id)

	return r.SendEnvelope(map[string]any{
		"message": "Campaign cancelled",
		"status":  models.CampaignStatusCancelled,
	})
}

// RetryFailed retries sending to all failed recipients
func (a *App) RetryFailed(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	id, err := parsePathUUID(r, "id", "campaign")
	if err != nil {
		return nil
	}

	campaign, err := findByIDAndOrg[models.BulkMessageCampaign](a.DB, r, id, orgID, "Campaign")
	if err != nil {
		return nil
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
	if campaign.SourceType == models.CampaignSourceRSVPReminder && len(failedRecipients) > 0 {
		var failedDeliveryRecipientIDs []uuid.UUID
		if err := a.DB.Model(&models.RSVPReminderDelivery{}).
			Where("campaign_id = ? AND status = ?", id, models.RSVPReminderDeliveryFailed).
			Pluck("campaign_recipient_id", &failedDeliveryRecipientIDs).Error; err != nil {
			a.Log.Error("Failed to load retryable RSVP reminder deliveries", "error", err, "campaign_id", id)
			return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to load retryable reminder deliveries", nil, "")
		}
		allowed := make(map[uuid.UUID]struct{}, len(failedDeliveryRecipientIDs))
		for _, recipientID := range failedDeliveryRecipientIDs {
			allowed[recipientID] = struct{}{}
		}
		filtered := failedRecipients[:0]
		for _, recipient := range failedRecipients {
			if _, ok := allowed[recipient.ID]; ok {
				filtered = append(filtered, recipient)
			}
		}
		failedRecipients = filtered
	}

	if len(failedRecipients) == 0 {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "No failed messages to retry", nil, "")
	}

	inactivePhones, err := a.loadInactiveContactPhoneSet(orgID, campaignRecipientPhones(failedRecipients))
	if err != nil {
		a.Log.Error("Failed to load inactive contacts for retry", "error", err, "campaign_id", id)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to load inactive contacts", nil, "")
	}

	retryableRecipients := make([]models.BulkMessageRecipient, 0, len(failedRecipients))
	for _, recipient := range failedRecipients {
		if _, inactive := inactivePhones[normalizeCampaignRecipientPhone(recipient.PhoneNumber)]; inactive {
			continue
		}
		retryableRecipients = append(retryableRecipients, recipient)
	}
	skippedInactiveCount := len(failedRecipients) - len(retryableRecipients)
	if len(retryableRecipients) == 0 {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "No retryable failed messages found", nil, "")
	}

	retryableRecipientIDs := make([]uuid.UUID, 0, len(retryableRecipients))
	for _, recipient := range retryableRecipients {
		retryableRecipientIDs = append(retryableRecipientIDs, recipient.ID)
	}

	// Reset failed recipients to pending
	if err := a.DB.Model(&models.BulkMessageRecipient{}).
		Where("campaign_id = ? AND status = ? AND id IN ?", id, models.MessageStatusFailed, retryableRecipientIDs).
		Updates(map[string]interface{}{
			"status":        models.MessageStatusPending,
			"error_message": "",
		}).Error; err != nil {
		a.Log.Error("Failed to reset failed recipients", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to reset failed recipients", nil, "")
	}
	if campaign.SourceType == models.CampaignSourceRSVPReminder {
		if err := a.DB.Model(&models.RSVPReminderDelivery{}).
			Where("campaign_id = ? AND status = ? AND campaign_recipient_id IN ?", id, models.RSVPReminderDeliveryFailed, retryableRecipientIDs).
			Updates(map[string]interface{}{"status": models.RSVPReminderDeliveryQueued, "error_message": "", "message_id": ""}).Error; err != nil {
			a.Log.Error("Failed to reset RSVP reminder deliveries", "error", err, "campaign_id", id)
			return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to reset reminder deliveries", nil, "")
		}
	}

	// Reset failed messages in messages table to pending
	retryableContactIDs, err := a.loadContactIDsByPhones(orgID, campaignRecipientPhones(retryableRecipients))
	if err != nil {
		a.Log.Error("Failed to load retryable contacts", "error", err, "campaign_id", id)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to load retryable contacts", nil, "")
	}
	if len(retryableContactIDs) > 0 {
		if err := a.DB.Model(&models.Message{}).
			Where("metadata->>'campaign_id' = ? AND status = ? AND contact_id IN ?", id.String(), models.MessageStatusFailed, retryableContactIDs).
			Updates(map[string]interface{}{
				"status":        models.MessageStatusPending,
				"error_message": "",
			}).Error; err != nil {
			a.Log.Error("Failed to reset failed messages", "error", err)
		}
	}

	// Recalculate campaign stats from messages table
	a.recalculateCampaignStats(id)

	// Update campaign status to processing. A retry after completion/failed starts a new
	// measured sending run, while paused campaigns keep their original start time.
	now := time.Now()
	updates := map[string]interface{}{
		"status":       models.CampaignStatusProcessing,
		"completed_at": nil,
	}
	if campaign.Status == models.CampaignStatusCompleted || campaign.Status == models.CampaignStatusFailed || campaign.StartedAt == nil {
		updates["started_at"] = now
	}
	if err := a.DB.Model(campaign).Updates(updates).Error; err != nil {
		a.Log.Error("Failed to update campaign status", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to update campaign", nil, "")
	}
	if campaign.SourceType == models.CampaignSourceRSVPReminder {
		a.DB.Model(&models.RSVPReminderSchedule{}).
			Where("campaign_id = ? AND status = ?", id, models.RSVPReminderScheduleCompletedWithErrors).
			Updates(map[string]interface{}{"status": models.RSVPReminderScheduleProcessing, "processed_at": nil})
	}

	a.Log.Info("Retrying failed messages", "campaign_id", id, "failed_count", len(retryableRecipients), "skipped_inactive", skippedInactiveCount)

	// Enqueue failed recipients as individual jobs for parallel processing
	jobs := make([]*queue.RecipientJob, len(retryableRecipients))
	for i, recipient := range retryableRecipients {
		jobs[i] = &queue.RecipientJob{
			CampaignID:     id,
			RecipientID:    recipient.ID,
			OrganizationID: orgID,
			PhoneNumber:    recipient.PhoneNumber,
			RecipientName:  recipient.RecipientName,
			TemplateParams: recipient.TemplateParams,
			HeaderParams:   recipient.HeaderParams,
		}
	}

	if err := a.Queue.EnqueueRecipients(r.RequestCtx, jobs); err != nil {
		if campaign.SourceType == models.CampaignSourceRSVPReminder {
			a.DB.Model(&models.RSVPReminderDelivery{}).
				Where("campaign_id = ? AND campaign_recipient_id IN ?", id, retryableRecipientIDs).
				Updates(map[string]interface{}{"status": models.RSVPReminderDeliveryFailed, "error_message": "Failed to queue reminder retry"})
			a.DB.Model(&models.RSVPReminderSchedule{}).
				Where("campaign_id = ? AND status = ?", id, models.RSVPReminderScheduleProcessing).
				Updates(map[string]interface{}{"status": models.RSVPReminderScheduleCompletedWithErrors, "processed_at": time.Now().UTC()})
		}
		a.Log.Error("Failed to enqueue recipients for retry", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to queue recipients", nil, "")
	}

	a.Log.Info("Failed recipients enqueued for retry", "campaign_id", id, "count", len(jobs))

	return r.SendEnvelope(map[string]interface{}{
		"message":                "Retrying failed messages",
		"retry_count":            len(retryableRecipients),
		"skipped_inactive_count": skippedInactiveCount,
		"status":                 models.CampaignStatusProcessing,
	})
}

// ResendCampaign duplicates a finished campaign and starts the duplicate as a new tracked run.
func (a *App) ResendCampaign(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	id, err := parsePathUUID(r, "id", "campaign")
	if err != nil {
		return nil
	}

	var source models.BulkMessageCampaign
	if err := a.DB.Where("id = ? AND organization_id = ?", id, orgID).
		Preload("Template").
		First(&source).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Campaign not found", nil, "")
	}
	if source.SourceType == models.CampaignSourceRSVPReminder {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "RSVP reminder campaigns cannot be resent; create a new reminder from RSVP Results", nil, "")
	}

	if !canResendCampaign(source.Status) {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Campaign cannot be resent in current state", nil, "")
	}

	if err := a.validateCampaignReadyForStart(&source); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, err.Error(), nil, "")
	}

	var sourceRecipients []models.BulkMessageRecipient
	if err := a.DB.Where("campaign_id = ?", source.ID).Order("created_at ASC").Find(&sourceRecipients).Error; err != nil {
		a.Log.Error("Failed to load campaign recipients for resend", "error", err, "campaign_id", id)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to load campaign recipients", nil, "")
	}
	if len(sourceRecipients) == 0 {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Campaign has no recipients to resend", nil, "")
	}

	inactivePhones, err := a.loadInactiveContactPhoneSet(orgID, campaignRecipientPhones(sourceRecipients))
	if err != nil {
		a.Log.Error("Failed to load inactive contacts for resend", "error", err, "campaign_id", id)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to load inactive contacts", nil, "")
	}

	newCampaign := models.BulkMessageCampaign{
		BaseModel:            models.BaseModel{ID: uuid.New()},
		OrganizationID:       orgID,
		WhatsAppAccount:      source.WhatsAppAccount,
		Name:                 nextResendCampaignName(source.Name),
		TemplateID:           source.TemplateID,
		HeaderMediaURL:       source.HeaderMediaURL,
		HeaderMediaID:        source.HeaderMediaID,
		HeaderMediaFilename:  source.HeaderMediaFilename,
		HeaderMediaMimeType:  source.HeaderMediaMimeType,
		HeaderMediaLocalPath: source.HeaderMediaLocalPath,
		Status:               models.CampaignStatusDraft,
		CreatedBy:            userID,
	}

	newRecipients := make([]models.BulkMessageRecipient, 0, len(sourceRecipients))
	seenPhones := make(map[string]struct{}, len(sourceRecipients))
	skippedInactiveCount := 0
	for _, recipient := range sourceRecipients {
		phone := normalizeCampaignRecipientPhone(recipient.PhoneNumber)
		if phone == "" {
			continue
		}
		if _, seen := seenPhones[phone]; seen {
			continue
		}
		seenPhones[phone] = struct{}{}
		if _, inactive := inactivePhones[phone]; inactive {
			skippedInactiveCount++
			continue
		}
		newRecipients = append(newRecipients, models.BulkMessageRecipient{
			BaseModel:      models.BaseModel{ID: uuid.New()},
			CampaignID:     newCampaign.ID,
			PhoneNumber:    phone,
			RecipientName:  strings.TrimSpace(recipient.RecipientName),
			TemplateParams: recipient.TemplateParams,
			HeaderParams:   recipient.HeaderParams,
			Status:         models.MessageStatusPending,
		})
	}

	if len(newRecipients) == 0 {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "No active recipients found to resend", nil, "")
	}
	newCampaign.TotalRecipients = len(newRecipients)

	if err := a.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&newCampaign).Error; err != nil {
			return err
		}
		return tx.CreateInBatches(&newRecipients, campaignRecipientCreateBatchSize).Error
	}); err != nil {
		a.Log.Error("Failed to create resend campaign", "error", err, "campaign_id", id)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to create resend campaign", nil, "")
	}

	if err := a.enqueueCampaignRecipients(r.RequestCtx, &newCampaign, newRecipients, time.Now(), models.CampaignStatusDraft); err != nil {
		a.Log.Error("Failed to queue resend campaign", "error", err, "campaign_id", newCampaign.ID)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to queue resend campaign", nil, "")
	}

	a.Log.Info("Campaign resent as new campaign", "source_campaign_id", id, "new_campaign_id", newCampaign.ID, "recipients", len(newRecipients), "skipped_inactive", skippedInactiveCount)

	return r.SendEnvelope(map[string]interface{}{
		"message":                "Campaign resent as a new campaign",
		"campaign":               campaignToResponse(newCampaign, source.Template),
		"queued_count":           len(newRecipients),
		"skipped_inactive_count": skippedInactiveCount,
	})
}

// ImportRecipients implements adding recipients to a campaign
func (a *App) ImportRecipients(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	id, err := parsePathUUID(r, "id", "campaign")
	if err != nil {
		return nil
	}

	campaign, err := findByIDAndOrg[models.BulkMessageCampaign](a.DB, r, id, orgID, "Campaign")
	if err != nil {
		return nil
	}
	if campaign.SourceType == models.CampaignSourceRSVPReminder {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Recipients are managed from the RSVP reminder", nil, "")
	}

	if !canImportRecipientsToCampaign(campaign.Status) {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Cannot add recipients to campaign in current state", nil, "")
	}

	autoQueueRecipients := shouldAutoQueueImportedRecipients(campaign.Status)
	if autoQueueRecipients {
		if err := a.validateCampaignReadyForStart(campaign); err != nil {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, err.Error(), nil, "")
		}
	}

	var req CampaignRecipientImportRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}

	if len(req.Recipients) == 0 && len(req.ContactIDs) == 0 && len(req.TagNames) == 0 {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "At least one recipient, contact, or contact group is required", nil, "")
	}

	existingPhones := make(map[string]struct{})
	var existingRecipients []models.BulkMessageRecipient
	if err := a.DB.Where("campaign_id = ?", id).Find(&existingRecipients).Error; err != nil {
		a.Log.Error("Failed to load existing campaign recipients", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to load campaign recipients", nil, "")
	}
	for _, recipient := range existingRecipients {
		phone := normalizeCampaignRecipientPhone(recipient.PhoneNumber)
		if phone != "" {
			existingPhones[phone] = struct{}{}
		}
	}

	recipientMap := make(map[string]RecipientRequest)
	for _, rec := range req.Recipients {
		phone := normalizeCampaignRecipientPhone(rec.PhoneNumber)
		if phone == "" {
			continue
		}
		if _, exists := existingPhones[phone]; exists {
			continue
		}
		if _, exists := recipientMap[phone]; exists {
			continue
		}
		recipientMap[phone] = RecipientRequest{
			PhoneNumber:    phone,
			RecipientName:  strings.TrimSpace(rec.RecipientName),
			TemplateParams: rec.TemplateParams,
			HeaderParams:   rec.HeaderParams,
		}
	}

	selectedContacts, err := a.loadCampaignImportContacts(orgID, req.ContactIDs, req.TagNames)
	if err != nil {
		a.Log.Error("Failed to load campaign contacts for import", "error", err, "campaign_id", id)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to load contacts for import", nil, "")
	}
	for _, contact := range selectedContacts {
		phone := normalizeCampaignRecipientPhone(contact.PhoneNumber)
		if phone == "" {
			continue
		}
		if _, exists := existingPhones[phone]; exists {
			continue
		}
		if _, exists := recipientMap[phone]; exists {
			continue
		}
		recipientMap[phone] = RecipientRequest{
			PhoneNumber:   phone,
			RecipientName: strings.TrimSpace(contact.ProfileName),
		}
	}

	if len(recipientMap) == 0 {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "No new recipients found to add", nil, "")
	}

	recipients := make([]models.BulkMessageRecipient, 0, len(recipientMap))
	for _, rec := range recipientMap {
		recipients = append(recipients, models.BulkMessageRecipient{
			CampaignID:     id,
			PhoneNumber:    rec.PhoneNumber,
			RecipientName:  rec.RecipientName,
			TemplateParams: models.JSONB(rec.TemplateParams),
			HeaderParams:   models.JSONB(rec.HeaderParams),
			Status:         models.MessageStatusPending,
		})
	}

	if err := a.DB.CreateInBatches(&recipients, campaignRecipientCreateBatchSize).Error; err != nil {
		a.Log.Error("Failed to add recipients", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to add recipients", nil, "")
	}

	// Update total recipients count
	var totalCount int64
	a.DB.Model(&models.BulkMessageRecipient{}).Where("campaign_id = ?", id).Count(&totalCount)
	a.DB.Model(campaign).Update("total_recipients", totalCount)

	queuedCount := 0
	if autoQueueRecipients {
		if err := a.enqueueImportedCampaignRecipients(r.RequestCtx, campaign, recipients, time.Now()); err != nil {
			a.Log.Error("Failed to queue imported recipients", "error", err, "campaign_id", id)
			return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to queue recipients", nil, "")
		}
		queuedCount = len(recipients)
	}

	a.Log.Info("Recipients added to campaign", "campaign_id", id, "count", len(recipients))

	// Log recipient addition as audit
	phoneNumbers := make([]string, len(req.Recipients))
	for i, rec := range req.Recipients {
		phoneNumbers[i] = rec.PhoneNumber
	}
	a.logAudit(orgID, userID,
		"campaign", id, models.AuditActionUpdated, nil, nil,
		map[string]any{
			"field":     "recipients_added",
			"old_value": nil,
			"new_value": fmt.Sprintf("%d recipients added", len(req.Recipients)),
		})

	return r.SendEnvelope(map[string]any{
		"message":          "Recipients added successfully",
		"added_count":      len(recipients),
		"total_recipients": totalCount,
		"queued_count":     queuedCount,
		"send_started":     autoQueueRecipients,
	})
}

// GetCampaignRecipients implements listing campaign recipients
func (a *App) GetCampaignRecipients(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	id, err := parsePathUUID(r, "id", "campaign")
	if err != nil {
		return nil
	}

	// Verify campaign belongs to org
	_, err = findByIDAndOrg[models.BulkMessageCampaign](a.DB, r, id, orgID, "Campaign")
	if err != nil {
		return nil
	}

	var recipients []models.BulkMessageRecipient
	if err := a.DB.Where("campaign_id = ?", id).Order("created_at ASC").Find(&recipients).Error; err != nil {
		a.Log.Error("Failed to list recipients", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to list recipients", nil, "")
	}

	if a.ShouldMaskPhoneNumbers(orgID) {
		for i := range recipients {
			recipients[i].PhoneNumber = utils.MaskPhoneNumber(recipients[i].PhoneNumber)
			recipients[i].RecipientName = utils.MaskIfPhoneNumber(recipients[i].RecipientName)
		}
	}

	return r.SendEnvelope(map[string]any{
		"recipients": recipients,
		"total":      len(recipients),
	})
}

// normalizeCampaignRecipientPhone uses the contact canonical form, so
// 9876543210, +919876543210 and 919876543210 are one recipient.
func normalizeCampaignRecipientPhone(phone string) string {
	return contactutil.CanonicalPhone(phone)
}

func campaignRecipientPhones(recipients []models.BulkMessageRecipient) []string {
	phones := make([]string, 0, len(recipients))
	for _, recipient := range recipients {
		if phone := normalizeCampaignRecipientPhone(recipient.PhoneNumber); phone != "" {
			phones = append(phones, phone)
		}
	}
	return phones
}

func campaignToResponse(campaign models.BulkMessageCampaign, template *models.Template) CampaignResponse {
	response := CampaignResponse{
		ID:                  campaign.ID,
		Name:                campaign.Name,
		WhatsAppAccount:     campaign.WhatsAppAccount,
		TemplateID:          campaign.TemplateID,
		SourceType:          campaign.SourceType,
		SourceID:            campaign.SourceID,
		HeaderMediaURL:      campaign.HeaderMediaURL,
		HeaderMediaID:       campaign.HeaderMediaID,
		HeaderMediaFilename: campaign.HeaderMediaFilename,
		HeaderMediaMimeType: campaign.HeaderMediaMimeType,
		Status:              campaign.Status,
		TotalRecipients:     campaign.TotalRecipients,
		SentCount:           campaign.SentCount,
		DeliveredCount:      campaign.DeliveredCount,
		ReadCount:           campaign.ReadCount,
		FailedCount:         campaign.FailedCount,
		ScheduledAt:         campaign.ScheduledAt,
		StartedAt:           campaign.StartedAt,
		CompletedAt:         campaign.CompletedAt,
		CreatedAt:           campaign.CreatedAt,
		UpdatedAt:           campaign.UpdatedAt,
	}
	if template != nil {
		response.TemplateName = template.Name
	}
	return response
}

func canResendCampaign(status models.CampaignStatus) bool {
	switch status {
	case models.CampaignStatusCompleted,
		models.CampaignStatusFailed,
		models.CampaignStatusCancelled,
		models.CampaignStatusPaused:
		return true
	default:
		return false
	}
}

func nextResendCampaignName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "Campaign"
	}
	return name + " - Resend " + time.Now().Format("2006-01-02 15:04")
}

func (a *App) loadInactiveContactPhoneSet(orgID uuid.UUID, phones []string) (map[string]struct{}, error) {
	inactivePhones := make(map[string]struct{})
	uniquePhones := make([]string, 0, len(phones))
	seen := make(map[string]struct{}, len(phones))
	for _, phone := range phones {
		phone = normalizeCampaignRecipientPhone(phone)
		if phone == "" {
			continue
		}
		if _, exists := seen[phone]; exists {
			continue
		}
		seen[phone] = struct{}{}
		uniquePhones = append(uniquePhones, phone)
	}
	if len(uniquePhones) == 0 {
		return inactivePhones, nil
	}

	for start := 0; start < len(uniquePhones); start += campaignRecipientCreateBatchSize {
		end := start + campaignRecipientCreateBatchSize
		if end > len(uniquePhones) {
			end = len(uniquePhones)
		}
		batch := uniquePhones[start:end]
		phoneVariants := make([]string, 0, len(batch)*2)
		for _, phone := range batch {
			phoneVariants = append(phoneVariants, phone, "+"+phone)
		}

		var contacts []models.Contact
		if err := a.DB.
			Select("phone_number").
			Where("organization_id = ? AND is_active = ? AND phone_number IN ?", orgID, false, phoneVariants).
			Find(&contacts).Error; err != nil {
			return nil, err
		}
		for _, contact := range contacts {
			if phone := normalizeCampaignRecipientPhone(contact.PhoneNumber); phone != "" {
				inactivePhones[phone] = struct{}{}
			}
		}
	}

	return inactivePhones, nil
}

func (a *App) loadContactIDsByPhones(orgID uuid.UUID, phones []string) ([]uuid.UUID, error) {
	contactIDs := make([]uuid.UUID, 0, len(phones))
	uniquePhones := make([]string, 0, len(phones))
	seen := make(map[string]struct{}, len(phones))
	for _, phone := range phones {
		phone = normalizeCampaignRecipientPhone(phone)
		if phone == "" {
			continue
		}
		if _, exists := seen[phone]; exists {
			continue
		}
		seen[phone] = struct{}{}
		uniquePhones = append(uniquePhones, phone)
	}
	if len(uniquePhones) == 0 {
		return contactIDs, nil
	}

	for start := 0; start < len(uniquePhones); start += campaignRecipientCreateBatchSize {
		end := start + campaignRecipientCreateBatchSize
		if end > len(uniquePhones) {
			end = len(uniquePhones)
		}
		batch := uniquePhones[start:end]
		phoneVariants := make([]string, 0, len(batch)*2)
		for _, phone := range batch {
			phoneVariants = append(phoneVariants, phone, "+"+phone)
		}

		var contacts []models.Contact
		if err := a.DB.
			Select("id").
			Where("organization_id = ? AND phone_number IN ?", orgID, phoneVariants).
			Find(&contacts).Error; err != nil {
			return nil, err
		}
		for _, contact := range contacts {
			contactIDs = append(contactIDs, contact.ID)
		}
	}

	return contactIDs, nil
}

func canImportRecipientsToCampaign(status models.CampaignStatus) bool {
	switch status {
	case models.CampaignStatusDraft,
		models.CampaignStatusScheduled,
		models.CampaignStatusQueued,
		models.CampaignStatusProcessing,
		models.CampaignStatusPaused,
		models.CampaignStatusCompleted,
		models.CampaignStatusFailed:
		return true
	default:
		return false
	}
}

func shouldAutoQueueImportedRecipients(status models.CampaignStatus) bool {
	switch status {
	case models.CampaignStatusQueued,
		models.CampaignStatusProcessing,
		models.CampaignStatusCompleted,
		models.CampaignStatusFailed:
		return true
	default:
		return false
	}
}

func (a *App) enqueueImportedCampaignRecipients(ctx context.Context, campaign *models.BulkMessageCampaign, recipients []models.BulkMessageRecipient, now time.Time) error {
	originalStatus := campaign.Status
	updates := map[string]interface{}{
		"status": models.CampaignStatusProcessing,
	}
	if originalStatus == models.CampaignStatusCompleted || originalStatus == models.CampaignStatusFailed {
		updates["started_at"] = now
		updates["completed_at"] = nil
	} else if campaign.StartedAt == nil {
		updates["started_at"] = now
	}

	if err := a.DB.Model(campaign).Updates(updates).Error; err != nil {
		return err
	}

	jobs := make([]*queue.RecipientJob, len(recipients))
	for i, recipient := range recipients {
		jobs[i] = &queue.RecipientJob{
			CampaignID:     campaign.ID,
			RecipientID:    recipient.ID,
			OrganizationID: campaign.OrganizationID,
			PhoneNumber:    recipient.PhoneNumber,
			RecipientName:  recipient.RecipientName,
			TemplateParams: recipient.TemplateParams,
			HeaderParams:   recipient.HeaderParams,
		}
	}

	if err := a.Queue.EnqueueRecipients(ctx, jobs); err != nil {
		revert := map[string]interface{}{
			"status": originalStatus,
		}
		if campaign.StartedAt != nil {
			revert["started_at"] = *campaign.StartedAt
		} else {
			revert["started_at"] = nil
		}
		if campaign.CompletedAt != nil {
			revert["completed_at"] = *campaign.CompletedAt
		} else {
			revert["completed_at"] = nil
		}
		a.DB.Model(campaign).Updates(revert)
		return err
	}

	campaign.Status = models.CampaignStatusProcessing
	if originalStatus == models.CampaignStatusCompleted || originalStatus == models.CampaignStatusFailed || campaign.StartedAt == nil {
		startedAt := now
		campaign.StartedAt = &startedAt
	}
	campaign.CompletedAt = nil
	return nil
}

func (a *App) loadCampaignImportContacts(orgID uuid.UUID, contactIDs []string, tagNames []string) ([]models.Contact, error) {
	contactsByID := make(map[uuid.UUID]models.Contact)

	if len(contactIDs) > 0 {
		parsedIDs := make([]uuid.UUID, 0, len(contactIDs))
		for _, contactID := range contactIDs {
			contactID = strings.TrimSpace(contactID)
			if contactID == "" {
				continue
			}
			parsedID, err := uuid.Parse(contactID)
			if err != nil {
				continue
			}
			parsedIDs = append(parsedIDs, parsedID)
		}

		if len(parsedIDs) > 0 {
			var directContacts []models.Contact
			if err := a.DB.Where("organization_id = ? AND is_active = ? AND id IN ?", orgID, true, parsedIDs).Find(&directContacts).Error; err != nil {
				return nil, err
			}
			for _, contact := range directContacts {
				contactsByID[contact.ID] = contact
			}
		}
	}

	cleanTags := make([]string, 0, len(tagNames))
	for _, tagName := range tagNames {
		tagName = strings.TrimSpace(tagName)
		if tagName != "" {
			cleanTags = append(cleanTags, tagName)
		}
	}

	if len(cleanTags) > 0 {
		query := a.DB.Where("organization_id = ? AND is_active = ?", orgID, true)
		conditions := make([]string, 0, len(cleanTags))
		args := make([]any, 0, len(cleanTags))
		for _, tagName := range cleanTags {
			tagJSON, _ := json.Marshal([]string{tagName})
			conditions = append(conditions, "tags @> ?::jsonb")
			args = append(args, string(tagJSON))
		}
		if len(conditions) > 0 {
			query = query.Where("("+strings.Join(conditions, " OR ")+")", args...)
		}

		var taggedContacts []models.Contact
		if err := query.Find(&taggedContacts).Error; err != nil {
			return nil, err
		}
		for _, contact := range taggedContacts {
			contactsByID[contact.ID] = contact
		}
	}

	contacts := make([]models.Contact, 0, len(contactsByID))
	for _, contact := range contactsByID {
		contacts = append(contacts, contact)
	}

	return contacts, nil
}

// DeleteCampaignRecipient deletes a single recipient from a campaign
func (a *App) DeleteCampaignRecipient(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	campaignUUID, err := parsePathUUID(r, "id", "campaign")
	if err != nil {
		return nil
	}

	recipientUUID, err := parsePathUUID(r, "recipientId", "recipient")
	if err != nil {
		return nil
	}

	// Verify campaign belongs to org and is in draft status
	campaign, err := findByIDAndOrg[models.BulkMessageCampaign](a.DB, r, campaignUUID, orgID, "Campaign")
	if err != nil {
		return nil
	}

	if campaign.Status != models.CampaignStatusDraft {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Can only delete recipients from draft campaigns", nil, "")
	}

	// Load recipient for audit before deleting
	var recipient models.BulkMessageRecipient
	a.DB.Where("id = ? AND campaign_id = ?", recipientUUID, campaignUUID).First(&recipient)

	// Delete recipient
	result := a.DB.Where("id = ? AND campaign_id = ?", recipientUUID, campaignUUID).Delete(&models.BulkMessageRecipient{})
	if result.Error != nil {
		a.Log.Error("Failed to delete recipient", "error", result.Error)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to delete recipient", nil, "")
	}

	if result.RowsAffected == 0 {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Recipient not found", nil, "")
	}

	// Update campaign recipient count
	a.DB.Model(campaign).Update("total_recipients", gorm.Expr("total_recipients - 1"))

	a.logAudit(orgID, userID,
		"campaign", campaignUUID, models.AuditActionUpdated, nil, nil,
		map[string]any{
			"field":     "recipient_removed",
			"old_value": recipient.PhoneNumber,
			"new_value": nil,
		})

	return r.SendEnvelope(map[string]any{
		"message": "Recipient deleted successfully",
	})
}

// UploadCampaignMedia uploads media for a campaign's template header
func (a *App) UploadCampaignMedia(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	campaignUUID, err := parsePathUUID(r, "id", "campaign")
	if err != nil {
		return nil
	}

	// Get campaign with template
	var campaign models.BulkMessageCampaign
	if err := a.DB.Where("id = ? AND organization_id = ?", campaignUUID, orgID).
		Preload("Template").
		First(&campaign).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Campaign not found", nil, "")
	}

	// Only allow media upload for draft campaigns
	if campaign.Status != models.CampaignStatusDraft {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Can only upload media for draft campaigns", nil, "")
	}

	// Verify template has media header
	if campaign.Template == nil || campaign.Template.HeaderType == "" || campaign.Template.HeaderType == "TEXT" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Template does not have a media header", nil, "")
	}

	// Parse multipart form
	form, err := r.RequestCtx.MultipartForm()
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid multipart form", nil, "")
	}

	files := form.File["file"]
	if len(files) == 0 {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "No file provided", nil, "")
	}

	fileHeader := files[0]
	file, err := fileHeader.Open()
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Failed to open file", nil, "")
	}
	defer func() { _ = file.Close() }()

	// Read file content (limit to 16MB)
	const maxMediaSize = 16 << 20 // 16MB
	data, err := io.ReadAll(io.LimitReader(file, maxMediaSize+1))
	if err != nil {
		a.Log.Error("Failed to read file", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to read file", nil, "")
	}
	if len(data) > maxMediaSize {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "File too large. Maximum size is 16MB", nil, "")
	}

	// Browsers and proxies often send uploads as application/octet-stream.
	// Fall back to filename/content sniffing so valid campaign media is not rejected.
	mimeType := detectCampaignMediaMimeType(fileHeader.Filename, fileHeader.Header.Get("Content-Type"), data)
	if !isAllowedCampaignMediaMimeType(mimeType) {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Unsupported file type: "+mimeType, nil, "")
	}

	// Save file locally and expose it via a signed public URL so template sends
	// can use header link values directly instead of pre-uploading to Meta.
	localPath, err := a.saveCampaignMedia(campaignUUID.String(), data, mimeType)
	if err != nil {
		a.Log.Error("Failed to save media locally", "error", err, "campaign_id", campaignUUID, "mime_type", mimeType, "filename", fileHeader.Filename)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to save media locally", nil, "")
	}

	filename := sanitizeFilename(fileHeader.Filename)
	updates := map[string]interface{}{
		"header_media_url":        "",
		"header_media_id":         "",
		"header_media_filename":   filename,
		"header_media_mime_type":  mimeType,
		"header_media_local_path": localPath,
	}
	if err := a.DB.Model(&campaign).Updates(updates).Error; err != nil {
		a.Log.Error("Failed to update campaign with media info", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to save media info", nil, "")
	}

	campaign.HeaderMediaID = ""
	campaign.HeaderMediaFilename = filename
	campaign.HeaderMediaMimeType = mimeType
	campaign.HeaderMediaLocalPath = localPath
	publicURL := a.buildPublicCampaignMediaURL(r, &campaign)

	if err := a.DB.Model(&campaign).Update("header_media_url", publicURL).Error; err != nil {
		a.Log.Error("Failed to update campaign media URL", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to save media info", nil, "")
	}

	a.Log.Info("Campaign media uploaded", "campaign_id", campaignUUID, "filename", fileHeader.Filename, "local_path", localPath, "public_url", publicURL)

	return r.SendEnvelope(map[string]interface{}{
		"media_id":   "",
		"media_url":  publicURL,
		"filename":   fileHeader.Filename,
		"mime_type":  mimeType,
		"local_path": localPath,
		"message":    "Media uploaded successfully",
	})
}

// saveCampaignMedia saves uploaded media locally for preview
func (a *App) saveCampaignMedia(campaignID string, data []byte, mimeType string) (string, error) {
	// Determine file extension
	ext := getExtensionFromMimeType(mimeType)
	if ext == "" {
		ext = ".bin"
	}

	// Create campaigns media directory
	subdir := "campaigns"
	if err := a.ensureMediaDir(subdir); err != nil {
		return "", fmt.Errorf("failed to create media directory: %w", err)
	}

	// Generate filename using campaign ID
	filename := campaignID + ext
	filePath := filepath.Join(a.getMediaStoragePath(), subdir, filename)

	// Save file
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to save media file: %w", err)
	}

	// Return relative path for storage
	relativePath := filepath.Join(subdir, filename)
	a.Log.Info("Campaign media saved locally", "path", relativePath, "size", len(data))

	return relativePath, nil
}

func (a *App) buildPublicCampaignMediaURL(r *fastglue.Request, campaign *models.BulkMessageCampaign) string {
	return a.buildCampaignMediaURLForBase(a.requestPublicBaseURL(r.RequestCtx), campaign)
}

// buildCampaignMediaURLForBase is the request-independent core of
// buildPublicCampaignMediaURL, for callers that have already resolved a base
// URL some other way (e.g. from config, when there is no HTTP request to
// derive one from - see publicBaseURLFromConfig).
func (a *App) buildCampaignMediaURLForBase(baseURL string, campaign *models.BulkMessageCampaign) string {
	filename := campaign.HeaderMediaFilename
	if filename == "" {
		filename = campaign.ID.String() + getExtensionFromMimeType(campaign.HeaderMediaMimeType)
	}

	token := url.QueryEscape(a.campaignMediaToken(campaign))
	return fmt.Sprintf("%s/public/campaigns/%s/media/%s?token=%s",
		baseURL,
		url.PathEscape(campaign.ID.String()),
		url.PathEscape(filename),
		token,
	)
}

// publicBaseURLFromConfig builds a public base URL for contexts with no
// incoming HTTP request to derive one from - namely the RSVP reminder
// scheduler (rsvp_scheduler.go), which runs on a ticker rather than in
// response to a request. Falls back to Host:Port when server.public_url is
// not configured; that fallback is very unlikely to be reachable from the
// public internet (Meta must be able to fetch header media from the result),
// so a warning is logged to keep the gap visible instead of silently
// producing an undeliverable link - the same class of failure this branch
// exists to fix, just triggered by the scheduler instead of a manual send.
func (a *App) publicBaseURLFromConfig() string {
	if configured := strings.TrimRight(strings.TrimSpace(a.Config.Server.PublicURL), "/"); configured != "" {
		return configured + sanitizeRedirectPath(a.Config.Server.BasePath)
	}
	host := a.Config.Server.Host
	if host == "" || host == "0.0.0.0" {
		host = "localhost"
	}
	a.Log.Warn("server.public_url is not configured; scheduled RSVP reminder media links will use a best-effort host Meta likely cannot reach",
		"host", host, "port", a.Config.Server.Port)
	return fmt.Sprintf("http://%s:%d%s", host, a.Config.Server.Port, sanitizeRedirectPath(a.Config.Server.BasePath))
}

func (a *App) requestPublicBaseURL(ctx *fasthttp.RequestCtx) string {
	scheme := firstForwardedValue(string(ctx.Request.Header.Peek("X-Forwarded-Proto")))
	host := firstForwardedValue(string(ctx.Request.Header.Peek("X-Forwarded-Host")))

	if originScheme, originHost := requestOriginParts(ctx); scheme == "" {
		scheme = originScheme
		if host == "" {
			host = originHost
		}
	} else if host == "" {
		if _, originHost := requestOriginParts(ctx); originHost != "" {
			host = originHost
		}
	}

	if scheme == "" {
		scheme = string(ctx.URI().Scheme())
	}
	if host == "" {
		host = string(ctx.Host())
	}
	if scheme == "" {
		scheme = "http"
	}
	if host == "" {
		host = "localhost"
	}

	basePath := sanitizeRedirectPath(a.Config.Server.BasePath)
	return fmt.Sprintf("%s://%s%s", scheme, host, basePath)
}

func requestOriginParts(ctx *fasthttp.RequestCtx) (string, string) {
	for _, raw := range []string{
		strings.TrimSpace(string(ctx.Request.Header.Peek("Origin"))),
		strings.TrimSpace(string(ctx.Request.Header.Peek("Referer"))),
	} {
		if raw == "" {
			continue
		}
		parsed, err := url.Parse(raw)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			continue
		}
		return parsed.Scheme, parsed.Host
	}
	return "", ""
}

func firstForwardedValue(value string) string {
	if value == "" {
		return ""
	}
	parts := strings.Split(value, ",")
	if len(parts) == 0 {
		return ""
	}
	return strings.TrimSpace(parts[0])
}

func (a *App) campaignMediaToken(campaign *models.BulkMessageCampaign) string {
	mac := hmac.New(sha256.New, []byte(a.Config.JWT.Secret))
	mac.Write([]byte(campaign.ID.String()))
	mac.Write([]byte{0})
	mac.Write([]byte(campaign.OrganizationID.String()))
	mac.Write([]byte{0})
	mac.Write([]byte(strings.TrimSpace(campaign.HeaderMediaFilename)))
	mac.Write([]byte{0})
	mac.Write([]byte(strings.TrimSpace(campaign.HeaderMediaMimeType)))
	mac.Write([]byte{0})
	mac.Write([]byte(strings.TrimSpace(campaign.HeaderMediaLocalPath)))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// ServeCampaignMedia serves campaign media files for preview
func (a *App) ServeCampaignMedia(r *fastglue.Request) error {
	// Get auth context
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	// Get campaign ID from URL
	campaignUUID, err := parsePathUUID(r, "id", "campaign")
	if err != nil {
		return nil
	}

	// Find campaign and verify access
	campaign, err := findByIDAndOrg[models.BulkMessageCampaign](a.DB, r, campaignUUID, orgID, "Campaign")
	if err != nil {
		return nil
	}

	// Check if campaign has media
	if strings.TrimSpace(campaign.HeaderMediaLocalPath) == "" && strings.TrimSpace(campaign.HeaderMediaID) == "" && strings.TrimSpace(campaign.HeaderMediaURL) == "" {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "No media found", nil, "")
	}

	data, contentType, err := a.readCampaignMediaFile(context.WithoutCancel(r.RequestCtx), campaign)
	if err != nil {
		status := fasthttp.StatusInternalServerError
		message := "Failed to read file"
		if os.IsNotExist(err) {
			status = fasthttp.StatusNotFound
			message = "File not found"
		}
		a.Log.Error("Failed to serve campaign media", "campaign_id", campaignUUID, "error", err)
		return r.SendErrorEnvelope(status, message, nil, "")
	}

	r.RequestCtx.Response.Header.Set("Content-Type", contentType)
	r.RequestCtx.Response.Header.Set("Cache-Control", "private, max-age=3600")
	r.RequestCtx.SetBody(data)

	return nil
}

// ServePublicCampaignMedia serves locally stored campaign media via a signed URL.
func (a *App) ServePublicCampaignMedia(r *fastglue.Request) error {
	campaignUUID, err := parsePathUUID(r, "id", "campaign")
	if err != nil {
		return nil
	}

	token := strings.TrimSpace(string(r.RequestCtx.QueryArgs().Peek("token")))
	if token == "" {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	var campaign models.BulkMessageCampaign
	if err := a.DB.First(&campaign, "id = ?", campaignUUID).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Campaign not found", nil, "")
	}

	if token != a.campaignMediaToken(&campaign) {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	if strings.TrimSpace(campaign.HeaderMediaLocalPath) == "" {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "No media found", nil, "")
	}

	data, contentType, err := a.readStoredMediaFile(campaign.HeaderMediaLocalPath, campaign.HeaderMediaMimeType)
	if err != nil {
		status := fasthttp.StatusInternalServerError
		message := "Failed to read file"
		if os.IsNotExist(err) {
			status = fasthttp.StatusNotFound
			message = "File not found"
		}
		a.Log.Error("Failed to serve public campaign media", "campaign_id", campaignUUID, "error", err)
		return r.SendErrorEnvelope(status, message, nil, "")
	}

	r.RequestCtx.Response.Header.Set("Content-Type", contentType)
	r.RequestCtx.Response.Header.Set("Cache-Control", "public, max-age=3600")
	r.RequestCtx.SetBody(data)

	return nil
}

func (a *App) readCampaignMediaFile(ctx context.Context, campaign *models.BulkMessageCampaign) ([]byte, string, error) {
	filePath := strings.TrimSpace(campaign.HeaderMediaLocalPath)
	if filePath != "" {
		data, contentType, err := a.readStoredMediaFile(filePath, campaign.HeaderMediaMimeType)
		if err == nil {
			return data, contentType, nil
		}
		if !os.IsNotExist(err) {
			a.Log.Warn("Stored campaign media read failed, attempting re-download", "campaign_id", campaign.ID, "error", err)
		}
	}

	if strings.TrimSpace(campaign.HeaderMediaURL) != "" {
		if a.isLocalCampaignMediaURL(campaign.HeaderMediaURL) {
			return nil, "", os.ErrNotExist
		}

		data, _, mimeType, err := a.downloadCampaignHeaderMedia(ctx, campaign.HeaderMediaURL)
		if err != nil {
			return nil, "", fmt.Errorf("download campaign media from url: %w", err)
		}

		localPath, err := a.saveCampaignMedia(campaign.ID.String(), data, mimeType)
		if err == nil {
			campaign.HeaderMediaLocalPath = localPath
			campaign.HeaderMediaMimeType = mimeType
			if dbErr := a.DB.Model(campaign).Updates(map[string]interface{}{
				"header_media_local_path": localPath,
				"header_media_mime_type":  mimeType,
			}).Error; dbErr != nil {
				return nil, "", fmt.Errorf("update campaign media path: %w", dbErr)
			}
		} else {
			a.Log.Warn("Failed to save campaign media downloaded from URL", "campaign_id", campaign.ID, "error", err)
		}

		if mimeType == "" {
			mimeType = campaign.HeaderMediaMimeType
		}
		if mimeType == "" {
			mimeType = http.DetectContentType(data)
		}
		return data, mimeType, nil
	}

	if strings.TrimSpace(campaign.HeaderMediaID) == "" {
		if filePath == "" {
			return nil, "", os.ErrNotExist
		}
		return nil, "", fmt.Errorf("read stored campaign media: %w", os.ErrNotExist)
	}

	account, err := a.resolveWhatsAppAccount(campaign.OrganizationID, campaign.WhatsAppAccount)
	if err != nil {
		return nil, "", fmt.Errorf("resolve whatsapp account: %w", err)
	}

	localPath, err := a.DownloadAndSaveMedia(ctx, campaign.HeaderMediaID, campaign.HeaderMediaMimeType, a.toWhatsAppAccount(account))
	if err != nil {
		return nil, "", fmt.Errorf("download campaign media: %w", err)
	}

	campaign.HeaderMediaLocalPath = localPath
	if err := a.DB.Model(campaign).Update("header_media_local_path", localPath).Error; err != nil {
		return nil, "", fmt.Errorf("update campaign media path: %w", err)
	}

	return a.readStoredMediaFile(localPath, campaign.HeaderMediaMimeType)
}

func (a *App) isLocalCampaignMediaURL(rawURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return false
	}

	pathPrefix := sanitizeRedirectPath(a.Config.Server.BasePath) + "/public/campaigns/"
	return strings.HasPrefix(parsed.EscapedPath(), pathPrefix) || strings.HasPrefix(parsed.Path, pathPrefix)
}

func (a *App) readStoredMediaFile(filePath, storedMimeType string) ([]byte, string, error) {
	filePath = filepath.Clean(filePath)
	baseDir, err := filepath.Abs(a.getMediaStoragePath())
	if err != nil {
		return nil, "", fmt.Errorf("storage configuration error: %w", err)
	}

	fullPath, err := filepath.Abs(filepath.Join(baseDir, filePath))
	if err != nil || !strings.HasPrefix(fullPath, baseDir+string(os.PathSeparator)) {
		return nil, "", fmt.Errorf("invalid file path")
	}

	info, err := os.Lstat(fullPath)
	if err != nil {
		return nil, "", err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, "", fmt.Errorf("invalid file path")
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, "", err
	}

	contentType := storedMimeType
	if contentType == "" {
		ext := strings.ToLower(filepath.Ext(filePath))
		contentType = getMimeTypeFromExtension(ext)
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	return data, contentType, nil
}

// getMimeTypeFromExtension returns MIME type from file extension
func getMimeTypeFromExtension(ext string) string {
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".mp4":
		return "video/mp4"
	case ".3gp":
		return "video/3gpp"
	case ".pdf":
		return "application/pdf"
	case ".doc":
		return "application/msword"
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".xls":
		return "application/vnd.ms-excel"
	case ".xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case ".ppt":
		return "application/vnd.ms-powerpoint"
	case ".pptx":
		return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	case ".txt":
		return "text/plain"
	default:
		return "application/octet-stream"
	}
}

func detectCampaignMediaMimeType(filename, contentType string, data []byte) string {
	mimeType := strings.TrimSpace(contentType)
	if idx := strings.Index(mimeType, ";"); idx >= 0 {
		mimeType = strings.TrimSpace(mimeType[:idx])
	}

	if mimeType == "" || mimeType == "application/octet-stream" {
		if inferred := getMimeTypeFromExtension(strings.ToLower(filepath.Ext(filename))); inferred != "application/octet-stream" {
			mimeType = inferred
		} else if len(data) > 0 {
			mimeType = http.DetectContentType(data)
		}
	}

	if mimeType == "" {
		return "application/octet-stream"
	}
	return mimeType
}

func isAllowedCampaignMediaMimeType(mimeType string) bool {
	switch mimeType {
	case "image/jpeg",
		"image/png",
		"image/webp",
		"video/mp4",
		"video/3gpp",
		"audio/aac",
		"audio/mp4",
		"audio/mpeg",
		"audio/ogg",
		"application/pdf",
		"application/msword",
		"application/vnd.ms-excel",
		"application/vnd.ms-powerpoint",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"application/vnd.openxmlformats-officedocument.presentationml.presentation",
		"text/plain":
		return true
	default:
		return false
	}
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

	var campaign models.BulkMessageCampaign
	campaign.ID = campaignUUID

	// atomic update and return updated record
	result := a.DB.Model(&campaign).
		Clauses(clause.Returning{}).
		Update(column, gorm.Expr(column+" + 1"))

	if result.Error != nil {
		a.Log.Error("Failed to increment campaign stat", "error", result.Error, "campaign_id", campaignID, "column", column)
		return
	}

	// Broadcast stats update via WebSocket
	if a.WSHub != nil && result.RowsAffected > 0 {
		a.WSHub.BroadcastToOrg(campaign.OrganizationID, websocket.WSMessage{
			Type: websocket.TypeCampaignStatsUpdate,
			Payload: map[string]interface{}{
				"campaign_id":      campaignID,
				"status":           campaign.Status,
				"total_recipients": campaign.TotalRecipients,
				"sent_count":       campaign.SentCount,
				"delivered_count":  campaign.DeliveredCount,
				"read_count":       campaign.ReadCount,
				"failed_count":     campaign.FailedCount,
				"started_at":       campaign.StartedAt,
				"completed_at":     campaign.CompletedAt,
			},
		})
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

	if err := a.DB.Model(&models.Message{}).
		Where("metadata->>'campaign_id' = ?", campaignID.String()).
		Select(`
			COUNT(CASE WHEN status IN ('sent','delivered','read') THEN 1 END) as sent,
			COUNT(CASE WHEN status IN ('delivered','read') THEN 1 END) as delivered,
			COUNT(CASE WHEN status = 'read' THEN 1 END) as read,
			COUNT(CASE WHEN status = 'failed' THEN 1 END) as failed
		`).Scan(&stats).Error; err != nil {
		a.Log.Error("Failed to scan campaign message stats", "error", err, "campaign_id", campaignID)
		return
	}

	if err := a.DB.Model(&models.BulkMessageCampaign{}).Where("id = ?", campaignID).
		Updates(map[string]any{
			"sent_count":      stats.Sent,
			"delivered_count": stats.Delivered,
			"read_count":      stats.Read,
			"failed_count":    stats.Failed,
		}).Error; err != nil {
		a.Log.Error("Failed to recalculate campaign stats", "error", err, "campaign_id", campaignID)
	}
}

// sanitizeFilename removes path separators, dangerous characters, and truncates length.
var safeFilenameRe = regexp.MustCompile(`[^a-zA-Z0-9._-]`)

func sanitizeFilename(name string) string {
	// Strip any path component
	name = filepath.Base(name)
	// Replace unsafe characters
	name = safeFilenameRe.ReplaceAllString(name, "_")
	// Truncate to 255 chars
	if len(name) > 255 {
		name = name[:255]
	}
	if name == "" || name == "." || name == ".." {
		name = "unnamed"
	}
	return name
}
