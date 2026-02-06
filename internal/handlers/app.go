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
	"github.com/valyala/fasthttp"
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

// getOrgID extracts organization ID from request context (set by auth middleware)
// Super admins can override the org by passing X-Organization-ID header
// Super admins MUST select an organization - no "all organizations" view
func (a *App) getOrgID(r *fastglue.Request) (uuid.UUID, error) {
	// Get user's default organization ID from JWT
	var defaultOrgID uuid.UUID
	orgIDVal := r.RequestCtx.UserValue("organization_id")
	if orgIDVal == nil {
		return uuid.Nil, errors.New("organization_id not found in context")
	}
	switch v := orgIDVal.(type) {
	case uuid.UUID:
		defaultOrgID = v
	case string:
		parsed, err := uuid.Parse(v)
		if err != nil {
			return uuid.Nil, errors.New("organization_id is not a valid UUID")
		}
		defaultOrgID = parsed
	default:
		return uuid.Nil, errors.New("organization_id is not a valid UUID")
	}

	// Check for X-Organization-ID header to switch organizations
	userID, _ := r.RequestCtx.UserValue("user_id").(uuid.UUID)
	overrideOrgID := string(r.RequestCtx.Request.Header.Peek("X-Organization-ID"))
	if overrideOrgID != "" {
		parsedOrgID, err := uuid.Parse(overrideOrgID)
		if err == nil && parsedOrgID != defaultOrgID {
			if a.IsSuperAdmin(userID) {
				// Super admins can access any org
				var count int64
				if err := a.DB.Table("organizations").Where("id = ?", parsedOrgID).Count(&count).Error; err == nil && count > 0 {
					return parsedOrgID, nil
				}
			} else {
				// Non-super-admins can switch if they have membership
				var count int64
				if err := a.DB.Table("user_organizations").
					Where("user_id = ? AND organization_id = ? AND deleted_at IS NULL", userID, parsedOrgID).
					Count(&count).Error; err == nil && count > 0 {
					return parsedOrgID, nil
				}
			}
		}
	}

	return defaultOrgID, nil
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

// getOrgAndUserID extracts both organization ID and user ID from the request context.
// Returns an error if either is missing or invalid.
func (a *App) getOrgAndUserID(r *fastglue.Request) (orgID, userID uuid.UUID, err error) {
	orgID, err = a.getOrgID(r)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}

	userIDVal := r.RequestCtx.UserValue("user_id")
	if userIDVal == nil {
		return uuid.Nil, uuid.Nil, errors.New("user_id not found in context")
	}
	switch v := userIDVal.(type) {
	case uuid.UUID:
		userID = v
	case string:
		userID, err = uuid.Parse(v)
		if err != nil {
			return uuid.Nil, uuid.Nil, errors.New("user_id is not a valid UUID")
		}
	default:
		return uuid.Nil, uuid.Nil, errors.New("user_id is not a valid UUID")
	}

	return orgID, userID, nil
}

// requirePermission checks if the user has the required permission.
// Returns nil if permitted, otherwise sends a 403 error envelope and returns errEnvelopeSent.
// Automatically extracts orgID from the request for org-aware permission checks.
func (a *App) requirePermission(r *fastglue.Request, userID uuid.UUID, resource, action string) error {
	orgID, _ := a.getOrgID(r)
	if !a.HasPermission(userID, resource, action, orgID) {
		_ = r.SendErrorEnvelope(fasthttp.StatusForbidden, "Insufficient permissions", nil, "")
		return errEnvelopeSent
	}
	return nil
}

// decodeRequest decodes a JSON request body into the provided struct.
// Returns nil on success, otherwise sends a 400 error envelope and returns errEnvelopeSent.
func (a *App) decodeRequest(r *fastglue.Request, v interface{}) error {
	if err := r.Decode(v, "json"); err != nil {
		_ = r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid request body", nil, "")
		return errEnvelopeSent
	}
	return nil
}
