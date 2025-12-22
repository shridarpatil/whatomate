package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/internal/websocket"
	"github.com/shridarpatil/whatomate/pkg/whatsapp"
	"gorm.io/gorm"
)

// IncomingTextMessage represents a text or interactive message from the webhook
type IncomingTextMessage struct {
	From      string `json:"from"`
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	Type      string `json:"type"`
	Text      *struct {
		Body string `json:"body"`
	} `json:"text,omitempty"`
	Interactive *struct {
		Type        string `json:"type"`
		ButtonReply *struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"button_reply,omitempty"`
		ListReply *struct {
			ID          string `json:"id"`
			Title       string `json:"title"`
			Description string `json:"description"`
		} `json:"list_reply,omitempty"`
	} `json:"interactive,omitempty"`
}

// processIncomingMessageFull processes incoming WhatsApp messages with chatbot logic
func (a *App) processIncomingMessageFull(phoneNumberID string, msg IncomingTextMessage, profileName string) {
	a.Log.Info("Processing incoming message",
		"phone_number_id", phoneNumberID,
		"from", msg.From,
		"type", msg.Type,
		"profile_name", profileName,
	)

	// Find the WhatsApp account by phone_number_id
	var account models.WhatsAppAccount
	if err := a.DB.Where("phone_id = ?", phoneNumberID).First(&account).Error; err != nil {
		a.Log.Error("WhatsApp account not found", "phone_id", phoneNumberID, "error", err)
		return
	}

	// Get or create contact (always do this for all incoming messages)
	contact := a.getOrCreateContact(account.OrganizationID, msg.From, profileName)

	// Get message content - handle text, button replies, and list replies
	messageText := ""
	messageType := msg.Type
	buttonID := "" // Track button/list ID for conditional routing

	if msg.Type == "text" && msg.Text != nil {
		messageText = msg.Text.Body
	} else if msg.Type == "interactive" && msg.Interactive != nil {
		// Handle button reply
		if msg.Interactive.ButtonReply != nil {
			messageText = msg.Interactive.ButtonReply.Title
			buttonID = msg.Interactive.ButtonReply.ID
		}
		// Handle list reply
		if msg.Interactive.ListReply != nil {
			messageText = msg.Interactive.ListReply.Title
			buttonID = msg.Interactive.ListReply.ID
		}
	}

	// Save incoming message to messages table (always, even if chatbot is disabled)
	a.saveIncomingMessage(&account, contact, msg.ID, messageType, messageText)

	// Check for active agent transfer - skip chatbot processing if transferred
	if a.hasActiveAgentTransfer(account.OrganizationID, contact.ID) {
		a.Log.Info("Contact has active agent transfer, skipping chatbot processing",
			"contact_id", contact.ID,
			"phone_number", contact.PhoneNumber)
		return
	}

	// Check if chatbot is enabled for this account
	var settings models.ChatbotSettings
	result := a.DB.Where("organization_id = ? AND (whats_app_account = ? OR whats_app_account = '')",
		account.OrganizationID, account.Name).
		Order("CASE WHEN whats_app_account = '' THEN 1 ELSE 0 END"). // Prefer account-specific settings
		First(&settings)

	if result.Error != nil {
		a.Log.Error("Failed to load chatbot settings", "error", result.Error, "account", account.Name, "org_id", account.OrganizationID)
		return
	}
	if !settings.IsEnabled {
		a.Log.Debug("Chatbot not enabled for this account", "account", account.Name, "settings_id", settings.ID)
		return
	}
	a.Log.Info("Chatbot settings loaded", "settings_id", settings.ID, "is_enabled", settings.IsEnabled, "ai_enabled", settings.AIEnabled, "ai_provider", settings.AIProvider, "default_response", settings.DefaultResponse)

	// Check business hours if enabled
	if settings.BusinessHoursEnabled && len(settings.BusinessHours) > 0 {
		if !a.isWithinBusinessHours(settings.BusinessHours) {
			// If automated responses are not allowed outside hours, send out-of-hours message and stop
			if !settings.AllowAutomatedOutsideHours {
				a.Log.Info("Outside business hours, sending out of hours message")
				if settings.OutOfHoursMessage != "" {
					a.sendAndSaveTextMessage(&account, contact, settings.OutOfHoursMessage)
				}
				return
			}
			// AllowAutomatedOutsideHours is true, continue processing flows/keywords/AI
			a.Log.Info("Outside business hours but automated responses allowed, continuing")
		}
	}

	// Only process text and interactive messages for chatbot
	if messageText == "" {
		a.Log.Debug("Skipping message with no text content for chatbot", "type", msg.Type)
		return
	}

	a.Log.Info("Processing message", "text", messageText, "buttonID", buttonID, "from", msg.From)

	// Get or create active session for this contact
	session, isNewSession := a.getOrCreateSession(account.OrganizationID, contact.ID, account.Name, msg.From, settings.SessionTimeoutMins)

	// Log incoming message to session
	a.logSessionMessage(session.ID, "incoming", messageText, "keyword_check")

	// Check for transfer keyword BEFORE sending greeting (transfer takes priority)
	keywordResponse, keywordMatched := a.matchKeywordRules(account.OrganizationID, account.Name, messageText)
	if keywordMatched && keywordResponse.ResponseType == "transfer" {
		a.Log.Info("Transfer keyword matched", "response", keywordResponse.Body)
		// Check business hours - if outside hours, send out of hours message instead
		if settings.BusinessHoursEnabled && len(settings.BusinessHours) > 0 {
			if !a.isWithinBusinessHours(settings.BusinessHours) {
				a.Log.Info("Outside business hours, sending out of hours message instead of transfer")
				if settings.OutOfHoursMessage != "" {
					a.sendAndSaveTextMessage(&account, contact, settings.OutOfHoursMessage)
				}
				return
			}
		}
		// Within business hours - send transfer message and create transfer
		if keywordResponse.Body != "" {
			a.sendAndSaveTextMessage(&account, contact, keywordResponse.Body)
		}
		a.createTransferFromKeyword(&account, contact)
		return
	}

	// Check if user is in an active flow
	if session.CurrentFlowID != nil {
		a.processFlowResponse(&account, session, contact, messageText, buttonID)
		return
	}

	// Try to match flow trigger keywords first (before greeting to avoid duplicate messages)
	if flow := a.matchFlowTrigger(account.OrganizationID, account.Name, messageText); flow != nil {
		a.startFlow(&account, session, contact, flow)
		return
	}

	// Send greeting message for new sessions (only if no flow was triggered)
	if isNewSession && settings.DefaultResponse != "" {
		a.Log.Info("New session - sending greeting message", "contact", contact.PhoneNumber)
		if len(settings.GreetingButtons) > 0 {
			greetingButtons := make([]map[string]interface{}, 0)
			for _, btn := range settings.GreetingButtons {
				if btnMap, ok := btn.(map[string]interface{}); ok {
					greetingButtons = append(greetingButtons, btnMap)
				}
			}
			if len(greetingButtons) > 0 {
				a.sendAndSaveInteractiveButtons(&account, contact, settings.DefaultResponse, greetingButtons)
			} else {
				a.sendAndSaveTextMessage(&account, contact, settings.DefaultResponse)
			}
		} else {
			a.sendAndSaveTextMessage(&account, contact, settings.DefaultResponse)
		}
		a.logSessionMessage(session.ID, "outgoing", settings.DefaultResponse, "greeting")
		return // After greeting, don't process further for new sessions
	}

	// Handle non-transfer keyword matches (transfer was already handled above)
	if keywordMatched && keywordResponse.ResponseType != "transfer" {
		a.Log.Info("Keyword rule matched", "response_type", keywordResponse.ResponseType, "response", keywordResponse.Body)

		// Handle regular text response
		if len(keywordResponse.Buttons) > 0 {
			a.sendAndSaveInteractiveButtons(&account, contact, keywordResponse.Body, keywordResponse.Buttons)
		} else {
			a.sendAndSaveTextMessage(&account, contact, keywordResponse.Body)
		}
		// Log outgoing message
		a.logSessionMessage(session.ID, "outgoing", keywordResponse.Body, "keyword_response")
		return
	}

	// If no keyword matched, try AI response if enabled
	if settings.AIEnabled && settings.AIProvider != "" && settings.AIAPIKey != "" {
		a.Log.Info("Attempting AI response", "provider", settings.AIProvider, "model", settings.AIModel)
		aiResponse, err := a.generateAIResponse(&settings, session, messageText)
		if err != nil {
			a.Log.Error("AI response failed", "error", err, "provider", settings.AIProvider, "model", settings.AIModel)
			// Fall through to default response
		} else if aiResponse != "" {
			a.Log.Info("AI response generated successfully", "response_length", len(aiResponse))
			a.sendAndSaveTextMessage(&account, contact, aiResponse)
			a.logSessionMessage(session.ID, "outgoing", aiResponse, "ai_response")
			return
		} else {
			a.Log.Warn("AI returned empty response")
		}
	} else {
		a.Log.Info("AI not configured", "ai_enabled", settings.AIEnabled, "has_provider", settings.AIProvider != "", "has_api_key", settings.AIAPIKey != "")
	}

	// If no AI response or AI not enabled, send fallback message (for existing sessions)
	// Greeting is already sent for new sessions above
	if settings.FallbackMessage != "" && !isNewSession {
		a.Log.Info("Sending fallback message", "response", settings.FallbackMessage)
		if len(settings.FallbackButtons) > 0 {
			fallbackButtons := make([]map[string]interface{}, 0)
			for _, btn := range settings.FallbackButtons {
				if btnMap, ok := btn.(map[string]interface{}); ok {
					fallbackButtons = append(fallbackButtons, btnMap)
				}
			}
			if len(fallbackButtons) > 0 {
				a.sendAndSaveInteractiveButtons(&account, contact, settings.FallbackMessage, fallbackButtons)
			} else {
				a.sendAndSaveTextMessage(&account, contact, settings.FallbackMessage)
			}
		} else {
			a.sendAndSaveTextMessage(&account, contact, settings.FallbackMessage)
		}
		a.logSessionMessage(session.ID, "outgoing", settings.FallbackMessage, "fallback_response")
	} else if !isNewSession {
		a.Log.Info("No fallback message configured for existing session")
	}
}

// KeywordResponse holds the response content and optional buttons
type KeywordResponse struct {
	Body         string
	Buttons      []map[string]interface{}
	ResponseType string // text, transfer
}

// matchKeywordRules checks if the message matches any keyword rules
func (a *App) matchKeywordRules(orgID uuid.UUID, accountName, messageText string) (*KeywordResponse, bool) {
	var rules []models.KeywordRule
	err := a.DB.Where("organization_id = ? AND whats_app_account = ? AND is_enabled = true",
		orgID, accountName).
		Order("priority DESC").
		Find(&rules).Error

	if err != nil {
		a.Log.Error("Failed to fetch keyword rules", "error", err)
		return nil, false
	}

	// Also get org-level rules if no account-specific ones
	if len(rules) == 0 {
		a.DB.Where("organization_id = ? AND whats_app_account = '' AND is_enabled = true", orgID).
			Order("priority DESC").
			Find(&rules)
	}

	messageLower := strings.ToLower(messageText)

	for _, rule := range rules {
		for _, keyword := range rule.Keywords {
			keywordLower := strings.ToLower(keyword)
			matched := false

			switch rule.MatchType {
			case "exact":
				if rule.CaseSensitive {
					matched = messageText == keyword
				} else {
					matched = messageLower == keywordLower
				}
			case "contains":
				if rule.CaseSensitive {
					matched = strings.Contains(messageText, keyword)
				} else {
					matched = strings.Contains(messageLower, keywordLower)
				}
			case "starts_with":
				if rule.CaseSensitive {
					matched = strings.HasPrefix(messageText, keyword)
				} else {
					matched = strings.HasPrefix(messageLower, keywordLower)
				}
			case "regex":
				re, err := regexp.Compile(keyword)
				if err == nil {
					matched = re.MatchString(messageText)
				}
			default:
				// Default to contains
				matched = strings.Contains(messageLower, keywordLower)
			}

			if matched {
				response := &KeywordResponse{
					ResponseType: rule.ResponseType,
				}

				// For transfer type, use body as the transfer message
				if rule.ResponseType == "transfer" {
					if body, ok := rule.ResponseContent["body"].(string); ok {
						response.Body = body
					}
					return response, true
				}

				// Get response body
				if body, ok := rule.ResponseContent["body"].(string); ok {
					response.Body = body
				}

				// Get buttons if present
				if buttons, ok := rule.ResponseContent["buttons"].([]interface{}); ok && len(buttons) > 0 {
					response.Buttons = make([]map[string]interface{}, 0, len(buttons))
					for _, btn := range buttons {
						if btnMap, ok := btn.(map[string]interface{}); ok {
							response.Buttons = append(response.Buttons, btnMap)
						}
					}
				}

				if response.Body != "" {
					return response, true
				}
			}
		}
	}

	return nil, false
}

// sendTextMessage sends a text message via WhatsApp Cloud API
func (a *App) sendTextMessage(account *models.WhatsAppAccount, to, message string) error {
	waAccount := &whatsapp.Account{
		PhoneID:     account.PhoneID,
		BusinessID:  account.BusinessID,
		APIVersion:  account.APIVersion,
		AccessToken: account.AccessToken,
	}
	ctx := context.Background()
	_, err := a.WhatsApp.SendTextMessage(ctx, waAccount, to, message)
	return err
}

// sendAndSaveTextMessage sends a text message and saves it to the database
func (a *App) sendAndSaveTextMessage(account *models.WhatsAppAccount, contact *models.Contact, message string) error {
	waAccount := &whatsapp.Account{
		PhoneID:     account.PhoneID,
		BusinessID:  account.BusinessID,
		APIVersion:  account.APIVersion,
		AccessToken: account.AccessToken,
	}
	ctx := context.Background()
	wamid, err := a.WhatsApp.SendTextMessage(ctx, waAccount, contact.PhoneNumber, message)

	// Create message record
	msg := models.Message{
		OrganizationID:  account.OrganizationID,
		WhatsAppAccount: account.Name,
		ContactID:       contact.ID,
		Direction:       "outgoing",
		MessageType:     "text",
		Content:         message,
		Status:          "sent",
	}
	if err != nil {
		msg.Status = "failed"
		msg.ErrorMessage = err.Error()
	} else if wamid != "" {
		msg.WhatsAppMessageID = wamid
	}

	if dbErr := a.DB.Create(&msg).Error; dbErr != nil {
		a.Log.Error("Failed to save chatbot message", "error", dbErr)
	}

	// Broadcast via WebSocket
	if a.WSHub != nil {
		var assignedUserIDStr string
		if contact.AssignedUserID != nil {
			assignedUserIDStr = contact.AssignedUserID.String()
		}
		a.WSHub.BroadcastToOrg(account.OrganizationID, websocket.WSMessage{
			Type: websocket.TypeNewMessage,
			Payload: map[string]any{
				"id":               msg.ID,
				"contact_id":       contact.ID.String(),
				"assigned_user_id": assignedUserIDStr,
				"profile_name":     contact.ProfileName,
				"direction":        msg.Direction,
				"message_type":     msg.MessageType,
				"content":          map[string]string{"body": msg.Content},
				"status":           msg.Status,
				"wamid":            msg.WhatsAppMessageID,
				"created_at":       msg.CreatedAt,
				"updated_at":       msg.UpdatedAt,
			},
		})
	}

	return err
}

// sendAndSaveInteractiveButtons sends an interactive button message and saves it to the database
func (a *App) sendAndSaveInteractiveButtons(account *models.WhatsAppAccount, contact *models.Contact, bodyText string, buttons []map[string]interface{}) error {
	err := a.sendInteractiveButtons(account, contact.PhoneNumber, bodyText, buttons)

	// Create message record with interactive data
	// Convert buttons to []interface{} for JSONB storage
	buttonsInterface := make([]interface{}, len(buttons))
	for i, b := range buttons {
		buttonsInterface[i] = b
	}

	// Store differently based on button count (matching WhatsApp API format)
	// 3 or fewer = button type, more than 3 = list type
	var interactiveData models.JSONB
	if len(buttons) <= 3 {
		interactiveData = models.JSONB{
			"type":    "button",
			"body":    bodyText,
			"buttons": buttonsInterface,
		}
	} else {
		interactiveData = models.JSONB{
			"type": "list",
			"body": bodyText,
			"rows": buttonsInterface,
		}
	}

	msg := models.Message{
		OrganizationID:  account.OrganizationID,
		WhatsAppAccount: account.Name,
		ContactID:       contact.ID,
		Direction:       "outgoing",
		MessageType:     "interactive",
		Content:         bodyText,
		InteractiveData: interactiveData,
		Status:          "sent",
	}
	if err != nil {
		msg.Status = "failed"
		msg.ErrorMessage = err.Error()
	}

	if dbErr := a.DB.Create(&msg).Error; dbErr != nil {
		a.Log.Error("Failed to save chatbot interactive message", "error", dbErr)
	}

	// Broadcast via WebSocket
	if a.WSHub != nil {
		var assignedUserIDStr string
		if contact.AssignedUserID != nil {
			assignedUserIDStr = contact.AssignedUserID.String()
		}
		a.WSHub.BroadcastToOrg(account.OrganizationID, websocket.WSMessage{
			Type: websocket.TypeNewMessage,
			Payload: map[string]any{
				"id":               msg.ID,
				"contact_id":       contact.ID.String(),
				"assigned_user_id": assignedUserIDStr,
				"profile_name":     contact.ProfileName,
				"direction":        msg.Direction,
				"message_type":     msg.MessageType,
				"content":          map[string]string{"body": msg.Content},
				"interactive_data": msg.InteractiveData,
				"status":           msg.Status,
				"wamid":            msg.WhatsAppMessageID,
				"created_at":       msg.CreatedAt,
				"updated_at":       msg.UpdatedAt,
			},
		})
	}

	return err
}

// sendInteractiveButtons sends an interactive button or list message via WhatsApp Cloud API
// If 3 or fewer buttons, sends as button message; if more than 3, sends as list message (max 10)
func (a *App) sendInteractiveButtons(account *models.WhatsAppAccount, to, bodyText string, buttons []map[string]interface{}) error {
	// Convert buttons to whatsapp.Button format
	waButtons := make([]whatsapp.Button, 0, len(buttons))
	for i, btn := range buttons {
		if i >= 10 {
			break
		}
		buttonID, _ := btn["id"].(string)
		buttonTitle, _ := btn["title"].(string)
		if buttonID == "" {
			buttonID = fmt.Sprintf("btn_%d", i+1)
		}
		if buttonTitle == "" {
			continue
		}
		waButtons = append(waButtons, whatsapp.Button{
			ID:    buttonID,
			Title: buttonTitle,
		})
	}

	if len(waButtons) == 0 {
		return a.sendTextMessage(account, to, bodyText)
	}

	waAccount := &whatsapp.Account{
		PhoneID:     account.PhoneID,
		BusinessID:  account.BusinessID,
		APIVersion:  account.APIVersion,
		AccessToken: account.AccessToken,
	}
	ctx := context.Background()
	_, err := a.WhatsApp.SendInteractiveButtons(ctx, waAccount, to, bodyText, waButtons)
	return err
}

// getOrCreateContact finds or creates a contact for the phone number
func (a *App) getOrCreateContact(orgID uuid.UUID, phoneNumber, profileName string) *models.Contact {
	var contact models.Contact
	result := a.DB.Where("organization_id = ? AND phone_number = ?", orgID, phoneNumber).First(&contact)
	if result.Error == nil {
		// Update profile name if changed
		if profileName != "" && contact.ProfileName != profileName {
			a.DB.Model(&contact).Update("profile_name", profileName)
		}
		return &contact
	}

	// Create new contact
	contact = models.Contact{
		BaseModel:      models.BaseModel{ID: uuid.New()},
		OrganizationID: orgID,
		PhoneNumber:    phoneNumber,
		ProfileName:    profileName,
	}
	if err := a.DB.Create(&contact).Error; err != nil {
		a.Log.Error("Failed to create contact", "error", err)
		// Try to fetch again in case of race condition
		a.DB.Where("organization_id = ? AND phone_number = ?", orgID, phoneNumber).First(&contact)
	}
	return &contact
}

// getOrCreateSession finds an active session or creates a new one
// Returns the session and a boolean indicating if it's a new session
func (a *App) getOrCreateSession(orgID, contactID uuid.UUID, accountName, phoneNumber string, timeoutMins int) (*models.ChatbotSession, bool) {
	now := time.Now()

	// Look for an active session that hasn't timed out
	var session models.ChatbotSession
	timeout := now.Add(-time.Duration(timeoutMins) * time.Minute)
	result := a.DB.Where("organization_id = ? AND contact_id = ? AND whats_app_account = ? AND status = ? AND last_activity_at > ?",
		orgID, contactID, accountName, "active", timeout).First(&session)

	if result.Error == nil {
		// Update last activity
		a.DB.Model(&session).Update("last_activity_at", now)
		return &session, false // existing session
	}

	// Create new session
	session = models.ChatbotSession{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  orgID,
		ContactID:       contactID,
		WhatsAppAccount: accountName,
		PhoneNumber:     phoneNumber,
		Status:          "active",
		SessionData:     models.JSONB{},
		StartedAt:       now,
		LastActivityAt:  now,
	}
	if err := a.DB.Create(&session).Error; err != nil {
		a.Log.Error("Failed to create session", "error", err)
	}
	return &session, true // new session
}

// logSessionMessage logs a message to the chatbot session
func (a *App) logSessionMessage(sessionID uuid.UUID, direction, message, stepName string) {
	msg := models.ChatbotSessionMessage{
		BaseModel: models.BaseModel{ID: uuid.New()},
		SessionID: sessionID,
		Direction: direction,
		Message:   message,
		StepName:  stepName,
	}
	if err := a.DB.Create(&msg).Error; err != nil {
		a.Log.Error("Failed to log session message", "error", err)
	}
}

// matchFlowTrigger checks if the message triggers any flow
func (a *App) matchFlowTrigger(orgID uuid.UUID, accountName, messageText string) *models.ChatbotFlow {
	var flows []models.ChatbotFlow
	a.DB.Where("organization_id = ? AND is_enabled = true", orgID).
		Preload("Steps", func(db *gorm.DB) *gorm.DB {
			return db.Order("step_order ASC")
		}).
		Find(&flows)

	messageLower := strings.ToLower(messageText)

	for _, flow := range flows {
		for _, keyword := range flow.TriggerKeywords {
			if strings.Contains(messageLower, strings.ToLower(keyword)) {
				return &flow
			}
		}
	}
	return nil
}

// startFlow initiates a chatbot flow for a user
func (a *App) startFlow(account *models.WhatsAppAccount, session *models.ChatbotSession, contact *models.Contact, flow *models.ChatbotFlow) {
	a.Log.Info("Starting flow", "flow_id", flow.ID, "flow_name", flow.Name, "contact", contact.PhoneNumber)

	// Update session with flow info
	session.CurrentFlowID = &flow.ID
	session.CurrentStep = ""
	session.StepRetries = 0
	session.SessionData = models.JSONB{}
	a.DB.Save(session)

	// Send initial message if configured
	if flow.InitialMessage != "" {
		a.sendAndSaveTextMessage(account, contact, flow.InitialMessage)
		a.logSessionMessage(session.ID, "outgoing", flow.InitialMessage, "flow_start")
	}

	// Send first step message
	if len(flow.Steps) > 0 {
		firstStep := &flow.Steps[0]
		session.CurrentStep = firstStep.StepName
		a.DB.Model(session).Update("current_step", firstStep.StepName)

		a.sendStepMessage(account, session, contact, firstStep)
	} else {
		// No steps, complete the flow
		a.completeFlow(account, session, contact, flow)
	}
}

// processFlowResponse handles user response within a flow
func (a *App) processFlowResponse(account *models.WhatsAppAccount, session *models.ChatbotSession, contact *models.Contact, userInput string, buttonID string) {
	// Load the current flow
	var flow models.ChatbotFlow
	if err := a.DB.Where("id = ?", session.CurrentFlowID).
		Preload("Steps", func(db *gorm.DB) *gorm.DB {
			return db.Order("step_order ASC")
		}).
		First(&flow).Error; err != nil {
		a.Log.Error("Failed to load flow", "error", err)
		a.exitFlow(session)
		return
	}

	// Check for cancel keywords
	userInputLower := strings.ToLower(userInput)
	for _, cancelKw := range flow.CancelKeywords {
		if strings.Contains(userInputLower, strings.ToLower(cancelKw)) {
			a.sendAndSaveTextMessage(account, contact, "Flow cancelled.")
			a.logSessionMessage(session.ID, "outgoing", "Flow cancelled.", "flow_cancel")
			a.exitFlow(session)
			return
		}
	}

	// Find current step
	var currentStep *models.ChatbotFlowStep
	var currentStepIndex int
	for i, step := range flow.Steps {
		if step.StepName == session.CurrentStep {
			currentStep = &flow.Steps[i]
			currentStepIndex = i
			break
		}
	}

	if currentStep == nil {
		a.Log.Error("Current step not found", "step_name", session.CurrentStep)
		a.exitFlow(session)
		return
	}

	// Validate input if required (skip validation for button/list responses)
	if currentStep.ValidationRegex != "" && buttonID == "" {
		re, err := regexp.Compile(currentStep.ValidationRegex)
		if err == nil && !re.MatchString(userInput) {
			// Invalid input
			session.StepRetries++
			if currentStep.RetryOnInvalid && session.StepRetries < currentStep.MaxRetries {
				a.DB.Model(session).Update("step_retries", session.StepRetries)
				errorMsg := currentStep.ValidationError
				if errorMsg == "" {
					errorMsg = "Invalid input. Please try again."
				}
				a.sendAndSaveTextMessage(account, contact, errorMsg)
				a.logSessionMessage(session.ID, "outgoing", errorMsg, currentStep.StepName+"_retry")
				return
			}
			// Max retries exceeded, continue anyway or exit
			a.Log.Warn("Max retries exceeded", "step", currentStep.StepName)
		}
	}

	// Store the user's response (use buttonID if available, otherwise userInput)
	if currentStep.StoreAs != "" {
		sessionData := session.SessionData
		if sessionData == nil {
			sessionData = models.JSONB{}
		}
		// Store both the ID and the title for button responses
		if buttonID != "" {
			sessionData[currentStep.StoreAs] = buttonID
			sessionData[currentStep.StoreAs+"_title"] = userInput
		} else {
			sessionData[currentStep.StoreAs] = userInput
		}
		a.DB.Model(session).Update("session_data", sessionData)
		session.SessionData = sessionData
	}

	// Determine next step
	nextStepName := currentStep.NextStep
	if nextStepName == "" && currentStepIndex+1 < len(flow.Steps) {
		nextStepName = flow.Steps[currentStepIndex+1].StepName
	}

	// Check conditional next - use buttonID first (for button/list responses), then userInput
	if len(currentStep.ConditionalNext) > 0 {
		// Try buttonID first (for interactive responses)
		if buttonID != "" {
			if next, ok := currentStep.ConditionalNext[buttonID].(string); ok {
				nextStepName = next
			} else if next, ok := currentStep.ConditionalNext[userInput].(string); ok {
				nextStepName = next
			} else if defaultNext, ok := currentStep.ConditionalNext["default"].(string); ok {
				nextStepName = defaultNext
			}
		} else {
			// Text input - try matching the text
			if next, ok := currentStep.ConditionalNext[userInput].(string); ok {
				nextStepName = next
			} else if defaultNext, ok := currentStep.ConditionalNext["default"].(string); ok {
				nextStepName = defaultNext
			}
		}
	}

	// Move to next step or complete flow
	if nextStepName == "" {
		a.completeFlow(account, session, contact, &flow)
		return
	}

	// Find and execute next step
	var nextStep *models.ChatbotFlowStep
	for i, step := range flow.Steps {
		if step.StepName == nextStepName {
			nextStep = &flow.Steps[i]
			break
		}
	}

	if nextStep == nil {
		a.Log.Warn("Next step not found, completing flow", "next_step", nextStepName)
		a.completeFlow(account, session, contact, &flow)
		return
	}

	// Update session and send next step message
	a.DB.Model(session).Updates(map[string]interface{}{
		"current_step": nextStep.StepName,
		"step_retries": 0,
	})

	a.sendStepMessage(account, session, contact, nextStep)
}

// completeFlow finishes a flow and sends completion message
func (a *App) completeFlow(account *models.WhatsAppAccount, session *models.ChatbotSession, contact *models.Contact, flow *models.ChatbotFlow) {
	a.Log.Info("Completing flow", "flow_id", flow.ID, "session_id", session.ID)

	// Send completion message
	if flow.CompletionMessage != "" {
		message := a.replaceVariables(flow.CompletionMessage, session.SessionData)
		a.sendAndSaveTextMessage(account, contact, message)
		a.logSessionMessage(session.ID, "outgoing", message, "flow_complete")
	}

	// Execute on-complete action
	if flow.OnCompleteAction == "webhook" && len(flow.CompletionConfig) > 0 {
		go a.sendFlowCompletionWebhook(flow, session, contact)
	}

	// Update session
	now := time.Now()
	a.DB.Model(session).Updates(map[string]interface{}{
		"current_flow_id": nil,
		"current_step":    "",
		"status":          "completed",
		"completed_at":    now,
	})
}

// sendFlowCompletionWebhook sends session data to configured webhook URL
func (a *App) sendFlowCompletionWebhook(flow *models.ChatbotFlow, session *models.ChatbotSession, contact *models.Contact) {
	config := flow.CompletionConfig

	// Get webhook URL (required)
	webhookURL, ok := config["url"].(string)
	if !ok || webhookURL == "" {
		a.Log.Error("Webhook URL not configured", "flow_id", flow.ID)
		return
	}

	// Replace variables in URL
	webhookURL = a.replaceVariables(webhookURL, session.SessionData)

	// Get HTTP method (default: POST)
	method := "POST"
	if m, ok := config["method"].(string); ok && m != "" {
		method = strings.ToUpper(m)
	}

	// Build the payload
	payload := map[string]interface{}{
		"flow_id":      flow.ID.String(),
		"flow_name":    flow.Name,
		"session_id":   session.ID.String(),
		"phone_number": session.PhoneNumber,
		"contact_id":   contact.ID.String(),
		"contact_name": contact.ProfileName,
		"session_data": session.SessionData,
		"completed_at": time.Now().UTC().Format(time.RFC3339),
	}

	// Allow custom body template if provided
	var bodyReader io.Reader
	if bodyTemplate, ok := config["body"].(string); ok && bodyTemplate != "" {
		// Replace variables in body template
		bodyWithVars := a.replaceVariables(bodyTemplate, session.SessionData)
		bodyReader = strings.NewReader(bodyWithVars)
	} else {
		// Use default payload
		jsonPayload, err := json.Marshal(payload)
		if err != nil {
			a.Log.Error("Failed to marshal webhook payload", "error", err)
			return
		}
		bodyReader = bytes.NewReader(jsonPayload)
	}

	// Create request
	req, err := http.NewRequest(method, webhookURL, bodyReader)
	if err != nil {
		a.Log.Error("Failed to create webhook request", "error", err)
		return
	}

	// Set default headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Whatomate-Webhook/1.0")

	// Add custom headers if configured
	if headers, ok := config["headers"].(map[string]interface{}); ok {
		for key, value := range headers {
			if strVal, ok := value.(string); ok {
				req.Header.Set(key, a.replaceVariables(strVal, session.SessionData))
			}
		}
	}

	// Make the request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		a.Log.Error("Webhook request failed", "error", err, "url", webhookURL)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		a.Log.Info("Webhook sent successfully",
			"flow_id", flow.ID,
			"session_id", session.ID,
			"status", resp.StatusCode,
		)
	} else {
		a.Log.Error("Webhook returned error",
			"flow_id", flow.ID,
			"session_id", session.ID,
			"status", resp.StatusCode,
			"response", string(body),
		)
	}
}

// exitFlow clears flow state from session without completion
func (a *App) exitFlow(session *models.ChatbotSession) {
	a.DB.Model(session).Updates(map[string]interface{}{
		"current_flow_id": nil,
		"current_step":    "",
		"step_retries":    0,
	})
}

// replaceVariables replaces {{variable}} placeholders with session data values
func (a *App) replaceVariables(message string, data models.JSONB) string {
	if data == nil {
		return message
	}
	result := message
	for key, value := range data {
		placeholder := "{{" + key + "}}"
		if strVal, ok := value.(string); ok {
			result = strings.ReplaceAll(result, placeholder, strVal)
		}
	}
	return result
}

// sendStepMessage sends the appropriate message based on step message_type
func (a *App) sendStepMessage(account *models.WhatsAppAccount, session *models.ChatbotSession, contact *models.Contact, step *models.ChatbotFlowStep) {
	var message string

	switch step.MessageType {
	case "api_fetch":
		// Fetch response from external API (may include message + buttons)
		apiResp, err := a.fetchApiResponse(step.ApiConfig, session.SessionData)
		if err != nil {
			a.Log.Error("Failed to fetch API response", "error", err, "step", step.StepName)
			// Use fallback message if configured, otherwise use the step message
			if fallback, ok := step.ApiConfig["fallback_message"].(string); ok && fallback != "" {
				message = a.replaceVariables(fallback, session.SessionData)
			} else if step.Message != "" {
				message = a.replaceVariables(step.Message, session.SessionData)
			} else {
				message = "Sorry, there was an error processing your request."
			}
			a.sendAndSaveTextMessage(account, contact, message)
		} else {
			message = apiResp.Message
			// Check if API returned buttons
			if len(apiResp.Buttons) > 0 {
				a.sendAndSaveInteractiveButtons(account, contact, message, apiResp.Buttons)
			} else {
				a.sendAndSaveTextMessage(account, contact, message)
			}
		}
		a.logSessionMessage(session.ID, "outgoing", message, step.StepName)

	case "buttons":
		// Send interactive buttons message
		message = a.replaceVariables(step.Message, session.SessionData)
		if len(step.Buttons) > 0 {
			// Convert JSONBArray to []map[string]interface{}
			buttons := make([]map[string]interface{}, 0, len(step.Buttons))
			for _, btn := range step.Buttons {
				if btnMap, ok := btn.(map[string]interface{}); ok {
					buttons = append(buttons, btnMap)
				}
			}
			a.sendAndSaveInteractiveButtons(account, contact, message, buttons)
		} else {
			// No buttons configured, fall back to text
			a.sendAndSaveTextMessage(account, contact, message)
		}
		a.logSessionMessage(session.ID, "outgoing", message, step.StepName)

	default:
		// Default: use the step message with variable replacement
		message = a.replaceVariables(step.Message, session.SessionData)
		a.sendAndSaveTextMessage(account, contact, message)
		a.logSessionMessage(session.ID, "outgoing", message, step.StepName)
	}
}

// ApiResponse represents a response from an external API that may include buttons
type ApiResponse struct {
	Message string
	Buttons []map[string]interface{}
}

// fetchApiResponse fetches a response from an external API, supporting message + buttons
func (a *App) fetchApiResponse(apiConfig models.JSONB, sessionData models.JSONB) (*ApiResponse, error) {
	if apiConfig == nil {
		return nil, fmt.Errorf("API config is empty")
	}

	// Get API URL (required)
	apiURL, ok := apiConfig["url"].(string)
	if !ok || apiURL == "" {
		return nil, fmt.Errorf("API URL is required")
	}

	// Replace variables in URL
	apiURL = a.replaceVariables(apiURL, sessionData)

	// Get HTTP method (default: GET)
	method := "GET"
	if m, ok := apiConfig["method"].(string); ok && m != "" {
		method = strings.ToUpper(m)
	}

	// Prepare request body if configured
	var bodyReader io.Reader
	if bodyTemplate, ok := apiConfig["body"].(string); ok && bodyTemplate != "" {
		bodyWithVars := a.replaceVariables(bodyTemplate, sessionData)
		bodyReader = strings.NewReader(bodyWithVars)
	}

	// Create request
	req, err := http.NewRequest(method, apiURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set default headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Add custom headers if configured
	if headers, ok := apiConfig["headers"].(map[string]interface{}); ok {
		for key, value := range headers {
			if strVal, ok := value.(string); ok {
				// Replace variables in header values
				req.Header.Set(key, a.replaceVariables(strVal, sessionData))
			}
		}
	}

	// Make the request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse JSON response
	var jsonResp map[string]interface{}
	if err := json.Unmarshal(respBody, &jsonResp); err != nil {
		// If not JSON, return raw response as message
		return &ApiResponse{Message: string(respBody)}, nil
	}

	result := &ApiResponse{}

	// Extract message - check for "message" field first, then use response_path
	if msg, ok := jsonResp["message"].(string); ok {
		result.Message = msg
	} else {
		// Try response_path for backwards compatibility
		responsePath, _ := apiConfig["response_path"].(string)
		if responsePath != "" {
			result.Message = a.extractJsonPath(jsonResp, responsePath)
		} else {
			// No message found, return raw response
			result.Message = string(respBody)
		}
	}

	// Extract buttons if present - format: [{"id": "test", "value": "Test"}, ...]
	if buttons, ok := jsonResp["buttons"].([]interface{}); ok && len(buttons) > 0 {
		result.Buttons = make([]map[string]interface{}, 0, len(buttons))
		for _, btn := range buttons {
			if btnMap, ok := btn.(map[string]interface{}); ok {
				// Normalize button format: ensure we have "id" and "title"
				normalizedBtn := make(map[string]interface{})

				// Handle "id" field
				if id, ok := btnMap["id"].(string); ok {
					normalizedBtn["id"] = id
				}

				// Handle "value" or "title" for display text
				if value, ok := btnMap["value"].(string); ok {
					normalizedBtn["title"] = value
				} else if title, ok := btnMap["title"].(string); ok {
					normalizedBtn["title"] = title
				}

				if normalizedBtn["id"] != nil && normalizedBtn["title"] != nil {
					result.Buttons = append(result.Buttons, normalizedBtn)
				}
			}
		}
	}

	return result, nil
}

// fetchApiMessage fetches a message from an external API (legacy function for backwards compatibility)
func (a *App) fetchApiMessage(apiConfig models.JSONB, sessionData models.JSONB) (string, error) {
	resp, err := a.fetchApiResponse(apiConfig, sessionData)
	if err != nil {
		return "", err
	}
	return resp.Message, nil
}

// extractJsonPath extracts a value from a JSON object using dot notation path
func (a *App) extractJsonPath(data interface{}, path string) string {
	parts := strings.Split(path, ".")
	current := data

	for _, part := range parts {
		switch v := current.(type) {
		case map[string]interface{}:
			current = v[part]
		default:
			return ""
		}
	}

	// Convert final value to string
	switch v := current.(type) {
	case string:
		return v
	case float64:
		return fmt.Sprintf("%v", v)
	case bool:
		return fmt.Sprintf("%v", v)
	case nil:
		return ""
	default:
		// For complex types, marshal to JSON string
		if jsonBytes, err := json.Marshal(v); err == nil {
			return string(jsonBytes)
		}
		return fmt.Sprintf("%v", v)
	}
}

// generateAIResponse generates a response using the configured AI provider
func (a *App) generateAIResponse(settings *models.ChatbotSettings, session *models.ChatbotSession, userMessage string) (string, error) {
	// Build context from AIContext entries
	contextData := a.buildAIContext(settings.OrganizationID, session, userMessage)

	switch settings.AIProvider {
	case "openai":
		return a.generateOpenAIResponse(settings, session, userMessage, contextData)
	case "anthropic":
		return a.generateAnthropicResponse(settings, session, userMessage, contextData)
	case "google":
		return a.generateGoogleResponse(settings, session, userMessage, contextData)
	default:
		return "", fmt.Errorf("unsupported AI provider: %s", settings.AIProvider)
	}
}

// buildAIContext fetches and combines all AI context data
func (a *App) buildAIContext(orgID uuid.UUID, session *models.ChatbotSession, userMessage string) string {
	var contexts []models.AIContext
	query := a.DB.Where("organization_id = ? AND is_enabled = true", orgID)

	// Include org-level and account-specific contexts
	if session != nil && session.WhatsAppAccount != "" {
		query = query.Where("whats_app_account = ? OR whats_app_account = ''", session.WhatsAppAccount)
	}

	query.Order("priority DESC").Find(&contexts)

	if len(contexts) == 0 {
		return ""
	}

	var contextParts []string

	for _, ctx := range contexts {
		var content string

		switch ctx.ContextType {
		case "static":
			content = ctx.StaticContent

		case "api":
			// Start with static content/prompt if provided
			content = ctx.StaticContent

			// Fetch data from external API and append
			apiContent, err := a.fetchAPIContext(ctx.ApiConfig, session, userMessage)
			if err != nil {
				a.Log.Error("Failed to fetch API context", "context_name", ctx.Name, "error", err)
				// Still use static content if API fails
			} else if apiContent != "" {
				if content != "" {
					content = content + "\n\nData:\n" + apiContent
				} else {
					content = apiContent
				}
			}
		}

		if content != "" {
			contextParts = append(contextParts, fmt.Sprintf("### %s\n%s", ctx.Name, content))
		}
	}

	if len(contextParts) == 0 {
		return ""
	}

	return "## Context Information\n\n" + strings.Join(contextParts, "\n\n")
}

// fetchAPIContext fetches context data from an external API
func (a *App) fetchAPIContext(apiConfig models.JSONB, session *models.ChatbotSession, userMessage string) (string, error) {
	if apiConfig == nil {
		return "", fmt.Errorf("API config is empty")
	}

	// Get API URL (required)
	apiURL, ok := apiConfig["url"].(string)
	if !ok || apiURL == "" {
		return "", fmt.Errorf("API URL is required")
	}

	// Build session data for variable replacement
	sessionData := models.JSONB{}
	if session != nil {
		sessionData = session.SessionData
		if sessionData == nil {
			sessionData = models.JSONB{}
		}
		sessionData["phone_number"] = session.PhoneNumber
		sessionData["user_message"] = userMessage
	}

	// Replace variables in URL
	apiURL = a.replaceVariables(apiURL, sessionData)

	// Get HTTP method (default: GET)
	method := "GET"
	if m, ok := apiConfig["method"].(string); ok && m != "" {
		method = strings.ToUpper(m)
	}

	// Prepare request body if configured
	var bodyReader io.Reader
	if bodyTemplate, ok := apiConfig["body"].(string); ok && bodyTemplate != "" {
		bodyWithVars := a.replaceVariables(bodyTemplate, sessionData)
		bodyReader = strings.NewReader(bodyWithVars)
	}

	// Create request
	req, err := http.NewRequest(method, apiURL, bodyReader)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set default headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Add custom headers if configured
	if headers, ok := apiConfig["headers"].(map[string]interface{}); ok {
		for key, value := range headers {
			if strVal, ok := value.(string); ok {
				req.Header.Set(key, a.replaceVariables(strVal, sessionData))
			}
		}
	}

	// Make the request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	// Check for response_path to extract specific field
	if responsePath, ok := apiConfig["response_path"].(string); ok && responsePath != "" {
		var jsonResp map[string]interface{}
		if err := json.Unmarshal(respBody, &jsonResp); err == nil {
			return a.extractJsonPath(jsonResp, responsePath), nil
		}
	}

	return string(respBody), nil
}

// generateOpenAIResponse generates a response using OpenAI API
func (a *App) generateOpenAIResponse(settings *models.ChatbotSettings, session *models.ChatbotSession, userMessage string, contextData string) (string, error) {
	url := "https://api.openai.com/v1/chat/completions"

	// Build messages array
	messages := []map[string]string{}

	// Build system prompt with context
	systemPrompt := settings.AISystemPrompt
	if contextData != "" {
		if systemPrompt != "" {
			systemPrompt = systemPrompt + "\n\n" + contextData
		} else {
			systemPrompt = contextData
		}
	}

	// Add system prompt if configured
	if systemPrompt != "" {
		messages = append(messages, map[string]string{
			"role":    "system",
			"content": systemPrompt,
		})
	}

	// Add conversation history if enabled
	if settings.AIIncludeHistory && session != nil {
		history := a.getSessionHistory(session.ID, settings.AIHistoryLimit)
		for _, msg := range history {
			role := "user"
			if msg.Direction == "outgoing" {
				role = "assistant"
			}
			messages = append(messages, map[string]string{
				"role":    role,
				"content": msg.Message,
			})
		}
	}

	// Add current user message
	messages = append(messages, map[string]string{
		"role":    "user",
		"content": userMessage,
	})

	payload := map[string]interface{}{
		"model":      settings.AIModel,
		"messages":   messages,
		"max_tokens": settings.AIMaxTokens,
	}

	if settings.AITemperature > 0 {
		payload["temperature"] = settings.AITemperature
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+settings.AIAPIKey)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		var errResp struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		json.Unmarshal(body, &errResp)
		return "", fmt.Errorf("OpenAI API error: %s", errResp.Error.Message)
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if len(result.Choices) > 0 {
		return strings.TrimSpace(result.Choices[0].Message.Content), nil
	}

	return "", fmt.Errorf("no response from OpenAI")
}

// generateAnthropicResponse generates a response using Anthropic API
func (a *App) generateAnthropicResponse(settings *models.ChatbotSettings, session *models.ChatbotSession, userMessage string, contextData string) (string, error) {
	url := "https://api.anthropic.com/v1/messages"

	// Build messages array
	messages := []map[string]string{}

	// Add conversation history if enabled
	if settings.AIIncludeHistory && session != nil {
		history := a.getSessionHistory(session.ID, settings.AIHistoryLimit)
		for _, msg := range history {
			role := "user"
			if msg.Direction == "outgoing" {
				role = "assistant"
			}
			messages = append(messages, map[string]string{
				"role":    role,
				"content": msg.Message,
			})
		}
	}

	// Add current user message
	messages = append(messages, map[string]string{
		"role":    "user",
		"content": userMessage,
	})

	payload := map[string]interface{}{
		"model":      settings.AIModel,
		"messages":   messages,
		"max_tokens": settings.AIMaxTokens,
	}

	// Build system prompt with context
	systemPrompt := settings.AISystemPrompt
	if contextData != "" {
		if systemPrompt != "" {
			systemPrompt = systemPrompt + "\n\n" + contextData
		} else {
			systemPrompt = contextData
		}
	}

	// Add system prompt if configured
	if systemPrompt != "" {
		payload["system"] = systemPrompt
	}

	if settings.AITemperature > 0 {
		payload["temperature"] = settings.AITemperature
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", settings.AIAPIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		var errResp struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		json.Unmarshal(body, &errResp)
		return "", fmt.Errorf("Anthropic API error: %s", errResp.Error.Message)
	}

	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	for _, content := range result.Content {
		if content.Type == "text" {
			return strings.TrimSpace(content.Text), nil
		}
	}

	return "", fmt.Errorf("no text response from Anthropic")
}

// generateGoogleResponse generates a response using Google Gemini API
func (a *App) generateGoogleResponse(settings *models.ChatbotSettings, session *models.ChatbotSession, userMessage string, contextData string) (string, error) {
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		settings.AIModel, settings.AIAPIKey)

	// Build contents array
	contents := []map[string]interface{}{}

	// Add conversation history if enabled
	if settings.AIIncludeHistory && session != nil {
		history := a.getSessionHistory(session.ID, settings.AIHistoryLimit)
		for _, msg := range history {
			role := "user"
			if msg.Direction == "outgoing" {
				role = "model"
			}
			contents = append(contents, map[string]interface{}{
				"role": role,
				"parts": []map[string]string{
					{"text": msg.Message},
				},
			})
		}
	}

	// Add current user message
	contents = append(contents, map[string]interface{}{
		"role": "user",
		"parts": []map[string]string{
			{"text": userMessage},
		},
	})

	payload := map[string]interface{}{
		"contents": contents,
		"generationConfig": map[string]interface{}{
			"maxOutputTokens": settings.AIMaxTokens,
		},
	}

	// Build system prompt with context
	systemPrompt := settings.AISystemPrompt
	if contextData != "" {
		if systemPrompt != "" {
			systemPrompt = systemPrompt + "\n\n" + contextData
		} else {
			systemPrompt = contextData
		}
	}

	// Add system instruction if configured
	if systemPrompt != "" {
		payload["systemInstruction"] = map[string]interface{}{
			"parts": []map[string]string{
				{"text": systemPrompt},
			},
		}
	}

	if settings.AITemperature > 0 {
		payload["generationConfig"].(map[string]interface{})["temperature"] = settings.AITemperature
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		var errResp struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		json.Unmarshal(body, &errResp)
		return "", fmt.Errorf("Google AI API error: %s", errResp.Error.Message)
	}

	var result struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if len(result.Candidates) > 0 && len(result.Candidates[0].Content.Parts) > 0 {
		return strings.TrimSpace(result.Candidates[0].Content.Parts[0].Text), nil
	}

	return "", fmt.Errorf("no response from Google AI")
}

// getSessionHistory retrieves recent messages from the session
func (a *App) getSessionHistory(sessionID uuid.UUID, limit int) []models.ChatbotSessionMessage {
	var messages []models.ChatbotSessionMessage
	a.DB.Where("session_id = ?", sessionID).
		Order("created_at DESC").
		Limit(limit).
		Find(&messages)

	// Reverse to get chronological order
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages
}

// saveIncomingMessage saves an incoming message to the messages table
func (a *App) saveIncomingMessage(account *models.WhatsAppAccount, contact *models.Contact, whatsappMsgID, msgType, content string) {
	now := time.Now()

	message := models.Message{
		BaseModel:         models.BaseModel{ID: uuid.New()},
		OrganizationID:    account.OrganizationID,
		WhatsAppAccount:   account.Name,
		ContactID:         contact.ID,
		WhatsAppMessageID: whatsappMsgID,
		Direction:         "incoming",
		MessageType:       msgType,
		Content:           content,
		Status:            "received",
	}

	if err := a.DB.Create(&message).Error; err != nil {
		a.Log.Error("Failed to save incoming message", "error", err)
		return
	}

	// Update contact's last message info
	preview := content
	if len(preview) > 100 {
		preview = preview[:97] + "..."
	}
	if msgType != "text" {
		preview = "[" + msgType + "]"
	}

	a.DB.Model(contact).Updates(map[string]interface{}{
		"last_message_at":      now,
		"last_message_preview": preview,
		"is_read":              false,
		"whats_app_account":    account.Name,
	})

	a.Log.Info("Saved incoming message", "message_id", message.ID, "contact_id", contact.ID)

	// Broadcast new message via WebSocket
	if a.WSHub != nil {
		var assignedUserIDStr string
		if contact.AssignedUserID != nil {
			assignedUserIDStr = contact.AssignedUserID.String()
		}
		a.WSHub.BroadcastToOrg(account.OrganizationID, websocket.WSMessage{
			Type: websocket.TypeNewMessage,
			Payload: map[string]any{
				"id":               message.ID,
				"contact_id":       contact.ID.String(),
				"assigned_user_id": assignedUserIDStr,
				"profile_name":     contact.ProfileName,
				"direction":        message.Direction,
				"message_type":     message.MessageType,
				"content":          map[string]string{"body": message.Content},
				"status":           message.Status,
				"wamid":            message.WhatsAppMessageID,
				"created_at":       message.CreatedAt,
				"updated_at":       message.UpdatedAt,
			},
		})
	}
}

// isWithinBusinessHours checks if current time is within configured business hours
func (a *App) isWithinBusinessHours(businessHours models.JSONBArray) bool {
	now := time.Now()
	currentDay := int(now.Weekday()) // 0 = Sunday, 1 = Monday, etc.
	currentTime := now.Format("15:04")

	for _, bh := range businessHours {
		bhMap, ok := bh.(map[string]interface{})
		if !ok {
			continue
		}

		// Get day (0-6, Sunday-Saturday)
		day, ok := bhMap["day"].(float64)
		if !ok {
			continue
		}

		if int(day) != currentDay {
			continue
		}

		// Check if enabled for this day
		enabled, ok := bhMap["enabled"].(bool)
		if !ok || !enabled {
			return false // Day exists but is disabled
		}

		// Get start and end times
		startTime, ok := bhMap["start_time"].(string)
		if !ok {
			continue
		}
		endTime, ok := bhMap["end_time"].(string)
		if !ok {
			continue
		}

		// Compare times (simple string comparison works for HH:MM format)
		if currentTime >= startTime && currentTime <= endTime {
			return true
		}
		return false // Found the day but outside hours
	}

	// If no matching day found, assume outside business hours
	return false
}
