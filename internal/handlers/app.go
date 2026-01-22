package handlers

import (
	"context"
	"errors"
	"net/http"
	"sync"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/shridarpatil/whatomate/internal/config"
	"github.com/shridarpatil/whatomate/internal/queue"
	"github.com/shridarpatil/whatomate/internal/websocket"
	"github.com/shridarpatil/whatomate/pkg/whatsapp"
	"github.com/zerodha/fastglue"
	"github.com/zerodha/logf"
	"gorm.io/gorm"
)

// App holds all dependencies for handlers
type App struct {
	Config            *config.Config
	DB                *gorm.DB
	Redis             *redis.Client
	Log               logf.Logger
	WhatsApp          *whatsapp.Client
	WSHub             *websocket.Hub
	Queue             queue.Queue
	CampaignSubCancel context.CancelFunc
	// HTTPClient is a shared HTTP client with connection pooling for external API calls
	HTTPClient *http.Client
	// wg tracks background goroutines for graceful shutdown
	wg sync.WaitGroup
}

// WaitForBackgroundTasks blocks until all background goroutines complete.
// Call this during graceful shutdown to ensure all async work finishes.
func (a *App) WaitForBackgroundTasks() {
	a.wg.Wait()
}

// getOrgIDFromContext extracts organization ID from request context (set by auth middleware)
// Super admins can override the org by passing X-Organization-ID header
// Super admins MUST select an organization - no "all organizations" view
func (a *App) getOrgIDFromContext(r *fastglue.Request) (uuid.UUID, error) {
	// Get user's default organization ID from JWT
	orgIDVal := r.RequestCtx.UserValue("organization_id")
	if orgIDVal == nil {
		return uuid.Nil, errors.New("organization_id not found in context")
	}
	orgID, ok := orgIDVal.(uuid.UUID)
	if !ok {
		return uuid.Nil, errors.New("organization_id is not a valid UUID")
	}

	// Check if super admin is trying to switch organizations
	userID, _ := r.RequestCtx.UserValue("user_id").(uuid.UUID)
	if a.IsSuperAdmin(userID) {
		// Check for X-Organization-ID header
		overrideOrgID := string(r.RequestCtx.Request.Header.Peek("X-Organization-ID"))
		if overrideOrgID != "" {
			// Header present = super admin selected a specific org
			parsedOrgID, err := uuid.Parse(overrideOrgID)
			if err == nil {
				// Verify the organization exists
				var count int64
				if err := a.DB.Table("organizations").Where("id = ?", parsedOrgID).Count(&count).Error; err == nil && count > 0 {
					return parsedOrgID, nil
				}
			}
		}
		// No header or invalid org ID - fall back to user's org
	}

	return orgID, nil
}

// HealthCheck returns server health status
func (a *App) HealthCheck(r *fastglue.Request) error {
	return r.SendEnvelope(map[string]string{
		"status":  "ok",
		"service": "whatomate",
	})
}

// ReadyCheck returns server readiness status
func (a *App) ReadyCheck(r *fastglue.Request) error {
	// Check database connection
	sqlDB, err := a.DB.DB()
	if err != nil {
		return r.SendErrorEnvelope(500, "Database connection error", nil, "")
	}
	if err := sqlDB.Ping(); err != nil {
		return r.SendErrorEnvelope(500, "Database ping failed", nil, "")
	}

	// Check Redis connection
	if err := a.Redis.Ping(r.RequestCtx).Err(); err != nil {
		return r.SendErrorEnvelope(500, "Redis connection error", nil, "")
	}

	return r.SendEnvelope(map[string]string{
		"status": "ready",
	})
}

// StartCampaignStatsSubscriber starts listening for campaign stats updates from Redis pub/sub
// and broadcasts them via WebSocket
func (a *App) StartCampaignStatsSubscriber() error {
	if a.WSHub == nil {
		a.Log.Warn("WebSocket hub not initialized, skipping campaign stats subscriber")
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	a.CampaignSubCancel = cancel

	subscriber := queue.NewSubscriber(a.Redis, a.Log)

	err := subscriber.SubscribeCampaignStats(ctx, func(update *queue.CampaignStatsUpdate) {
		a.Log.Debug("Received campaign stats update from Redis",
			"campaign_id", update.CampaignID,
			"status", update.Status,
			"sent", update.SentCount,
		)

		// Broadcast to organization via WebSocket
		a.WSHub.BroadcastToOrg(update.OrganizationID, websocket.WSMessage{
			Type: websocket.TypeCampaignStatsUpdate,
			Payload: map[string]interface{}{
				"campaign_id":     update.CampaignID,
				"status":          update.Status,
				"sent_count":      update.SentCount,
				"delivered_count": update.DeliveredCount,
				"read_count":      update.ReadCount,
				"failed_count":    update.FailedCount,
			},
		})
	})

	if err != nil {
		cancel()
		return err
	}

	a.Log.Info("Campaign stats subscriber started")
	return nil
}

// StopCampaignStatsSubscriber stops the campaign stats subscriber
func (a *App) StopCampaignStatsSubscriber() {
	if a.CampaignSubCancel != nil {
		a.CampaignSubCancel()
	}
}
