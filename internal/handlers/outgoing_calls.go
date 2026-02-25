package handlers

import (
	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/pkg/whatsapp"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

// InitiateOutgoingCall handles POST /api/calls/outgoing
// Lets an agent start a voice call to a WhatsApp consumer.
func (a *App) InitiateOutgoingCall(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}
	if err := a.requirePermission(r, userID, models.ResourceOutgoingCalls, models.ActionWrite); err != nil {
		return nil
	}

	var req struct {
		ContactID       string `json:"contact_id"`
		WhatsAppAccount string `json:"whatsapp_account"`
		SDPOffer        string `json:"sdp_offer"`
	}
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}

	if req.ContactID == "" || req.WhatsAppAccount == "" || req.SDPOffer == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "contact_id, whatsapp_account, and sdp_offer are required", nil, "")
	}

	if !a.IsCallingEnabledForOrg(orgID) {
		return r.SendErrorEnvelope(fasthttp.StatusServiceUnavailable, "Calling is not enabled for this organization", nil, "")
	}

	// Look up account
	var account models.WhatsAppAccount
	if err := a.DB.Where("organization_id = ? AND name = ?", orgID, req.WhatsAppAccount).
		First(&account).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "WhatsApp account not found", nil, "")
	}

	// Look up contact by ID
	contactID, parseErr := uuid.Parse(req.ContactID)
	if parseErr != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid contact_id", nil, "")
	}

	var contact models.Contact
	if err := a.DB.Where("id = ? AND organization_id = ?", contactID, orgID).
		First(&contact).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Contact not found", nil, "")
	}

	waAccount := &whatsapp.Account{
		PhoneID:     account.PhoneID,
		BusinessID:  account.BusinessID,
		APIVersion:  account.APIVersion,
		AccessToken: account.AccessToken,
	}

	callLogID, sdpAnswer, err := a.CallManager.InitiateOutgoingCall(
		orgID, userID, contact.ID,
		contact.PhoneNumber, req.WhatsAppAccount,
		waAccount, req.SDPOffer,
	)
	if err != nil {
		a.Log.Error("Failed to initiate outgoing call", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to initiate call: "+err.Error(), nil, "")
	}

	return r.SendEnvelope(map[string]string{
		"call_log_id": callLogID.String(),
		"sdp_answer":  sdpAnswer,
	})
}

// HangupOutgoingCall handles POST /api/calls/outgoing/{id}/hangup
func (a *App) HangupOutgoingCall(r *fastglue.Request) error {
	_, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}
	if err := a.requirePermission(r, userID, models.ResourceOutgoingCalls, models.ActionWrite); err != nil {
		return nil
	}

	callLogID, err := parsePathUUID(r, "id", "call log")
	if err != nil {
		return nil
	}

	if a.CallManager == nil {
		return r.SendErrorEnvelope(fasthttp.StatusServiceUnavailable, "Calling is not enabled", nil, "")
	}

	if err := a.CallManager.HangupOutgoingCall(callLogID, userID); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, err.Error(), nil, "")
	}

	return r.SendEnvelope(map[string]string{"status": "ok"})
}

