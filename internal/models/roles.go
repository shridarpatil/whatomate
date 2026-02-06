package models

import (
	"github.com/google/uuid"
)

// Permission represents a granular permission for a specific resource and action
type Permission struct {
	BaseModel
	Resource    string `gorm:"size:50;not null;uniqueIndex:idx_permission_resource_action" json:"resource"`
	Action      string `gorm:"size:20;not null;uniqueIndex:idx_permission_resource_action" json:"action"`
	Description string `gorm:"size:200" json:"description"`
}

func (Permission) TableName() string {
	return "permissions"
}

// CustomRole represents a role with specific permissions
type CustomRole struct {
	BaseModel
	OrganizationID uuid.UUID    `gorm:"type:uuid;index;not null" json:"organization_id"`
	Name           string       `gorm:"size:100;not null" json:"name"`
	Description    string       `gorm:"size:500" json:"description"`
	IsSystem       bool         `gorm:"default:false" json:"is_system"` // true for default admin/manager/agent
	IsDefault      bool         `gorm:"default:false" json:"is_default"` // default role for new users in org
	Permissions    []Permission `gorm:"many2many:role_permissions;" json:"permissions"`

	// Relations
	Organization *Organization `gorm:"foreignKey:OrganizationID" json:"organization,omitempty"`
}

func (CustomRole) TableName() string {
	return "custom_roles"
}

// RolePermission is the junction table for CustomRole and Permission many-to-many relationship
type RolePermission struct {
	CustomRoleID uuid.UUID `gorm:"type:uuid;primaryKey" json:"custom_role_id"`
	PermissionID uuid.UUID `gorm:"type:uuid;primaryKey" json:"permission_id"`
}

func (RolePermission) TableName() string {
	return "role_permissions"
}

// PermissionResource constants for available resources
const (
	ResourceUsers           = "users"
	ResourceTeams           = "teams"
	ResourceRoles           = "roles"
	ResourceSettingsGeneral = "settings.general"
	ResourceSettingsChatbot = "settings.chatbot"
	ResourceSettingsSSO     = "settings.sso"
	ResourceAccounts        = "accounts"
	ResourceTemplates       = "templates"
	ResourceFlowsWhatsApp   = "flows.whatsapp"
	ResourceFlowsChatbot    = "flows.chatbot"
	ResourceCampaigns       = "campaigns"
	ResourceChatbotKeywords = "chatbot.keywords"
	ResourceChatbotAI       = "chatbot.ai"
	ResourceChat            = "chat"
	ResourceChatAssign      = "chat.assign"
	ResourceContacts        = "contacts"
	ResourceTags            = "tags"
	ResourceAnalytics       = "analytics"
	ResourceAnalyticsAgents = "analytics.agents"
	ResourceTransfers       = "transfers"
	ResourceWebhooks        = "webhooks"
	ResourceAPIKeys         = "api_keys"
	ResourceCannedResponses = "canned_responses"
	ResourceCustomActions   = "custom_actions"
	ResourceOrganizations   = "organizations"
)

// PermissionAction constants for available actions
const (
	ActionRead    = "read"
	ActionWrite   = "write"
	ActionDelete  = "delete"
	ActionSync    = "sync"
	ActionExecute = "execute"
	ActionImport  = "import"
	ActionExport  = "export"
	ActionPickup  = "pickup"
	ActionAssign  = "assign"
)

