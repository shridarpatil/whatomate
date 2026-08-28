package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

// serviceWindowOpen reports whether the customer messaged within the last 24
// hours. Outside that window WhatsApp only accepts templates.
//
// The same expression exists inline in contacts.go, where it feeds
// service_window_open on the contact payload. It is duplicated rather than
// extracted because extracting it would edit a production file for no
// behavioural gain — see the spec's "no production change" constraint.
func serviceWindowOpen(contact *models.Contact) bool {
	return contact.LastInboundAt != nil && time.Since(*contact.LastInboundAt) < 24*time.Hour
}

// SendOccurrenceProtocol sends the protocol number to the customer.
//
// This is the only endpoint in the system that enforces the 24-hour window.
// SendMessage does not — it computes the window, reports it, and sends anyway.
// The inconsistency is deliberate and recorded in the spec: enforcing it
// globally would change production behaviour.
func (a *App) SendOccurrenceProtocol(r *fastglue.Request) error {
	orgID, userID, err := a.requireAuth(r, models.ResourceChat, models.ActionWrite)
	if err != nil {
		return nil
	}

	occ, err := a.loadAuthorizedOccurrence(r, orgID, userID, true)
	if err != nil {
		return nil
	}

	contact := occ.Contact
	if !serviceWindowOpen(contact) {
		return r.SendErrorEnvelope(fasthttp.StatusUnprocessableEntity,
			"The 24-hour service window is closed; only templates can be sent", nil, "")
	}

	var account models.WhatsAppAccount
	if err := a.DB.Where("organization_id = ? AND name = ?", orgID, contact.WhatsAppAccount).
		First(&account).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest,
			"WhatsApp account not found for this contact", nil, "")
	}

	body := fmt.Sprintf("Seu protocolo de atendimento é %s. Guarde este número para consultas futuras.",
		occ.ProtocolNumber)

	if _, err := a.SendOutgoingMessage(context.Background(), OutgoingMessageRequest{
		Account: &account,
		Contact: contact,
		Type:    models.MessageTypeText,
		Content: body,
	}, DefaultSendOptions()); err != nil {
		a.Log.Error("Failed to send protocol", "error", err, "occurrence", occ.ID)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError,
			"Failed to send protocol", nil, "")
	}

	a.DB.Create(&models.OccurrenceEvent{
		OrganizationID: orgID,
		OccurrenceID:   occ.ID,
		Type:           models.OccurrenceEventProtocolSent,
		Content:        occ.ProtocolNumber,
		CreatedByID:    &userID,
	})

	return r.SendEnvelope(map[string]any{"sent": true, "protocol_number": occ.ProtocolNumber})
}
