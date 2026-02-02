package handlers

import (
	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
	"gorm.io/gorm"
)

// CannedResponseRequest represents the request body for creating/updating a canned response
type CannedResponseRequest struct {
	Name     string `json:"name"`
	Shortcut string `json:"shortcut"`
	Content  string `json:"content"`
	Category string `json:"category"`
	IsActive bool   `json:"is_active"`
}

// CannedResponseResponse represents the API response for a canned response
type CannedResponseResponse struct {
	ID         uuid.UUID `json:"id"`
	Name       string    `json:"name"`
	Shortcut   string    `json:"shortcut"`
	Content    string    `json:"content"`
	Category   string    `json:"category"`
	IsActive   bool      `json:"is_active"`
	UsageCount int       `json:"usage_count"`
	CreatedAt  string    `json:"created_at"`
	UpdatedAt  string    `json:"updated_at"`
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
		CreatedByID:    userID,
	}

	if err := a.DB.Create(&cannedResponse).Error; err != nil {
		a.Log.Error("Failed to create canned response", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError,
			"Failed to create canned response", nil, "")
	}

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

	var req CannedResponseRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}

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

	if err := a.DB.Save(&cannedResponse).Error; err != nil {
		a.Log.Error("Failed to update canned response", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError,
			"Failed to update canned response", nil, "")
	}

	return r.SendEnvelope(cannedResponseToResponse(cannedResponse))
}

// DeleteCannedResponse deletes a canned response
func (a *App) DeleteCannedResponse(r *fastglue.Request) error {
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

	if err := a.DB.Delete(&cannedResponse).Error; err != nil {
		a.Log.Error("Failed to delete canned response", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError,
			"Failed to delete canned response", nil, "")
	}

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
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError,
			"Failed to update usage", nil, "")
	}

	return r.SendEnvelope(map[string]string{"message": "Usage incremented"})
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
		CreatedAt:  cr.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:  cr.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}