// DefaultPermissions returns the list of all available permissions to seed
func DefaultPermissions() []Permission {
	return []Permission{
		// Users
		{Resource: ResourceUsers, Action: ActionRead, Description: "View users"},
		{Resource: ResourceUsers, Action: ActionWrite, Description: "Create and edit users"},
		{Resource: ResourceUsers, Action: ActionDelete, Description: "Delete users"},

		// Teams
		{Resource: ResourceTeams, Action: ActionRead, Description: "View teams"},
		{Resource: ResourceTeams, Action: ActionWrite, Description: "Create and edit teams"},
		{Resource: ResourceTeams, Action: ActionDelete, Description: "Delete teams"},

		// Roles
		{Resource: ResourceRoles, Action: ActionRead, Description: "View roles"},
		{Resource: ResourceRoles, Action: ActionWrite, Description: "Create and edit roles"},
		{Resource: ResourceRoles, Action: ActionDelete, Description: "Delete roles"},

		// Settings
		{Resource: ResourceSettingsGeneral, Action: ActionRead, Description: "View general settings"},
		{Resource: ResourceSettingsGeneral, Action: ActionWrite, Description: "Edit general settings"},
		{Resource: ResourceSettingsChatbot, Action: ActionRead, Description: "View chatbot settings"},
		{Resource: ResourceSettingsChatbot, Action: ActionWrite, Description: "Edit chatbot settings"},
		{Resource: ResourceSettingsSSO, Action: ActionRead, Description: "View SSO settings"},
		{Resource: ResourceSettingsSSO, Action: ActionWrite, Description: "Edit SSO settings"},

		// Accounts
		{Resource: ResourceAccounts, Action: ActionRead, Description: "View WhatsApp accounts"},
		{Resource: ResourceAccounts, Action: ActionWrite, Description: "Create and edit WhatsApp accounts"},
		{Resource: ResourceAccounts, Action: ActionDelete, Description: "Delete WhatsApp accounts"},

		// Templates
		{Resource: ResourceTemplates, Action: ActionRead, Description: "View message templates"},
		{Resource: ResourceTemplates, Action: ActionWrite, Description: "Create and edit templates"},
		{Resource: ResourceTemplates, Action: ActionDelete, Description: "Delete templates"},
		{Resource: ResourceTemplates, Action: ActionSync, Description: "Sync templates with Meta"},

		// WhatsApp Flows
		{Resource: ResourceFlowsWhatsApp, Action: ActionRead, Description: "View WhatsApp flows"},
		{Resource: ResourceFlowsWhatsApp, Action: ActionWrite, Description: "Create and edit WhatsApp flows"},
		{Resource: ResourceFlowsWhatsApp, Action: ActionDelete, Description: "Delete WhatsApp flows"},

		// Chatbot Flows
		{Resource: ResourceFlowsChatbot, Action: ActionRead, Description: "View chatbot flows"},
		{Resource: ResourceFlowsChatbot, Action: ActionWrite, Description: "Create and edit chatbot flows"},
		{Resource: ResourceFlowsChatbot, Action: ActionDelete, Description: "Delete chatbot flows"},

		// Campaigns
		{Resource: ResourceCampaigns, Action: ActionRead, Description: "View campaigns"},
		{Resource: ResourceCampaigns, Action: ActionWrite, Description: "Create and edit campaigns"},
		{Resource: ResourceCampaigns, Action: ActionDelete, Description: "Delete campaigns"},
		{Resource: ResourceCampaigns, Action: ActionExecute, Description: "Execute campaigns"},

		// Chatbot Keywords
		{Resource: ResourceChatbotKeywords, Action: ActionRead, Description: "View keyword rules"},
		{Resource: ResourceChatbotKeywords, Action: ActionWrite, Description: "Create and edit keyword rules"},
		{Resource: ResourceChatbotKeywords, Action: ActionDelete, Description: "Delete keyword rules"},

		// Chatbot AI
		{Resource: ResourceChatbotAI, Action: ActionRead, Description: "View AI contexts"},
		{Resource: ResourceChatbotAI, Action: ActionWrite, Description: "Create and edit AI contexts"},

		// Chat
		{Resource: ResourceChat, Action: ActionRead, Description: "View chat conversations"},
		{Resource: ResourceChat, Action: ActionWrite, Description: "Send messages"},
		{Resource: ResourceChatAssign, Action: ActionWrite, Description: "Assign conversations to agents"},

		// Contacts
		{Resource: ResourceContacts, Action: ActionRead, Description: "View contacts"},
		{Resource: ResourceContacts, Action: ActionWrite, Description: "Create and edit contacts"},
		{Resource: ResourceContacts, Action: ActionDelete, Description: "Delete contacts"},
		{Resource: ResourceContacts, Action: ActionImport, Description: "Import contacts"},
		{Resource: ResourceContacts, Action: ActionExport, Description: "Export contacts"},

		// Tags
		{Resource: ResourceTags, Action: ActionRead, Description: "View tags"},
		{Resource: ResourceTags, Action: ActionWrite, Description: "Create and edit tags"},
		{Resource: ResourceTags, Action: ActionDelete, Description: "Delete tags"},

		// Analytics
		{Resource: ResourceAnalytics, Action: ActionRead, Description: "View analytics dashboard"},
		{Resource: ResourceAnalytics, Action: ActionWrite, Description: "Create and edit dashboard widgets"},
		{Resource: ResourceAnalytics, Action: ActionDelete, Description: "Delete dashboard widgets"},
		{Resource: ResourceAnalyticsAgents, Action: ActionRead, Description: "View agent analytics"},

		// Transfers
		{Resource: ResourceTransfers, Action: ActionRead, Description: "View agent transfers"},
		{Resource: ResourceTransfers, Action: ActionWrite, Description: "Create transfers"},
		{Resource: ResourceTransfers, Action: ActionPickup, Description: "Pickup transfers from queue"},

		// Webhooks
		{Resource: ResourceWebhooks, Action: ActionRead, Description: "View webhooks"},
		{Resource: ResourceWebhooks, Action: ActionWrite, Description: "Create and edit webhooks"},
		{Resource: ResourceWebhooks, Action: ActionDelete, Description: "Delete webhooks"},

		// API Keys
		{Resource: ResourceAPIKeys, Action: ActionRead, Description: "View API keys"},
		{Resource: ResourceAPIKeys, Action: ActionWrite, Description: "Create API keys"},
		{Resource: ResourceAPIKeys, Action: ActionDelete, Description: "Delete API keys"},

		// Canned Responses
		{Resource: ResourceCannedResponses, Action: ActionRead, Description: "View canned responses"},
		{Resource: ResourceCannedResponses, Action: ActionWrite, Description: "Create and edit canned responses"},
		{Resource: ResourceCannedResponses, Action: ActionDelete, Description: "Delete canned responses"},

		// Custom Actions
		{Resource: ResourceCustomActions, Action: ActionRead, Description: "View custom actions"},
		{Resource: ResourceCustomActions, Action: ActionWrite, Description: "Create and edit custom actions"},
		{Resource: ResourceCustomActions, Action: ActionDelete, Description: "Delete custom actions"},

		// Organizations
		{Resource: ResourceOrganizations, Action: ActionRead, Description: "View organizations"},
		{Resource: ResourceOrganizations, Action: ActionWrite, Description: "Create organizations"},
		{Resource: ResourceOrganizations, Action: ActionDelete, Description: "Delete organizations"},
		{Resource: ResourceOrganizations, Action: ActionAssign, Description: "Manage organization members"},
	}
}

