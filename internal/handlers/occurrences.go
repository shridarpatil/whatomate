package handlers

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
	"gorm.io/gorm"
)

// CreateOccurrenceRequest is the body for opening a case.
type CreateOccurrenceRequest struct {
	ContactID        string  `json:"contact_id"`
	Title            string  `json:"title"`
	Description      string  `json:"description"`
	Priority         string  `json:"priority"`
	AssignedUserID   *string `json:"assigned_user_id"`
	SourceTransferID *string `json:"source_transfer_id"`
}

// OccurrenceResponse is the API shape of an occurrence.
type OccurrenceResponse struct {
	ID               uuid.UUID  `json:"id"`
	ProtocolNumber   string     `json:"protocol_number"`
	ContactID        uuid.UUID  `json:"contact_id"`
	ContactName      string     `json:"contact_name"`
	Title            string     `json:"title"`
	Description      string     `json:"description"`
	StageID          uuid.UUID  `json:"stage_id"`
	StageName        string     `json:"stage_name"`
	Priority         string     `json:"priority"`
	AssignedUserID   *uuid.UUID `json:"assigned_user_id,omitempty"`
	AssignedUserName string     `json:"assigned_user_name,omitempty"`
	OpenedAt         time.Time  `json:"opened_at"`
	ClosedAt         *time.Time `json:"closed_at,omitempty"`
	SourceTransferID *uuid.UUID `json:"source_transfer_id,omitempty"`
}

func occurrenceToResponse(o models.Occurrence) OccurrenceResponse {
	resp := OccurrenceResponse{
		ID:               o.ID,
		ProtocolNumber:   o.ProtocolNumber,
		ContactID:        o.ContactID,
		Title:            o.Title,
		Description:      o.Description,
		StageID:          o.StageID,
		Priority:         string(o.Priority),
		AssignedUserID:   o.AssignedUserID,
		OpenedAt:         o.OpenedAt,
		ClosedAt:         o.ClosedAt,
		SourceTransferID: o.SourceTransferID,
	}
	if o.Contact != nil {
		resp.ContactName = o.Contact.ProfileName
	}
	if o.Stage != nil {
		resp.StageName = o.Stage.Name
	}
	if o.AssignedUser != nil {
		resp.AssignedUserName = o.AssignedUser.FullName
	}
	return resp
}

// visibleOccurrences scopes a query on the occurrences table to what the user
// may see.
//
// Subquery, NOT a join: scopeVisibleConversations writes `id`,
// `assigned_user_id` and `contacts.*` unqualified, so joined onto occurrences
// it would resolve `id` to occurrences.id and silently return wrong rows. This
// mirrors ListAgentTransfers and the chatbot session listing.
func (a *App) visibleOccurrences(query *gorm.DB, userID, orgID uuid.UUID) *gorm.DB {
	visibleContacts := a.scopeVisibleConversations(
		a.DB.Model(&models.Contact{}).Where("organization_id = ?", orgID).Select("id"),
		userID, orgID)

	// The OR is the assignee exception: a case assigned to you is always yours
	// to open, even when the contact itself is outside your scope.
	return query.Where("occurrences.contact_id IN (?) OR occurrences.assigned_user_id = ?",
		visibleContacts, userID)
}

// resolveAssignee validates that a user id from a request body names a real
// user in this organisation. Returns nil when the caller sent no assignee.
// Without this check an arbitrary UUID lands in assigned_user_id and the
// occurrence points at nobody, with no error surfaced to the caller.
func (a *App) resolveAssignee(orgID uuid.UUID, raw *string) (*uuid.UUID, error) {
	if raw == nil || *raw == "" {
		return nil, nil
	}
	id, err := uuid.Parse(*raw)
	if err != nil {
		return nil, errors.New("invalid assigned_user_id")
	}
	var user models.User
	if err := a.DB.Where("id = ? AND organization_id = ?", id, orgID).First(&user).Error; err != nil {
		return nil, errors.New("assigned_user_id does not belong to this organization")
	}
	return &id, nil
}

