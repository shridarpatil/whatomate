package handlers

import (
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

// Stub handlers - not yet implemented. They still enforce the permission the
// real handler will need, so implementing them later can't silently open them up.

// Message handlers
func (a *App) MarkMessageRead(r *fastglue.Request) error {
	if _, _, err := a.requireAuth(r, models.ResourceChat, models.ActionRead); err != nil {
		return nil
	}
	return r.SendErrorEnvelope(fasthttp.StatusNotImplemented, "Not implemented yet", nil, "")
}

// Analytics handlers
func (a *App) GetMessageAnalytics(r *fastglue.Request) error {
	if _, _, err := a.requireAuth(r, models.ResourceAnalytics, models.ActionRead); err != nil {
		return nil
	}
	return r.SendErrorEnvelope(fasthttp.StatusNotImplemented, "Not implemented yet", nil, "")
}

func (a *App) GetChatbotAnalytics(r *fastglue.Request) error {
	if _, _, err := a.requireAuth(r, models.ResourceAnalytics, models.ActionRead); err != nil {
		return nil
	}
	return r.SendErrorEnvelope(fasthttp.StatusNotImplemented, "Not implemented yet", nil, "")
}
