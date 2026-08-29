# Permissão dedicada do CRM — plano de implementação

> **Para agentes executores:** SUB-SKILL OBRIGATÓRIA: use `superpowers:subagent-driven-development` (recomendado) ou `superpowers:executing-plans` para implementar tarefa a tarefa. Os passos usam caixas (`- [ ]`) para acompanhamento.

**Objetivo:** dar ao módulo CRM permissões próprias — `occurrences` e `occurrences.stages` — sem que nenhum usuário legítimo perca acesso no deploy.

**Arquitetura:** dois recursos novos no modelo de permissões, a troca da constante em 13 chamadas de `requireAuth`, e um backfill puramente aditivo que concede as permissões novas a quem já tem a capacidade equivalente por `chat:*` e `settings.general:write`. O backfill roda no bloco de migração do boot, antes do `ListenAndServe`, e é idempotente por guarda de organização.

**Stack:** Go 1.25 + fastglue + GORM + Postgres; Vue 3 + TypeScript + Pinia.

**Spec:** `docs/superpowers/specs/2026-08-29-permissao-crm-design.md` (aprovada; §1 e §4 corrigidas em `b19fd09`).

## Restrições globais

Valem para **todas** as tarefas.

- **Nada de mudança de visibilidade.** `visibleOccurrences`, `resolveAssignee` e `loadAuthorizedOccurrence` não são tocados.
- **Nenhuma permissão `chat:*` é removida de papel algum, em lugar algum.** É o que torna o rollback seguro: a versão anterior autoriza por `chat:*`.
- **O backfill só concede, nunca revoga.**
- Sem `view_all`/`view_team` para ocorrências; sem permissão de exclusão de ocorrência (não existe endpoint).
- Nenhuma alteração na lógica interna dos handlers — só a constante passada a `requireAuth`.
- Os testes de banco em Go **pulam em silêncio** sem `TEST_DATABASE_URL` **e** `TEST_REDIS_URL`, e a suíte reporta `ok` sem ter rodado nada. Sempre passe as duas.
- **Não rode `npm run lint`** — tem `--fix` e reescreve o repositório. Use `npx eslint <arquivos>`.
- Único erro aceitável no `npm run typecheck`: o pré-existente `AccountDetailView.vue(172,45) business_calling_enabled`.

## Estrutura de arquivos

| Arquivo | Responsabilidade | Tarefa |
|---|---|---|
| `internal/models/roles.go` | modificar: constantes, `DefaultPermissions`, `SystemRolePermissions` | T1 |
| `internal/handlers/occurrences.go` | modificar: 8 chamadas de `requireAuth` | T1 |
| `internal/handlers/occurrence_send.go` | modificar: 1 chamada | T1 |
| `internal/handlers/occurrence_stages.go` | modificar: 4 chamadas | T1 |
| `internal/handlers/occurrence_permissions_test.go` | criar: testes de autorização | T1 |
| `internal/database/permissions_backfill.go` | criar: `BackfillOccurrencePermissions` | T2 |
| `internal/database/permissions_backfill_test.go` | criar: testes do backfill | T2 |
| `cmd/whatomate/main.go` | modificar: chamada no bloco de migração | T2 |
| `frontend/src/router/index.ts` | modificar: 3 rotas, em 2 listas | T3 |
| `frontend/src/components/layout/navigation.ts` | modificar: entrada `nav.crm` | T3 |
| `frontend/src/lib/constants.ts` | modificar: 2 rótulos | T3 |
| `frontend/e2e/tests/crm/occurrence-permissions.spec.ts` | criar: bloqueio de rota por permissão | T3 |

---

## Task 1: Os recursos e o mapeamento dos handlers

**Files:**
- Modify: `internal/models/roles.go`
- Modify: `internal/handlers/occurrences.go` (linhas 107, 190, 255, 367, 381, 451, 527, 563)
- Modify: `internal/handlers/occurrence_send.go` (linha 31)
- Modify: `internal/handlers/occurrence_stages.go` (linhas 26, 50, 103, 185)
- Test: `internal/handlers/occurrence_permissions_test.go` (criar)

