package handlers

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/audit"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
	"gorm.io/gorm"
)

// CannedResponseButton mirrors the chatbot flow ButtonConfig shape.
// type is one of "reply", "url", "phone", "voice_call". For voice_call,
// Title is the on-button label (Meta's display_text, 20-char cap applied at
// send time) and TTLMinutes is how long the button stays clickable (0 ⇒
// Meta default, 15 min).
type CannedResponseButton struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Type        string `json:"type,omitempty"`
	URL         string `json:"url,omitempty"`
	PhoneNumber string `json:"phone_number,omitempty"`
	TTLMinutes  int    `json:"ttl_minutes,omitempty"`
}

// CannedResponseRequest represents the request body for creating/updating a canned response
type CannedResponseRequest struct {
	Name     string                 `json:"name"`
	Shortcut string                 `json:"shortcut"`
	Content  string                 `json:"content"`
	Category string                 `json:"category"`
	IsActive bool                   `json:"is_active"`
	Buttons  []CannedResponseButton `json:"buttons"`
}

// CannedResponseResponse represents the API response for a canned response
type CannedResponseResponse struct {
	ID         uuid.UUID              `json:"id"`
	Name       string                 `json:"name"`
	Shortcut   string                 `json:"shortcut"`
	Content    string                 `json:"content"`
	Category   string                 `json:"category"`
	IsActive   bool                   `json:"is_active"`
	UsageCount int                    `json:"usage_count"`
	Buttons    []CannedResponseButton `json:"buttons"`
	CreatedAt  string                 `json:"created_at"`
	UpdatedAt  string                 `json:"updated_at"`
}

// ListCannedResponses returns all canned responses for the organization
func (a *App) ListCannedResponses(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	pg := parsePagination(r)

	// Optional filters
	category := string(r.RequestCtx.QueryArgs().Peek("category"))
	search := string(r.RequestCtx.QueryArgs().Peek("search"))
	activeOnly := string(r.RequestCtx.QueryArgs().Peek("active_only"))

	query := a.DB.Where("organization_id = ?", orgID)

	// By default show all, but allow filtering to active only (for chat picker)
	if activeOnly == "true" {
		query = query.Where("is_active = ?", true)
	}

	if category != "" {
		query = query.Where("category = ?", category)
	}
	if search != "" {
		searchPattern := "%" + search + "%"
		query = query.Where("name ILIKE ? OR content ILIKE ? OR shortcut ILIKE ?",
			searchPattern, searchPattern, searchPattern)
	}

	var total int64
	query.Model(&models.CannedResponse{}).Count(&total)

	var responses []models.CannedResponse
	if err := pg.Apply(query.Order("usage_count DESC, name ASC")).
		Find(&responses).Error; err != nil {
		a.Log.Error("Failed to list canned responses", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError,
			"Failed to list canned responses", nil, "")
	}

	result := make([]CannedResponseResponse, len(responses))
	for i, cr := range responses {
		result[i] = cannedResponseToResponse(cr)
	}

	return r.SendEnvelope(map[string]any{
		"canned_responses": result,
		"total":            total,
		"page":             pg.Page,
		"limit":            pg.Limit,
	})
}

// CreateCannedResponse creates a new canned response
func (a *App) CreateCannedResponse(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	var req CannedResponseRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}

	if req.Name == "" || req.Content == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest,
			"name and content are required", nil, "")
	}

	if err := validateCannedResponseButtons(req.Buttons); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, err.Error(), nil, "")
	}

	// Check for duplicate name
	var existing models.CannedResponse
	if err := a.DB.Where("organization_id = ? AND name = ?", orgID, req.Name).
		First(&existing).Error; err == nil {
		return r.SendErrorEnvelope(fasthttp.StatusConflict,
			"Canned response with this name already exists", nil, "")
	}

	cannedResponse := models.CannedResponse{
		OrganizationID: orgID,
		Name:           req.Name,
		Shortcut:       req.Shortcut,
		Content:        req.Content,
		Category:       req.Category,
		IsActive:       true,
		Buttons:        buttonsToJSONBArray(req.Buttons),
		CreatedByID:    userID,
	}

	if err := a.DB.Create(&cannedResponse).Error; err != nil {
		a.Log.Error("Failed to create canned response", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError,
			"Failed to create canned response", nil, "")
	}

	audit.LogAudit(a.DB, orgID, userID, audit.GetUserName(a.DB, userID),
		"canned_response", cannedResponse.ID, models.AuditActionCreated, nil, cannedResponseAuditSnapshot(&cannedResponse))

	return r.SendEnvelope(cannedResponseToResponse(cannedResponse))
}

