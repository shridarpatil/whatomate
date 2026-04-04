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

	// JobTypeEmail is for sending an email via the org's SMTP config
	JobTypeEmail JobType = "email"
)

// RecipientJob represents a single recipient message job
type RecipientJob struct {
	CampaignID     uuid.UUID     `json:"campaign_id"`
	RecipientID    uuid.UUID     `json:"recipient_id"`
	OrganizationID uuid.UUID     `json:"organization_id"`
	PhoneNumber    string        `json:"phone_number"`
	RecipientName  string        `json:"recipient_name"`
	TemplateParams models.JSONB  `json:"template_params"`
	EnqueuedAt     time.Time     `json:"enqueued_at"`
}

// EmailJob represents an email to be sent asynchronously via the org's SMTP settings.
type EmailJob struct {
	OrganizationID uuid.UUID      `json:"organization_id"`
	TemplateName   string         `json:"template_name"` // Template file name (e.g., "welcome.html")
	To             []string       `json:"to"`
	Subject        string         `json:"subject"`
	TemplateData   map[string]any `json:"template_data"`
	EnqueuedAt     time.Time      `json:"enqueued_at"`
}

// Queue defines the interface for job queue operations
type Queue interface {
	// EnqueueRecipient adds a single recipient job to the queue
	EnqueueRecipient(ctx context.Context, job *RecipientJob) error

	// EnqueueRecipients adds multiple recipient jobs to the queue
	EnqueueRecipients(ctx context.Context, jobs []*RecipientJob) error

	// EnqueueEmail adds an email job to the queue
	EnqueueEmail(ctx context.Context, job *EmailJob) error

	// Close closes the queue connection
	Close() error
}

// JobHandler handles different job types
type JobHandler interface {
	HandleRecipientJob(ctx context.Context, job *RecipientJob) error
	HandleEmailJob(ctx context.Context, job *EmailJob) error
}

// Consumer defines the interface for consuming jobs from the queue
type Consumer interface {
	// Consume starts consuming jobs from the queue
	// Returns when context is cancelled
	Consume(ctx context.Context, handler JobHandler) error

	// Close closes the consumer connection
	Close() error
}
