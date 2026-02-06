package handlers

import (
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

// AgentAnalyticsSummary represents overall agent analytics
type AgentAnalyticsSummary struct {
	TotalTransfersHandled int64            `json:"total_transfers_handled"`
	ActiveTransfers       int64            `json:"active_transfers"`
	AvgQueueTimeMins      float64          `json:"avg_queue_time_mins"`
	AvgFirstResponseMins  float64          `json:"avg_first_response_mins"`
	AvgResolutionMins     float64          `json:"avg_resolution_mins"`
	TransfersBySource     map[string]int64 `json:"transfers_by_source"`
	TotalBreakTimeMins    float64          `json:"total_break_time_mins"`
	BreakCount            int64            `json:"break_count"`
}

// AgentPerformanceStats represents performance metrics for an agent
type AgentPerformanceStats struct {
	AgentID              string   `json:"agent_id"`
	AgentName            string   `json:"agent_name"`
	AvgFirstResponseMins float64  `json:"avg_first_response_mins"`
	AvgResolutionMins    float64  `json:"avg_resolution_mins"`
	TransfersHandled     int64    `json:"transfers_handled"`
	ActiveTransfers      int64    `json:"active_transfers"`
	MessagesSent         int64    `json:"messages_sent"`
	TotalBreakTimeMins   float64  `json:"total_break_time_mins"`
	BreakCount           int64    `json:"break_count"`
	IsAvailable          bool     `json:"is_available"`
	CurrentBreakStart    *string  `json:"current_break_start,omitempty"`
}

// TrendPoint represents a data point for time-series charts
type TrendPoint struct {
	Date             string  `json:"date"`
	TransfersHandled int64   `json:"transfers_handled"`
	AvgResponseMins  float64 `json:"avg_response_mins"`
}

// AgentAnalyticsResponse is the full API response
type AgentAnalyticsResponse struct {
	Summary    AgentAnalyticsSummary   `json:"summary"`
	AgentStats []AgentPerformanceStats `json:"agent_stats,omitempty"`
	TrendData  []TrendPoint            `json:"trend_data"`
	MyStats    *AgentPerformanceStats  `json:"my_stats,omitempty"`
}

// GetAgentAnalytics returns agent analytics for the organization
// Agents see only their own stats; Admin/Manager see all agents
func (a *App) GetAgentAnalytics(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	// Parse date range
	fromStr := string(r.RequestCtx.QueryArgs().Peek("from"))
	toStr := string(r.RequestCtx.QueryArgs().Peek("to"))
	groupBy := string(r.RequestCtx.QueryArgs().Peek("group_by"))
	agentIDStr := string(r.RequestCtx.QueryArgs().Peek("agent_id"))
	if groupBy == "" {
		groupBy = "day"
	}

	now := time.Now()
	var periodStart, periodEnd time.Time

	if fromStr != "" && toStr != "" {
		var errMsg string
		periodStart, periodEnd, errMsg = parseDateRange(fromStr, toStr)
		if errMsg != "" {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, errMsg, nil, "")
		}
	} else {
		// Default to current month
		periodStart = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
		periodEnd = now
	}

	response := AgentAnalyticsResponse{
		Summary: AgentAnalyticsSummary{
			TransfersBySource: make(map[string]int64),
		},
		TrendData: []TrendPoint{},
	}

	// Check if filtering by specific agent (requires analytics permission)
	var filterAgentID *uuid.UUID
	if a.HasPermission(userID, models.ResourceAnalytics, models.ActionRead, orgID) && agentIDStr != "" {
		parsedID, err := uuid.Parse(agentIDStr)
		if err == nil {
			filterAgentID = &parsedID
		}
	}

	if filterAgentID != nil {
		// User with analytics permission viewing specific agent
		agentStats := a.calculateAgentStats(orgID, *filterAgentID, periodStart, periodEnd)
		response.MyStats = &agentStats
		response.TrendData = a.calculateTrendData(orgID, periodStart, periodEnd, groupBy, filterAgentID)
		// Calculate summary for this specific agent
		a.calculateAgentSummaryStats(orgID, *filterAgentID, periodStart, periodEnd, &response.Summary)
	} else if !a.HasPermission(userID, models.ResourceAnalytics, models.ActionRead, orgID) {
		// Users without analytics permission only see their own stats
		myStats := a.calculateAgentStats(orgID, userID, periodStart, periodEnd)
		response.MyStats = &myStats
		response.TrendData = a.calculateTrendData(orgID, periodStart, periodEnd, groupBy, &userID)
		a.calculateAgentSummaryStats(orgID, userID, periodStart, periodEnd, &response.Summary)
	} else {
		// Users with analytics permission see all agents
		a.calculateSummaryStats(orgID, periodStart, periodEnd, &response.Summary)
		response.TrendData = a.calculateTrendData(orgID, periodStart, periodEnd, groupBy, nil)
		response.AgentStats = a.calculateAllAgentStats(orgID, periodStart, periodEnd)
		// Also include current user's stats (for their own break time tracking)
		myStats := a.calculateAgentStats(orgID, userID, periodStart, periodEnd)
		response.MyStats = &myStats
	}

	return r.SendEnvelope(response)
}

