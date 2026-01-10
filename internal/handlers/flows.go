package handlers

import (
	"context"
	"encoding/json"

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
	CreatedAt       string                 `json:"created_at"`
	UpdatedAt       string                 `json:"updated_at"`
}

// ListFlows returns all flows for the organization
func (a *App) ListFlows(r *fastglue.Request) error {
	orgID, err := getOrganizationID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	// Optional filters
	accountName := string(r.RequestCtx.QueryArgs().Peek("account"))
	status := string(r.RequestCtx.QueryArgs().Peek("status"))

	query := a.DB.Where("organization_id = ?", orgID)

	if accountName != "" {
		query = query.Where("whats_app_account = ?", accountName)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}

	var flows []models.WhatsAppFlow
	if err := query.Order("created_at DESC").Find(&flows).Error; err != nil {
		a.Log.Error("Failed to list flows", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to list flows", nil, "")
	}

	response := make([]FlowResponse, len(flows))
	for i, f := range flows {
		response[i] = flowToResponse(f)
	}

	return r.SendEnvelope(map[string]interface{}{
		"flows": response,
	})
}

// CreateFlow creates a new WhatsApp flow
func (a *App) CreateFlow(r *fastglue.Request) error {
	orgID, err := getOrganizationID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	var req FlowRequest
	if err := r.Decode(&req, "json"); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid request body", nil, "")
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
	orgID, err := getOrganizationID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	idStr := r.RequestCtx.UserValue("id").(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid flow ID", nil, "")
	}

	var flow models.WhatsAppFlow
	if err := a.DB.Where("id = ? AND organization_id = ?", id, orgID).First(&flow).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Flow not found", nil, "")
	}

	return r.SendEnvelope(map[string]interface{}{
		"flow": flowToResponse(flow),
	})
}

// UpdateFlow updates an existing flow
func (a *App) UpdateFlow(r *fastglue.Request) error {
	orgID, err := getOrganizationID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	idStr := r.RequestCtx.UserValue("id").(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid flow ID", nil, "")
	}

	var flow models.WhatsAppFlow
	if err := a.DB.Where("id = ? AND organization_id = ?", id, orgID).First(&flow).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Flow not found", nil, "")
	}

	// Only DRAFT flows can be updated
	if flow.Status != "DRAFT" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Only DRAFT flows can be updated", nil, "")
	}

	var req FlowRequest
	if err := r.Decode(&req, "json"); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid request body", nil, "")
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
		if err := a.DB.Model(&flow).Updates(updates).Error; err != nil {
			a.Log.Error("Failed to update flow", "error", err, "flow_id", id)
			return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to update flow", nil, "")
		}
	}

	// Reload flow
	a.DB.First(&flow, id)

	a.Log.Info("Flow updated", "flow_id", flow.ID)

	return r.SendEnvelope(map[string]interface{}{
		"flow": flowToResponse(flow),
	})
}

// DeleteFlow deletes a flow
func (a *App) DeleteFlow(r *fastglue.Request) error {
	orgID, err := getOrganizationID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	idStr := r.RequestCtx.UserValue("id").(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid flow ID", nil, "")
	}

	var flow models.WhatsAppFlow
	if err := a.DB.Where("id = ? AND organization_id = ?", id, orgID).First(&flow).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Flow not found", nil, "")
	}

	// Delete the flow (soft delete)
	if err := a.DB.Delete(&flow).Error; err != nil {
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
	orgID, err := getOrganizationID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	idStr := r.RequestCtx.UserValue("id").(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid flow ID", nil, "")
	}

	var flow models.WhatsAppFlow
	if err := a.DB.Where("id = ? AND organization_id = ?", id, orgID).First(&flow).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Flow not found", nil, "")
	}

	// Only DRAFT flows can be saved to Meta
	if flow.Status != "DRAFT" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Only DRAFT flows can be saved to Meta", nil, "")
	}

	// Get the WhatsApp account
	var account models.WhatsAppAccount
	if err := a.DB.Where("organization_id = ? AND name = ?", orgID, flow.WhatsAppAccount).First(&account).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "WhatsApp account not found", nil, "")
	}

	// Create WhatsApp API client
	waClient := whatsapp.New(a.Log)
	waAccount := &whatsapp.Account{
		PhoneID:     account.PhoneID,
		BusinessID:  account.BusinessID,
		APIVersion:  account.APIVersion,
		AccessToken: account.AccessToken,
	}

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
			return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to create flow in Meta: "+err.Error(), nil, "")
		}
	} else {
		metaFlowID = flow.MetaFlowID
	}

	// Step 2: Upload flow JSON if we have screens
	if len(flow.Screens) > 0 {
		flowJSON := &whatsapp.FlowJSON{
			Version: flow.JSONVersion,
			Screens: flow.Screens,
		}

		if err := waClient.UpdateFlowJSON(ctx, waAccount, metaFlowID, flowJSON); err != nil {
			a.Log.Error("Failed to update flow JSON in Meta", "error", err, "flow_id", id, "meta_flow_id", metaFlowID)
			// Save the meta flow ID even if JSON update fails
			a.DB.Model(&flow).Updates(map[string]interface{}{
				"meta_flow_id": metaFlowID,
			})
			return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to update flow JSON: "+err.Error(), nil, "")
		}
	}

	// Update local database with meta flow ID
	if err := a.DB.Model(&flow).Updates(map[string]interface{}{
		"meta_flow_id": metaFlowID,
	}).Error; err != nil {
		a.Log.Error("Failed to update flow", "error", err, "flow_id", id)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to update flow", nil, "")
	}

	// Reload flow
	a.DB.First(&flow, id)

	a.Log.Info("Flow saved to Meta", "flow_id", flow.ID, "meta_flow_id", metaFlowID)

	return r.SendEnvelope(map[string]interface{}{
		"flow":    flowToResponse(flow),
		"message": "Flow saved to Meta successfully",
	})
}

