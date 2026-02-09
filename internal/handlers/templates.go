package handlers

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/pkg/whatsapp"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

// TemplateRequest represents the request body for creating/updating a template
type TemplateRequest struct {
	WhatsAppAccount string        `json:"whatsapp_account" validate:"required"` // WhatsApp account name
	Name            string        `json:"name" validate:"required"`
	DisplayName     string        `json:"display_name"`
	Language        string        `json:"language" validate:"required"`
	Category        string        `json:"category" validate:"required"` // MARKETING, UTILITY, AUTHENTICATION
	HeaderType      string        `json:"header_type"`                  // TEXT, IMAGE, DOCUMENT, VIDEO, NONE
	HeaderContent   string        `json:"header_content"`
	BodyContent     string        `json:"body_content" validate:"required"`
	FooterContent   string        `json:"footer_content"`
	Buttons         []interface{} `json:"buttons"`
	SampleValues    []interface{} `json:"sample_values"`
}

// TemplateResponse represents the response for a template
type TemplateResponse struct {
	ID              uuid.UUID     `json:"id"`
	WhatsAppAccount string        `json:"whatsapp_account"` // WhatsApp account name
	MetaTemplateID  string        `json:"meta_template_id"`
	Name            string        `json:"name"`
	DisplayName     string        `json:"display_name"`
	Language        string        `json:"language"`
	Category        string        `json:"category"`
	Status          string        `json:"status"`
	HeaderType      string        `json:"header_type"`
	HeaderContent   string        `json:"header_content"`
	BodyContent     string        `json:"body_content"`
	FooterContent   string        `json:"footer_content"`
	Buttons         []interface{} `json:"buttons"`
	SampleValues    []interface{} `json:"sample_values"`
	CreatedAt       string        `json:"created_at"`
	UpdatedAt       string        `json:"updated_at"`
}

// ListTemplates returns all templates for the organization
func (a *App) ListTemplates(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	pg := parsePagination(r)

	// Optional filters
	accountName := string(r.RequestCtx.QueryArgs().Peek("account")) // Filter by account name
	status := string(r.RequestCtx.QueryArgs().Peek("status"))
	category := string(r.RequestCtx.QueryArgs().Peek("category"))

	query := a.DB.Where("organization_id = ?", orgID)

	if accountName != "" {
		query = query.Where("whats_app_account = ?", accountName)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if category != "" {
		query = query.Where("category = ?", category)
	}

	var total int64
	query.Model(&models.Template{}).Count(&total)

	var templates []models.Template
	if err := pg.Apply(query.Order("created_at DESC")).
		Find(&templates).Error; err != nil {
		a.Log.Error("Failed to list templates", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to list templates", nil, "")
	}

	response := make([]TemplateResponse, len(templates))
	for i, t := range templates {
		response[i] = templateToResponse(t)
	}

	return r.SendEnvelope(map[string]any{
		"templates": response,
		"total":     total,
		"page":      pg.Page,
		"limit":     pg.Limit,
	})
}

// CreateTemplate creates a new message template
func (a *App) CreateTemplate(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	var req TemplateRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}

	// Validate required fields
	if req.WhatsAppAccount == "" || req.Name == "" || req.Language == "" || req.Category == "" || req.BodyContent == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "whatsapp_account, name, language, category, and body_content are required", nil, "")
	}

	// Verify account belongs to organization
	var account models.WhatsAppAccount
	if err := a.DB.Where("name = ? AND organization_id = ?", req.WhatsAppAccount, orgID).First(&account).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "WhatsApp account not found", nil, "")
	}

	// Normalize template name (lowercase, underscores)
	templateName := normalizeTemplateName(req.Name)

	// Check if template with same name exists for this account
	var existingTemplate models.Template
	if err := a.DB.Where("organization_id = ? AND whats_app_account = ? AND name = ?", orgID, req.WhatsAppAccount, templateName).First(&existingTemplate).Error; err == nil {
		return r.SendErrorEnvelope(fasthttp.StatusConflict, "Template with this name already exists", nil, "")
	}

	displayName := req.DisplayName
	if displayName == "" {
		displayName = req.Name
	}

	template := models.Template{
		OrganizationID:  orgID,
		WhatsAppAccount: req.WhatsAppAccount,
		Name:            templateName,
		DisplayName:     displayName,
		Language:        req.Language,
		Category:        strings.ToUpper(req.Category),
		Status:          "DRAFT", // Local draft until submitted to Meta
		HeaderType:      strings.ToUpper(req.HeaderType),
		HeaderContent:   req.HeaderContent,
		BodyContent:     req.BodyContent,
		FooterContent:   req.FooterContent,
		Buttons:         convertToJSONBArray(req.Buttons),
		SampleValues:    convertToJSONBArray(req.SampleValues),
	}

	if err := a.DB.Create(&template).Error; err != nil {
		a.Log.Error("Failed to create template", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to create template", nil, "")
	}

	return r.SendEnvelope(templateToResponse(template))
}

