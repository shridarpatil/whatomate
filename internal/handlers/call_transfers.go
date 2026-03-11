package handlers

import (
	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

// ListCallTransfers returns call transfers for the organization
func (a *App) ListCallTransfers(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}
	if err := a.requirePermission(r, userID, models.ResourceCallTransfers, models.ActionRead); err != nil {
		return nil
	}

	pg := parsePagination(r)
	status := string(r.RequestCtx.QueryArgs().Peek("status"))

	query := a.DB.Where("call_transfers.organization_id = ?", orgID).
		Preload("Contact").
		Preload("Agent").
		Preload("InitiatingAgent").
		Preload("Team").
		Preload("CallLog").
		Order("call_transfers.created_at DESC")

	countQuery := a.DB.Model(&models.CallTransfer{}).Where("organization_id = ?", orgID)

	if status != "" {
		query = query.Where("call_transfers.status = ?", status)
		countQuery = countQuery.Where("status = ?", status)
	}

	var total int64
	countQuery.Count(&total)

	var transfers []models.CallTransfer
	if err := pg.Apply(query).Find(&transfers).Error; err != nil {
		a.Log.Error("Failed to fetch call transfers", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to fetch call transfers", nil, "")
	}

	return r.SendEnvelope(map[string]any{
		"call_transfers": transfers,
		"total":          total,
		"page":           pg.Page,
		"limit":          pg.Limit,
	})
}

// GetCallTransfer returns a single call transfer by ID
func (a *App) GetCallTransfer(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}
	if err := a.requirePermission(r, userID, models.ResourceCallTransfers, models.ActionRead); err != nil {
		return nil
	}

	transferID, err := parsePathUUID(r, "id", "call transfer")
	if err != nil {
		return nil
	}

	var transfer models.CallTransfer
	if err := a.DB.Where("id = ? AND organization_id = ?", transferID, orgID).
		Preload("Contact").
		Preload("Agent").
		Preload("InitiatingAgent").
		Preload("Team").
		Preload("CallLog").
		First(&transfer).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Call transfer not found", nil, "")
	}

	return r.SendEnvelope(transfer)
}

// ConnectCallTransfer handles an agent accepting a call transfer via WebRTC SDP exchange
func (a *App) ConnectCallTransfer(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}
	if err := a.requirePermission(r, userID, models.ResourceCallTransfers, models.ActionWrite); err != nil {
		return nil
	}

	transferID, err := parsePathUUID(r, "id", "call transfer")
	if err != nil {
		return nil
	}

	// Validate transfer exists and belongs to this org
	var transfer models.CallTransfer
	if err := a.DB.Where("id = ? AND organization_id = ?", transferID, orgID).
		First(&transfer).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Call transfer not found", nil, "")
	}

	if transfer.Status != models.CallTransferStatusWaiting {
		return r.SendErrorEnvelope(fasthttp.StatusConflict, "Transfer is no longer waiting", nil, "")
	}

	// Atomically claim the transfer in the DB so concurrent accepts are rejected
	res := a.DB.Model(&models.CallTransfer{}).
		Where("id = ? AND status = ?", transferID, models.CallTransferStatusWaiting).
		Update("status", models.CallTransferStatusConnected)
	if res.RowsAffected == 0 {
		return r.SendErrorEnvelope(fasthttp.StatusConflict, "Transfer was already accepted by another agent", nil, "")
	}

	// If transfer has a team_id, check agent is a member (unless super admin)
	if transfer.TeamID != nil && !a.IsSuperAdmin(userID) {
		var memberCount int64
		a.DB.Table("team_members").
			Where("team_id = ? AND user_id = ? AND deleted_at IS NULL", transfer.TeamID, userID).
			Count(&memberCount)
		if memberCount == 0 {
			return r.SendErrorEnvelope(fasthttp.StatusForbidden, "You are not a member of the target team", nil, "")
		}
	}

	// If transfer is directed to a specific agent, reject other agents
	if transfer.AgentID != nil && *transfer.AgentID != userID {
		// Revert the atomic claim
		a.DB.Model(&models.CallTransfer{}).
			Where("id = ?", transferID).
			Update("status", models.CallTransferStatusWaiting)
		return r.SendErrorEnvelope(fasthttp.StatusForbidden,
			"This transfer is directed to a specific agent", nil, "")
	}

	// Parse SDP offer from body
	var req struct {
		SDPOffer string `json:"sdp_offer"`
	}
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}
	if req.SDPOffer == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "sdp_offer is required", nil, "")
	}

	if err := a.requireCallingEnabled(r, orgID); err != nil {
		return nil
	}

	sdpAnswer, err := a.CallManager.ConnectAgentToTransfer(transferID, userID, req.SDPOffer)
	if err != nil {
		// Revert DB status so another agent can try
		a.DB.Model(&models.CallTransfer{}).
			Where("id = ? AND status = ?", transferID, models.CallTransferStatusConnected).
			Update("status", models.CallTransferStatusWaiting)
		a.Log.Error("Failed to connect agent to transfer", "error", err, "transfer_id", transferID)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to connect: "+err.Error(), nil, "")
	}

	return r.SendEnvelope(map[string]string{
		"sdp_answer": sdpAnswer,
	})
}

