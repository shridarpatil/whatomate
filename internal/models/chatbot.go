package models

import (
	"time"

	"github.com/google/uuid"
)

// BusinessHoursConfig holds business hours settings
type BusinessHoursConfig struct {
	Enabled              bool       `gorm:"column:business_hours_enabled;default:false" json:"business_hours_enabled"`
	Hours                JSONBArray `gorm:"column:business_hours;type:jsonb;default:'[]'" json:"business_hours"` // [{day, enabled, start_time, end_time}]
	OutOfHoursMessage    string     `gorm:"column:out_of_hours_message;type:text" json:"out_of_hours_message"`
	AllowAutomatedOutside bool      `gorm:"column:allow_automated_outside_hours;default:true" json:"allow_automated_outside_hours"` // Allow flows/keywords/AI outside business hours
}

// AgentAssignmentConfig holds agent assignment and queue settings
type AgentAssignmentConfig struct {
	AllowQueuePickup        bool `gorm:"column:allow_agent_queue_pickup;default:true" json:"allow_agent_queue_pickup"`           // Allow agents to pick transfers from queue
	AssignToSameAgent       bool `gorm:"column:assign_to_same_agent;default:true" json:"assign_to_same_agent"`                   // Auto-assign transfers to contact's existing agent
	CurrentConversationOnly bool `gorm:"column:agent_current_conversation_only;default:false" json:"agent_current_conversation_only"` // Agents see only current session messages
}

// SLAConfig holds SLA tracking settings
type SLAConfig struct {
	Enabled             bool        `gorm:"column:sla_enabled;default:false" json:"sla_enabled"`                            // Enable SLA tracking
	ResponseMinutes     int         `gorm:"column:sla_response_minutes;default:15" json:"sla_response_minutes"`             // Time to pick up transfer (default 15 min)
	ResolutionMinutes   int         `gorm:"column:sla_resolution_minutes;default:60" json:"sla_resolution_minutes"`         // Time to resolve transfer (default 60 min)
	EscalationMinutes   int         `gorm:"column:sla_escalation_minutes;default:30" json:"sla_escalation_minutes"`         // Time before escalation (default 30 min)
	AutoCloseHours      int         `gorm:"column:sla_auto_close_hours;default:24" json:"sla_auto_close_hours"`             // Auto-close stale transfers (default 24h)
	AutoCloseMessage    string      `gorm:"column:sla_auto_close_message;type:text" json:"sla_auto_close_message"`          // Message to customer when chat is auto-closed
	WarningMessage      string      `gorm:"column:sla_warning_message;type:text" json:"sla_warning_message"`                // Message to customer when SLA breached
	EscalationNotifyIDs StringArray `gorm:"column:sla_escalation_notify_ids;type:jsonb;default:'[]'" json:"sla_escalation_notify_ids"` // User IDs to notify on escalation
}

// ClientInactivityConfig holds client inactivity and reminder settings
type ClientInactivityConfig struct {
	ReminderEnabled  bool   `gorm:"column:client_reminder_enabled;default:false" json:"client_reminder_enabled"`   // Enable client inactivity reminders
	ReminderMinutes  int    `gorm:"column:client_reminder_minutes;default:30" json:"client_reminder_minutes"`      // Send reminder after X minutes of client inactivity
	ReminderMessage  string `gorm:"column:client_reminder_message;type:text" json:"client_reminder_message"`       // Reminder message to client
	AutoCloseMinutes int    `gorm:"column:client_auto_close_minutes;default:60" json:"client_auto_close_minutes"`  // Auto-close after Y minutes of client inactivity
	AutoCloseMessage string `gorm:"column:client_auto_close_message;type:text" json:"client_auto_close_message"`   // Message when closing due to client inactivity
}

