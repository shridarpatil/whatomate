package handlers

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/internal/websocket"
)

// SLAProcessor handles periodic SLA checks and escalations
type SLAProcessor struct {
	app      *App
	interval time.Duration
	stopCh   chan struct{}
}

// NewSLAProcessor creates a new SLA processor
func NewSLAProcessor(app *App, interval time.Duration) *SLAProcessor {
	return &SLAProcessor{
		app:      app,
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

// Start begins the SLA processing loop
func (p *SLAProcessor) Start(ctx context.Context) {
	p.app.Log.Info("SLA processor started", "interval", p.interval)

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.app.Log.Info("SLA processor stopped by context")
			return
		case <-p.stopCh:
			p.app.Log.Info("SLA processor stopped")
			return
		case <-ticker.C:
			p.processStaleTransfers()
		}
	}
}

// Stop stops the SLA processor
func (p *SLAProcessor) Stop() {
	close(p.stopCh)
}

// processStaleTransfers checks for transfers that need escalation or auto-close
func (p *SLAProcessor) processStaleTransfers() {
	now := time.Now()

	// Get all organizations with SLA enabled (use cache)
	settings, err := p.app.getSLAEnabledSettingsCached()
	if err != nil {
		p.app.Log.Error("Failed to load SLA settings", "error", err)
		return
	}

	for _, s := range settings {
		p.processOrganizationSLA(s, now)
	}
}

// processOrganizationSLA processes SLA for a single organization
func (p *SLAProcessor) processOrganizationSLA(settings models.ChatbotSettings, now time.Time) {
	orgID := settings.OrganizationID

	// 1. Auto-close expired transfers
	if settings.SLA.AutoCloseHours > 0 {
		p.autoCloseExpiredTransfers(orgID, settings, now)
	}

	// 2. Escalate transfers past escalation deadline
	if settings.SLA.EscalationMinutes > 0 {
		p.escalateTransfers(orgID, settings, now)
	}

	// 3. Mark SLA breached for transfers past response deadline
	if settings.SLA.ResponseMinutes > 0 {
		p.markSLABreached(orgID, settings, now)
	}

	// 4. Handle client inactivity (reminders and auto-close)
	if settings.ClientInactivity.ReminderEnabled {
		p.processClientInactivity(orgID, settings, now)
	}
}

// autoCloseExpiredTransfers closes transfers that have exceeded their expiry time
func (p *SLAProcessor) autoCloseExpiredTransfers(orgID uuid.UUID, settings models.ChatbotSettings, now time.Time) {
	var transfers []models.AgentTransfer
	if err := p.app.DB.Where(
		"organization_id = ? AND status = ? AND expires_at IS NOT NULL AND expires_at < ?",
		orgID, models.TransferStatusActive, now,
	).Find(&transfers).Error; err != nil {
		p.app.Log.Error("Failed to find expired transfers", "error", err, "org_id", orgID)
		return
	}

	closedCount := 0
	for _, transfer := range transfers {
		// Check if the assigned agent has been actively responding.
		// If so, extend the expiry deadline instead of auto-closing.
		deadline := transfer.SLA.ExpiresAt
		if deadline != nil && p.agentRespondedSince(transfer, deadline.Add(-time.Duration(settings.SLA.AutoCloseHours)*time.Hour)) {
			newExpiry := now.Add(time.Duration(settings.SLA.AutoCloseHours) * time.Hour)
			if err := p.app.DB.Model(&transfer).Update("expires_at", newExpiry).Error; err != nil {
				p.app.Log.Error("Failed to extend transfer expiry", "error", err, "transfer_id", transfer.ID)
			} else {
				p.app.Log.Info("Extended transfer expiry due to agent activity",
					"transfer_id", transfer.ID,
					"new_expires_at", newExpiry,
				)
			}
			// Also record first_response_at if not yet set
			p.app.UpdateSLAOnFirstResponse(&transfer)
			if transfer.SLA.FirstResponseAt != nil {
				p.app.DB.Model(&transfer).Update("first_response_at", transfer.SLA.FirstResponseAt)
			}
			continue
		}

		// Send auto-close message to customer if configured
		if settings.SLA.AutoCloseMessage != "" {
			p.sendSLAAutoCloseToCustomer(transfer, settings.SLA.AutoCloseMessage)
		}

		// Update transfer status
		if err := p.app.DB.Model(&transfer).Updates(map[string]interface{}{
			"status":     models.TransferStatusExpired,
			"resumed_at": now,
			"notes":      transfer.Notes + "\n[Auto-closed: No agent response within SLA]",
		}).Error; err != nil {
			p.app.Log.Error("Failed to expire transfer", "error", err, "transfer_id", transfer.ID)
			continue
		}

		closedCount++
		p.app.Log.Info("Transfer auto-closed due to expiry",
			"transfer_id", transfer.ID,
			"contact_id", transfer.ContactID,
			"expires_at", transfer.SLA.ExpiresAt,
		)

		// Broadcast update
		p.broadcastTransferUpdate(transfer, string(models.TransferStatusExpired))
	}

	if closedCount > 0 {
		p.app.Log.Info("Auto-closed expired transfers", "count", closedCount, "org_id", orgID)
	}
}

