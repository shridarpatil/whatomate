package models

import (
	"time"

	"github.com/google/uuid"
)

const CampaignSourceRSVPReminder = "rsvp_reminder"
const CampaignSourceRSVPFollowUp = "rsvp_followup"

// BulkMessageCampaign represents a bulk message campaign
type BulkMessageCampaign struct {
	BaseModel
	OrganizationID       uuid.UUID      `gorm:"type:uuid;index;not null" json:"organization_id"`
	WhatsAppAccount      string         `gorm:"size:100;index;not null" json:"whatsapp_account"` // References WhatsAppAccount.Name
	Name                 string         `gorm:"size:255;not null" json:"name"`
	TemplateID           uuid.UUID      `gorm:"type:uuid;not null" json:"template_id"`
	HeaderMediaURL       string         `gorm:"type:text" json:"header_media_url"`        // Source URL used to fetch campaign header media
	HeaderMediaID        string         `gorm:"type:text" json:"header_media_id"`         // Meta media ID (from uploaded media)
	HeaderMediaFilename  string         `gorm:"type:text" json:"header_media_filename"`   // Original filename
	HeaderMediaMimeType  string         `gorm:"type:text" json:"header_media_mime_type"`  // MIME type (image/jpeg, video/mp4, etc.)
	HeaderMediaLocalPath string         `gorm:"type:text" json:"header_media_local_path"` // Local file path for preview
	Status               CampaignStatus `gorm:"size:20;default:'draft'" json:"status"`    // draft, queued, processing, completed, failed
	// PauseReason explains an automatic pause (e.g. WhatsApp delivery reports
	// not reaching the app). Empty for manual pauses; cleared on start/resume.
	PauseReason string `gorm:"type:text" json:"pause_reason,omitempty"`
	TotalRecipients      int            `gorm:"default:0" json:"total_recipients"`
	SentCount            int            `gorm:"default:0" json:"sent_count"`
	DeliveredCount       int            `gorm:"default:0" json:"delivered_count"`
	ReadCount            int            `gorm:"default:0" json:"read_count"`
	FailedCount          int            `gorm:"default:0" json:"failed_count"`
	ScheduledAt          *time.Time     `json:"scheduled_at,omitempty"`
	StartedAt            *time.Time     `json:"started_at,omitempty"`
	CompletedAt          *time.Time     `json:"completed_at,omitempty"`
	CreatedBy            uuid.UUID      `gorm:"type:uuid;not null" json:"created_by"`
	SourceType           string         `gorm:"size:50;index" json:"source_type,omitempty"`
	SourceID             *uuid.UUID     `gorm:"type:uuid;index" json:"source_id,omitempty"`
	// FlowID records which chatbot flow a source-specific send should hand a
	// recipient into when they tap through (e.g. an RSVP follow-up's chosen
	// flow). Unset for campaign types with no follow-up flow of their own.
	FlowID *uuid.UUID `gorm:"type:uuid" json:"flow_id,omitempty"`
	UpdatedByID          *uuid.UUID     `gorm:"type:uuid" json:"updated_by_id,omitempty"`

	// Relations
	Organization *Organization          `gorm:"foreignKey:OrganizationID" json:"organization,omitempty"`
	Template     *Template              `gorm:"foreignKey:TemplateID" json:"template,omitempty"`
	Creator      *User                  `gorm:"foreignKey:CreatedBy" json:"creator,omitempty"`
	UpdatedBy    *User                  `gorm:"foreignKey:UpdatedByID" json:"updated_by,omitempty"`
	Recipients   []BulkMessageRecipient `gorm:"foreignKey:CampaignID" json:"recipients,omitempty"`
}

func (BulkMessageCampaign) TableName() string {
	return "bulk_message_campaigns"
}

// BulkMessageRecipient represents a recipient in a bulk message campaign
type BulkMessageRecipient struct {
	BaseModel
	CampaignID     uuid.UUID `gorm:"type:uuid;index;not null" json:"campaign_id"`
	PhoneNumber    string    `gorm:"size:50;not null" json:"phone_number"`
	RecipientName  string    `gorm:"size:255" json:"recipient_name"`
	TemplateParams JSONB     `gorm:"type:jsonb;default:'{}'" json:"template_params"`
	// Header parameter values for TEXT-header templates with a {{var}}. Meta
	// indexes positional vars per component, so header {{1}} and body {{1}}
	// are separate values — keeping them in a dedicated map avoids that
	// collision. Empty when the template has no header variable.
	HeaderParams      JSONB         `gorm:"type:jsonb;default:'{}'" json:"header_params"`
	Status            MessageStatus `gorm:"size:20;default:'pending'" json:"status"` // pending, sent, delivered, read, failed
	WhatsAppMessageID string        `gorm:"column:whats_app_message_id;size:100;index" json:"whatsapp_message_id,omitempty"`
	MessageID         *uuid.UUID    `gorm:"type:uuid" json:"message_id,omitempty"`
	ErrorMessage      string        `gorm:"type:text" json:"error_message"`
	SentAt            *time.Time    `json:"sent_at,omitempty"`
	DeliveredAt       *time.Time    `json:"delivered_at,omitempty"`
	ReadAt            *time.Time    `json:"read_at,omitempty"`

	// Relations
	Campaign *BulkMessageCampaign `gorm:"foreignKey:CampaignID" json:"campaign,omitempty"`
	Message  *Message             `gorm:"foreignKey:MessageID" json:"message,omitempty"`
}

func (BulkMessageRecipient) TableName() string {
	return "bulk_message_recipients"
}

// NotificationRule defines automated notification rules
type NotificationRule struct {
	BaseModel
	OrganizationID   uuid.UUID `gorm:"type:uuid;index;not null" json:"organization_id"`
	WhatsAppAccount  string    `gorm:"size:100;index;not null" json:"whatsapp_account"` // References WhatsAppAccount.Name
	Name             string    `gorm:"size:255;not null" json:"name"`
	IsEnabled        bool      `gorm:"default:true" json:"is_enabled"`
	TriggerType      string    `gorm:"size:50;not null" json:"trigger_type"` // webhook, scheduler, api
	TriggerConfig    JSONB     `gorm:"type:jsonb;not null" json:"trigger_config"`
	TemplateID       uuid.UUID `gorm:"type:uuid;not null" json:"template_id"`
	FieldMappings    JSONB     `gorm:"type:jsonb;default:'{}'" json:"field_mappings"`
	Conditions       JSONB     `gorm:"type:jsonb;default:'{}'" json:"conditions"`
	AttachmentConfig JSONB     `gorm:"type:jsonb" json:"attachment_config"`

	// Relations
	Organization *Organization `gorm:"foreignKey:OrganizationID" json:"organization,omitempty"`
	Template     *Template     `gorm:"foreignKey:TemplateID" json:"template,omitempty"`
}

func (NotificationRule) TableName() string {
	return "notification_rules"
}