**Interfaces:**
- Consumes: nada.
- Produces: as constantes `models.ResourceOccurrences = "occurrences"` e `models.ResourceOccurrenceStages = "occurrences.stages"`, e as quatro permissões `occurrences:read`, `occurrences:write`, `occurrences.stages:write`, `occurrences.stages:delete` em `DefaultPermissions()`. A T2 depende dessas quatro chaves exatas; a T3 depende das duas strings de recurso.

### Contexto que você precisa

Hoje **todos** os handlers de ocorrência autorizam por `chat`, exceto as três mutações de etapa, que já usam `settings.general:write` — decisão explícita da Fase 1, com comentário no código. Não há falha de segurança sendo corrigida aqui; o que se ganha é granularidade.

`SystemRolePermissions()` monta a lista do admin **derivando de `DefaultPermissions()`** (`roles.go:251-254`), então o admin recebe as permissões novas automaticamente em organizações novas. `manager` e `agent` são listas literais e precisam das entradas escritas à mão.

Estas são as capacidades atuais que importam:
- `manager` tem `chat:read`, `chat:write` e `settings.general:write`.
- `agent` tem `chat:read` e `chat:write`, e **não** tem `settings.general:write`.

- [ ] **Step 1: Escreva os testes que falham**

Crie `internal/handlers/occurrence_permissions_test.go`:

```go
package handlers_test

import (
	"testing"

	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func TestOccurrencePermissions_ListRequiresOccurrencesRead(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	role := testutil.CreateTestRoleWithKeys(t, app.DB, org.ID, "sem-crm",
		[]string{"chat:read", "chat:write"})
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	require.NoError(t, app.ListOccurrences(req))
	require.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(req))
}

func TestOccurrencePermissions_ReadOnlyCannotCreate(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	contact := testutil.CreateTestContact(t, app.DB, org.ID)
	role := testutil.CreateTestRoleWithKeys(t, app.DB, org.ID, "leitura",
		[]string{"occurrences:read"})
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

	req := testutil.NewJSONRequest(t, map[string]any{
		"contact_id": contact.ID.String(),
		"title":      "Nao deve criar",
	})
	testutil.SetAuthContext(req, org.ID, user.ID)
	require.NoError(t, app.CreateOccurrence(req))
	require.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(req))
}

// Listar etapas é parte de USAR o CRM: o quadro não renderiza sem elas.
// Não pode exigir permissão administrativa.
func TestOccurrencePermissions_ListStagesNeedsOnlyOccurrencesRead(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	require.NoError(t, app.EnsureDefaultStagesForTest(org.ID))
	role := testutil.CreateTestRoleWithKeys(t, app.DB, org.ID, "atendente",
		[]string{"occurrences:read", "occurrences:write"})
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	require.NoError(t, app.ListOccurrenceStages(req))
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))
}

// O caso central da spec: quem usa o CRM não administra o funil.
// É caracterização, não regressão — já passa hoje, porque as mutações de
// etapa exigem settings.general:write. Existe para travar a separação
// depois da troca, e fica vermelho se alguém ligar administração de funil
// a occurrences:write.
func TestOccurrencePermissions_CRMUserCannotAdministerStages(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	require.NoError(t, app.EnsureDefaultStagesForTest(org.ID))
	role := testutil.CreateTestRoleWithKeys(t, app.DB, org.ID, "atendente",
		[]string{"occurrences:read", "occurrences:write"})
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

	req := testutil.NewJSONRequest(t, map[string]any{
		"name":  "Etapa proibida",
		"color": "#000000",
	})
	testutil.SetAuthContext(req, org.ID, user.ID)
	require.NoError(t, app.CreateOccurrenceStage(req))
	require.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(req))
}

func TestOccurrencePermissions_StageAdminCanCreateStage(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	require.NoError(t, app.EnsureDefaultStagesForTest(org.ID))
	role := testutil.CreateTestRoleWithKeys(t, app.DB, org.ID, "gestor-funil",
		[]string{"occurrences:read", "occurrences.stages:write"})
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

	req := testutil.NewJSONRequest(t, map[string]any{
		"name":  "Etapa permitida",
		"color": "#123456",
	})
	testutil.SetAuthContext(req, org.ID, user.ID)
	require.NoError(t, app.CreateOccurrenceStage(req))
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))
}

// Apagar exige a ação de exclusão, não a de escrita.
func TestOccurrencePermissions_StageWriteCannotDelete(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	require.NoError(t, app.EnsureDefaultStagesForTest(org.ID))

	var stage models.OccurrenceStage
	require.NoError(t, app.DB.Where("organization_id = ? AND is_closing = ?", org.ID, false).
		Where("is_initial = ?", false).First(&stage).Error)

	role := testutil.CreateTestRoleWithKeys(t, app.DB, org.ID, "so-escrita",
		[]string{"occurrences:read", "occurrences.stages:write"})
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

	req := testutil.NewRequest(t)
	testutil.SetPathParam(req, "id", stage.ID.String())
	testutil.SetAuthContext(req, org.ID, user.ID)
	require.NoError(t, app.DeleteOccurrenceStage(req))
	require.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(req))
}

func TestOccurrencePermissions_SendProtocolRequiresOccurrencesWrite(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	role := testutil.CreateTestRoleWithKeys(t, app.DB, org.ID, "leitura",
		[]string{"occurrences:read"})
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))

	req := testutil.NewJSONRequest(t, map[string]any{})
	testutil.SetAuthContext(req, org.ID, user.ID)
	require.NoError(t, app.SendOccurrenceProtocol(req))
	require.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(req))
}
```

