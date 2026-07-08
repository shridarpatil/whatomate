package handlers

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

// ForwardMessageRequest identifies the destination of a forward.
type ForwardMessageRequest struct {
	ContactID       string `json:"contact_id"`
	WhatsAppAccount string `json:"whatsapp_account"` // optional account-name override
}

// ForwardMessage re-sends the content of an existing message to another
// contact as a NEW outgoing message. The WhatsApp Cloud API has no native
// forward (the "Forwarded" label is consumer-app only), so this mirrors what
// business inboxes do: text is re-sent as text, media is re-read from local
// storage and re-uploaded (inbound Meta media IDs are not reusable for
// sending). Subject to the destination contact's 24h service window like any
// free-form message.
func (a *App) ForwardMessage(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	messageID, err := parsePathUUID(r, "id", "message")
	if err != nil {
		return nil
	}

	var req ForwardMessageRequest
	if err := json.Unmarshal(r.RequestCtx.PostBody(), &req); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid request body", nil, "")
	}
	destContactID, err := uuid.Parse(req.ContactID)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid contact_id", nil, "")
	}

	// Source message, org-guarded.
	message, err := findByIDAndOrg[models.Message](a.DB, r, messageID, orgID, "Message")
	if err != nil {
		return nil
	}

	// The agent must be able to see the source conversation (same assignment
	// scoping as the rest of the chat surface).
	var sourceContact models.Contact
	srcQuery := a.DB.Where("id = ? AND organization_id = ?", message.ContactID, orgID)
	srcQuery = a.scopeAssignedContact(srcQuery, userID, orgID)
	if err := srcQuery.First(&sourceContact).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusForbidden, "Access denied", nil, "")
	}

	// ...and be allowed to message the destination contact (agents can only
	// send to their assigned contacts, mirroring SendMessage).
	var destContact models.Contact
	destQuery := a.DB.Where("id = ? AND organization_id = ?", destContactID, orgID)
	destQuery = a.scopeAssignedContact(destQuery, userID, orgID)
	if err := destQuery.First(&destContact).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Contact not found", nil, "")
	}
	if destContact.ID == message.ContactID {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Cannot forward to the same conversation", nil, "")
	}

	// Outgoing account: explicit override > destination contact's account > org default.
	accountName := destContact.WhatsAppAccount
	if req.WhatsAppAccount != "" {
		accountName = req.WhatsAppAccount
	}
	account, err := a.resolveWhatsAppAccount(orgID, accountName)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Failed to resolve WhatsApp account", nil, "")
	}

	msgReq := OutgoingMessageRequest{
		Account: account,
		Contact: &destContact,
	}

	switch message.MessageType {
	case models.MessageTypeText:
		msgReq.Type = models.MessageTypeText
		msgReq.Content = message.Content

	case models.MessageTypeImage, models.MessageTypeVideo, models.MessageTypeAudio, models.MessageTypeDocument:
		if message.MediaURL == "" {
			return r.SendErrorEnvelope(fasthttp.StatusNotFound, "No media found for this message", nil, "")
		}
		data, errResp := a.readLocalMedia(r, message.MediaURL)
		if data == nil {
			return errResp
		}
		msgReq.Type = message.MessageType
		msgReq.MediaData = data
		msgReq.MediaURL = message.MediaURL // reuse the stored file for the new message record
		msgReq.MediaMimeType = message.MediaMimeType
		// Inbound images/videos/audio/stickers are stored without a filename
		// (only documents carry one). The Cloud API media upload rejects an
		// empty multipart filename with "(#100) The parameter file is
		// required", so synthesize one from the mime type when it's missing.
		filename := message.MediaFilename
		if filename == "" {
			filename = "forwarded" + getExtensionFromMimeType(message.MediaMimeType)
		}
		msgReq.MediaFilename = filename
		msgReq.Caption = message.Content
		msgReq.Content = message.Content

	default:
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "This message type cannot be forwarded", nil, "")
	}

	opts := DefaultSendOptions()
	opts.SentByUserID = &userID

	sent, err := a.SendOutgoingMessage(context.Background(), msgReq, opts)
	if err != nil {
		a.Log.Error("Failed to forward message", "error", err, "source_message_id", messageID)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to forward message", nil, "")
	}

	return r.SendEnvelope(map[string]any{
		"id":         sent.ID,
		"contact_id": sent.ContactID,
		"status":     sent.Status,
	})
}

// readLocalMedia safely reads a stored media file by its relative MediaURL,
// applying the same traversal/symlink guards as ServeMedia. On failure it
// writes the error envelope and returns nil data.
func (a *App) readLocalMedia(r *fastglue.Request, mediaURL string) ([]byte, error) {
	filePath := filepath.Clean(mediaURL)
	baseDir, err := filepath.Abs(a.getMediaStoragePath())
	if err != nil {
		a.Log.Error("Storage configuration error", "error", err)
		return nil, r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Storage configuration error", nil, "")
	}
	fullPath, err := filepath.Abs(filepath.Join(baseDir, filePath))
	if err != nil || !strings.HasPrefix(fullPath, baseDir+string(os.PathSeparator)) {
		return nil, r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid file path", nil, "")
	}
	info, err := os.Lstat(fullPath)
	if err != nil {
		return nil, r.SendErrorEnvelope(fasthttp.StatusNotFound, "Media file not found", nil, "")
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid file path", nil, "")
	}
	data, err := os.ReadFile(fullPath)
	if err != nil {
		a.Log.Error("Failed to read media file", "path", fullPath, "error", err)
		return nil, r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to read file", nil, "")
	}
	return data, nil
}
