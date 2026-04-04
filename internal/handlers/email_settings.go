package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/shridarpatil/whatomate/internal/crypto"
	"github.com/shridarpatil/whatomate/internal/email"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

// EmailSettingsResponse represents the SMTP settings returned to the frontend.
// Sensitive fields are masked.
type EmailSettingsResponse struct {
	Enabled   bool   `json:"enabled"`
	SMTPHost  string `json:"smtp_host"`
	SMTPPort  int    `json:"smtp_port"`
	SMTPUser  string `json:"smtp_user"`
	SMTPPass  string `json:"smtp_pass"` // Masked: "••••••••" if set
	FromEmail string `json:"email_from_address"`
	FromName  string `json:"email_from_name"`
	SMTPTLS   bool   `json:"smtp_tls"`
}

// GetEmailSettings returns the organization's SMTP settings.
// Password is masked for security — never returned in plaintext.
func (a *App) GetEmailSettings(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	if err := a.requirePermission(r, userID, models.ResourceSettingsGeneral, models.ActionRead); err != nil {
		return nil
	}

	var org models.Organization
	if err := a.DB.Where("id = ?", orgID).First(&org).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Organization not found", nil, "")
	}

	resp := EmailSettingsResponse{
		SMTPPort: 587,
	}

	if org.Settings != nil {
		if v, ok := org.Settings["smtp_host"].(string); ok {
			resp.SMTPHost = v
		}
		if v, ok := org.Settings["smtp_port"].(float64); ok && v > 0 {
			resp.SMTPPort = int(v)
		}
		if v, ok := org.Settings["smtp_user"].(string); ok {
			resp.SMTPUser = v
		}
		if v, ok := org.Settings["email_from_address"].(string); ok {
			resp.FromEmail = v
		}
		if v, ok := org.Settings["email_from_name"].(string); ok {
			resp.FromName = v
		}
		if v, ok := org.Settings["smtp_tls"].(bool); ok {
			resp.SMTPTLS = v
		}
		if v, ok := org.Settings["email_enabled"].(bool); ok {
			resp.Enabled = v
		}
		// Mask password — never return the real value
		if v, ok := org.Settings["smtp_pass"].(string); ok && v != "" {
			resp.SMTPPass = "••••••••"
		}
	}

	return r.SendEnvelope(resp)
}

// UpdateEmailSettingsRequest represents the request body for updating SMTP settings.
type UpdateEmailSettingsRequest struct {
	Enabled   *bool   `json:"enabled"`
	SMTPHost  *string `json:"smtp_host"`
	SMTPPort  *int    `json:"smtp_port"`
	SMTPUser  *string `json:"smtp_user"`
	SMTPPass  *string `json:"smtp_pass"`
	FromEmail *string `json:"email_from_address"`
	FromName  *string `json:"email_from_name"`
	SMTPTLS   *bool   `json:"smtp_tls"`
}

// UpdateEmailSettings updates the organization's SMTP settings.
// Password is encrypted before storage using the app-level encryption key.
func (a *App) UpdateEmailSettings(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	if err := a.requirePermission(r, userID, models.ResourceSettingsGeneral, models.ActionWrite); err != nil {
		return nil
	}

	var req UpdateEmailSettingsRequest
	if err := json.Unmarshal(r.RequestCtx.PostBody(), &req); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid request body", nil, "")
	}

	var org models.Organization
	if err := a.DB.Where("id = ?", orgID).First(&org).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Organization not found", nil, "")
	}

	if org.Settings == nil {
		org.Settings = models.JSONB{}
	}

	if req.Enabled != nil {
		org.Settings["email_enabled"] = *req.Enabled
	}
	if req.SMTPHost != nil {
		org.Settings["smtp_host"] = *req.SMTPHost
	}
	if req.SMTPPort != nil && *req.SMTPPort > 0 {
		org.Settings["smtp_port"] = *req.SMTPPort
	}
	if req.SMTPUser != nil {
		org.Settings["smtp_user"] = *req.SMTPUser
	}
	if req.FromEmail != nil {
		org.Settings["email_from_address"] = *req.FromEmail
	}
	if req.FromName != nil {
		org.Settings["email_from_name"] = *req.FromName
	}
	if req.SMTPTLS != nil {
		org.Settings["smtp_tls"] = *req.SMTPTLS
	}

	// Encrypt password before storing (skip the "••••••••" placeholder)
	if req.SMTPPass != nil && *req.SMTPPass != "" && *req.SMTPPass != "••••••••" {
		encrypted, err := crypto.Encrypt(*req.SMTPPass, a.Config.App.EncryptionKey)
		if err != nil {
			a.Log.Error("Failed to encrypt SMTP password", "error", err)
			return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to save settings", nil, "")
		}
		org.Settings["smtp_pass"] = encrypted
	}

	if err := a.DB.Save(&org).Error; err != nil {
		a.Log.Error("Failed to update email settings", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to update settings", nil, "")
	}

	return r.SendEnvelope(map[string]string{"message": "Email settings updated successfully"})
}

