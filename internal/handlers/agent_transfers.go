package handlers

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/internal/websocket"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
	"gorm.io/gorm/clause"
)

// agentTransferRow represents a flat row result from the JOINed query
type agentTransferRow struct {
	// AgentTransfer fields
	ID                    uuid.UUID             `gorm:"column:id"`
	OrganizationID        uuid.UUID             `gorm:"column:organization_id"`
	ContactID             uuid.UUID             `gorm:"column:contact_id"`
	WhatsAppAccount       string                `gorm:"column:whatsapp_account"`
	PhoneNumber           string                `gorm:"column:phone_number"`
	Status                models.TransferStatus `gorm:"column:status"`
	Source                models.TransferSource `gorm:"column:source"`
	AgentID               *uuid.UUID `gorm:"column:agent_id"`
	TeamID                *uuid.UUID `gorm:"column:team_id"`
	TransferredByUserID   *uuid.UUID `gorm:"column:transferred_by_user_id"`
	Notes                 string     `gorm:"column:notes"`
	TransferredAt         time.Time  `gorm:"column:transferred_at"`
	ResumedAt             *time.Time `gorm:"column:resumed_at"`
	ResumedBy             *uuid.UUID `gorm:"column:resumed_by"`
	SLAResponseDeadline   *time.Time `gorm:"column:sla_response_deadline"`
	SLAResolutionDeadline *time.Time `gorm:"column:sla_resolution_deadline"`
	SLABreached           bool       `gorm:"column:sla_breached"`
	SLABreachedAt         *time.Time `gorm:"column:sla_breached_at"`
	EscalationLevel       int        `gorm:"column:escalation_level"`
	EscalatedAt           *time.Time `gorm:"column:escalated_at"`
	PickedUpAt            *time.Time `gorm:"column:picked_up_at"`
	ExpiresAt             *time.Time `gorm:"column:expires_at"`

	// Joined fields
	ContactName       *string `gorm:"column:contact_name"`
	AgentName         *string `gorm:"column:agent_name"`
	TeamName          *string `gorm:"column:team_name"`
	TransferredByName *string `gorm:"column:transferred_by_name"`
	ResumedByName     *string `gorm:"column:resumed_by_name"`
}

// CreateAgentTransferRequest represents the request to create an agent transfer
type CreateAgentTransferRequest struct {
	ContactID       string               `json:"contact_id"`
	WhatsAppAccount string               `json:"whatsapp_account"`
	AgentID         *string              `json:"agent_id"`
	TeamID          *string              `json:"team_id"` // Optional team queue
	Notes           string               `json:"notes"`
	Source          models.TransferSource `json:"source"` // manual, flow, keyword
}

// AssignTransferRequest represents the request to assign a transfer to an agent
type AssignTransferRequest struct {
	AgentID *string `json:"agent_id"` // null or empty string = unassign, UUID = assign to agent
	TeamID  *string `json:"team_id"`  // optional: move to different team queue
}

// AgentTransferResponse represents an agent transfer in API responses
type AgentTransferResponse struct {
	ID                string               `json:"id"`
	ContactID         string               `json:"contact_id"`
	ContactName       string               `json:"contact_name"`
	PhoneNumber       string               `json:"phone_number"`
	WhatsAppAccount   string               `json:"whatsapp_account"`
	Status            models.TransferStatus `json:"status"`
	Source            models.TransferSource `json:"source"`
	AgentID           *string              `json:"agent_id,omitempty"`
	AgentName         *string              `json:"agent_name,omitempty"`
	TeamID            *string              `json:"team_id,omitempty"`
	TeamName          *string              `json:"team_name,omitempty"`
	TransferredBy     *string              `json:"transferred_by,omitempty"`
	TransferredByName *string              `json:"transferred_by_name,omitempty"`
	Notes             string               `json:"notes"`
	TransferredAt     string               `json:"transferred_at"`
	ResumedAt         *string              `json:"resumed_at,omitempty"`
	ResumedBy         *string              `json:"resumed_by,omitempty"`
	ResumedByName     *string              `json:"resumed_by_name,omitempty"`

	// SLA fields
	SLAResponseDeadline   *string `json:"sla_response_deadline,omitempty"`
	SLAResolutionDeadline *string `json:"sla_resolution_deadline,omitempty"`
	SLABreached           bool    `json:"sla_breached"`
	SLABreachedAt         *string `json:"sla_breached_at,omitempty"`
	EscalationLevel       int     `json:"escalation_level"`
	EscalatedAt           *string `json:"escalated_at,omitempty"`
	PickedUpAt            *string `json:"picked_up_at,omitempty"`
	ExpiresAt             *string `json:"expires_at,omitempty"`
}