// escalateTransfers escalates transfers past their escalation deadline
func (p *SLAProcessor) escalateTransfers(orgID uuid.UUID, settings models.ChatbotSettings, now time.Time) {
	var transfers []models.AgentTransfer
	if err := p.app.DB.Where(
		"organization_id = ? AND status = ? AND sla_escalation_at IS NOT NULL AND sla_escalation_at < ? AND escalation_level < 2",
		orgID, models.TransferStatusActive, now,
	).Find(&transfers).Error; err != nil {
		p.app.Log.Error("Failed to find transfers for escalation", "error", err, "org_id", orgID)
		return
	}

	escalatedCount := 0
	for _, transfer := range transfers {
		// Check if the assigned agent has been actively responding.
		// If so, push the escalation deadline forward instead of escalating.
		escalationAt := transfer.SLA.EscalationAt
		if escalationAt != nil && p.agentRespondedSince(transfer, escalationAt.Add(-time.Duration(settings.SLA.EscalationMinutes)*time.Minute)) {
			newEscalation := now.Add(time.Duration(settings.SLA.EscalationMinutes) * time.Minute)
			if err := p.app.DB.Model(&transfer).Update("sla_escalation_at", newEscalation).Error; err != nil {
				p.app.Log.Error("Failed to extend transfer escalation", "error", err, "transfer_id", transfer.ID)
			} else {
				p.app.Log.Info("Extended transfer escalation due to agent activity",
					"transfer_id", transfer.ID,
					"new_escalation_at", newEscalation,
				)
			}
			// Also record first_response_at if not yet set
			p.app.UpdateSLAOnFirstResponse(&transfer)
			if transfer.SLA.FirstResponseAt != nil {
				p.app.DB.Model(&transfer).Update("first_response_at", transfer.SLA.FirstResponseAt)
			}
			continue
		}

		newLevel := transfer.SLA.EscalationLevel + 1

		// Update transfer
		updates := map[string]interface{}{
			"escalation_level": newLevel,
			"escalated_at":     now,
		}

		// If not yet breached and past response deadline, mark as breached
		if !transfer.SLA.Breached && transfer.SLA.ResponseDeadline != nil && now.After(*transfer.SLA.ResponseDeadline) {
			updates["sla_breached"] = true
			updates["sla_breached_at"] = now
		}

		if err := p.app.DB.Model(&transfer).Updates(updates).Error; err != nil {
			p.app.Log.Error("Failed to escalate transfer", "error", err, "transfer_id", transfer.ID)
			continue
		}

		escalatedCount++
		p.app.Log.Warn("Transfer escalated",
			"transfer_id", transfer.ID,
			"contact_id", transfer.ContactID,
			"new_level", newLevel,
			"escalation_at", transfer.SLA.EscalationAt,
		)

		// Send notification to escalation contacts
		p.notifyEscalation(transfer, settings, newLevel)

		// Broadcast update
		p.broadcastTransferUpdate(transfer, "escalated")

		// Send warning message to customer if configured
		if newLevel == 1 && settings.SLA.WarningMessage != "" {
			p.sendSLAWarningToCustomer(transfer, settings.SLA.WarningMessage)
		}
	}

	if escalatedCount > 0 {
		p.app.Log.Info("Escalated transfers", "count", escalatedCount, "org_id", orgID)
	}
}

