package handlers

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
	"gorm.io/gorm"
)

// WidgetRequest represents the request body for creating/updating a widget
type WidgetRequest struct {
	Name        string        `json:"name"`
	Description string        `json:"description"`
	DataSource  string        `json:"data_source"`  // messages, contacts, campaigns, transfers, sessions
	Metric      string        `json:"metric"`       // count, sum, avg
	Field       string        `json:"field"`        // Field for sum/avg
	Filters     []FilterInput `json:"filters"`      // Filter conditions
	DisplayType  string        `json:"display_type"`   // number, percentage, chart
	ChartType    string        `json:"chart_type"`     // line, bar, pie
	GroupByField string        `json:"group_by_field"` // Field to group by
	ShowChange   *bool         `json:"show_change"`
	Color       string        `json:"color"`
	Size        string        `json:"size"` // small, medium, large
	IsShared    *bool         `json:"is_shared"`
}

// FilterInput represents a filter condition from the request
type FilterInput struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
}

// WidgetResponse represents the response for a widget
type WidgetResponse struct {
	ID           uuid.UUID     `json:"id"`
	Name         string        `json:"name"`
	Description  string        `json:"description"`
	DataSource   string        `json:"data_source"`
	Metric       string        `json:"metric"`
	Field        string        `json:"field"`
	Filters      []FilterInput `json:"filters"`
	DisplayType  string        `json:"display_type"`
	ChartType    string        `json:"chart_type"`
	GroupByField string        `json:"group_by_field"`
	ShowChange   bool          `json:"show_change"`
	Color        string        `json:"color"`
	Size         string        `json:"size"`
	DisplayOrder int           `json:"display_order"`
	IsShared     bool          `json:"is_shared"`
	IsDefault    bool          `json:"is_default"`
	IsOwner      bool          `json:"is_owner"` // True if current user created this widget
	CreatedBy    string        `json:"created_by"`
	CreatedAt    string        `json:"created_at"`
	UpdatedAt    string        `json:"updated_at"`
}

// WidgetDataResponse represents the computed data for a widget
type WidgetDataResponse struct {
	WidgetID      uuid.UUID          `json:"widget_id"`
	Value         float64            `json:"value"`
	Change        float64            `json:"change"`          // Percentage change from previous period
	ChartData     []ChartPoint       `json:"chart_data"`      // For chart display type
	PrevValue     float64            `json:"prev_value"`      // Previous period value
	DataPoints    []DataPoint        `json:"data_points"`     // Breakdown data
	GroupedSeries *GroupedSeriesData `json:"grouped_series"`  // For grouped time-series (line charts with group_by)
}

// GroupedSeriesData represents multiple datasets for grouped time-series charts
type GroupedSeriesData struct {
	Labels   []string              `json:"labels"`
	Datasets []GroupedSeriesDataset `json:"datasets"`
}

// GroupedSeriesDataset represents a single series in a grouped chart
type GroupedSeriesDataset struct {
	Label string    `json:"label"`
	Data  []float64 `json:"data"`
}

// ChartPoint represents a data point for charts
type ChartPoint struct {
	Label string  `json:"label"`
	Value float64 `json:"value"`
}

// DataPoint represents a breakdown data point
type DataPoint struct {
	Label string  `json:"label"`
	Value float64 `json:"value"`
	Color string  `json:"color,omitempty"`
}

// Available data sources and their filterable fields
var widgetDataSources = map[string][]string{
	"messages":  {"status", "direction", "message_type", "whatsapp_account"},
	"contacts":  {"whatsapp_account", "is_read"},
	"campaigns": {"status", "message_status"},
	"transfers": {"status", "source"},
	"sessions":  {"status"},
}

// Available metrics
var widgetMetrics = []string{"count", "sum", "avg"}

// Available display types
var widgetDisplayTypes = []string{"number", "percentage", "chart"}

