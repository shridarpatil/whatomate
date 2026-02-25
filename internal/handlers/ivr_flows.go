package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

// IVRFlowRequest represents the request body for creating/updating an IVR flow
type IVRFlowRequest struct {
	WhatsAppAccount string       `json:"whatsapp_account"`
	Name            string       `json:"name"`
	Description     string       `json:"description"`
	IsActive        bool         `json:"is_active"`
	IsCallStart     bool         `json:"is_call_start"`
	Menu            models.JSONB `json:"menu"`
	WelcomeAudioURL string       `json:"welcome_audio_url"`
}

// ListIVRFlows returns all IVR flows for the organization
func (a *App) ListIVRFlows(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}
	if err := a.requirePermission(r, userID, models.ResourceIVRFlows, models.ActionRead); err != nil {
		return nil
	}

	pg := parsePagination(r)
	account := string(r.RequestCtx.QueryArgs().Peek("account"))

	query := a.DB.Where("organization_id = ?", orgID).Order("created_at DESC")
	if account != "" {
		query = query.Where("whatsapp_account = ?", account)
	}

	var total int64
	a.DB.Model(&models.IVRFlow{}).Where("organization_id = ?", orgID).Count(&total)

	var flows []models.IVRFlow
	if err := pg.Apply(query).Find(&flows).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to fetch IVR flows", nil, "")
	}

	return r.SendEnvelope(map[string]any{
		"ivr_flows": flows,
		"total":     total,
		"page":      pg.Page,
		"limit":     pg.Limit,
	})
}

// GetIVRFlow returns a single IVR flow by ID
func (a *App) GetIVRFlow(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}
	if err := a.requirePermission(r, userID, models.ResourceIVRFlows, models.ActionRead); err != nil {
		return nil
	}

	flowID, err := parsePathUUID(r, "id", "IVR flow")
	if err != nil {
		return nil
	}

	flow, err := findByIDAndOrg[models.IVRFlow](a.DB, r, flowID, orgID, "IVR Flow")
	if err != nil {
		return nil
	}

	return r.SendEnvelope(flow)
}

// CreateIVRFlow creates a new IVR flow
func (a *App) CreateIVRFlow(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}
	if err := a.requirePermission(r, userID, models.ResourceIVRFlows, models.ActionWrite); err != nil {
		return nil
	}

	var req IVRFlowRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}

	if req.Name == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Name is required", nil, "")
	}
	if req.WhatsAppAccount == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "WhatsApp account is required", nil, "")
	}

	// If marking this as call start, unset others for the same account
	if req.IsCallStart {
		a.DB.Model(&models.IVRFlow{}).
			Where("organization_id = ? AND whatsapp_account = ? AND is_call_start = ?", orgID, req.WhatsAppAccount, true).
			Update("is_call_start", false)
	}

	// Generate TTS audio for greeting_text fields in the menu tree
	if req.Menu != nil {
		if a.TTS == nil {
			if menuHasGreetingText(req.Menu) {
				return r.SendErrorEnvelope(fasthttp.StatusBadRequest,
					"Text-to-speech is not configured on this server. Please upload audio files instead.", nil, "")
			}
		} else {
			if err := a.generateIVRAudio(req.Menu); err != nil {
				a.Log.Error("TTS generation failed", "error", err)
				return r.SendErrorEnvelope(fasthttp.StatusBadRequest,
					"Text-to-speech generation failed: "+err.Error(), nil, "")
			}
		}
	}

	flow := models.IVRFlow{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  orgID,
		WhatsAppAccount: req.WhatsAppAccount,
		Name:            req.Name,
		Description:     req.Description,
		IsActive:        req.IsActive,
		IsCallStart:     req.IsCallStart,
		Menu:            req.Menu,
		WelcomeAudioURL: req.WelcomeAudioURL,
	}

	if err := a.DB.Create(&flow).Error; err != nil {
		a.Log.Error("Failed to create IVR flow", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to create IVR flow", nil, "")
	}

	return r.SendEnvelope(flow)
}