// SendCallPermissionRequest handles POST /api/calls/permission-request
func (a *App) SendCallPermissionRequest(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}
	if err := a.requirePermission(r, userID, models.ResourceOutgoingCalls, models.ActionWrite); err != nil {
		return nil
	}

	var req struct {
		ContactID       string `json:"contact_id"`
		WhatsAppAccount string `json:"whatsapp_account"`
	}
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}

	if req.ContactID == "" || req.WhatsAppAccount == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "contact_id and whatsapp_account are required", nil, "")
	}

	if !a.IsCallingEnabledForOrg(orgID) {
		return r.SendErrorEnvelope(fasthttp.StatusServiceUnavailable, "Calling is not enabled for this organization", nil, "")
	}

	contactID, parseErr := uuid.Parse(req.ContactID)
	if parseErr != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid contact_id", nil, "")
	}

	// Verify contact exists
	var contact models.Contact
	if err := a.DB.Where("id = ? AND organization_id = ?", contactID, orgID).First(&contact).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Contact not found", nil, "")
	}

	// Look up account
	var account models.WhatsAppAccount
	if err := a.DB.Where("organization_id = ? AND name = ?", orgID, req.WhatsAppAccount).
		First(&account).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "WhatsApp account not found", nil, "")
	}

	waAccount := &whatsapp.Account{
		PhoneID:     account.PhoneID,
		BusinessID:  account.BusinessID,
		APIVersion:  account.APIVersion,
		AccessToken: account.AccessToken,
	}

	// Send permission request via WhatsApp Messages API
	ctx := r.RequestCtx
	messageID, err := a.WhatsApp.SendCallPermissionRequest(ctx, waAccount, contact.PhoneNumber, "")
	if err != nil {
		a.Log.Error("Failed to send call permission request", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to send permission request", nil, "")
	}

	// Create CallPermission record
	permission := models.CallPermission{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  orgID,
		ContactID:       contactID,
		WhatsAppAccount: req.WhatsAppAccount,
		Status:          models.CallPermissionPending,
		MessageID:       messageID,
		RequestedByID:   &userID,
	}
	if err := a.DB.Create(&permission).Error; err != nil {
		a.Log.Error("Failed to create call permission record", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to save permission", nil, "")
	}

	return r.SendEnvelope(map[string]string{
		"permission_id": permission.ID.String(),
	})
}

// GetICEServers handles GET /api/calls/ice-servers
// Returns the configured ICE (STUN/TURN) servers for the frontend to use in WebRTC peer connections.
func (a *App) GetICEServers(r *fastglue.Request) error {
	_, _, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	type iceServer struct {
		URLs       []string `json:"urls"`
		Username   string   `json:"username,omitempty"`
		Credential string   `json:"credential,omitempty"`
	}

	servers := make([]iceServer, 0, len(a.Config.Calling.ICEServers))
	for _, s := range a.Config.Calling.ICEServers {
		servers = append(servers, iceServer{
			URLs:       s.URLs,
			Username:   s.Username,
			Credential: s.Credential,
		})
	}

	return r.SendEnvelope(map[string]any{
		"ice_servers": servers,
	})
}

// GetCallPermission handles GET /api/calls/permission/{contactId}?whatsapp_account=X
// Checks call permission state directly via WhatsApp API.
func (a *App) GetCallPermission(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}
	if err := a.requirePermission(r, userID, models.ResourceOutgoingCalls, models.ActionRead); err != nil {
		return nil
	}

	contactID, err := parsePathUUID(r, "contactId", "contact")
	if err != nil {
		return nil
	}

	accountName := string(r.RequestCtx.QueryArgs().Peek("whatsapp_account"))
	if accountName == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "whatsapp_account query param is required", nil, "")
	}

	// Look up contact
	var contact models.Contact
	if err := a.DB.Where("id = ? AND organization_id = ?", contactID, orgID).First(&contact).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Contact not found", nil, "")
	}

	// Look up WhatsApp account
	var account models.WhatsAppAccount
	if err := a.DB.Where("organization_id = ? AND name = ?", orgID, accountName).First(&account).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "WhatsApp account not found", nil, "")
	}

	waAccount := &whatsapp.Account{
		PhoneID:     account.PhoneID,
		BusinessID:  account.BusinessID,
		APIVersion:  account.APIVersion,
		AccessToken: account.AccessToken,
	}

	// Check permission via WhatsApp API
	ctx := r.RequestCtx
	status, err := a.WhatsApp.GetCallPermission(ctx, waAccount, contact.PhoneNumber)
	if err != nil {
		a.Log.Error("Failed to check call permission via API", "error", err, "phone", contact.PhoneNumber)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to check permission", nil, "")
	}

	a.Log.Info("Call permission check result", "contact_id", contactID, "phone", contact.PhoneNumber, "status", status)

	return r.SendEnvelope(map[string]string{
		"status": status,
	})
}

