package handlers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/pkg/whatsapp"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

// FlowRequest represents the request body for creating/updating a flow
type FlowRequest struct {
	WhatsAppAccount string                 `json:"whatsapp_account" validate:"required"`
	Name            string                 `json:"name" validate:"required"`
	Category        string                 `json:"category"`
	JSONVersion     string                 `json:"json_version"`
	FlowJSON        map[string]interface{} `json:"flow_json"`
	Screens         []interface{}          `json:"screens"`
}

// FlowResponse represents the response for a flow
type FlowResponse struct {
	ID              uuid.UUID              `json:"id"`
	WhatsAppAccount string                 `json:"whatsapp_account"`
	MetaFlowID      string                 `json:"meta_flow_id"`
	Name            string                 `json:"name"`
	Status          string                 `json:"status"`
	Category        string                 `json:"category"`
	JSONVersion     string                 `json:"json_version"`
	FlowJSON        map[string]interface{} `json:"flow_json"`
	Screens         []interface{}          `json:"screens"`
	PreviewURL      string                 `json:"preview_url"`
	HasLocalChanges bool                   `json:"has_local_changes"`
	CreatedAt       string                 `json:"created_at"`
	UpdatedAt       string                 `json:"updated_at"`
}

// ListFlows returns all flows for the organization
func (a *App) ListFlows(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	pg := parsePagination(r)

	// Optional filters
	accountName := string(r.RequestCtx.QueryArgs().Peek("account"))
	status := string(r.RequestCtx.QueryArgs().Peek("status"))
	search := string(r.RequestCtx.QueryArgs().Peek("search"))

	query := a.DB.Where("organization_id = ?", orgID)

	if accountName != "" {
		query = query.Where("whats_app_account = ?", accountName)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if search != "" {
		searchPattern := "%" + search + "%"
		// Search by flow name (case-insensitive)
		query = query.Where("name ILIKE ?", searchPattern)
	}

	var total int64
	query.Model(&models.WhatsAppFlow{}).Count(&total)

	var flows []models.WhatsAppFlow
	if err := pg.Apply(query.Order("created_at DESC")).
		Find(&flows).Error; err != nil {
		a.Log.Error("Failed to list flows", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to list flows", nil, "")
	}

	response := make([]FlowResponse, len(flows))
	for i, f := range flows {
		response[i] = flowToResponse(f)
	}

	return r.SendEnvelope(map[string]any{
		"flows": response,
		"total": total,
		"page":  pg.Page,
		"limit": pg.Limit,
	})
}

// CreateFlow creates a new WhatsApp flow
func (a *App) CreateFlow(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	var req FlowRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}

	// Validate required fields
	if req.Name == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Name is required", nil, "")
	}
	if req.WhatsAppAccount == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "WhatsApp account is required", nil, "")
	}

	// Verify account exists and belongs to org
	var account models.WhatsAppAccount
	if err := a.DB.Where("organization_id = ? AND name = ?", orgID, req.WhatsAppAccount).First(&account).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "WhatsApp account not found", nil, "")
	}

	// Set defaults
	jsonVersion := req.JSONVersion
	if jsonVersion == "" {
		jsonVersion = "6.0"
	}

	flow := models.WhatsAppFlow{
		OrganizationID:  orgID,
		WhatsAppAccount: req.WhatsAppAccount,
		Name:            req.Name,
		Status:          "DRAFT",
		Category:        req.Category,
		JSONVersion:     jsonVersion,
		FlowJSON:        models.JSONB(req.FlowJSON),
		Screens:         models.JSONBArray(req.Screens),
	}

	if err := a.DB.Create(&flow).Error; err != nil {
		a.Log.Error("Failed to create flow", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to create flow", nil, "")
	}

	a.Log.Info("Flow created", "flow_id", flow.ID, "name", flow.Name)

	return r.SendEnvelope(map[string]interface{}{
		"flow": flowToResponse(flow),
	})
}

