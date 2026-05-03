package queue

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
)

// JobType represents the type of job
type JobType string

const (
	// JobTypeRecipient is for processing a single recipient message
	JobTypeRecipient JobType = "recipient"
)

// RecipientJob represents a single recipient message job
type RecipientJob struct {
	CampaignID     uuid.UUID    `json:"campaign_id"`
	RecipientID    uuid.UUID    `json:"recipient_id"`
	OrganizationID uuid.UUID    `json:"organization_id"`
	PhoneNumber    string       `json:"phone_number"`
	RecipientName  string       `json:"recipient_name"`
	TemplateParams models.JSONB `json:"template_params"`
	EnqueuedAt     time.Time    `json:"enqueued_at"`
}

// Queue defines the interface for job queue operations
type Queue interface {
	// EnqueueRecipient adds a single recipient job to the queue
	EnqueueRecipient(ctx context.Context, job *RecipientJob) error

	// EnqueueRecipients adds multiple recipient jobs to the queue
	EnqueueRecipients(ctx context.Context, jobs []*RecipientJob) error

	// Close closes the queue connection
	Close() error
}

// JobHandler handles different job types
type JobHandler interface {
	HandleRecipientJob(ctx context.Context, job *RecipientJob) error
}

// Consumer defines the interface for consuming jobs from the queue
type Consumer interface {
	// Consume starts consuming jobs from the queue
	// Returns when context is cancelled
	Consume(ctx context.Context, handler JobHandler) error

	// Close closes the consumer connection
	Close() error
}