Os helpers usados acima são os reais de `test/testutil/http.go`: `NewGETRequest(t)` (linha 31), `NewJSONRequest(t, body)` para requisição com corpo (linha 14), `NewRequest(t)` para as sem corpo, como o DELETE (linha 41), e `SetPathParam(req, key, value)` (linha 64). **Não existe `NewPOSTRequest` nem `NewDELETERequest`** — não invente esses nomes.

`CreateTestRoleWithKeys` (`test/testutil/fixtures.go:337`) **ignora silenciosamente chave desconhecida**. É por isso que o Step 2 falha da forma descrita: antes de as permissões existirem, os papéis saem vazios.

- [ ] **Step 2: Rode e confirme que falham**

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc" && TEST_DATABASE_URL='postgres://postgres:postgres@127.0.0.1:5432/whatomate_test?sslmode=disable' TEST_REDIS_URL='redis://127.0.0.1:6379/1' go test ./internal/handlers/ -run 'TestOccurrencePermissions' -count=1 -v
```

Esperado: os que dependem das permissões novas falham, porque `occurrences:read` ainda não existe e `CreateTestRoleWithKeys` ignora chave desconhecida — então o papel sai sem permissão nenhuma e tudo dá 403, inclusive onde se espera 200.

Se aparecer `ok` com `no tests to run`, **as variáveis de ambiente não chegaram**.

- [ ] **Step 3: Declare os recursos**

Em `internal/models/roles.go`, no bloco de constantes de recurso, logo após `ResourceAuditLogs`:

```go
	ResourceAuditLogs               = "audit_logs"
	ResourceOccurrences             = "occurrences"
	ResourceOccurrenceStages        = "occurrences.stages"
)
```

- [ ] **Step 4: Acrescente as permissões ao catálogo**

Ainda em `roles.go`, ao final da lista de `DefaultPermissions()`, antes do fechamento do slice:

```go
		// CRM — ocorrências. Não há permissão de exclusão porque não existe
		// endpoint de exclusão de ocorrência. `occurrences:read` também cobre
		// a LEITURA das etapas: o quadro não renderiza sem elas, então isso é
		// usar o CRM, não administrá-lo.
		{Resource: ResourceOccurrences, Action: ActionRead, Description: "View occurrences and pipeline stages"},
		{Resource: ResourceOccurrences, Action: ActionWrite, Description: "Create and edit occurrences"},

		// CRM — administração do funil, separada de settings.general para que
		// configurar etapas não exija as configurações gerais da organização.
		{Resource: ResourceOccurrenceStages, Action: ActionWrite, Description: "Create and edit occurrence stages"},
		{Resource: ResourceOccurrenceStages, Action: ActionDelete, Description: "Delete occurrence stages"},
```

- [ ] **Step 5: Conceda aos papéis de sistema**

O admin deriva de `DefaultPermissions()` e já recebe tudo. Acrescente às listas literais.

Em `managerPermissions`, junto das entradas de chat:

```go
		"chat:read", "chat:write", "chat.assign:write",
		// CRM: o gestor usa e administra o funil, coerente com ter settings.general:write
		"occurrences:read", "occurrences:write",
		"occurrences.stages:write", "occurrences.stages:delete",