// AIConfig holds AI provider settings
type AIConfig struct {
	Enabled        bool    `gorm:"column:ai_enabled;default:false" json:"ai_enabled"`
	Provider       AIProvider `gorm:"column:ai_provider;size:20" json:"ai_provider"`                     // openai, anthropic, google
	APIKey         string  `gorm:"column:ai_api_key;type:text" json:"-"`                                 // encrypted
	Model          string  `gorm:"column:ai_model;size:100" json:"ai_model"`
	MaxTokens      int     `gorm:"column:ai_max_tokens;default:500" json:"ai_max_tokens"`
	Temperature    float64 `gorm:"column:ai_temperature;type:decimal(3,2);default:0.7" json:"ai_temperature"`
	SystemPrompt   string  `gorm:"column:ai_system_prompt;type:text" json:"ai_system_prompt"`
	IncludeHistory bool    `gorm:"column:ai_include_history;default:true" json:"ai_include_history"`
	HistoryLimit   int     `gorm:"column:ai_history_limit;default:4" json:"ai_history_limit"`
}

// PanelFieldConfig defines a field to display in the contact info panel
type PanelFieldConfig struct {
	Key         string `json:"key"`                    // Variable name (from StoreAs or response_mapping)
	Label       string `json:"label"`                  // Admin-defined display label
	Order       int    `json:"order"`                  // Field order within section
	DisplayType string `json:"display_type,omitempty"` // text (default), badge, tag
	Color       string `json:"color,omitempty"`        // default, success, warning, error, info
}

// PanelSection defines a section in the contact info panel
type PanelSection struct {
	ID               string             `json:"id"`                // Unique section ID
	Label            string             `json:"label"`             // Admin-defined section label
	Columns          int                `json:"columns"`           // 1 or 2 columns layout
	Collapsible      bool               `json:"collapsible"`       // Can section be collapsed
	DefaultCollapsed bool               `json:"default_collapsed"` // Start collapsed
	Order            int                `json:"order"`             // Section display order
	Fields           []PanelFieldConfig `json:"fields"`            // Fields in this section
}

// PanelConfig defines the contact info panel configuration for a flow
type PanelConfig struct {
	Sections []PanelSection `json:"sections"`
}

// ChatbotSettings holds chatbot configuration per WhatsApp account
// WhatsAppAccount can be empty for organization-level default settings
type ChatbotSettings struct {
	BaseModel
	OrganizationID  uuid.UUID `gorm:"type:uuid;index;not null" json:"organization_id"`
	WhatsAppAccount string    `gorm:"size:100;index" json:"whatsapp_account"` // References WhatsAppAccount.Name (empty for org-level defaults)
	IsEnabled       bool      `gorm:"default:false" json:"is_enabled"`

	// Response settings
	DefaultResponse string     `gorm:"type:text" json:"default_response"`
	GreetingButtons JSONBArray `gorm:"type:jsonb;default:'[]'" json:"greeting_buttons"` // [{id, title}] - max 10 buttons
	FallbackMessage string     `gorm:"type:text" json:"fallback_message"`
	FallbackButtons JSONBArray `gorm:"type:jsonb;default:'[]'" json:"fallback_buttons"` // [{id, title}] - max 10 buttons

	// Embedded configs (all fields stored in same table)
	BusinessHours    BusinessHoursConfig    `gorm:"embedded"`
	AgentAssignment  AgentAssignmentConfig  `gorm:"embedded"`
	SLA              SLAConfig              `gorm:"embedded"`
	ClientInactivity ClientInactivityConfig `gorm:"embedded"`
	AI               AIConfig               `gorm:"embedded"`

	// Session settings
	SessionTimeoutMins int        `gorm:"default:30" json:"session_timeout_minutes"`
	ExcludedNumbers    JSONBArray `gorm:"type:jsonb;default:'[]'" json:"excluded_numbers"`

	// Relations
	Organization *Organization `gorm:"foreignKey:OrganizationID" json:"organization,omitempty"`
}

func (ChatbotSettings) TableName() string {
	return "chatbot_settings"
}

