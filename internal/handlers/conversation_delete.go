package handlers

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/internal/websocket"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
	"gorm.io/gorm"
)

// DeleteConversation permanently deletes a contact's chat history and removes
// the chat from the chat list.
//
// Hard-deleted: messages (and their local media files, when nothing else
// references them), internal notes and chatbot sessions. Active agent
// transfers are closed so they leave the agent queues. The contact itself is
// soft-deleted, so if the customer writes again GetOrCreateContact restores it
// with an empty history. Call logs and campaign stats are kept.
func (a *App) DeleteConversation(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	if !a.HasPermission(userID, models.ResourceContacts, models.ActionDelete, orgID) {
		return r.SendErrorEnvelope(fasthttp.StatusForbidden, "You do not have permission to delete chats", nil, "")
	}

	contactID, err := parsePathUUID(r, "id", "contact")
	if err != nil {
		return nil
	}

	contact, err := findByIDAndOrg[models.Contact](a.DB, r, contactID, orgID, "Contact")
	if err != nil {
		return nil
	}

	var (
		mediaPaths      []string
		deletedMessages int64
		closedTransfers []models.AgentTransfer
	)
	now := time.Now()

	err = a.DB.Transaction(func(tx *gorm.DB) error {
		const contactMessageIDs = "SELECT id FROM messages WHERE contact_id = ?"

		if err := tx.Unscoped().Model(&models.Message{}).
			Where("contact_id = ? AND media_url <> ''", contactID).
			Distinct().Pluck("media_url", &mediaPaths).Error; err != nil {
			return err
		}

		// Campaign recipients keep their delivery stats, only the link to the message goes.
		if err := tx.Unscoped().Model(&models.BulkMessageRecipient{}).
			Where("message_id IN ("+contactMessageIDs+")", contactID).
			UpdateColumn("message_id", nil).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Model(&models.Message{}).
			Where("reply_to_message_id IN ("+contactMessageIDs+")", contactID).
			UpdateColumn("reply_to_message_id", nil).Error; err != nil {
			return err
		}

		result := tx.Unscoped().Where("contact_id = ?", contactID).Delete(&models.Message{})
		if result.Error != nil {
			return result.Error
		}
		deletedMessages = result.RowsAffected

		if err := tx.Unscoped().
			Where("session_id IN (SELECT id FROM chatbot_sessions WHERE contact_id = ?)", contactID).
			Delete(&models.ChatbotSessionMessage{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("contact_id = ?", contactID).Delete(&models.ChatbotSession{}).Error; err != nil {
			return err
		}

		if err := tx.Unscoped().Where("contact_id = ?", contactID).Delete(&models.ConversationNote{}).Error; err != nil {
			return err
		}

		if err := tx.Where("contact_id = ? AND status = ?", contactID, models.TransferStatusActive).
			Find(&closedTransfers).Error; err != nil {
			return err
		}
		if len(closedTransfers) > 0 {
			if err := tx.Model(&models.AgentTransfer{}).
				Where("contact_id = ? AND status = ?", contactID, models.TransferStatusActive).
				Updates(map[string]any{
					"status":     models.TransferStatusResumed,
					"resumed_at": now,
					"resumed_by": userID,
				}).Error; err != nil {
				return err
			}
		}

		// Reset message-derived fields so a restored contact doesn't show a stale preview.
		if err := tx.Model(contact).Updates(map[string]any{
			"last_message_at":         nil,
			"last_message_preview":    "",
			"is_read":                 true,
			"chatbot_last_message_at": nil,
			"chatbot_reminder_sent":   false,
		}).Error; err != nil {
			return err
		}

		return tx.Delete(contact).Error
	})
	if err != nil {
		a.Log.Error("Failed to delete conversation", "error", err, "contact_id", contactID)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to delete chat", nil, "")
	}

	a.removeUnreferencedMediaFiles(mediaPaths)

	for i := range closedTransfers {
		closedTransfers[i].Status = models.TransferStatusResumed
		closedTransfers[i].ResumedAt = &now
		closedTransfers[i].ResumedBy = &userID
		a.broadcastTransferResumed(&closedTransfers[i])
	}

	if a.WSHub != nil {
		a.WSHub.BroadcastToOrg(orgID, websocket.WSMessage{
			Type:    websocket.TypeConversationDeleted,
			Payload: map[string]any{"contact_id": contactID.String()},
		})
	}

	a.logAudit(orgID, userID, "contact", contactID, models.AuditActionDeleted, contact, nil,
		map[string]any{"deleted_messages": deletedMessages})

	return r.SendEnvelope(map[string]any{
		"message":          "Chat deleted successfully",
		"deleted_messages": deletedMessages,
	})
}

// removeUnreferencedMediaFiles deletes local media files that no message or
// campaign still points at. Campaign header files are shared by every
// recipient's message, so a path is only removed once its last reference is gone.
// Failures are logged and ignored: the database rows are already deleted.
func (a *App) removeUnreferencedMediaFiles(paths []string) {
	for _, p := range paths {
		if strings.Contains(p, "://") {
			continue // external URL, not a file we stored
		}
		var refs int64
		a.DB.Unscoped().Model(&models.Message{}).Where("media_url = ?", p).Count(&refs)
		if refs > 0 {
			continue
		}
		a.DB.Unscoped().Model(&models.BulkMessageCampaign{}).Where("header_media_local_path = ?", p).Count(&refs)
		if refs > 0 {
			continue
		}

		fullPath, err := a.resolveMediaPath(p)
		if err != nil {
			continue // path outside the media directory
		}
		if err := os.Remove(fullPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			a.Log.Warn("Failed to remove media file", "error", err, "path", p)
		}
	}
}

// resolveMediaPath turns a stored relative media path into an absolute path
// inside the media storage directory, rejecting anything that escapes it.
func (a *App) resolveMediaPath(relPath string) (string, error) {
	baseDir, err := filepath.Abs(a.getMediaStoragePath())
	if err != nil {
		return "", err
	}
	fullPath, err := filepath.Abs(filepath.Join(baseDir, filepath.Clean(relPath)))
	if err != nil || !strings.HasPrefix(fullPath, baseDir+string(os.PathSeparator)) {
		return "", errors.New("invalid media path")
	}
	return fullPath, nil
}
