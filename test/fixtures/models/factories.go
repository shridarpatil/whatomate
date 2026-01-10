// Package models provides factory functions for creating test data.
package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"golang.org/x/crypto/bcrypt"
)

const (
	// DefaultPassword is the default password for test users.
	DefaultPassword = "password123"
)

// defaultPasswordHash returns a bcrypt hash of the default test password.
func defaultPasswordHash() string {
	hash, _ := bcrypt.GenerateFromPassword([]byte(DefaultPassword), bcrypt.DefaultCost)
	return string(hash)
}

// OrganizationBuilder provides a fluent interface for creating test organizations.
type OrganizationBuilder struct {
	org models.Organization
}

// NewOrganization creates a new organization builder with default values.
func NewOrganization() *OrganizationBuilder {
	id := uuid.New()
	return &OrganizationBuilder{
		org: models.Organization{
			BaseModel: models.BaseModel{
				ID:        id,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			Name:     "Test Organization",
			Slug:     "test-org-" + id.String()[:8],
			Settings: models.JSONB{},
		},
	}
}

// WithID sets the organization ID.
func (b *OrganizationBuilder) WithID(id uuid.UUID) *OrganizationBuilder {
	b.org.ID = id
	return b
}

// WithName sets the organization name.
func (b *OrganizationBuilder) WithName(name string) *OrganizationBuilder {
	b.org.Name = name
	return b
}

// WithSlug sets the organization slug.
func (b *OrganizationBuilder) WithSlug(slug string) *OrganizationBuilder {
	b.org.Slug = slug
	return b
}

// WithSettings sets the organization settings.
func (b *OrganizationBuilder) WithSettings(settings models.JSONB) *OrganizationBuilder {
	b.org.Settings = settings
	return b
}

// Build returns the built organization.
func (b *OrganizationBuilder) Build() models.Organization {
	return b.org
}

// UserBuilder provides a fluent interface for creating test users.
type UserBuilder struct {
	user models.User
}

// NewUser creates a new user builder with default values.
func NewUser(orgID uuid.UUID) *UserBuilder {
	id := uuid.New()
	return &UserBuilder{
		user: models.User{
			BaseModel: models.BaseModel{
				ID:        id,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			OrganizationID: orgID,
			Email:          "test-" + id.String()[:8] + "@example.com",
			PasswordHash:   defaultPasswordHash(),
			FullName:       "Test User",
			Role:           "agent",
			Settings:       models.JSONB{},
			IsActive:       true,
			IsAvailable:    true,
		},
	}
}

// WithID sets the user ID.
func (b *UserBuilder) WithID(id uuid.UUID) *UserBuilder {
	b.user.ID = id
	return b
}

// WithEmail sets the user email.
func (b *UserBuilder) WithEmail(email string) *UserBuilder {
	b.user.Email = email
	return b
}

// WithFullName sets the user's full name.
func (b *UserBuilder) WithFullName(name string) *UserBuilder {
	b.user.FullName = name
	return b
}

// WithPassword sets the user password (hashes it).
func (b *UserBuilder) WithPassword(password string) *UserBuilder {
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	b.user.PasswordHash = string(hash)
	return b
}

// AsAdmin sets the user role to admin.
func (b *UserBuilder) AsAdmin() *UserBuilder {
	b.user.Role = "admin"
	return b
}

// AsManager sets the user role to manager.
func (b *UserBuilder) AsManager() *UserBuilder {
	b.user.Role = "manager"
	return b
}

// AsAgent sets the user role to agent.
func (b *UserBuilder) AsAgent() *UserBuilder {
	b.user.Role = "agent"
	return b
}

// Inactive sets the user as inactive.
func (b *UserBuilder) Inactive() *UserBuilder {
	b.user.IsActive = false
	return b
}

// Unavailable sets the user as unavailable.
func (b *UserBuilder) Unavailable() *UserBuilder {
	b.user.IsAvailable = false
	return b
}

// WithSSO sets SSO provider information.
func (b *UserBuilder) WithSSO(provider, providerID string) *UserBuilder {
	b.user.SSOProvider = provider
	b.user.SSOProviderID = providerID
	return b
}

// Build returns the built user.
func (b *UserBuilder) Build() models.User {
	return b.user
}

// WhatsAppAccountBuilder provides a fluent interface for creating test WhatsApp accounts.
type WhatsAppAccountBuilder struct {
	account models.WhatsAppAccount
}

// NewWhatsAppAccount creates a new WhatsApp account builder with default values.
func NewWhatsAppAccount(orgID uuid.UUID) *WhatsAppAccountBuilder {
	id := uuid.New()
	return &WhatsAppAccountBuilder{
		account: models.WhatsAppAccount{
			BaseModel: models.BaseModel{
				ID:        id,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			OrganizationID:     orgID,
			Name:               "test-account-" + id.String()[:8],
			AppID:              "123456789",
			PhoneID:            "987654321",
			BusinessID:         "111222333",
			AccessToken:        "test-access-token",
			WebhookVerifyToken: "test-verify-token",
			APIVersion:         "v21.0",
			IsDefaultIncoming:  true,
			IsDefaultOutgoing:  true,
			AutoReadReceipt:    false,
			Status:             "active",
		},
	}
}

// WithID sets the account ID.
func (b *WhatsAppAccountBuilder) WithID(id uuid.UUID) *WhatsAppAccountBuilder {
	b.account.ID = id
	return b
}

// WithName sets the account name.
func (b *WhatsAppAccountBuilder) WithName(name string) *WhatsAppAccountBuilder {
	b.account.Name = name
	return b
}

// WithPhoneID sets the phone ID.
func (b *WhatsAppAccountBuilder) WithPhoneID(phoneID string) *WhatsAppAccountBuilder {
	b.account.PhoneID = phoneID
	return b
}

// WithBusinessID sets the business ID.
func (b *WhatsAppAccountBuilder) WithBusinessID(businessID string) *WhatsAppAccountBuilder {
	b.account.BusinessID = businessID
	return b
}

// WithAccessToken sets the access token.
func (b *WhatsAppAccountBuilder) WithAccessToken(token string) *WhatsAppAccountBuilder {
	b.account.AccessToken = token
	return b
}

// AsDefault sets this account as both default incoming and outgoing.
func (b *WhatsAppAccountBuilder) AsDefault() *WhatsAppAccountBuilder {
	b.account.IsDefaultIncoming = true
	b.account.IsDefaultOutgoing = true
	return b
}

// NotDefault clears the default flags.
func (b *WhatsAppAccountBuilder) NotDefault() *WhatsAppAccountBuilder {
	b.account.IsDefaultIncoming = false
	b.account.IsDefaultOutgoing = false
	return b
}

// Build returns the built WhatsApp account.
func (b *WhatsAppAccountBuilder) Build() models.WhatsAppAccount {
	return b.account
}

// ContactBuilder provides a fluent interface for creating test contacts.
type ContactBuilder struct {
	contact models.Contact
}

// NewContact creates a new contact builder with default values.
func NewContact(orgID uuid.UUID) *ContactBuilder {
	id := uuid.New()
	return &ContactBuilder{
		contact: models.Contact{
			BaseModel: models.BaseModel{
				ID:        id,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			OrganizationID: orgID,
			PhoneNumber:    "1234567890",
			ProfileName:    "Test Contact",
			IsRead:         true,
			Tags:           models.JSONBArray{},
			Metadata:       models.JSONB{},
		},
	}
}

// WithID sets the contact ID.
func (b *ContactBuilder) WithID(id uuid.UUID) *ContactBuilder {
	b.contact.ID = id
	return b
}

// WithPhone sets the phone number.
func (b *ContactBuilder) WithPhone(phone string) *ContactBuilder {
	b.contact.PhoneNumber = phone
	return b
}

// WithName sets the profile name.
func (b *ContactBuilder) WithName(name string) *ContactBuilder {
	b.contact.ProfileName = name
	return b
}

// WithWhatsAppAccount sets the WhatsApp account reference.
func (b *ContactBuilder) WithWhatsAppAccount(accountName string) *ContactBuilder {
	b.contact.WhatsAppAccount = accountName
	return b
}

// AssignedTo sets the assigned user ID.
func (b *ContactBuilder) AssignedTo(userID uuid.UUID) *ContactBuilder {
	b.contact.AssignedUserID = &userID
	return b
}

// WithLastMessage sets the last message timestamp and preview.
func (b *ContactBuilder) WithLastMessage(at time.Time, preview string) *ContactBuilder {
	b.contact.LastMessageAt = &at
	b.contact.LastMessagePreview = preview
	return b
}

// Unread marks the contact as having unread messages.
func (b *ContactBuilder) Unread() *ContactBuilder {
	b.contact.IsRead = false
	return b
}

// Build returns the built contact.
func (b *ContactBuilder) Build() models.Contact {
	return b.contact
}

// TemplateBuilder provides a fluent interface for creating test templates.
type TemplateBuilder struct {
	template models.Template
}

// NewTemplate creates a new template builder with default values.
func NewTemplate(orgID uuid.UUID, accountName string) *TemplateBuilder {
	id := uuid.New()
	return &TemplateBuilder{
		template: models.Template{
			BaseModel: models.BaseModel{
				ID:        id,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			OrganizationID:  orgID,
			WhatsAppAccount: accountName,
			MetaTemplateID:  "meta-" + id.String()[:8],
			Name:            "test_template_" + id.String()[:8],
			DisplayName:     "Test Template",
			Language:        "en",
			Category:        "UTILITY",
			Status:          "APPROVED",
			BodyContent:     "Hello {{1}}, this is a test message.",
			Buttons:         models.JSONBArray{},
			SampleValues:    models.JSONBArray{},
		},
	}
}

// WithID sets the template ID.
func (b *TemplateBuilder) WithID(id uuid.UUID) *TemplateBuilder {
	b.template.ID = id
	return b
}

// WithName sets the template name.
func (b *TemplateBuilder) WithName(name string) *TemplateBuilder {
	b.template.Name = name
	return b
}

// WithBody sets the body content.
func (b *TemplateBuilder) WithBody(body string) *TemplateBuilder {
	b.template.BodyContent = body
	return b
}

// WithHeader sets the header type and content.
func (b *TemplateBuilder) WithHeader(headerType, content string) *TemplateBuilder {
	b.template.HeaderType = headerType
	b.template.HeaderContent = content
	return b
}

// WithFooter sets the footer content.
func (b *TemplateBuilder) WithFooter(footer string) *TemplateBuilder {
	b.template.FooterContent = footer
	return b
}

// WithStatus sets the template status.
func (b *TemplateBuilder) WithStatus(status string) *TemplateBuilder {
	b.template.Status = status
	return b
}

// WithCategory sets the template category.
func (b *TemplateBuilder) WithCategory(category string) *TemplateBuilder {
	b.template.Category = category
	return b
}

// Build returns the built template.
func (b *TemplateBuilder) Build() models.Template {
	return b.template
}

// MessageBuilder provides a fluent interface for creating test messages.
type MessageBuilder struct {
	message models.Message
}

// NewMessage creates a new message builder with default values.
func NewMessage(orgID, contactID uuid.UUID, accountName string) *MessageBuilder {
	id := uuid.New()
	return &MessageBuilder{
		message: models.Message{
			BaseModel: models.BaseModel{
				ID:        id,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			OrganizationID:    orgID,
			WhatsAppAccount:   accountName,
			ContactID:         contactID,
			WhatsAppMessageID: "wamid." + id.String()[:16],
			Direction:         "incoming",
			MessageType:       "text",
			Content:           "Test message content",
			Status:            "delivered",
			Metadata:          models.JSONB{},
		},
	}
}

// WithID sets the message ID.
func (b *MessageBuilder) WithID(id uuid.UUID) *MessageBuilder {
	b.message.ID = id
	return b
}

// WithContent sets the message content.
func (b *MessageBuilder) WithContent(content string) *MessageBuilder {
	b.message.Content = content
	return b
}

// Incoming sets the message as incoming.
func (b *MessageBuilder) Incoming() *MessageBuilder {
	b.message.Direction = "incoming"
	return b
}

// Outgoing sets the message as outgoing.
func (b *MessageBuilder) Outgoing() *MessageBuilder {
	b.message.Direction = "outgoing"
	return b
}

// SentByUser sets who sent the outgoing message.
func (b *MessageBuilder) SentByUser(userID uuid.UUID) *MessageBuilder {
	b.message.SentByUserID = &userID
	return b
}

// WithType sets the message type (text, image, template, etc.).
func (b *MessageBuilder) WithType(msgType string) *MessageBuilder {
	b.message.MessageType = msgType
	return b
}

// WithStatus sets the message status.
func (b *MessageBuilder) WithStatus(status string) *MessageBuilder {
	b.message.Status = status
	return b
}

// WithMedia sets media information.
func (b *MessageBuilder) WithMedia(url, mimeType, filename string) *MessageBuilder {
	b.message.MediaURL = url
	b.message.MediaMimeType = mimeType
	b.message.MediaFilename = filename
	return b
}

// AsTemplate sets the message as a template message.
func (b *MessageBuilder) AsTemplate(templateName string, params models.JSONB) *MessageBuilder {
	b.message.MessageType = "template"
	b.message.TemplateName = templateName
	b.message.TemplateParams = params
	return b
}

// Build returns the built message.
func (b *MessageBuilder) Build() models.Message {
	return b.message
}

// CampaignBuilder provides a fluent interface for creating test campaigns.
type CampaignBuilder struct {
	campaign models.BulkMessageCampaign
}

// NewCampaign creates a new campaign builder with default values.
func NewCampaign(orgID, templateID, creatorID uuid.UUID, accountName string) *CampaignBuilder {
	id := uuid.New()
	return &CampaignBuilder{
		campaign: models.BulkMessageCampaign{
			BaseModel: models.BaseModel{
				ID:        id,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			OrganizationID:  orgID,
			WhatsAppAccount: accountName,
			Name:            "Test Campaign " + id.String()[:8],
			TemplateID:      templateID,
			Status:          "draft",
			TotalRecipients: 0,
			SentCount:       0,
			DeliveredCount:  0,
			ReadCount:       0,
			FailedCount:     0,
			CreatedBy:       creatorID,
		},
	}
}

// WithID sets the campaign ID.
func (b *CampaignBuilder) WithID(id uuid.UUID) *CampaignBuilder {
	b.campaign.ID = id
	return b
}

// WithName sets the campaign name.
func (b *CampaignBuilder) WithName(name string) *CampaignBuilder {
	b.campaign.Name = name
	return b
}

// WithStatus sets the campaign status.
func (b *CampaignBuilder) WithStatus(status string) *CampaignBuilder {
	b.campaign.Status = status
	return b
}

// WithRecipientCounts sets the recipient counts.
func (b *CampaignBuilder) WithRecipientCounts(total, sent, delivered, read, failed int) *CampaignBuilder {
	b.campaign.TotalRecipients = total
	b.campaign.SentCount = sent
	b.campaign.DeliveredCount = delivered
	b.campaign.ReadCount = read
	b.campaign.FailedCount = failed
	return b
}

// ScheduledAt sets when the campaign is scheduled to start.
func (b *CampaignBuilder) ScheduledAt(t time.Time) *CampaignBuilder {
	b.campaign.ScheduledAt = &t
	return b
}

// Build returns the built campaign.
func (b *CampaignBuilder) Build() models.BulkMessageCampaign {
	return b.campaign
}

// AgentTransferBuilder provides a fluent interface for creating test agent transfers.
type AgentTransferBuilder struct {
	transfer models.AgentTransfer
}

// NewAgentTransfer creates a new agent transfer builder with default values.
func NewAgentTransfer(orgID, contactID uuid.UUID, accountName, phone string) *AgentTransferBuilder {
	id := uuid.New()
	return &AgentTransferBuilder{
		transfer: models.AgentTransfer{
			BaseModel: models.BaseModel{
				ID:        id,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			OrganizationID:  orgID,
			ContactID:       contactID,
			WhatsAppAccount: accountName,
			PhoneNumber:     phone,
			Status:          "active",
			Source:          "manual",
			TransferredAt:   time.Now(),
		},
	}
}

// WithID sets the transfer ID.
func (b *AgentTransferBuilder) WithID(id uuid.UUID) *AgentTransferBuilder {
	b.transfer.ID = id
	return b
}

// WithStatus sets the transfer status.
func (b *AgentTransferBuilder) WithStatus(status string) *AgentTransferBuilder {
	b.transfer.Status = status
	return b
}

// WithSource sets the transfer source.
func (b *AgentTransferBuilder) WithSource(source string) *AgentTransferBuilder {
	b.transfer.Source = source
	return b
}

// AssignedTo sets the assigned agent.
func (b *AgentTransferBuilder) AssignedTo(agentID uuid.UUID) *AgentTransferBuilder {
	b.transfer.AgentID = &agentID
	return b
}

// WithTeam sets the team.
func (b *AgentTransferBuilder) WithTeam(teamID uuid.UUID) *AgentTransferBuilder {
	b.transfer.TeamID = &teamID
	return b
}

// WithNotes sets notes.
func (b *AgentTransferBuilder) WithNotes(notes string) *AgentTransferBuilder {
	b.transfer.Notes = notes
	return b
}

// TransferredAt sets when the transfer occurred.
func (b *AgentTransferBuilder) TransferredAt(t time.Time) *AgentTransferBuilder {
	b.transfer.TransferredAt = t
	return b
}

// WithSLADeadlines sets SLA deadline fields.
func (b *AgentTransferBuilder) WithSLADeadlines(response, resolution, escalation *time.Time) *AgentTransferBuilder {
	b.transfer.SLAResponseDeadline = response
	b.transfer.SLAResolutionDeadline = resolution
	b.transfer.SLAEscalationAt = escalation
	return b
}

// Build returns the built agent transfer.
func (b *AgentTransferBuilder) Build() models.AgentTransfer {
	return b.transfer
}

// TeamBuilder provides a fluent interface for creating test teams.
type TeamBuilder struct {
	team models.Team
}

// NewTeam creates a new team builder with default values.
func NewTeam(orgID uuid.UUID) *TeamBuilder {
	id := uuid.New()
	return &TeamBuilder{
		team: models.Team{
			BaseModel: models.BaseModel{
				ID:        id,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			OrganizationID:     orgID,
			Name:               "Test Team " + id.String()[:8],
			AssignmentStrategy: "round_robin",
			IsActive:           true,
		},
	}
}

// WithID sets the team ID.
func (b *TeamBuilder) WithID(id uuid.UUID) *TeamBuilder {
	b.team.ID = id
	return b
}

// WithName sets the team name.
func (b *TeamBuilder) WithName(name string) *TeamBuilder {
	b.team.Name = name
	return b
}

// WithDescription sets the team description.
func (b *TeamBuilder) WithDescription(desc string) *TeamBuilder {
	b.team.Description = desc
	return b
}

// WithStrategy sets the assignment strategy.
func (b *TeamBuilder) WithStrategy(strategy string) *TeamBuilder {
	b.team.AssignmentStrategy = strategy
	return b
}

// Inactive sets the team as inactive.
func (b *TeamBuilder) Inactive() *TeamBuilder {
	b.team.IsActive = false
	return b
}

// Build returns the built team.
func (b *TeamBuilder) Build() models.Team {
	return b.team
}