// KeywordRule defines automatic response rules based on keywords
type KeywordRule struct {
	BaseModel
	OrganizationID  uuid.UUID   `gorm:"type:uuid;index;not null" json:"organization_id"`
	WhatsAppAccount string      `gorm:"size:100;index;not null" json:"whatsapp_account"` // References WhatsAppAccount.Name
	Name            string      `gorm:"size:255;not null" json:"name"`
	IsEnabled       bool        `gorm:"default:true" json:"is_enabled"`
	Priority        int         `gorm:"default:10" json:"priority"`
	Keywords        StringArray `gorm:"type:jsonb;not null" json:"keywords"`
	MatchType       MatchType    `gorm:"size:20;default:'contains'" json:"match_type"` // exact, contains, starts_with, regex
	CaseSensitive   bool         `gorm:"default:false" json:"case_sensitive"`
	ResponseType    ResponseType `gorm:"size:20;not null" json:"response_type"` // text, template, media, flow, script
	ResponseContent JSONB       `gorm:"type:jsonb;not null" json:"response_content"`
	Conditions      string      `gorm:"type:text" json:"conditions"`
	ActiveFrom      *time.Time  `json:"active_from,omitempty"`
	ActiveUntil     *time.Time  `json:"active_until,omitempty"`

	// Relations
	Organization *Organization `gorm:"foreignKey:OrganizationID" json:"organization,omitempty"`
}

func (KeywordRule) TableName() string {
	return "keyword_rules"
}

// ChatbotFlow defines multi-step conversation flows
type ChatbotFlow struct {
	BaseModel
	OrganizationID     uuid.UUID   `gorm:"type:uuid;index;not null" json:"organization_id"`
	WhatsAppAccount    string      `gorm:"size:100;index;not null" json:"whatsapp_account"` // References WhatsAppAccount.Name
	Name               string      `gorm:"size:255;not null" json:"name"`
	IsEnabled          bool        `gorm:"default:true" json:"is_enabled"`
	Description        string      `gorm:"type:text" json:"description"`
	TriggerKeywords    StringArray `gorm:"type:jsonb" json:"trigger_keywords"`
	TriggerButtonID    string      `gorm:"size:100" json:"trigger_button_id"`
	InitialMessage     string       `gorm:"type:text" json:"initial_message"`
	InitialMessageType FlowStepType `gorm:"size:20;default:'text'" json:"initial_message_type"`
	InitialTemplateID  *uuid.UUID  `gorm:"type:uuid" json:"initial_template_id,omitempty"`
	CompletionMessage  string      `gorm:"type:text" json:"completion_message"`
	OnCompleteAction   string      `gorm:"size:20" json:"on_complete_action"` // none, webhook, create_record
	CompletionConfig   JSONB       `gorm:"type:jsonb" json:"completion_config"`
	TimeoutMessage     string      `gorm:"type:text" json:"timeout_message"`
	CancelKeywords     StringArray `gorm:"type:jsonb" json:"cancel_keywords"`
	PanelConfig        JSONB       `gorm:"type:jsonb;default:'{}'" json:"panel_config"` // Contact info panel configuration

	// Relations
	Organization    *Organization     `gorm:"foreignKey:OrganizationID" json:"organization,omitempty"`
	InitialTemplate *Template         `gorm:"foreignKey:InitialTemplateID" json:"initial_template,omitempty"`
	Steps           []ChatbotFlowStep `gorm:"foreignKey:FlowID" json:"steps,omitempty"`
}

func (ChatbotFlow) TableName() string {
	return "chatbot_flows"
}

// ChatbotFlowStep defines individual steps in a conversation flow
type ChatbotFlowStep struct {
	BaseModel
	FlowID          uuid.UUID  `gorm:"type:uuid;index;not null" json:"flow_id"`
	StepName        string     `gorm:"size:100;not null" json:"step_name"`
	StepOrder       int        `gorm:"not null" json:"step_order"`
	Message         string       `gorm:"type:text;not null" json:"message"`
	MessageType     FlowStepType `gorm:"size:20;default:'text'" json:"message_type"` // text, template, script, api_fetch, buttons, transfer
	TemplateID      *uuid.UUID `gorm:"type:uuid" json:"template_id,omitempty"`
	ApiConfig       JSONB      `gorm:"type:jsonb" json:"api_config"`      // {url, method, headers, body, response_path, fallback_message}
	Buttons         JSONBArray `gorm:"type:jsonb" json:"buttons"`         // [{id, title}] - max 10 options (3=buttons, 4-10=list)
	TransferConfig  JSONB      `gorm:"type:jsonb" json:"transfer_config"` // {team_id: uuid, notes: string} - for transfer message type
	InputType       InputType  `gorm:"size:20" json:"input_type"`         // none, text, number, email, phone, date, select, button, whatsapp_flow
	InputConfig     JSONB      `gorm:"type:jsonb" json:"input_config"`
	ValidationRegex string     `gorm:"size:255" json:"validation_regex"`
	ValidationError string     `gorm:"type:text" json:"validation_error"`
	StoreAs         string     `gorm:"size:100" json:"store_as"`
	NextStep        string     `gorm:"size:100" json:"next_step"`
	ConditionalNext JSONB      `gorm:"type:jsonb" json:"conditional_next"` // {"option1": "step_a", "default": "step_b"}
	SkipCondition   string     `gorm:"type:text" json:"skip_condition"`
	RetryOnInvalid  bool       `gorm:"default:true" json:"retry_on_invalid"`
	MaxRetries      int        `gorm:"default:3" json:"max_retries"`

	// Relations
	Flow     *ChatbotFlow `gorm:"foreignKey:FlowID" json:"flow,omitempty"`
	Template *Template    `gorm:"foreignKey:TemplateID" json:"template,omitempty"`
}