// GetTemplate returns a single template
func (a *App) GetTemplate(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	id, err := parsePathUUID(r, "id", "template")
	if err != nil {
		return nil
	}

	template, err := findByIDAndOrg[models.Template](a.DB, r, id, orgID, "Template")
	if err != nil {
		return nil
	}

	return r.SendEnvelope(templateToResponse(*template))
}

// UpdateTemplate updates a message template
func (a *App) UpdateTemplate(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	id, err := parsePathUUID(r, "id", "template")
	if err != nil {
		return nil
	}

	template, err := findByIDAndOrg[models.Template](a.DB, r, id, orgID, "Template")
	if err != nil {
		return nil
	}

	// When editing approved or rejected templates, set to DRAFT to indicate local changes pending submission
	if template.Status == "APPROVED" || template.Status == "REJECTED" {
		template.Status = "DRAFT"
	}

	var req TemplateRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}

	// Update fields
	if req.DisplayName != "" {
		template.DisplayName = req.DisplayName
	}
	if req.Language != "" {
		template.Language = req.Language
	}
	if req.Category != "" {
		template.Category = strings.ToUpper(req.Category)
	}
	if req.HeaderType != "" {
		template.HeaderType = strings.ToUpper(req.HeaderType)
	}
	template.HeaderContent = req.HeaderContent
	if req.BodyContent != "" {
		template.BodyContent = req.BodyContent
	}
	template.FooterContent = req.FooterContent
	if req.Buttons != nil {
		template.Buttons = convertToJSONBArray(req.Buttons)
	}
	if req.SampleValues != nil {
		template.SampleValues = convertToJSONBArray(req.SampleValues)
	}

	if err := a.DB.Save(template).Error; err != nil {
		a.Log.Error("Failed to update template", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to update template", nil, "")
	}

	return r.SendEnvelope(templateToResponse(*template))
}

// DeleteTemplate deletes a message template
func (a *App) DeleteTemplate(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	id, err := parsePathUUID(r, "id", "template")
	if err != nil {
		return nil
	}

	template, err := findByIDAndOrg[models.Template](a.DB, r, id, orgID, "Template")
	if err != nil {
		return nil
	}

	// If template exists on Meta, delete it there too
	if template.MetaTemplateID != "" {
		var account models.WhatsAppAccount
		if err := a.DB.Where("name = ? AND organization_id = ?", template.WhatsAppAccount, orgID).First(&account).Error; err == nil {
			// Delete from Meta API
			go a.deleteTemplateFromMeta(&account, template.Name)
		}
	}

	if err := a.DB.Delete(template).Error; err != nil {
		a.Log.Error("Failed to delete template", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to delete template", nil, "")
	}

	return r.SendEnvelope(map[string]string{"message": "Template deleted successfully"})
}

// SubmitTemplate submits a template to Meta for approval
func (a *App) SubmitTemplate(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	id, err := parsePathUUID(r, "id", "template")
	if err != nil {
		return nil
	}

	template, err := findByIDAndOrg[models.Template](a.DB, r, id, orgID, "Template")
	if err != nil {
		return nil
	}

	// Only block if status is PENDING (awaiting approval - can't modify)
	if template.MetaTemplateID != "" && template.Status == "PENDING" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Template is pending approval and cannot be modified", nil, "")
	}

	// Get the WhatsApp account
	var account models.WhatsAppAccount
	if err := a.DB.Where("name = ? AND organization_id = ?", template.WhatsAppAccount, orgID).First(&account).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "WhatsApp account not found", nil, "")
	}

	// Check if this is an update to an existing template on Meta
	isUpdate := template.MetaTemplateID != ""

	// Submit template to Meta
	metaTemplateID, submitErr := a.submitTemplateToMeta(&account, template)
	if submitErr != nil {
		a.Log.Error("Failed to submit template to Meta", "error", submitErr)
		return r.SendErrorEnvelope(fasthttp.StatusBadGateway, "Failed to submit template to Meta: "+submitErr.Error(), nil, "")
	}
	template.MetaTemplateID = metaTemplateID

	// Update template status
	// Both new submissions and updates go to PENDING for approval
	message := "Template submitted to Meta for approval"
	if isUpdate {
		message = "Template updated and pending re-approval"
	}
	template.Status = "PENDING"

	if err := a.DB.Save(template).Error; err != nil {
		a.Log.Error("Failed to update template after submission", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Template submitted but failed to update local record", nil, "")
	}

	return r.SendEnvelope(map[string]interface{}{
		"message":          message,
		"meta_template_id": metaTemplateID,
		"status":           template.Status,
		"template":         templateToResponse(*template),
	})
}