// ListAgentTransfers lists agent transfers for the organization
// Agents see only their assigned transfers + their team queues; Admin see all; Managers see their teams
func (a *App) ListAgentTransfers(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	// Check permissions - users with write permission have full access (like admin)
	hasFullAccess := a.HasPermission(userID, models.ResourceTransfers, models.ActionWrite, orgID)

	// Query params
	status := string(r.RequestCtx.QueryArgs().Peek("status"))
	teamIDStr := string(r.RequestCtx.QueryArgs().Peek("team_id"))

	// Pagination params
	limit := 100 // Default limit
	offset := 0
	if limitStr := string(r.RequestCtx.QueryArgs().Peek("limit")); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 && parsed <= 500 {
			limit = parsed
		}
	}
	if offsetStr := string(r.RequestCtx.QueryArgs().Peek("offset")); offsetStr != "" {
		if parsed, err := strconv.Atoi(offsetStr); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	// Lazy loading: parse include parameter for optional relations
	// Example: ?include=contact,agent,team or ?include=all (default: all)
	includeParam := string(r.RequestCtx.QueryArgs().Peek("include"))
	includeAll := includeParam == "" || includeParam == "all"
	includeSet := make(map[string]bool)
	if !includeAll {
		for _, inc := range strings.Split(includeParam, ",") {
			includeSet[strings.TrimSpace(inc)] = true
		}
	}

	// Build SELECT clause based on what relations are needed
	selectCols := []string{"agent_transfers.*"}
	if includeAll || includeSet["contact"] {
		selectCols = append(selectCols, "contacts.profile_name AS contact_name")
	}
	if includeAll || includeSet["agent"] {
		selectCols = append(selectCols, "agent.full_name AS agent_name")
	}
	if includeAll || includeSet["team"] {
		selectCols = append(selectCols, "teams.name AS team_name")
	}
	if includeAll || includeSet["transferred_by"] {
		selectCols = append(selectCols, "transferred_by.full_name AS transferred_by_name")
	}
	if includeAll || includeSet["resumed_by"] {
		selectCols = append(selectCols, "resumed_by.full_name AS resumed_by_name")
	}

	// Build query with conditional JOINs for better performance
	query := a.DB.Table("agent_transfers").
		Select(strings.Join(selectCols, ", ")).
		Where("agent_transfers.organization_id = ?", orgID).
		Order("agent_transfers.transferred_at ASC") // FIFO

	// Only add JOINs for requested relations (lazy loading)
	if includeAll || includeSet["contact"] {
		query = query.Joins("LEFT JOIN contacts ON contacts.id = agent_transfers.contact_id")
	}
	if includeAll || includeSet["agent"] {
		query = query.Joins("LEFT JOIN users AS agent ON agent.id = agent_transfers.agent_id")
	}
	if includeAll || includeSet["team"] {
		query = query.Joins("LEFT JOIN teams ON teams.id = agent_transfers.team_id")
	}
	if includeAll || includeSet["transferred_by"] {
		query = query.Joins("LEFT JOIN users AS transferred_by ON transferred_by.id = agent_transfers.transferred_by_user_id")
	}
	if includeAll || includeSet["resumed_by"] {
		query = query.Joins("LEFT JOIN users AS resumed_by ON resumed_by.id = agent_transfers.resumed_by")
	}

	// Filter by status if provided
	if status != "" {
		query = query.Where("agent_transfers.status = ?", status)
	}

	// Filter by team if provided
	if teamIDStr != "" {
		if teamIDStr == "general" {
			query = query.Where("agent_transfers.team_id IS NULL")
		} else {
			teamID, err := uuid.Parse(teamIDStr)
			if err == nil {
				query = query.Where("agent_transfers.team_id = ?", teamID)
			}
		}
	}

	// Get user's team memberships for filtering (needed for users without full access)
	var userTeamIDs []uuid.UUID
	if !hasFullAccess {
		var memberships []models.TeamMember
		if err := a.DB.Where("user_id = ?", userID).Find(&memberships).Error; err != nil {
			a.Log.Error("Failed to fetch team memberships", "error", err, "user_id", userID)
		}
		for _, m := range memberships {
			userTeamIDs = append(userTeamIDs, m.TeamID)
		}
	}

	// Filter based on permissions
	if !hasFullAccess {
		// Users without full access see their assigned transfers + unassigned in their team queues + general queue
		if len(userTeamIDs) > 0 {
			query = query.Where("agent_transfers.agent_id = ? OR (agent_transfers.agent_id IS NULL AND (agent_transfers.team_id IS NULL OR agent_transfers.team_id IN ?))", userID, userTeamIDs)
		} else {
			// User not in any team - see own transfers + general queue only
			query = query.Where("agent_transfers.agent_id = ? OR (agent_transfers.agent_id IS NULL AND agent_transfers.team_id IS NULL)", userID)
		}
	}
	// Users with full access see all transfers (no filter applied)

	// Get total count before pagination (for frontend to know if more exist)
	var totalCount int64
	countQuery := a.DB.Table("agent_transfers").Where("agent_transfers.organization_id = ?", orgID)
	if status != "" {
		countQuery = countQuery.Where("agent_transfers.status = ?", status)
	}
	if teamIDStr != "" {
		if teamIDStr == "general" {
			countQuery = countQuery.Where("agent_transfers.team_id IS NULL")
		} else if teamID, err := uuid.Parse(teamIDStr); err == nil {
			countQuery = countQuery.Where("agent_transfers.team_id = ?", teamID)
		}
	}
	if !hasFullAccess {
		if len(userTeamIDs) > 0 {
			countQuery = countQuery.Where("agent_transfers.agent_id = ? OR (agent_transfers.agent_id IS NULL AND (agent_transfers.team_id IS NULL OR agent_transfers.team_id IN ?))", userID, userTeamIDs)
		} else {
			countQuery = countQuery.Where("agent_transfers.agent_id = ? OR (agent_transfers.agent_id IS NULL AND agent_transfers.team_id IS NULL)", userID)
		}
	}
	countQuery.Count(&totalCount)

	// Apply pagination
	query = query.Limit(limit).Offset(offset)

	var transfers []agentTransferRow
	if err := query.Scan(&transfers).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to fetch transfers", nil, "")
	}

	// Get queue counts
	var generalQueueCount int64
	a.DB.Model(&models.AgentTransfer{}).
		Where("organization_id = ? AND status = ? AND agent_id IS NULL AND team_id IS NULL", orgID, models.TransferStatusActive).
		Count(&generalQueueCount)

	// Get team queue counts (filtered by user's teams for non-admin)
	type TeamQueueCount struct {
		TeamID uuid.UUID
		Count  int64
	}
	var teamQueueCounts []TeamQueueCount
	teamCountQuery := a.DB.Model(&models.AgentTransfer{}).
		Select("team_id, COUNT(*) as count").
		Where("organization_id = ? AND status = ? AND agent_id IS NULL AND team_id IS NOT NULL", orgID, models.TransferStatusActive)

	// Filter team counts by user's team membership for users without full access
	if !hasFullAccess && len(userTeamIDs) > 0 {
		teamCountQuery = teamCountQuery.Where("team_id IN ?", userTeamIDs)
	} else if !hasFullAccess && len(userTeamIDs) == 0 {
		// User is not in any team, don't show any team queue counts
		teamQueueCounts = []TeamQueueCount{}
	}

	if hasFullAccess || len(userTeamIDs) > 0 {
		teamCountQuery.Group("team_id").Scan(&teamQueueCounts)
	}

	// Build team counts map
	teamCounts := make(map[string]int64)
	for _, tc := range teamQueueCounts {
		teamCounts[tc.TeamID.String()] = tc.Count
	}

	a.Log.Info("ListAgentTransfers", "org_id", orgID, "has_full_access", hasFullAccess, "user_id", userID, "user_teams", userTeamIDs, "transfers_count", len(transfers), "general_queue", generalQueueCount, "team_queue_counts", teamCounts)

	// Check if phone masking is enabled
	shouldMask := a.ShouldMaskPhoneNumbers(orgID)

	// Build response from flat joined rows
	response := make([]AgentTransferResponse, len(transfers))
	for i, t := range transfers {
		phoneNumber := t.PhoneNumber
		if shouldMask {
			phoneNumber = MaskPhoneNumber(phoneNumber)
		}

		resp := AgentTransferResponse{
			ID:              t.ID.String(),
			ContactID:       t.ContactID.String(),
			PhoneNumber:     phoneNumber,
			WhatsAppAccount: t.WhatsAppAccount,
			Status:          t.Status,
			Source:          t.Source,
			Notes:           t.Notes,
			TransferredAt:   t.TransferredAt.Format(time.RFC3339),
		}

		if t.ContactName != nil {
			contactName := *t.ContactName
			if shouldMask {
				contactName = MaskIfPhoneNumber(contactName)
			}
			resp.ContactName = contactName
		}

		if t.AgentID != nil {
			agentIDStr := t.AgentID.String()
			resp.AgentID = &agentIDStr
			resp.AgentName = t.AgentName
		}

		if t.TransferredByUserID != nil {
			transferredBy := t.TransferredByUserID.String()
			resp.TransferredBy = &transferredBy
			resp.TransferredByName = t.TransferredByName
		}

		if t.TeamID != nil {
			teamIDStr := t.TeamID.String()
			resp.TeamID = &teamIDStr
			resp.TeamName = t.TeamName
		}

		if t.ResumedAt != nil {
			resumedAt := t.ResumedAt.Format(time.RFC3339)
			resp.ResumedAt = &resumedAt
		}

		if t.ResumedBy != nil {
			resumedBy := t.ResumedBy.String()
			resp.ResumedBy = &resumedBy
			resp.ResumedByName = t.ResumedByName
		}

		// SLA fields
		resp.SLABreached = t.SLABreached
		resp.EscalationLevel = t.EscalationLevel
		if t.SLAResponseDeadline != nil {
			deadline := t.SLAResponseDeadline.Format(time.RFC3339)
			resp.SLAResponseDeadline = &deadline
		}
		if t.SLAResolutionDeadline != nil {
			deadline := t.SLAResolutionDeadline.Format(time.RFC3339)
			resp.SLAResolutionDeadline = &deadline
		}
		if t.SLABreachedAt != nil {
			breachedAt := t.SLABreachedAt.Format(time.RFC3339)
			resp.SLABreachedAt = &breachedAt
		}
		if t.EscalatedAt != nil {
			escalatedAt := t.EscalatedAt.Format(time.RFC3339)
			resp.EscalatedAt = &escalatedAt
		}
		if t.PickedUpAt != nil {
			pickedUpAt := t.PickedUpAt.Format(time.RFC3339)
			resp.PickedUpAt = &pickedUpAt
		}
		if t.ExpiresAt != nil {
			expiresAt := t.ExpiresAt.Format(time.RFC3339)
			resp.ExpiresAt = &expiresAt
		}

		response[i] = resp
	}

	return r.SendEnvelope(map[string]any{
		"transfers":           response,
		"general_queue_count": generalQueueCount,
		"team_queue_counts":   teamCounts,
		"total_count":         totalCount,
		"limit":               limit,
		"offset":              offset,
	})
}