// GetAgentDetails returns detailed analytics for a specific agent
func (a *App) GetAgentDetails(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	if err := a.requirePermission(r, userID, models.ResourceAnalytics, models.ActionRead); err != nil {
		return nil
	}

	agentID, err := parsePathUUID(r, "id", "agent")
	if err != nil {
		return nil
	}

	// Parse date range
	fromStr := string(r.RequestCtx.QueryArgs().Peek("from"))
	toStr := string(r.RequestCtx.QueryArgs().Peek("to"))
	groupBy := string(r.RequestCtx.QueryArgs().Peek("group_by"))
	if groupBy == "" {
		groupBy = "day"
	}

	now := time.Now()
	var periodStart, periodEnd time.Time

	if fromStr != "" && toStr != "" {
		var errMsg string
		periodStart, periodEnd, errMsg = parseDateRange(fromStr, toStr)
		if errMsg != "" {
			// Fall back to current month on parse error
			periodStart = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
			periodEnd = now
		}
	} else {
		periodStart = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
		periodEnd = now
	}

	// Verify agent exists
	_, err = findByIDAndOrg[models.User](a.DB, r, agentID, orgID, "Agent")
	if err != nil {
		return nil
	}

	stats := a.calculateAgentStats(orgID, agentID, periodStart, periodEnd)
	trendData := a.calculateTrendData(orgID, periodStart, periodEnd, groupBy, &agentID)

	return r.SendEnvelope(map[string]any{
		"agent":      stats,
		"trend_data": trendData,
	})
}

// GetAgentComparison returns comparison data for multiple agents
func (a *App) GetAgentComparison(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	if !a.HasPermission(userID, models.ResourceAnalytics, models.ActionRead, orgID) {
		return r.SendErrorEnvelope(fasthttp.StatusForbidden, "Access denied", nil, "")
	}

	// Parse date range
	fromStr := string(r.RequestCtx.QueryArgs().Peek("from"))
	toStr := string(r.RequestCtx.QueryArgs().Peek("to"))

	now := time.Now()
	var periodStart, periodEnd time.Time

	if fromStr != "" && toStr != "" {
		var errMsg string
		periodStart, periodEnd, errMsg = parseDateRange(fromStr, toStr)
		if errMsg != "" {
			// Fall back to current month on parse error
			periodStart = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
			periodEnd = now
		}
	} else {
		periodStart = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
		periodEnd = now
	}

	agentStats := a.calculateAllAgentStats(orgID, periodStart, periodEnd)

	return r.SendEnvelope(map[string]any{
		"agents": agentStats,
	})
}

// Helper functions

func (a *App) calculateSummaryStats(orgID uuid.UUID, start, end time.Time, summary *AgentAnalyticsSummary) {
	// Total transfers handled (resumed)
	a.DB.Model(&models.AgentTransfer{}).
		Where("organization_id = ? AND status = ? AND transferred_at >= ? AND transferred_at <= ?",
			orgID, models.TransferStatusResumed, start, end).
		Count(&summary.TotalTransfersHandled)

	// Active transfers
	a.DB.Model(&models.AgentTransfer{}).
		Where("organization_id = ? AND status = ?", orgID, models.TransferStatusActive).
		Count(&summary.ActiveTransfers)

	// Average queue time (time from transfer to assignment for assigned transfers)
	type AvgResult struct {
		Avg float64
	}
	var queueTimeResult AvgResult
	a.DB.Model(&models.AgentTransfer{}).
		Select("AVG(EXTRACT(EPOCH FROM (updated_at - transferred_at))/60) as avg").
		Where("organization_id = ? AND agent_id IS NOT NULL AND transferred_at >= ? AND transferred_at <= ?",
			orgID, start, end).
		Scan(&queueTimeResult)
	summary.AvgQueueTimeMins = queueTimeResult.Avg

	// Average resolution time (time from transfer to resume)
	var resolutionTimeResult AvgResult
	a.DB.Model(&models.AgentTransfer{}).
		Select("AVG(EXTRACT(EPOCH FROM (resumed_at - transferred_at))/60) as avg").
		Where("organization_id = ? AND status = ? AND resumed_at IS NOT NULL AND transferred_at >= ? AND transferred_at <= ?",
			orgID, models.TransferStatusResumed, start, end).
		Scan(&resolutionTimeResult)
	summary.AvgResolutionMins = resolutionTimeResult.Avg

	// Transfers by source
	type SourceCount struct {
		Source string
		Count  int64
	}
	var sourceCounts []SourceCount
	a.DB.Model(&models.AgentTransfer{}).
		Select("source, COUNT(*) as count").
		Where("organization_id = ? AND transferred_at >= ? AND transferred_at <= ?", orgID, start, end).
		Group("source").
		Scan(&sourceCounts)

	for _, sc := range sourceCounts {
		summary.TransfersBySource[sc.Source] = sc.Count
	}
}

