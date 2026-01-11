package handlers

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/pkg/whatsapp"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

// SendTemplateMessageRequest represents the request to send a template message
type SendTemplateMessageRequest struct {
	ContactID      string            `json:"contact_id"`
	PhoneNumber    string            `json:"phone_number"`     // Alternative to contact_id - send to phone directly
	TemplateName   string            `json:"template_name"`    // Template name
	TemplateID     string            `json:"template_id"`      // Alternative: template UUID
	TemplateParams map[string]string `json:"template_params"`  // Named or positional params
	AccountName    string            `json:"account_name"`     // Optional: specific WhatsApp account
}

// SendTemplateMessage sends a template message to a contact or phone number
func (a *App) SendTemplateMessage(r *fastglue.Request) error {
	orgID, err := getOrganizationID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}
	userID, _ := r.RequestCtx.UserValue("user_id").(uuid.UUID)

	var req SendTemplateMessageRequest
	if err := r.Decode(&req, "json"); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid request body", nil, "")
	}

	// Must have either contact_id or phone_number
	if req.ContactID == "" && req.PhoneNumber == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Either contact_id or phone_number is required", nil, "")
	}

	// Must have either template_name or template_id
	if req.TemplateName == "" && req.TemplateID == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Either template_name or template_id is required", nil, "")
	}

	// Get template
	var template models.Template
	if req.TemplateID != "" {
		templateID, err := uuid.Parse(req.TemplateID)
		if err != nil {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid template_id", nil, "")
		}
		if err := a.DB.Where("id = ? AND organization_id = ?", templateID, orgID).First(&template).Error; err != nil {
			return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Template not found", nil, "")
		}
	} else {
		if err := a.DB.Where("name = ? AND organization_id = ?", req.TemplateName, orgID).First(&template).Error; err != nil {
			return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Template not found", nil, "")
		}
	}

	// Check template is approved
	if template.Status != "APPROVED" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, fmt.Sprintf("Template is not approved (status: %s)", template.Status), nil, "")
	}

	// Get contact or use phone number directly
	var contact *models.Contact
	var phoneNumber string
	var contactID *uuid.UUID

	if req.ContactID != "" {
		cID, err := uuid.Parse(req.ContactID)
		if err != nil {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid contact_id", nil, "")
		}
		var c models.Contact
		if err := a.DB.Where("id = ? AND organization_id = ?", cID, orgID).First(&c).Error; err != nil {
			return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Contact not found", nil, "")
		}
		contact = &c
		phoneNumber = c.PhoneNumber
		contactID = &cID
	} else {
		phoneNumber = req.PhoneNumber
	}

	// Get WhatsApp account
	var account models.WhatsAppAccount
	if req.AccountName != "" {
		if err := a.DB.Where("name = ? AND organization_id = ?", req.AccountName, orgID).First(&account).Error; err != nil {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "WhatsApp account not found", nil, "")
		}
	} else if template.WhatsAppAccount != "" {
		if err := a.DB.Where("name = ? AND organization_id = ?", template.WhatsAppAccount, orgID).First(&account).Error; err != nil {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Template's WhatsApp account not found", nil, "")
		}
	} else if contact != nil && contact.WhatsAppAccount != "" {
		if err := a.DB.Where("name = ? AND organization_id = ?", contact.WhatsAppAccount, orgID).First(&account).Error; err != nil {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Contact's WhatsApp account not found", nil, "")
		}
	} else {
		// Get default outgoing account
		if err := a.DB.Where("organization_id = ? AND is_default_outgoing = ?", orgID, true).First(&account).Error; err != nil {
			if err := a.DB.Where("organization_id = ?", orgID).First(&account).Error; err != nil {
				return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "No WhatsApp account configured", nil, "")
			}
		}
	}

	// Extract parameter names and resolve values
	paramNames := extractParamNamesFromContent(template.BodyContent)
	bodyParams := resolveParams(paramNames, req.TemplateParams)

	// Create message record
	message := models.Message{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  orgID,
		WhatsAppAccount: account.Name,
		Direction:       models.DirectionOutgoing,
		MessageType:     "template",
		Content:         fmt.Sprintf("[Template: %s]", template.DisplayName),
		Status:          models.MessageStatusPending,
		SentByUserID:    &userID,
		Metadata: models.JSONB{
			"template_name":   template.Name,
			"template_id":     template.ID.String(),
			"template_params": req.TemplateParams,
		},
	}

	if contactID != nil {
		message.ContactID = *contactID
	}

	if err := a.DB.Create(&message).Error; err != nil {
		a.Log.Error("Failed to create message", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to create message", nil, "")
	}

	// Send via WhatsApp API
	go a.sendTemplateMessageAsync(&account, phoneNumber, &template, bodyParams, &message)

	// Update contact's last message if we have a contact
	if contact != nil {
		now := time.Now()
		a.DB.Model(contact).Updates(map[string]any{
			"last_message_at":      now,
			"last_message_preview": fmt.Sprintf("[Template: %s]", template.DisplayName),
		})
	}

	return r.SendEnvelope(map[string]any{
		"message_id":    message.ID,
		"status":        "pending",
		"template_name": template.Name,
		"phone_number":  phoneNumber,
	})
}

// sendTemplateMessageAsync sends template message and updates status
func (a *App) sendTemplateMessageAsync(account *models.WhatsAppAccount, phoneNumber string, template *models.Template, bodyParams []string, message *models.Message) {
	waAccount := &whatsapp.Account{
		PhoneID:     account.PhoneID,
		BusinessID:  account.BusinessID,
		AppID:       account.AppID,
		APIVersion:  account.APIVersion,
		AccessToken: account.AccessToken,
	}

	ctx := context.Background()
	waMessageID, err := a.WhatsApp.SendTemplateMessage(ctx, waAccount, phoneNumber, template.Name, template.Language, bodyParams)

	if err != nil {
		a.Log.Error("Failed to send template message", "error", err, "template", template.Name, "phone", phoneNumber)
		a.DB.Model(message).Updates(map[string]any{
			"status":        models.MessageStatusFailed,
			"error_message": err.Error(),
		})
		return
	}

	a.DB.Model(message).Updates(map[string]any{
		"status":               models.MessageStatusSent,
		"whats_app_message_id": waMessageID,
	})

	a.Log.Info("Template message sent", "message_id", message.ID, "wa_message_id", waMessageID, "template", template.Name)
}

// extractParamNamesFromContent extracts parameter names from template content
// Supports both positional ({{1}}, {{2}}) and named ({{name}}, {{order_id}}) parameters
var templateParamPattern = regexp.MustCompile(`\{\{([^}]+)\}\}`)

func extractParamNamesFromContent(content string) []string {
	matches := templateParamPattern.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil
	}

	seen := make(map[string]bool)
	var names []string
	for _, match := range matches {
		if len(match) > 1 {
			name := strings.TrimSpace(match[1])
			if name != "" && !seen[name] {
				seen[name] = true
				names = append(names, name)
			}
		}
	}
	return names
}

// resolveParams resolves both positional and named parameters to ordered values
func resolveParams(paramNames []string, params map[string]string) []string {
	if len(paramNames) == 0 || len(params) == 0 {
		return nil
	}

	result := make([]string, len(paramNames))
	for i, name := range paramNames {
		// Try named key first
		if val, ok := params[name]; ok {
			result[i] = val
			continue
		}
		// Fall back to positional key (1-indexed)
		key := fmt.Sprintf("%d", i+1)
		if val, ok := params[key]; ok {
			result[i] = val
			continue
		}
		// Default to empty string
		result[i] = ""
	}
	return result
}