// CreateAgentTransfer creates a new agent transfer
func (a *App) CreateAgentTransfer(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	var req CreateAgentTransferRequest
	if err := json.Unmarshal(r.RequestCtx.PostBody(), &req); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid request body", nil, "")
	}

	if req.ContactID == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "contact_id is required", nil, "")
	}

	contactID, err := uuid.Parse(req.ContactID)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid contact_id", nil, "")
	}

	// Get contact
	contact, err := findByIDAndOrg[models.Contact](a.DB, r, contactID, orgID, "Contact")
	if err != nil {
		return nil
	}

	// Check for existing active transfer
	var existingCount int64
	a.DB.Model(&models.AgentTransfer{}).
		Where("organization_id = ? AND contact_id = ? AND status = ?", orgID, contactID, models.TransferStatusActive).
		Count(&existingCount)

	if existingCount > 0 {
		return r.SendErrorEnvelope(fasthttp.StatusConflict, "Contact already has an active transfer", nil, "")
	}

	// Get chatbot settings to check AssignToSameAgent (use cache)
	settings, _ := a.getChatbotSettingsCached(orgID, req.WhatsAppAccount)

	// Parse team_id if provided
	var teamID *uuid.UUID
	if req.TeamID != nil && *req.TeamID != "" {
		parsedTeamID, err := uuid.Parse(*req.TeamID)
		if err != nil {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid team_id", nil, "")
		}
		// Verify team exists and is active
		var team models.Team
		if err := a.DB.Where("id = ? AND organization_id = ? AND is_active = ?", parsedTeamID, orgID, true).First(&team).Error; err != nil {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Team not found or inactive", nil, "")
		}
		teamID = &parsedTeamID
	}

	// Determine agent assignment
	var agentID *uuid.UUID

	// First, try explicit agent from request
	if req.AgentID != nil && *req.AgentID != "" {
		parsedAgentID, err := uuid.Parse(*req.AgentID)
		if err != nil {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid agent_id", nil, "")
		}
		// Verify agent exists and is available
		agent, err := findByIDAndOrg[models.User](a.DB, r, parsedAgentID, orgID, "Agent")
		if err != nil {
			return nil
		}
		if !agent.IsAvailable {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Agent is currently away", nil, "")
		}
		agentID = &parsedAgentID
	} else if teamID != nil {
		// Apply team's assignment strategy
		agentID = a.assignToTeam(*teamID, orgID)
	} else if settings != nil && settings.AgentAssignment.AssignToSameAgent && contact.AssignedUserID != nil {
		// Auto-assign to contact's existing assigned agent (if setting enabled and agent is available)
		var assignedAgent models.User
		if a.DB.Where("id = ?", contact.AssignedUserID).First(&assignedAgent).Error == nil && assignedAgent.IsAvailable {
			agentID = contact.AssignedUserID
		}
		// If agent is not available, falls through to queue (agentID remains nil)
	}
	// Otherwise, agentID remains nil (goes to queue)

	// Determine source
	source := req.Source
	if source == "" {
		source = models.TransferSourceManual
	}

	// Create transfer
	transfer := models.AgentTransfer{
		BaseModel:           models.BaseModel{ID: uuid.New()},
		OrganizationID:      orgID,
		ContactID:           contactID,
		WhatsAppAccount:     req.WhatsAppAccount,
		PhoneNumber:         contact.PhoneNumber,
		Status:              models.TransferStatusActive,
		Source:              source,
		AgentID:             agentID,
		TeamID:              teamID,
		TransferredByUserID: &userID,
		Notes:               req.Notes,
		TransferredAt:       time.Now(),
	}

	// Set SLA deadlines if SLA is enabled
	if settings != nil {
		a.SetSLADeadlines(&transfer, settings)
	}

	// If agent is already assigned, mark as picked up
	if agentID != nil {
		a.UpdateSLAOnPickup(&transfer)
	}

	if err := a.DB.Create(&transfer).Error; err != nil {
		a.Log.Error("Failed to create agent transfer", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to create transfer", nil, "")
	}

	// Update contact assignment if agent assigned
	if agentID != nil {
		a.DB.Model(contact).Update("assigned_user_id", agentID)
	}

	// End any active chatbot session
	a.DB.Model(&models.ChatbotSession{}).
		Where("organization_id = ? AND contact_id = ? AND status = ?", orgID, contactID, models.SessionStatusActive).
		Updates(map[string]any{
			"status":       models.SessionStatusCancelled,
			"completed_at": time.Now(),
		})

	// Broadcast WebSocket notification
	a.broadcastTransferCreated(&transfer, contact)

	// Dispatch webhook for transfer created
	var agentIDStr *string
	var agentName *string
	if transfer.AgentID != nil {
		idStr := transfer.AgentID.String()
		agentIDStr = &idStr
	}
	a.DispatchWebhook(orgID, models.WebhookEventTransferCreated, TransferEventData{
		TransferID:      transfer.ID.String(),
		ContactID:       contact.ID.String(),
		ContactPhone:    contact.PhoneNumber,
		ContactName:     contact.ProfileName,
		Source:          transfer.Source,
		Reason:          transfer.Notes,
		AgentID:         agentIDStr,
		AgentName:       agentName,
		WhatsAppAccount: transfer.WhatsAppAccount,
	})

	// Load relations for response
	a.DB.Preload("Agent").Preload("Team").Preload("TransferredByUser").First(&transfer, transfer.ID)

	// Apply phone masking if enabled
	shouldMask := a.ShouldMaskPhoneNumbers(orgID)
	phoneNumber := transfer.PhoneNumber
	contactName := contact.ProfileName
	if shouldMask {
		phoneNumber = MaskPhoneNumber(phoneNumber)
		contactName = MaskIfPhoneNumber(contactName)
	}

	resp := AgentTransferResponse{
		ID:              transfer.ID.String(),
		ContactID:       transfer.ContactID.String(),
		ContactName:     contactName,
		PhoneNumber:     phoneNumber,
		WhatsAppAccount: transfer.WhatsAppAccount,
		Status:          transfer.Status,
		Source:          transfer.Source,
		Notes:           transfer.Notes,
		TransferredAt:   transfer.TransferredAt.Format(time.RFC3339),
	}

	if transfer.AgentID != nil {
		agentIDStr := transfer.AgentID.String()
		resp.AgentID = &agentIDStr
		if transfer.Agent != nil {
			resp.AgentName = &transfer.Agent.FullName
		}
	}

	if transfer.TeamID != nil {
		teamIDStr := transfer.TeamID.String()
		resp.TeamID = &teamIDStr
		if transfer.Team != nil {
			resp.TeamName = &transfer.Team.Name
		}
	}

	if transfer.TransferredByUserID != nil {
		transferredBy := transfer.TransferredByUserID.String()
		resp.TransferredBy = &transferredBy
		if transfer.TransferredByUser != nil {
			resp.TransferredByName = &transfer.TransferredByUser.FullName
		}
	}

	// SLA fields
	resp.SLABreached = transfer.SLA.Breached
	resp.EscalationLevel = transfer.SLA.EscalationLevel
	if transfer.SLA.ResponseDeadline != nil {
		deadline := transfer.SLA.ResponseDeadline.Format(time.RFC3339)
		resp.SLAResponseDeadline = &deadline
	}
	if transfer.SLA.ResolutionDeadline != nil {
		deadline := transfer.SLA.ResolutionDeadline.Format(time.RFC3339)
		resp.SLAResolutionDeadline = &deadline
	}
	if transfer.SLA.PickedUpAt != nil {
		pickedUpAt := transfer.SLA.PickedUpAt.Format(time.RFC3339)
		resp.PickedUpAt = &pickedUpAt
	}
	if transfer.SLA.ExpiresAt != nil {
		expiresAt := transfer.SLA.ExpiresAt.Format(time.RFC3339)
		resp.ExpiresAt = &expiresAt
	}

	return r.SendEnvelope(map[string]any{
		"transfer": resp,
		"message":  "Transfer created successfully",
	})
}