// GetFlow returns a single flow by ID
func (a *App) GetFlow(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	id, err := parsePathUUID(r, "id", "flow")
	if err != nil {
		return nil
	}

	flow, err := findByIDAndOrg[models.WhatsAppFlow](a.DB, r, id, orgID, "Flow")
	if err != nil {
		return nil
	}

	return r.SendEnvelope(map[string]interface{}{
		"flow": flowToResponse(*flow),
	})
}

// UpdateFlow updates an existing flow
func (a *App) UpdateFlow(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	id, err := parsePathUUID(r, "id", "flow")
	if err != nil {
		return nil
	}

	flow, err := findByIDAndOrg[models.WhatsAppFlow](a.DB, r, id, orgID, "Flow")
	if err != nil {
		return nil
	}

	var req FlowRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}

	// Update fields
	updates := map[string]interface{}{}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Category != "" {
		updates["category"] = req.Category
	}
	if req.JSONVersion != "" {
		updates["json_version"] = req.JSONVersion
	}
	if req.FlowJSON != nil {
		updates["flow_json"] = models.JSONB(req.FlowJSON)
	}
	if req.Screens != nil {
		updates["screens"] = models.JSONBArray(req.Screens)
	}

	if len(updates) > 0 {
		// Mark as having local changes that need to be synced to Meta
		updates["has_local_changes"] = true
		if err := a.DB.Model(flow).Updates(updates).Error; err != nil {
			a.Log.Error("Failed to update flow", "error", err, "flow_id", id)
			return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to update flow", nil, "")
		}
	}

	// Reload flow
	a.DB.First(flow, id)

	a.Log.Info("Flow updated", "flow_id", flow.ID)

	return r.SendEnvelope(map[string]interface{}{
		"flow": flowToResponse(*flow),
	})
}

// DeleteFlow deletes a flow
func (a *App) DeleteFlow(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	id, err := parsePathUUID(r, "id", "flow")
	if err != nil {
		return nil
	}

	flow, err := findByIDAndOrg[models.WhatsAppFlow](a.DB, r, id, orgID, "Flow")
	if err != nil {
		return nil
	}

	// Delete the flow (soft delete)
	if err := a.DB.Delete(flow).Error; err != nil {
		a.Log.Error("Failed to delete flow", "error", err, "flow_id", id)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to delete flow", nil, "")
	}

	a.Log.Info("Flow deleted", "flow_id", id)

	return r.SendEnvelope(map[string]interface{}{
		"message": "Flow deleted successfully",
	})
}