// submitTemplateToMeta submits a template to Meta's API (creates new or updates existing)
func (a *App) submitTemplateToMeta(account *models.WhatsAppAccount, template *models.Template) (string, error) {
	waAccount := a.toWhatsAppAccount(account)

	submission := &whatsapp.TemplateSubmission{
		MetaTemplateID: template.MetaTemplateID, // If set, will update instead of create
		Name:           template.Name,
		Language:       template.Language,
		Category:       template.Category,
		HeaderType:     template.HeaderType,
		HeaderContent:  template.HeaderContent,
		BodyContent:    template.BodyContent,
		FooterContent:  template.FooterContent,
		Buttons:        template.Buttons,
		SampleValues:   template.SampleValues,
	}

	ctx := context.Background()
	return a.WhatsApp.SubmitTemplate(ctx, waAccount, submission)
}

// SyncTemplates syncs templates from Meta API
func (a *App) SyncTemplates(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	// Get account name from query or body
	accountName := string(r.RequestCtx.QueryArgs().Peek("account"))
	if accountName == "" {
		var body struct {
			WhatsAppAccount string `json:"whatsapp_account"`
		}
		_ = r.Decode(&body, "json")
		accountName = body.WhatsAppAccount
	}

	if accountName == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "whatsapp_account is required", nil, "")
	}

	var account models.WhatsAppAccount
	if err := a.DB.Where("name = ? AND organization_id = ?", accountName, orgID).First(&account).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "WhatsApp account not found", nil, "")
	}

	// Fetch templates from Meta API
	templates, err := a.fetchTemplatesFromMeta(&account)
	if err != nil {
		a.Log.Error("Failed to fetch templates from Meta", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusBadGateway, "Failed to fetch templates from Meta", nil, "")
	}

	// Sync to database
	synced := 0
	for _, metaTemplate := range templates {
		template := models.Template{
			OrganizationID:  orgID,
			WhatsAppAccount: account.Name,
			MetaTemplateID:  metaTemplate.ID,
			Name:            metaTemplate.Name,
			DisplayName:     metaTemplate.Name,
			Language:        metaTemplate.Language,
			Category:        metaTemplate.Category,
			Status:          metaTemplate.Status,
		}

		// Parse components
		for _, comp := range metaTemplate.Components {
			switch comp.Type {
			case "HEADER":
				template.HeaderType = comp.Format
				if comp.Text != "" {
					template.HeaderContent = comp.Text
				}
			case "BODY":
				template.BodyContent = comp.Text
			case "FOOTER":
				template.FooterContent = comp.Text
			case "BUTTONS":
				// Convert []TemplateButton to []interface{}
				buttons := make([]interface{}, len(comp.Buttons))
				for i, btn := range comp.Buttons {
					buttons[i] = btn
				}
				template.Buttons = convertToJSONBArray(buttons)
			}
		}

		// Upsert (including soft-deleted templates to restore them)
		existing := models.Template{}
		if err := a.DB.Unscoped().Where("organization_id = ? AND whats_app_account = ? AND name = ? AND language = ?",
			orgID, account.Name, template.Name, template.Language).First(&existing).Error; err == nil {
			// Update existing and restore if soft-deleted (explicitly set deleted_at to NULL)
			template.ID = existing.ID
			a.DB.Unscoped().Model(&template).Updates(map[string]interface{}{
				"meta_template_id": template.MetaTemplateID,
				"display_name":     template.DisplayName,
				"category":         template.Category,
				"status":           template.Status,
				"header_type":      template.HeaderType,
				"header_content":   template.HeaderContent,
				"body_content":     template.BodyContent,
				"footer_content":   template.FooterContent,
				"buttons":          template.Buttons,
				"deleted_at":       nil, // Restore soft-deleted template
			})
		} else {
			// Create new
			a.DB.Create(&template)
		}
		synced++
	}

	return r.SendEnvelope(map[string]interface{}{
		"message": fmt.Sprintf("Synced %d templates", synced),
		"count":   synced,
	})
}

func (a *App) fetchTemplatesFromMeta(account *models.WhatsAppAccount) ([]whatsapp.MetaTemplate, error) {
	waAccount := a.toWhatsAppAccount(account)

	ctx := context.Background()
	return a.WhatsApp.FetchTemplates(ctx, waAccount)
}

