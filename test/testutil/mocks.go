package testutil

import (
	"context"
	"sync"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/queue"
	"github.com/shridarpatil/whatomate/pkg/whatsapp"
)

// MockSentMessage records a message sent through the mock WhatsApp client.
type MockSentMessage struct {
	Type        string
	PhoneNumber string
	Content     interface{}
	Account     *whatsapp.Account
	TemplateID  string
	MessageID   string
}

// MockWhatsAppClient is a mock implementation of WhatsApp client operations.
type MockWhatsAppClient struct {
	mu sync.Mutex

	// Recorded calls
	SentMessages []MockSentMessage

	// Configurable behavior
	SendTextMessageFunc                   func(ctx context.Context, account *whatsapp.Account, phone, text string) (string, error)
	SendInteractiveButtonsFunc            func(ctx context.Context, account *whatsapp.Account, phone, body string, buttons []whatsapp.Button) (string, error)
	SendTemplateMessageFunc               func(ctx context.Context, account *whatsapp.Account, phone, template, lang string, params []string) (string, error)
	SendTemplateMessageWithComponentsFunc func(ctx context.Context, account *whatsapp.Account, phone, template, lang string, components []map[string]interface{}) (string, error)
	SendImageMessageFunc                  func(ctx context.Context, account *whatsapp.Account, phone, mediaID, caption string) (string, error)
	SendDocumentMessageFunc               func(ctx context.Context, account *whatsapp.Account, phone, mediaID, filename, caption string) (string, error)
	MarkMessageReadFunc                   func(ctx context.Context, account *whatsapp.Account, messageID string) error
	GetMediaURLFunc                       func(ctx context.Context, mediaID string, account *whatsapp.Account) (string, error)
	DownloadMediaFunc                     func(ctx context.Context, mediaURL, accessToken string) ([]byte, error)
	UploadMediaFunc                       func(ctx context.Context, account *whatsapp.Account, data []byte, mimeType, filename string) (string, error)

	// Error to return (if set, overrides function)
	Error error

	// Counter for generating message IDs
	messageCounter int
}

// NewMockWhatsAppClient creates a new mock WhatsApp client.
func NewMockWhatsAppClient() *MockWhatsAppClient {
	return &MockWhatsAppClient{
		SentMessages: make([]MockSentMessage, 0),
	}
}

func (m *MockWhatsAppClient) nextMessageID() string {
	m.messageCounter++
	return "wamid.mock-" + uuid.New().String()[:8]
}

// SendTextMessage mocks sending a text message.
func (m *MockWhatsAppClient) SendTextMessage(ctx context.Context, account *whatsapp.Account, phone, text string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Error != nil {
		return "", m.Error
	}

	msgID := m.nextMessageID()
	m.SentMessages = append(m.SentMessages, MockSentMessage{
		Type:        "text",
		PhoneNumber: phone,
		Content:     text,
		Account:     account,
		MessageID:   msgID,
	})

	if m.SendTextMessageFunc != nil {
		return m.SendTextMessageFunc(ctx, account, phone, text)
	}
	return msgID, nil
}

// SendInteractiveButtons mocks sending an interactive message.
func (m *MockWhatsAppClient) SendInteractiveButtons(ctx context.Context, account *whatsapp.Account, phone, body string, buttons []whatsapp.Button) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Error != nil {
		return "", m.Error
	}

	msgID := m.nextMessageID()
	m.SentMessages = append(m.SentMessages, MockSentMessage{
		Type:        "interactive",
		PhoneNumber: phone,
		Content:     map[string]interface{}{"body": body, "buttons": buttons},
		Account:     account,
		MessageID:   msgID,
	})

	if m.SendInteractiveButtonsFunc != nil {
		return m.SendInteractiveButtonsFunc(ctx, account, phone, body, buttons)
	}
	return msgID, nil
}