```

Em `agentPermissions`, junto das entradas de chat:

```go
		"chat:read", "chat:write",
		// CRM: o atendente usa; administrar o funil não é papel dele
		"occurrences:read", "occurrences:write",
```

- [ ] **Step 6: Troque as constantes nos handlers**

Em `internal/handlers/occurrences.go`, troque `models.ResourceChat` por `models.ResourceOccurrences` nas oito chamadas de `requireAuth` (linhas 107, 190, 255, 367, 381, 451, 527, 563). As ações ficam como estão.

Em `internal/handlers/occurrence_send.go` linha 31, a mesma troca.

Em `internal/handlers/occurrence_stages.go`:

- linha 26 (`ListOccurrenceStages`): `models.ResourceChat` → `models.ResourceOccurrences`, ação segue `ActionRead`
- linha 50 (`CreateOccurrenceStage`): `models.ResourceSettingsGeneral` → `models.ResourceOccurrenceStages`, ação segue `ActionWrite`
- linha 103 (`UpdateOccurrenceStage`): `models.ResourceSettingsGeneral` → `models.ResourceOccurrenceStages`, ação segue `ActionWrite`
- linha 185 (`DeleteOccurrenceStage`): `models.ResourceSettingsGeneral` → `models.ResourceOccurrenceStages`, ação passa de `ActionWrite` para **`ActionDelete`**

Atenção na última: é a única em que a **ação** muda, e é o que o teste `StageWriteCannotDelete` verifica.

Atualize também o comentário acima de `CreateOccurrenceStage`, que hoje diz que a configuração vive sob `settings.general` e que o CRM não introduz permissão nova. Passou a introduzir:

```go
// CreateOccurrenceStage adds a stage. Administering the pipeline lives under
// occurrences.stages, separate from settings.general so that configuring the
// funnel does not require the organisation's general settings.
```

Nenhuma outra linha desses handlers muda.

- [ ] **Step 7: Rode e confirme que passam**

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc" && TEST_DATABASE_URL='postgres://postgres:postgres@127.0.0.1:5432/whatomate_test?sslmode=disable' TEST_REDIS_URL='redis://127.0.0.1:6379/1' go test ./internal/handlers/ -run 'TestOccurrence' -count=1
```

Esperado: `ok`. Os novos passam **e** os 39 testes de ocorrência anteriores continuam passando — eles usam papel de admin, que tem tudo.

- [ ] **Step 8: Teste de mutação no mapeamento**

Troque temporariamente a permissão de `CreateOccurrenceStage` para `models.ResourceOccurrences, models.ActionWrite`:

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc" && TEST_DATABASE_URL='postgres://postgres:postgres@127.0.0.1:5432/whatomate_test?sslmode=disable' TEST_REDIS_URL='redis://127.0.0.1:6379/1' go test ./internal/handlers/ -run 'TestOccurrencePermissions_CRMUserCannotAdministerStages' -count=1
```

Esperado: **FALHA**. É o erro que a spec diz que este teste existe para pegar. Restaure e rode o Step 7 de novo.

- [ ] **Step 9: Build, vet e commit**

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc" && go build ./... && go vet ./...
```

```bash
git add internal/models/roles.go internal/handlers/ && git commit -m "feat(crm): give the CRM its own permissions instead of borrowing chat's"
```

---

## Task 2: O backfill

**Files:**
- Create: `internal/database/permissions_backfill.go`
- Create: `internal/database/permissions_backfill_test.go`
- Modify: `cmd/whatomate/main.go` (bloco `if *migrate`, por volta da linha 160)

**Interfaces:**
- Consumes: as quatro chaves de permissão da T1.
- Produces: `database.BackfillOccurrencePermissions(db *gorm.DB) error`.

### Contexto que você precisa

**Por que existe.** `SeedPermissionsAndRoles` cria as linhas de permissão novas, mas `FixSystemRolePermissions` pula qualquer papel que já tenha alguma permissão (`postgres.go:544`, `if permCount > 0 { continue }`). Em produção todos os papéis já têm permissões, então sem este backfill ninguém — nem o admin — recebe as permissões novas, e o módulo responde 403 para todos.