// markSLABreached marks transfers as SLA breached when past response deadline
func (p *SLAProcessor) markSLABreached(orgID uuid.UUID, settings models.ChatbotSettings, now time.Time) {
	result := p.app.DB.Model(&models.AgentTransfer{}).Where(
		"organization_id = ? AND status = ? AND sla_breached = ? AND sla_response_deadline IS NOT NULL AND sla_response_deadline < ? AND agent_id IS NULL",
		orgID, models.TransferStatusActive, false, now,
	).Updates(map[string]interface{}{
		"sla_breached":    true,
		"sla_breached_at": now,
	})

	if result.Error != nil {
		p.app.Log.Error("Failed to mark SLA breached", "error", result.Error, "org_id", orgID)
		return
	}

	if result.RowsAffected > 0 {
		p.app.Log.Warn("Marked transfers as SLA breached", "count", result.RowsAffected, "org_id", orgID)
	}
}

// notifyEscalation sends notifications to escalation contacts via WebSocket broadcast
func (p *SLAProcessor) notifyEscalation(transfer models.AgentTransfer, settings models.ChatbotSettings, level int) {
	if len(settings.SLA.EscalationNotifyIDs) == 0 {
		return
	}

	// Get contact info for the notification
	var contact models.Contact
	if err := p.app.DB.Where("id = ?", transfer.ContactID).First(&contact).Error; err != nil {
		p.app.Log.Error("Failed to load contact for escalation notification", "error", err)
		return
	}

	// Prepare notification payload
	levelName := "warning"
	if level >= 2 {
		levelName = "critical"
	}

	// Broadcast escalation notification to the organization
	// Escalation contacts will receive this via the org-wide broadcast
	contactName := contact.ProfileName
	phoneNumber := contact.PhoneNumber
	if p.app.ShouldMaskPhoneNumbers(transfer.OrganizationID) {
		contactName = MaskIfPhoneNumber(contactName)
		phoneNumber = MaskPhoneNumber(phoneNumber)
	}

	p.app.WSHub.BroadcastToOrg(transfer.OrganizationID, websocket.WSMessage{
		Type: "transfer_escalation",
		Payload: map[string]interface{}{
			"transfer_id":           transfer.ID.String(),
			"contact_id":            transfer.ContactID.String(),
			"contact_name":          contactName,
			"phone_number":          phoneNumber,
			"escalation_level":      level,
			"level_name":            levelName,
			"waiting_since":         transfer.TransferredAt.Format(time.RFC3339),
			"team_id":               transfer.TeamID,
			"escalation_notify_ids": settings.SLA.EscalationNotifyIDs,
		},
	})

	p.app.Log.Info("Escalation notification sent",
		"transfer_id", transfer.ID,
		"level", level,
		"notify_count", len(settings.SLA.EscalationNotifyIDs),
	)
}