// TestEmailRequest represents the request body for sending a test email.
type TestEmailRequest struct {
	RecipientEmail string `json:"recipient_email"`
}

// TestEmailSettings sends a test email using the organization's current SMTP settings.
func (a *App) TestEmailSettings(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	if err := a.requirePermission(r, userID, models.ResourceSettingsGeneral, models.ActionWrite); err != nil {
		return nil
	}

	var req TestEmailRequest
	if err := json.Unmarshal(r.RequestCtx.PostBody(), &req); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid request body", nil, "")
	}

	if req.RecipientEmail == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "recipient_email is required", nil, "")
	}

	// Load org SMTP config
	cfg, orgName, err := a.getOrgEmailConfig(orgID)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, err.Error(), nil, "")
	}

	mailer := email.New(*cfg)
	if mailer == nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid SMTP configuration", nil, "")
	}

	// First test the connection
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := mailer.TestConnection(ctx); err != nil {
		a.Log.Error("SMTP connection test failed", "error", err, "host", cfg.Host)
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "SMTP connection failed: "+err.Error(), nil, "")
	}

	// Dispatch test email asynchronously
	a.SendEmailAsync(r.RequestCtx, orgID, "test.html", []string{req.RecipientEmail}, "Whatomate - Test Email", map[string]any{
		"OrgName":   orgName,
		"SMTPHost":  cfg.Host,
		"FromEmail": cfg.FromEmail,
	})

	// Also trigger an audit log email for super admin verification if this is a test
	a.SendEmailAsync(r.RequestCtx, orgID, "audit_log.html", []string{req.RecipientEmail}, "Security Alert: SMTP Test Dispatch", map[string]any{
		"OrgName":     orgName,
		"ActionItem":  "SMTP TEST",
		"PerformedBy": "System Admin",
		"Timestamp":   time.Now().Format(time.RFC3339),
		"Severity":    "HIGH",
		"IPAddress":   r.RequestCtx.RemoteIP().String(),
		"Details":     "SMTP settings test initiated by user for " + req.RecipientEmail,
	})

	a.Log.Info("Test email queued", "to", req.RecipientEmail, "org_id", orgID)
	return r.SendEnvelope(map[string]string{"message": "Test email queued successfully for " + req.RecipientEmail + ". Please check your inbox in a moment."})
}

// getOrgEmailConfig loads and decrypts SMTP settings from the organization's settings JSONB.
func (a *App) getOrgEmailConfig(orgID interface{}) (*email.Config, string, error) {
	var org models.Organization
	if err := a.DB.Where("id = ?", orgID).First(&org).Error; err != nil {
		return nil, "", fmt.Errorf("organization not found")
	}

	if org.Settings == nil {
		return nil, org.Name, fmt.Errorf("email not configured for this organization")
	}

	host, _ := org.Settings["smtp_host"].(string)
	if host == "" {
		return nil, org.Name, fmt.Errorf("SMTP host is not configured")
	}

	port := 587
	if v, ok := org.Settings["smtp_port"].(float64); ok && v > 0 {
		port = int(v)
	}

	password, _ := org.Settings["smtp_pass"].(string)
	if password != "" {
		if dec, err := crypto.Decrypt(password, a.Config.App.EncryptionKey); err == nil {
			password = dec
		}
	}

	useTLS := false
	if v, ok := org.Settings["smtp_tls"].(bool); ok {
		useTLS = v
	}

	user, _ := org.Settings["smtp_user"].(string)
	fromEmail, _ := org.Settings["email_from_address"].(string)
	fromName, _ := org.Settings["email_from_name"].(string)

	cfg := &email.Config{
		Host:      host,
		Port:      port,
		Username:  user,
		Password:  password,
		FromEmail: fromEmail,
		FromName:  fromName,
		TLS:       useTLS,
	}

	return cfg, org.Name, nil
}