func (a *App) calculateAgentSummaryStats(orgID, agentID uuid.UUID, start, end time.Time, summary *AgentAnalyticsSummary) {
	// Total transfers handled by this agent (resumed)
	a.DB.Model(&models.AgentTransfer{}).
		Where("organization_id = ? AND agent_id = ? AND status = ? AND transferred_at >= ? AND transferred_at <= ?",
			orgID, agentID, models.TransferStatusResumed, start, end).
		Count(&summary.TotalTransfersHandled)

	// Active transfers for this agent
	a.DB.Model(&models.AgentTransfer{}).
		Where("organization_id = ? AND agent_id = ? AND status = ?", orgID, agentID, models.TransferStatusActive).
		Count(&summary.ActiveTransfers)

	// Average resolution time for this agent
	type AvgResult struct {
		Avg float64
	}
	var resolutionTimeResult AvgResult
	a.DB.Model(&models.AgentTransfer{}).
		Select("AVG(EXTRACT(EPOCH FROM (resumed_at - transferred_at))/60) as avg").
		Where("organization_id = ? AND agent_id = ? AND status = ? AND resumed_at IS NOT NULL AND transferred_at >= ? AND transferred_at <= ?",
			orgID, agentID, models.TransferStatusResumed, start, end).
		Scan(&resolutionTimeResult)
	summary.AvgResolutionMins = resolutionTimeResult.Avg

	// Transfers by source for this agent
	type SourceCount struct {
		Source string
		Count  int64
	}
	var sourceCounts []SourceCount
	a.DB.Model(&models.AgentTransfer{}).
		Select("source, COUNT(*) as count").
		Where("organization_id = ? AND agent_id = ? AND transferred_at >= ? AND transferred_at <= ?", orgID, agentID, start, end).
		Group("source").
		Scan(&sourceCounts)

	for _, sc := range sourceCounts {
		summary.TransfersBySource[sc.Source] = sc.Count
	}

	// Calculate break time
	summary.TotalBreakTimeMins, summary.BreakCount = a.calculateBreakTime(agentID, start, end)
}

func (a *App) calculateAgentStats(orgID, agentID uuid.UUID, start, end time.Time) AgentPerformanceStats {
	stats := AgentPerformanceStats{
		AgentID: agentID.String(),
	}

	// Get agent name and availability
	var agent models.User
	if a.DB.Where("id = ?", agentID).First(&agent).Error == nil {
		stats.AgentName = agent.FullName
		stats.IsAvailable = agent.IsAvailable
	}

	// Transfers handled (resumed)
	a.DB.Model(&models.AgentTransfer{}).
		Where("organization_id = ? AND agent_id = ? AND status = ? AND transferred_at >= ? AND transferred_at <= ?",
			orgID, agentID, models.TransferStatusResumed, start, end).
		Count(&stats.TransfersHandled)

	// Active transfers
	a.DB.Model(&models.AgentTransfer{}).
		Where("organization_id = ? AND agent_id = ? AND status = ?", orgID, agentID, models.TransferStatusActive).
		Count(&stats.ActiveTransfers)

	// Messages sent - count outgoing messages to contacts during agent's active transfers
	// This captures all messages sent while the agent was handling the conversation
	a.DB.Model(&models.Message{}).
		Where("organization_id = ? AND direction = ? AND created_at >= ? AND created_at <= ?", orgID, models.DirectionOutgoing, start, end).
		Where("contact_id IN (SELECT contact_id FROM agent_transfers WHERE agent_id = ? AND organization_id = ?)", agentID, orgID).
		Count(&stats.MessagesSent)

	// Average resolution time
	type AvgResult struct {
		Avg float64
	}
	var resolutionTimeResult AvgResult
	a.DB.Model(&models.AgentTransfer{}).
		Select("AVG(EXTRACT(EPOCH FROM (resumed_at - transferred_at))/60) as avg").
		Where("organization_id = ? AND agent_id = ? AND status = ? AND resumed_at IS NOT NULL AND transferred_at >= ? AND transferred_at <= ?",
			orgID, agentID, models.TransferStatusResumed, start, end).
		Scan(&resolutionTimeResult)
	stats.AvgResolutionMins = resolutionTimeResult.Avg

	// Calculate break time from availability logs
	stats.TotalBreakTimeMins, stats.BreakCount = a.calculateBreakTime(agentID, start, end)

	// Check if currently on break and get break start time
	if !stats.IsAvailable {
		var currentBreak models.UserAvailabilityLog
		if a.DB.Where("user_id = ? AND is_available = false AND ended_at IS NULL", agentID).
			Order("started_at DESC").First(&currentBreak).Error == nil {
			breakStart := currentBreak.StartedAt.Format(time.RFC3339)
			stats.CurrentBreakStart = &breakStart
		}
	}

	return stats
}