// sendSLAWarningToCustomer sends a warning message to the customer
func (p *SLAProcessor) sendSLAWarningToCustomer(transfer models.AgentTransfer, message string) {
	// Get WhatsApp account
	var account models.WhatsAppAccount
	if err := p.app.DB.Where("name = ?", transfer.WhatsAppAccount).First(&account).Error; err != nil {
		p.app.Log.Error("Failed to load WhatsApp account for SLA warning", "error", err)
		return
	}

	// Get contact
	var contact models.Contact
	if err := p.app.DB.Where("id = ?", transfer.ContactID).First(&contact).Error; err != nil {
		p.app.Log.Error("Failed to load contact for SLA warning", "error", err)
		return
	}

	// Send using unified message sender
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := p.app.SendOutgoingMessage(ctx, OutgoingMessageRequest{
		Account: &account,
		Contact: &contact,
		Type:    models.MessageTypeText,
		Content: message,
	}, SLASendOptions())

	if err != nil {
		p.app.Log.Error("Failed to send SLA warning message", "error", err, "phone", transfer.PhoneNumber)
		return
	}

	p.app.Log.Info("SLA warning message sent to customer", "phone", transfer.PhoneNumber, "transfer_id", transfer.ID)
}

// sendSLAAutoCloseToCustomer sends an auto-close notification message to the customer
func (p *SLAProcessor) sendSLAAutoCloseToCustomer(transfer models.AgentTransfer, message string) {
	// Get WhatsApp account
	var account models.WhatsAppAccount
	if err := p.app.DB.Where("name = ?", transfer.WhatsAppAccount).First(&account).Error; err != nil {
		p.app.Log.Error("Failed to load WhatsApp account for SLA auto-close message", "error", err)
		return
	}

	// Get contact
	var contact models.Contact
	if err := p.app.DB.Where("id = ?", transfer.ContactID).First(&contact).Error; err != nil {
		p.app.Log.Error("Failed to load contact for SLA auto-close message", "error", err)
		return
	}

	// Send using unified message sender
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := p.app.SendOutgoingMessage(ctx, OutgoingMessageRequest{
		Account: &account,
		Contact: &contact,
		Type:    models.MessageTypeText,
		Content: message,
	}, SLASendOptions())

	if err != nil {
		p.app.Log.Error("Failed to send SLA auto-close message", "error", err, "phone", transfer.PhoneNumber)
		return
	}

	p.app.Log.Info("SLA auto-close message sent to customer", "phone", transfer.PhoneNumber, "transfer_id", transfer.ID)
}

// broadcastTransferUpdate broadcasts transfer update via WebSocket
func (p *SLAProcessor) broadcastTransferUpdate(transfer models.AgentTransfer, eventType string) {
	// Get contact info
	var contact models.Contact
	p.app.DB.Where("id = ?", transfer.ContactID).First(&contact)

	contactName := contact.ProfileName
	phoneNumber := contact.PhoneNumber
	if p.app.ShouldMaskPhoneNumbers(transfer.OrganizationID) {
		contactName = MaskIfPhoneNumber(contactName)
		phoneNumber = MaskPhoneNumber(phoneNumber)
	}

	p.app.WSHub.BroadcastToOrg(transfer.OrganizationID, websocket.WSMessage{
		Type: "transfer_" + eventType,
		Payload: map[string]interface{}{
			"id":               transfer.ID.String(),
			"contact_id":       transfer.ContactID.String(),
			"contact_name":     contactName,
			"phone_number":     phoneNumber,
			"status":           transfer.Status,
			"escalation_level": transfer.SLA.EscalationLevel,
			"sla_breached":     transfer.SLA.Breached,
		},
	})
}

// agentRespondedSince checks if the assigned agent sent an outgoing message
// after the given timestamp. This is used to detect active agent conversations
// so that SLA deadlines can be extended instead of firing warnings/auto-close.
func (p *SLAProcessor) agentRespondedSince(transfer models.AgentTransfer, since time.Time) bool {
	if transfer.AgentID == nil {
		return false
	}

	var count int64
	p.app.DB.Model(&models.Message{}).
		Where("contact_id = ? AND sent_by_user_id = ? AND direction = ? AND created_at > ?",
			transfer.ContactID, *transfer.AgentID, models.DirectionOutgoing, since,
		).Count(&count)

	return count > 0
}

