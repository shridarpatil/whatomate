package models

import (
	"time"

	"github.com/google/uuid"
)

// CallDirection represents the direction of a voice call
type CallDirection string

const (
	CallDirectionIncoming CallDirection = "incoming"
	CallDirectionOutgoing CallDirection = "outgoing"
)

// CallStatus represents the status of a voice call
type CallStatus string

const (
	CallStatusRinging      CallStatus = "ringing"
	CallStatusAnswered     CallStatus = "answered"
	CallStatusCompleted    CallStatus = "completed"
	CallStatusMissed       CallStatus = "missed"
	CallStatusRejected     CallStatus = "rejected"
	CallStatusFailed       CallStatus = "failed"
	CallStatusTransferring CallStatus = "transferring"
	CallStatusInitiating   CallStatus = "initiating" // outgoing: offer sent, awaiting WA answer
	CallStatusAccepted     CallStatus = "accepted"   // outgoing: consumer picked up
)

// CallLog represents a voice call record
type CallLog struct {
	BaseModel
	OrganizationID  uuid.UUID     `gorm:"type:uuid;not null;index" json:"organization_id"`
	WhatsAppAccount string        `gorm:"column:whatsapp_account;size:100;not null" json:"whatsapp_account"`
	ContactID       uuid.UUID     `gorm:"type:uuid;index" json:"contact_id"`
	WhatsAppCallID  string        `gorm:"column:whatsapp_call_id;size:255;index" json:"whatsapp_call_id"`
	CallerPhone     string        `gorm:"size:50;not null" json:"caller_phone"`
	Direction       CallDirection `gorm:"size:20;not null;default:'incoming'" json:"direction"`
	Status          CallStatus    `gorm:"size:20;not null;default:'ringing'" json:"status"`
	Duration        int           `gorm:"default:0" json:"duration"`
	IVRFlowID       *uuid.UUID    `gorm:"type:uuid" json:"ivr_flow_id,omitempty"`
	IVRPath         JSONB         `gorm:"type:jsonb" json:"ivr_path,omitempty"`
	AgentID         *uuid.UUID    `gorm:"type:uuid" json:"agent_id,omitempty"`
	StartedAt       *time.Time    `json:"started_at,omitempty"`
	AnsweredAt      *time.Time    `json:"answered_at,omitempty"`
	EndedAt         *time.Time    `json:"ended_at,omitempty"`
	ErrorMessage      string        `gorm:"type:text" json:"error_message,omitempty"`
	RecordingS3Key    string        `gorm:"size:500" json:"recording_s3_key,omitempty"`
	RecordingDuration int           `gorm:"default:0" json:"recording_duration,omitempty"`

	// Relations
	Contact *Contact `gorm:"foreignKey:ContactID" json:"contact,omitempty"`
	IVRFlow *IVRFlow `gorm:"foreignKey:IVRFlowID" json:"ivr_flow,omitempty"`
	Agent   *User    `gorm:"foreignKey:AgentID" json:"agent,omitempty"`
}

func (CallLog) TableName() string {
	return "call_logs"
}

// IVRFlow represents an IVR (Interactive Voice Response) menu flow
type IVRFlow struct {
	BaseModel
	OrganizationID  uuid.UUID `gorm:"type:uuid;not null;index" json:"organization_id"`
	WhatsAppAccount string    `gorm:"column:whatsapp_account;size:100;not null" json:"whatsapp_account"`
	Name            string    `gorm:"size:255;not null" json:"name"`
	Description     string    `gorm:"type:text" json:"description"`
	IsActive        bool      `gorm:"default:true" json:"is_active"`
	IsCallStart     bool      `gorm:"default:false" json:"is_call_start"`
	Menu            JSONB     `gorm:"type:jsonb" json:"menu"`
	WelcomeAudioURL string    `gorm:"type:text" json:"welcome_audio_url"`

	// Relations
	Organization *Organization `gorm:"foreignKey:OrganizationID" json:"organization,omitempty"`
}

func (IVRFlow) TableName() string {
	return "ivr_flows"
}