// SaveFlowToMeta saves/updates a flow to Meta (keeps it in DRAFT status on Meta)
func (a *App) SaveFlowToMeta(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	id, err := parsePathUUID(r, "id", "flow")
	if err != nil {
		return nil
	}

	flow, err := findByIDAndOrg[models.WhatsAppFlow](a.DB, r, id, orgID, "Flow")
	if err != nil {
		return nil
	}

	// Deprecated flows cannot be updated
	if flow.Status == "DEPRECATED" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Deprecated flows cannot be updated", nil, "")
	}

	// Get the WhatsApp account
	var account models.WhatsAppAccount
	if err := a.DB.Where("organization_id = ? AND name = ?", orgID, flow.WhatsAppAccount).First(&account).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "WhatsApp account not found", nil, "")
	}

	// Create WhatsApp API client
	waClient := whatsapp.New(a.Log)
	waAccount := a.toWhatsAppAccount(&account)

	a.Log.Info("SaveFlowToMeta: Account details",
		"account_name", account.Name,
		"phone_id", account.PhoneID,
		"business_id", account.BusinessID,
		"api_version", account.APIVersion,
		"flow_name", flow.Name,
		"flow_category", flow.Category)

	ctx := context.Background()

	// Step 1: Create flow in Meta (if not already created)
	var metaFlowID string
	if flow.MetaFlowID == "" {
		categories := []string{}
		if flow.Category != "" {
			categories = append(categories, flow.Category)
		}

		a.Log.Info("SaveFlowToMeta: Creating flow in Meta", "name", flow.Name, "categories", categories)
		metaFlowID, err = waClient.CreateFlow(ctx, waAccount, flow.Name, categories)
		if err != nil {
			a.Log.Error("Failed to create flow in Meta", "error", err, "flow_id", id, "business_id", account.BusinessID)
			return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to create flow in Meta", nil, "")
		}
	} else {
		metaFlowID = flow.MetaFlowID
	}

	// Step 2: Upload flow JSON if we have screens
	if len(flow.Screens) > 0 {
		// Validate flow structure before sending to Meta
		if err := validateFlowStructure([]interface{}(flow.Screens)); err != nil {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, err.Error(), nil, "")
		}

		// Sanitize screens before sending to Meta
		sanitizedScreens := sanitizeScreensForMeta([]interface{}(flow.Screens))

		flowJSON := &whatsapp.FlowJSON{
			Version: flow.JSONVersion,
			Screens: sanitizedScreens,
		}

		if err := waClient.UpdateFlowJSON(ctx, waAccount, metaFlowID, flowJSON); err != nil {
			a.Log.Error("Failed to update flow JSON in Meta", "error", err, "flow_id", id, "meta_flow_id", metaFlowID)
			// Save the meta flow ID even if JSON update fails
			a.DB.Model(flow).Updates(map[string]interface{}{
				"meta_flow_id": metaFlowID,
			})
			return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to update flow JSON", nil, "")
		}
	}

	// Update local database with meta flow ID and set status to DRAFT
	// (updating on Meta creates a new draft version that needs to be published)
	if err := a.DB.Model(flow).Updates(map[string]interface{}{
		"meta_flow_id":      metaFlowID,
		"status":            "DRAFT",
		"has_local_changes": false,
	}).Error; err != nil {
		a.Log.Error("Failed to update flow", "error", err, "flow_id", id)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to update flow", nil, "")
	}

	// Reload flow
	a.DB.First(flow, id)

	a.Log.Info("Flow saved to Meta", "flow_id", flow.ID, "meta_flow_id", metaFlowID)

	return r.SendEnvelope(map[string]interface{}{
		"flow":    flowToResponse(*flow),
		"message": "Flow saved to Meta successfully",
	})
}

// PublishFlow publishes a flow to Meta
func (a *App) PublishFlow(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	id, err := parsePathUUID(r, "id", "flow")
	if err != nil {
		return nil
	}

	flow, err := findByIDAndOrg[models.WhatsAppFlow](a.DB, r, id, orgID, "Flow")
	if err != nil {
		return nil
	}

	// Only DRAFT flows can be published
	if flow.Status != "DRAFT" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Only DRAFT flows can be published", nil, "")
	}

	// Flow must be saved to Meta first
	if flow.MetaFlowID == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Flow must be saved to Meta first before publishing", nil, "")
	}

	// Get the WhatsApp account
	var account models.WhatsAppAccount
	if err := a.DB.Where("organization_id = ? AND name = ?", orgID, flow.WhatsAppAccount).First(&account).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "WhatsApp account not found", nil, "")
	}

	// Create WhatsApp API client
	waClient := whatsapp.New(a.Log)
	waAccount := a.toWhatsAppAccount(&account)

	ctx := context.Background()

	// Publish the flow
	if err := waClient.PublishFlow(ctx, waAccount, flow.MetaFlowID); err != nil {
		a.Log.Error("Failed to publish flow in Meta", "error", err, "flow_id", id, "meta_flow_id", flow.MetaFlowID)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to publish flow", nil, "")
	}

	// Get the flow details including preview URL
	metaFlow, err := waClient.GetFlow(ctx, waAccount, flow.MetaFlowID)
	previewURL := ""
	if err == nil && metaFlow != nil {
		previewURL = metaFlow.PreviewURL
	}

	// Update local database
	if err := a.DB.Model(flow).Updates(map[string]interface{}{
		"status":      "PUBLISHED",
		"preview_url": previewURL,
	}).Error; err != nil {
		a.Log.Error("Failed to update flow status", "error", err, "flow_id", id)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to update flow status", nil, "")
	}

	// Reload flow
	a.DB.First(flow, id)

	a.Log.Info("Flow published to Meta", "flow_id", flow.ID, "meta_flow_id", flow.MetaFlowID)

	return r.SendEnvelope(map[string]interface{}{
		"flow":    flowToResponse(*flow),
		"message": "Flow published successfully",
	})
}