**A regra é equivalência exata com a capacidade de hoje:**

| Papel que tem | Ganha |
|---|---|
| `chat:read` | `occurrences:read` |
| `chat:write` | `occurrences:write` |
| `settings.general:write` | `occurrences.stages:write` **e** `occurrences.stages:delete` |

`roles:write` **não** entra: quem tem só isso não administra etapas hoje, e concedê-lo alargaria acesso numa migração que promete não alterar o acesso de ninguém.

**Idempotência por forma do dado**, como `BackfillChatbotFlowGraph`, sem tabela de controle: uma organização que já tenha qualquer papel com qualquer permissão `occurrences%` é pulada inteira.

**Puramente aditivo.** Nunca revogue nada. É o que torna o rollback seguro, porque a versão anterior autoriza por `chat:*`.

A tabela de junção é `role_permissions`, com colunas `custom_role_id` e `permission_id`.

- [ ] **Step 1: Escreva os testes que falham**

Crie `internal/database/permissions_backfill_test.go`:

```go
package database_test

import (
	"testing"

	"github.com/shridarpatil/whatomate/internal/database"
	"github.com/shridarpatil/whatomate/internal/models"
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
```

O par `testutil.SetupTestDB(t)` + `cleanAll(t, db)` é o padrão já em uso neste pacote — `cleanAll` está definido em `internal/database/database_test.go:18` e trunca todas as tabelas, então **não o redeclare**: o arquivo novo fica no mesmo pacote `database_test` e usa o que já existe. Os fixtures de `testutil` são acessíveis daqui; `database_test.go` já importa o pacote.

- [ ] **Step 2: Rode e confirme que falham**

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc" && TEST_DATABASE_URL='postgres://postgres:postgres@127.0.0.1:5432/whatomate_test?sslmode=disable' TEST_REDIS_URL='redis://127.0.0.1:6379/1' go test ./internal/database/ -run 'TestBackfill' -count=1 -v
```

Esperado: não compila, porque `BackfillOccurrencePermissions` não existe.

- [ ] **Step 3: Implemente o backfill**

Crie `internal/database/permissions_backfill.go`:

```go
package database

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"gorm.io/gorm"
)

// grantRule liga uma capacidade que o papel já tem à permissão que ele ganha.
type grantRule struct {
	fromResource string
	fromAction   string
	toResource   string
	toAction     string
}

// occurrenceGrants é a equivalência exata com a capacidade de hoje. Quem
// administra etapas neste momento é quem tem settings.general:write, e é só
// esse que recebe as permissões de funil — roles:write não entra, porque
// concedê-lo alargaria acesso numa migração que promete não alterar o de
// ninguém.
var occurrenceGrants = []grantRule{
	{models.ResourceChat, models.ActionRead, models.ResourceOccurrences, models.ActionRead},
	{models.ResourceChat, models.ActionWrite, models.ResourceOccurrences, models.ActionWrite},
	{models.ResourceSettingsGeneral, models.ActionWrite, models.ResourceOccurrenceStages, models.ActionWrite},
	{models.ResourceSettingsGeneral, models.ActionWrite, models.ResourceOccurrenceStages, models.ActionDelete},
}

