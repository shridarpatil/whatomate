package handlers

import (
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nikyjain/whatomate/internal/models"
	"github.com/nikyjain/whatomate/internal/utils"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

// MessageSearchResult represents a single matching message from global search
type MessageSearchResult struct {
	ID              uuid.UUID          `json:"id"`
	ContactID       uuid.UUID          `json:"contact_id"`
	ContactName     string             `json:"contact_name"`
	PhoneNumber     string             `json:"phone_number"`
	Direction       models.Direction   `json:"direction"`
	MessageType     models.MessageType `json:"message_type"`
	Content         string             `json:"content"`
	MediaFilename   string             `json:"media_filename,omitempty"`
	WhatsAppAccount string             `json:"whatsapp_account"`
	CreatedAt       time.Time          `json:"created_at"`
}

// SearchMessages searches messages across all conversations in the organization by text content.
// Gated by org scoping; agents without contacts:read are restricted to their assigned contacts.
func (a *App) SearchMessages(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	q := strings.TrimSpace(string(r.RequestCtx.QueryArgs().Peek("q")))
	if q == "" {
		q = strings.TrimSpace(string(r.RequestCtx.QueryArgs().Peek("search")))
	}

	if q == "" {
		return r.SendEnvelope(map[string]any{
			"messages": []MessageSearchResult{},
			"total":    0,
		})
	}

	if len(q) > 200 {
		q = q[:200]
	}

	limit := 30
	if l, err := strconv.Atoi(string(r.RequestCtx.QueryArgs().Peek("limit"))); err == nil && l > 0 {
		if l > 50 {
			limit = 50
		} else {
			limit = l
		}
	}

	accountFilter := strings.TrimSpace(string(r.RequestCtx.QueryArgs().Peek("account")))
	searchPattern := "%" + q + "%"

	type rowResult struct {
		ID              uuid.UUID          `gorm:"column:id"`
		ContactID       uuid.UUID          `gorm:"column:contact_id"`
		ProfileName     string             `gorm:"column:profile_name"`
		PhoneNumber     string             `gorm:"column:phone_number"`
		Direction       models.Direction   `gorm:"column:direction"`
		MessageType     models.MessageType `gorm:"column:message_type"`
		Content         string             `gorm:"column:content"`
		MediaFilename   string             `gorm:"column:media_filename"`
		WhatsAppAccount string             `gorm:"column:whats_app_account"`
		CreatedAt       time.Time          `gorm:"column:created_at"`
	}

	dbQuery := a.DB.Table("messages").
		Select("messages.id, messages.contact_id, messages.direction, messages.message_type, messages.content, messages.media_filename, messages.whats_app_account, messages.created_at, contacts.profile_name, contacts.phone_number").
		Joins("INNER JOIN contacts ON contacts.id = messages.contact_id AND contacts.deleted_at IS NULL").
		Where("messages.organization_id = ? AND messages.deleted_at IS NULL", orgID).
		Where("(messages.content ILIKE ? OR messages.media_filename ILIKE ?)", searchPattern, searchPattern)

	if accountFilter != "" {
		dbQuery = dbQuery.Where("messages.whats_app_account = ?", accountFilter)
	}

	// Permission scoping for users without contacts:read permission
	if !a.HasPermission(userID, models.ResourceContacts, models.ActionRead, orgID) {
		dbQuery = dbQuery.Where("(contacts.assigned_user_id = ? OR contacts.id IN (?))",
			userID,
			a.DB.Model(&models.AgentTransfer{}).
				Select("contact_id").
				Where("agent_id = ? AND organization_id = ? AND status = ?", userID, orgID, models.TransferStatusActive),
		)
	}

	var rows []rowResult
	if err := dbQuery.Order("messages.created_at DESC").Limit(limit).Scan(&rows).Error; err != nil {
		a.Log.Error("Failed to search messages", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to search messages", nil, "")
	}

	shouldMask := a.ShouldMaskPhoneNumbers(orgID)

	results := make([]MessageSearchResult, len(rows))
	for i, row := range rows {
		phone := row.PhoneNumber
		name := row.ProfileName
		if shouldMask {
			phone = utils.MaskPhoneNumber(phone)
			name = utils.MaskIfPhoneNumber(name)
		}
		if name == "" {
			name = phone
		}
		results[i] = MessageSearchResult{
			ID:              row.ID,
			ContactID:       row.ContactID,
			ContactName:     name,
			PhoneNumber:     phone,
			Direction:       row.Direction,
			MessageType:     row.MessageType,
			Content:         row.Content,
			MediaFilename:   row.MediaFilename,
			WhatsAppAccount: row.WhatsAppAccount,
			CreatedAt:       row.CreatedAt,
		}
	}

	return r.SendEnvelope(map[string]any{
		"messages": results,
		"total":    len(results),
	})
}