func (ChatbotFlowStep) TableName() string {
	return "chatbot_flow_steps"
}

// ChatbotSession tracks active conversation sessions
type ChatbotSession struct {
	BaseModel
	OrganizationID  uuid.UUID  `gorm:"type:uuid;index;not null" json:"organization_id"`
	ContactID       uuid.UUID  `gorm:"type:uuid;index;not null" json:"contact_id"`
	WhatsAppAccount string     `gorm:"size:100;index;not null" json:"whatsapp_account"` // References WhatsAppAccount.Name
	PhoneNumber     string     `gorm:"size:50;not null" json:"phone_number"`
	Status          SessionStatus `gorm:"size:20;default:'active'" json:"status"` // active, completed, cancelled, timeout
	CurrentFlowID   *uuid.UUID `gorm:"type:uuid" json:"current_flow_id,omitempty"`
	CurrentStep     string     `gorm:"size:100" json:"current_step"`
	StepRetries     int        `gorm:"default:0" json:"step_retries"`
	SessionData     JSONB      `gorm:"type:jsonb;default:'{}'" json:"session_data"`
	StartedAt       time.Time  `gorm:"autoCreateTime" json:"started_at"`
	LastActivityAt  time.Time  `json:"last_activity_at"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`

	// Relations
	Organization *Organization           `gorm:"foreignKey:OrganizationID" json:"organization,omitempty"`
	Contact      *Contact                `gorm:"foreignKey:ContactID" json:"contact,omitempty"`
	CurrentFlow  *ChatbotFlow            `gorm:"foreignKey:CurrentFlowID" json:"current_flow,omitempty"`
	Messages     []ChatbotSessionMessage `gorm:"foreignKey:SessionID" json:"messages,omitempty"`
}

func (ChatbotSession) TableName() string {
	return "chatbot_sessions"
}

// ChatbotSessionMessage stores message history within a session
type ChatbotSessionMessage struct {
	BaseModel
	SessionID uuid.UUID `gorm:"type:uuid;index;not null" json:"session_id"`
	Direction Direction `gorm:"size:10;not null" json:"direction"` // incoming, outgoing
	Message   string    `gorm:"type:text" json:"message"`
	StepName  string    `gorm:"size:100" json:"step_name"`

	// Relations
	Session *ChatbotSession `gorm:"foreignKey:SessionID" json:"session,omitempty"`
}

func (ChatbotSessionMessage) TableName() string {
	return "chatbot_session_messages"
}

// AIContext provides context data for AI responses
type AIContext struct {
	BaseModel
	OrganizationID  uuid.UUID   `gorm:"type:uuid;index;not null" json:"organization_id"`
	WhatsAppAccount string      `gorm:"size:100;index" json:"whatsapp_account"` // References WhatsAppAccount.Name (empty for org-level)
	Name            string      `gorm:"size:255;not null" json:"name"`
	IsEnabled       bool        `gorm:"default:true" json:"is_enabled"`
	Priority        int         `gorm:"default:10" json:"priority"`
	ContextType     ContextType `gorm:"size:20;not null" json:"context_type"` // static, api
	TriggerKeywords StringArray `gorm:"type:jsonb" json:"trigger_keywords"`
	StaticContent   string      `gorm:"type:text" json:"static_content"`
	ApiConfig       JSONB       `gorm:"type:jsonb" json:"api_config"` // url, method, headers, body

	// Relations
	Organization *Organization `gorm:"foreignKey:OrganizationID" json:"organization,omitempty"`
}