// BackfillOccurrencePermissions concede as permissões do CRM aos papéis que já
// têm a capacidade equivalente por chat e por configurações gerais.
//
// Existe porque FixSystemRolePermissions pula qualquer papel que já tenha
// permissões, para não desfazer customizações — o que significa que uma
// permissão nova jamais chega a uma instalação existente por aquele caminho.
//
// É PURAMENTE ADITIVO: nunca revoga nada. Essa é a propriedade que torna o
// rollback seguro, porque a versão anterior autoriza por chat:* e ele
// permanece intacto.
//
// Idempotência é pela forma do dado, como BackfillChatbotFlowGraph: uma
// organização que já tenha qualquer papel com qualquer permissão occurrences%
// é pulada inteira, então isto roda uma vez por organização.
func BackfillOccurrencePermissions(db *gorm.DB) error {
	// As linhas de permissão são criadas por SeedPermissionsAndRoles. Se ainda
	// não existem, não há o que ligar e este boot não faz nada.
	var seeded int64
	if err := db.Model(&models.Permission{}).
		Where("resource IN ?", []string{models.ResourceOccurrences, models.ResourceOccurrenceStages}).
		Count(&seeded).Error; err != nil {
		return fmt.Errorf("failed to count occurrence permissions: %w", err)
	}
	if seeded < int64(len(occurrenceGrants)) {
		return nil
	}

	var pending []uuid.UUID
	if err := db.Raw(`
		SELECT o.id
		FROM organizations o
		WHERE NOT EXISTS (
			SELECT 1
			FROM custom_roles r
			JOIN role_permissions rp ON rp.custom_role_id = r.id
			JOIN permissions p ON p.id = rp.permission_id
			WHERE r.organization_id = o.id
			  AND p.resource LIKE 'occurrences%'
		)`).Scan(&pending).Error; err != nil {
		return fmt.Errorf("failed to list organisations pending the occurrence backfill: %w", err)
	}
	if len(pending) == 0 {
		return nil
	}

	for _, g := range occurrenceGrants {
		if err := db.Exec(`
			INSERT INTO role_permissions (custom_role_id, permission_id)
			SELECT r.id, target.id
			FROM custom_roles r
			JOIN role_permissions rp ON rp.custom_role_id = r.id
			JOIN permissions src ON src.id = rp.permission_id
			CROSS JOIN permissions target
			WHERE r.organization_id IN ?
			  AND src.resource = ? AND src.action = ?
			  AND target.resource = ? AND target.action = ?
			  AND NOT EXISTS (
				SELECT 1 FROM role_permissions existing
				WHERE existing.custom_role_id = r.id
				  AND existing.permission_id = target.id
			  )`,
			pending, g.fromResource, g.fromAction, g.toResource, g.toAction,
		).Error; err != nil {
			return fmt.Errorf("failed to grant %s:%s from %s:%s: %w",
				g.toResource, g.toAction, g.fromResource, g.fromAction, err)
		}
	}

	return nil
}
```

- [ ] **Step 4: Rode e confirme que passam**

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc" && TEST_DATABASE_URL='postgres://postgres:postgres@127.0.0.1:5432/whatomate_test?sslmode=disable' TEST_REDIS_URL='redis://127.0.0.1:6379/1' go test ./internal/database/ -run 'TestBackfill' -count=1 -v
```

Esperado: os sete passam.

- [ ] **Step 5: Teste de mutação na guarda de organização**

Remova temporariamente a cláusula `AND r.organization_id IN ?` e o filtro de organizações pendentes, deixando o `INSERT` valer para todas:

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc" && TEST_DATABASE_URL='postgres://postgres:postgres@127.0.0.1:5432/whatomate_test?sslmode=disable' TEST_REDIS_URL='redis://127.0.0.1:6379/1' go test ./internal/database/ -run 'TestBackfill_SkipsOrganisationAlreadyMigrated' -count=1
```

Esperado: **FALHA**. Restaure e rode o Step 4 de novo.

- [ ] **Step 6: Ligue no boot**

Em `cmd/whatomate/main.go`, dentro do bloco `if *migrate`, **depois** de `BackfillChatbotFlowGraph`:

```go
		// Concede as permissões do CRM a quem já tem a capacidade equivalente.
		// Precisa rodar aqui, no bloco de migração: o ListenAndServe só
		// acontece depois, então nenhuma requisição chega antes de os papéis
		// estarem corrigidos e não existe janela de 403.
		if err := database.BackfillOccurrencePermissions(db); err != nil {
			lo.Fatal("Occurrence permissions backfill failed", "error", err)
		}
```

- [ ] **Step 7: Verifique o boot completo**

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc" && go build ./... && go vet ./... && TEST_DATABASE_URL='postgres://postgres:postgres@127.0.0.1:5432/whatomate_test?sslmode=disable' TEST_REDIS_URL='redis://127.0.0.1:6379/1' go test ./internal/database/ ./internal/handlers/ -count=1
```

Esperado: `ok` nos dois pacotes.

- [ ] **Step 8: Commit**

```bash
git add internal/database/ cmd/whatomate/main.go && git commit -m "feat(crm): backfill the new CRM permissions onto existing roles"
```

---

## Task 3: Frontend

**Files:**
- Modify: `frontend/src/router/index.ts`
- Modify: `frontend/src/components/layout/navigation.ts`
- Modify: `frontend/src/lib/constants.ts`