// SystemRolePermissions returns the default permission mappings for system roles
func SystemRolePermissions() map[string][]string {
	// Format: "resource:action"
	allPermissions := []string{}
	for _, p := range DefaultPermissions() {
		allPermissions = append(allPermissions, p.Resource+":"+p.Action)
	}

	managerPermissions := []string{
		// Teams (read only)
		"teams:read",
		// Settings
		"settings.general:read", "settings.general:write",
		"settings.chatbot:read", "settings.chatbot:write",
		// Accounts
		"accounts:read", "accounts:write", "accounts:delete",
		// Templates
		"templates:read", "templates:write", "templates:delete", "templates:sync",
		// Flows
		"flows.whatsapp:read", "flows.whatsapp:write", "flows.whatsapp:delete",
		"flows.chatbot:read", "flows.chatbot:write", "flows.chatbot:delete",
		// Campaigns
		"campaigns:read", "campaigns:write", "campaigns:delete", "campaigns:execute",
		// Chatbot
		"chatbot.keywords:read", "chatbot.keywords:write", "chatbot.keywords:delete",
		"chatbot.ai:read", "chatbot.ai:write",
		// Chat
		"chat:read", "chat:write", "chat.assign:write",
		// Contacts
		"contacts:read", "contacts:write", "contacts:delete", "contacts:import", "contacts:export",
		// Tags
		"tags:read", "tags:write", "tags:delete",
		// Analytics
		"analytics:read", "analytics.agents:read",
		// Transfers
		"transfers:read", "transfers:write", "transfers:pickup",
		// Webhooks
		"webhooks:read", "webhooks:write", "webhooks:delete",
		// Canned Responses
		"canned_responses:read", "canned_responses:write", "canned_responses:delete",
		// Custom Actions
		"custom_actions:read", "custom_actions:write", "custom_actions:delete",
		// Organizations (read only)
		"organizations:read",
	}

	agentPermissions := []string{
		// Chat
		"chat:read", "chat:write",
		// Contacts (read only)
		"contacts:read",
		// Tags (read only - agents can see tags on contacts)
		"tags:read",
		// Analytics (own)
		"analytics.agents:read",
		// Transfers
		"transfers:read", "transfers:write", "transfers:pickup",
		// Canned Responses (read only)
		"canned_responses:read",
	}

	return map[string][]string{
		"admin":   allPermissions,
		"manager": managerPermissions,
		"agent":   agentPermissions,
	}
}