func (a *App) deleteTemplateFromMeta(account *models.WhatsAppAccount, templateName string) {
	waAccount := a.toWhatsAppAccount(account)

	ctx := context.Background()
	if err := a.WhatsApp.DeleteTemplate(ctx, waAccount, templateName); err != nil {
		a.Log.Error("Failed to delete template from Meta", "error", err, "template", templateName)
	}
}

// Helper functions

func templateToResponse(t models.Template) TemplateResponse {
	return TemplateResponse{
		ID:              t.ID,
		WhatsAppAccount: t.WhatsAppAccount,
		MetaTemplateID:  t.MetaTemplateID,
		Name:            t.Name,
		DisplayName:     t.DisplayName,
		Language:        t.Language,
		Category:        t.Category,
		Status:          t.Status,
		HeaderType:      t.HeaderType,
		HeaderContent:   t.HeaderContent,
		BodyContent:     t.BodyContent,
		FooterContent:   t.FooterContent,
		Buttons:         convertFromJSONBArray(t.Buttons),
		SampleValues:    convertFromJSONBArray(t.SampleValues),
		CreatedAt:       t.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:       t.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

func normalizeTemplateName(name string) string {
	// Convert to lowercase and replace spaces with underscores
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "_")
	name = strings.ReplaceAll(name, "-", "_")
	// Remove any non-alphanumeric characters except underscores
	var result strings.Builder
	for _, c := range name {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_' {
			result.WriteRune(c)
		}
	}
	return result.String()
}

func convertToJSONBArray(arr []interface{}) models.JSONBArray {
	if arr == nil {
		return models.JSONBArray{}
	}
	return models.JSONBArray(arr)
}

func convertFromJSONBArray(arr models.JSONBArray) []interface{} {
	if arr == nil {
		return []interface{}{}
	}
	return []interface{}(arr)
}

// UploadTemplateMedia uploads a media file for use as template header sample
// Returns a file handle that can be used in template creation
func (a *App) UploadTemplateMedia(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	// Get account name from form or query
	accountName := string(r.RequestCtx.FormValue("account"))
	if accountName == "" {
		accountName = string(r.RequestCtx.QueryArgs().Peek("account"))
	}
	if accountName == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "account is required", nil, "")
	}

	// Verify account belongs to organization
	var account models.WhatsAppAccount
	if err := a.DB.Where("name = ? AND organization_id = ?", accountName, orgID).First(&account).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "WhatsApp account not found", nil, "")
	}

	// Check if account has app_id configured
	if account.AppID == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "WhatsApp account does not have app_id configured. Please update the account settings.", nil, "")
	}

	// Get the uploaded file
	fileHeader, err := r.RequestCtx.FormFile("file")
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "No file provided", nil, "")
	}

	file, err := fileHeader.Open()
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to open uploaded file", nil, "")
	}
	defer func() { _ = file.Close() }()

	// Read file data
	fileData := make([]byte, fileHeader.Size)
	if _, err := file.Read(fileData); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to read file data", nil, "")
	}

	// Determine mime type from Content-Type header or filename
	mimeType := fileHeader.Header.Get("Content-Type")
	if mimeType == "" || mimeType == "application/octet-stream" {
		// Try to infer from filename
		filename := fileHeader.Filename
		switch {
		case strings.HasSuffix(strings.ToLower(filename), ".jpg") || strings.HasSuffix(strings.ToLower(filename), ".jpeg"):
			mimeType = "image/jpeg"
		case strings.HasSuffix(strings.ToLower(filename), ".png"):
			mimeType = "image/png"
		case strings.HasSuffix(strings.ToLower(filename), ".mp4"):
			mimeType = "video/mp4"
		case strings.HasSuffix(strings.ToLower(filename), ".pdf"):
			mimeType = "application/pdf"
		default:
			mimeType = "application/octet-stream"
		}
	}

	// Create whatsapp account with AppID
	waAccount := a.toWhatsAppAccount(&account)

	// Perform resumable upload to get handle
	ctx := context.Background()
	handle, err := a.WhatsApp.ResumableUpload(ctx, waAccount, fileData, mimeType, fileHeader.Filename)
	if err != nil {
		a.Log.Error("Failed to upload template media", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusBadGateway, "Failed to upload media to Meta", nil, "")
	}

	return r.SendEnvelope(map[string]interface{}{
		"handle":    handle,
		"filename":  fileHeader.Filename,
		"mime_type": mimeType,
		"size":      fileHeader.Size,
	})
}