// SetSLADeadlines sets SLA deadlines on a new transfer based on settings
func (a *App) SetSLADeadlines(transfer *models.AgentTransfer, settings *models.ChatbotSettings) {
	if !settings.SLA.Enabled {
		return
	}

	now := time.Now()

	// Response deadline (time to pick up)
	if settings.SLA.ResponseMinutes > 0 {
		deadline := now.Add(time.Duration(settings.SLA.ResponseMinutes) * time.Minute)
		transfer.SLA.ResponseDeadline = &deadline
	}

	// Resolution deadline
	if settings.SLA.ResolutionMinutes > 0 {
		deadline := now.Add(time.Duration(settings.SLA.ResolutionMinutes) * time.Minute)
		transfer.SLA.ResolutionDeadline = &deadline
	}

	// Escalation deadline
	if settings.SLA.EscalationMinutes > 0 {
		deadline := now.Add(time.Duration(settings.SLA.EscalationMinutes) * time.Minute)
		transfer.SLA.EscalationAt = &deadline
	}

	// Expiry deadline (auto-close)
	if settings.SLA.AutoCloseHours > 0 {
		deadline := now.Add(time.Duration(settings.SLA.AutoCloseHours) * time.Hour)
		transfer.SLA.ExpiresAt = &deadline
	}

	a.Log.Debug("SLA deadlines set",
		"transfer_id", transfer.ID,
		"response_deadline", transfer.SLA.ResponseDeadline,
		"escalation_at", transfer.SLA.EscalationAt,
		"expires_at", transfer.SLA.ExpiresAt,
	)
}

// UpdateSLAOnPickup updates SLA tracking when a transfer is picked up
func (a *App) UpdateSLAOnPickup(transfer *models.AgentTransfer) {
	now := time.Now()
	transfer.SLA.PickedUpAt = &now

	// Check if SLA was breached (picked up after response deadline)
	if transfer.SLA.ResponseDeadline != nil && now.After(*transfer.SLA.ResponseDeadline) {
		transfer.SLA.Breached = true
		transfer.SLA.BreachedAt = &now
	}
}

// UpdateSLAOnFirstResponse updates SLA tracking when agent sends first response
func (a *App) UpdateSLAOnFirstResponse(transfer *models.AgentTransfer) {
	if transfer.SLA.FirstResponseAt != nil {
		return // Already responded
	}

	now := time.Now()
	transfer.SLA.FirstResponseAt = &now
}

// processClientInactivity handles client inactivity reminders and auto-close for chatbot conversations only
func (p *SLAProcessor) processClientInactivity(orgID uuid.UUID, settings models.ChatbotSettings, now time.Time) {
	// Find contacts where chatbot has sent a message and is waiting for client response
	var contacts []models.Contact
	if err := p.app.DB.Where(
		"organization_id = ? AND chatbot_last_message_at IS NOT NULL",
		orgID,
	).Find(&contacts).Error; err != nil {
		p.app.Log.Error("Failed to find contacts for client inactivity check", "error", err, "org_id", orgID)
		return
	}

	for _, contact := range contacts {
		// Skip if contact has an active agent transfer
		if p.app.hasActiveAgentTransfer(orgID, contact.ID) {
			continue
		}

		// Calculate time since chatbot's last message
		timeSinceChatbotMsg := now.Sub(*contact.ChatbotLastMessageAt)

		// Check if we should auto-close (takes precedence over reminder)
		if settings.ClientInactivity.AutoCloseMinutes > 0 {
			autoCloseThreshold := time.Duration(settings.ClientInactivity.AutoCloseMinutes) * time.Minute
			if timeSinceChatbotMsg >= autoCloseThreshold {
				p.autoCloseChatbotSession(contact, settings)
				continue
			}
		}

		// Check if we should send reminder
		if settings.ClientInactivity.ReminderMinutes > 0 && !contact.ChatbotReminderSent {
			reminderThreshold := time.Duration(settings.ClientInactivity.ReminderMinutes) * time.Minute
			if timeSinceChatbotMsg >= reminderThreshold {
				p.sendChatbotReminder(contact, settings)
			}
		}
	}
}

