package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// JSONB is a custom type for PostgreSQL JSONB columns
type JSONB map[string]interface{}

func (j JSONB) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(bytes, j)
}

// JSONBArray is a custom type for JSONB arrays
type JSONBArray []interface{}

func (j JSONBArray) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

func (j *JSONBArray) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(bytes, j)
}

// StringArray is a custom type for PostgreSQL text[] columns
type StringArray []string

func (s StringArray) Value() (driver.Value, error) {
	if s == nil {
		return nil, nil
	}
	return json.Marshal(s)
}

func (s *StringArray) Scan(value interface{}) error {
	if value == nil {
		*s = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(bytes, s)
}

// BaseModel contains common fields for all models
type BaseModel struct {
	ID        uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

// Organization represents a tenant in the multi-tenant system
type Organization struct {
	BaseModel
	Name     string `gorm:"size:255;not null" json:"name"`
	Slug     string `gorm:"size:100;uniqueIndex;not null" json:"slug"`
	Settings JSONB  `gorm:"type:jsonb;default:'{}'" json:"settings"`

	// Relations
	Users              []User               `gorm:"foreignKey:OrganizationID" json:"users,omitempty"`
	UserOrganizations  []UserOrganization   `gorm:"foreignKey:OrganizationID" json:"user_organizations,omitempty"`
	WhatsAppAccounts   []WhatsAppAccount    `gorm:"foreignKey:OrganizationID" json:"whatsapp_accounts,omitempty"`
}

func (Organization) TableName() string {
	return "organizations"
}

// User represents a user in the system
type User struct {
	BaseModel
	OrganizationID uuid.UUID  `gorm:"type:uuid;index" json:"organization_id"`
	Email          string     `gorm:"size:255;uniqueIndex;not null" json:"email"`
	PasswordHash   string     `gorm:"size:255" json:"-"`
	FullName       string     `gorm:"size:255" json:"full_name"`
	RoleID         *uuid.UUID `gorm:"type:uuid;index" json:"role_id,omitempty"`
	Settings       JSONB      `gorm:"type:jsonb;default:'{}'" json:"settings"`
	IsActive       bool       `gorm:"default:true" json:"is_active"`
	IsAvailable    bool       `gorm:"default:true" json:"is_available"` // Agent availability status (away/available)
	IsSuperAdmin   bool       `gorm:"default:false" json:"is_super_admin"`  // Super admin can access all organizations

	// SSO fields
	SSOProvider   string `gorm:"size:50" json:"sso_provider,omitempty"`     // google, microsoft, github, facebook, custom
	SSOProviderID string `gorm:"size:255" json:"sso_provider_id,omitempty"` // External user ID from provider

	// Relations
	Organization      *Organization      `gorm:"foreignKey:OrganizationID" json:"organization,omitempty"`
	Role              *CustomRole        `gorm:"foreignKey:RoleID" json:"role,omitempty"`
	UserOrganizations []UserOrganization `gorm:"foreignKey:UserID" json:"user_organizations,omitempty"`
}

func (User) TableName() string {
	return "users"
}

// UserOrganization represents a many-to-many relationship between users and organizations
type UserOrganization struct {
	BaseModel
	UserID         uuid.UUID  `gorm:"type:uuid;uniqueIndex:idx_user_org;not null" json:"user_id"`
	OrganizationID uuid.UUID  `gorm:"type:uuid;uniqueIndex:idx_user_org;not null" json:"organization_id"`
	RoleID         *uuid.UUID `gorm:"type:uuid;index" json:"role_id,omitempty"`
	IsDefault      bool       `gorm:"default:false" json:"is_default"`

	// Relations
	User         *User         `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Organization *Organization `gorm:"foreignKey:OrganizationID" json:"organization,omitempty"`
	Role         *CustomRole   `gorm:"foreignKey:RoleID" json:"role,omitempty"`
}

func (UserOrganization) TableName() string {
	return "user_organizations"
}

// UserAvailabilityLog tracks user availability changes for break time calculation
type UserAvailabilityLog struct {
	ID             uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID         uuid.UUID  `gorm:"type:uuid;index;not null" json:"user_id"`
	OrganizationID uuid.UUID  `gorm:"type:uuid;index;not null" json:"organization_id"`
	IsAvailable    bool       `gorm:"not null" json:"is_available"`
	StartedAt      time.Time  `gorm:"not null" json:"started_at"`
	EndedAt        *time.Time `json:"ended_at,omitempty"` // null means current status

	// Relations
	User *User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (UserAvailabilityLog) TableName() string {
	return "user_availability_logs"
}

// Team represents a group of agents handling specific types of chats
type Team struct {
	BaseModel
	OrganizationID     uuid.UUID `gorm:"type:uuid;index;not null" json:"organization_id"`
	Name               string    `gorm:"size:100;not null" json:"name"`
	Description        string    `gorm:"size:500" json:"description"`
	AssignmentStrategy AssignmentStrategy `gorm:"size:50;default:'round_robin'" json:"assignment_strategy"` // round_robin, load_balanced, manual
	IsActive           bool      `gorm:"default:true" json:"is_active"`

	// Relations
	Organization *Organization `gorm:"foreignKey:OrganizationID" json:"organization,omitempty"`
	Members      []TeamMember  `gorm:"foreignKey:TeamID" json:"members,omitempty"`
}

func (Team) TableName() string {
	return "teams"
}

// TeamMember represents a user's membership in a team
type TeamMember struct {
	BaseModel
	TeamID         uuid.UUID  `gorm:"type:uuid;index;not null" json:"team_id"`
	UserID         uuid.UUID  `gorm:"type:uuid;index;not null" json:"user_id"`
	Role           TeamRole   `gorm:"size:50;default:'agent'" json:"role"` // manager, agent
	LastAssignedAt *time.Time `json:"last_assigned_at,omitempty"`          // For round-robin tracking

	// Relations
	Team *Team `gorm:"foreignKey:TeamID" json:"team,omitempty"`
	User *User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (TeamMember) TableName() string {
	return "team_members"
}

// APIKey represents an API key for programmatic access
type APIKey struct {
	BaseModel
	OrganizationID uuid.UUID  `gorm:"type:uuid;index;not null" json:"organization_id"`
	UserID         uuid.UUID  `gorm:"type:uuid;index;not null" json:"user_id"` // Creator
	Name           string     `gorm:"size:255;not null" json:"name"`
	KeyPrefix      string     `gorm:"size:8;index" json:"key_prefix"` // First 8 chars for identification
	KeyHash        string     `gorm:"size:255;not null" json:"-"`     // bcrypt hash of full key
	LastUsedAt     *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"` // null = never expires
	IsActive       bool       `gorm:"default:true" json:"is_active"`

	// Relations
	Organization *Organization `gorm:"foreignKey:OrganizationID" json:"organization,omitempty"`
	User         *User         `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (APIKey) TableName() string {
	return "api_keys"
}

// SSOProvider represents an SSO/OAuth provider configuration for an organization
type SSOProvider struct {
	BaseModel
	OrganizationID  uuid.UUID `gorm:"type:uuid;index;not null" json:"organization_id"`
	Provider        string    `gorm:"size:50;not null" json:"provider"` // google, microsoft, github, facebook, custom
	ClientID        string    `gorm:"size:500;not null" json:"client_id"`
	ClientSecret    string    `gorm:"size:500;not null" json:"-"` // Never exposed in JSON
	IsEnabled       bool   `gorm:"default:false" json:"is_enabled"`
	AllowAutoCreate bool   `gorm:"default:false" json:"allow_auto_create"`         // Auto-create new users on SSO login
	DefaultRoleName string `gorm:"size:50;default:'agent'" json:"default_role"`    // Role name for auto-created users (references CustomRole.Name)
	AllowedDomains  string    `gorm:"type:text" json:"allowed_domains,omitempty"` // Comma-separated email domains

	// Custom OIDC provider fields (only used when Provider = "custom")
	AuthURL     string `gorm:"size:500" json:"auth_url,omitempty"`
	TokenURL    string `gorm:"size:500" json:"token_url,omitempty"`
	UserInfoURL string `gorm:"size:500" json:"user_info_url,omitempty"`

	// Relations
	Organization *Organization `gorm:"foreignKey:OrganizationID" json:"organization,omitempty"`
}

func (SSOProvider) TableName() string {
	return "sso_providers"
}

// Webhook represents an outbound webhook configuration for integrations
type Webhook struct {
	BaseModel
	OrganizationID uuid.UUID   `gorm:"type:uuid;index;not null" json:"organization_id"`
	Name           string      `gorm:"size:255;not null" json:"name"`
	URL            string      `gorm:"type:text;not null" json:"url"`
	Events         StringArray `gorm:"type:jsonb;default:'[]'" json:"events"` // ["message.incoming", "transfer.created"]
	Headers        JSONB       `gorm:"type:jsonb;default:'{}'" json:"headers"`
	Secret         string      `gorm:"size:255" json:"-"` // For HMAC signature
	IsActive       bool        `gorm:"default:true" json:"is_active"`

	// Relations
	Organization *Organization `gorm:"foreignKey:OrganizationID" json:"organization,omitempty"`
}

func (Webhook) TableName() string {
	return "webhooks"
}

// CustomAction represents a custom action button for chat integrations
type CustomAction struct {
	BaseModel
	OrganizationID uuid.UUID `gorm:"type:uuid;index;not null" json:"organization_id"`
	Name           string    `gorm:"size:100;not null" json:"name"`
	Icon           string    `gorm:"size:50" json:"icon"`                      // lucide icon name
	ActionType     ActionType `gorm:"size:20;not null" json:"action_type"`     // webhook, url, javascript
	Config         JSONB     `gorm:"type:jsonb;default:'{}'" json:"config"`    // Type-specific configuration
	IsActive       bool      `gorm:"default:true" json:"is_active"`
	DisplayOrder   int       `gorm:"default:0" json:"display_order"`

	// Relations
	Organization *Organization `gorm:"foreignKey:OrganizationID" json:"organization,omitempty"`
}

func (CustomAction) TableName() string {
	return "custom_actions"
}

// WhatsAppAccount represents a WhatsApp Business Account
type WhatsAppAccount struct {
	BaseModel
	OrganizationID     uuid.UUID `gorm:"type:uuid;index;not null" json:"organization_id"`
	Name               string    `gorm:"size:100;uniqueIndex:idx_wa_org_name;not null" json:"name"` // Unique per org, used as reference
	AppID              string    `gorm:"size:100" json:"app_id"`                                    // Meta App ID
	PhoneID            string    `gorm:"size:100;not null" json:"phone_id"`
	BusinessID         string    `gorm:"size:100;not null" json:"business_id"`
	AccessToken        string    `gorm:"type:text;not null" json:"-"` // encrypted
	AppSecret          string    `gorm:"size:255" json:"-"`           // Meta App Secret for webhook signature verification
	WebhookVerifyToken string    `gorm:"size:255" json:"webhook_verify_token"`
	APIVersion         string    `gorm:"size:20;default:'v21.0'" json:"api_version"`
	IsDefaultIncoming  bool      `gorm:"default:false" json:"is_default_incoming"`
	IsDefaultOutgoing  bool      `gorm:"default:false" json:"is_default_outgoing"`
	AutoReadReceipt    bool      `gorm:"default:false" json:"auto_read_receipt"`
	Status             string    `gorm:"size:20;default:'active'" json:"status"`

	// Relations
	Organization *Organization `gorm:"foreignKey:OrganizationID" json:"organization,omitempty"`
}

func (WhatsAppAccount) TableName() string {
	return "whatsapp_accounts"
}

// Contact represents a WhatsApp contact/profile
type Contact struct {
	BaseModel
	OrganizationID     uuid.UUID  `gorm:"type:uuid;index;not null" json:"organization_id"`
	PhoneNumber        string     `gorm:"size:50;not null" json:"phone_number"`
	ProfileName        string     `gorm:"size:255" json:"profile_name"`
	WhatsAppAccount    string     `gorm:"size:100;index" json:"whatsapp_account"` // References WhatsAppAccount.Name
	AssignedUserID     *uuid.UUID `gorm:"type:uuid;index" json:"assigned_user_id,omitempty"`
	LastMessageAt      *time.Time `json:"last_message_at,omitempty"`
	LastMessagePreview string     `gorm:"type:text" json:"last_message_preview"`
	IsRead             bool       `gorm:"default:true" json:"is_read"`
	Tags               JSONBArray `gorm:"type:jsonb;default:'[]'" json:"tags"`
	Metadata           JSONB      `gorm:"type:jsonb;default:'{}'" json:"metadata"`

	// Chatbot SLA tracking
	ChatbotLastMessageAt *time.Time `json:"chatbot_last_message_at,omitempty"` // When chatbot last sent a message
	ChatbotReminderSent  bool       `gorm:"default:false" json:"chatbot_reminder_sent"`

	// Relations
	Organization *Organization `gorm:"foreignKey:OrganizationID" json:"organization,omitempty"`
	AssignedUser *User         `gorm:"foreignKey:AssignedUserID" json:"assigned_user,omitempty"`
	Messages     []Message     `gorm:"foreignKey:ContactID" json:"messages,omitempty"`
}

func (Contact) TableName() string {
	return "contacts"
}

// Message represents a WhatsApp message
type Message struct {
	BaseModel
	OrganizationID    uuid.UUID  `gorm:"type:uuid;index;not null" json:"organization_id"`
	WhatsAppAccount   string     `gorm:"size:100;index;not null" json:"whatsapp_account"` // References WhatsAppAccount.Name
	ContactID         uuid.UUID  `gorm:"type:uuid;index;not null" json:"contact_id"`
	WhatsAppMessageID string     `gorm:"column:whats_app_message_id;size:255;index" json:"whatsapp_message_id"`
	ConversationID    string     `gorm:"size:255;index" json:"conversation_id"`
	Direction         Direction   `gorm:"size:10;not null" json:"direction"`
	MessageType       MessageType `gorm:"size:20;not null" json:"message_type"`
	Content           string     `gorm:"type:text" json:"content"`
	MediaURL          string     `gorm:"type:text" json:"media_url"`
	MediaMimeType     string     `gorm:"size:100" json:"media_mime_type"`
	MediaFilename     string     `gorm:"size:255" json:"media_filename"`
	TemplateName      string     `gorm:"size:255" json:"template_name"`
	TemplateParams    JSONB      `gorm:"type:jsonb" json:"template_params"`
	InteractiveData   JSONB      `gorm:"type:jsonb" json:"interactive_data"`
	FlowResponse      JSONB      `gorm:"type:jsonb" json:"flow_response"`
	Status            MessageStatus `gorm:"size:20;default:'pending'" json:"status"`
	ErrorMessage      string     `gorm:"type:text" json:"error_message"`
	IsReply           bool       `gorm:"default:false" json:"is_reply"`
	ReplyToMessageID  *uuid.UUID `gorm:"type:uuid" json:"reply_to_message_id,omitempty"`
	SentByUserID      *uuid.UUID `gorm:"type:uuid;index" json:"sent_by_user_id,omitempty"` // User who sent outgoing message
	Metadata          JSONB      `gorm:"type:jsonb;default:'{}'" json:"metadata"`

	// Relations
	Organization   *Organization `gorm:"foreignKey:OrganizationID" json:"organization,omitempty"`
	Contact        *Contact      `gorm:"foreignKey:ContactID" json:"contact,omitempty"`
	ReplyToMessage *Message      `gorm:"foreignKey:ReplyToMessageID" json:"reply_to_message,omitempty"`
	SentByUser     *User         `gorm:"foreignKey:SentByUserID" json:"sent_by_user,omitempty"`
}

func (Message) TableName() string {
	return "messages"
}

// Template represents a WhatsApp message template
type Template struct {
	BaseModel
	OrganizationID  uuid.UUID  `gorm:"type:uuid;index;not null" json:"organization_id"`
	WhatsAppAccount string     `gorm:"size:100;index;not null" json:"whatsapp_account"` // References WhatsAppAccount.Name
	MetaTemplateID  string     `gorm:"size:100" json:"meta_template_id"`
	Name            string     `gorm:"size:255;not null" json:"name"`
	DisplayName     string     `gorm:"size:255" json:"display_name"`
	Language        string     `gorm:"size:10;not null" json:"language"`
	Category        string     `gorm:"size:50" json:"category"`                       // MARKETING, UTILITY, AUTHENTICATION
	Status          string     `gorm:"size:20;default:'PENDING'" json:"status"`       // PENDING, APPROVED, REJECTED
	HeaderType      string     `gorm:"size:20" json:"header_type"`                    // TEXT, IMAGE, DOCUMENT, VIDEO
	HeaderContent   string     `gorm:"type:text" json:"header_content"`
	BodyContent     string     `gorm:"type:text;not null" json:"body_content"`
	FooterContent   string     `gorm:"type:text" json:"footer_content"`
	Buttons         JSONBArray  `gorm:"type:jsonb;default:'[]'" json:"buttons"`
	SampleValues    JSONBArray  `gorm:"type:jsonb;default:'[]'" json:"sample_values"`

	// Relations
	Organization *Organization `gorm:"foreignKey:OrganizationID" json:"organization,omitempty"`
}

func (Template) TableName() string {
	return "templates"
}

// WhatsAppFlow represents a WhatsApp interactive flow
type WhatsAppFlow struct {
	BaseModel
	OrganizationID  uuid.UUID  `gorm:"type:uuid;index;not null" json:"organization_id"`
	WhatsAppAccount string     `gorm:"size:100;index;not null" json:"whatsapp_account"` // References WhatsAppAccount.Name
	MetaFlowID      string     `gorm:"size:100" json:"meta_flow_id"`
	Name            string     `gorm:"size:255;not null" json:"name"`
	Status          string     `gorm:"size:20;default:'DRAFT'" json:"status"` // DRAFT, PUBLISHED, DEPRECATED, BLOCKED
	Category        string     `gorm:"size:50" json:"category"`
	JSONVersion     string     `gorm:"size:10;default:'6.0'" json:"json_version"`
	FlowJSON        JSONB      `gorm:"type:jsonb" json:"flow_json"`
	Screens         JSONBArray `gorm:"type:jsonb;default:'[]'" json:"screens"`
	PreviewURL      string     `gorm:"type:text" json:"preview_url"`
	HasLocalChanges bool       `gorm:"default:true" json:"has_local_changes"` // True when local changes need to be synced to Meta

	// Relations
	Organization *Organization `gorm:"foreignKey:OrganizationID" json:"organization,omitempty"`
}

func (WhatsAppFlow) TableName() string {
	return "whatsapp_flows"
}

// Widget represents a customizable analytics widget on the dashboard
type Widget struct {
	BaseModel
	OrganizationID uuid.UUID  `gorm:"type:uuid;index;not null" json:"organization_id"`
	UserID         *uuid.UUID `gorm:"type:uuid;index" json:"user_id"` // Creator of the widget (nil for system defaults)
	Name           string     `gorm:"size:255;not null" json:"name"`
	Description    string     `gorm:"type:text" json:"description"`
	DataSource     string     `gorm:"size:50;not null" json:"data_source"` // messages, contacts, campaigns, transfers, sessions
	Metric         string     `gorm:"size:20;not null" json:"metric"`      // count, sum, avg
	Field          string     `gorm:"size:100" json:"field"`               // Field for sum/avg (e.g., resolution_time)
	Filters        JSONBArray `gorm:"type:jsonb;default:'[]'" json:"filters"`
	DisplayType    string     `gorm:"size:20;default:'number'" json:"display_type"` // number, percentage, chart
	ChartType      string     `gorm:"size:20" json:"chart_type"`                    // line, bar, pie (when display_type is chart)
	GroupByField   string     `gorm:"size:100" json:"group_by_field"`               // Field to group by (e.g., status, direction)
	ShowChange     bool       `gorm:"default:true" json:"show_change"`              // Show % change vs previous period
	Color          string     `gorm:"size:20" json:"color"`                         // Widget color theme
	Size           string     `gorm:"size:10;default:'small'" json:"size"`          // small, medium, large
	DisplayOrder   int        `gorm:"default:0" json:"display_order"`
	GridX          int        `gorm:"default:0" json:"grid_x"`
	GridY          int        `gorm:"default:0" json:"grid_y"`
	GridW          int        `gorm:"default:0" json:"grid_w"`
	GridH          int        `gorm:"default:0" json:"grid_h"`
	Config         JSONB      `gorm:"type:jsonb;default:'{}'" json:"config"`
	IsShared       bool       `gorm:"default:false" json:"is_shared"` // Visible to entire org or just creator
	IsDefault      bool       `gorm:"default:false" json:"is_default"` // System default widget

	// Relations
	Organization *Organization `gorm:"foreignKey:OrganizationID" json:"organization,omitempty"`
	User         *User         `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (Widget) TableName() string {
	return "widgets"
}

// WidgetFilter represents a filter condition for a dashboard widget
type WidgetFilter struct {
	Field    string `json:"field"`    // status, direction, type, etc.
	Operator string `json:"operator"` // equals, not_equals, contains, gt, lt, gte, lte
	Value    string `json:"value"`
}