// SendTemplateMessage mocks sending a template message.
func (m *MockWhatsAppClient) SendTemplateMessage(ctx context.Context, account *whatsapp.Account, phone, template, lang string, params []string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Error != nil {
		return "", m.Error
	}

	msgID := m.nextMessageID()
	m.SentMessages = append(m.SentMessages, MockSentMessage{
		Type:        "template",
		PhoneNumber: phone,
		Content:     map[string]interface{}{"template": template, "lang": lang, "params": params},
		Account:     account,
		TemplateID:  template,
		MessageID:   msgID,
	})

	if m.SendTemplateMessageFunc != nil {
		return m.SendTemplateMessageFunc(ctx, account, phone, template, lang, params)
	}
	return msgID, nil
}

// SendTemplateMessageWithComponents mocks sending a template message with components.
func (m *MockWhatsAppClient) SendTemplateMessageWithComponents(ctx context.Context, account *whatsapp.Account, phone, template, lang string, components []map[string]interface{}) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Error != nil {
		return "", m.Error
	}

	msgID := m.nextMessageID()
	m.SentMessages = append(m.SentMessages, MockSentMessage{
		Type:        "template",
		PhoneNumber: phone,
		Content:     map[string]interface{}{"template": template, "lang": lang, "components": components},
		Account:     account,
		TemplateID:  template,
		MessageID:   msgID,
	})

	if m.SendTemplateMessageWithComponentsFunc != nil {
		return m.SendTemplateMessageWithComponentsFunc(ctx, account, phone, template, lang, components)
	}
	return msgID, nil
}

// SendImageMessage mocks sending an image message.
func (m *MockWhatsAppClient) SendImageMessage(ctx context.Context, account *whatsapp.Account, phone, mediaID, caption string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Error != nil {
		return "", m.Error
	}

	msgID := m.nextMessageID()
	m.SentMessages = append(m.SentMessages, MockSentMessage{
		Type:        "image",
		PhoneNumber: phone,
		Content:     map[string]interface{}{"media_id": mediaID, "caption": caption},
		Account:     account,
		MessageID:   msgID,
	})

	if m.SendImageMessageFunc != nil {
		return m.SendImageMessageFunc(ctx, account, phone, mediaID, caption)
	}
	return msgID, nil
}

// SendDocumentMessage mocks sending a document message.
func (m *MockWhatsAppClient) SendDocumentMessage(ctx context.Context, account *whatsapp.Account, phone, mediaID, filename, caption string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Error != nil {
		return "", m.Error
	}

	msgID := m.nextMessageID()
	m.SentMessages = append(m.SentMessages, MockSentMessage{
		Type:        "document",
		PhoneNumber: phone,
		Content:     map[string]interface{}{"media_id": mediaID, "filename": filename, "caption": caption},
		Account:     account,
		MessageID:   msgID,
	})

	if m.SendDocumentMessageFunc != nil {
		return m.SendDocumentMessageFunc(ctx, account, phone, mediaID, filename, caption)
	}
	return msgID, nil
}

// MarkMessageRead mocks marking a message as read.
func (m *MockWhatsAppClient) MarkMessageRead(ctx context.Context, account *whatsapp.Account, messageID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Error != nil {
		return m.Error
	}

	if m.MarkMessageReadFunc != nil {
		return m.MarkMessageReadFunc(ctx, account, messageID)
	}
	return nil
}

// GetMediaURL mocks getting a media URL.
func (m *MockWhatsAppClient) GetMediaURL(ctx context.Context, mediaID string, account *whatsapp.Account) (string, error) {
	if m.Error != nil {
		return "", m.Error
	}
	if m.GetMediaURLFunc != nil {
		return m.GetMediaURLFunc(ctx, mediaID, account)
	}
	return "https://cdn.example.com/media/" + mediaID, nil
}

// DownloadMedia mocks downloading media.
func (m *MockWhatsAppClient) DownloadMedia(ctx context.Context, mediaURL, accessToken string) ([]byte, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	if m.DownloadMediaFunc != nil {
		return m.DownloadMediaFunc(ctx, mediaURL, accessToken)
	}
	return []byte("mock media content"), nil
}

// UploadMedia mocks uploading media.
func (m *MockWhatsAppClient) UploadMedia(ctx context.Context, account *whatsapp.Account, data []byte, mimeType, filename string) (string, error) {
	if m.Error != nil {
		return "", m.Error
	}
	if m.UploadMediaFunc != nil {
		return m.UploadMediaFunc(ctx, account, data, mimeType, filename)
	}
	return "media-" + uuid.New().String()[:8], nil
}

