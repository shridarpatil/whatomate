package handlers

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)


// GetFlowSubmissions returns all submissions for a specific WhatsApp flow.
func (a *App) GetFlowSubmissions(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	flowID, err := parsePathUUID(r, "id", "flow")
	if err != nil {
		return nil
	}

	// Verify the flow belongs to this org
	var flow models.WhatsAppFlow
	if err := a.DB.Where("id = ? AND organization_id = ?", flowID, orgID).First(&flow).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Flow not found", nil, "")
	}

	// Fetch submissions
	type submissionRow struct {
		ID           uuid.UUID  `gorm:"column:id" json:"id"`
		PhoneNumber  string     `gorm:"column:phone_number" json:"phone_number"`
		ResponseData string     `gorm:"column:response_data" json:"-"`
		CreatedAt    time.Time  `gorm:"column:created_at" json:"created_at"`
	}

	var rows []submissionRow
	if err := a.DB.Raw(`
		SELECT id, phone_number, response_data, created_at
		FROM whatsapp_flow_submissions
		WHERE flow_id = ? AND organization_id = ?
		ORDER BY created_at DESC
	`, flowID, orgID).Scan(&rows).Error; err != nil {
		a.Log.Error("Failed to fetch flow submissions", "error", err, "flow_id", flowID)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to fetch submissions", nil, "")
	}

	submissions := make([]map[string]any, len(rows))
	for i, row := range rows {
		// Parse response_data from JSON string
		var respData models.JSONB
		if row.ResponseData != "" {
			if err := json.Unmarshal([]byte(row.ResponseData), &respData); err != nil {
				respData = models.JSONB{}
			}
		}
		if respData == nil {
			respData = models.JSONB{}
		}

		submissions[i] = map[string]any{
			"id":            row.ID,
			"phone_number":  row.PhoneNumber,
			"response_data": respData,
			"created_at":    row.CreatedAt,
		}
	}

	return r.SendEnvelope(map[string]any{
		"flow": map[string]any{
			"id":     flow.ID,
			"name":   flow.Name,
			"status": flow.Status,
		},
		"submissions": submissions,
		"total":       len(submissions),
	})
}
