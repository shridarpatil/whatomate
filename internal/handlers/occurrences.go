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
	// req.AssignedUserID != nil is what says the field participated in the
	// request at all; resolveAssignee alone can't tell "absent" from "sent
	// empty" apart, since both come back (nil, nil). A body that omits the
	// field defaults the case to its creator — no occurrence is born orphaned.
	// A body that explicitly sends "" still clears it, same as UpdateOccurrence.
	if req.AssignedUserID == nil {
		occ.AssignedUserID = &userID
	} else {
		assigneeID, err := a.resolveAssignee(orgID, req.AssignedUserID)
		if err != nil {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, err.Error(), nil, "")
		}
		occ.AssignedUserID = assigneeID
	}
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

	// O quadro pede os casos fechados a partir de um corte que o cliente
	// calcula e envia absoluto. Ao contrário dos filtros de audit_logs, um
	// valor ilegível é recusado em vez de ignorado: descartá-lo em silêncio
	// transformaria a coluna "fechadas recentemente" na lista inteira.
	if v := string(r.RequestCtx.QueryArgs().Peek("closed_since")); v != "" {
		since, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest,
				"closed_since must be an RFC3339 timestamp", nil, "")
		}
		query = query.Where("occurrences.closed_at IS NOT NULL AND occurrences.closed_at >= ?", since)
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

// UpdateOccurrenceRequest is the body for editing a case's editable fields.
type UpdateOccurrenceRequest struct {
	Title          string  `json:"title"`
	Description    string  `json:"description"`
	Priority       string  `json:"priority"`
	AssignedUserID *string `json:"assigned_user_id"`
}

// ChangeStageRequest moves a case to another stage.
type ChangeStageRequest struct {
	StageID string `json:"stage_id"`
}

// OccurrenceEventRequest adds a manual note to the timeline.
type OccurrenceEventRequest struct {
	Content string `json:"content"`
}