// HangupCallTransfer ends a connected call transfer
func (a *App) HangupCallTransfer(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}
	if err := a.requirePermission(r, userID, models.ResourceCallTransfers, models.ActionWrite); err != nil {
		return nil
	}

	transferID, err := parsePathUUID(r, "id", "call transfer")
	if err != nil {
		return nil
	}

	// Validate transfer belongs to this org
	var transfer models.CallTransfer
	if err := a.DB.Where("id = ? AND organization_id = ?", transferID, orgID).
		First(&transfer).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Call transfer not found", nil, "")
	}

	if a.CallManager == nil {
		return r.SendErrorEnvelope(fasthttp.StatusServiceUnavailable, "Calling is not enabled", nil, "")
	}

	a.CallManager.EndTransfer(transferID)

	// Mark the call as disconnected by agent
	a.DB.Model(&models.CallLog{}).
		Where("id = ?", transfer.CallLogID).
		Update("disconnected_by", models.DisconnectedByAgent)

	return r.SendEnvelope(map[string]string{
		"status": "completed",
	})
}

// InitiateAgentTransfer allows a connected agent to transfer their active call to another team/agent
func (a *App) InitiateAgentTransfer(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}
	if err := a.requirePermission(r, userID, models.ResourceCallTransfers, models.ActionWrite); err != nil {
		return nil
	}
	if err := a.requireCallingEnabled(r, orgID); err != nil {
		return nil
	}

	var req struct {
		CallLogID string `json:"call_log_id"`
		TeamID    string `json:"team_id"`
		AgentID   string `json:"agent_id"`
	}
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}

	if req.CallLogID == "" || req.TeamID == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "call_log_id and team_id are required", nil, "")
	}

	callLogID, err := uuid.Parse(req.CallLogID)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid call_log_id", nil, "")
	}

	teamID, err := uuid.Parse(req.TeamID)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid team_id", nil, "")
	}

	// Verify team belongs to this org
	var teamCount int64
	a.DB.Model(&models.Team{}).Where("id = ? AND organization_id = ?", teamID, orgID).Count(&teamCount)
	if teamCount == 0 {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Team not found", nil, "")
	}

	var targetAgentID *uuid.UUID
	if req.AgentID != "" {
		agentID, err := uuid.Parse(req.AgentID)
		if err != nil {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid agent_id", nil, "")
		}
		// Verify agent is a member of the team
		var memberCount int64
		a.DB.Table("team_members").
			Where("team_id = ? AND user_id = ? AND deleted_at IS NULL", teamID, agentID).
			Count(&memberCount)
		if memberCount == 0 {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Agent is not a member of the specified team", nil, "")
		}
		targetAgentID = &agentID
	}

	if a.CallManager == nil {
		return r.SendErrorEnvelope(fasthttp.StatusServiceUnavailable, "Calling is not enabled", nil, "")
	}

	if err := a.CallManager.InitiateAgentTransfer(callLogID, userID, &teamID, targetAgentID); err != nil {
		a.Log.Error("Failed to initiate agent transfer", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to initiate transfer: "+err.Error(), nil, "")
	}

	return r.SendEnvelope(map[string]string{
		"status": "transferring",
	})
}