// GetCannedResponse returns a single canned response
func (a *App) GetCannedResponse(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	id, err := parsePathUUID(r, "id", "canned response")
	if err != nil {
		return nil
	}

	var cannedResponse models.CannedResponse
	if err := a.DB.Where("id = ? AND organization_id = ?", id, orgID).
		First(&cannedResponse).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound,
			"Canned response not found", nil, "")
	}

	return r.SendEnvelope(cannedResponseToResponse(cannedResponse))
}

// UpdateCannedResponse updates an existing canned response
func (a *App) UpdateCannedResponse(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	id, err := parsePathUUID(r, "id", "canned response")
	if err != nil {
		return nil
	}

	var cannedResponse models.CannedResponse
	if err := a.DB.Where("id = ? AND organization_id = ?", id, orgID).
		First(&cannedResponse).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound,
			"Canned response not found", nil, "")
	}

	var req CannedResponseRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}

	if err := validateCannedResponseButtons(req.Buttons); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, err.Error(), nil, "")
	}

	oldSnap := cannedResponseAuditSnapshot(&cannedResponse)

	// Update fields
	if req.Name != "" {
		cannedResponse.Name = req.Name
	}
	cannedResponse.Shortcut = req.Shortcut
	if req.Content != "" {
		cannedResponse.Content = req.Content
	}
	cannedResponse.Category = req.Category
	cannedResponse.IsActive = req.IsActive
	cannedResponse.Buttons = buttonsToJSONBArray(req.Buttons)

	if err := a.DB.Save(&cannedResponse).Error; err != nil {
		a.Log.Error("Failed to update canned response", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError,
			"Failed to update canned response", nil, "")
	}

	audit.LogAudit(a.DB, orgID, userID, audit.GetUserName(a.DB, userID),
		"canned_response", cannedResponse.ID, models.AuditActionUpdated, oldSnap, cannedResponseAuditSnapshot(&cannedResponse))

	return r.SendEnvelope(cannedResponseToResponse(cannedResponse))
}

// DeleteCannedResponse deletes a canned response
func (a *App) DeleteCannedResponse(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	id, err := parsePathUUID(r, "id", "canned response")
	if err != nil {
		return nil
	}

	var cannedResponse models.CannedResponse
	if err := a.DB.Where("id = ? AND organization_id = ?", id, orgID).
		First(&cannedResponse).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound,
			"Canned response not found", nil, "")
	}

	if err := a.DB.Delete(&cannedResponse).Error; err != nil {
		a.Log.Error("Failed to delete canned response", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError,
			"Failed to delete canned response", nil, "")
	}

	audit.LogAudit(a.DB, orgID, userID, audit.GetUserName(a.DB, userID),
		"canned_response", cannedResponse.ID, models.AuditActionDeleted, cannedResponseAuditSnapshot(&cannedResponse), nil)

	return r.SendEnvelope(map[string]string{"message": "Canned response deleted"})
}

// IncrementCannedResponseUsage increments the usage counter
func (a *App) IncrementCannedResponseUsage(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	id, err := parsePathUUID(r, "id", "canned response")
	if err != nil {
		return nil
	}

	if err := a.DB.Model(&models.CannedResponse{}).
		Where("id = ? AND organization_id = ?", id, orgID).
		UpdateColumn("usage_count", gorm.Expr("usage_count + 1")).Error; err != nil {
		a.Log.Error("Failed to update usage", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError,
			"Failed to update usage", nil, "")
	}

	return r.SendEnvelope(map[string]string{"message": "Usage incremented"})
}

// cannedResponseAuditSnapshot returns a diff-friendly representation of a
// canned response for audit logging. Noisy fields (usage_count, timestamps) are
// intentionally excluded so the activity log reflects user edits only.
//
// Note: the buttons array is serialised under "button_config" because the
// shared audit "buttons" field is on the global skipFields list (chatbot flow
// step buttons are noisy on every edit). Stringifying gives a readable
// before/after in the activity log.
func cannedResponseAuditSnapshot(cr *models.CannedResponse) map[string]any {
	if cr == nil {
		return nil
	}
	return map[string]any{
		"name":          cr.Name,
		"shortcut":      cr.Shortcut,
		"content":       cr.Content,
		"category":      cr.Category,
		"is_active":     cr.IsActive,
		"button_config": buttonsToAuditString(cr.Buttons),
	}
}