// CreateOccurrence opens a case and issues its protocol.
func (a *App) CreateOccurrence(r *fastglue.Request) error {
	orgID, userID, err := a.requireAuth(r, models.ResourceChat, models.ActionWrite)
	if err != nil {
		return nil
	}

	var req CreateOccurrenceRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}
	if req.Title == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "title is required", nil, "")
	}

	contactID, err := uuid.Parse(req.ContactID)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid contact_id", nil, "")
	}

	contact, err := findByIDAndOrg[models.Contact](a.DB, r, contactID, orgID, "Contact")
	if err != nil {
		return nil
	}

	// GATE 2: opening a case is interacting with the conversation.
	if !a.canInteractWithConversation(userID, orgID, contact) {
		return r.SendErrorEnvelope(fasthttp.StatusForbidden,
			"You do not have access to this conversation", nil, "")
	}

	stage, err := a.initialStage(orgID)
	if err != nil {
		a.Log.Error("Failed to resolve initial stage", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError,
			"Failed to resolve initial stage", nil, "")
	}

	priority := models.OccurrencePriority(req.Priority)
	if priority == "" {
		priority = models.OccurrencePriorityNormal
	}

	occ := models.Occurrence{
		OrganizationID: orgID,
		ContactID:      contactID,
		Title:          req.Title,
		Description:    req.Description,
		StageID:        stage.ID,
		Priority:       priority,
		OpenedByUserID: userID,
	}
	assigneeID, err := a.resolveAssignee(orgID, req.AssignedUserID)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, err.Error(), nil, "")
	}
	occ.AssignedUserID = assigneeID
	if req.SourceTransferID != nil && *req.SourceTransferID != "" {
		if id, err := uuid.Parse(*req.SourceTransferID); err == nil {
			occ.SourceTransferID = &id
		}
	}

	if err := a.insertOccurrenceWithProtocol(&occ); err != nil {
		a.Log.Error("Failed to create occurrence", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError,
			"Failed to create occurrence", nil, "")
	}

	occ.Stage = stage
	occ.Contact = contact
	return r.SendEnvelope(occurrenceToResponse(occ))
}

// ListOccurrences returns the cases the user may see, newest first.
func (a *App) ListOccurrences(r *fastglue.Request) error {
	orgID, userID, err := a.requireAuth(r, models.ResourceChat, models.ActionRead)
	if err != nil {
		return nil
	}

	pg := parsePaginationWithDefaults(r, 30, 100)

	query := a.DB.Model(&models.Occurrence{}).
		Where("occurrences.organization_id = ?", orgID)
	query = a.visibleOccurrences(query, userID, orgID)

	if stageID := string(r.RequestCtx.QueryArgs().Peek("stage_id")); stageID != "" {
		query = query.Where("occurrences.stage_id = ?", stageID)
	}
	if contactID := string(r.RequestCtx.QueryArgs().Peek("contact_id")); contactID != "" {
		query = query.Where("occurrences.contact_id = ?", contactID)
	}
	if protocol := string(r.RequestCtx.QueryArgs().Peek("protocol")); protocol != "" {
		query = query.Where("occurrences.protocol_number ILIKE ?", "%"+protocol+"%")
	}
	if open := string(r.RequestCtx.QueryArgs().Peek("open")); open == "true" {
		query = query.Where("occurrences.closed_at IS NULL")
	}

	var total int64
	query.Count(&total)

	var occurrences []models.Occurrence
	if err := query.
		Preload("Contact").Preload("Stage").Preload("AssignedUser").
		Order("occurrences.opened_at DESC").
		Limit(pg.Limit).Offset(pg.Offset).
		Find(&occurrences).Error; err != nil {
		a.Log.Error("Failed to list occurrences", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError,
			"Failed to list occurrences", nil, "")
	}

	result := make([]OccurrenceResponse, len(occurrences))
	for i, o := range occurrences {
		result[i] = occurrenceToResponse(o)
	}

	return r.SendEnvelope(map[string]any{
		"occurrences": result,
		"total":       total,
		"has_more":    len(occurrences) == pg.Limit,
	})
}

// ListContactOccurrences feeds the chat panel with one contact's cases.
func (a *App) ListContactOccurrences(r *fastglue.Request) error {
	orgID, userID, err := a.requireAuth(r, models.ResourceChat, models.ActionRead)
	if err != nil {
		return nil
	}

	contactID, err := parsePathUUID(r, "id", "contact")
	if err != nil {
		return nil
	}

	contact, err := findByIDAndOrg[models.Contact](a.DB, r, contactID, orgID, "Contact")
	if err != nil {
		return nil
	}
	if !a.canViewConversation(userID, orgID, contact) {
		return r.SendErrorEnvelope(fasthttp.StatusForbidden,
			"You do not have access to this conversation", nil, "")
	}

	var occurrences []models.Occurrence
	if err := a.DB.Where("organization_id = ? AND contact_id = ?", orgID, contactID).
		Preload("Stage").Preload("AssignedUser").
		Order("opened_at DESC").Find(&occurrences).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError,
			"Failed to list occurrences", nil, "")
	}

	result := make([]OccurrenceResponse, len(occurrences))
	for i, o := range occurrences {
		o.Contact = contact
		result[i] = occurrenceToResponse(o)
	}

	return r.SendEnvelope(map[string]any{"occurrences": result})
}
