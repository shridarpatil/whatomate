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
	WhatsAppAccount string         `json:"whatsapp_account" validate:"required"`
	Name            string         `json:"name" validate:"required"`
	Category        string         `json:"category"`
	JSONVersion     string         `json:"json_version"`
	FlowJSON        map[string]any `json:"flow_json"`
	Screens         []any          `json:"screens"`
}

// FlowResponse represents the response for a flow
type FlowResponse struct {
	ID              uuid.UUID      `json:"id"`
	WhatsAppAccount string         `json:"whatsapp_account"`
	MetaFlowID      string         `json:"meta_flow_id"`
	Name            string         `json:"name"`
	Status          string         `json:"status"`
	Category        string         `json:"category"`
	JSONVersion     string         `json:"json_version"`
	FlowJSON        map[string]any `json:"flow_json"`
	Screens         []any          `json:"screens"`
	PreviewURL      string         `json:"preview_url"`
	HasLocalChanges bool           `json:"has_local_changes"`
	CreatedAt       string         `json:"created_at"`
	UpdatedAt       string         `json:"updated_at"`
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

	return r.SendEnvelope(listEnvelope("flows", response, total, pg))
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
	if _, err := a.resolveWhatsAppAccount(orgID, req.WhatsAppAccount); err != nil {
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

	return r.SendEnvelope(map[string]any{
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

	return r.SendEnvelope(map[string]any{
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
	updates := map[string]any{}
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

	return r.SendEnvelope(map[string]any{
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

	return r.SendEnvelope(map[string]any{
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
	account, err := a.resolveWhatsAppAccount(orgID, flow.WhatsAppAccount)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "WhatsApp account not found", nil, "")
	}

	// Create WhatsApp API client
	waClient := whatsapp.New(a.Log)
	waAccount := a.toWhatsAppAccount(account)

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
			return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, fmt.Sprintf("Failed to create flow in Meta: %s", err.Error()), nil, "")
		}
	} else {
		metaFlowID = flow.MetaFlowID
	}

	// Step 2: Upload flow JSON if we have screens
	if len(flow.Screens) > 0 {
		// Validate flow structure before sending to Meta
		if err := validateFlowStructure([]any(flow.Screens)); err != nil {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, err.Error(), nil, "")
		}

		// Sanitize screens before sending to Meta
		sanitizedScreens := sanitizeScreensForMeta([]any(flow.Screens))

		flowJSON := &whatsapp.FlowJSON{
			Version: flow.JSONVersion,
			Screens: sanitizedScreens,
		}

		if err := waClient.UpdateFlowJSON(ctx, waAccount, metaFlowID, flowJSON); err != nil {
			a.Log.Error("Failed to update flow JSON in Meta", "error", err, "flow_id", id, "meta_flow_id", metaFlowID)
			// Save the meta flow ID even if JSON update fails
			a.DB.Model(flow).Updates(map[string]any{
				"meta_flow_id": metaFlowID,
			})
			return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, fmt.Sprintf("Failed to update flow JSON: %s", err.Error()), nil, "")
		}
	}

	// Update local database with meta flow ID and set status to DRAFT
	// (updating on Meta creates a new draft version that needs to be published)
	if err := a.DB.Model(flow).Updates(map[string]any{
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

	return r.SendEnvelope(map[string]any{
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
	account, err := a.resolveWhatsAppAccount(orgID, flow.WhatsAppAccount)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "WhatsApp account not found", nil, "")
	}

	// Create WhatsApp API client
	waClient := whatsapp.New(a.Log)
	waAccount := a.toWhatsAppAccount(account)

	ctx := context.Background()

	// Publish the flow
	if err := waClient.PublishFlow(ctx, waAccount, flow.MetaFlowID); err != nil {
		a.Log.Error("Failed to publish flow in Meta", "error", err, "flow_id", id, "meta_flow_id", flow.MetaFlowID)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, fmt.Sprintf("Failed to publish flow: %s", err.Error()), nil, "")
	}

	// Get the flow details including preview URL
	metaFlow, err := waClient.GetFlow(ctx, waAccount, flow.MetaFlowID)
	previewURL := ""
	if err == nil && metaFlow != nil {
		previewURL = metaFlow.PreviewURL
	}

	// Update local database
	if err := a.DB.Model(flow).Updates(map[string]any{
		"status":      "PUBLISHED",
		"preview_url": previewURL,
	}).Error; err != nil {
		a.Log.Error("Failed to update flow status", "error", err, "flow_id", id)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to update flow status", nil, "")
	}

	// Reload flow
	a.DB.First(flow, id)

	a.Log.Info("Flow published to Meta", "flow_id", flow.ID, "meta_flow_id", flow.MetaFlowID)

	return r.SendEnvelope(map[string]any{
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
		account, err := a.resolveWhatsAppAccount(orgID, flow.WhatsAppAccount)
		if err != nil {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "WhatsApp account not found", nil, "")
		}

		waClient := whatsapp.New(a.Log)
		waAccount := a.toWhatsAppAccount(account)

		ctx := context.Background()
		if err := waClient.DeprecateFlow(ctx, waAccount, flow.MetaFlowID); err != nil {
			a.Log.Error("Failed to deprecate flow in Meta", "error", err, "flow_id", id, "meta_flow_id", flow.MetaFlowID)
			return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to deprecate flow in Meta", nil, "")
		}
	}

	if err := a.DB.Model(flow).Updates(map[string]any{
		"status": "DEPRECATED",
	}).Error; err != nil {
		a.Log.Error("Failed to deprecate flow", "error", err, "flow_id", id)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to deprecate flow", nil, "")
	}

	// Reload flow
	a.DB.First(flow, id)

	a.Log.Info("Flow deprecated", "flow_id", flow.ID)

	return r.SendEnvelope(map[string]any{
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

	return r.SendEnvelope(map[string]any{
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
	account, err := a.resolveWhatsAppAccount(orgID, req.WhatsAppAccount)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "WhatsApp account not found", nil, "")
	}

	// Create WhatsApp API client
	waClient := whatsapp.New(a.Log)
	waAccount := a.toWhatsAppAccount(account)

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
			updates := map[string]any{
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

	return r.SendEnvelope(map[string]any{
		"message": "Flows synced successfully",
		"synced":  synced,
		"created": created,
		"updated": updated,
	})
}

// validateFlowStructure validates the flow structure before sending to Meta.
// If no screen has a Footer with "complete" action, one is auto-injected on
// the last screen so the user doesn't have to manually add it.
func validateFlowStructure(screens []any) error {
	if len(screens) == 0 {
		return fmt.Errorf("flow must have at least one screen")
	}

	// Check if any screen already has a complete action
	for _, screen := range screens {
		screenMap, ok := screen.(map[string]any)
		if !ok {
			continue
		}

		layout, ok := screenMap["layout"].(map[string]any)
		if !ok {
			continue
		}

		children, ok := layout["children"].([]any)
		if !ok {
			continue
		}

		if hasCompleteAction(children) {
			return nil // Already has a complete action — nothing to do
		}
	}

	// No screen has a complete action — auto-inject a Footer on the last screen
	lastScreen, ok := screens[len(screens)-1].(map[string]any)
	if !ok {
		return fmt.Errorf("last screen has invalid structure")
	}

	layout, ok := lastScreen["layout"].(map[string]any)
	if !ok {
		layout = map[string]any{"type": "SingleColumnLayout", "children": []any{}}
		lastScreen["layout"] = layout
	}

	children, ok := layout["children"].([]any)
	if !ok {
		children = []any{}
	}

	footer := map[string]any{
		"type":  "Footer",
		"label": "Complete",
		"on-click-action": map[string]any{
			"name":    "complete",
			"payload": map[string]any{},
		},
	}

	layout["children"] = append(children, footer)

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
// - Correctly handles branching flows (If/Else navigation) by tracing the navigation graph
func sanitizeScreensForMeta(screens []any) []any {
	// First pass: collect form field names per screen (by sanitized screen ID)
	screenFieldsByID := make(map[string][]string) // screenID -> field names on that screen
	screenIDOrder := make([]string, 0, len(screens))
	screenByID := make(map[string]int) // screenID -> index

	for i, screen := range screens {
		screenMap, ok := screen.(map[string]any)
		if !ok {
			continue
		}
		screenID := ""
		if id, ok := screenMap["id"].(string); ok {
			screenID = sanitizeID(id)
		}
		screenIDOrder = append(screenIDOrder, screenID)
		screenByID[screenID] = i

		layout, ok := screenMap["layout"].(map[string]any)
		if !ok {
			continue
		}
		children, ok := layout["children"].([]any)
		if !ok {
			continue
		}
		fields := collectFormFieldNamesFromChildren(children)
		// Sanitize the field names
		for j, f := range fields {
			fields[j] = sanitizeID(f)
		}
		screenFieldsByID[screenID] = fields
	}

	// Second pass: build navigation graph (which screens navigate TO which screens)
	// incomingFields[screenID] = fields that should be in its data model (from screens that navigate to it)
	incomingFields := make(map[string]map[string]bool) // screenID -> set of field names

	for _, screen := range screens {
		screenMap, ok := screen.(map[string]any)
		if !ok {
			continue
		}
		sourceID := ""
		if id, ok := screenMap["id"].(string); ok {
			sourceID = sanitizeID(id)
		}

		layout, ok := screenMap["layout"].(map[string]any)
		if !ok {
			continue
		}
		children, ok := layout["children"].([]any)
		if !ok {
			continue
		}

		// Find all navigate targets from this screen (including inside If/Else)
		targets := collectNavigateTargets(children)
		sourceFields := screenFieldsByID[sourceID]

		for _, targetID := range targets {
			targetID = sanitizeID(targetID)
			if _, exists := incomingFields[targetID]; !exists {
				incomingFields[targetID] = make(map[string]bool)
			}
			// The target screen needs all fields from the source screen in its data model
			for _, f := range sourceFields {
				incomingFields[targetID][f] = true
			}
		}
	}

	// Propagate: if screen A navigates to screen B, and screen B navigates to screen C,
	// then C needs fields from both A and B. We already have B's own fields.
	// But also need to propagate A's fields through B to C.
	// Do a simple BFS/propagation pass
	changed := true
	for changed {
		changed = false
		for _, screen := range screens {
			screenMap, ok := screen.(map[string]any)
			if !ok {
				continue
			}
			sourceID := ""
			if id, ok := screenMap["id"].(string); ok {
				sourceID = sanitizeID(id)
			}
			layout, ok := screenMap["layout"].(map[string]any)
			if !ok {
				continue
			}
			children, ok := layout["children"].([]any)
			if !ok {
				continue
			}
			targets := collectNavigateTargets(children)

			// Fields available at this screen = its own fields + fields in its data model (from incoming)
			allAvailable := make(map[string]bool)
			for _, f := range screenFieldsByID[sourceID] {
				allAvailable[f] = true
			}
			if incoming, ok := incomingFields[sourceID]; ok {
				for f := range incoming {
					allAvailable[f] = true
				}
			}

			for _, targetID := range targets {
				targetID = sanitizeID(targetID)
				if _, exists := incomingFields[targetID]; !exists {
					incomingFields[targetID] = make(map[string]bool)
				}
				for f := range allAvailable {
					if !incomingFields[targetID][f] {
						incomingFields[targetID][f] = true
						changed = true
					}
				}
			}
		}
	}

	// Third pass: build the result
	result := make([]any, len(screens))

	for i, screen := range screens {
		screenMap, ok := screen.(map[string]any)
		if !ok {
			result[i] = screen
			continue
		}

		// Create a new screen map
		newScreen := make(map[string]any)
		for k, v := range screenMap {
			newScreen[k] = v
		}

		// Fix screen ID if it contains numbers
		screenID := ""
		if id, ok := newScreen["id"].(string); ok {
			screenID = sanitizeID(id)
			newScreen["id"] = screenID
		}

		// Set data model based on navigation graph (not sequential order)
		if i > 0 {
			dataModel := make(map[string]any)
			// Copy existing data model if present
			if existingData, ok := newScreen["data"].(map[string]any); ok {
				for k, v := range existingData {
					dataModel[k] = v
				}
			}
			// Add entries for fields coming from navigating screens
			if incoming, ok := incomingFields[screenID]; ok {
				for fieldName := range incoming {
					if _, exists := dataModel[fieldName]; !exists {
						dataModel[fieldName] = map[string]any{
							"type":        "string",
							"__example__": "",
						}
					}
				}
			}
			if len(dataModel) > 0 {
				newScreen["data"] = dataModel
			}
		}

		// Get the fields that are actually in this screen's data model
		dataModelFields := make(map[string]bool)
		if dm, ok := newScreen["data"].(map[string]any); ok {
			for k := range dm {
				dataModelFields[k] = true
			}
		}

		// Get this screen's own form fields
		thisScreenFields := screenFieldsByID[screenID]

		// Sanitize layout children and check for terminal action
		isTerminal := false
		if layout, ok := newScreen["layout"].(map[string]any); ok {
			newLayout := make(map[string]any)
			for k, v := range layout {
				newLayout[k] = v
			}

			if children, ok := layout["children"].([]any); ok {
				// Sanitize and auto-populate action payloads using ONLY known fields
				sanitizedChildren := sanitizeComponentsWithPayloadV2(children, thisScreenFields, dataModelFields)
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
	}

	return result
}

// collectNavigateTargets extracts all screen IDs that are navigation targets in children (including If/Else)
func collectNavigateTargets(children []any) []string {
	var targets []string
	for _, child := range children {
		compMap, ok := child.(map[string]any)
		if !ok {
			continue
		}

		if action, ok := compMap["on-click-action"].(map[string]any); ok {
			if name, ok := action["name"].(string); ok && name == "navigate" {
				if next, ok := action["next"].(map[string]any); ok {
					if screenName, ok := next["name"].(string); ok {
						targets = append(targets, screenName)
					}
				}
			}
		}

		if compMap["type"] == "If" {
			if thenBlock, ok := compMap["then"].([]any); ok {
				targets = append(targets, collectNavigateTargets(thenBlock)...)
			}
			if elseBlock, ok := compMap["else"].([]any); ok {
				targets = append(targets, collectNavigateTargets(elseBlock)...)
			}
		}
	}
	return targets
}

// sanitizeComponentsWithPayloadV2 sanitizes components using only the known fields
// - For navigate actions: passes current screen's form fields and data model fields
// - For complete actions: uses ${data.fieldName} for data model fields, ${form.fieldName} for current screen
func sanitizeComponentsWithPayloadV2(children []any, thisScreenFields []string, dataModelFields map[string]bool) []any {
	thisScreenFieldSet := make(map[string]bool)
	for _, f := range thisScreenFields {
		thisScreenFieldSet[f] = true
	}

	return sanitizeComponentsRecursiveV2(children, thisScreenFields, dataModelFields, thisScreenFieldSet)
}

func sanitizeComponentsRecursiveV2(children []any, thisScreenFields []string, dataModelFields map[string]bool, thisScreenFieldSet map[string]bool) []any {
	result := make([]any, len(children))

	for i, child := range children {
		compMap, ok := child.(map[string]any)
		if !ok {
			result[i] = child
			continue
		}

		// Create a new component map
		newComp := make(map[string]any)
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
		if dataSource, ok := newComp["data-source"].([]any); ok {
			newDataSource := make([]any, len(dataSource))
			for j, opt := range dataSource {
				if optMap, ok := opt.(map[string]any); ok {
					newOpt := make(map[string]any)
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
		if action, ok := newComp["on-click-action"].(map[string]any); ok {
			actionName, _ := action["name"].(string)

			newAction := make(map[string]any)
			for k, v := range action {
				newAction[k] = v
			}

			switch actionName {
			case "complete":
				payload := make(map[string]any)

				// Preserve existing payload
				if existingPayload, ok := newAction["payload"].(map[string]any); ok {
					for k, v := range existingPayload {
						payload[k] = v
					}
				}

				// Add data model fields (from previous screens) using ${data.xxx}
				for fieldName := range dataModelFields {
					if _, exists := payload[fieldName]; !exists {
						payload[fieldName] = "${data." + fieldName + "}"
					}
				}
				// Add this screen's own fields using ${form.xxx}
				for _, fieldName := range thisScreenFields {
					if _, exists := payload[fieldName]; !exists {
						payload[fieldName] = "${form." + fieldName + "}"
					}
				}

				newAction["payload"] = payload
			case "navigate":
				if len(thisScreenFields) > 0 || len(dataModelFields) > 0 {
					payload := make(map[string]any)

					// Preserve existing payload
					if existingPayload, ok := newAction["payload"].(map[string]any); ok {
						for k, v := range existingPayload {
							payload[k] = v
						}
					}

					// Pass data model fields forward
					for fieldName := range dataModelFields {
						if _, exists := payload[fieldName]; !exists {
							payload[fieldName] = "${data." + fieldName + "}"
						}
					}
					// Pass this screen's form fields
					for _, fieldName := range thisScreenFields {
						if _, exists := payload[fieldName]; !exists {
							payload[fieldName] = "${form." + fieldName + "}"
						}
					}

					newAction["payload"] = payload
				}
			}

			newComp["on-click-action"] = newAction
		}

		if compType == "If" {
			if thenBlock, ok := newComp["then"].([]any); ok {
				newComp["then"] = sanitizeComponentsRecursiveV2(thenBlock, thisScreenFields, dataModelFields, thisScreenFieldSet)
			}
			if elseBlock, ok := newComp["else"].([]any); ok {
				newComp["else"] = sanitizeComponentsRecursiveV2(elseBlock, thisScreenFields, dataModelFields, thisScreenFieldSet)
			}
		}

		result[i] = newComp
	}

	return result
}

func collectFormFieldNamesFromChildren(children []any) []string {
	var fieldNames []string
	for _, child := range children {
		compMap, ok := child.(map[string]any)
		if !ok {
			continue
		}

		if name, ok := compMap["name"].(string); ok && name != "" {
			sanitizedName := sanitizeID(name)
			fieldNames = append(fieldNames, sanitizedName)
		}

		if compMap["type"] == "If" {
			if thenBlock, ok := compMap["then"].([]any); ok {
				fieldNames = append(fieldNames, collectFormFieldNamesFromChildren(thenBlock)...)
			}
			if elseBlock, ok := compMap["else"].([]any); ok {
				fieldNames = append(fieldNames, collectFormFieldNamesFromChildren(elseBlock)...)
			}
		}
	}
	return fieldNames
}

// collectFormFieldNames collects all form field names from all screens
// These are components that have a "name" attribute (TextInput, TextArea, Dropdown, etc.)
func collectFormFieldNames(screens []any) []string {
	var fieldNames []string

	for _, screen := range screens {
		screenMap, ok := screen.(map[string]any)
		if !ok {
			continue
		}

		layout, ok := screenMap["layout"].(map[string]any)
		if !ok {
			continue
		}

		children, ok := layout["children"].([]any)
		if !ok {
			continue
		}

		fieldNames = append(fieldNames, collectFormFieldNamesFromChildren(children)...)
	}

	return fieldNames
}

// collectFormFieldsPerScreen collects form field names for each screen by index
func collectFormFieldsPerScreen(screens []any) map[int][]string {
	result := make(map[int][]string)

	for i, screen := range screens {
		screenMap, ok := screen.(map[string]any)
		if !ok {
			continue
		}

		layout, ok := screenMap["layout"].(map[string]any)
		if !ok {
			continue
		}

		children, ok := layout["children"].([]any)
		if !ok {
			continue
		}

		fieldNames := collectFormFieldNamesFromChildren(children)

		if len(fieldNames) > 0 {
			result[i] = fieldNames
		}
	}

	return result
}

// hasCompleteAction checks if any component has an on-click-action with name "complete"
func hasCompleteAction(children []any) bool {
	for _, child := range children {
		compMap, ok := child.(map[string]any)
		if !ok {
			continue
		}

		if action, ok := compMap["on-click-action"].(map[string]any); ok {
			if name, ok := action["name"].(string); ok && name == "complete" {
				return true
			}
		}

		if compMap["type"] == "If" {
			if thenBlock, ok := compMap["then"].([]any); ok {
				if hasCompleteAction(thenBlock) {
					return true
				}
			}
			if elseBlock, ok := compMap["else"].([]any); ok {
				if hasCompleteAction(elseBlock) {
					return true
				}
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
func sanitizeComponentsWithPayload(children []any, allFieldNames []string, fieldsFromPreviousScreens []string) []any {
	// Collect this screen's field names
	thisScreenFields := collectFormFieldNamesFromChildren(children)

	// Create a set for quick lookup of this screen's fields
	thisScreenFieldSet := make(map[string]bool)
	for _, f := range thisScreenFields {
		thisScreenFieldSet[f] = true
	}

	return sanitizeComponentsRecursive(children, allFieldNames, fieldsFromPreviousScreens, thisScreenFieldSet, thisScreenFields)
}

func sanitizeComponentsRecursive(children []any, allFieldNames []string, fieldsFromPreviousScreens []string, thisScreenFieldSet map[string]bool, thisScreenFields []string) []any {
	result := make([]any, len(children))

	for i, child := range children {
		compMap, ok := child.(map[string]any)
		if !ok {
			result[i] = child
			continue
		}

		// Create a new component map
		newComp := make(map[string]any)
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
		if dataSource, ok := newComp["data-source"].([]any); ok {
			newDataSource := make([]any, len(dataSource))
			for j, opt := range dataSource {
				if optMap, ok := opt.(map[string]any); ok {
					newOpt := make(map[string]any)
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
		if action, ok := newComp["on-click-action"].(map[string]any); ok {
			actionName, _ := action["name"].(string)

			newAction := make(map[string]any)
			for k, v := range action {
				newAction[k] = v
			}

			switch actionName {
			case "complete":
				payload := make(map[string]any)
				
				// Preserve existing payload
				if existingPayload, ok := newAction["payload"].(map[string]any); ok {
					for k, v := range existingPayload {
						payload[k] = v
					}
				}

				for _, fieldName := range fieldsFromPreviousScreens {
					if _, exists := payload[fieldName]; !exists {
						payload[fieldName] = "${data." + fieldName + "}"
					}
				}
				for _, fieldName := range thisScreenFields {
					if _, exists := payload[fieldName]; !exists {
						payload[fieldName] = "${form." + fieldName + "}"
					}
				}
				
				newAction["payload"] = payload
			case "navigate":
				if len(thisScreenFields) > 0 || len(fieldsFromPreviousScreens) > 0 {
					payload := make(map[string]any)
					
					// Preserve existing payload
					if existingPayload, ok := newAction["payload"].(map[string]any); ok {
						for k, v := range existingPayload {
							payload[k] = v
						}
					}
					
					for _, fieldName := range fieldsFromPreviousScreens {
						if _, exists := payload[fieldName]; !exists {
							payload[fieldName] = "${data." + fieldName + "}"
						}
					}
					for _, fieldName := range thisScreenFields {
						if _, exists := payload[fieldName]; !exists {
							payload[fieldName] = "${form." + fieldName + "}"
						}
					}
					
					newAction["payload"] = payload
				}
			}

			newComp["on-click-action"] = newAction
		}

		if compType == "If" {
			if thenBlock, ok := newComp["then"].([]any); ok {
				newComp["then"] = sanitizeComponentsRecursive(thenBlock, allFieldNames, fieldsFromPreviousScreens, thisScreenFieldSet, thisScreenFields)
			}
			if elseBlock, ok := newComp["else"].([]any); ok {
				newComp["else"] = sanitizeComponentsRecursive(elseBlock, allFieldNames, fieldsFromPreviousScreens, thisScreenFieldSet, thisScreenFields)
			}
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
		FlowJSON:        map[string]any(f.FlowJSON),
		Screens:         []any(f.Screens),
		PreviewURL:      f.PreviewURL,
		HasLocalChanges: f.HasLocalChanges,
		CreatedAt:       f.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:       f.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}