**Interfaces:**
- Consumes: as strings de recurso `occurrences` e `occurrences.stages` da T1.
- Produces: nada.

### Contexto que você precisa

O editor de funções monta os grupos sozinho a partir da API (`permissionGroups`, `stores/roles.ts:47`), com fallback para o nome do recurso capitalizado. `RESOURCE_LABELS` fica em `lib/constants.ts` e é um mapa de strings **fixas em inglês** — não passa pelo i18n. Esta tarefa **não acrescenta nenhuma chave de tradução**.

A rota de configuração de etapas aparece em **duas** listas no `router/index.ts`: a definição da rota (por volta da linha 303) e uma lista de permissões por caminho (por volta da linha 393). As duas precisam mudar, senão a tela fica acessível por um caminho e barrada pelo outro.

- [ ] **Step 1: Troque as permissões das rotas**

Em `frontend/src/router/index.ts`:

```ts
        {
          path: 'crm/occurrences',
          name: 'occurrences',
          component: () => import('@/views/crm/OccurrencesView.vue'),
          meta: { permission: 'occurrences' }
        },
        {
          path: 'crm/occurrences/:id',
          name: 'occurrence-detail',
          component: () => import('@/views/crm/OccurrenceDetailView.vue'),
          meta: { permission: 'occurrences' }
        },
```

e a rota de etapas:

```ts
        {
          path: 'settings/occurrence-stages',
          name: 'occurrence-stages',
          component: () => import('@/views/settings/OccurrenceStagesView.vue'),
          meta: { permission: 'occurrences.stages' }
        },
```

e a entrada correspondente na lista de permissões por caminho:

```ts
    { path: '/settings/occurrence-stages', permission: 'occurrences.stages' },
```

- [ ] **Step 2: Troque a permissão do menu**

Em `frontend/src/components/layout/navigation.ts`:

```ts
      {
        name: 'nav.crm',
        path: '/crm/occurrences',
        icon: ClipboardList,
        permission: 'occurrences'
      },
```

- [ ] **Step 3: Acrescente os rótulos**

Em `frontend/src/lib/constants.ts`, dentro de `RESOURCE_LABELS`:

```ts
  occurrences: 'Occurrences',
  'occurrences.stages': 'Occurrence Stages',
```

Sem eles o editor exibiria `Occurrences` pelo fallback — aceitável — e `Occurrences.stages`, que não é.

- [ ] **Step 4: Verifique**

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc/frontend" && npm run typecheck && npm run i18n:keys && npx eslint src/router/index.ts src/components/layout/navigation.ts src/lib/constants.ts && npm run build
```

Esperado: passa, com o único erro conhecido do `AccountDetailView`.

- [ ] **Step 5: Escreva os testes E2E de bloqueio**

A spec (§7) exige provar que um papel sem a permissão não alcança o módulo. Crie `frontend/e2e/tests/crm/occurrence-permissions.spec.ts`:

```ts
import { test, expect } from '@playwright/test'
import { login } from '../../helpers'
import { ApiHelper } from '../../helpers/api'
import { createTestScope } from '../../framework'

const scope = createTestScope('crm-perms')

/** Cria um papel com exatamente estas permissões e um usuário nele. */
async function userWithPermissions(api: ApiHelper, label: string, permissions: string[]) {
  await api.loginAsAdmin()
  const role = await api.createRole({
    name: scope.name(label),
    description: `E2E ${label}`,
    permissions,
  })
  const email = `${scope.name(label)}@example.com`
  const password = 'Test1234!'
  await api.createUser({
    email,
    password,
    full_name: scope.name(label),
    role_id: role.id,
  })
  return { email, password }
}

