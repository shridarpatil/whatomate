package handlers

import (
	"time"

	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

// DashboardStats represents dashboard statistics
type DashboardStats struct {
	TotalMessages   int64   `json:"total_messages"`
	MessagesChange  float64 `json:"messages_change"`
	TotalContacts   int64   `json:"total_contacts"`
	ContactsChange  float64 `json:"contacts_change"`
	ChatbotSessions int64   `json:"chatbot_sessions"`
	ChatbotChange   float64 `json:"chatbot_change"`
	CampaignsSent   int64   `json:"campaigns_sent"`
	CampaignsChange float64 `json:"campaigns_change"`
}

// RecentMessageResponse represents a recent message in the dashboard
type RecentMessageResponse struct {
	ID          string               `json:"id"`
	ContactName string               `json:"contact_name"`
	Content     string               `json:"content"`
	Direction   models.Direction     `json:"direction"`
	CreatedAt   string               `json:"created_at"`
	Status      models.MessageStatus `json:"status"`
}

// GetDashboardStats returns dashboard statistics for the organization
func (a *App) GetDashboardStats(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	now := time.Now()

	// Parse date range from query params
	fromStr := string(r.RequestCtx.QueryArgs().Peek("from"))
	toStr := string(r.RequestCtx.QueryArgs().Peek("to"))

	var periodStart, periodEnd time.Time
	if fromStr != "" && toStr != "" {
		var errMsg string
		periodStart, periodEnd, errMsg = parseDateRange(fromStr, toStr)
		if errMsg != "" {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, errMsg, nil, "")
		}
	} else {
		// Default to current month
		periodStart = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
		periodEnd = now
	}

	// Calculate the previous period for comparison (same duration, before the current period)
	periodDuration := periodEnd.Sub(periodStart)
	previousPeriodStart := periodStart.Add(-periodDuration - time.Nanosecond)
	previousPeriodEnd := periodStart.Add(-time.Nanosecond)

	// Get message counts for the selected period
	var previousPeriodMessages, currentPeriodMessages int64
	a.DB.Model(&models.Message{}).
		Where("organization_id = ? AND created_at >= ? AND created_at <= ?", orgID, previousPeriodStart, previousPeriodEnd).
		Count(&previousPeriodMessages)

	a.DB.Model(&models.Message{}).
		Where("organization_id = ? AND created_at >= ? AND created_at <= ?", orgID, periodStart, periodEnd).
		Count(&currentPeriodMessages)

	messagesChange := calculatePercentageChange(previousPeriodMessages, currentPeriodMessages)

	// Get contact counts for the selected period
	var previousPeriodContacts, currentPeriodContacts int64
	a.DB.Model(&models.Contact{}).
		Where("organization_id = ? AND created_at >= ? AND created_at <= ?", orgID, previousPeriodStart, previousPeriodEnd).
		Count(&previousPeriodContacts)

	a.DB.Model(&models.Contact{}).
		Where("organization_id = ? AND created_at >= ? AND created_at <= ?", orgID, periodStart, periodEnd).
		Count(&currentPeriodContacts)

	contactsChange := calculatePercentageChange(previousPeriodContacts, currentPeriodContacts)

	// Get chatbot session counts for the selected period
	var previousPeriodSessions, currentPeriodSessions int64
	a.DB.Model(&models.ChatbotSession{}).
		Where("organization_id = ? AND created_at >= ? AND created_at <= ?", orgID, previousPeriodStart, previousPeriodEnd).
		Count(&previousPeriodSessions)

	a.DB.Model(&models.ChatbotSession{}).
		Where("organization_id = ? AND created_at >= ? AND created_at <= ?", orgID, periodStart, periodEnd).
		Count(&currentPeriodSessions)

	sessionsChange := calculatePercentageChange(previousPeriodSessions, currentPeriodSessions)

	// Get campaign counts for the selected period
	var previousPeriodCampaigns, currentPeriodCampaigns int64
	a.DB.Model(&models.BulkMessageCampaign{}).
		Where("organization_id = ? AND status IN ('completed', 'processing') AND created_at >= ? AND created_at <= ?", orgID, previousPeriodStart, previousPeriodEnd).
		Count(&previousPeriodCampaigns)

	a.DB.Model(&models.BulkMessageCampaign{}).
		Where("organization_id = ? AND status IN ('completed', 'processing') AND created_at >= ? AND created_at <= ?", orgID, periodStart, periodEnd).
		Count(&currentPeriodCampaigns)

	campaignsChange := calculatePercentageChange(previousPeriodCampaigns, currentPeriodCampaigns)

	stats := DashboardStats{
		TotalMessages:   currentPeriodMessages,
		MessagesChange:  messagesChange,
		TotalContacts:   currentPeriodContacts,
		ContactsChange:  contactsChange,
		ChatbotSessions: currentPeriodSessions,
		ChatbotChange:   sessionsChange,
		CampaignsSent:   currentPeriodCampaigns,
		CampaignsChange: campaignsChange,
	}

	// Get recent messages
	var messages []models.Message
	a.DB.Where("organization_id = ?", orgID).
		Preload("Contact").
		Order("created_at DESC").
		Limit(5).
		Find(&messages)

	recentMessages := make([]RecentMessageResponse, len(messages))
	for i, msg := range messages {
		contactName := "Unknown"
		if msg.Contact != nil {
			if msg.Contact.ProfileName != "" {
				contactName = msg.Contact.ProfileName
			} else {
				contactName = msg.Contact.PhoneNumber
			}
		}

		content := msg.Content
		if content == "" && msg.MessageType != models.MessageTypeText {
			content = "[" + string(msg.MessageType) + "]"
		}

		recentMessages[i] = RecentMessageResponse{
			ID:          msg.ID.String(),
			ContactName: contactName,
			Content:     content,
			Direction:   msg.Direction,
			CreatedAt:   msg.CreatedAt.Format(time.RFC3339),
			Status:      msg.Status,
		}
	}

	return r.SendEnvelope(map[string]interface{}{
		"stats":           stats,
		"recent_messages": recentMessages,
	})
}

// calculatePercentageChange calculates the percentage change between two values
func calculatePercentageChange(previous, current int64) float64 {
	if previous == 0 {
		if current > 0 {
			return 100.0
		}
		return 0.0
	}
	return float64(current-previous) / float64(previous) * 100.0
}
