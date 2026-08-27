package handlers_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// GATE 1. Protocol numbering must be atomic. COUNT(*)+1 passes a serial test
// and collides in production exactly when volume rises, so the test that
// matters is the concurrent one.
func TestOccurrenceProtocol_UniqueUnderConcurrency(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	contact := testutil.CreateTestContact(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	require.NoError(t, app.EnsureDefaultStagesForTest(org.ID))
	stage, err := app.InitialStageForTest(org.ID)
	require.NoError(t, err)

	const n = 30
	var wg sync.WaitGroup
	errs := make([]error, n)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			occ := models.Occurrence{
				OrganizationID: org.ID,
				ContactID:      contact.ID,
				Title:          fmt.Sprintf("Caso %d", idx),
				StageID:        stage.ID,
				OpenedByUserID: user.ID,
			}
			errs[idx] = app.CreateOccurrenceForTest(&occ)
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		require.NoError(t, err, "criação %d falhou", i)
	}

	var protocols []string
	require.NoError(t, app.DB.Model(&models.Occurrence{}).
		Where("organization_id = ?", org.ID).
		Pluck("protocol_number", &protocols).Error)

	require.Len(t, protocols, n)
	seen := map[string]bool{}
	for _, p := range protocols {
		assert.False(t, seen[p], "protocolo duplicado: %s", p)
		seen[p] = true
	}
}

// GATE 2. ensureDefaultStages precisa de uma constraint real para o
// OnConflict conflitar. Sem ela, duas primeiras chamadas concorrentes para
// uma organização nova passam ambas pelo count==0 e ambas inserem, deixando
// duplicatas de is_initial/is_closing.
func TestOccurrenceProtocol_EnsureDefaultStages_ConcurrentSeedIsIdempotent(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)

	const n = 100
	var wg sync.WaitGroup
	start := make(chan struct{})
	errs := make([]error, n)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start
			errs[idx] = app.EnsureDefaultStagesForTest(org.ID)
		}(i)
	}
	close(start)
	wg.Wait()

	for i, err := range errs {
		require.NoError(t, err, "seed %d falhou", i)
	}

	var stages []models.OccurrenceStage
	require.NoError(t, app.DB.Where("organization_id = ?", org.ID).Find(&stages).Error)
	require.Len(t, stages, 4, "pipeline deveria ter exatamente 4 etapas")

	initial, closing := 0, 0
	for _, s := range stages {
		if s.IsInitial {
			initial++
		}
		if s.IsClosing {
			closing++
		}
	}
	assert.Equal(t, 1, initial, "deveria haver exatamente 1 etapa inicial")
	assert.Equal(t, 1, closing, "deveria haver exatamente 1 etapa de fechamento")
}

// A virada de ano começa em 000001, não continua a sequência do ano anterior.
func TestOccurrenceProtocol_ResetsOnNewYear(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)

	require.NoError(t, app.DB.Create(&models.OccurrenceCounter{
		OrganizationID: org.ID,
		Year:           time.Now().Year() - 1,
		LastSeq:        457,
	}).Error)

	got, err := app.NextProtocolNumberForTest(org.ID, time.Now().Year())
	require.NoError(t, err)
	assert.Equal(t, fmt.Sprintf("%d-000001", time.Now().Year()), got)
}

// O formato é YYYY-NNNNNN com zeros à esquerda, para ordenar lexicograficamente.
func TestOccurrenceProtocol_Format(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)

	require.NoError(t, app.DB.Create(&models.OccurrenceCounter{
		OrganizationID: org.ID,
		Year:           2026,
		LastSeq:        122,
	}).Error)

	got, err := app.NextProtocolNumberForTest(org.ID, 2026)
	require.NoError(t, err)
	assert.Equal(t, "2026-000123", got)
}
