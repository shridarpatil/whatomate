package database

import (
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/config"
	"github.com/shridarpatil/whatomate/internal/models"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)


// NewPostgres creates a new PostgreSQL connection
func NewPostgres(cfg *config.DatabaseConfig, debug bool) (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Name, cfg.SSLMode,
	)

	logLevel := logger.Silent
	if debug {
		logLevel = logger.Info
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database instance: %w", err)
	}

	// Configure connection pool
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetime) * time.Second)

	return db, nil
}

// MigrationModel holds model info for migration progress
type MigrationModel struct {
	Name  string
	Model interface{}
}

// GetMigrationModels returns all models to migrate with their names
func GetMigrationModels() []MigrationModel {
	return []MigrationModel{
		// Core models
		{"Organization", &models.Organization{}},
		{"User", &models.User{}},
		{"Team", &models.Team{}},
		{"TeamMember", &models.TeamMember{}},
		{"APIKey", &models.APIKey{}},
		{"SSOProvider", &models.SSOProvider{}},
		{"Webhook", &models.Webhook{}},
		{"CustomAction", &models.CustomAction{}},
		{"WhatsAppAccount", &models.WhatsAppAccount{}},
		{"Contact", &models.Contact{}},
		{"Message", &models.Message{}},
		{"Template", &models.Template{}},
		{"WhatsAppFlow", &models.WhatsAppFlow{}},

		// Bulk & Notifications
		{"BulkMessageCampaign", &models.BulkMessageCampaign{}},
		{"BulkMessageRecipient", &models.BulkMessageRecipient{}},
		{"NotificationRule", &models.NotificationRule{}},

		// Chatbot models
		{"ChatbotSettings", &models.ChatbotSettings{}},
		{"KeywordRule", &models.KeywordRule{}},
		{"ChatbotFlow", &models.ChatbotFlow{}},
		{"ChatbotFlowStep", &models.ChatbotFlowStep{}},
		{"ChatbotSession", &models.ChatbotSession{}},
		{"ChatbotSessionMessage", &models.ChatbotSessionMessage{}},
		{"AIContext", &models.AIContext{}},
		{"AgentTransfer", &models.AgentTransfer{}},

		// User tracking
		{"UserAvailabilityLog", &models.UserAvailabilityLog{}},

		// Canned responses
		{"CannedResponse", &models.CannedResponse{}},

		// Catalogs
		{"Catalog", &models.Catalog{}},
		{"CatalogProduct", &models.CatalogProduct{}},
	}
}

// AutoMigrate runs auto migration for all models (silent mode)
func AutoMigrate(db *gorm.DB) error {
	migrationModels := GetMigrationModels()
	for _, m := range migrationModels {
		if err := db.AutoMigrate(m.Model); err != nil {
			return err
		}
	}
	return nil
}

// RunMigrationWithProgress runs migrations with a progress bar display
func RunMigrationWithProgress(db *gorm.DB) error {
	// Silence GORM logging during migration
	silentDB := db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})

	migrationModels := GetMigrationModels()
	indexes := getIndexes()

	// Total steps: models + indexes + default admin check
	totalSteps := len(migrationModels) + len(indexes) + 1
	currentStep := 0
	barWidth := 40

	printProgress := func(step int, total int) {
		percent := float64(step) / float64(total)
		filled := int(percent * float64(barWidth))
		empty := barWidth - filled

		bar := repeatChar("█", filled) + "\033[90m" + repeatChar("░", empty) + "\033[0m"
		fmt.Printf("\r  Running migrations  %s %3d%%", bar, int(percent*100))
		_ = os.Stdout.Sync()
	}

	fmt.Println()

	// Migrate models
	for _, m := range migrationModels {
		printProgress(currentStep, totalSteps)
		if err := silentDB.AutoMigrate(m.Model); err != nil {
			fmt.Printf("\n  \033[31m✗ Migration failed: %s\033[0m\n\n", m.Name)
			return fmt.Errorf("failed to migrate %s: %w", m.Name, err)
		}
		currentStep++
	}

	// Create indexes
	for _, idx := range indexes {
		printProgress(currentStep, totalSteps)
		if err := silentDB.Exec(idx).Error; err != nil {
			fmt.Printf("\n  \033[31m✗ Index creation failed\033[0m\n\n")
			return fmt.Errorf("failed to create index: %w", err)
		}
		currentStep++
	}

	// Create default admin
	printProgress(currentStep, totalSteps)
	if err := CreateDefaultAdmin(silentDB); err != nil {
		fmt.Printf("\n  \033[31m✗ Setup failed\033[0m\n\n")
		return err
	}
	currentStep++

	printProgress(currentStep, totalSteps)
	fmt.Printf("\n  \033[32m✓ Migration completed\033[0m\n\n")

	return nil
}