// ResumeFromTransfer resumes chatbot processing for a transferred contact
func (a *App) ResumeFromTransfer(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	transferID, err := parsePathUUID(r, "id", "transfer")
	if err != nil {
		return nil
	}

	transfer, err := findByIDAndOrg[models.AgentTransfer](a.DB, r, transferID, orgID, "Transfer")
	if err != nil {
		return nil
	}

	if transfer.Status != models.TransferStatusActive {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Transfer is not active", nil, "")
	}

	// Update transfer
	now := time.Now()
	transfer.Status = models.TransferStatusResumed
	transfer.ResumedAt = &now
	transfer.ResumedBy = &userID

	if err := a.DB.Save(transfer).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to resume transfer", nil, "")
	}

	// Clear chatbot tracking so client inactivity SLA doesn't trigger after transfer is closed
	a.ClearContactChatbotTracking(transfer.ContactID)

	// Get chatbot settings to check AssignToSameAgent (use cache)
	settings, _ := a.getChatbotSettingsCached(orgID, transfer.WhatsAppAccount)

	// If AssignToSameAgent is disabled, unassign the contact
	if settings != nil && !settings.AgentAssignment.AssignToSameAgent {
		a.DB.Model(&models.Contact{}).
			Where("id = ?", transfer.ContactID).
			Update("assigned_user_id", nil)
	}

	// Broadcast WebSocket notification
	a.broadcastTransferResumed(transfer)

	// Get contact for webhook data
	var contact models.Contact
	a.DB.Where("id = ?", transfer.ContactID).First(&contact)

	// Dispatch webhook for transfer resumed
	a.DispatchWebhook(orgID, models.WebhookEventTransferResumed, TransferEventData{
		TransferID:      transfer.ID.String(),
		ContactID:       contact.ID.String(),
		ContactPhone:    contact.PhoneNumber,
		ContactName:     contact.ProfileName,
		Source:          transfer.Source,
		WhatsAppAccount: transfer.WhatsAppAccount,
	})

	return r.SendEnvelope(map[string]any{
		"message": "Transfer resumed, chatbot is now active for this contact",
	})
}