// DeprecateFlow deprecates a published flow
func (a *App) DeprecateFlow(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	id, err := parsePathUUID(r, "id", "flow")
	if err != nil {
		return nil
	}

	flow, err := findByIDAndOrg[models.WhatsAppFlow](a.DB, r, id, orgID, "Flow")
	if err != nil {
		return nil
	}

	// Only PUBLISHED flows can be deprecated
	if flow.Status != "PUBLISHED" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Only PUBLISHED flows can be deprecated", nil, "")
	}

	// Call Meta API to deprecate the flow if we have a Meta flow ID
	if flow.MetaFlowID != "" {
		// Get the WhatsApp account
		var account models.WhatsAppAccount
		if err := a.DB.Where("organization_id = ? AND name = ?", orgID, flow.WhatsAppAccount).First(&account).Error; err != nil {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "WhatsApp account not found", nil, "")
		}

		waClient := whatsapp.New(a.Log)
		waAccount := a.toWhatsAppAccount(&account)

		ctx := context.Background()
		if err := waClient.DeprecateFlow(ctx, waAccount, flow.MetaFlowID); err != nil {
			a.Log.Error("Failed to deprecate flow in Meta", "error", err, "flow_id", id, "meta_flow_id", flow.MetaFlowID)
			return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to deprecate flow in Meta", nil, "")
		}
	}

	if err := a.DB.Model(flow).Updates(map[string]interface{}{
		"status": "DEPRECATED",
	}).Error; err != nil {
		a.Log.Error("Failed to deprecate flow", "error", err, "flow_id", id)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to deprecate flow", nil, "")
	}

	// Reload flow
	a.DB.First(flow, id)

	a.Log.Info("Flow deprecated", "flow_id", flow.ID)

	return r.SendEnvelope(map[string]interface{}{
		"flow":    flowToResponse(*flow),
		"message": "Flow deprecated successfully",
	})
}

// DuplicateFlow creates a copy of an existing flow as a new DRAFT
// This is useful for editing published flows - duplicate, edit, then publish the new one
func (a *App) DuplicateFlow(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	id, err := parsePathUUID(r, "id", "flow")
	if err != nil {
		return nil
	}

	flow, err := findByIDAndOrg[models.WhatsAppFlow](a.DB, r, id, orgID, "Flow")
	if err != nil {
		return nil
	}

	// Create a duplicate with a new name
	newFlow := models.WhatsAppFlow{
		OrganizationID:  orgID,
		WhatsAppAccount: flow.WhatsAppAccount,
		Name:            flow.Name + " (Copy)",
		Status:          "DRAFT",
		Category:        flow.Category,
		JSONVersion:     flow.JSONVersion,
		FlowJSON:        flow.FlowJSON,
		Screens:         flow.Screens,
		// MetaFlowID is intentionally left empty - this is a new flow
	}

	if err := a.DB.Create(&newFlow).Error; err != nil {
		a.Log.Error("Failed to duplicate flow", "error", err, "original_flow_id", id)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to duplicate flow", nil, "")
	}

	a.Log.Info("Flow duplicated", "original_flow_id", id, "new_flow_id", newFlow.ID)

	return r.SendEnvelope(map[string]interface{}{
		"flow":    flowToResponse(newFlow),
		"message": "Flow duplicated successfully. You can now edit and publish the new flow.",
	})
}