// UpdateIVRFlow updates an existing IVR flow
func (a *App) UpdateIVRFlow(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}
	if err := a.requirePermission(r, userID, models.ResourceIVRFlows, models.ActionWrite); err != nil {
		return nil
	}

	flowID, err := parsePathUUID(r, "id", "IVR flow")
	if err != nil {
		return nil
	}

	flow, err := findByIDAndOrg[models.IVRFlow](a.DB, r, flowID, orgID, "IVR Flow")
	if err != nil {
		return nil
	}

	var req IVRFlowRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}

	// If marking this as call start, unset others for the same account
	if req.IsCallStart && !flow.IsCallStart {
		a.DB.Model(&models.IVRFlow{}).
			Where("organization_id = ? AND whatsapp_account = ? AND is_call_start = ? AND id != ?",
				orgID, flow.WhatsAppAccount, true, flowID).
			Update("is_call_start", false)
	}

	// Generate TTS audio for greeting_text fields in the menu tree
	if req.Menu != nil {
		if a.TTS == nil {
			if menuHasGreetingText(req.Menu) {
				return r.SendErrorEnvelope(fasthttp.StatusBadRequest,
					"Text-to-speech is not configured on this server. Please upload audio files instead.", nil, "")
			}
		} else {
			if err := a.generateIVRAudio(req.Menu); err != nil {
				a.Log.Error("TTS generation failed", "error", err)
				return r.SendErrorEnvelope(fasthttp.StatusBadRequest,
					"Text-to-speech generation failed: "+err.Error(), nil, "")
			}
		}
	}

	// Only update fields that were actually provided (non-zero) to support
	// partial updates like toggling is_active without wiping the menu.
	updates := map[string]any{
		"is_active":    req.IsActive,
		"is_call_start": req.IsCallStart,
	}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Description != "" || req.Name != "" {
		// Include description when saving from the editor (name is always sent)
		updates["description"] = req.Description
	}
	if req.Menu != nil {
		updates["menu"] = req.Menu
	}
	if req.WelcomeAudioURL != "" {
		updates["welcome_audio_url"] = req.WelcomeAudioURL
	}
	if req.WhatsAppAccount != "" {
		updates["whatsapp_account"] = req.WhatsAppAccount
	}

	if err := a.DB.Model(flow).Updates(updates).Error; err != nil {
		a.Log.Error("Failed to update IVR flow", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to update IVR flow", nil, "")
	}

	// Reload for response
	a.DB.First(flow, flowID)
	return r.SendEnvelope(flow)
}

// DeleteIVRFlow soft-deletes an IVR flow
func (a *App) DeleteIVRFlow(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}
	if err := a.requirePermission(r, userID, models.ResourceIVRFlows, models.ActionDelete); err != nil {
		return nil
	}

	flowID, err := parsePathUUID(r, "id", "IVR flow")
	if err != nil {
		return nil
	}

	flow, err := findByIDAndOrg[models.IVRFlow](a.DB, r, flowID, orgID, "IVR Flow")
	if err != nil {
		return nil
	}

	if err := a.DB.Delete(flow).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to delete IVR flow", nil, "")
	}

	return r.SendEnvelope(map[string]string{"message": "IVR flow deleted"})
}

// getAudioDir returns the configured audio directory path.
func (a *App) getAudioDir() string {
	dir := a.Config.Calling.AudioDir
	if dir == "" {
		dir = "./audio"
	}
	return dir
}

// UploadIVRAudio handles multipart audio file uploads for IVR greetings.
func (a *App) UploadIVRAudio(r *fastglue.Request) error {
	_, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}
	if err := a.requirePermission(r, userID, models.ResourceIVRFlows, models.ActionWrite); err != nil {
		return nil
	}

	// Parse multipart form
	contentType := string(r.RequestCtx.Request.Header.ContentType())
	a.Log.Debug("IVR audio upload", "content_type", contentType, "body_size", len(r.RequestCtx.Request.Body()))

	form, err := r.RequestCtx.MultipartForm()
	if err != nil {
		a.Log.Error("Multipart parse failed", "error", err, "content_type", contentType)
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid multipart form: "+err.Error(), nil, "")
	}

	files := form.File["file"]
	if len(files) == 0 {
		a.Log.Error("No file in multipart form", "form_keys", fmt.Sprintf("%v", form.Value))
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "No file provided", nil, "")
	}

	fileHeader := files[0]
	file, err := fileHeader.Open()
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Failed to open file", nil, "")
	}
	defer func() { _ = file.Close() }()

	// Read file content (limit to 5MB for IVR prompts)
	const maxAudioSize = 5 << 20 // 5MB
	data, err := io.ReadAll(io.LimitReader(file, maxAudioSize+1))
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to read file", nil, "")
	}
	if len(data) > maxAudioSize {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "File too large. Maximum size is 5MB", nil, "")
	}

	// Validate MIME type
	mimeType := fileHeader.Header.Get("Content-Type")
	allowedAudio := map[string]bool{
		"audio/ogg":             true,
		"audio/opus":            true,
		"audio/mpeg":            true,
		"audio/mp3":             true,
		"audio/aac":             true,
		"audio/mp4":             true,
		"audio/wav":             true,
		"audio/x-wav":           true,
		"audio/wave":            true,
		"audio/webm":            true,
		"audio/flac":            true,
		"audio/x-flac":          true,
		"audio/x-m4a":           true,
		"audio/m4a":             true,
		"application/ogg":       true,
		"application/octet-stream": true, // fallback for unknown audio
		"video/ogg":             true, // some browsers report .ogg as video/ogg
	}
	if !allowedAudio[mimeType] {
		a.Log.Error("Unsupported audio MIME type", "mime_type", mimeType, "filename", fileHeader.Filename)
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Unsupported audio type: "+mimeType, nil, "")
	}

	// Determine extension
	ext := getExtensionFromMimeType(mimeType)
	if ext == "" {
		ext = ".bin"
	}

	// Ensure audio directory exists
	audioDir := a.getAudioDir()
	if err := os.MkdirAll(audioDir, 0755); err != nil {
		a.Log.Error("Failed to create audio directory", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to create audio directory", nil, "")
	}

	// Generate filename: uuid + extension
	filename := uuid.New().String() + ext
	filePath := filepath.Join(audioDir, filename)

	// Save file
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		a.Log.Error("Failed to save audio file", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to save audio file", nil, "")
	}

	a.Log.Info("IVR audio uploaded", "filename", filename, "mime_type", mimeType, "size", len(data))

	return r.SendEnvelope(map[string]any{
		"filename":  filename,
		"mime_type": mimeType,
		"size":      len(data),
	})
}

