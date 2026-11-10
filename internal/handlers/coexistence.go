package handlers

import (
	"context"
	"sort"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/contactutil"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/pkg/whatsapp"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

// historyDeclinedCode is the error code Meta sends in a history webhook when
// the business turned off chat history sharing in the WhatsApp Business app.
const historyDeclinedCode = 2593109

// historyMediaPlaceholder is stored for media messages that Meta sends as
// media_placeholder. Meta sends the asset later for media from the last 14
// days; the row is then updated in place.
const historyMediaPlaceholder = "[media]"

// SMBHistoryItem is one entry of a history webhook (coexistence chat history).
type SMBHistoryItem struct {
	Metadata struct {
		Phase      int `json:"phase"`
		ChunkOrder int `json:"chunk_order"`
		Progress   int `json:"progress"`
	} `json:"metadata"`
	Threads []SMBHistoryThread `json:"threads"`
	Errors  []struct {
		Code    int    `json:"code"`
		Title   string `json:"title"`
		Message string `json:"message"`
	} `json:"errors,omitempty"`
}

// SMBHistoryThread is one 1:1 chat in a history webhook.
type SMBHistoryThread struct {
	ID      string `json:"id"`
	Context struct {
		WaID   string `json:"wa_id"`
		UserID string `json:"user_id,omitempty"`
	} `json:"context"`
	Messages []SMBHistoryMessage `json:"messages"`
}

// SMBHistoryMessage is a message in a history thread.
type SMBHistoryMessage struct {
	IncomingTextMessage
	HistoryContext struct {
		Status string `json:"status"`
	} `json:"history_context"`
}

// requestCoexistenceSync asks Meta to send the Business app contacts and chat
// history. Meta allows this once, within 24 hours of onboarding.
func (a *App) requestCoexistenceSync(ctx context.Context, account *models.WhatsAppAccount) map[string]any {
	waAccount := a.toWhatsAppAccount(account)
	result := map[string]any{}
	for _, syncType := range []string{whatsapp.SMBSyncTypeContacts, whatsapp.SMBSyncTypeHistory} {
		requestID, err := a.WhatsApp.RequestSMBAppDataSync(ctx, waAccount, syncType)
		if err != nil {
			a.Log.Error("Coexistence sync request failed", "sync_type", syncType, "phone_id", account.PhoneID, "error", err)
			result[syncType] = map[string]any{"success": false, "error": err.Error()}
			continue
		}
		a.Log.Info("Coexistence sync requested", "sync_type", syncType, "phone_id", account.PhoneID, "request_id", requestID)
		result[syncType] = map[string]any{"success": true, "request_id": requestID}
	}
	return result
}

// SyncCoexistenceData requests the one-time contacts and chat history sync
// for a coexistence (WhatsApp Business app) number.
// POST /api/accounts/{id}/coexistence-sync
func (a *App) SyncCoexistenceData(r *fastglue.Request) error {
	orgID, userID, err := a.requireAuth(r, models.ResourceAccounts, models.ActionWrite)
	if err != nil {
		return nil
	}

	id, err := parsePathUUID(r, "id", "account")
	if err != nil {
		return nil
	}

	account, err := a.resolveWhatsAppAccountByID(r, id, orgID)
	if err != nil {
		return nil
	}

	if !account.IsSMB {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest,
			"Account is not a WhatsApp Business app (coexistence) number", nil, "")
	}

	result := a.requestCoexistenceSync(context.Background(), account)
	a.logAudit(orgID, userID, "account", account.ID, models.AuditActionUpdated, nil,
		map[string]any{"coexistence_sync": result})

	return r.SendEnvelope(map[string]any{"sync": result})
}

// processHistorySync imports chat history chunks sent after a coexistence
// history sync. Messages are saved without triggering chatbots or outgoing
// webhooks, and duplicates (by WhatsApp message ID) are skipped.
func (a *App) processHistorySync(phoneNumberID string, items []SMBHistoryItem) {
	defer func() {
		if r := recover(); r != nil {
			a.Log.Error("Panic recovered in processHistorySync", "panic", r, "phone_id", phoneNumberID)
		}
	}()

	account, err := a.getWhatsAppAccountCached(phoneNumberID)
	if err != nil {
		a.Log.Error("WhatsApp account not found for history sync", "phone_id", phoneNumberID, "error", err)
		return
	}

	// Chunks may arrive out of order.
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].Metadata.ChunkOrder < items[j].Metadata.ChunkOrder
	})

	ctx := context.Background()
	imported := 0
	for _, item := range items {
		for _, e := range item.Errors {
			if e.Code == historyDeclinedCode {
				a.Log.Warn("Business declined chat history sharing", "phone_id", phoneNumberID)
			} else {
				a.Log.Error("History sync error", "phone_id", phoneNumberID, "code", e.Code, "message", e.Message)
			}
		}
		for _, thread := range item.Threads {
			imported += a.importHistoryThread(ctx, account, thread)
		}
		a.Log.Info("Processed history chunk",
			"phone_id", phoneNumberID,
			"phase", item.Metadata.Phase,
			"chunk_order", item.Metadata.ChunkOrder,
			"progress", item.Metadata.Progress,
		)
	}
	a.Log.Info("History sync batch imported", "phone_id", phoneNumberID, "messages", imported)
}