// repeatChar repeats a character n times
func repeatChar(char string, n int) string {
	result := ""
	for i := 0; i < n; i++ {
		result += char
	}
	return result
}

// getIndexes returns all index creation SQL statements
func getIndexes() []string {
	return []string{
		`CREATE INDEX IF NOT EXISTS idx_messages_contact_created ON messages(contact_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_messages_conversation ON messages(conversation_id)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_contacts_org_phone ON contacts(organization_id, phone_number)`,
		`CREATE INDEX IF NOT EXISTS idx_contacts_assigned_read ON contacts(assigned_user_id, is_read)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_phone_status ON chatbot_sessions(organization_id, phone_number, status)`,
		`CREATE INDEX IF NOT EXISTS idx_keyword_rules_priority ON keyword_rules(organization_id, is_enabled, priority DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_agent_transfers_active ON agent_transfers(organization_id, phone_number, status)`,
		`CREATE INDEX IF NOT EXISTS idx_agent_transfers_org_contact ON agent_transfers(organization_id, contact_id, status)`,
		`CREATE INDEX IF NOT EXISTS idx_agent_transfers_agent_active ON agent_transfers(agent_id, status) WHERE status = 'active'`,
		`CREATE INDEX IF NOT EXISTS idx_agent_transfers_team ON agent_transfers(team_id, status) WHERE team_id IS NOT NULL`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_whatsapp_accounts_org_phone ON whatsapp_accounts(organization_id, phone_id)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_templates_account_name_lang ON templates(whats_app_account, name, language)`,
		`CREATE INDEX IF NOT EXISTS idx_keyword_rules_account ON keyword_rules(whats_app_account, is_enabled, priority DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_chatbot_flows_account ON chatbot_flows(whats_app_account, is_enabled)`,
		`CREATE INDEX IF NOT EXISTS idx_ai_contexts_account ON ai_contexts(whats_app_account, is_enabled, priority DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_bulk_campaigns_account ON bulk_message_campaigns(whats_app_account, status)`,
		`CREATE INDEX IF NOT EXISTS idx_notification_rules_account ON notification_rules(whats_app_account, is_enabled)`,
		`CREATE INDEX IF NOT EXISTS idx_messages_account ON messages(whats_app_account, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_contacts_account ON contacts(whats_app_account)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_canned_responses_org_name ON canned_responses(organization_id, name)`,
		`CREATE INDEX IF NOT EXISTS idx_canned_responses_active ON canned_responses(organization_id, is_active, usage_count DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_webhooks_org_active ON webhooks(organization_id, is_active)`,
		`CREATE INDEX IF NOT EXISTS idx_availability_logs_user_time ON user_availability_logs(user_id, started_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_availability_logs_org_time ON user_availability_logs(organization_id, started_at DESC)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_sso_providers_org_provider ON sso_providers(organization_id, provider)`,
	}
}

// CreateIndexes creates additional indexes not handled by GORM tags
func CreateIndexes(db *gorm.DB) error {
	indexes := []string{
		// Messages indexes
		`CREATE INDEX IF NOT EXISTS idx_messages_contact_created ON messages(contact_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_messages_conversation ON messages(conversation_id)`,

		// Contacts indexes
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_contacts_org_phone ON contacts(organization_id, phone_number)`,
		`CREATE INDEX IF NOT EXISTS idx_contacts_assigned_read ON contacts(assigned_user_id, is_read)`,

		// Sessions indexes
		`CREATE INDEX IF NOT EXISTS idx_sessions_phone_status ON chatbot_sessions(organization_id, phone_number, status)`,

		// Keyword rules indexes
		`CREATE INDEX IF NOT EXISTS idx_keyword_rules_priority ON keyword_rules(organization_id, is_enabled, priority DESC)`,

		// Agent transfers indexes
		`CREATE INDEX IF NOT EXISTS idx_agent_transfers_active ON agent_transfers(organization_id, phone_number, status)`,
		`CREATE INDEX IF NOT EXISTS idx_agent_transfers_org_contact ON agent_transfers(organization_id, contact_id, status)`,
		`CREATE INDEX IF NOT EXISTS idx_agent_transfers_agent_active ON agent_transfers(agent_id, status) WHERE status = 'active'`,
		`CREATE INDEX IF NOT EXISTS idx_agent_transfers_team ON agent_transfers(team_id, status) WHERE team_id IS NOT NULL`,

		// Teams indexes
		`CREATE INDEX IF NOT EXISTS idx_teams_org_active ON teams(organization_id, is_active)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_team_members_unique ON team_members(team_id, user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_team_members_user ON team_members(user_id)`,

		// WhatsApp accounts indexes
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_whatsapp_accounts_org_phone ON whatsapp_accounts(organization_id, phone_id)`,

		// Templates indexes
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_templates_account_name_lang ON templates(whats_app_account, name, language)`,

		// Chatbot indexes with account
		`CREATE INDEX IF NOT EXISTS idx_keyword_rules_account ON keyword_rules(whats_app_account, is_enabled, priority DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_chatbot_flows_account ON chatbot_flows(whats_app_account, is_enabled)`,
		`CREATE INDEX IF NOT EXISTS idx_ai_contexts_account ON ai_contexts(whats_app_account, is_enabled, priority DESC)`,

		// Bulk messaging indexes
		`CREATE INDEX IF NOT EXISTS idx_bulk_campaigns_account ON bulk_message_campaigns(whats_app_account, status)`,
		`CREATE INDEX IF NOT EXISTS idx_notification_rules_account ON notification_rules(whats_app_account, is_enabled)`,

		// Messages and contacts by account
		`CREATE INDEX IF NOT EXISTS idx_messages_account ON messages(whats_app_account, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_contacts_account ON contacts(whats_app_account)`,

		// Canned responses indexes
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_canned_responses_org_name ON canned_responses(organization_id, name)`,
		`CREATE INDEX IF NOT EXISTS idx_canned_responses_active ON canned_responses(organization_id, is_active, usage_count DESC)`,

		// Webhooks indexes
		`CREATE INDEX IF NOT EXISTS idx_webhooks_org_active ON webhooks(organization_id, is_active)`,

		// User availability logs indexes
		`CREATE INDEX IF NOT EXISTS idx_availability_logs_user_time ON user_availability_logs(user_id, started_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_availability_logs_org_time ON user_availability_logs(organization_id, started_at DESC)`,

		// SSO providers indexes
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_sso_providers_org_provider ON sso_providers(organization_id, provider)`,
	}

	for _, idx := range indexes {
		if err := db.Exec(idx).Error; err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}
	}

	return nil
}

// CreateDefaultAdmin creates a default admin user if no users exist
// This should only be called once during initial setup
func CreateDefaultAdmin(db *gorm.DB) error {
	// Check if any users exist
	var count int64
	if err := db.Model(&models.User{}).Count(&count).Error; err != nil {
		return fmt.Errorf("failed to count users: %w", err)
	}

	// If users already exist, skip creating default admin
	if count > 0 {
		return nil
	}

	// Create default organization
	org := models.Organization{
		BaseModel: models.BaseModel{ID: uuid.New()},
		Name:      "Default Organization",
		Settings:  models.JSONB{},
	}
	if err := db.Create(&org).Error; err != nil {
		return fmt.Errorf("failed to create default organization: %w", err)
	}

	// Hash the default password
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Create default admin user
	admin := models.User{
		BaseModel:      models.BaseModel{ID: uuid.New()},
		OrganizationID: org.ID,
		Email:          "admin@admin.com",
		PasswordHash:   string(passwordHash),
		FullName:       "Admin",
		Role:           "admin",
		IsActive:       true,
		IsAvailable:    true,
		Settings:       models.JSONB{},
	}
	if err := db.Create(&admin).Error; err != nil {
		return fmt.Errorf("failed to create default admin user: %w", err)
	}

	return nil
}