// sendChatbotReminder sends a reminder message to an inactive client during chatbot conversation
func (p *SLAProcessor) sendChatbotReminder(contact models.Contact, settings models.ChatbotSettings) {
	if settings.ClientInactivity.ReminderMessage == "" {
		return
	}

	// Get WhatsApp account
	var account models.WhatsAppAccount
	if err := p.app.DB.Where("name = ?", contact.WhatsAppAccount).First(&account).Error; err != nil {
		p.app.Log.Error("Failed to load WhatsApp account for chatbot reminder", "error", err)
		return
	}

	// Send using unified message sender
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := p.app.SendOutgoingMessage(ctx, OutgoingMessageRequest{
		Account: &account,
		Contact: &contact,
		Type:    models.MessageTypeText,
		Content: settings.ClientInactivity.ReminderMessage,
	}, SLASendOptions())

	if err != nil {
		p.app.Log.Error("Failed to send chatbot reminder message", "error", err, "phone", contact.PhoneNumber)
		return
	}

	// Mark reminder as sent
	if err := p.app.DB.Model(&contact).Update("chatbot_reminder_sent", true).Error; err != nil {
		p.app.Log.Error("Failed to update chatbot_reminder_sent", "error", err, "contact_id", contact.ID)
	}

	p.app.Log.Info("Chatbot reminder sent",
		"contact_id", contact.ID,
		"phone", contact.PhoneNumber,
		"inactive_since", contact.ChatbotLastMessageAt,
	)
}

// autoCloseChatbotSession closes a chatbot session due to client inactivity
func (p *SLAProcessor) autoCloseChatbotSession(contact models.Contact, settings models.ChatbotSettings) {
	// Save the timestamp before clearing for logging
	var inactiveSince time.Time
	if contact.ChatbotLastMessageAt != nil {
		inactiveSince = *contact.ChatbotLastMessageAt
	}

	// Send auto-close message if configured
	if settings.ClientInactivity.AutoCloseMessage != "" {
		var account models.WhatsAppAccount
		if err := p.app.DB.Where("name = ?", contact.WhatsAppAccount).First(&account).Error; err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			_, err := p.app.SendOutgoingMessage(ctx, OutgoingMessageRequest{
				Account: &account,
				Contact: &contact,
				Type:    models.MessageTypeText,
				Content: settings.ClientInactivity.AutoCloseMessage,
			}, SLASendOptions())

			if err != nil {
				p.app.Log.Error("Failed to send chatbot auto-close message", "error", err, "phone", contact.PhoneNumber)
			}
		}
	}

	// Clear chatbot tracking fields to close the session
	if err := p.app.DB.Model(&contact).Updates(map[string]any{
		"chatbot_last_message_at": nil,
		"chatbot_reminder_sent":   false,
	}).Error; err != nil {
		p.app.Log.Error("Failed to close chatbot session for client inactivity", "error", err, "contact_id", contact.ID)
		return
	}

	p.app.Log.Info("Chatbot session closed due to client inactivity",
		"contact_id", contact.ID,
		"phone", contact.PhoneNumber,
		"inactive_since", inactiveSince,
	)
}

// UpdateContactChatbotMessage updates the chatbot last message timestamp for a contact
func (a *App) UpdateContactChatbotMessage(contactID uuid.UUID) {
	now := time.Now()
	a.DB.Model(&models.Contact{}).
		Where("id = ?", contactID).
		Updates(map[string]interface{}{
			"chatbot_last_message_at": now,
			"chatbot_reminder_sent":   false, // Reset reminder when chatbot sends a new message
		})
}

// ClearContactChatbotTracking clears chatbot tracking when client replies or is transferred
func (a *App) ClearContactChatbotTracking(contactID uuid.UUID) {
	a.DB.Model(&models.Contact{}).
		Where("id = ?", contactID).
		Updates(map[string]interface{}{
			"chatbot_last_message_at": nil,
			"chatbot_reminder_sent":   false,
		})
}