// ListDashboardWidgets returns all widgets for the user (their own + shared)
func (a *App) ListDashboardWidgets(r *fastglue.Request) error {
	orgID, err := getOrganizationID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	userID, _ := r.RequestCtx.UserValue("user_id").(uuid.UUID)

	// Check analytics read permission
	if !a.HasPermission(userID, models.ResourceAnalytics, models.ActionRead) {
		return r.SendErrorEnvelope(fasthttp.StatusForbidden, "You don't have permission to view analytics", nil, "")
	}

	// Get user's own widgets + shared widgets from org
	var widgets []models.DashboardWidget
	if err := a.DB.Where(
		"organization_id = ? AND (user_id = ? OR is_shared = true)",
		orgID, userID,
	).Order("display_order ASC, created_at ASC").Find(&widgets).Error; err != nil {
		a.Log.Error("Failed to list dashboard widgets", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to list widgets", nil, "")
	}

	// Convert to response format
	response := make([]WidgetResponse, len(widgets))
	for i, w := range widgets {
		response[i] = widgetToResponse(w, userID)
	}

	return r.SendEnvelope(map[string]interface{}{
		"widgets": response,
	})
}

// GetDashboardWidget returns a single widget
func (a *App) GetDashboardWidget(r *fastglue.Request) error {
	orgID, err := getOrganizationID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	userID, _ := r.RequestCtx.UserValue("user_id").(uuid.UUID)

	// Check analytics read permission
	if !a.HasPermission(userID, models.ResourceAnalytics, models.ActionRead) {
		return r.SendErrorEnvelope(fasthttp.StatusForbidden, "You don't have permission to view analytics", nil, "")
	}

	idStr := r.RequestCtx.UserValue("id").(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid widget ID", nil, "")
	}

	var widget models.DashboardWidget
	if err := a.DB.Where(
		"id = ? AND organization_id = ? AND (user_id = ? OR is_shared = true)",
		id, orgID, userID,
	).First(&widget).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Widget not found", nil, "")
	}

	return r.SendEnvelope(widgetToResponse(widget, userID))
}

// CreateDashboardWidget creates a new widget
func (a *App) CreateDashboardWidget(r *fastglue.Request) error {
	orgID, err := getOrganizationID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	userID, _ := r.RequestCtx.UserValue("user_id").(uuid.UUID)

	// Check analytics write permission
	if !a.HasPermission(userID, models.ResourceAnalytics, models.ActionWrite) {
		return r.SendErrorEnvelope(fasthttp.StatusForbidden, "You don't have permission to create widgets", nil, "")
	}

	var req WidgetRequest
	if err := r.Decode(&req, "json"); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid request body", nil, "")
	}

	// Validate required fields
	if req.Name == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Name is required", nil, "")
	}
	if req.DataSource == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Data source is required", nil, "")
	}
	if req.Metric == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Metric is required", nil, "")
	}

	// Validate data source
	if _, ok := widgetDataSources[req.DataSource]; !ok {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid data source", nil, "")
	}

	// Validate metric
	if !contains(widgetMetrics, req.Metric) {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid metric", nil, "")
	}

	// Validate display type
	displayType := req.DisplayType
	if displayType == "" {
		displayType = "number"
	}
	if !contains(widgetDisplayTypes, displayType) {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid display type", nil, "")
	}

	// Get max display order
	var maxOrder int
	a.DB.Model(&models.DashboardWidget{}).
		Where("organization_id = ? AND user_id = ?", orgID, userID).
		Select("COALESCE(MAX(display_order), 0)").
		Scan(&maxOrder)

	// Convert filters to JSONBArray
	filters := make(models.JSONBArray, len(req.Filters))
	for i, f := range req.Filters {
		filters[i] = map[string]interface{}{
			"field":    f.Field,
			"operator": f.Operator,
			"value":    f.Value,
		}
	}

	showChange := true
	if req.ShowChange != nil {
		showChange = *req.ShowChange
	}

	isShared := false
	if req.IsShared != nil {
		isShared = *req.IsShared
	}

	size := req.Size
	if size == "" {
		size = "small"
	}

	// Validate group_by_field if provided
	if req.GroupByField != "" {
		fields := widgetDataSources[req.DataSource]
		if !contains(fields, req.GroupByField) {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid group by field for this data source", nil, "")
		}
	}

	widget := models.DashboardWidget{
		OrganizationID: orgID,
		UserID:         &userID,
		Name:           req.Name,
		Description:    req.Description,
		DataSource:     req.DataSource,
		Metric:         req.Metric,
		Field:          req.Field,
		Filters:        filters,
		DisplayType:    displayType,
		ChartType:      req.ChartType,
		GroupByField:   req.GroupByField,
		ShowChange:     showChange,
		Color:          req.Color,
		Size:           size,
		DisplayOrder:   maxOrder + 1,
		IsShared:       isShared,
	}

	if err := a.DB.Create(&widget).Error; err != nil {
		a.Log.Error("Failed to create dashboard widget", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to create widget", nil, "")
	}

	return r.SendEnvelope(widgetToResponse(widget, userID))
}

