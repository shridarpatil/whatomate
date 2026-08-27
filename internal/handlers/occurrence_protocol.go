package handlers

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// defaultStages is the pipeline every organisation starts with.
var defaultStages = []models.OccurrenceStage{
	{Name: "Aberto", Color: "#3b82f6", Position: 0, IsInitial: true},
	{Name: "Em análise", Color: "#f59e0b", Position: 1},
	{Name: "Aguardando cliente", Color: "#a855f7", Position: 2},
	{Name: "Resolvido", Color: "#10b981", Position: 3, IsClosing: true},
}

// ensureDefaultStages seeds the pipeline the first time an organisation touches
// the CRM. Lazy rather than a migration pass: it covers organisations created
// after the migration too, with no iteration over every tenant at boot.
func (a *App) ensureDefaultStages(orgID uuid.UUID) error {
	var count int64
	if err := a.DB.Model(&models.OccurrenceStage{}).
		Where("organization_id = ?", orgID).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	stages := make([]models.OccurrenceStage, len(defaultStages))
	for i, s := range defaultStages {
		s.OrganizationID = orgID
		stages[i] = s
	}
	// Another request may have seeded between the count and here; ignoring the
	// conflict is correct, the pipeline just already exists.
	return a.DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&stages).Error
}

// initialStage returns the organisation's entry stage, seeding the defaults if
// the pipeline has never been configured.
func (a *App) initialStage(orgID uuid.UUID) (*models.OccurrenceStage, error) {
	if err := a.ensureDefaultStages(orgID); err != nil {
		return nil, err
	}
	var stage models.OccurrenceStage
	if err := a.DB.Where("organization_id = ? AND is_initial = ?", orgID, true).
		Order("position ASC").First(&stage).Error; err != nil {
		return nil, err
	}
	return &stage, nil
}

// nextProtocolNumber returns the next protocol for the organisation and year.
//
// It MUST run inside the same transaction as the occurrence insert. The
// UPDATE ... RETURNING is atomic in Postgres: concurrent callers serialise on
// the counter row and each gets a distinct sequence. A COUNT(*)+1 would pass
// every serial test and hand out duplicates under load.
func (a *App) nextProtocolNumber(tx *gorm.DB, orgID uuid.UUID, year int) (string, error) {
	// Create the row for a fresh year; DoNothing when it already exists.
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).
		Create(&models.OccurrenceCounter{
			OrganizationID: orgID,
			Year:           year,
			LastSeq:        0,
		}).Error; err != nil {
		return "", err
	}

	var seq int
	row := tx.Raw(`UPDATE occurrence_counters
		SET last_seq = last_seq + 1
		WHERE organization_id = ? AND year = ?
		RETURNING last_seq`, orgID, year).Row()
	if err := row.Scan(&seq); err != nil {
		return "", err
	}

	return fmt.Sprintf("%d-%06d", year, seq), nil
}

// insertOccurrenceWithProtocol assigns the protocol and inserts the occurrence in one
// transaction, then records the opening event.
func (a *App) insertOccurrenceWithProtocol(occ *models.Occurrence) error {
	return a.DB.Transaction(func(tx *gorm.DB) error {
		protocol, err := a.nextProtocolNumber(tx, occ.OrganizationID, time.Now().Year())
		if err != nil {
			return err
		}
		occ.ProtocolNumber = protocol

		if err := tx.Create(occ).Error; err != nil {
			return err
		}

		return tx.Create(&models.OccurrenceEvent{
			OrganizationID: occ.OrganizationID,
			OccurrenceID:   occ.ID,
			Type:           models.OccurrenceEventOpened,
			Content:        occ.Title,
			CreatedByID:    &occ.OpenedByUserID,
		}).Error
	})
}