// AssignAgentTransfer assigns a transfer to a specific agent
func (a *App) AssignAgentTransfer(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	// Check permissions - users with write permission can assign transfers to others
	hasWriteAccess := a.HasPermission(userID, models.ResourceTransfers, models.ActionWrite, orgID)

	transferID, err := parsePathUUID(r, "id", "transfer")
	if err != nil {
		return nil
	}

	var req AssignTransferRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}

	var transfer models.AgentTransfer
	if err := a.DB.Where("id = ? AND organization_id = ?", transferID, orgID).
		Preload("Contact").First(&transfer).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Transfer not found", nil, "")
	}

	if transfer.Status != models.TransferStatusActive {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Transfer is not active", nil, "")
	}

	// Determine target agent
	var targetAgentID *uuid.UUID

	if req.AgentID != nil && *req.AgentID != "" {
		// Explicit assignment - requires write permission
		if !hasWriteAccess {
			return r.SendErrorEnvelope(fasthttp.StatusForbidden, "You don't have permission to assign transfers to others", nil, "")
		}

		parsedAgentID, err := uuid.Parse(*req.AgentID)
		if err != nil {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid agent_id", nil, "")
		}

		// Verify agent exists and is available
		agent, err := findByIDAndOrg[models.User](a.DB, r, parsedAgentID, orgID, "Agent")
		if err != nil {
			return nil
		}
		if !agent.IsAvailable {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Agent is currently away", nil, "")
		}
		targetAgentID = &parsedAgentID
	} else if req.AgentID == nil && !hasWriteAccess {
		// User without write permission self-assigning (null means "assign to me")
		targetAgentID = &userID
	}

	// Handle team reassignment (requires write permission)
	if req.TeamID != nil {
		if !hasWriteAccess {
			return r.SendErrorEnvelope(fasthttp.StatusForbidden, "You don't have permission to change team assignment", nil, "")
		}

		if *req.TeamID == "" {
			// Move to general queue
			transfer.TeamID = nil
		} else {
			// Move to specific team
			parsedTeamID, err := uuid.Parse(*req.TeamID)
			if err != nil {
				return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid team_id", nil, "")
			}
			// Verify team exists
			var team models.Team
			if err := a.DB.Where("id = ? AND organization_id = ?", parsedTeamID, orgID).First(&team).Error; err != nil {
				return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Team not found", nil, "")
			}
			transfer.TeamID = &parsedTeamID
		}
	}

	// Update transfer
	transfer.AgentID = targetAgentID

	// Update SLA tracking if being assigned
	if targetAgentID != nil && transfer.SLA.PickedUpAt == nil {
		a.UpdateSLAOnPickup(&transfer)
	}

	if err := a.DB.Save(&transfer).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to assign transfer", nil, "")
	}

	// Update contact assignment
	if targetAgentID != nil && transfer.Contact != nil {
		a.DB.Model(transfer.Contact).Update("assigned_user_id", targetAgentID)
	} else if targetAgentID == nil && transfer.Contact != nil {
		// Clear assignment when unassigning
		a.DB.Model(transfer.Contact).Update("assigned_user_id", nil)
	}

	// Broadcast WebSocket notification
	a.broadcastTransferAssigned(&transfer)

	// Dispatch webhook for transfer assigned
	var agentIDStr *string
	var agentName *string
	if targetAgentID != nil {
		idStr := targetAgentID.String()
		agentIDStr = &idStr
		// Get agent name
		var agent models.User
		if a.DB.Where("id = ?", targetAgentID).First(&agent).Error == nil {
			agentName = &agent.FullName
		}
	}
	contactPhone := ""
	contactName := ""
	if transfer.Contact != nil {
		contactPhone = transfer.Contact.PhoneNumber
		contactName = transfer.Contact.ProfileName
	}
	a.DispatchWebhook(orgID, models.WebhookEventTransferAssigned, TransferEventData{
		TransferID:      transfer.ID.String(),
		ContactID:       transfer.ContactID.String(),
		ContactPhone:    contactPhone,
		ContactName:     contactName,
		Source:          transfer.Source,
		AgentID:         agentIDStr,
		AgentName:       agentName,
		WhatsAppAccount: transfer.WhatsAppAccount,
	})

	return r.SendEnvelope(map[string]any{
		"message":  "Transfer assigned successfully",
		"agent_id": targetAgentID,
	})
}

