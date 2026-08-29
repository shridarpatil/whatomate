package handlers

import (
	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
)

// Test-only aliases. The production entry points stay unexported; these exist
// so package-external tests can drive them without widening the real API.

func (a *App) EnsureDefaultStagesForTest(orgID uuid.UUID) error {
	return a.ensureDefaultStages(orgID)
}

func (a *App) InitialStageForTest(orgID uuid.UUID) (*models.OccurrenceStage, error) {
	return a.initialStage(orgID)
}

func (a *App) CreateOccurrenceForTest(occ *models.Occurrence) error {
	return a.insertOccurrenceWithProtocol(occ)
}

func (a *App) NextProtocolNumberForTest(orgID uuid.UUID, year int) (string, error) {
	return a.nextProtocolNumber(a.DB, orgID, year)
}