// SyncFlows syncs flows from Meta for a specific WhatsApp account
func (a *App) SyncFlows(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	// Get account name from request
	var req struct {
		WhatsAppAccount string `json:"whatsapp_account"`
	}
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}

	if req.WhatsAppAccount == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "WhatsApp account is required", nil, "")
	}

	// Get the WhatsApp account
	var account models.WhatsAppAccount
	if err := a.DB.Where("organization_id = ? AND name = ?", orgID, req.WhatsAppAccount).First(&account).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "WhatsApp account not found", nil, "")
	}

	// Create WhatsApp API client
	waClient := whatsapp.New(a.Log)
	waAccount := a.toWhatsAppAccount(&account)

	ctx := context.Background()

	// Fetch flows from Meta
	metaFlows, err := waClient.ListFlows(ctx, waAccount)
	if err != nil {
		a.Log.Error("Failed to fetch flows from Meta", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to fetch flows from Meta", nil, "")
	}

	// Sync each flow
	synced := 0
	created := 0
	updated := 0

	for _, mf := range metaFlows {
		var existingFlow models.WhatsAppFlow
		err := a.DB.Where("organization_id = ? AND meta_flow_id = ?", orgID, mf.ID).First(&existingFlow).Error

		category := ""
		if len(mf.Categories) > 0 {
			category = mf.Categories[0]
		}

		// Fetch flow assets (JSON) from Meta
		var flowJSON models.JSONB
		var screens models.JSONBArray
		var jsonVersion string

		flowAssets, assetsErr := waClient.GetFlowAssets(ctx, waAccount, mf.ID)
		if assetsErr != nil {
			a.Log.Warn("Failed to fetch flow assets", "error", assetsErr, "meta_flow_id", mf.ID)
			// Continue without assets - flow will be synced without screens
		} else if flowAssets != nil {
			// Convert flow assets to JSONB
			flowJSONBytes, _ := json.Marshal(flowAssets)
			_ = json.Unmarshal(flowJSONBytes, &flowJSON)

			// Extract screens
			screensBytes, _ := json.Marshal(flowAssets.Screens)
			_ = json.Unmarshal(screensBytes, &screens)

			jsonVersion = flowAssets.Version
		}

		if err != nil {
			// Flow doesn't exist locally, create it
			newFlow := models.WhatsAppFlow{
				OrganizationID:  orgID,
				WhatsAppAccount: req.WhatsAppAccount,
				MetaFlowID:      mf.ID,
				Name:            mf.Name,
				Status:          mf.Status,
				Category:        category,
				PreviewURL:      mf.PreviewURL,
				FlowJSON:        flowJSON,
				Screens:         screens,
				JSONVersion:     jsonVersion,
			}
			if err := a.DB.Create(&newFlow).Error; err != nil {
				a.Log.Error("Failed to create flow from Meta", "error", err, "meta_flow_id", mf.ID)
				continue
			}
			created++
		} else {
			// Flow exists, update it
			updates := map[string]interface{}{
				"name":        mf.Name,
				"status":      mf.Status,
				"category":    category,
				"preview_url": mf.PreviewURL,
			}
			// Only update flow JSON if we got new assets
			if flowAssets != nil {
				updates["flow_json"] = flowJSON
				updates["screens"] = screens
				updates["json_version"] = jsonVersion
			}
			if err := a.DB.Model(&existingFlow).Updates(updates).Error; err != nil {
				a.Log.Error("Failed to update flow from Meta", "error", err, "flow_id", existingFlow.ID)
				continue
			}
			updated++
		}
		synced++
	}

	a.Log.Info("Flows synced from Meta", "total", synced, "created", created, "updated", updated)

	return r.SendEnvelope(map[string]interface{}{
		"message": "Flows synced successfully",
		"synced":  synced,
		"created": created,
		"updated": updated,
	})
}