func (AIContext) TableName() string {
	return "ai_contexts"
}

// SLATracking holds SLA-related tracking fields for agent transfers
type SLATracking struct {
	ResponseDeadline   *time.Time `gorm:"column:sla_response_deadline;index" json:"sla_response_deadline,omitempty"`   // When pickup is due
	ResolutionDeadline *time.Time `gorm:"column:sla_resolution_deadline;index" json:"sla_resolution_deadline,omitempty"` // When resolution is due
	EscalationAt       *time.Time `gorm:"column:sla_escalation_at" json:"sla_escalation_at,omitempty"`                  // When escalation is due
	ExpiresAt          *time.Time `gorm:"column:expires_at;index" json:"expires_at,omitempty"`                          // Auto-close deadline
	PickedUpAt         *time.Time `gorm:"column:picked_up_at" json:"picked_up_at,omitempty"`                            // When agent first picked up
	FirstResponseAt    *time.Time `gorm:"column:first_response_at" json:"first_response_at,omitempty"`                  // When agent first responded
	EscalationLevel    int        `gorm:"column:escalation_level;default:0" json:"escalation_level"`                    // 0=normal, 1=warning, 2=escalated, 3=critical
	EscalatedAt        *time.Time `gorm:"column:escalated_at" json:"escalated_at,omitempty"`                            // When escalation occurred
	Breached           bool       `gorm:"column:sla_breached;default:false" json:"sla_breached"`                        // Whether SLA was breached
	BreachedAt         *time.Time `gorm:"column:sla_breached_at" json:"sla_breached_at,omitempty"`                      // When SLA was breached
}

// AgentTransfer tracks when conversations are transferred to human agents
type AgentTransfer struct {
	BaseModel
	OrganizationID      uuid.UUID  `gorm:"type:uuid;index;not null" json:"organization_id"`
	ContactID           uuid.UUID  `gorm:"type:uuid;index;not null" json:"contact_id"`
	WhatsAppAccount     string     `gorm:"size:100;index;not null" json:"whatsapp_account"` // References WhatsAppAccount.Name
	PhoneNumber         string     `gorm:"size:50;not null" json:"phone_number"`
	Status              TransferStatus `gorm:"size:20;default:'active'" json:"status"` // active, resumed
	Source              TransferSource `gorm:"size:20;default:'manual'" json:"source"` // manual, flow, keyword, chatbot_disabled
	AgentID             *uuid.UUID `gorm:"type:uuid" json:"agent_id,omitempty"`
	TeamID              *uuid.UUID `gorm:"type:uuid;index" json:"team_id,omitempty"` // Team queue (null = general queue)
	TransferredByUserID *uuid.UUID `gorm:"type:uuid" json:"transferred_by_user_id,omitempty"` // User who initiated the transfer (null for system)
	Notes               string     `gorm:"type:text" json:"notes"`
	TransferredAt       time.Time  `gorm:"autoCreateTime" json:"transferred_at"`
	ResumedAt           *time.Time `json:"resumed_at,omitempty"`
	ResumedBy           *uuid.UUID `gorm:"type:uuid" json:"resumed_by,omitempty"`

	// SLA Tracking (embedded - all fields stored in same table)
	SLA SLATracking `gorm:"embedded"`

	// Relations
	Organization      *Organization `gorm:"foreignKey:OrganizationID" json:"organization,omitempty"`
	Contact           *Contact      `gorm:"foreignKey:ContactID" json:"contact,omitempty"`
	Agent             *User         `gorm:"foreignKey:AgentID" json:"agent,omitempty"`
	Team              *Team         `gorm:"foreignKey:TeamID" json:"team,omitempty"`
	TransferredByUser *User         `gorm:"foreignKey:TransferredByUserID" json:"transferred_by_user,omitempty"`
	ResumedByUser     *User         `gorm:"foreignKey:ResumedBy" json:"resumed_by_user,omitempty"`
}

func (AgentTransfer) TableName() string {
	return "agent_transfers"
}