func (a *App) importHistoryThread(ctx context.Context, account *models.WhatsAppAccount, thread SMBHistoryThread) int {
	contactPhone := thread.Context.WaID
	if contactPhone == "" {
		contactPhone = thread.ID
	}
	if contactPhone == "" {
		return 0
	}

	contact, _, err := contactutil.GetOrCreateContact(a.DB, account.OrganizationID, contactPhone, "")
	if err != nil {
		a.Log.Error("Failed to get or create contact for history", "phone", contactPhone, "error", err)
		return 0
	}
	if thread.Context.UserID != "" && contact.BSUID == "" {
		a.DB.Model(contact).Update("bsuid", thread.Context.UserID)
		contact.BSUID = thread.Context.UserID
	}

	var latest time.Time
	var latestPreview string
	imported := 0
	for _, hm := range thread.Messages {
		msg := hm.IncomingTextMessage
		if msg.ID == "" {
			continue
		}

		incoming := msg.FromUserID != "" || msg.From == contactPhone || msg.From == thread.ID
		createdAt := parseUnixTimestamp(msg.Timestamp)

		var text, msgType string
		var media *MediaInfo
		if msg.Type == "media_placeholder" {
			text, msgType = historyMediaPlaceholder, string(models.MessageTypeText)
		} else {
			extracted := a.extractMessageContent(ctx, msg, account)
			text, msgType, media = extracted.Text, extracted.Type, extracted.Media
		}

		var existing models.Message
		if err := a.DB.Where("whats_app_message_id = ? AND organization_id = ?", msg.ID, account.OrganizationID).
			First(&existing).Error; err == nil {
			// Fill in media that arrives after its placeholder.
			if existing.Content == historyMediaPlaceholder && msg.Type != "media_placeholder" {
				updates := map[string]any{"message_type": msgType, "content": text}
				if media != nil {
					updates["media_url"] = media.MediaURL
					updates["media_mime_type"] = media.MediaMimeType
					updates["media_filename"] = media.MediaFilename
				}
				a.DB.Model(&existing).Updates(updates)
			}
			continue
		}

		message := models.Message{
			BaseModel:         models.BaseModel{ID: uuid.New(), CreatedAt: createdAt},
			OrganizationID:    account.OrganizationID,
			WhatsAppAccount:   account.Name,
			ContactID:         contact.ID,
			WhatsAppMessageID: msg.ID,
			MessageType:       models.MessageType(msgType),
			Content:           text,
		}
		if incoming {
			message.Direction = models.DirectionIncoming
			message.Status = models.MessageStatusReceived
		} else {
			message.Direction = models.DirectionOutgoing
			message.Status = historyStatus(hm.HistoryContext.Status)
		}
		if media != nil {
			message.MediaURL = media.MediaURL
			message.MediaMimeType = media.MediaMimeType
			message.MediaFilename = media.MediaFilename
		}

		if err := a.DB.Create(&message).Error; err != nil {
			a.Log.Error("Failed to save history message", "message_id", msg.ID, "error", err)
			continue
		}
		imported++

		if createdAt.After(latest) {
			latest = createdAt
			latestPreview = text
			if msgType != string(models.MessageTypeText) {
				latestPreview = "[" + msgType + "]"
			}
		}
	}

	// Only move the conversation forward; never overwrite a newer message.
	if !latest.IsZero() && (contact.LastMessageAt == nil || latest.After(*contact.LastMessageAt)) {
		if len(latestPreview) > 100 {
			latestPreview = latestPreview[:97] + "..."
		}
		a.DB.Model(contact).Updates(map[string]any{
			"last_message_at":      latest,
			"last_message_preview": latestPreview,
			"whats_app_account":    account.Name,
		})
	}
	return imported
}

func historyStatus(s string) models.MessageStatus {
	switch s {
	case "DELIVERED":
		return models.MessageStatusDelivered
	case "READ", "PLAYED":
		return models.MessageStatusRead
	case "ERROR":
		return models.MessageStatusFailed
	case "PENDING":
		return models.MessageStatusPending
	default:
		return models.MessageStatusSent
	}
}

func parseUnixTimestamp(ts string) time.Time {
	sec, err := strconv.ParseInt(ts, 10, 64)
	if err != nil || sec <= 0 {
		return time.Now()
	}
	return time.Unix(sec, 0)
}