// validateFlowStructure validates the flow structure before sending to Meta
// - Ensures at least one screen has a Footer with "complete" action
// - If multiple screens, only the last screen should have "complete" action
func validateFlowStructure(screens []interface{}) error {
	if len(screens) == 0 {
		return fmt.Errorf("flow must have at least one screen")
	}

	// Find which screens have complete action
	screensWithComplete := []int{}
	for i, screen := range screens {
		screenMap, ok := screen.(map[string]interface{})
		if !ok {
			continue
		}

		layout, ok := screenMap["layout"].(map[string]interface{})
		if !ok {
			continue
		}

		children, ok := layout["children"].([]interface{})
		if !ok {
			continue
		}

		if hasCompleteAction(children) {
			screensWithComplete = append(screensWithComplete, i)
		}
	}

	// Check if any screen has a complete action
	if len(screensWithComplete) == 0 {
		return fmt.Errorf("flow must have a Footer button with 'Complete Flow' action: add a Footer component to your last screen and set its action to 'Complete Flow'")
	}

	// If multiple screens, complete action should only be on the last screen
	if len(screens) > 1 {
		lastScreenIndex := len(screens) - 1
		for _, idx := range screensWithComplete {
			if idx != lastScreenIndex {
				return fmt.Errorf("'Complete Flow' action should only be on the last screen. Screen %d has a complete action but it's not the last screen. Use 'Navigate to Screen' action for intermediate screens", idx+1)
			}
		}
	}

	return nil
}

// componentsWithoutID lists component types that should NOT have an 'id' property when sent to Meta API
var componentsWithoutID = map[string]bool{
	"TextHeading":       true,
	"TextSubheading":    true,
	"TextBody":          true,
	"TextInput":         true,
	"TextArea":          true,
	"Dropdown":          true,
	"RadioButtonsGroup": true,
	"CheckboxGroup":     true,
	"DatePicker":        true,
	"Image":             true,
	"Footer":            true,
}

// sanitizeScreensForMeta sanitizes flow screens before sending to Meta API
// - Fixes screen IDs to only use alphabets and underscores
// - Removes 'id' property from components that don't support it
// - Marks screens with 'complete' action as terminal screens
// - Auto-populates the complete action's payload with all form field values
func sanitizeScreensForMeta(screens []interface{}) []interface{} {
	// First pass: collect form field names per screen
	screenFields := collectFormFieldsPerScreen(screens)
	allFieldNames := collectFormFieldNames(screens)

	result := make([]interface{}, len(screens))

	// Track cumulative fields from previous screens
	var fieldsFromPreviousScreens []string

	for i, screen := range screens {
		screenMap, ok := screen.(map[string]interface{})
		if !ok {
			result[i] = screen
			continue
		}

		// Create a new screen map
		newScreen := make(map[string]interface{})
		for k, v := range screenMap {
			newScreen[k] = v
		}

		// Fix screen ID if it contains numbers
		if id, ok := newScreen["id"].(string); ok {
			newScreen["id"] = sanitizeID(id)
		}

		// Add data model for fields from previous screens (required for multi-screen flows)
		if i > 0 && len(fieldsFromPreviousScreens) > 0 {
			dataModel := make(map[string]interface{})
			// Copy existing data model if present
			if existingData, ok := newScreen["data"].(map[string]interface{}); ok {
				for k, v := range existingData {
					dataModel[k] = v
				}
			}
			// Add entries for fields from previous screens
			for _, fieldName := range fieldsFromPreviousScreens {
				dataModel[fieldName] = map[string]interface{}{
					"type":        "string",
					"__example__": "",
				}
			}
			newScreen["data"] = dataModel
		}

		// Sanitize layout children and check for terminal action
		isTerminal := false
		if layout, ok := newScreen["layout"].(map[string]interface{}); ok {
			newLayout := make(map[string]interface{})
			for k, v := range layout {
				newLayout[k] = v
			}

			if children, ok := layout["children"].([]interface{}); ok {
				// Sanitize and auto-populate action payloads
				sanitizedChildren := sanitizeComponentsWithPayload(children, allFieldNames, fieldsFromPreviousScreens)
				newLayout["children"] = sanitizedChildren

				// Check if any child has on-click-action with name "complete"
				isTerminal = hasCompleteAction(sanitizedChildren)
			}

			newScreen["layout"] = newLayout
		}

		// Mark screen as terminal if it has a complete action
		if isTerminal {
			newScreen["terminal"] = true
		}

		result[i] = newScreen

		// Add this screen's fields to cumulative list for next screens
		if fields, ok := screenFields[i]; ok {
			fieldsFromPreviousScreens = append(fieldsFromPreviousScreens, fields...)
		}
	}

	return result
}