func cannedResponseToResponse(cr models.CannedResponse) CannedResponseResponse {
	return CannedResponseResponse{
		ID:         cr.ID,
		Name:       cr.Name,
		Shortcut:   cr.Shortcut,
		Content:    cr.Content,
		Category:   cr.Category,
		IsActive:   cr.IsActive,
		UsageCount: cr.UsageCount,
		Buttons:    jsonbArrayToButtons(cr.Buttons),
		CreatedAt:  cr.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:  cr.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

// buttonsToJSONBArray converts the typed request shape into the JSONBArray
// column. We round-trip through JSON so the stored shape matches what the
// chatbot flow steps use (and what the frontend / whatsapp client expect).
func buttonsToJSONBArray(buttons []CannedResponseButton) models.JSONBArray {
	if len(buttons) == 0 {
		return models.JSONBArray{}
	}
	arr := make(models.JSONBArray, 0, len(buttons))
	for _, b := range buttons {
		raw, err := json.Marshal(b)
		if err != nil {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			continue
		}
		arr = append(arr, m)
	}
	return arr
}

func jsonbArrayToButtons(arr models.JSONBArray) []CannedResponseButton {
	out := make([]CannedResponseButton, 0, len(arr))
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		raw, err := json.Marshal(m)
		if err != nil {
			continue
		}
		var b CannedResponseButton
		if err := json.Unmarshal(raw, &b); err != nil {
			continue
		}
		out = append(out, b)
	}
	return out
}

// buttonsToAuditString renders the buttons array as a compact comparable
// string (e.g. "Yes [reply], Open (https://x.com) [url]") so the audit diff
// records a single readable change rather than a deep JSON blob.
func buttonsToAuditString(arr models.JSONBArray) string {
	buttons := jsonbArrayToButtons(arr)
	if len(buttons) == 0 {
		return ""
	}
	parts := make([]string, 0, len(buttons))
	for _, b := range buttons {
		t := b.Type
		if t == "" {
			t = "reply"
		}
		switch t {
		case "url":
			parts = append(parts, b.Title+" ("+b.URL+") [url]")
		case "phone":
			parts = append(parts, b.Title+" ("+b.PhoneNumber+") [phone]")
		case "voice_call":
			label := b.Title + " [voice_call"
			if b.TTLMinutes > 0 {
				label += ", " + strconv.Itoa(b.TTLMinutes) + "m"
			}
			parts = append(parts, label+"]")
		default:
			parts = append(parts, b.Title+" [reply]")
		}
	}
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += ", "
		}
		out += p
	}
	return out
}

// validateCannedResponseButtons enforces the combo rules WhatsApp Cloud API
// imposes on free-form interactive messages. We block at save time so the
// agent gets a clear error instead of a silent fallback to plain text at
// send time. Frontend mirrors these checks in
// CannedResponseDetailView.vue:buttonsValidationError; keep them in sync.
//
//   - voice_call is interactive.type:"voice_call" — Meta does not allow it to
//     coexist with reply / url / phone buttons in a single send, and only
//     one voice_call button per message.
//   - voice_call needs a non-empty title (becomes Meta's display_text) and a
//     ttl_minutes in [0, 60]; 0 means "use Meta's default" (15 min).
//
// Other combo rules (no phone, max 1 url, no reply+url mix, max 10 reply)
// are enforced on the frontend today and left there for now since the
// existing send path falls back gracefully to text.
func validateCannedResponseButtons(buttons []CannedResponseButton) error {
	if len(buttons) == 0 {
		return nil
	}
	voiceCalls := 0
	others := 0
	for _, b := range buttons {
		if strings.ToLower(b.Type) == "voice_call" {
			voiceCalls++
			if strings.TrimSpace(b.Title) == "" {
				return fmt.Errorf("voice_call button needs a title")
			}
			if b.TTLMinutes < 0 || b.TTLMinutes > 60 {
				return fmt.Errorf("voice_call ttl_minutes must be between 0 and 60")
			}
			continue
		}
		others++
	}
	if voiceCalls > 1 {
		return fmt.Errorf("only one voice_call button is allowed per message")
	}
	if voiceCalls > 0 && others > 0 {
		return fmt.Errorf("voice_call cannot be combined with other button types")
	}
	return nil
}
