package handlers

import (
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
	"gorm.io/gorm"
)

// OccurrenceStageRequest is the create/update body for a pipeline stage.
type OccurrenceStageRequest struct {
	Name      string `json:"name"`
	Color     string `json:"color"`
	Position  int    `json:"position"`
	IsInitial bool   `json:"is_initial"`
	IsClosing bool   `json:"is_closing"`
}

// ListOccurrenceStages returns the org's pipeline, seeding defaults on first use.
// Read needs only chat:read — every agent must see stage names to work.
func (a *App) ListOccurrenceStages(r *fastglue.Request) error {
	orgID, _, err := a.requireAuth(r, models.ResourceChat, models.ActionRead)
	if err != nil {
		return nil
	}

	if err := a.ensureDefaultStages(orgID); err != nil {
		a.Log.Error("Failed to seed occurrence stages", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError,
			"Failed to load stages", nil, "")
	}

	var stages []models.OccurrenceStage
	if err := a.DB.Where("organization_id = ?", orgID).
		Order("position ASC").Find(&stages).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError,
			"Failed to load stages", nil, "")
	}

	return r.SendEnvelope(map[string]any{"stages": stages})
}

// CreateOccurrenceStage adds a stage. Configuration lives under the existing
// settings.general permission — no new permission is introduced by the CRM.
func (a *App) CreateOccurrenceStage(r *fastglue.Request) error {
	orgID, _, err := a.requireAuth(r, models.ResourceSettingsGeneral, models.ActionWrite)
	if err != nil {
		return nil
	}

	var req OccurrenceStageRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}
	if req.Name == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "name is required", nil, "")
	}

	stage := models.OccurrenceStage{
		OrganizationID: orgID,
		Name:           req.Name,
		Color:          req.Color,
		Position:       req.Position,
		IsInitial:      req.IsInitial,
		IsClosing:      req.IsClosing,
	}

	err = a.DB.Transaction(func(tx *gorm.DB) error {
		if req.IsInitial {
			if err := unsetOtherInitial(tx, orgID, uuid.Nil); err != nil {
				return err
			}
		}
		return tx.Create(&stage).Error
	})
	if err != nil {
		if isUniqueNameViolation(err) {
			return r.SendErrorEnvelope(fasthttp.StatusConflict,
				"A stage with this name already exists", nil, "")
		}
		a.Log.Error("Failed to create occurrence stage", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError,
			"Failed to create stage", nil, "")
	}

	return r.SendEnvelope(stage)
}

// UpdateOccurrenceStage edits a stage, keeping exactly one initial stage and
// refusing to leave the org without a closing stage.
func (a *App) UpdateOccurrenceStage(r *fastglue.Request) error {
	orgID, _, err := a.requireAuth(r, models.ResourceSettingsGeneral, models.ActionWrite)
	if err != nil {
		return nil
	}

	stageID, err := parsePathUUID(r, "id", "stage")
	if err != nil {
		return nil
	}

	stage, err := findByIDAndOrg[models.OccurrenceStage](a.DB, r, stageID, orgID, "Stage")
	if err != nil {
		return nil
	}

	var req OccurrenceStageRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}
	if req.Name == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "name is required", nil, "")
	}

	// Clearing the last closing stage would make it impossible to close any
	// occurrence at all.
	if stage.IsClosing && !req.IsClosing {
		var closing int64
		a.DB.Model(&models.OccurrenceStage{}).
			Where("organization_id = ? AND is_closing = ? AND id <> ?", orgID, true, stageID).
			Count(&closing)
		if closing == 0 {
			return r.SendErrorEnvelope(fasthttp.StatusConflict,
				"At least one closing stage is required", nil, "")
		}
	}

	err = a.DB.Transaction(func(tx *gorm.DB) error {
		if req.IsInitial {
			if err := unsetOtherInitial(tx, orgID, stageID); err != nil {
				return err
			}
		}
		return tx.Model(stage).Updates(map[string]any{
			"name":       req.Name,
			"color":      req.Color,
			"position":   req.Position,
			"is_initial": req.IsInitial,
			"is_closing": req.IsClosing,
		}).Error
	})
	if err != nil {
		if isUniqueNameViolation(err) {
			return r.SendErrorEnvelope(fasthttp.StatusConflict,
				"A stage with this name already exists", nil, "")
		}
		a.Log.Error("Failed to update occurrence stage", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError,
			"Failed to update stage", nil, "")
	}

	return r.SendEnvelope(stage)
}

// DeleteOccurrenceStage removes a stage only when nothing depends on it.
func (a *App) DeleteOccurrenceStage(r *fastglue.Request) error {
	orgID, _, err := a.requireAuth(r, models.ResourceSettingsGeneral, models.ActionWrite)
	if err != nil {
		return nil
	}

	stageID, err := parsePathUUID(r, "id", "stage")
	if err != nil {
		return nil
	}

	stage, err := findByIDAndOrg[models.OccurrenceStage](a.DB, r, stageID, orgID, "Stage")
	if err != nil {
		return nil
	}

	var inUse int64
	a.DB.Model(&models.Occurrence{}).
		Where("organization_id = ? AND stage_id = ?", orgID, stageID).Count(&inUse)
	if inUse > 0 {
		return r.SendErrorEnvelope(fasthttp.StatusConflict,
			"Stage is in use by existing occurrences", nil, "")
	}

	if stage.IsInitial {
		return r.SendErrorEnvelope(fasthttp.StatusConflict,
			"Cannot delete the initial stage", nil, "")
	}

	if stage.IsClosing {
		var closing int64
		a.DB.Model(&models.OccurrenceStage{}).
			Where("organization_id = ? AND is_closing = ? AND id <> ?", orgID, true, stageID).
			Count(&closing)
		if closing == 0 {
			return r.SendErrorEnvelope(fasthttp.StatusConflict,
				"At least one closing stage is required", nil, "")
		}
	}

	if err := a.DB.Delete(stage).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError,
			"Failed to delete stage", nil, "")
	}

	return r.SendEnvelope(map[string]any{"deleted": true})
}

// unsetOtherInitial clears is_initial everywhere except keepID.
func unsetOtherInitial(tx *gorm.DB, orgID, keepID uuid.UUID) error {
	return tx.Model(&models.OccurrenceStage{}).
		Where("organization_id = ? AND id <> ?", orgID, keepID).
		Update("is_initial", false).Error
}

// isUniqueNameViolation reports whether err is a Postgres unique-constraint
// violation (SQLSTATE 23505) — specifically the (organization_id, name) index
// on occurrence_stages. Both create and rename can race a concurrent writer
// past an application-level pre-check, so the DB constraint is the only
// reliable signal; without translating it here, callers would leak a raw
// database error as a 500 instead of a clean 409.
func isUniqueNameViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