// collectFormFieldNames collects all form field names from all screens
// These are components that have a "name" attribute (TextInput, TextArea, Dropdown, etc.)
func collectFormFieldNames(screens []interface{}) []string {
	var fieldNames []string

	for _, screen := range screens {
		screenMap, ok := screen.(map[string]interface{})
		if !ok {
			continue
		}

		layout, ok := screenMap["layout"].(map[string]interface{})
		if !ok {
			continue
		}

		children, ok := layout["children"].([]interface{})
		if !ok {
			continue
		}

		for _, child := range children {
			compMap, ok := child.(map[string]interface{})
			if !ok {
				continue
			}

			// Check if component has a "name" attribute (form field)
			if name, ok := compMap["name"].(string); ok && name != "" {
				// Sanitize the name to match what will be sent to Meta
				sanitizedName := sanitizeID(name)
				fieldNames = append(fieldNames, sanitizedName)
			}
		}
	}

	return fieldNames
}

// collectFormFieldsPerScreen collects form field names for each screen by index
func collectFormFieldsPerScreen(screens []interface{}) map[int][]string {
	result := make(map[int][]string)

	for i, screen := range screens {
		screenMap, ok := screen.(map[string]interface{})
		if !ok {
			continue
		}

		layout, ok := screenMap["layout"].(map[string]interface{})
		if !ok {
			continue
		}

		children, ok := layout["children"].([]interface{})
		if !ok {
			continue
		}

		var fieldNames []string
		for _, child := range children {
			compMap, ok := child.(map[string]interface{})
			if !ok {
				continue
			}

			// Check if component has a "name" attribute (form field)
			if name, ok := compMap["name"].(string); ok && name != "" {
				// Sanitize the name to match what will be sent to Meta
				sanitizedName := sanitizeID(name)
				fieldNames = append(fieldNames, sanitizedName)
			}
		}

		if len(fieldNames) > 0 {
			result[i] = fieldNames
		}
	}

	return result
}

// hasCompleteAction checks if any component has an on-click-action with name "complete"
func hasCompleteAction(children []interface{}) bool {
	for _, child := range children {
		compMap, ok := child.(map[string]interface{})
		if !ok {
			continue
		}

		if action, ok := compMap["on-click-action"].(map[string]interface{}); ok {
			if name, ok := action["name"].(string); ok && name == "complete" {
				return true
			}
		}
	}
	return false
}

// sanitizeID converts an ID to use only alphabets and underscores
// e.g., "SCREEN_1" -> "SCREEN_A", "id_1234_abc" -> "id_abcd_abc"
func sanitizeID(id string) string {
	// Check if ID already only contains valid characters
	valid := true
	for _, c := range id {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_') { //nolint:staticcheck // More readable than De Morgan's law
			valid = false
			break
		}
	}
	if valid {
		return id
	}

	// Replace numbers with letters
	result := make([]byte, 0, len(id))
	for _, c := range id {
		if c >= '0' && c <= '9' {
			// Convert 0-9 to A-J
			result = append(result, byte('A'+c-'0'))
		} else if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_' {
			result = append(result, byte(c))
		}
		// Skip other characters
	}

	return string(result)
}

