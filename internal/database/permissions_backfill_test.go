package database_test

import (
	"testing"

	"github.com/shridarpatil/whatomate/internal/database"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// roleKeys devolve as chaves "recurso:acao" ligadas a um papel.
func roleKeys(t *testing.T, db *gorm.DB, roleID any) []string {
	t.Helper()
	var keys []string
	require.NoError(t, db.Raw(`
		SELECT p.resource || ':' || p.action
		FROM role_permissions rp
		JOIN permissions p ON p.id = rp.permission_id
		WHERE rp.custom_role_id = ?`, roleID).Scan(&keys).Error)
	return keys
}

func TestBackfill_GrantsOccurrencesFromChat(t *testing.T) {
	db := testutil.SetupTestDB(t)
	cleanAll(t, db)
	require.NoError(t, database.SeedPermissionsAndRoles(db))

	org := testutil.CreateTestOrganization(t, db)
	role := testutil.CreateTestRoleWithKeys(t, db, org.ID, "atendente",
		[]string{"chat:read", "chat:write"})

	require.NoError(t, database.BackfillOccurrencePermissions(db))

	keys := roleKeys(t, db, role.ID)
	assert.Contains(t, keys, "occurrences:read")
	assert.Contains(t, keys, "occurrences:write")
	assert.NotContains(t, keys, "occurrences.stages:read")
	assert.NotContains(t, keys, "occurrences.stages:write")
	assert.NotContains(t, keys, "occurrences.stages:delete")
}

func TestBackfill_GrantsStagesFromSettingsGeneral(t *testing.T) {
	db := testutil.SetupTestDB(t)
	cleanAll(t, db)
	require.NoError(t, database.SeedPermissionsAndRoles(db))

	org := testutil.CreateTestOrganization(t, db)
	role := testutil.CreateTestRoleWithKeys(t, db, org.ID, "gestor",
		[]string{"chat:read", "chat:write", "settings.general:write"})

	require.NoError(t, database.BackfillOccurrencePermissions(db))

	keys := roleKeys(t, db, role.ID)
	// As tres: sem o read a tela de configuracao fica inalcancavel mesmo
	// para quem pode editar, porque a guarda de rota checa a acao read.
	assert.Contains(t, keys, "occurrences.stages:read")
	assert.Contains(t, keys, "occurrences.stages:write")
	assert.Contains(t, keys, "occurrences.stages:delete")
}

// roles:write NAO concede administracao de funil: a regra e equivalencia
// com a capacidade atual, nao generosidade.
func TestBackfill_RolesWriteDoesNotGrantStages(t *testing.T) {
	db := testutil.SetupTestDB(t)
	cleanAll(t, db)
	require.NoError(t, database.SeedPermissionsAndRoles(db))

	org := testutil.CreateTestOrganization(t, db)
	role := testutil.CreateTestRoleWithKeys(t, db, org.ID, "admin-de-papeis",
		[]string{"roles:read", "roles:write"})

	require.NoError(t, database.BackfillOccurrencePermissions(db))

	keys := roleKeys(t, db, role.ID)
	assert.NotContains(t, keys, "occurrences.stages:read")
	assert.NotContains(t, keys, "occurrences.stages:write")
	assert.NotContains(t, keys, "occurrences.stages:delete")
	assert.NotContains(t, keys, "occurrences:read")
}

// A propriedade que torna o rollback seguro.
func TestBackfill_NeverRemovesChatPermissions(t *testing.T) {
	db := testutil.SetupTestDB(t)
	cleanAll(t, db)
	require.NoError(t, database.SeedPermissionsAndRoles(db))

	org := testutil.CreateTestOrganization(t, db)
	role := testutil.CreateTestRoleWithKeys(t, db, org.ID, "atendente",
		[]string{"chat:read", "chat:write"})

	require.NoError(t, database.BackfillOccurrencePermissions(db))

	keys := roleKeys(t, db, role.ID)
	assert.Contains(t, keys, "chat:read")
	assert.Contains(t, keys, "chat:write")
}

func TestBackfill_IsIdempotent(t *testing.T) {
	db := testutil.SetupTestDB(t)
	cleanAll(t, db)
	require.NoError(t, database.SeedPermissionsAndRoles(db))

	org := testutil.CreateTestOrganization(t, db)
	role := testutil.CreateTestRoleWithKeys(t, db, org.ID, "atendente",
		[]string{"chat:read", "chat:write"})

	require.NoError(t, database.BackfillOccurrencePermissions(db))
	first := len(roleKeys(t, db, role.ID))

	require.NoError(t, database.BackfillOccurrencePermissions(db))
	assert.Equal(t, first, len(roleKeys(t, db, role.ID)), "rodar duas vezes duplicou vinculos")
}

// A guarda por organizacao: uma org ja migrada e pulada inteira, mesmo que
// algum papel dela tenha sido criado depois sem as permissoes.
func TestBackfill_SkipsOrganisationAlreadyMigrated(t *testing.T) {
	db := testutil.SetupTestDB(t)
	cleanAll(t, db)
	require.NoError(t, database.SeedPermissionsAndRoles(db))

	org := testutil.CreateTestOrganization(t, db)
	testutil.CreateTestRoleWithKeys(t, db, org.ID, "ja-migrado",
		[]string{"chat:read", "occurrences:read"})
	novo := testutil.CreateTestRoleWithKeys(t, db, org.ID, "criado-depois",
		[]string{"chat:read", "chat:write"})

	require.NoError(t, database.BackfillOccurrencePermissions(db))

	keys := roleKeys(t, db, novo.ID)
	assert.NotContains(t, keys, "occurrences:read",
		"organizacao ja migrada deveria ser pulada inteira")
}

func TestBackfill_NoOpWhenPermissionsNotSeeded(t *testing.T) {
	db := testutil.SetupTestDB(t)
	cleanAll(t, db)
	// Sem SeedPermissionsAndRoles: as linhas de permissao nao existem.
	org := testutil.CreateTestOrganization(t, db)
	_ = org

	require.NoError(t, database.BackfillOccurrencePermissions(db),
		"sem as permissoes semeadas o backfill deve nao fazer nada, sem erro")
}
