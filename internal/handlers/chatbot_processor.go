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
)

// IncomingTextMessage represents a text, interactive, or media message from the webhook
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
	Image *struct {
		ID       string `json:"id"`
		MimeType string `json:"mime_type"`
		SHA256   string `json:"sha256"`
		Caption  string `json:"caption,omitempty"`
	} `json:"image,omitempty"`
	Document *struct {
		ID       string `json:"id"`
		MimeType string `json:"mime_type"`
		SHA256   string `json:"sha256"`
		Filename string `json:"filename,omitempty"`
		Caption  string `json:"caption,omitempty"`
	} `json:"document,omitempty"`
	Audio *struct {
		ID       string `json:"id"`
		MimeType string `json:"mime_type"`
	} `json:"audio,omitempty"`
	Video *struct {
		ID       string `json:"id"`
		MimeType string `json:"mime_type"`
		SHA256   string `json:"sha256"`
		Caption  string `json:"caption,omitempty"`
	} `json:"video,omitempty"`
	Sticker *struct {
		ID       string `json:"id"`
		MimeType string `json:"mime_type"`
		SHA256   string `json:"sha256"`
		Animated bool   `json:"animated,omitempty"`
	} `json:"sticker,omitempty"`
	Context *struct {
		From string `json:"from"`
		ID   string `json:"id"` // WhatsApp message ID being replied to
	} `json:"context,omitempty"`
	Reaction *struct {
		MessageID string `json:"message_id"` // WhatsApp message ID being reacted to
		Emoji     string `json:"emoji"`      // The emoji reaction (empty string = remove reaction)
	} `json:"reaction,omitempty"`
	Location *struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
		Name      string  `json:"name,omitempty"`
		Address   string  `json:"address,omitempty"`
	} `json:"location,omitempty"`
	Contacts []struct {
		Name struct {
			FormattedName string `json:"formatted_name"`
			FirstName     string `json:"first_name,omitempty"`
			LastName      string `json:"last_name,omitempty"`
		} `json:"name"`
		Phones []struct {
			Phone string `json:"phone"`
			Type  string `json:"type,omitempty"`
		} `json:"phones,omitempty"`
	} `json:"contacts,omitempty"`
}