// PickNextTransfer allows an agent to pick the next unassigned transfer from the queue
func (a *App) PickNextTransfer(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	// Check permissions - users with write permission have full access
	hasFullAccess := a.HasPermission(userID, models.ResourceTransfers, models.ActionWrite, orgID)
	hasPickupPermission := a.HasPermission(userID, models.ResourceTransfers, models.ActionPickup, orgID)

	// Check if agent queue pickup is allowed (use cache)
	settings, _ := a.getChatbotSettingsCached(orgID, "")

	// Users without full access need pickup permission and AllowQueuePickup setting enabled
	if !hasFullAccess && settings != nil && !settings.AgentAssignment.AllowQueuePickup {
		return r.SendErrorEnvelope(fasthttp.StatusForbidden, "Queue pickup is not allowed", nil, "")
	}

	// Users without full access need pickup permission
	if !hasFullAccess && !hasPickupPermission {
		return r.SendErrorEnvelope(fasthttp.StatusForbidden, "You don't have permission to pick up transfers", nil, "")
	}

	// Get optional team filter
	teamIDStr := string(r.RequestCtx.QueryArgs().Peek("team_id"))

	// Get user's team memberships
	var userTeamIDs []uuid.UUID
	var memberships []models.TeamMember
	if err := a.DB.Where("user_id = ?", userID).Find(&memberships).Error; err != nil {
		a.Log.Error("Failed to fetch team memberships for pick", "error", err, "user_id", userID)
	}
	for _, m := range memberships {
		userTeamIDs = append(userTeamIDs, m.TeamID)
	}

	// Use transaction with FOR UPDATE lock to prevent race conditions
	tx := a.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Build query for picking transfer with row-level locking
	query := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
		Where("organization_id = ? AND status = ? AND agent_id IS NULL", orgID, models.TransferStatusActive).
		Order("transferred_at ASC")

	if teamIDStr != "" {
		// Pick from specific team
		if teamIDStr == "general" {
			query = query.Where("team_id IS NULL")
		} else {
			teamID, err := uuid.Parse(teamIDStr)
			if err == nil {
				// Verify user is member of this team (unless they have full access)
				if !hasFullAccess {
					found := false
					for _, tid := range userTeamIDs {
						if tid == teamID {
							found = true
							break
						}
					}
					if !found {
						tx.Rollback()
						return r.SendErrorEnvelope(fasthttp.StatusForbidden, "You are not a member of this team", nil, "")
					}
				}
				query = query.Where("team_id = ?", teamID)
			}
		}
	} else if !hasFullAccess {
		// Users without full access can only pick from their teams or general queue
		if len(userTeamIDs) > 0 {
			query = query.Where("team_id IS NULL OR team_id IN ?", userTeamIDs)
		} else {
			query = query.Where("team_id IS NULL")
		}
	}
	// Users with full access can pick from any queue if no team_id specified

	// Find oldest unassigned active transfer (FIFO) - locked row
	var transfer models.AgentTransfer
	result := query.First(&transfer)

	if result.Error != nil {
		tx.Rollback()
		return r.SendEnvelope(map[string]any{
			"message":  "No transfers in queue",
			"transfer": nil,
		})
	}

	// Assign to current user (self-pick)
	transfer.AgentID = &userID
	// If no one initiated the transfer, mark the picker as the one who initiated (self-pick)
	if transfer.TransferredByUserID == nil {
		transfer.TransferredByUserID = &userID
	}

	// Update SLA tracking for pickup
	a.UpdateSLAOnPickup(&transfer)

	if err := tx.Save(&transfer).Error; err != nil {
		tx.Rollback()
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to pick transfer", nil, "")
	}

	// Update contact assignment within transaction
	if err := tx.Model(&models.Contact{}).Where("id = ?", transfer.ContactID).Update("assigned_user_id", userID).Error; err != nil {
		tx.Rollback()
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to update contact assignment", nil, "")
	}

	// Commit the transaction
	if err := tx.Commit().Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to complete pickup", nil, "")
	}

	// Load related data for response (outside transaction)
	a.DB.Where("id = ?", transfer.ContactID).First(&transfer.Contact)
	if transfer.TeamID != nil {
		a.DB.Where("id = ?", transfer.TeamID).First(&transfer.Team)
	}

	// Load agent info
	var agent models.User
	a.DB.First(&agent, userID)

	// Broadcast WebSocket notification
	a.broadcastTransferAssigned(&transfer)

	// Apply phone masking if enabled
	shouldMask := a.ShouldMaskPhoneNumbers(orgID)
	phoneNumber := transfer.PhoneNumber
	if shouldMask {
		phoneNumber = MaskPhoneNumber(phoneNumber)
	}

	resp := AgentTransferResponse{
		ID:              transfer.ID.String(),
		ContactID:       transfer.ContactID.String(),
		PhoneNumber:     phoneNumber,
		WhatsAppAccount: transfer.WhatsAppAccount,
		Status:          transfer.Status,
		Source:          transfer.Source,
		Notes:           transfer.Notes,
		TransferredAt:   transfer.TransferredAt.Format(time.RFC3339),
	}

	if transfer.Contact != nil {
		contactName := transfer.Contact.ProfileName
		if shouldMask {
			contactName = MaskIfPhoneNumber(contactName)
		}
		resp.ContactName = contactName
	}

	agentIDStr := userID.String()
	resp.AgentID = &agentIDStr
	resp.AgentName = &agent.FullName

	if transfer.TeamID != nil {
		teamIDStr := transfer.TeamID.String()
		resp.TeamID = &teamIDStr
		if transfer.Team != nil {
			resp.TeamName = &transfer.Team.Name
		}
	}

	// Set TransferredBy (self-pick)
	if transfer.TransferredByUserID != nil {
		transferredBy := transfer.TransferredByUserID.String()
		resp.TransferredBy = &transferredBy
		resp.TransferredByName = &agent.FullName
	}

	// SLA fields
	resp.SLABreached = transfer.SLA.Breached
	resp.EscalationLevel = transfer.SLA.EscalationLevel
	if transfer.SLA.ResponseDeadline != nil {
		deadline := transfer.SLA.ResponseDeadline.Format(time.RFC3339)
		resp.SLAResponseDeadline = &deadline
	}
	if transfer.SLA.ResolutionDeadline != nil {
		deadline := transfer.SLA.ResolutionDeadline.Format(time.RFC3339)
		resp.SLAResolutionDeadline = &deadline
	}
	if transfer.SLA.BreachedAt != nil {
		breachedAt := transfer.SLA.BreachedAt.Format(time.RFC3339)
		resp.SLABreachedAt = &breachedAt
	}
	if transfer.SLA.PickedUpAt != nil {
		pickedUpAt := transfer.SLA.PickedUpAt.Format(time.RFC3339)
		resp.PickedUpAt = &pickedUpAt
	}
	if transfer.SLA.ExpiresAt != nil {
		expiresAt := transfer.SLA.ExpiresAt.Format(time.RFC3339)
		resp.ExpiresAt = &expiresAt
	}

	return r.SendEnvelope(map[string]any{
		"message":  "Transfer picked successfully",
		"transfer": resp,
	})
}

// hasActiveAgentTransfer checks if a contact has an active agent transfer
func (a *App) hasActiveAgentTransfer(orgID, contactID uuid.UUID) bool {
	var count int64
	a.DB.Model(&models.AgentTransfer{}).
		Where("organization_id = ? AND contact_id = ? AND status = ?", orgID, contactID, models.TransferStatusActive).
		Count(&count)
	return count > 0
}

// WebSocket broadcast helpers

func (a *App) broadcastTransferCreated(transfer *models.AgentTransfer, contact *models.Contact) {
	if a.WSHub == nil {
		return
	}

	payload := map[string]any{
		"id":               transfer.ID.String(),
		"contact_id":       transfer.ContactID.String(),
		"contact_name":     contact.ProfileName,
		"phone_number":     transfer.PhoneNumber,
		"whatsapp_account": transfer.WhatsAppAccount,
		"status":           transfer.Status,
		"source":           transfer.Source,
		"notes":            transfer.Notes,
		"transferred_at":   transfer.TransferredAt.Format(time.RFC3339),
	}

	if transfer.AgentID != nil {
		payload["agent_id"] = transfer.AgentID.String()
	}

	if transfer.TeamID != nil {
		payload["team_id"] = transfer.TeamID.String()
	}

	a.WSHub.BroadcastToOrg(transfer.OrganizationID, websocket.WSMessage{
		Type:    websocket.TypeAgentTransfer,
		Payload: payload,
	})
}

func (a *App) broadcastTransferResumed(transfer *models.AgentTransfer) {
	if a.WSHub == nil {
		return
	}

	payload := map[string]any{
		"id":         transfer.ID.String(),
		"contact_id": transfer.ContactID.String(),
		"status":     transfer.Status,
	}

	if transfer.ResumedAt != nil {
		payload["resumed_at"] = transfer.ResumedAt.Format(time.RFC3339)
	}
	if transfer.ResumedBy != nil {
		payload["resumed_by"] = transfer.ResumedBy.String()
	}

	a.WSHub.BroadcastToOrg(transfer.OrganizationID, websocket.WSMessage{
		Type:    websocket.TypeAgentTransferResume,
		Payload: payload,
	})
}

func (a *App) broadcastTransferAssigned(transfer *models.AgentTransfer) {
	if a.WSHub == nil {
		return
	}

	payload := map[string]any{
		"id":         transfer.ID.String(),
		"contact_id": transfer.ContactID.String(),
		"status":     transfer.Status,
	}

	if transfer.AgentID != nil {
		payload["agent_id"] = transfer.AgentID.String()
	} else {
		payload["agent_id"] = nil
	}

	if transfer.TeamID != nil {
		payload["team_id"] = transfer.TeamID.String()
	} else {
		payload["team_id"] = nil
	}

	a.WSHub.BroadcastToOrg(transfer.OrganizationID, websocket.WSMessage{
		Type:    websocket.TypeAgentTransferAssign,
		Payload: payload,
	})
}