// UpdateDashboardWidget updates a widget
func (a *App) UpdateDashboardWidget(r *fastglue.Request) error {
	orgID, err := getOrganizationID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	userID, _ := r.RequestCtx.UserValue("user_id").(uuid.UUID)

	// Check analytics write permission
	if !a.HasPermission(userID, models.ResourceAnalytics, models.ActionWrite) {
		return r.SendErrorEnvelope(fasthttp.StatusForbidden, "You don't have permission to edit widgets", nil, "")
	}

	idStr := r.RequestCtx.UserValue("id").(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid widget ID", nil, "")
	}

	// Find the widget - must belong to same organization
	var widget models.DashboardWidget
	if err := a.DB.Where("id = ? AND organization_id = ?", id, orgID).First(&widget).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Widget not found", nil, "")
	}

	// Only the owner can edit the widget
	if widget.UserID == nil || *widget.UserID != userID {
		return r.SendErrorEnvelope(fasthttp.StatusForbidden, "Only the widget owner can edit this widget", nil, "")
	}

	var req WidgetRequest
	if err := r.Decode(&req, "json"); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid request body", nil, "")
	}

	// Update fields
	if req.Name != "" {
		widget.Name = req.Name
	}
	if req.Description != "" {
		widget.Description = req.Description
	}
	if req.DataSource != "" {
		if _, ok := widgetDataSources[req.DataSource]; !ok {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid data source", nil, "")
		}
		widget.DataSource = req.DataSource
	}
	if req.Metric != "" {
		if !contains(widgetMetrics, req.Metric) {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid metric", nil, "")
		}
		widget.Metric = req.Metric
	}
	if req.Field != "" {
		widget.Field = req.Field
	}
	if req.Filters != nil {
		filters := make(models.JSONBArray, len(req.Filters))
		for i, f := range req.Filters {
			filters[i] = map[string]interface{}{
				"field":    f.Field,
				"operator": f.Operator,
				"value":    f.Value,
			}
		}
		widget.Filters = filters
	}
	if req.DisplayType != "" {
		if !contains(widgetDisplayTypes, req.DisplayType) {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid display type", nil, "")
		}
		widget.DisplayType = req.DisplayType
	}
	if req.ChartType != "" {
		widget.ChartType = req.ChartType
	}
	// Always update group_by_field (empty string clears it)
	if req.GroupByField != "" {
		ds := widget.DataSource
		if req.DataSource != "" {
			ds = req.DataSource
		}
		fields := widgetDataSources[ds]
		if !contains(fields, req.GroupByField) {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid group by field for this data source", nil, "")
		}
	}
	widget.GroupByField = req.GroupByField
	if req.ShowChange != nil {
		widget.ShowChange = *req.ShowChange
	}
	if req.Color != "" {
		widget.Color = req.Color
	}
	if req.Size != "" {
		widget.Size = req.Size
	}
	if req.IsShared != nil {
		widget.IsShared = *req.IsShared
	}

	if err := a.DB.Save(&widget).Error; err != nil {
		a.Log.Error("Failed to update dashboard widget", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to update widget", nil, "")
	}

	return r.SendEnvelope(widgetToResponse(widget, userID))
}

// DeleteDashboardWidget deletes a widget
func (a *App) DeleteDashboardWidget(r *fastglue.Request) error {
	orgID, err := getOrganizationID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	userID, _ := r.RequestCtx.UserValue("user_id").(uuid.UUID)

	// Check analytics delete permission
	if !a.HasPermission(userID, models.ResourceAnalytics, models.ActionDelete) {
		return r.SendErrorEnvelope(fasthttp.StatusForbidden, "You don't have permission to delete widgets", nil, "")
	}

	idStr := r.RequestCtx.UserValue("id").(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid widget ID", nil, "")
	}

	// Find the widget - must belong to same organization
	var widget models.DashboardWidget
	if err := a.DB.Where("id = ? AND organization_id = ?", id, orgID).First(&widget).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Widget not found", nil, "")
	}

	// Only the owner can delete the widget
	if widget.UserID == nil || *widget.UserID != userID {
		return r.SendErrorEnvelope(fasthttp.StatusForbidden, "Only the widget owner can delete this widget", nil, "")
	}

	if err := a.DB.Delete(&widget).Error; err != nil {
		a.Log.Error("Failed to delete dashboard widget", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to delete widget", nil, "")
	}

	return r.SendEnvelope(map[string]string{"message": "Widget deleted successfully"})
}

