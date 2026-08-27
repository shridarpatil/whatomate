package models

import (
	"time"

	"github.com/google/uuid"
)

// OccurrenceEventType identifies what a timeline entry records.
type OccurrenceEventType string

const (
	OccurrenceEventOpened       OccurrenceEventType = "opened"
	OccurrenceEventNote         OccurrenceEventType = "note"
	OccurrenceEventStageChange  OccurrenceEventType = "stage_change"
	OccurrenceEventAssignment   OccurrenceEventType = "assignment"
	OccurrenceEventProtocolSent OccurrenceEventType = "protocol_sent"
	OccurrenceEventClosed       OccurrenceEventType = "closed"
)

// OccurrencePriority ranks how urgent a case is.
type OccurrencePriority string

const (
	OccurrencePriorityLow    OccurrencePriority = "low"
	OccurrencePriorityNormal OccurrencePriority = "normal"
	OccurrencePriorityHigh   OccurrencePriority = "high"
	OccurrencePriorityUrgent OccurrencePriority = "urgent"
)

// OccurrenceStage is one configurable column of the org's pipeline.
type OccurrenceStage struct {
	BaseModel
	OrganizationID uuid.UUID `gorm:"type:uuid;index;not null;uniqueIndex:idx_occ_stage_org_name" json:"organization_id"`
	Name           string    `gorm:"size:100;not null;uniqueIndex:idx_occ_stage_org_name" json:"name"`
	Color          string    `gorm:"size:20;default:'#6b7280'" json:"color"`
	Position       int       `gorm:"not null;default:0" json:"position"`
	IsInitial      bool      `gorm:"default:false" json:"is_initial"`
	IsClosing      bool      `gorm:"default:false" json:"is_closing"`
}

func (OccurrenceStage) TableName() string { return "occurrence_stages" }

// Occurrence is one tracked case for a contact. Its ProtocolNumber is the
// human-readable identifier the customer keeps and quotes back.
type Occurrence struct {
	BaseModel
	OrganizationID uuid.UUID `gorm:"type:uuid;index;not null;uniqueIndex:idx_occ_org_protocol" json:"organization_id"`
	ContactID      uuid.UUID `gorm:"type:uuid;index;not null" json:"contact_id"`

	// Total unique index, deliberately NOT partial on soft delete: a deleted
	// protocol must never be reissued, because the customer still has the
	// number written down.
	ProtocolNumber string `gorm:"size:20;not null;uniqueIndex:idx_occ_org_protocol" json:"protocol_number"`

	Title       string             `gorm:"size:255;not null" json:"title"`
	Description string             `gorm:"type:text" json:"description"`
	StageID     uuid.UUID          `gorm:"type:uuid;index;not null" json:"stage_id"`
	Priority    OccurrencePriority `gorm:"size:20;not null;default:'normal'" json:"priority"`

	AssignedUserID *uuid.UUID `gorm:"type:uuid;index" json:"assigned_user_id,omitempty"`
	TeamID         *uuid.UUID `gorm:"type:uuid;index" json:"team_id,omitempty"`
	OpenedByUserID uuid.UUID  `gorm:"type:uuid;not null" json:"opened_by_user_id"`

	OpenedAt time.Time  `gorm:"autoCreateTime" json:"opened_at"`
	ClosedAt *time.Time `json:"closed_at,omitempty"`

	// SourceTransferID records which attendance spawned this occurrence. It is
	// traceability only — the lifecycles stay independent, so closing the chat
	// attendance never closes the occurrence.
	SourceTransferID *uuid.UUID `gorm:"type:uuid;index" json:"source_transfer_id,omitempty"`

	// Relations
	Contact      *Contact         `gorm:"foreignKey:ContactID" json:"contact,omitempty"`
	Stage        *OccurrenceStage `gorm:"foreignKey:StageID" json:"stage,omitempty"`
	AssignedUser *User            `gorm:"foreignKey:AssignedUserID" json:"assigned_user,omitempty"`
}

func (Occurrence) TableName() string { return "occurrences" }

// OccurrenceEvent is one entry on an occurrence's timeline. Manual notes and
// automatic events share this table on purpose: two tables drift, and a
// timeline built from two queries has to interleave them anyway.
type OccurrenceEvent struct {
	BaseModel
	OrganizationID uuid.UUID           `gorm:"type:uuid;index;not null" json:"organization_id"`
	OccurrenceID   uuid.UUID           `gorm:"type:uuid;index;not null" json:"occurrence_id"`
	Type           OccurrenceEventType `gorm:"size:30;not null" json:"type"`
	Content        string              `gorm:"type:text" json:"content"`
	Metadata       JSONB               `gorm:"type:jsonb;default:'{}'" json:"metadata"`
	CreatedByID    *uuid.UUID          `gorm:"type:uuid" json:"created_by_id,omitempty"` // nil = system

	CreatedBy *User `gorm:"foreignKey:CreatedByID" json:"created_by,omitempty"`
}

func (OccurrenceEvent) TableName() string { return "occurrence_events" }

// OccurrenceCounter holds the per-org, per-year protocol sequence. It does not
// embed BaseModel: the natural key is (organization_id, year), and a uuid PK
// would invite a second row for the same pair.
type OccurrenceCounter struct {
	OrganizationID uuid.UUID `gorm:"type:uuid;primaryKey" json:"organization_id"`
	Year           int       `gorm:"primaryKey" json:"year"`
	LastSeq        int       `gorm:"not null;default:0" json:"last_seq"`
}

func (OccurrenceCounter) TableName() string { return "occurrence_counters" }
