package database_test

import (
	"testing"

	"github.com/shridarpatil/whatomate/internal/database"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zerodha/logf"
	"gorm.io/gorm"
)

// testLog é um logger silencioso (só Error+) para não poluir a saída dos
// testes com os Info/Warn que o backfill agora emite.
func testLog() logf.Logger {
	return logf.New(logf.Opts{Level: logf.ErrorLevel})
}

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

	require.NoError(t, database.BackfillOccurrencePermissions(db, testLog()))

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

	require.NoError(t, database.BackfillOccurrencePermissions(db, testLog()))

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

	require.NoError(t, database.BackfillOccurrencePermissions(db, testLog()))

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

	require.NoError(t, database.BackfillOccurrencePermissions(db, testLog()))

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

	require.NoError(t, database.BackfillOccurrencePermissions(db, testLog()))
	first := len(roleKeys(t, db, role.ID))

	require.NoError(t, database.BackfillOccurrencePermissions(db, testLog()))
	assert.Equal(t, first, len(roleKeys(t, db, role.ID)), "rodar duas vezes duplicou vinculos")
}

// A guarda por organizacao: uma org ja migrada e pulada inteira, mesmo que
// algum papel dela tenha sido criado depois sem as permissoes. Precisa de
// uma SEGUNDA organizacao ainda pendente: com uma unica org ja migrada,
// `pending` fica vazio e a funcao retorna antes do INSERT rodar, entao o
// teste passaria mesmo que a clausula de organizacao sumisse do INSERT (foi
// exatamente isso que a auditoria provou). Ver Fix: achados da revisao do
// backfill no relatorio para a prova da mutacao.
func TestBackfill_SkipsOrganisationAlreadyMigrated(t *testing.T) {
	db := testutil.SetupTestDB(t)
	cleanAll(t, db)
	require.NoError(t, database.SeedPermissionsAndRoles(db))

	migrada := testutil.CreateTestOrganization(t, db)
	jaMigrado := testutil.CreateTestRoleWithKeys(t, db, migrada.ID, "ja-migrado",
		[]string{"chat:read", "occurrences:read"})
	criadoDepois := testutil.CreateTestRoleWithKeys(t, db, migrada.ID, "criado-depois",
		[]string{"chat:read", "chat:write"})

	pendente := testutil.CreateTestOrganization(t, db)
	pendenteRole := testutil.CreateTestRoleWithKeys(t, db, pendente.ID, "pendente",
		[]string{"chat:read", "chat:write"})

	require.NoError(t, database.BackfillOccurrencePermissions(db, testLog()))

	// A org ja migrada e pulada inteira: o papel criado depois dela nao
	// ganha nada, mesmo tendo a capacidade equivalente.
	novoKeys := roleKeys(t, db, criadoDepois.ID)
	assert.NotContains(t, novoKeys, "occurrences:read",
		"organizacao ja migrada deveria ser pulada inteira")

	// O papel ja-migrado tambem nao ganha occurrences:write: ele so tinha
	// occurrences:read antes do backfill, e a org inteira foi pulada.
	jaMigradoKeys := roleKeys(t, db, jaMigrado.ID)
	assert.NotContains(t, jaMigradoKeys, "occurrences:write",
		"organizacao ja migrada nao deveria ganhar nada a mais")

	// A org pendente, por outro lado, e processada normalmente: prova que o
	// INSERT realmente rodou neste teste.
	pendenteKeys := roleKeys(t, db, pendenteRole.ID)
	assert.Contains(t, pendenteKeys, "occurrences:read",
		"organizacao pendente deveria ser migrada")
	assert.Contains(t, pendenteKeys, "occurrences:write",
		"organizacao pendente deveria ser migrada")
}

// O papel fantasma: um papel soft-deleted com occurrences:read não pode
// fazer a organização parecer já migrada. A guarda "organização ainda não
// migrada" tem `r2.deleted_at IS NULL` correlacionado dentro do NOT EXISTS
// exatamente para isso — sem essa cláusula, este teste falha (ver prova da
// mutação no relatório).
func TestBackfill_IgnoresSoftDeletedRoleWithOccurrencePermission(t *testing.T) {
	db := testutil.SetupTestDB(t)
	cleanAll(t, db)
	require.NoError(t, database.SeedPermissionsAndRoles(db))

	org := testutil.CreateTestOrganization(t, db)
	fantasma := testutil.CreateTestRoleWithKeys(t, db, org.ID, "fantasma",
		[]string{"occurrences:read"})
	require.NoError(t, db.Delete(fantasma).Error)

	vivo := testutil.CreateTestRoleWithKeys(t, db, org.ID, "vivo",
		[]string{"chat:read", "chat:write"})

	require.NoError(t, database.BackfillOccurrencePermissions(db, testLog()))

	keys := roleKeys(t, db, vivo.ID)
	assert.Contains(t, keys, "occurrences:read",
		"papel soft-deleted com occurrences:read nao deveria esconder a organizacao do backfill")
	assert.Contains(t, keys, "occurrences:write")
}

func TestBackfill_NoOpWhenPermissionsNotSeeded(t *testing.T) {
	db := testutil.SetupTestDB(t)
	cleanAll(t, db)
	// Sem SeedPermissionsAndRoles: as linhas de permissao nao existem.
	org := testutil.CreateTestOrganization(t, db)
	_ = org

	require.NoError(t, database.BackfillOccurrencePermissions(db, testLog()),
		"sem as permissoes semeadas o backfill deve nao fazer nada, sem erro")
}