// ReorderDashboardWidgets updates the display order of widgets
func (a *App) ReorderDashboardWidgets(r *fastglue.Request) error {
	orgID, err := getOrganizationID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	userID, _ := r.RequestCtx.UserValue("user_id").(uuid.UUID)

	var req struct {
		WidgetIDs []uuid.UUID `json:"widget_ids"`
	}
	if err := r.Decode(&req, "json"); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid request body", nil, "")
	}

	// Update order for each widget
	for i, widgetID := range req.WidgetIDs {
		a.DB.Model(&models.DashboardWidget{}).
			Where("id = ? AND organization_id = ? AND user_id = ?", widgetID, orgID, userID).
			Update("display_order", i)
	}

	return r.SendEnvelope(map[string]string{"message": "Widgets reordered successfully"})
}

// GetWidgetDataSources returns available data sources and their filterable fields
func (a *App) GetWidgetDataSources(r *fastglue.Request) error {
	sources := make([]map[string]interface{}, 0)
	for source, fields := range widgetDataSources {
		sources = append(sources, map[string]interface{}{
			"name":   source,
			"label":  formatLabel(source),
			"fields": fields,
		})
	}

	return r.SendEnvelope(map[string]interface{}{
		"data_sources":  sources,
		"metrics":       widgetMetrics,
		"display_types": widgetDisplayTypes,
		"operators": []map[string]string{
			{"value": "equals", "label": "Equals"},
			{"value": "not_equals", "label": "Not Equals"},
			{"value": "contains", "label": "Contains"},
			{"value": "gt", "label": "Greater Than"},
			{"value": "lt", "label": "Less Than"},
			{"value": "gte", "label": "Greater Than or Equal"},
			{"value": "lte", "label": "Less Than or Equal"},
		},
	})
}

// Helper functions