// processIncomingMessageFull processes incoming WhatsApp messages with chatbot logic
func (a *App) processIncomingMessageFull(phoneNumberID string, msg IncomingTextMessage, profileName string) {
	a.Log.Info("Processing incoming message",
		"phone_number_id", phoneNumberID,
		"from", msg.From,
		"type", msg.Type,
		"profile_name", profileName,
	)

	// Find the WhatsApp account by phone_number_id (use cache)
	account, err := a.getWhatsAppAccountCached(phoneNumberID)
	if err != nil {
		a.Log.Error("WhatsApp account not found", "phone_id", phoneNumberID, "error", err)
		return
	}

	// Handle reaction messages specially - they update existing messages, not create new ones
	if msg.Type == "reaction" && msg.Reaction != nil {
		a.handleIncomingReaction(account, msg.From, msg.Reaction.MessageID, msg.Reaction.Emoji, profileName)
		return
	}

	// Get or create contact (always do this for all incoming messages)
	contact, isNewContact := a.getOrCreateContact(account.OrganizationID, msg.From, profileName)

	// Dispatch webhook if new contact was created
	if isNewContact {
		a.DispatchWebhook(account.OrganizationID, EventContactCreated, ContactEventData{
			ContactID:       contact.ID.String(),
			ContactPhone:    contact.PhoneNumber,
			ContactName:     contact.ProfileName,
			WhatsAppAccount: account.Name,
		})
	}

	// Get message content - handle text, button replies, list replies, and media
	messageText := ""
	messageType := msg.Type
	buttonID := "" // Track button/list ID for conditional routing
	var mediaInfo *MediaInfo

	if msg.Type == "text" && msg.Text != nil {
		messageText = msg.Text.Body
	} else if msg.Type == "interactive" && msg.Interactive != nil {
		// Handle button reply
		if msg.Interactive.ButtonReply != nil {
			messageText = msg.Interactive.ButtonReply.Title
			buttonID = msg.Interactive.ButtonReply.ID
			messageType = "button_reply"
		}
		// Handle list reply
		if msg.Interactive.ListReply != nil {
			messageText = msg.Interactive.ListReply.Title
			buttonID = msg.Interactive.ListReply.ID
			messageType = "button_reply"
		}
	} else if msg.Type == "image" && msg.Image != nil {
		// Handle image message
		messageText = msg.Image.Caption
		mediaInfo = &MediaInfo{
			MediaMimeType: msg.Image.MimeType,
		}
		// Download and save media locally
		waAccount := &whatsapp.Account{
			PhoneID:     account.PhoneID,
			BusinessID:  account.BusinessID,
			APIVersion:  account.APIVersion,
			AccessToken: account.AccessToken,
		}
		if localPath, err := a.DownloadAndSaveMedia(context.Background(), msg.Image.ID, msg.Image.MimeType, waAccount); err != nil {
			a.Log.Error("Failed to download image", "error", err, "media_id", msg.Image.ID)
		} else {
			mediaInfo.MediaURL = localPath
		}
	} else if msg.Type == "document" && msg.Document != nil {
		// Handle document message
		messageText = msg.Document.Caption
		mediaInfo = &MediaInfo{
			MediaMimeType: msg.Document.MimeType,
			MediaFilename: msg.Document.Filename,
		}
		// Download and save media locally
		waAccount := &whatsapp.Account{
			PhoneID:     account.PhoneID,
			BusinessID:  account.BusinessID,
			APIVersion:  account.APIVersion,
			AccessToken: account.AccessToken,
		}
		if localPath, err := a.DownloadAndSaveMedia(context.Background(), msg.Document.ID, msg.Document.MimeType, waAccount); err != nil {
			a.Log.Error("Failed to download document", "error", err, "media_id", msg.Document.ID)
		} else {
			mediaInfo.MediaURL = localPath
		}
	} else if msg.Type == "video" && msg.Video != nil {
		// Handle video message
		messageText = msg.Video.Caption
		mediaInfo = &MediaInfo{
			MediaMimeType: msg.Video.MimeType,
		}
		// Download and save media locally
		waAccount := &whatsapp.Account{
			PhoneID:     account.PhoneID,
			BusinessID:  account.BusinessID,
			APIVersion:  account.APIVersion,
			AccessToken: account.AccessToken,
		}
		if localPath, err := a.DownloadAndSaveMedia(context.Background(), msg.Video.ID, msg.Video.MimeType, waAccount); err != nil {
			a.Log.Error("Failed to download video", "error", err, "media_id", msg.Video.ID)
		} else {
			mediaInfo.MediaURL = localPath
		}
	} else if msg.Type == "audio" && msg.Audio != nil {
		// Handle audio message
		mediaInfo = &MediaInfo{
			MediaMimeType: msg.Audio.MimeType,
		}
		// Download and save media locally
		waAccount := &whatsapp.Account{
			PhoneID:     account.PhoneID,
			BusinessID:  account.BusinessID,
			APIVersion:  account.APIVersion,
			AccessToken: account.AccessToken,
		}
		if localPath, err := a.DownloadAndSaveMedia(context.Background(), msg.Audio.ID, msg.Audio.MimeType, waAccount); err != nil {
			a.Log.Error("Failed to download audio", "error", err, "media_id", msg.Audio.ID)
		} else {
			mediaInfo.MediaURL = localPath
		}
	} else if msg.Type == "sticker" && msg.Sticker != nil {
		// Handle sticker message (treat like image)
		mediaInfo = &MediaInfo{
			MediaMimeType: msg.Sticker.MimeType,
		}
		// Download and save media locally
		waAccount := &whatsapp.Account{
			PhoneID:     account.PhoneID,
			BusinessID:  account.BusinessID,
			APIVersion:  account.APIVersion,
			AccessToken: account.AccessToken,
		}
		if localPath, err := a.DownloadAndSaveMedia(context.Background(), msg.Sticker.ID, msg.Sticker.MimeType, waAccount); err != nil {
			a.Log.Error("Failed to download sticker", "error", err, "media_id", msg.Sticker.ID)
		} else {
			mediaInfo.MediaURL = localPath
		}
	} else if msg.Type == "location" && msg.Location != nil {
		// Handle location message - store as JSON in content
		locationData := map[string]any{
			"latitude":  msg.Location.Latitude,
			"longitude": msg.Location.Longitude,
		}
		if msg.Location.Name != "" {
			locationData["name"] = msg.Location.Name
		}
		if msg.Location.Address != "" {
			locationData["address"] = msg.Location.Address
		}
		if jsonBytes, err := json.Marshal(locationData); err == nil {
			messageText = string(jsonBytes)
		}
	} else if msg.Type == "contacts" && len(msg.Contacts) > 0 {
		// Handle contacts message - store as JSON in content
		contactsData := make([]map[string]any, 0, len(msg.Contacts))
		for _, c := range msg.Contacts {
			contact := map[string]any{
				"name": c.Name.FormattedName,
			}
			if len(c.Phones) > 0 {
				phones := make([]string, 0, len(c.Phones))
				for _, p := range c.Phones {
					phones = append(phones, p.Phone)
				}
				contact["phones"] = phones
			}
			contactsData = append(contactsData, contact)
		}
		if jsonBytes, err := json.Marshal(contactsData); err == nil {
			messageText = string(jsonBytes)
		}
	}

	// Save incoming message to messages table (always, even if chatbot is disabled)
	var replyToWAMID string
	if msg.Context != nil && msg.Context.ID != "" {
		replyToWAMID = msg.Context.ID
	}
	a.saveIncomingMessage(account, contact, msg.ID, messageType, messageText, mediaInfo, replyToWAMID)

	// Clear chatbot tracking since client has replied
	a.ClearContactChatbotTracking(contact.ID)

	// Check for active agent transfer - skip chatbot processing if transferred
	if a.hasActiveAgentTransfer(account.OrganizationID, contact.ID) {
		a.Log.Info("Contact has active agent transfer, skipping chatbot processing",
			"contact_id", contact.ID,
			"phone_number", contact.PhoneNumber)
		return
	}

	// Check if chatbot is enabled for this account (use cache)
	settings, err := a.getChatbotSettingsCached(account.OrganizationID, account.Name)
	if err != nil {
		a.Log.Error("Failed to load chatbot settings", "error", err, "account", account.Name, "org_id", account.OrganizationID)
		return
	}
	if !settings.IsEnabled {
		a.Log.Debug("Chatbot not enabled for this account, creating transfer for agent queue", "account", account.Name, "settings_id", settings.ID)
		// Create transfer to agent queue when chatbot is disabled
		a.createTransferToQueue(account, contact, "chatbot_disabled")
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
					if err := a.sendAndSaveTextMessage(account, contact, settings.OutOfHoursMessage); err != nil {
						a.Log.Error("Failed to send out of hours message", "error", err, "contact", contact.PhoneNumber)
					}
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
					if err := a.sendAndSaveTextMessage(account, contact, settings.OutOfHoursMessage); err != nil {
						a.Log.Error("Failed to send out of hours message", "error", err, "contact", contact.PhoneNumber)
					}
				}
				return
			}
		}
		// Within business hours - send transfer message and create transfer
		if keywordResponse.Body != "" {
			if err := a.sendAndSaveTextMessage(account, contact, keywordResponse.Body); err != nil {
				a.Log.Error("Failed to send transfer message", "error", err, "contact", contact.PhoneNumber)
			}
		}
		a.createTransferFromKeyword(account, contact)
		return
	}

	// Check if user is in an active flow
	if session.CurrentFlowID != nil {
		a.processFlowResponse(account, session, contact, messageText, buttonID)
		return
	}

	// Try to match flow trigger keywords first (before greeting to avoid duplicate messages)
	if flow := a.matchFlowTrigger(account.OrganizationID, account.Name, messageText); flow != nil {
		a.startFlow(account, session, contact, flow)
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
				if err := a.sendAndSaveInteractiveButtons(account, contact, settings.DefaultResponse, greetingButtons); err != nil {
					a.Log.Error("Failed to send greeting buttons", "error", err, "contact", contact.PhoneNumber)
				}
			} else {
				if err := a.sendAndSaveTextMessage(account, contact, settings.DefaultResponse); err != nil {
					a.Log.Error("Failed to send greeting message", "error", err, "contact", contact.PhoneNumber)
				}
			}
		} else {
			if err := a.sendAndSaveTextMessage(account, contact, settings.DefaultResponse); err != nil {
				a.Log.Error("Failed to send greeting message", "error", err, "contact", contact.PhoneNumber)
			}
		}
		a.logSessionMessage(session.ID, "outgoing", settings.DefaultResponse, "greeting")
		return // After greeting, don't process further for new sessions
	}

	// Handle non-transfer keyword matches (transfer was already handled above)
	if keywordMatched && keywordResponse.ResponseType != "transfer" {
		a.Log.Info("Keyword rule matched", "response_type", keywordResponse.ResponseType, "response", keywordResponse.Body)

		// Handle regular text response
		if len(keywordResponse.Buttons) > 0 {
			if err := a.sendAndSaveInteractiveButtons(account, contact, keywordResponse.Body, keywordResponse.Buttons); err != nil {
				a.Log.Error("Failed to send interactive buttons", "error", err, "contact", contact.PhoneNumber)
			}
		} else {
			if err := a.sendAndSaveTextMessage(account, contact, keywordResponse.Body); err != nil {
				a.Log.Error("Failed to send text message", "error", err, "contact", contact.PhoneNumber)
			}
		}
		// Log outgoing message
		a.logSessionMessage(session.ID, "outgoing", keywordResponse.Body, "keyword_response")
		return
	}

	// If no keyword matched, try AI response if enabled
	if settings.AIEnabled && settings.AIProvider != "" && settings.AIAPIKey != "" {
		a.Log.Info("Attempting AI response", "provider", settings.AIProvider, "model", settings.AIModel)
		aiResponse, err := a.generateAIResponse(settings, session, messageText)
		if err != nil {
			a.Log.Error("AI response failed", "error", err, "provider", settings.AIProvider, "model", settings.AIModel)
			// Fall through to default response
		} else if aiResponse != "" {
			a.Log.Info("AI response generated successfully", "response_length", len(aiResponse))
			if err := a.sendAndSaveTextMessage(account, contact, aiResponse); err != nil {
				a.Log.Error("Failed to send AI response", "error", err, "contact", contact.PhoneNumber)
			}
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
				if err := a.sendAndSaveInteractiveButtons(account, contact, settings.FallbackMessage, fallbackButtons); err != nil {
					a.Log.Error("Failed to send fallback buttons", "error", err, "contact", contact.PhoneNumber)
				}
			} else {
				if err := a.sendAndSaveTextMessage(account, contact, settings.FallbackMessage); err != nil {
					a.Log.Error("Failed to send fallback message", "error", err, "contact", contact.PhoneNumber)
				}
			}
		} else {
			if err := a.sendAndSaveTextMessage(account, contact, settings.FallbackMessage); err != nil {
				a.Log.Error("Failed to send fallback message", "error", err, "contact", contact.PhoneNumber)
			}
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
	// Use cached keyword rules (includes both account-specific and global rules)
	rules, err := a.getKeywordRulesCached(orgID, accountName)
	if err != nil {
		a.Log.Error("Failed to fetch keyword rules", "error", err)
		return nil, false
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
// Returns the WhatsApp message ID and any error
func (a *App) sendTextMessage(account *models.WhatsAppAccount, to, message string) (string, error) {
	waAccount := &whatsapp.Account{
		PhoneID:     account.PhoneID,
		BusinessID:  account.BusinessID,
		APIVersion:  account.APIVersion,
		AccessToken: account.AccessToken,
	}
	ctx := context.Background()
	return a.WhatsApp.SendTextMessage(ctx, waAccount, to, message)
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

	// Track chatbot message for client inactivity SLA
	if err == nil {
		a.UpdateContactChatbotMessage(contact.ID)
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
	wamid, err := a.sendInteractiveButtons(account, contact.PhoneNumber, bodyText, buttons)

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
	} else if wamid != "" {
		msg.WhatsAppMessageID = wamid
	}

	if dbErr := a.DB.Create(&msg).Error; dbErr != nil {
		a.Log.Error("Failed to save chatbot interactive message", "error", dbErr)
	}

	// Track chatbot message for client inactivity SLA
	if err == nil {
		a.UpdateContactChatbotMessage(contact.ID)
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
// Returns the WhatsApp message ID and any error
func (a *App) sendInteractiveButtons(account *models.WhatsAppAccount, to, bodyText string, buttons []map[string]interface{}) (string, error) {
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
	return a.WhatsApp.SendInteractiveButtons(ctx, waAccount, to, bodyText, waButtons)
}

// sendAndSaveCTAURLButton sends a CTA URL button message and saves it to the database
func (a *App) sendAndSaveCTAURLButton(account *models.WhatsAppAccount, contact *models.Contact, bodyText, buttonText, url string) error {
	waAccount := &whatsapp.Account{
		PhoneID:     account.PhoneID,
		BusinessID:  account.BusinessID,
		APIVersion:  account.APIVersion,
		AccessToken: account.AccessToken,
	}
	ctx := context.Background()
	wamid, err := a.WhatsApp.SendCTAURLButton(ctx, waAccount, contact.PhoneNumber, bodyText, buttonText, url)

	// Create message record with interactive data
	interactiveData := models.JSONB{
		"type":        "cta_url",
		"body":        bodyText,
		"button_text": buttonText,
		"url":         url,
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
	} else if wamid != "" {
		msg.WhatsAppMessageID = wamid
	}

	if dbErr := a.DB.Create(&msg).Error; dbErr != nil {
		a.Log.Error("Failed to save chatbot CTA URL message", "error", dbErr)
	}

	// Track chatbot message for client inactivity SLA
	if err == nil {
		a.UpdateContactChatbotMessage(contact.ID)
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

// getOrCreateContact finds or creates a contact for the phone number
// Returns the contact and a boolean indicating if the contact was newly created
func (a *App) getOrCreateContact(orgID uuid.UUID, phoneNumber, profileName string) (*models.Contact, bool) {
	var contact models.Contact
	result := a.DB.Where("organization_id = ? AND phone_number = ?", orgID, phoneNumber).First(&contact)
	if result.Error == nil {
		// Update profile name if changed
		if profileName != "" && contact.ProfileName != profileName {
			a.DB.Model(&contact).Update("profile_name", profileName)
		}
		return &contact, false
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
		return &contact, false
	}
	return &contact, true
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
	// Use cached flows (includes steps)
	flows, err := a.getChatbotFlowsCached(orgID)
	if err != nil {
		a.Log.Error("Failed to fetch chatbot flows", "error", err)
		return nil
	}

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
	a.Log.Info("Starting flow", "flow_id", flow.ID, "flow_name", flow.Name, "contact", contact.PhoneNumber, "num_steps", len(flow.Steps))

	// Log all steps for debugging
	for i, step := range flow.Steps {
		a.Log.Info("Flow step", "index", i, "step_name", step.StepName, "step_order", step.StepOrder, "message_type", step.MessageType)
	}

	// Update session with flow info
	session.CurrentFlowID = &flow.ID
	session.CurrentStep = ""
	session.StepRetries = 0
	session.SessionData = models.JSONB{}
	a.DB.Save(session)

	// Send initial message if configured
	if flow.InitialMessage != "" {
		if err := a.sendAndSaveTextMessage(account, contact, flow.InitialMessage); err != nil {
			a.Log.Error("Failed to send flow initial message", "error", err, "contact", contact.PhoneNumber)
		}
		a.logSessionMessage(session.ID, "outgoing", flow.InitialMessage, "flow_start")
	}

	// Send first step message (with skip check)
	if len(flow.Steps) > 0 {
		firstStep := &flow.Steps[0]
		a.Log.Info("Sending first step", "step_name", firstStep.StepName, "message_type", firstStep.MessageType, "message", firstStep.Message)
		session.CurrentStep = firstStep.StepName
		a.DB.Model(session).Update("current_step", firstStep.StepName)

		a.sendStepWithSkipCheck(account, session, contact, firstStep, flow, nil)
	} else {
		// No steps, complete the flow
		a.completeFlow(account, session, contact, flow)
	}
}

// processFlowResponse handles user response within a flow
func (a *App) processFlowResponse(account *models.WhatsAppAccount, session *models.ChatbotSession, contact *models.Contact, userInput string, buttonID string) {
	// Load the current flow from cache
	flow, err := a.getChatbotFlowByIDCached(account.OrganizationID, *session.CurrentFlowID)
	if err != nil {
		a.Log.Error("Failed to load flow", "error", err)
		a.exitFlow(session)
		return
	}

	// Check for cancel keywords
	userInputLower := strings.ToLower(userInput)
	for _, cancelKw := range flow.CancelKeywords {
		if strings.Contains(userInputLower, strings.ToLower(cancelKw)) {
			if err := a.sendAndSaveTextMessage(account, contact, "Flow cancelled."); err != nil {
				a.Log.Error("Failed to send flow cancel message", "error", err, "contact", contact.PhoneNumber)
			}
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
				if err := a.sendAndSaveTextMessage(account, contact, errorMsg); err != nil {
					a.Log.Error("Failed to send validation error", "error", err, "contact", contact.PhoneNumber)
				}
				a.logSessionMessage(session.ID, "outgoing", errorMsg, currentStep.StepName+"_retry")
				return
			}
			// Max retries exceeded, continue anyway or exit
			a.Log.Warn("Max retries exceeded", "step", currentStep.StepName)
		}
	}

	// Auto-validate button responses when step expects button/select input
	// Only validate if InputType is button/select, or if buttons are configured and user clicked a button
	shouldValidateButtons := len(currentStep.Buttons) > 0 &&
		(currentStep.InputType == "button" || currentStep.InputType == "select" || buttonID != "")

	if shouldValidateButtons {
		isValidButton := false
		userInputLower := strings.ToLower(userInput)

		// Check if buttonID or userInput matches any configured button
		for i, btn := range currentStep.Buttons {
			if btnMap, ok := btn.(map[string]interface{}); ok {
				btnID, _ := btnMap["id"].(string)
				btnTitle, _ := btnMap["title"].(string)

				// Auto-generate ID if not set (must match what sendInteractiveButtons does)
				if btnID == "" {
					btnID = fmt.Sprintf("btn_%d", i+1)
				}

				// Match by buttonID (exact match) or by title (case-insensitive)
				if buttonID != "" && buttonID == btnID {
					isValidButton = true
					break
				}
				if strings.ToLower(btnTitle) == userInputLower || btnID == userInput {
					isValidButton = true
					// Set buttonID if not already set (user typed the button text)
					if buttonID == "" {
						buttonID = btnID
					}
					break
				}
			}
		}

		if !isValidButton {
			// Invalid button selection
			session.StepRetries++
			a.Log.Debug("Invalid button selection", "buttonID", buttonID, "userInput", userInput, "step", currentStep.StepName, "retries", session.StepRetries)
			a.DB.Model(session).Update("step_retries", session.StepRetries)

			maxRetries := currentStep.MaxRetries
			if maxRetries == 0 {
				maxRetries = 3 // Default max retries
			}

			if session.StepRetries >= maxRetries {
				// Max retries exceeded - exit flow and close conversation
				a.Log.Warn("Max button retries exceeded, closing conversation", "step", currentStep.StepName)
				if err := a.sendAndSaveTextMessage(account, contact, "Sorry, we couldn't continue. Please try again later."); err != nil {
					a.Log.Error("Failed to send max retries message", "error", err, "contact", contact.PhoneNumber)
				}
				a.exitFlow(session)
				a.closeSession(session)
				return
			}

			// Resend the step message with buttons
			a.sendStepMessage(account, session, contact, currentStep)
			return
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
		a.completeFlow(account, session, contact, flow)
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
		a.completeFlow(account, session, contact, flow)
		return
	}

	// Update session and send next step message (with skip check)
	a.DB.Model(session).Updates(map[string]interface{}{
		"current_step": nextStep.StepName,
		"step_retries": 0,
	})

	a.Log.Info("Moving to next step", "nextStep", nextStep.StepName, "skipCondition", nextStep.SkipCondition, "sessionData", session.SessionData)
	a.sendStepWithSkipCheck(account, session, contact, nextStep, flow, nil)
}

// completeFlow finishes a flow and sends completion message
func (a *App) completeFlow(account *models.WhatsAppAccount, session *models.ChatbotSession, contact *models.Contact, flow *models.ChatbotFlow) {
	a.Log.Info("Completing flow", "flow_id", flow.ID, "session_id", session.ID)

	// Send completion message
	if flow.CompletionMessage != "" {
		message := a.replaceVariables(flow.CompletionMessage, session.SessionData)
		if err := a.sendAndSaveTextMessage(account, contact, message); err != nil {
			a.Log.Error("Failed to send flow completion message", "error", err, "contact", contact.PhoneNumber)
		}
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

	// Clear chatbot tracking so SLA doesn't fire after flow completion
	a.ClearContactChatbotTracking(contact.ID)
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
	defer func() { _ = resp.Body.Close() }()

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

// closeSession ends the chatbot session and clears contact tracking
func (a *App) closeSession(session *models.ChatbotSession) {
	a.DB.Model(session).Updates(map[string]interface{}{
		"status":       "completed",
		"completed_at": time.Now(),
	})

	// Clear chatbot tracking on contact
	a.ClearContactChatbotTracking(session.ContactID)
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

// sendStepWithSkipCheck checks if a step should be skipped and sends the appropriate step message
// It takes the full flow to find next steps when skipping
func (a *App) sendStepWithSkipCheck(account *models.WhatsAppAccount, session *models.ChatbotSession, contact *models.Contact, step *models.ChatbotFlowStep, flow *models.ChatbotFlow, skippedSteps map[string]bool) {
	// Prevent infinite loops
	if skippedSteps == nil {
		skippedSteps = make(map[string]bool)
	}
	if skippedSteps[step.StepName] {
		a.Log.Warn("Skip loop detected, completing flow", "step", step.StepName)
		a.completeFlow(account, session, contact, flow)
		return
	}

	// Check if step should be skipped
	sessionData := session.SessionData
	if sessionData == nil {
		sessionData = models.JSONB{}
	}

	if a.shouldSkipStep(step, sessionData) {
		a.Log.Info("Skipping step", "step", step.StepName, "condition", step.SkipCondition)
		skippedSteps[step.StepName] = true

		// Find next step
		nextStepName := step.NextStep
		if nextStepName == "" {
			// Find by step order
			for i, s := range flow.Steps {
				if s.StepName == step.StepName && i+1 < len(flow.Steps) {
					nextStepName = flow.Steps[i+1].StepName
					break
				}
			}
		}

		if nextStepName == "" {
			// No next step, complete flow
			a.completeFlow(account, session, contact, flow)
			return
		}

		// Find and execute next step
		var nextStep *models.ChatbotFlowStep
		for i := range flow.Steps {
			if flow.Steps[i].StepName == nextStepName {
				nextStep = &flow.Steps[i]
				break
			}
		}

		if nextStep == nil {
			a.Log.Warn("Next step not found after skip, completing flow", "next_step", nextStepName)
			a.completeFlow(account, session, contact, flow)
			return
		}

		// Update session to next step
		session.CurrentStep = nextStep.StepName
		a.DB.Model(session).Update("current_step", nextStep.StepName)

		// Recursively check next step (it may also need to be skipped)
		a.sendStepWithSkipCheck(account, session, contact, nextStep, flow, skippedSteps)
		return
	}

	// Not skipping - send the step message normally
	a.sendStepMessage(account, session, contact, step)

	// If input type is "none", automatically advance to next step without waiting for user input
	if step.InputType == "none" {

		// Find next step
		nextStepName := step.NextStep
		if nextStepName == "" {
			// Find by step order
			for i, s := range flow.Steps {
				if s.StepName == step.StepName && i+1 < len(flow.Steps) {
					nextStepName = flow.Steps[i+1].StepName
					break
				}
			}
		}

		if nextStepName == "" {
			// No next step, complete flow
			a.completeFlow(account, session, contact, flow)
			return
		}

		// Find and execute next step
		var nextStep *models.ChatbotFlowStep
		for i := range flow.Steps {
			if flow.Steps[i].StepName == nextStepName {
				nextStep = &flow.Steps[i]
				break
			}
		}

		if nextStep == nil {
			a.Log.Warn("Next step not found after no-input step, completing flow", "next_step", nextStepName)
			a.completeFlow(account, session, contact, flow)
			return
		}

		// Update session to next step
		session.CurrentStep = nextStep.StepName
		a.DB.Model(session).Update("current_step", nextStep.StepName)

		// Recursively process next step (it may also need to skip or have no input)
		a.sendStepWithSkipCheck(account, session, contact, nextStep, flow, skippedSteps)
	}
}

// sendStepMessage sends the appropriate message based on step message_type
func (a *App) sendStepMessage(account *models.WhatsAppAccount, session *models.ChatbotSession, contact *models.Contact, step *models.ChatbotFlowStep) {
	var message string

	switch step.MessageType {
	case "api_fetch":
		// Fetch response from external API (may include message + buttons)
		// Pass the step message as template - it will be processed with API response data
		apiResp, err := a.fetchApiResponse(step.ApiConfig, session.SessionData, step.Message)
		if err != nil {
			a.Log.Error("Failed to fetch API response", "error", err, "step", step.StepName)
			// Use fallback message if configured, otherwise use the step message
			if fallback, ok := step.ApiConfig["fallback_message"].(string); ok && fallback != "" {
				message = processTemplate(fallback, session.SessionData)
			} else if step.Message != "" {
				message = processTemplate(step.Message, session.SessionData)
			} else {
				message = "Sorry, there was an error processing your request."
			}
			if err := a.sendAndSaveTextMessage(account, contact, message); err != nil {
				a.Log.Error("Failed to send API error message", "error", err, "contact", contact.PhoneNumber)
			}
		} else {
			message = apiResp.Message

			// Save mapped data to session for future steps
			if apiResp.MappedData != nil {
				for k, v := range apiResp.MappedData {
					session.SessionData[k] = v
				}
				a.DB.Model(session).Update("session_data", session.SessionData)
			}

			// Check if API returned buttons
			if len(apiResp.Buttons) > 0 {
				if err := a.sendAndSaveInteractiveButtons(account, contact, message, apiResp.Buttons); err != nil {
					a.Log.Error("Failed to send API response buttons", "error", err, "contact", contact.PhoneNumber)
				}
			} else {
				if err := a.sendAndSaveTextMessage(account, contact, message); err != nil {
					a.Log.Error("Failed to send API response message", "error", err, "contact", contact.PhoneNumber)
				}
			}
		}
		a.logSessionMessage(session.ID, "outgoing", message, step.StepName)

	case "buttons":
		// Send interactive buttons message
		message = processTemplate(step.Message, session.SessionData)
		if len(step.Buttons) > 0 {
			// Separate reply buttons from URL buttons
			// WhatsApp doesn't allow mixing them in the same message
			replyButtons := make([]map[string]interface{}, 0)
			urlButtons := make([]map[string]interface{}, 0)

			for _, btn := range step.Buttons {
				if btnMap, ok := btn.(map[string]interface{}); ok {
					btnType, _ := btnMap["type"].(string)
					if btnType == "url" {
						urlButtons = append(urlButtons, btnMap)
					} else {
						// Default to reply button
						replyButtons = append(replyButtons, btnMap)
					}
				}
			}

			// Send reply buttons first (with the main message)
			if len(replyButtons) > 0 {
				if err := a.sendAndSaveInteractiveButtons(account, contact, message, replyButtons); err != nil {
					a.Log.Error("Failed to send reply buttons", "error", err, "contact", contact.PhoneNumber)
				}
			} else if len(urlButtons) == 0 {
				// No buttons at all, fall back to text
				if err := a.sendAndSaveTextMessage(account, contact, message); err != nil {
					a.Log.Error("Failed to send text message", "error", err, "contact", contact.PhoneNumber)
				}
			}

			// Send URL buttons as separate CTA URL messages
			// WhatsApp only allows one CTA URL button per message
			for _, urlBtn := range urlButtons {
				btnTitle, _ := urlBtn["title"].(string)
				btnURL, _ := urlBtn["url"].(string)
				if btnTitle != "" && btnURL != "" {
					// Use empty body for subsequent URL buttons, or use message if no reply buttons
					bodyText := ""
					if len(replyButtons) == 0 {
						bodyText = message
						message = "" // Clear so we don't repeat it
					}
					if err := a.sendAndSaveCTAURLButton(account, contact, bodyText, btnTitle, btnURL); err != nil {
						a.Log.Error("Failed to send CTA URL button", "error", err, "contact", contact.PhoneNumber)
					}
				}
			}
		} else {
			// No buttons configured, fall back to text
			if err := a.sendAndSaveTextMessage(account, contact, message); err != nil {
				a.Log.Error("Failed to send step message", "error", err, "contact", contact.PhoneNumber)
			}
		}
		a.logSessionMessage(session.ID, "outgoing", message, step.StepName)

	case "transfer":
		// Transfer to team/agent queue
		message = processTemplate(step.Message, session.SessionData)
		if message != "" {
			if err := a.sendAndSaveTextMessage(account, contact, message); err != nil {
				a.Log.Error("Failed to send transfer message", "error", err, "contact", contact.PhoneNumber)
			}
			a.logSessionMessage(session.ID, "outgoing", message, step.StepName)
		}

		// Get transfer configuration
		var teamID *uuid.UUID
		var notes string
		if step.TransferConfig != nil {
			if teamIDStr, ok := step.TransferConfig["team_id"].(string); ok && teamIDStr != "" && teamIDStr != "_general" {
				if parsedID, err := uuid.Parse(teamIDStr); err == nil {
					teamID = &parsedID
				}
			}
			if n, ok := step.TransferConfig["notes"].(string); ok {
				notes = processTemplate(n, session.SessionData)
			}
		}

		// Create the transfer
		if teamID != nil {
			a.createTransferToTeam(account, contact, *teamID, notes, "flow")
		} else {
			// General queue transfer
			a.createTransferToQueue(account, contact, "flow")
		}

		// End the flow session (transfer takes over)
		a.exitFlow(session)
		return

	default:
		// Default: use the step message with template processing
		message = processTemplate(step.Message, session.SessionData)
		if err := a.sendAndSaveTextMessage(account, contact, message); err != nil {
			a.Log.Error("Failed to send step message", "error", err, "contact", contact.PhoneNumber)
		}
		a.logSessionMessage(session.ID, "outgoing", message, step.StepName)
	}
}

// ApiResponse represents a response from an external API that may include buttons
type ApiResponse struct {
	Message      string
	Buttons      []map[string]interface{}
	MappedData   map[string]interface{} // Data extracted via response_mapping
	ResponseData map[string]interface{} // Full API response data
}

// fetchApiResponse fetches a response from an external API, supporting message + buttons
// and response_mapping for storing API data in session variables
func (a *App) fetchApiResponse(apiConfig models.JSONB, sessionData models.JSONB, messageTemplate string) (*ApiResponse, error) {
	if apiConfig == nil {
		return nil, fmt.Errorf("API config is empty")
	}

	// Get API URL (required)
	apiURL, ok := apiConfig["url"].(string)
	if !ok || apiURL == "" {
		return nil, fmt.Errorf("API URL is required")
	}

	// Replace variables in URL using template engine
	apiURL = processTemplate(apiURL, sessionData)

	// Get HTTP method (default: GET)
	method := "GET"
	if m, ok := apiConfig["method"].(string); ok && m != "" {
		method = strings.ToUpper(m)
	}

	// Prepare request body if configured
	var bodyReader io.Reader
	if bodyTemplate, ok := apiConfig["body"].(string); ok && bodyTemplate != "" {
		bodyWithVars := processTemplate(bodyTemplate, sessionData)
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
				req.Header.Set(key, processTemplate(strVal, sessionData))
			}
		}
	}

	// Make the request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Read response body (limit to 1MB to prevent memory issues)
	limitReader := io.LimitReader(resp.Body, 1024*1024)
	respBody, err := io.ReadAll(limitReader)
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

	result := &ApiResponse{
		ResponseData: jsonResp,
	}

	// Process response_mapping if configured
	// This maps API response fields to session variables for use in templates
	if responseMapping, ok := apiConfig["response_mapping"].(map[string]interface{}); ok {
		mappingStrings := make(map[string]string)
		for varName, path := range responseMapping {
			if pathStr, ok := path.(string); ok {
				mappingStrings[varName] = pathStr
			}
		}
		result.MappedData = extractResponseMapping(jsonResp, mappingStrings)

		// Merge mapped data into sessionData for template processing
		for k, v := range result.MappedData {
			sessionData[k] = v
		}
	}

	// Process the message template with all available data (including mapped data)
	if messageTemplate != "" {
		result.Message = processTemplate(messageTemplate, sessionData)
	} else if msg, ok := jsonResp["message"].(string); ok {
		// Fallback: check for "message" field in response
		result.Message = msg
	} else {
		// No message template, return raw response
		result.Message = string(respBody)
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
	// Get WhatsApp account for cache key
	whatsAppAccount := ""
	if session != nil {
		whatsAppAccount = session.WhatsAppAccount
	}

	// Use cached AI contexts
	contexts, err := a.getAIContextsCached(orgID, whatsAppAccount)
	if err != nil || len(contexts) == 0 {
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
	defer func() { _ = resp.Body.Close() }()

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
			if value := getNestedValue(jsonResp, responsePath); value != nil {
				return formatValue(value), nil
			}
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
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		var errResp struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.Unmarshal(body, &errResp)
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
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		var errResp struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.Unmarshal(body, &errResp)
		return "", fmt.Errorf("anthropic API error: %s", errResp.Error.Message)
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
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		var errResp struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.Unmarshal(body, &errResp)
		return "", fmt.Errorf("google AI API error: %s", errResp.Error.Message)
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

// Reaction represents a reaction on a message
type Reaction struct {
	Emoji     string `json:"emoji"`
	FromPhone string `json:"from_phone,omitempty"` // Phone number if from contact
	FromUser  string `json:"from_user,omitempty"`  // User ID if from agent
}

// handleIncomingReaction handles incoming reaction messages from WhatsApp
func (a *App) handleIncomingReaction(account *models.WhatsAppAccount, fromPhone, messageWAMID, emoji, profileName string) {
	a.Log.Info("Handling incoming reaction",
		"from", fromPhone,
		"message_wamid", messageWAMID,
		"emoji", emoji,
	)

	// Find the message being reacted to
	// WhatsApp encodes phone numbers in the WAMID prefix, so the same message
	// has different WAMIDs from sender vs recipient perspective.
	// We match on the suffix after "FQIA" + 4 chars (type indicator like "ERgS" or "EhgU")
	var message models.Message
	if err := a.DB.Where("whats_app_message_id = ?", messageWAMID).First(&message).Error; err != nil {
		// Try matching on WAMID suffix (the unique message ID part)
		if idx := strings.Index(messageWAMID, "FQIA"); idx != -1 {
			// Extract suffix after "FQIA" + 4 char type indicator (e.g., "ERgS", "EhgU")
			suffixStart := idx + 8
			if suffixStart < len(messageWAMID) {
				suffix := messageWAMID[suffixStart:]
				if err := a.DB.Where("whats_app_message_id LIKE ?", "%"+suffix).First(&message).Error; err != nil {
					a.Log.Warn("Message not found for reaction", "wamid", messageWAMID, "suffix", suffix)
					return
				}
			} else {
				a.Log.Warn("Message not found for reaction - invalid WAMID format", "wamid", messageWAMID)
				return
			}
		} else {
			a.Log.Warn("Message not found for reaction - no FQIA pattern", "wamid", messageWAMID)
			return
		}
	}

	// Get or create contact
	contact, _ := a.getOrCreateContact(account.OrganizationID, fromPhone, profileName)

	// Parse existing reactions from Metadata
	var metadata map[string]interface{}
	if message.Metadata != nil {
		metadata = message.Metadata
	} else {
		metadata = make(map[string]interface{})
	}

	// Get or initialize reactions array
	var reactions []Reaction
	if reactionsRaw, ok := metadata["reactions"]; ok {
		if reactionsArray, ok := reactionsRaw.([]interface{}); ok {
			for _, r := range reactionsArray {
				if rMap, ok := r.(map[string]interface{}); ok {
					emoji, _ := rMap["emoji"].(string)
					reactions = append(reactions, Reaction{
						Emoji:     emoji,
						FromPhone: getStringFromMap(rMap, "from_phone"),
						FromUser:  getStringFromMap(rMap, "from_user"),
					})
				}
			}
		}
	}

	// Remove existing reaction from this contact (each contact can only have one reaction)
	var newReactions []Reaction
	for _, r := range reactions {
		if r.FromPhone != fromPhone {
			newReactions = append(newReactions, r)
		}
	}

	// Add new reaction if emoji is not empty (empty = remove reaction)
	if emoji != "" {
		newReactions = append(newReactions, Reaction{
			Emoji:     emoji,
			FromPhone: fromPhone,
		})
	}

	// Update metadata
	metadata["reactions"] = newReactions

	// Save to database
	if err := a.DB.Model(&message).Update("metadata", metadata).Error; err != nil {
		a.Log.Error("Failed to update message reactions", "error", err)
		return
	}

	a.Log.Info("Updated message reaction", "message_id", message.ID, "reactions_count", len(newReactions))

	// Broadcast via WebSocket
	if a.WSHub != nil {
		a.WSHub.BroadcastToOrg(account.OrganizationID, websocket.WSMessage{
			Type: "reaction_update",
			Payload: map[string]any{
				"message_id": message.ID.String(),
				"contact_id": contact.ID.String(),
				"reactions":  newReactions,
			},
		})
	}
}

// Helper function to safely get string from map
func getStringFromMap(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// MediaInfo holds media-related information for an incoming message
type MediaInfo struct {
	MediaURL      string
	MediaMimeType string
	MediaFilename string
}

// saveIncomingMessage saves an incoming message to the messages table
func (a *App) saveIncomingMessage(account *models.WhatsAppAccount, contact *models.Contact, whatsappMsgID, msgType, content string, mediaInfo *MediaInfo, replyToWAMID string) {
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

	// Handle reply context - look up the original message by WhatsApp message ID
	if replyToWAMID != "" {
		var replyToMsg models.Message
		if err := a.DB.Where("whats_app_message_id = ?", replyToWAMID).First(&replyToMsg).Error; err == nil {
			message.IsReply = true
			message.ReplyToMessageID = &replyToMsg.ID
		} else {
			a.Log.Warn("Reply-to message not found", "reply_to_wamid", replyToWAMID)
		}
	}

	// Add media fields if present
	if mediaInfo != nil {
		message.MediaURL = mediaInfo.MediaURL
		message.MediaMimeType = mediaInfo.MediaMimeType
		message.MediaFilename = mediaInfo.MediaFilename
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

	a.Log.Info("Saved incoming message", "message_id", message.ID, "contact_id", contact.ID, "media_url", message.MediaURL)

	// Broadcast new message via WebSocket
	if a.WSHub != nil {
		var assignedUserIDStr string
		if contact.AssignedUserID != nil {
			assignedUserIDStr = contact.AssignedUserID.String()
		}
		wsPayload := map[string]any{
			"id":               message.ID.String(),
			"contact_id":       contact.ID.String(),
			"assigned_user_id": assignedUserIDStr,
			"profile_name":     contact.ProfileName,
			"direction":        message.Direction,
			"message_type":     message.MessageType,
			"content":          map[string]string{"body": message.Content},
			"media_url":        message.MediaURL,
			"media_mime_type":  message.MediaMimeType,
			"media_filename":   message.MediaFilename,
			"status":           message.Status,
			"wamid":            message.WhatsAppMessageID,
			"created_at":       message.CreatedAt,
			"updated_at":       message.UpdatedAt,
			"is_reply":         message.IsReply,
		}
		// Include reply context if this is a reply
		if message.IsReply && message.ReplyToMessageID != nil {
			wsPayload["reply_to_message_id"] = message.ReplyToMessageID.String()
			// Load the replied-to message for preview
			var replyToMsg models.Message
			if err := a.DB.First(&replyToMsg, message.ReplyToMessageID).Error; err == nil {
				wsPayload["reply_to_message"] = map[string]any{
					"id":           replyToMsg.ID.String(),
					"content":      map[string]string{"body": replyToMsg.Content},
					"message_type": replyToMsg.MessageType,
					"direction":    replyToMsg.Direction,
				}
			}
		}
		a.WSHub.BroadcastToOrg(account.OrganizationID, websocket.WSMessage{
			Type:    websocket.TypeNewMessage,
			Payload: wsPayload,
		})
	}

	// Dispatch webhook for incoming message
	a.DispatchWebhook(account.OrganizationID, EventMessageIncoming, MessageEventData{
		MessageID:       message.ID.String(),
		ContactID:       contact.ID.String(),
		ContactPhone:    contact.PhoneNumber,
		ContactName:     contact.ProfileName,
		MessageType:     msgType,
		Content:         content,
		WhatsAppAccount: account.Name,
		Direction:       "incoming",
	})
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

// shouldSkipStep evaluates a text expression like "(status == 'vip' OR amount > 100) AND name != ''"
func (a *App) shouldSkipStep(step *models.ChatbotFlowStep, sessionData map[string]interface{}) bool {
	if step.SkipCondition == "" {
		a.Log.Debug("No skip condition for step", "step", step.StepName)
		return false
	}
	a.Log.Info("Evaluating skip condition", "step", step.StepName, "condition", step.SkipCondition, "sessionData", sessionData)
	result := evaluateExpression(step.SkipCondition, sessionData)
	a.Log.Info("Skip condition result", "step", step.StepName, "result", result)
	return result
}

// evaluateExpression handles parentheses, AND, OR, and single conditions
func evaluateExpression(expr string, data map[string]interface{}) bool {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return false
	}

	// Handle parentheses first - find innermost pairs and evaluate recursively
	for strings.Contains(expr, "(") {
		start := strings.LastIndex(expr, "(")
		end := strings.Index(expr[start:], ")")
		if end == -1 {
			break // Malformed expression
		}
		end = start + end
		inner := expr[start+1 : end]
		result := evaluateExpression(inner, data)
		// Replace (inner) with result
		replacement := "false"
		if result {
			replacement = "true"
		}
		expr = expr[:start] + replacement + expr[end+1:]
		expr = strings.TrimSpace(expr)
	}

	// Handle boolean literals (from parentheses evaluation)
	if expr == "true" {
		return true
	}
	if expr == "false" {
		return false
	}

	// Split by OR (lower precedence) - case insensitive
	orParts := splitByLogicOperator(expr, " OR ")
	if len(orParts) > 1 {
		for _, part := range orParts {
			if evaluateExpression(part, data) {
				return true
			}
		}
		return false
	}

	// Split by AND (higher precedence) - case insensitive
	andParts := splitByLogicOperator(expr, " AND ")
	if len(andParts) > 1 {
		for _, part := range andParts {
			if !evaluateExpression(part, data) {
				return false
			}
		}
		return true
	}

	// Single condition
	return evaluateSingleCondition(expr, data)
}

// splitByLogicOperator splits by AND/OR while preserving case-insensitivity
func splitByLogicOperator(expr, op string) []string {
	upperExpr := strings.ToUpper(expr)
	upperOp := strings.ToUpper(op)

	var parts []string
	lastIdx := 0
	for {
		idx := strings.Index(upperExpr[lastIdx:], upperOp)
		if idx == -1 {
			parts = append(parts, strings.TrimSpace(expr[lastIdx:]))
			break
		}
		parts = append(parts, strings.TrimSpace(expr[lastIdx:lastIdx+idx]))
		lastIdx = lastIdx + idx + len(op)
	}
	return parts
}

// evaluateSingleCondition handles: phone != '' or age > 18 or status == 'confirmed'
func evaluateSingleCondition(expr string, data map[string]interface{}) bool {
	expr = strings.TrimSpace(expr)

	// Handle boolean literals
	if expr == "true" {
		return true
	}
	if expr == "false" {
		return false
	}

	operators := []string{"!=", "==", ">=", "<=", ">", "<"}

	for _, op := range operators {
		if strings.Contains(expr, op) {
			parts := strings.SplitN(expr, op, 2)
			if len(parts) == 2 {
				varName := strings.TrimSpace(parts[0])
				expectedValue := strings.TrimSpace(parts[1])
				expectedValue = strings.Trim(expectedValue, "'\"")

				actualValue := ""
				if val, exists := data[varName]; exists && val != nil {
					actualValue = fmt.Sprintf("%v", val)
				}

				return compareValues(actualValue, op, expectedValue)
			}
		}
	}
	return false
}

// compareValues compares two values using the specified operator
func compareValues(actual, operator, expected string) bool {
	switch operator {
	case "==":
		return actual == expected
	case "!=":
		return actual != expected
	case ">", "<", ">=", "<=":
		// Try numeric comparison first
		actualNum, err1 := parseNumber(actual)
		expectedNum, err2 := parseNumber(expected)
		if err1 == nil && err2 == nil {
			switch operator {
			case ">":
				return actualNum > expectedNum
			case "<":
				return actualNum < expectedNum
			case ">=":
				return actualNum >= expectedNum
			case "<=":
				return actualNum <= expectedNum
			}
		}
		// Fall back to string comparison
		switch operator {
		case ">":
			return actual > expected
		case "<":
			return actual < expected
		case ">=":
			return actual >= expected
		case "<=":
			return actual <= expected
		}
	}
	return false
}

// parseNumber attempts to parse a string as a float64
func parseNumber(s string) (float64, error) {
	var n float64
	_, err := fmt.Sscanf(s, "%f", &n)
	return n, err
}
