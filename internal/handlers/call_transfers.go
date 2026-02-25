package handlers

import (
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

	if !a.IsCallingEnabledForOrg(orgID) {
		return r.SendErrorEnvelope(fasthttp.StatusServiceUnavailable, "Calling is not enabled for this organization", nil, "")
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

	return r.SendEnvelope(map[string]string{
		"status": "completed",
	})
}