// Reset clears all recorded messages.
func (m *MockWhatsAppClient) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.SentMessages = m.SentMessages[:0]
	m.Error = nil
	m.messageCounter = 0
}

// MessageCount returns the number of messages sent.
func (m *MockWhatsAppClient) MessageCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.SentMessages)
}

// GetMessagesSentTo returns all messages sent to a specific phone number.
func (m *MockWhatsAppClient) GetMessagesSentTo(phone string) []MockSentMessage {
	m.mu.Lock()
	defer m.mu.Unlock()

	var messages []MockSentMessage
	for _, msg := range m.SentMessages {
		if msg.PhoneNumber == phone {
			messages = append(messages, msg)
		}
	}
	return messages
}

// MockQueue is a mock implementation of queue.Queue.
type MockQueue struct {
	mu   sync.Mutex
	Jobs []*queue.RecipientJob

	// Configurable behavior
	EnqueueFunc  func(ctx context.Context, job *queue.RecipientJob) error
	EnqueuesFunc func(ctx context.Context, jobs []*queue.RecipientJob) error

	// Error to return
	Error error
}

// NewMockQueue creates a new mock queue.
func NewMockQueue() *MockQueue {
	return &MockQueue{
		Jobs: make([]*queue.RecipientJob, 0),
	}
}

// EnqueueRecipient mocks enqueueing a single job.
func (m *MockQueue) EnqueueRecipient(ctx context.Context, job *queue.RecipientJob) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Error != nil {
		return m.Error
	}

	m.Jobs = append(m.Jobs, job)

	if m.EnqueueFunc != nil {
		return m.EnqueueFunc(ctx, job)
	}
	return nil
}

// EnqueueRecipients mocks enqueueing multiple jobs.
func (m *MockQueue) EnqueueRecipients(ctx context.Context, jobs []*queue.RecipientJob) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Error != nil {
		return m.Error
	}

	m.Jobs = append(m.Jobs, jobs...)

	if m.EnqueuesFunc != nil {
		return m.EnqueuesFunc(ctx, jobs)
	}
	return nil
}

// Close is a no-op for the mock.
func (m *MockQueue) Close() error {
	return nil
}

// JobCount returns the number of jobs in the queue.
func (m *MockQueue) JobCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.Jobs)
}

// GetJobs returns a copy of all jobs in the queue.
func (m *MockQueue) GetJobs() []*queue.RecipientJob {
	m.mu.Lock()
	defer m.mu.Unlock()

	jobs := make([]*queue.RecipientJob, len(m.Jobs))
	copy(jobs, m.Jobs)
	return jobs
}

// Reset clears all jobs from the queue.
func (m *MockQueue) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Jobs = m.Jobs[:0]
	m.Error = nil
}

// MockJobHandler is a mock implementation of queue.JobHandler.
type MockJobHandler struct {
	mu          sync.Mutex
	ProcessedCh chan *queue.RecipientJob
	Processed   []*queue.RecipientJob
	HandleFunc  func(ctx context.Context, job *queue.RecipientJob) error
	Error       error
}

// NewMockJobHandler creates a new mock job handler.
func NewMockJobHandler() *MockJobHandler {
	return &MockJobHandler{
		ProcessedCh: make(chan *queue.RecipientJob, 100),
		Processed:   make([]*queue.RecipientJob, 0),
	}
}

// HandleRecipientJob mocks handling a recipient job.
func (m *MockJobHandler) HandleRecipientJob(ctx context.Context, job *queue.RecipientJob) error {
	m.mu.Lock()
	m.Processed = append(m.Processed, job)
	m.mu.Unlock()

	// Non-blocking send to channel
	select {
	case m.ProcessedCh <- job:
	default:
	}

	if m.Error != nil {
		return m.Error
	}
	if m.HandleFunc != nil {
		return m.HandleFunc(ctx, job)
	}
	return nil
}

// ProcessedCount returns the number of jobs processed.
func (m *MockJobHandler) ProcessedCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.Processed)
}
