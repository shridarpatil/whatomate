package handlers_test

import (
	"testing"

	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// GATE 7. Uma etapa em uso não pode ser excluída — senão a ocorrência aponta
// para uma linha que não existe mais.
func TestOccurrenceStages_DeleteInUseIsRejected(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	role := testutil.CreateTestRoleWithKeys(t, app.DB, org.ID, "stages-rw", []string{"settings.general:write", "chat:read"})
	contact := testutil.CreateTestContact(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

	require.NoError(t, app.EnsureDefaultStagesForTest(org.ID))
	stage, err := app.InitialStageForTest(org.ID)
	require.NoError(t, err)

	occ := models.Occurrence{
		OrganizationID: org.ID, ContactID: contact.ID, Title: "Em uso",
		StageID: stage.ID, OpenedByUserID: user.ID,
	}
	require.NoError(t, app.CreateOccurrenceForTest(&occ))

	req := testutil.NewRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", stage.ID.String())

	require.NoError(t, app.DeleteOccurrenceStage(req))
	assert.Equal(t, fasthttp.StatusConflict, testutil.GetResponseStatusCode(req))

	var count int64
	app.DB.Model(&models.OccurrenceStage{}).Where("id = ?", stage.ID).Count(&count)
	assert.EqualValues(t, 1, count, "a etapa não pode ter sido apagada")
}

// Sem etapa de fechamento não existe como concluir uma ocorrência.
func TestOccurrenceStages_DeleteLastClosingIsRejected(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	role := testutil.CreateTestRoleWithKeys(t, app.DB, org.ID, "stages-rw", []string{"settings.general:write", "chat:read"})
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

	require.NoError(t, app.EnsureDefaultStagesForTest(org.ID))

	var closing models.OccurrenceStage
	require.NoError(t, app.DB.Where("organization_id = ? AND is_closing = ?", org.ID, true).
		First(&closing).Error)

	req := testutil.NewRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", closing.ID.String())

	require.NoError(t, app.DeleteOccurrenceStage(req))
	assert.Equal(t, fasthttp.StatusConflict, testutil.GetResponseStatusCode(req))
}

// Excluir a etapa is_initial deixaria o pipeline sem porta de entrada.
func TestOccurrenceStages_DeleteInitialIsRejected(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	role := testutil.CreateTestRoleWithKeys(t, app.DB, org.ID, "stages-rw", []string{"settings.general:write", "chat:read"})
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

	require.NoError(t, app.EnsureDefaultStagesForTest(org.ID))
	stage, err := app.InitialStageForTest(org.ID)
	require.NoError(t, err)

	req := testutil.NewRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", stage.ID.String())

	require.NoError(t, app.DeleteOccurrenceStage(req))
	assert.Equal(t, fasthttp.StatusConflict, testutil.GetResponseStatusCode(req))

	var count int64
	app.DB.Model(&models.OccurrenceStage{}).Where("id = ?", stage.ID).Count(&count)
	assert.EqualValues(t, 1, count, "a etapa inicial não pode ter sido apagada")
}

// Exatamente uma etapa inicial por organização.
func TestOccurrenceStages_SettingInitialUnsetsPrevious(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	role := testutil.CreateTestRoleWithKeys(t, app.DB, org.ID, "stages-rw", []string{"settings.general:write", "chat:read"})
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

	require.NoError(t, app.EnsureDefaultStagesForTest(org.ID))

	var other models.OccurrenceStage
	require.NoError(t, app.DB.Where("organization_id = ? AND is_initial = ?", org.ID, false).
		Order("position ASC").First(&other).Error)

	req := testutil.NewJSONRequest(t, map[string]any{
		"name": other.Name, "color": other.Color, "position": other.Position,
		"is_initial": true, "is_closing": other.IsClosing,
	})
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", other.ID.String())

	require.NoError(t, app.UpdateOccurrenceStage(req))
	assert.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var initialCount int64
	app.DB.Model(&models.OccurrenceStage{}).
		Where("organization_id = ? AND is_initial = ?", org.ID, true).Count(&initialCount)
	assert.EqualValues(t, 1, initialCount, "deve haver exatamente uma etapa inicial")

	var reloaded models.OccurrenceStage
	require.NoError(t, app.DB.First(&reloaded, "id = ?", other.ID).Error)
	assert.True(t, reloaded.IsInitial)
}

// Desmarcar is_closing da última etapa de fechamento deixaria o pipeline sem
// como concluir uma ocorrência.
func TestOccurrenceStages_UnsetLastClosingIsRejected(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	role := testutil.CreateTestRoleWithKeys(t, app.DB, org.ID, "stages-rw", []string{"settings.general:write", "chat:read"})
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

	require.NoError(t, app.EnsureDefaultStagesForTest(org.ID))

	var closing models.OccurrenceStage
	require.NoError(t, app.DB.Where("organization_id = ? AND is_closing = ?", org.ID, true).
		First(&closing).Error)

	req := testutil.NewJSONRequest(t, map[string]any{
		"name": closing.Name, "color": closing.Color, "position": closing.Position,
		"is_initial": closing.IsInitial, "is_closing": false,
	})
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", closing.ID.String())

	require.NoError(t, app.UpdateOccurrenceStage(req))
	assert.Equal(t, fasthttp.StatusConflict, testutil.GetResponseStatusCode(req))

	var reloaded models.OccurrenceStage
	require.NoError(t, app.DB.First(&reloaded, "id = ?", closing.ID).Error)
	assert.True(t, reloaded.IsClosing, "a etapa deve continuar de fechamento")
}

// Criar uma etapa com nome já usado na organização viola o índice único
// (organization_id, name) — o handler precisa devolver 409, não um 500 cru.
func TestOccurrenceStages_CreateDuplicateNameIsRejected(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	role := testutil.CreateTestRoleWithKeys(t, app.DB, org.ID, "stages-rw", []string{"settings.general:write", "chat:read"})
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

	require.NoError(t, app.EnsureDefaultStagesForTest(org.ID))

	req := testutil.NewJSONRequest(t, map[string]any{
		"name": "Aberto", "color": "#000000", "position": 9,
	})
	testutil.SetAuthContext(req, org.ID, user.ID)

	require.NoError(t, app.CreateOccurrenceStage(req))
	assert.Equal(t, fasthttp.StatusConflict, testutil.GetResponseStatusCode(req))
}

// Renomear uma etapa para um nome já usado na mesma organização tem o mesmo
// problema de constraint — mesmo tratamento 409.
func TestOccurrenceStages_RenameToDuplicateNameIsRejected(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	role := testutil.CreateTestRoleWithKeys(t, app.DB, org.ID, "stages-rw", []string{"settings.general:write", "chat:read"})
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

	require.NoError(t, app.EnsureDefaultStagesForTest(org.ID))

	var other models.OccurrenceStage
	require.NoError(t, app.DB.Where("organization_id = ? AND is_initial = ?", org.ID, false).
		Order("position ASC").First(&other).Error)

	req := testutil.NewJSONRequest(t, map[string]any{
		"name": "Aberto", "color": other.Color, "position": other.Position,
		"is_initial": other.IsInitial, "is_closing": other.IsClosing,
	})
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetPathParam(req, "id", other.ID.String())

	require.NoError(t, app.UpdateOccurrenceStage(req))
	assert.Equal(t, fasthttp.StatusConflict, testutil.GetResponseStatusCode(req))

	var reloaded models.OccurrenceStage
	require.NoError(t, app.DB.First(&reloaded, "id = ?", other.ID).Error)
	assert.NotEqual(t, "Aberto", reloaded.Name, "o rename não pode ter sido aplicado")
}