// createTransferToQueue creates an unassigned agent transfer that goes to the queue
func (a *App) createTransferToQueue(account *models.WhatsAppAccount, contact *models.Contact, source models.TransferSource) {
	// Check for existing active transfer
	var existingCount int64
	a.DB.Model(&models.AgentTransfer{}).
		Where("organization_id = ? AND contact_id = ? AND status = ?", account.OrganizationID, contact.ID, models.TransferStatusActive).
		Count(&existingCount)

	if existingCount > 0 {
		a.Log.Debug("Contact already has active transfer, skipping", "contact_id", contact.ID, "source", source)
		return
	}

	// Get chatbot settings for SLA (use cache)
	settings, _ := a.getChatbotSettingsCached(account.OrganizationID, account.Name)

	// Create unassigned transfer (goes to queue)
	transfer := models.AgentTransfer{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  account.OrganizationID,
		ContactID:       contact.ID,
		WhatsAppAccount: account.Name,
		PhoneNumber:     contact.PhoneNumber,
		Status:          models.TransferStatusActive,
		Source:          source,
		AgentID:         nil, // Unassigned - goes to queue
		TransferredAt:   time.Now(),
	}

	// Set SLA deadlines
	if settings != nil {
		a.SetSLADeadlines(&transfer, settings)
	}

	if err := a.DB.Create(&transfer).Error; err != nil {
		a.Log.Error("Failed to create transfer to queue", "error", err, "contact_id", contact.ID, "source", string(source))
		return
	}

	a.Log.Info("Transfer created to agent queue", "transfer_id", transfer.ID, "contact_id", contact.ID, "source", source)

	// Broadcast to WebSocket
	a.broadcastTransferCreated(&transfer, contact)
}

// createTransferFromKeyword creates an agent transfer triggered by a keyword rule
func (a *App) createTransferFromKeyword(account *models.WhatsAppAccount, contact *models.Contact) {
	// Check for existing active transfer
	var existingCount int64
	a.DB.Model(&models.AgentTransfer{}).
		Where("organization_id = ? AND contact_id = ? AND status = ?", account.OrganizationID, contact.ID, models.TransferStatusActive).
		Count(&existingCount)

	if existingCount > 0 {
		a.Log.Info("Contact already has active transfer, skipping keyword transfer", "contact_id", contact.ID)
		return
	}

	// Get chatbot settings to check AssignToSameAgent and business hours (use cache)
	settings, _ := a.getChatbotSettingsCached(account.OrganizationID, account.Name)

	// Check business hours - if outside hours, send out of hours message instead of transfer
	if settings != nil && settings.BusinessHours.Enabled && len(settings.BusinessHours.Hours) > 0 {
		if !a.isWithinBusinessHours(settings.BusinessHours.Hours) {
			a.Log.Info("Outside business hours, sending out of hours message instead of transfer", "contact_id", contact.ID)
			if settings.BusinessHours.OutOfHoursMessage != "" {
				_ = a.sendAndSaveTextMessage(account, contact, settings.BusinessHours.OutOfHoursMessage)
			}
			return
		}
	}

	// Determine agent assignment
	var agentID *uuid.UUID
	if settings != nil && settings.AgentAssignment.AssignToSameAgent && contact.AssignedUserID != nil {
		// Check if the assigned agent is available
		var assignedAgent models.User
		if a.DB.Where("id = ?", contact.AssignedUserID).First(&assignedAgent).Error == nil && assignedAgent.IsAvailable {
			agentID = contact.AssignedUserID
		}
		// If agent is not available, falls through to queue (agentID remains nil)
	}

	// Create transfer
	transfer := models.AgentTransfer{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  account.OrganizationID,
		ContactID:       contact.ID,
		WhatsAppAccount: account.Name,
		PhoneNumber:     contact.PhoneNumber,
		Status:          models.TransferStatusActive,
		Source:          models.TransferSourceKeyword,
		AgentID:         agentID,
		TransferredAt:   time.Now(),
	}

	// Set SLA deadlines
	if settings != nil {
		a.SetSLADeadlines(&transfer, settings)
	}

	// If agent is already assigned, mark as picked up
	if agentID != nil {
		a.UpdateSLAOnPickup(&transfer)
	}

	if err := a.DB.Create(&transfer).Error; err != nil {
		a.Log.Error("Failed to create keyword-triggered transfer", "error", err, "contact_id", contact.ID)
		return
	}

	// Update contact assignment if agent assigned
	if agentID != nil {
		a.DB.Model(&contact).Update("assigned_user_id", agentID)
	}

	// End any active chatbot session
	a.DB.Model(&models.ChatbotSession{}).
		Where("organization_id = ? AND contact_id = ? AND status = ?", account.OrganizationID, contact.ID, models.SessionStatusActive).
		Updates(map[string]any{
			"status":       models.SessionStatusCancelled,
			"completed_at": time.Now(),
		})

	var agentIDStr string
	if agentID != nil {
		agentIDStr = agentID.String()
	}
	a.Log.Info("Agent transfer created from keyword rule",
		"transfer_id", transfer.ID,
		"contact_id", contact.ID,
		"agent_id", agentIDStr,
	)

	// Broadcast to WebSocket
	a.broadcastTransferCreated(&transfer, contact)
}

// assignToTeam applies the team's assignment strategy to select an agent
// Returns nil if manual strategy or no available agents
func (a *App) assignToTeam(teamID uuid.UUID, orgID uuid.UUID) *uuid.UUID {
	// Get team and its assignment strategy
	var team models.Team
	if err := a.DB.Where("id = ? AND organization_id = ? AND is_active = ?", teamID, orgID, true).First(&team).Error; err != nil {
		a.Log.Error("Failed to get team for assignment", "error", err, "team_id", teamID)
		return nil
	}

	switch team.AssignmentStrategy {
	case models.AssignmentStrategyRoundRobin:
		return a.assignToTeamRoundRobin(teamID, orgID)
	case models.AssignmentStrategyLoadBalanced:
		return a.assignToTeamLoadBalanced(teamID, orgID)
	case models.AssignmentStrategyManual:
		// Manual means no auto-assignment
		return nil
	default:
		// Default to round-robin
		return a.assignToTeamRoundRobin(teamID, orgID)
	}
}

// assignToTeamRoundRobin selects the next agent using round-robin
func (a *App) assignToTeamRoundRobin(teamID uuid.UUID, orgID uuid.UUID) *uuid.UUID {
	// Get team members who are available agents, ordered by last assigned time
	var members []models.TeamMember
	err := a.DB.
		Joins("JOIN users ON users.id = team_members.user_id").
		Where("team_members.team_id = ? AND team_members.role = ? AND users.is_available = ? AND users.is_active = ?",
			teamID, models.TeamRoleAgent, true, true).
		Order("team_members.last_assigned_at ASC NULLS FIRST").
		Find(&members).Error

	if err != nil || len(members) == 0 {
		a.Log.Debug("No available agents in team for round-robin", "team_id", teamID)
		return nil
	}

	// Pick the first agent (least recently assigned)
	selectedMember := members[0]

	// Update last_assigned_at
	now := time.Now()
	a.DB.Model(&selectedMember).Update("last_assigned_at", now)

	a.Log.Debug("Round-robin assigned to agent", "team_id", teamID, "user_id", selectedMember.UserID)
	return &selectedMember.UserID
}