// CallTransferStatus represents the status of a call transfer
type CallTransferStatus string

const (
	CallTransferStatusWaiting   CallTransferStatus = "waiting"
	CallTransferStatusConnected CallTransferStatus = "connected"
	CallTransferStatusCompleted CallTransferStatus = "completed"
	CallTransferStatusAbandoned CallTransferStatus = "abandoned"
	CallTransferStatusNoAnswer  CallTransferStatus = "no_answer"
)

// CallTransfer represents a call being transferred from IVR to an agent
type CallTransfer struct {
	BaseModel
	OrganizationID  uuid.UUID          `gorm:"type:uuid;not null;index" json:"organization_id"`
	CallLogID       uuid.UUID          `gorm:"type:uuid;not null;index" json:"call_log_id"`
	WhatsAppCallID  string             `gorm:"size:255;not null;index" json:"whatsapp_call_id"`
	CallerPhone     string             `gorm:"size:50;not null" json:"caller_phone"`
	ContactID       uuid.UUID          `gorm:"type:uuid;index" json:"contact_id"`
	WhatsAppAccount string             `gorm:"size:100;not null" json:"whatsapp_account"`
	Status          CallTransferStatus `gorm:"size:20;not null;default:'waiting'" json:"status"`
	TeamID          *uuid.UUID         `gorm:"type:uuid;index" json:"team_id,omitempty"`
	AgentID         *uuid.UUID         `gorm:"type:uuid" json:"agent_id,omitempty"`
	TransferredAt   time.Time          `gorm:"autoCreateTime" json:"transferred_at"`
	ConnectedAt     *time.Time         `json:"connected_at,omitempty"`
	CompletedAt     *time.Time         `json:"completed_at,omitempty"`
	HoldDuration    int                `gorm:"default:0" json:"hold_duration"`
	TalkDuration    int                `gorm:"default:0" json:"talk_duration"`
	IVRPath         JSONB              `gorm:"type:jsonb" json:"ivr_path,omitempty"`
	// Relations
	CallLog *CallLog `gorm:"foreignKey:CallLogID" json:"call_log,omitempty"`
	Contact *Contact `gorm:"foreignKey:ContactID" json:"contact,omitempty"`
	Agent   *User    `gorm:"foreignKey:AgentID" json:"agent,omitempty"`
	Team    *Team    `gorm:"foreignKey:TeamID" json:"team,omitempty"`
}

func (CallTransfer) TableName() string {
	return "call_transfers"
}

// CallPermissionStatus represents the status of a call permission request
type CallPermissionStatus string

const (
	CallPermissionPending  CallPermissionStatus = "pending"
	CallPermissionAccepted CallPermissionStatus = "accepted"
	CallPermissionDeclined CallPermissionStatus = "declined"
	CallPermissionExpired  CallPermissionStatus = "expired"
)

// CallPermission tracks call permission requests sent to contacts
type CallPermission struct {
	BaseModel
	OrganizationID uuid.UUID            `gorm:"type:uuid;not null;index" json:"organization_id"`
	ContactID      uuid.UUID            `gorm:"type:uuid;not null;index" json:"contact_id"`
	WhatsAppAccount string              `gorm:"size:100;not null" json:"whatsapp_account"`
	Status         CallPermissionStatus `gorm:"size:20;not null;default:'pending'" json:"status"`
	MessageID      string               `gorm:"size:255" json:"message_id"`
	RequestedAt    time.Time            `gorm:"autoCreateTime" json:"requested_at"`
	RespondedAt    *time.Time           `json:"responded_at,omitempty"`
	ExpiresAt      *time.Time           `json:"expires_at,omitempty"`
	RequestedByID  *uuid.UUID           `gorm:"type:uuid" json:"requested_by_id,omitempty"`

	// Relations
	Contact     *Contact `gorm:"foreignKey:ContactID" json:"contact,omitempty"`
	RequestedBy *User    `gorm:"foreignKey:RequestedByID" json:"requested_by,omitempty"`
}

func (CallPermission) TableName() string {
	return "call_permissions"
}