// sanitizeComponentsWithPayload sanitizes components and auto-populates action payloads
// - For navigate actions: passes current screen's form fields using ${form.fieldName}
// - For complete actions: uses ${data.fieldName} for previous screens, ${form.fieldName} for current
func sanitizeComponentsWithPayload(children []interface{}, allFieldNames []string, fieldsFromPreviousScreens []string) []interface{} {
	result := make([]interface{}, len(children))

	// Collect this screen's field names
	var thisScreenFields []string
	for _, child := range children {
		compMap, ok := child.(map[string]interface{})
		if !ok {
			continue
		}
		if name, ok := compMap["name"].(string); ok && name != "" {
			thisScreenFields = append(thisScreenFields, sanitizeID(name))
		}
	}

	// Create a set for quick lookup of this screen's fields
	thisScreenFieldSet := make(map[string]bool)
	for _, f := range thisScreenFields {
		thisScreenFieldSet[f] = true
	}

	for i, child := range children {
		compMap, ok := child.(map[string]interface{})
		if !ok {
			result[i] = child
			continue
		}

		// Create a new component map
		newComp := make(map[string]interface{})
		for k, v := range compMap {
			newComp[k] = v
		}

		// Check if this component type should not have an id
		compType, _ := newComp["type"].(string)
		if componentsWithoutID[compType] {
			delete(newComp, "id")
		}

		// Sanitize name field if it contains numbers
		if name, ok := newComp["name"].(string); ok {
			newComp["name"] = sanitizeID(name)
		}

		// Sanitize data-source option IDs
		if dataSource, ok := newComp["data-source"].([]interface{}); ok {
			newDataSource := make([]interface{}, len(dataSource))
			for j, opt := range dataSource {
				if optMap, ok := opt.(map[string]interface{}); ok {
					newOpt := make(map[string]interface{})
					for k, v := range optMap {
						newOpt[k] = v
					}
					if optID, ok := newOpt["id"].(string); ok {
						newOpt["id"] = sanitizeID(optID)
					}
					newDataSource[j] = newOpt
				} else {
					newDataSource[j] = opt
				}
			}
			newComp["data-source"] = newDataSource
		}

		// Auto-populate action payloads
		if action, ok := newComp["on-click-action"].(map[string]interface{}); ok {
			actionName, _ := action["name"].(string)

			newAction := make(map[string]interface{})
			for k, v := range action {
				newAction[k] = v
			}

			switch actionName {
			case "complete":
				// Complete action: include all form fields from all screens
				// - Fields from previous screens: use ${data.fieldName} (passed via data model)
				// - Fields on current screen: use ${form.fieldName} (form input)
				payload := make(map[string]interface{})
				for _, fieldName := range allFieldNames {
					if thisScreenFieldSet[fieldName] {
						// Current screen's field - use form reference
						payload[fieldName] = "${form." + fieldName + "}"
					} else {
						// Previous screen's field - use data reference
						payload[fieldName] = "${data." + fieldName + "}"
					}
				}
				newAction["payload"] = payload
			case "navigate":
				// Navigate action: pass current screen's form fields to next screen
				// Use ${form.fieldName} for current screen's fields
				if len(thisScreenFields) > 0 {
					payload := make(map[string]interface{})
					// Pass previous screen data through
					for _, fieldName := range fieldsFromPreviousScreens {
						payload[fieldName] = "${data." + fieldName + "}"
					}
					// Add current screen's form fields
					for _, fieldName := range thisScreenFields {
						payload[fieldName] = "${form." + fieldName + "}"
					}
					newAction["payload"] = payload
				}
			}

			newComp["on-click-action"] = newAction
		}

		result[i] = newComp
	}

	return result
}

// flowToResponse converts a flow model to response
func flowToResponse(f models.WhatsAppFlow) FlowResponse {
	return FlowResponse{
		ID:              f.ID,
		WhatsAppAccount: f.WhatsAppAccount,
		MetaFlowID:      f.MetaFlowID,
		Name:            f.Name,
		Status:          f.Status,
		Category:        f.Category,
		JSONVersion:     f.JSONVersion,
		FlowJSON:        map[string]interface{}(f.FlowJSON),
		Screens:         []interface{}(f.Screens),
		PreviewURL:      f.PreviewURL,
		HasLocalChanges: f.HasLocalChanges,
		CreatedAt:       f.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:       f.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}