// assignToTeamLoadBalanced selects the agent with fewest active transfers
func (a *App) assignToTeamLoadBalanced(teamID uuid.UUID, orgID uuid.UUID) *uuid.UUID {
	// Get team members who are available agents
	var members []models.TeamMember
	err := a.DB.
		Joins("JOIN users ON users.id = team_members.user_id").
		Where("team_members.team_id = ? AND team_members.role = ? AND users.is_available = ? AND users.is_active = ?",
			teamID, models.TeamRoleAgent, true, true).
		Find(&members).Error

	if err != nil || len(members) == 0 {
		a.Log.Debug("No available agents in team for load-balanced", "team_id", teamID)
		return nil
	}

	// Extract member user IDs
	memberIDs := make([]uuid.UUID, len(members))
	for i, m := range members {
		memberIDs[i] = m.UserID
	}

	// Count active transfers for all members in a single query (optimized from N+1)
	type AgentLoad struct {
		AgentID uuid.UUID `gorm:"column:agent_id"`
		Count   int64     `gorm:"column:count"`
	}
	var loads []AgentLoad
	a.DB.Model(&models.AgentTransfer{}).
		Select("agent_id, COUNT(*) as count").
		Where("organization_id = ? AND agent_id IN ? AND status = ?", orgID, memberIDs, models.TransferStatusActive).
		Group("agent_id").
		Scan(&loads)

	// Build a map of agent loads
	loadMap := make(map[uuid.UUID]int64)
	for _, l := range loads {
		loadMap[l.AgentID] = l.Count
	}

	// Find agent with lowest load (agents with 0 transfers won't be in loadMap)
	var lowestUserID *uuid.UUID
	var lowestCount int64 = -1
	for _, m := range members {
		count := loadMap[m.UserID] // Will be 0 if not in map (no active transfers)
		if lowestCount < 0 || count < lowestCount {
			lowestCount = count
			userID := m.UserID
			lowestUserID = &userID
		}
	}

	if lowestUserID == nil {
		return nil
	}

	a.Log.Debug("Load-balanced assigned to agent", "team_id", teamID, "user_id", *lowestUserID, "current_load", lowestCount)
	return lowestUserID
}

// createTransferToTeam creates an agent transfer to a specific team with appropriate assignment
func (a *App) createTransferToTeam(account *models.WhatsAppAccount, contact *models.Contact, teamID uuid.UUID, notes string, source models.TransferSource) {
	// Check for existing active transfer
	var existingCount int64
	a.DB.Model(&models.AgentTransfer{}).
		Where("organization_id = ? AND contact_id = ? AND status = ?", account.OrganizationID, contact.ID, models.TransferStatusActive).
		Count(&existingCount)

	if existingCount > 0 {
		a.Log.Debug("Contact already has active transfer, skipping team transfer", "contact_id", contact.ID, "team_id", teamID)
		return
	}

	// Get chatbot settings for SLA (use cache)
	settings, _ := a.getChatbotSettingsCached(account.OrganizationID, account.Name)

	// Apply team's assignment strategy
	agentID := a.assignToTeam(teamID, account.OrganizationID)

	// Create transfer
	transfer := models.AgentTransfer{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  account.OrganizationID,
		ContactID:       contact.ID,
		WhatsAppAccount: account.Name,
		PhoneNumber:     contact.PhoneNumber,
		Status:          models.TransferStatusActive,
		Source:          source,
		AgentID:         agentID,
		TeamID:          &teamID,
		Notes:           notes,
		TransferredAt:   time.Now(),
	}

	// Set SLA deadlines
	if settings != nil {
		a.SetSLADeadlines(&transfer, settings)
	}

	// If agent is already assigned, mark as picked up
	if agentID != nil {
		a.UpdateSLAOnPickup(&transfer)
	}

	if err := a.DB.Create(&transfer).Error; err != nil {
		a.Log.Error("Failed to create team transfer", "error", err, "contact_id", contact.ID, "team_id", teamID)
		return
	}

	// Update contact assignment if agent assigned
	if agentID != nil {
		a.DB.Model(&contact).Update("assigned_user_id", agentID)
	}

	// End any active chatbot session
	a.DB.Model(&models.ChatbotSession{}).
		Where("organization_id = ? AND contact_id = ? AND status = ?", account.OrganizationID, contact.ID, models.SessionStatusActive).
		Updates(map[string]any{
			"status":       models.SessionStatusCancelled,
			"completed_at": time.Now(),
		})

	var agentIDStrLog string
	if agentID != nil {
		agentIDStrLog = agentID.String()
	}
	a.Log.Info("Agent transfer created to team",
		"transfer_id", transfer.ID,
		"contact_id", contact.ID,
		"team_id", teamID,
		"agent_id", agentIDStrLog,
		"source", source,
	)

	// Broadcast to WebSocket
	a.broadcastTransferCreated(&transfer, contact)
}


// ReturnAgentTransfersToQueue returns all active transfers assigned to an agent back to their team queues
// Called when an agent goes offline/unavailable
func (a *App) ReturnAgentTransfersToQueue(userID, orgID uuid.UUID) int {
	var transfers []models.AgentTransfer
	if err := a.DB.Where("agent_id = ? AND organization_id = ? AND status = ?", userID, orgID, models.TransferStatusActive).
		Preload("Contact").Find(&transfers).Error; err != nil {
		a.Log.Error("Failed to find agent transfers for queue return", "error", err, "user_id", userID)
		return 0
	}

	if len(transfers) == 0 {
		return 0
	}

	// Return each transfer to its team queue (or general queue)
	for i := range transfers {
		transfer := &transfers[i]
		transfer.AgentID = nil

		if err := a.DB.Save(transfer).Error; err != nil {
			a.Log.Error("Failed to return transfer to queue", "error", err, "transfer_id", transfer.ID)
			continue
		}

		// Clear contact assignment
		if transfer.ContactID != uuid.Nil {
			a.DB.Model(&models.Contact{}).Where("id = ?", transfer.ContactID).Update("assigned_user_id", nil)
		}

		// Broadcast the unassignment
		a.broadcastTransferAssigned(transfer)
	}

	a.Log.Info("Returned agent transfers to queue",
		"user_id", userID,
		"count", len(transfers),
	)

	return len(transfers)
}