// PublishFlow publishes a flow to Meta
func (a *App) PublishFlow(r *fastglue.Request) error {
	orgID, err := getOrganizationID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	idStr := r.RequestCtx.UserValue("id").(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid flow ID", nil, "")
	}

	var flow models.WhatsAppFlow
	if err := a.DB.Where("id = ? AND organization_id = ?", id, orgID).First(&flow).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Flow not found", nil, "")
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
	waAccount := &whatsapp.Account{
		PhoneID:     account.PhoneID,
		BusinessID:  account.BusinessID,
		APIVersion:  account.APIVersion,
		AccessToken: account.AccessToken,
	}

	ctx := context.Background()

	// Publish the flow
	if err := waClient.PublishFlow(ctx, waAccount, flow.MetaFlowID); err != nil {
		a.Log.Error("Failed to publish flow in Meta", "error", err, "flow_id", id, "meta_flow_id", flow.MetaFlowID)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to publish flow: "+err.Error(), nil, "")
	}

	// Get the flow details including preview URL
	metaFlow, err := waClient.GetFlow(ctx, waAccount, flow.MetaFlowID)
	previewURL := ""
	if err == nil && metaFlow != nil {
		previewURL = metaFlow.PreviewURL
	}

	// Update local database
	if err := a.DB.Model(&flow).Updates(map[string]interface{}{
		"status":      "PUBLISHED",
		"preview_url": previewURL,
	}).Error; err != nil {
		a.Log.Error("Failed to update flow status", "error", err, "flow_id", id)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to update flow status", nil, "")
	}

	// Reload flow
	a.DB.First(&flow, id)

	a.Log.Info("Flow published to Meta", "flow_id", flow.ID, "meta_flow_id", flow.MetaFlowID)

	return r.SendEnvelope(map[string]interface{}{
		"flow":    flowToResponse(flow),
		"message": "Flow published successfully",
	})
}

// DeprecateFlow deprecates a published flow
func (a *App) DeprecateFlow(r *fastglue.Request) error {
	orgID, err := getOrganizationID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	idStr := r.RequestCtx.UserValue("id").(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid flow ID", nil, "")
	}

	var flow models.WhatsAppFlow
	if err := a.DB.Where("id = ? AND organization_id = ?", id, orgID).First(&flow).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Flow not found", nil, "")
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
		waAccount := &whatsapp.Account{
			PhoneID:     account.PhoneID,
			BusinessID:  account.BusinessID,
			APIVersion:  account.APIVersion,
			AccessToken: account.AccessToken,
		}

		ctx := context.Background()
		if err := waClient.DeprecateFlow(ctx, waAccount, flow.MetaFlowID); err != nil {
			a.Log.Error("Failed to deprecate flow in Meta", "error", err, "flow_id", id, "meta_flow_id", flow.MetaFlowID)
			return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to deprecate flow in Meta: "+err.Error(), nil, "")
		}
	}

	if err := a.DB.Model(&flow).Updates(map[string]interface{}{
		"status": "DEPRECATED",
	}).Error; err != nil {
		a.Log.Error("Failed to deprecate flow", "error", err, "flow_id", id)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to deprecate flow", nil, "")
	}

	// Reload flow
	a.DB.First(&flow, id)

	a.Log.Info("Flow deprecated", "flow_id", flow.ID)

	return r.SendEnvelope(map[string]interface{}{
		"flow":    flowToResponse(flow),
		"message": "Flow deprecated successfully",
	})
}

// SyncFlows syncs flows from Meta for a specific WhatsApp account
func (a *App) SyncFlows(r *fastglue.Request) error {
	orgID, err := getOrganizationID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	// Get account name from request
	var req struct {
		WhatsAppAccount string `json:"whatsapp_account"`
	}
	if err := r.Decode(&req, "json"); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid request body", nil, "")
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
	waAccount := &whatsapp.Account{
		PhoneID:     account.PhoneID,
		BusinessID:  account.BusinessID,
		APIVersion:  account.APIVersion,
		AccessToken: account.AccessToken,
	}

	ctx := context.Background()

	// Fetch flows from Meta
	metaFlows, err := waClient.ListFlows(ctx, waAccount)
	if err != nil {
		a.Log.Error("Failed to fetch flows from Meta", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to fetch flows from Meta: "+err.Error(), nil, "")
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
		CreatedAt:       f.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:       f.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}