// ServeIVRAudio serves audio files from the IVR audio directory.
func (a *App) ServeIVRAudio(r *fastglue.Request) error {
	_, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}
	if err := a.requirePermission(r, userID, models.ResourceIVRFlows, models.ActionRead); err != nil {
		return nil
	}

	filename := r.RequestCtx.UserValue("filename").(string)
	filename = sanitizeFilename(filename)

	// Security: prevent directory traversal and symlink attacks
	audioDir := a.getAudioDir()
	baseDir, err := filepath.Abs(audioDir)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Storage configuration error", nil, "")
	}
	fullPath, err := filepath.Abs(filepath.Join(baseDir, filename))
	if err != nil || !strings.HasPrefix(fullPath, baseDir+string(os.PathSeparator)) {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid file path", nil, "")
	}

	// Reject symlinks
	info, err := os.Lstat(fullPath)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "File not found", nil, "")
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid file path", nil, "")
	}

	// Read file
	data, err := os.ReadFile(fullPath)
	if err != nil {
		a.Log.Error("Failed to read audio file", "path", fullPath, "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to read file", nil, "")
	}

	// Determine content type from extension
	ext := strings.ToLower(filepath.Ext(filename))
	contentType := getMimeTypeFromExtension(ext)

	r.RequestCtx.Response.Header.Set("Content-Type", contentType)
	r.RequestCtx.Response.Header.Set("Cache-Control", "private, max-age=3600")
	r.RequestCtx.SetBody(data)

	return nil
}

// generateIVRAudio walks the IVR menu JSONB tree and generates TTS audio
// for any node with a non-empty "greeting_text" field. The generated audio
// filename is set as the node's "greeting" field.
func (a *App) generateIVRAudio(menu models.JSONB) error {
	return walkMenuTTS(menu, a.TTS.Generate)
}

// walkMenuTTS recursively walks a menu JSONB node and calls generate for each
// node with greeting_text set. It updates the greeting field in-place.
func walkMenuTTS(menu models.JSONB, generate func(string) (string, error)) error {
	greetingText, _ := menu["greeting_text"].(string)
	if greetingText != "" {
		filename, err := generate(greetingText)
		if err != nil {
			return err
		}
		menu["greeting"] = filename
	}

	// Recurse into options → submenu
	opts, _ := menu["options"].(map[string]interface{})
	if opts == nil {
		// Handle case where JSONB was deserialized via json.Unmarshal
		if raw, ok := menu["options"]; ok {
			if b, err := json.Marshal(raw); err == nil {
				var parsed map[string]interface{}
				if json.Unmarshal(b, &parsed) == nil {
					opts = parsed
				}
			}
		}
	}

	for _, optRaw := range opts {
		opt, ok := optRaw.(map[string]interface{})
		if !ok {
			continue
		}
		subRaw, ok := opt["menu"]
		if !ok {
			continue
		}
		sub, ok := subRaw.(map[string]interface{})
		if !ok {
			continue
		}
		if err := walkMenuTTS(sub, generate); err != nil {
			return err
		}
	}

	return nil
}

// menuHasGreetingText recursively checks if any node in the menu tree uses greeting_text.
func menuHasGreetingText(menu models.JSONB) bool {
	if text, _ := menu["greeting_text"].(string); text != "" {
		return true
	}

	opts, _ := menu["options"].(map[string]interface{})
	for _, optRaw := range opts {
		opt, ok := optRaw.(map[string]interface{})
		if !ok {
			continue
		}
		sub, ok := opt["menu"].(map[string]interface{})
		if !ok {
			continue
		}
		if menuHasGreetingText(sub) {
			return true
		}
	}
	return false
}
