package handlers

import (
	"net/url"

	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

// TagRequest represents the request body for creating/updating a tag
type TagRequest struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

// TagResponse represents the API response for a tag
type TagResponse struct {
	Name      string `json:"name"`
	Color     string `json:"color"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// ListTags returns all tags for the organization
func (a *App) ListTags(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	if err := a.requirePermission(r, userID, models.ResourceTags, models.ActionRead); err != nil {
		return nil
	}

	pg := parsePagination(r)

	tags, err := a.getTagsCached(orgID)
	if err != nil {
		a.Log.Error("Failed to list tags", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to list tags", nil, "")
	}

	total := len(tags)

	// Apply pagination
	start := pg.Offset
	end := pg.Offset + pg.Limit
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	result := make([]TagResponse, 0, end-start)
	for i := start; i < end; i++ {
		result = append(result, tagToResponse(tags[i]))
	}

	return r.SendEnvelope(map[string]any{
		"tags":  result,
		"total": total,
		"page":  pg.Page,
		"limit": pg.Limit,
	})
}

// CreateTag creates a new tag
func (a *App) CreateTag(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	if err := a.requirePermission(r, userID, models.ResourceTags, models.ActionWrite); err != nil {
		return nil
	}

	var req TagRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}

	if req.Name == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "name is required", nil, "")
	}

	if len(req.Name) > 50 {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "name must be at most 50 characters", nil, "")
	}

	if !models.IsValidTagColor(req.Color) {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "invalid color. Valid colors: blue, red, green, yellow, purple, gray", nil, "")
	}

	// Check for duplicate name
	var existing models.Tag
	if err := a.DB.Where("organization_id = ? AND name = ?", orgID, req.Name).First(&existing).Error; err == nil {
		return r.SendErrorEnvelope(fasthttp.StatusConflict, "Tag with this name already exists", nil, "")
	}

	tag := models.Tag{
		OrganizationID: orgID,
		Name:           req.Name,
		Color:          req.Color,
	}

	if err := a.DB.Create(&tag).Error; err != nil {
		a.Log.Error("Failed to create tag", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to create tag", nil, "")
	}

	// Invalidate cache
	a.InvalidateTagsCache(orgID)

	return r.SendEnvelope(tagToResponse(tag))
}

// UpdateTag updates an existing tag
func (a *App) UpdateTag(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	if err := a.requirePermission(r, userID, models.ResourceTags, models.ActionWrite); err != nil {
		return nil
	}

	// Get tag name from path (URL-encoded)
	tagNameEncoded := r.RequestCtx.UserValue("name").(string)
	tagName, err := url.PathUnescape(tagNameEncoded)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid tag name", nil, "")
	}

	var tag models.Tag
	if err := a.DB.Where("organization_id = ? AND name = ?", orgID, tagName).First(&tag).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Tag not found", nil, "")
	}

	var req TagRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}

	// Validate color if provided
	if req.Color != "" && !models.IsValidTagColor(req.Color) {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "invalid color. Valid colors: blue, red, green, yellow, purple, gray", nil, "")
	}

	// Check if renaming and new name already exists
	if req.Name != "" && req.Name != tag.Name {
		if len(req.Name) > 50 {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "name must be at most 50 characters", nil, "")
		}
		var existing models.Tag
		if err := a.DB.Where("organization_id = ? AND name = ?", orgID, req.Name).First(&existing).Error; err == nil {
			return r.SendErrorEnvelope(fasthttp.StatusConflict, "Tag with this name already exists", nil, "")
		}
	}

	// If renaming, we need to delete old and create new (composite primary key)
	if req.Name != "" && req.Name != tag.Name {
		// Update contacts that use this tag
		// Note: Tags are stored as JSONB array of strings in contacts
		// This requires a raw SQL update
		if err := a.DB.Exec(`
			UPDATE contacts
			SET tags = (
				SELECT jsonb_agg(
					CASE WHEN elem::text = ? THEN ?::jsonb ELSE elem END
				)
				FROM jsonb_array_elements(COALESCE(tags, '[]'::jsonb)) elem
			)
			WHERE organization_id = ?
			AND tags @> ?::jsonb
		`, `"`+tagName+`"`, `"`+req.Name+`"`, orgID, `["`+tagName+`"]`).Error; err != nil {
			a.Log.Error("Failed to update contacts with renamed tag", "error", err)
			// Continue anyway - tag rename will still work
		}

		// Delete old tag
		if err := a.DB.Delete(&tag).Error; err != nil {
			a.Log.Error("Failed to delete old tag", "error", err)
			return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to update tag", nil, "")
		}

		// Create new tag
		newTag := models.Tag{
			OrganizationID: orgID,
			Name:           req.Name,
			Color:          req.Color,
		}
		if newTag.Color == "" {
			newTag.Color = tag.Color
		}

		if err := a.DB.Create(&newTag).Error; err != nil {
			a.Log.Error("Failed to create renamed tag", "error", err)
			return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to update tag", nil, "")
		}

		// Invalidate cache
		a.InvalidateTagsCache(orgID)

		return r.SendEnvelope(tagToResponse(newTag))
	}

	// Just updating color - use Updates for composite primary key
	if req.Color != "" && req.Color != tag.Color {
		if err := a.DB.Model(&models.Tag{}).
			Where("organization_id = ? AND name = ?", orgID, tagName).
			Update("color", req.Color).Error; err != nil {
			a.Log.Error("Failed to update tag", "error", err)
			return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to update tag", nil, "")
		}
		tag.Color = req.Color

		// Invalidate cache
		a.InvalidateTagsCache(orgID)
	}

	// Reload tag to get updated timestamp
	if err := a.DB.Where("organization_id = ? AND name = ?", orgID, tagName).First(&tag).Error; err != nil {
		a.Log.Error("Failed to reload tag", "error", err)
	}

	return r.SendEnvelope(tagToResponse(tag))
}

// DeleteTag deletes a tag
func (a *App) DeleteTag(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	if err := a.requirePermission(r, userID, models.ResourceTags, models.ActionDelete); err != nil {
		return nil
	}

	// Get tag name from path (URL-encoded)
	tagNameEncoded := r.RequestCtx.UserValue("name").(string)
	tagName, err := url.PathUnescape(tagNameEncoded)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid tag name", nil, "")
	}

	var tag models.Tag
	if err := a.DB.Where("organization_id = ? AND name = ?", orgID, tagName).First(&tag).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Tag not found", nil, "")
	}

	// Remove tag from all contacts that have it
	if err := a.DB.Exec(`
		UPDATE contacts
		SET tags = (
			SELECT COALESCE(jsonb_agg(elem), '[]'::jsonb)
			FROM jsonb_array_elements(COALESCE(tags, '[]'::jsonb)) elem
			WHERE elem::text != ?
		)
		WHERE organization_id = ?
		AND tags @> ?::jsonb
	`, `"`+tagName+`"`, orgID, `["`+tagName+`"]`).Error; err != nil {
		a.Log.Error("Failed to remove tag from contacts", "error", err)
		// Continue anyway - tag deletion will still work
	}

	if err := a.DB.Delete(&tag).Error; err != nil {
		a.Log.Error("Failed to delete tag", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to delete tag", nil, "")
	}

	// Invalidate cache
	a.InvalidateTagsCache(orgID)

	return r.SendEnvelope(map[string]string{"message": "Tag deleted"})
}

func tagToResponse(tag models.Tag) TagResponse {
	return TagResponse{
		Name:      tag.Name,
		Color:     tag.Color,
		CreatedAt: tag.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt: tag.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}