test.describe('CRM permissions', () => {
  test('a role without occurrences cannot reach the CRM', async ({ page, request }) => {
    const api = new ApiHelper(request)
    const user = await userWithPermissions(api, 'sem-crm', ['chat:read', 'chat:write'])

    await login(page, user)
    await page.goto('/crm/occurrences')
    await page.waitForLoadState('networkidle')

    // Nem o item de menu, nem a tela.
    await expect(page.locator('#occurrences-list')).toBeHidden()
    await expect(page.locator('#occurrences-board')).toBeHidden()
    expect(page.url()).not.toContain('/crm/occurrences')
  })

  test('a CRM role cannot reach the stage settings', async ({ page, request }) => {
    const api = new ApiHelper(request)
    const user = await userWithPermissions(api, 'so-crm', [
      'chat:read', 'chat:write', 'occurrences:read', 'occurrences:write',
    ])

    await login(page, user)

    await page.goto('/crm/occurrences')
    await page.waitForLoadState('networkidle')
    await expect(page.locator('#occurrences-list')).toBeVisible()

    await page.goto('/settings/occurrence-stages')
    await page.waitForLoadState('networkidle')
    expect(page.url()).not.toContain('/settings/occurrence-stages')
  })
})
```

O segundo teste é o que dá valor: prova que a mesma pessoa **usa** o CRM e **não administra** o funil — que é a separação inteira desta spec, vista pela interface.

`login(page, user)` está em `frontend/e2e/helpers/auth.ts:28`; `createRole` e `createUser` em `helpers/api.ts:217` e `:236`. O `scope.name()` é o prefixo por onde a limpeza global encontra o que apagar.

- [ ] **Step 6: Rode os E2E**

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc/frontend" && TEST_DATABASE_URL='postgres://postgres:postgres@127.0.0.1:5432/whatomate_e2e?sslmode=disable' BASE_URL=http://localhost:3000 npx playwright test e2e/tests/crm/ --workers=1
```

Esperado: os dois novos passam e os 18 do quadro continuam passando. Backend e vite precisam estar de pé **contra o banco `whatomate_e2e`** — se falhar em massa por dados ausentes ou login, é ambiente, não código.

Confira depois que a limpeza não deixou papéis nem usuários órfãos:

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc" && docker exec whatc-pg psql -U postgres -d whatomate_e2e -tAc "select (select count(*) from custom_roles where name like '%crm-perms%'), (select count(*) from users where email like '%crm-perms%');"
```

Esperado: `0|0` depois da rodada seguinte. Se sobrar, acrescente a limpeza em `global-cleanup.ts`.

- [ ] **Step 7: Confira na tela**

Com backend e vite de pé, abra Configurações › Funções e edite uma função. Confirme que aparecem os grupos **Occurrences** (com View e Create/Edit) e **Occurrence Stages** (com Create/Edit e Delete), e que nenhum deles aparece como `Occurrences.stages`.

- [ ] **Step 8: Commit**

```bash
git add frontend/src frontend/e2e && git commit -m "feat(crm): point the CRM screens at their own permission"
```

---

## Verificação final

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc" && go build ./... && go vet ./... && TEST_DATABASE_URL='postgres://postgres:postgres@127.0.0.1:5432/whatomate_test?sslmode=disable' TEST_REDIS_URL='redis://127.0.0.1:6379/1' go test ./internal/... -count=1
```

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc/frontend" && npm run typecheck && npm run build
```

## Pré-condição de deploy

**Antes do rollout**, confirmar que o comando da aplicação no `docker-compose.yml` de produção inclui `-migrate`. O `deploy-whatc` faz `docker compose up -d app` sem passar argumentos próprios, então a flag tem de estar no compose.

- **Se estiver:** nada mais é necessário. O backfill roda antes do `ListenAndServe` e não existe janela de 403.
- **Se não estiver:** rodar o binário novo com `-migrate` uma vez contra o banco de produção **antes** de subir a aplicação nova.

Sem essa verificação confirmada, o rollout não começa. Rollback é seguro em qualquer ponto: o backfill nunca revoga nada, e a versão anterior autoriza por `chat:*`.

## Riscos

| Risco | Mitigação |
|---|---|
| Produção sem `-migrate`: CRM inacessível para todos | Pré-condição verificável acima; sem ela, o rollout não começa |
| Gestor perder a administração do funil | Backfill mapeia `settings.general:write` um-para-um; teste dedicado na T2 |
| Erro de mapeamento ligar administração de funil a `occurrences:write` | Teste de mutação da T1, Step 8 |
| Backfill re-conceder permissão removida de propósito | Aceito e registrado na spec §4; estreito e reversível na tela |
| Rota de etapas mudar em um lugar só | A T3 lista explicitamente as duas ocorrências no `router/index.ts` |