func widgetToResponse(w models.DashboardWidget, currentUserID uuid.UUID) WidgetResponse {
	// Parse filters from JSONBArray
	filters := make([]FilterInput, 0)
	for _, f := range w.Filters {
		if filterMap, ok := f.(map[string]interface{}); ok {
			filters = append(filters, FilterInput{
				Field:    widgetGetString(filterMap, "field"),
				Operator: widgetGetString(filterMap, "operator"),
				Value:    widgetGetString(filterMap, "value"),
			})
		}
	}

	return WidgetResponse{
		ID:           w.ID,
		Name:         w.Name,
		Description:  w.Description,
		DataSource:   w.DataSource,
		Metric:       w.Metric,
		Field:        w.Field,
		Filters:      filters,
		DisplayType:  w.DisplayType,
		ChartType:    w.ChartType,
		GroupByField: w.GroupByField,
		ShowChange:   w.ShowChange,
		Color:        w.Color,
		Size:         w.Size,
		DisplayOrder: w.DisplayOrder,
		IsShared:     w.IsShared,
		IsDefault:    w.IsDefault,
		IsOwner:      w.UserID != nil && *w.UserID == currentUserID,
		CreatedAt:    w.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:    w.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

func widgetGetString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func formatLabel(s string) string {
	s = strings.ReplaceAll(s, "_", " ")
	if len(s) > 0 {
		return strings.ToUpper(s[:1]) + s[1:]
	}
	return s
}

// GetWidgetData executes the widget query and returns the data
func (a *App) GetWidgetData(r *fastglue.Request) error {
	orgID, err := getOrganizationID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	userID, _ := r.RequestCtx.UserValue("user_id").(uuid.UUID)

	idStr := r.RequestCtx.UserValue("id").(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid widget ID", nil, "")
	}

	// Parse date range from query params
	fromStr := string(r.RequestCtx.QueryArgs().Peek("from"))
	toStr := string(r.RequestCtx.QueryArgs().Peek("to"))

	// Get the widget
	var widget models.DashboardWidget
	if err := a.DB.Where(
		"id = ? AND organization_id = ? AND (user_id = ? OR is_shared = true)",
		id, orgID, userID,
	).First(&widget).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Widget not found", nil, "")
	}

	// Execute the query
	data, err := a.executeWidgetQuery(orgID, widget, fromStr, toStr)
	if err != nil {
		a.Log.Error("Failed to execute widget query", "error", err, "widget_id", id)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to get widget data", nil, "")
	}

	data.WidgetID = widget.ID
	return r.SendEnvelope(data)
}

// GetAllWidgetsData returns data for all user's widgets in a single request
func (a *App) GetAllWidgetsData(r *fastglue.Request) error {
	orgID, err := getOrganizationID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	userID, _ := r.RequestCtx.UserValue("user_id").(uuid.UUID)

	// Parse date range from query params
	fromStr := string(r.RequestCtx.QueryArgs().Peek("from"))
	toStr := string(r.RequestCtx.QueryArgs().Peek("to"))

	// Get user's widgets
	var widgets []models.DashboardWidget
	if err := a.DB.Where(
		"organization_id = ? AND (user_id = ? OR is_shared = true)",
		orgID, userID,
	).Order("display_order ASC").Find(&widgets).Error; err != nil {
		a.Log.Error("Failed to list dashboard widgets", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to list widgets", nil, "")
	}

	// Execute queries for all widgets
	results := make(map[string]WidgetDataResponse)
	for _, widget := range widgets {
		data, err := a.executeWidgetQuery(orgID, widget, fromStr, toStr)
		if err != nil {
			a.Log.Error("Failed to execute widget query", "error", err, "widget_id", widget.ID)
			continue
		}
		data.WidgetID = widget.ID
		results[widget.ID.String()] = data
	}

	return r.SendEnvelope(map[string]interface{}{
		"data": results,
	})
}

// executeWidgetQuery executes the query for a widget and returns the data
func (a *App) executeWidgetQuery(orgID uuid.UUID, widget models.DashboardWidget, fromStr, toStr string) (WidgetDataResponse, error) {
	now := time.Now()

	var periodStart, periodEnd time.Time
	var err error

	if fromStr != "" && toStr != "" {
		periodStart, err = time.Parse("2006-01-02", fromStr)
		if err != nil {
			periodStart = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
		}
		periodEnd, err = time.Parse("2006-01-02", toStr)
		if err != nil {
			periodEnd = now
		}
		periodEnd = periodEnd.Add(24*time.Hour - time.Nanosecond)
	} else {
		// Default to current month
		periodStart = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
		periodEnd = now
	}

	// Calculate previous period for comparison
	periodDuration := periodEnd.Sub(periodStart)
	previousPeriodStart := periodStart.Add(-periodDuration - time.Nanosecond)
	previousPeriodEnd := periodStart.Add(-time.Nanosecond)

	response := WidgetDataResponse{}

	// Parse filters
	filters := make([]FilterInput, 0)
	for _, f := range widget.Filters {
		if filterMap, ok := f.(map[string]interface{}); ok {
			filters = append(filters, FilterInput{
				Field:    widgetGetString(filterMap, "field"),
				Operator: widgetGetString(filterMap, "operator"),
				Value:    widgetGetString(filterMap, "value"),
			})
		}
	}

	// Get the model and execute query based on data source
	var currentValue, previousValue float64

	switch widget.DataSource {
	case "messages":
		currentValue = a.queryMessages(orgID, widget.Metric, widget.Field, filters, periodStart, periodEnd)
		previousValue = a.queryMessages(orgID, widget.Metric, widget.Field, filters, previousPeriodStart, previousPeriodEnd)

	case "contacts":
		currentValue = a.queryContacts(orgID, widget.Metric, filters, periodStart, periodEnd)
		previousValue = a.queryContacts(orgID, widget.Metric, filters, previousPeriodStart, previousPeriodEnd)

	case "campaigns":
		currentValue = a.queryCampaigns(orgID, widget.Metric, filters, periodStart, periodEnd)
		previousValue = a.queryCampaigns(orgID, widget.Metric, filters, previousPeriodStart, previousPeriodEnd)

	case "transfers":
		currentValue = a.queryTransfers(orgID, widget.Metric, widget.Field, filters, periodStart, periodEnd)
		previousValue = a.queryTransfers(orgID, widget.Metric, widget.Field, filters, previousPeriodStart, previousPeriodEnd)

	case "sessions":
		currentValue = a.querySessions(orgID, widget.Metric, filters, periodStart, periodEnd)
		previousValue = a.querySessions(orgID, widget.Metric, filters, previousPeriodStart, previousPeriodEnd)
	}

	response.Value = currentValue
	response.PrevValue = previousValue
	response.Change = calculatePercentageChange(int64(previousValue), int64(currentValue))

	// Get chart data if display type is chart
	if widget.DisplayType == "chart" {
		if widget.GroupByField != "" {
			if widget.ChartType == "line" {
				// Line chart with group by → grouped time-series
				groupedSeries := a.getGroupedTimeSeriesData(orgID, widget, filters, periodStart, periodEnd)
				response.GroupedSeries = &groupedSeries
			} else {
				// Bar/Pie chart with group by → data points (group → count)
				response.DataPoints = a.getGroupedData(orgID, widget, filters, periodStart, periodEnd)
			}
		} else {
			response.ChartData = a.getChartData(orgID, widget, filters, periodStart, periodEnd)
		}
	}

	return response, nil
}

// Query helper functions for each data source
func (a *App) queryMessages(orgID uuid.UUID, metric, field string, filters []FilterInput, start, end time.Time) float64 {
	query := a.DB.Model(&models.Message{}).Where("organization_id = ? AND created_at >= ? AND created_at <= ?", orgID, start, end)

	// Apply filters
	for _, f := range filters {
		query = applyFilter(query, f)
	}

	var result float64
	switch metric {
	case "count":
		var count int64
		query.Count(&count)
		result = float64(count)
	case "sum", "avg":
		// For messages, sum/avg might be on a numeric field
		if field != "" {
			var val float64
			if metric == "sum" {
				query.Select("COALESCE(SUM(" + field + "), 0)").Scan(&val)
			} else {
				query.Select("COALESCE(AVG(" + field + "), 0)").Scan(&val)
			}
			result = val
		}
	}
	return result
}

func (a *App) queryContacts(orgID uuid.UUID, _ string, filters []FilterInput, start, end time.Time) float64 {
	// Filter by last_message_at to get "active" contacts with recent activity
	query := a.DB.Model(&models.Contact{}).Where("organization_id = ? AND last_message_at >= ? AND last_message_at <= ?", orgID, start, end)

	for _, f := range filters {
		query = applyFilter(query, f)
	}

	var count int64
	query.Count(&count)
	return float64(count)
}

func (a *App) queryCampaigns(orgID uuid.UUID, _ string, filters []FilterInput, start, end time.Time) float64 {
	query := a.DB.Model(&models.BulkMessageCampaign{}).Where("organization_id = ? AND created_at >= ? AND created_at <= ?", orgID, start, end)

	for _, f := range filters {
		query = applyFilter(query, f)
	}

	var count int64
	query.Count(&count)
	return float64(count)
}

func (a *App) queryTransfers(orgID uuid.UUID, metric, field string, filters []FilterInput, start, end time.Time) float64 {
	query := a.DB.Model(&models.AgentTransfer{}).Where("organization_id = ? AND transferred_at >= ? AND transferred_at <= ?", orgID, start, end)

	for _, f := range filters {
		query = applyFilter(query, f)
	}

	var result float64
	switch metric {
	case "count":
		var count int64
		query.Count(&count)
		result = float64(count)
	case "avg":
		if field == "resolution_time" {
			var val float64
			query.Where("status = ? AND resumed_at IS NOT NULL", models.TransferStatusResumed).
				Select("COALESCE(AVG(EXTRACT(EPOCH FROM (resumed_at - transferred_at))/60), 0)").
				Scan(&val)
			result = val
		}
	}
	return result
}

func (a *App) querySessions(orgID uuid.UUID, _ string, filters []FilterInput, start, end time.Time) float64 {
	query := a.DB.Model(&models.ChatbotSession{}).Where("organization_id = ? AND created_at >= ? AND created_at <= ?", orgID, start, end)

	for _, f := range filters {
		query = applyFilter(query, f)
	}

	var count int64
	query.Count(&count)
	return float64(count)
}

func (a *App) getChartData(orgID uuid.UUID, widget models.DashboardWidget, filters []FilterInput, start, end time.Time) []ChartPoint {
	chartData := make([]ChartPoint, 0)

	tableName, dateField, ok := resolveDataSourceTable(widget.DataSource)
	if !ok {
		return chartData
	}

	// Build raw query for daily aggregation
	query := fmt.Sprintf(`
		SELECT DATE_TRUNC('day', %s) as date, COUNT(*) as count
		FROM %s
		WHERE organization_id = ? AND %s >= ? AND %s <= ?
	`, dateField, tableName, dateField, dateField)

	args := []interface{}{orgID, start, end}
	query, args = appendFilterSQL(query, args, filters)

	query += fmt.Sprintf(" GROUP BY DATE_TRUNC('day', %s) ORDER BY date ASC", dateField)

	type DailyCount struct {
		Date  time.Time
		Count int64
	}

	var results []DailyCount
	a.DB.Raw(query, args...).Scan(&results)

	for _, r := range results {
		chartData = append(chartData, ChartPoint{
			Label: r.Date.Format("Jan 02"),
			Value: float64(r.Count),
		})
	}

	return chartData
}

// resolveDataSourceTable returns the table name and date field for a data source
func resolveDataSourceTable(dataSource string) (tableName, dateField string, ok bool) {
	switch dataSource {
	case "messages":
		return "messages", "created_at", true
	case "contacts":
		return "contacts", "last_message_at", true
	case "campaigns":
		return "bulk_message_campaigns", "created_at", true
	case "transfers":
		return "agent_transfers", "transferred_at", true
	case "sessions":
		return "chatbot_sessions", "created_at", true
	default:
		return "", "", false
	}
}

// appendFilterSQL appends filter conditions to a raw SQL query string and args slice
func appendFilterSQL(query string, args []interface{}, filters []FilterInput) (string, []interface{}) {
	for _, f := range filters {
		condition, value := buildFilterSQL(f)
		query += " AND " + condition
		args = append(args, value)
	}
	return query, args
}

// getGroupedData returns aggregated counts grouped by a field (for bar/pie charts)
func (a *App) getGroupedData(orgID uuid.UUID, widget models.DashboardWidget, filters []FilterInput, start, end time.Time) []DataPoint {
	dataPoints := make([]DataPoint, 0)

	// Special case: campaigns grouped by message_status uses pre-aggregated counters
	if widget.DataSource == "campaigns" && widget.GroupByField == "message_status" {
		return a.getCampaignMessageStatusData(orgID, filters, start, end)
	}

	tableName, dateField, ok := resolveDataSourceTable(widget.DataSource)
	if !ok {
		return dataPoints
	}

	query := fmt.Sprintf(`
		SELECT %s as label, COUNT(*) as value
		FROM %s
		WHERE organization_id = ? AND %s >= ? AND %s <= ?
	`, widget.GroupByField, tableName, dateField, dateField)

	args := []interface{}{orgID, start, end}
	query, args = appendFilterSQL(query, args, filters)

	query += fmt.Sprintf(" GROUP BY %s ORDER BY value DESC", widget.GroupByField)

	type GroupedCount struct {
		Label string
		Value int64
	}

	var results []GroupedCount
	a.DB.Raw(query, args...).Scan(&results)

	for _, r := range results {
		label := r.Label
		if label == "" {
			label = "(empty)"
		}
		dataPoints = append(dataPoints, DataPoint{
			Label: label,
			Value: float64(r.Value),
		})
	}

	return dataPoints
}

// getCampaignMessageStatusData returns sent/delivered/read/failed totals from campaign counters
func (a *App) getCampaignMessageStatusData(orgID uuid.UUID, filters []FilterInput, start, end time.Time) []DataPoint {
	query := `
		SELECT
			COALESCE(SUM(sent_count), 0) as sent,
			COALESCE(SUM(delivered_count), 0) as delivered,
			COALESCE(SUM(read_count), 0) as read_count,
			COALESCE(SUM(failed_count), 0) as failed
		FROM bulk_message_campaigns
		WHERE organization_id = ? AND created_at >= ? AND created_at <= ?
	`

	args := []interface{}{orgID, start, end}
	query, args = appendFilterSQL(query, args, filters)

	type CampaignCounts struct {
		Sent      int64
		Delivered int64
		ReadCount int64 `gorm:"column:read_count"`
		Failed    int64
	}

	var counts CampaignCounts
	a.DB.Raw(query, args...).Scan(&counts)

	return []DataPoint{
		{Label: "sent", Value: float64(counts.Sent)},
		{Label: "delivered", Value: float64(counts.Delivered)},
		{Label: "read", Value: float64(counts.ReadCount)},
		{Label: "failed", Value: float64(counts.Failed)},
	}
}

// getGroupedTimeSeriesData returns time-series data grouped by a field (for line charts with group_by)
func (a *App) getGroupedTimeSeriesData(orgID uuid.UUID, widget models.DashboardWidget, filters []FilterInput, start, end time.Time) GroupedSeriesData {
	result := GroupedSeriesData{
		Labels:   make([]string, 0),
		Datasets: make([]GroupedSeriesDataset, 0),
	}

	// Special case: campaigns grouped by message_status over time
	if widget.DataSource == "campaigns" && widget.GroupByField == "message_status" {
		return a.getCampaignMessageStatusTimeSeries(orgID, filters, start, end)
	}

	tableName, dateField, ok := resolveDataSourceTable(widget.DataSource)
	if !ok {
		return result
	}

	query := fmt.Sprintf(`
		SELECT DATE_TRUNC('day', %s) as date, %s as group_value, COUNT(*) as count
		FROM %s
		WHERE organization_id = ? AND %s >= ? AND %s <= ?
	`, dateField, widget.GroupByField, tableName, dateField, dateField)

	args := []interface{}{orgID, start, end}
	query, args = appendFilterSQL(query, args, filters)

	query += fmt.Sprintf(" GROUP BY DATE_TRUNC('day', %s), %s ORDER BY date ASC", dateField, widget.GroupByField)

	type GroupedRow struct {
		Date       time.Time
		GroupValue string
		Count      int64
	}

	var rows []GroupedRow
	a.DB.Raw(query, args...).Scan(&rows)

	// Collect unique dates and groups
	dateSet := make(map[string]bool)
	groupSet := make(map[string]bool)
	dateOrder := make([]string, 0)
	groupOrder := make([]string, 0)

	for _, row := range rows {
		dateLabel := row.Date.Format("Jan 02")
		if !dateSet[dateLabel] {
			dateSet[dateLabel] = true
			dateOrder = append(dateOrder, dateLabel)
		}
		gv := row.GroupValue
		if gv == "" {
			gv = "(empty)"
		}
		if !groupSet[gv] {
			groupSet[gv] = true
			groupOrder = append(groupOrder, gv)
		}
	}

	result.Labels = dateOrder

	// Build a lookup: group → date → count
	lookup := make(map[string]map[string]float64)
	for _, row := range rows {
		gv := row.GroupValue
		if gv == "" {
			gv = "(empty)"
		}
		dateLabel := row.Date.Format("Jan 02")
		if lookup[gv] == nil {
			lookup[gv] = make(map[string]float64)
		}
		lookup[gv][dateLabel] = float64(row.Count)
	}

	// Build datasets
	for _, group := range groupOrder {
		data := make([]float64, len(dateOrder))
		for i, dateLabel := range dateOrder {
			data[i] = lookup[group][dateLabel]
		}
		result.Datasets = append(result.Datasets, GroupedSeriesDataset{
			Label: group,
			Data:  data,
		})
	}

	return result
}

// getCampaignMessageStatusTimeSeries returns daily sent/delivered/read/failed from campaign counters over time
func (a *App) getCampaignMessageStatusTimeSeries(orgID uuid.UUID, filters []FilterInput, start, end time.Time) GroupedSeriesData {
	result := GroupedSeriesData{
		Labels:   make([]string, 0),
		Datasets: make([]GroupedSeriesDataset, 0),
	}

	query := `
		SELECT DATE_TRUNC('day', created_at) as date,
			COALESCE(SUM(sent_count), 0) as sent,
			COALESCE(SUM(delivered_count), 0) as delivered,
			COALESCE(SUM(read_count), 0) as read_count,
			COALESCE(SUM(failed_count), 0) as failed
		FROM bulk_message_campaigns
		WHERE organization_id = ? AND created_at >= ? AND created_at <= ?
	`

	args := []interface{}{orgID, start, end}
	query, args = appendFilterSQL(query, args, filters)

	query += " GROUP BY DATE_TRUNC('day', created_at) ORDER BY date ASC"

	type DailyCampaignCounts struct {
		Date      time.Time
		Sent      int64
		Delivered int64
		ReadCount int64 `gorm:"column:read_count"`
		Failed    int64
	}

	var rows []DailyCampaignCounts
	a.DB.Raw(query, args...).Scan(&rows)

	labels := make([]string, len(rows))
	sentData := make([]float64, len(rows))
	deliveredData := make([]float64, len(rows))
	readData := make([]float64, len(rows))
	failedData := make([]float64, len(rows))

	for i, row := range rows {
		labels[i] = row.Date.Format("Jan 02")
		sentData[i] = float64(row.Sent)
		deliveredData[i] = float64(row.Delivered)
		readData[i] = float64(row.ReadCount)
		failedData[i] = float64(row.Failed)
	}

	result.Labels = labels
	result.Datasets = []GroupedSeriesDataset{
		{Label: "sent", Data: sentData},
		{Label: "delivered", Data: deliveredData},
		{Label: "read", Data: readData},
		{Label: "failed", Data: failedData},
	}

	return result
}

func applyFilter(query *gorm.DB, filter FilterInput) *gorm.DB {
	condition, value := buildFilterSQL(filter)
	return query.Where(condition, value)
}

func buildFilterSQL(filter FilterInput) (string, interface{}) {
	field := filter.Field
	value := filter.Value

	switch filter.Operator {
	case "equals":
		return fmt.Sprintf("%s = ?", field), value
	case "not_equals":
		return fmt.Sprintf("%s != ?", field), value
	case "contains":
		return fmt.Sprintf("%s ILIKE ?", field), "%" + value + "%"
	case "gt":
		return fmt.Sprintf("%s > ?", field), value
	case "lt":
		return fmt.Sprintf("%s < ?", field), value
	case "gte":
		return fmt.Sprintf("%s >= ?", field), value
	case "lte":
		return fmt.Sprintf("%s <= ?", field), value
	default:
		return fmt.Sprintf("%s = ?", field), value
	}
}
