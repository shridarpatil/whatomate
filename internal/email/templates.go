package email

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"strings"
)

//go:embed templates/*.html
var templateFS embed.FS

// TemplateRenderer renders email templates.
type TemplateRenderer struct {
	templates *template.Template
}

// NewTemplateRenderer creates a new TemplateRenderer by parsing all embedded templates.
func NewTemplateRenderer() (*TemplateRenderer, error) {
	funcMap := template.FuncMap{
		"upper": strings.ToUpper,
		"lower": strings.ToLower,
		"safe":  func(s string) template.HTML { return template.HTML(s) },
	}

	tmpl, err := template.New("").Funcs(funcMap).ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("email: failed to parse templates: %w", err)
	}

	return &TemplateRenderer{templates: tmpl}, nil
}

// Render renders a named template with the given data and returns the HTML output.
func (r *TemplateRenderer) Render(name string, data any) (string, error) {
	var buf bytes.Buffer
	if err := r.templates.ExecuteTemplate(&buf, name, data); err != nil {
		return "", fmt.Errorf("email: failed to render template %q: %w", name, err)
	}
	return buf.String(), nil
}

// WelcomeData holds data for the welcome email template.
type WelcomeData struct {
	UserName     string
	OrgName      string
	LoginURL     string
	SupportEmail string
}

// PasswordResetData holds data for the password reset email template.
type PasswordResetData struct {
	UserName  string
	ResetURL  string
	ExpiryMin int
	IPAddress string
}

// AlertNoAgentData holds data for the no-agent alert email template.
type AlertNoAgentData struct {
	OrgName      string
	ContactPhone string
	ContactName  string
	TeamName     string
	Timestamp    string
	DashboardURL string
}

// InviteData holds data for the user invitation email template.
type InviteData struct {
	InviteeName string
	InviterName string
	OrgName     string
	Email       string
	Password    string
	InviteURL   string
}

// WeeklyReportData holds data for the organization weekly usage report.
type WeeklyReportData struct {
	OrgName         string
	ReportDate      string
	MessagesSent    int
	MessagesRecv    int
	CallsMade       int
	ActiveAgents    int
	DashboardURL    string
}

// PlanLimitData holds data for the plan limit reached alert.
type PlanLimitData struct {
	OrgName         string
	LimitType       string // e.g., "Monthly Message Limit" or "Agent Seat Limit"
	CurrentUsage    int
	MaxAllowed      int
	UpgradeURL      string
}

// AuditLogData holds data for the super admin audit log email.
type AuditLogData struct {
	OrgName         string
	ActionItem      string
	PerformedBy     string
	Timestamp       string
	Severity        string // e.g., "HIGH", "CRITICAL"
	IPAddress       string
	Details         string
}