// OccurrenceEventResponse is the API shape of a timeline entry.
type OccurrenceEventResponse struct {
	ID            uuid.UUID  `json:"id"`
	Type          string     `json:"type"`
	Content       string     `json:"content"`
	Metadata      any        `json:"metadata"`
	CreatedByID   *uuid.UUID `json:"created_by_id,omitempty"`
	CreatedByName string     `json:"created_by_name,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

// loadAuthorizedOccurrence resolves the occurrence from the path and applies the
// conversation gate. Every occurrence endpoint goes through it — the repetition
// is the point: in this codebase's four visibility leaks, the hole was always an
// endpoint someone forgot to gate.
//
// needInteract asks for write access; false checks view access only.
func (a *App) loadAuthorizedOccurrence(r *fastglue.Request, orgID, userID uuid.UUID, needInteract bool) (*models.Occurrence, error) {
	occurrenceID, err := parsePathUUID(r, "id", "occurrence")
	if err != nil {
		return nil, errEnvelopeSent
	}

	occ, err := findByIDAndOrg[models.Occurrence](a.DB, r, occurrenceID, orgID, "Occurrence")
	if err != nil {
		return nil, errEnvelopeSent
	}

	var contact models.Contact
	if err := a.DB.Where("id = ? AND organization_id = ?", occ.ContactID, orgID).
		First(&contact).Error; err != nil {
		_ = r.SendErrorEnvelope(fasthttp.StatusNotFound, "Contact not found", nil, "")
		return nil, errEnvelopeSent
	}

	// The assignee exception, mirroring visibleOccurrences.
	isAssignee := occ.AssignedUserID != nil && *occ.AssignedUserID == userID

	allowed := isAssignee
	if !allowed {
		if needInteract {
			allowed = a.canInteractWithConversation(userID, orgID, &contact)
		} else {
			allowed = a.canViewConversation(userID, orgID, &contact)
		}
	}
	if !allowed {
		_ = r.SendErrorEnvelope(fasthttp.StatusForbidden,
			"You do not have access to this occurrence", nil, "")
		return nil, errEnvelopeSent
	}

	occ.Contact = &contact
	return occ, nil
}

// GetOccurrence returns one case.
func (a *App) GetOccurrence(r *fastglue.Request) error {
	orgID, userID, err := a.requireAuth(r, models.ResourceChat, models.ActionRead)
	if err != nil {
		return nil
	}
	occ, err := a.loadAuthorizedOccurrence(r, orgID, userID, false)
	if err != nil {
		return nil
	}
	a.DB.Preload("Stage").Preload("AssignedUser").First(occ, occ.ID)
	return r.SendEnvelope(occurrenceToResponse(*occ))
}

// UpdateOccurrence edits title, description, priority and assignee.
func (a *App) UpdateOccurrence(r *fastglue.Request) error {
	orgID, userID, err := a.requireAuth(r, models.ResourceChat, models.ActionWrite)
	if err != nil {
		return nil
	}
	occ, err := a.loadAuthorizedOccurrence(r, orgID, userID, true)
	if err != nil {
		return nil
	}

	var req UpdateOccurrenceRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}
	if req.Title == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "title is required", nil, "")
	}

	updates := map[string]any{
		"title":       req.Title,
		"description": req.Description,
	}
	if req.Priority != "" {
		updates["priority"] = req.Priority
	}

	// req.AssignedUserID != nil is what says the field participated in the
	// request at all; resolveAssignee alone can't tell "absent" from "sent
	// empty" apart, since both come back (nil, nil). A PUT that omits the
	// field must leave the assignee untouched — only a PUT that explicitly
	// sends "" clears it.
	assigneeChanged := false
	if req.AssignedUserID != nil {
		resolved, err := a.resolveAssignee(orgID, req.AssignedUserID)
		if err != nil {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, err.Error(), nil, "")
		}
		updates["assigned_user_id"] = resolved
		switch {
		case occ.AssignedUserID == nil && resolved == nil:
			assigneeChanged = false
		case occ.AssignedUserID == nil || resolved == nil:
			assigneeChanged = true
		default:
			assigneeChanged = *occ.AssignedUserID != *resolved
		}
	}

	if err := a.DB.Model(occ).Updates(updates).Error; err != nil {
		a.Log.Error("Failed to update occurrence", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError,
			"Failed to update occurrence", nil, "")
	}

	if assigneeChanged {
		a.DB.Create(&models.OccurrenceEvent{
			OrganizationID: orgID,
			OccurrenceID:   occ.ID,
			Type:           models.OccurrenceEventAssignment,
			CreatedByID:    &userID,
		})
	}

	a.DB.Preload("Stage").Preload("AssignedUser").First(occ, occ.ID)
	return r.SendEnvelope(occurrenceToResponse(*occ))
}

// ChangeOccurrenceStage moves a case and records the transition. Entering a
// closing stage stamps closed_at; leaving one clears it — that is how reopening
// works, without a separate endpoint.
func (a *App) ChangeOccurrenceStage(r *fastglue.Request) error {
	orgID, userID, err := a.requireAuth(r, models.ResourceChat, models.ActionWrite)
	if err != nil {
		return nil
	}
	occ, err := a.loadAuthorizedOccurrence(r, orgID, userID, true)
	if err != nil {
		return nil
	}

	var req ChangeStageRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}
	stageID, err := uuid.Parse(req.StageID)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid stage_id", nil, "")
	}

	target, err := findByIDAndOrg[models.OccurrenceStage](a.DB, r, stageID, orgID, "Stage")
	if err != nil {
		return nil
	}

	// Resending the stage the occurrence is already in is a no-op, not a
	// transition: without this guard it would log a spurious "X → X"
	// stage_change event, and if the current stage closes cases, it would
	// restamp closed_at with now() even though the case was never reopened
	// (plus a duplicate "closed" event). A stage picker that resubmits on
	// blur can trigger this by itself.
	if target.ID == occ.StageID {
		occ.Stage = target
		return r.SendEnvelope(occurrenceToResponse(*occ))
	}

	var from models.OccurrenceStage
	a.DB.Where("id = ?", occ.StageID).First(&from)

	updates := map[string]any{"stage_id": target.ID}
	if target.IsClosing {
		now := time.Now()
		updates["closed_at"] = &now
	} else {
		updates["closed_at"] = nil
	}

	if err := a.DB.Model(occ).Updates(updates).Error; err != nil {
		a.Log.Error("Failed to change occurrence stage", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError,
			"Failed to change stage", nil, "")
	}

	eventType := models.OccurrenceEventStageChange
	a.DB.Create(&models.OccurrenceEvent{
		OrganizationID: orgID,
		OccurrenceID:   occ.ID,
		Type:           eventType,
		Content:        from.Name + " → " + target.Name,
		Metadata:       models.JSONB{"from_stage_id": from.ID.String(), "to_stage_id": target.ID.String()},
		CreatedByID:    &userID,
	})

	if target.IsClosing {
		a.DB.Create(&models.OccurrenceEvent{
			OrganizationID: orgID,
			OccurrenceID:   occ.ID,
			Type:           models.OccurrenceEventClosed,
			CreatedByID:    &userID,
		})
	}

	occ.Stage = target
	return r.SendEnvelope(occurrenceToResponse(*occ))
}

// ListOccurrenceEvents returns the timeline, oldest first.
func (a *App) ListOccurrenceEvents(r *fastglue.Request) error {
	orgID, userID, err := a.requireAuth(r, models.ResourceChat, models.ActionRead)
	if err != nil {
		return nil
	}
	occ, err := a.loadAuthorizedOccurrence(r, orgID, userID, false)
	if err != nil {
		return nil
	}

	var events []models.OccurrenceEvent
	if err := a.DB.Where("organization_id = ? AND occurrence_id = ?", orgID, occ.ID).
		Preload("CreatedBy").Order("created_at ASC").Find(&events).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError,
			"Failed to list events", nil, "")
	}

	result := make([]OccurrenceEventResponse, len(events))
	for i, e := range events {
		result[i] = OccurrenceEventResponse{
			ID:          e.ID,
			Type:        string(e.Type),
			Content:     e.Content,
			Metadata:    e.Metadata,
			CreatedByID: e.CreatedByID,
			CreatedAt:   e.CreatedAt,
		}
		if e.CreatedBy != nil {
			result[i].CreatedByName = e.CreatedBy.FullName
		}
	}

	return r.SendEnvelope(map[string]any{"events": result})
}

// CreateOccurrenceEvent adds a manual note to the timeline.
func (a *App) CreateOccurrenceEvent(r *fastglue.Request) error {
	orgID, userID, err := a.requireAuth(r, models.ResourceChat, models.ActionWrite)
	if err != nil {
		return nil
	}
	occ, err := a.loadAuthorizedOccurrence(r, orgID, userID, true)
	if err != nil {
		return nil
	}

	var req OccurrenceEventRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}
	if req.Content == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "content is required", nil, "")
	}

	event := models.OccurrenceEvent{
		OrganizationID: orgID,
		OccurrenceID:   occ.ID,
		Type:           models.OccurrenceEventNote,
		Content:        req.Content,
		CreatedByID:    &userID,
	}
	if err := a.DB.Create(&event).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError,
			"Failed to create event", nil, "")
	}

	return r.SendEnvelope(OccurrenceEventResponse{
		ID:          event.ID,
		Type:        string(event.Type),
		Content:     event.Content,
		CreatedByID: event.CreatedByID,
		CreatedAt:   event.CreatedAt,
	})
}