func (a *App) calculateAllAgentStats(orgID uuid.UUID, start, end time.Time) []AgentPerformanceStats {
	// Get all agents in the organization through team membership
	var agents []models.User
	if err := a.DB.
		Joins("JOIN team_members ON team_members.user_id = users.id").
		Joins("JOIN teams ON teams.id = team_members.team_id").
		Where("users.organization_id = ? AND team_members.role = ?", orgID, models.TeamRoleAgent).
		Distinct().
		Find(&agents).Error; err != nil {
		a.Log.Error("Failed to fetch agents for analytics", "error", err, "org_id", orgID)
		return []AgentPerformanceStats{}
	}

	stats := make([]AgentPerformanceStats, 0, len(agents))
	for _, agent := range agents {
		agentStats := a.calculateAgentStats(orgID, agent.ID, start, end)
		stats = append(stats, agentStats)
	}

	return stats
}

// calculateBreakTime calculates total break time and count for an agent within a time period
func (a *App) calculateBreakTime(agentID uuid.UUID, start, end time.Time) (totalMins float64, count int64) {
	// Get all "away" periods that overlap with the time range
	var logs []models.UserAvailabilityLog
	if err := a.DB.Where("user_id = ? AND is_available = false AND started_at <= ? AND (ended_at >= ? OR ended_at IS NULL)",
		agentID, end, start).
		Find(&logs).Error; err != nil {
		a.Log.Error("Failed to fetch availability logs for break time calculation", "error", err, "agent_id", agentID)
		return 0, 0
	}

	for _, log := range logs {
		// Calculate the overlap with our time range
		logStart := log.StartedAt
		if logStart.Before(start) {
			logStart = start
		}

		var logEnd time.Time
		if log.EndedAt != nil {
			logEnd = *log.EndedAt
		} else {
			// Still on break, use current time but cap at end of period
			logEnd = time.Now()
		}
		if logEnd.After(end) {
			logEnd = end
		}

		// Add duration in minutes
		if logEnd.After(logStart) {
			duration := logEnd.Sub(logStart).Minutes()
			totalMins += duration
			count++
		}
	}

	return totalMins, count
}

func (a *App) calculateTrendData(orgID uuid.UUID, start, end time.Time, groupBy string, agentID *uuid.UUID) []TrendPoint {
	var dateFormat string
	var dateTrunc string

	switch groupBy {
	case "week":
		dateFormat = "2006-01-02"
		dateTrunc = "week"
	default: // day
		dateFormat = "2006-01-02"
		dateTrunc = "day"
	}

	type TrendResult struct {
		Date  time.Time
		Count int64
	}

	query := a.DB.Model(&models.AgentTransfer{}).
		Select("DATE_TRUNC('" + dateTrunc + "', transferred_at) as date, COUNT(*) as count").
		Where("organization_id = ? AND status = ? AND transferred_at >= ? AND transferred_at <= ?",
			orgID, models.TransferStatusResumed, start, end)

	if agentID != nil {
		query = query.Where("agent_id = ?", *agentID)
	}

	var results []TrendResult
	query.Group("DATE_TRUNC('" + dateTrunc + "', transferred_at)").
		Order("date ASC").
		Scan(&results)

	trendData := make([]TrendPoint, len(results))
	for i, r := range results {
		trendData[i] = TrendPoint{
			Date:             r.Date.Format(dateFormat),
			TransfersHandled: r.Count,
		}
	}

	return trendData
}
