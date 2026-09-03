# Polimento de atendimento — plano de implementação

> **Para agentes executores:** SUB-SKILL OBRIGATÓRIA: use `superpowers:subagent-driven-development` (recomendado) ou `superpowers:executing-plans` para implementar tarefa a tarefa. Os passos usam caixas (`- [ ]`) para acompanhamento.

**Objetivo:** quatro melhorias de atendimento — tooltips no menu recolhido, copiar nome e telefone, o agente renomeando o contato, e rótulos no card do Kanban.

**Arquitetura:** três das quatro são interface pura. Renomear o contato cria o recurso de permissão `contacts.name` atrás de um endpoint próprio, seguindo o padrão que `contacts.go` já usa em `/assign`, `/tags` e `/status`, com backfill aditivo no bloco de migração do boot.

**Stack:** Go 1.25 + fastglue + GORM + Postgres; Vue 3 + TypeScript + Pinia + reka-ui.

**Spec:** `docs/superpowers/specs/2026-09-02-polimento-atendimento-design.md`

## Restrições globais

Valem para **todas** as tarefas.

- **Nenhuma permissão existente é removida de papel algum.** O backfill só concede.
- Renomear **não** dá acesso a nenhum outro campo do contato. O endpoint aceita um único campo.
- A visibilidade de conversa continua valendo: `canInteractWithConversation` no handler de renomear. Nada de mexer em `scopeVisibleConversations` ou na lógica de visibilidade.
- Os testes de banco em Go **pulam em silêncio** sem `TEST_DATABASE_URL` **e** `TEST_REDIS_URL`, e a suíte reporta `ok` sem ter rodado nada. Sempre passe as duas.
- **Não rode `npm run lint`** — tem `--fix` e reescreve o repositório. Use `npx eslint <arquivos>`.
- Único erro aceitável no `npm run typecheck`: o pré-existente `AccountDetailView.vue(172,45) business_calling_enabled`.
- Toda string visível existe nos **dois** locales, mantidos paralelos.
- Falhas pré-existentes conhecidas, que não são suas: `TestApp_ServeMedia_RejectsSymlink` (privilégio de symlink no Windows) e o teste do processador de SLA em `sla_processor_test.go:207`.

## Estrutura de arquivos

| Arquivo | Responsabilidade | Tarefa |
|---|---|---|
| `internal/models/models.go` | referência: `Contact.ProfileName` é o campo real | — |
| `internal/models/roles.go` | modificar: `ResourceContactName`, `DefaultPermissions`, `agentPermissions` | T1 |
| `internal/handlers/contacts.go` | modificar: `UpdateContactNameRequest` e `UpdateContactName` | T1 |
| `internal/handlers/contact_name_test.go` | criar: testes do endpoint | T1 |
| `internal/database/permissions_backfill.go` | modificar: `BackfillContactNamePermission` | T1 |
| `internal/database/permissions_backfill_test.go` | modificar: testes do backfill novo | T1 |
| `cmd/whatomate/main.go` | modificar: rota nova e chamada do backfill | T1 |
| `frontend/src/services/api.ts` | modificar: `contactsService.updateName` | T2 |
| `frontend/src/components/chat/ContactInfoPanel.vue` | modificar: copiar e renomear | T2 |
| `frontend/src/stores/contacts.ts` | modificar: `updateContactName` | T2 |
| `frontend/src/views/chat/ChatView.vue` | modificar: ouvinte de `name-updated` | T2 |
| `frontend/e2e/tests/chat/contact-info.spec.ts` | criar: copiar e renomear | T2 |
| `frontend/e2e/tests/layout/sidebar-tooltips.spec.ts` | criar: tooltips quando recolhido | T3 |
| `frontend/src/components/layout/AppLayout.vue` | modificar: tooltips quando recolhido | T3 |
| `frontend/src/components/crm/OccurrenceCard.vue` | modificar: rótulos | T3 |
| `frontend/src/i18n/locales/{en,pt-BR}.json` | modificar: chaves novas | T2, T3 |

---

## Task 1: Permissão, endpoint e backfill para renomear o contato

**Files:**
- Modify: `internal/models/roles.go`
- Modify: `internal/handlers/contacts.go`
- Create: `internal/handlers/contact_name_test.go`
- Modify: `internal/database/permissions_backfill.go`
- Modify: `internal/database/permissions_backfill_test.go`
- Modify: `cmd/whatomate/main.go`

**Interfaces:**
- Consumes: nada.
- Produces: a constante `models.ResourceContactName = "contacts.name"`, a permissão `contacts.name:write`, a rota `PUT /api/contacts/{id}/name` e a função `database.BackfillContactNamePermission(db *gorm.DB, lo logf.Logger) error`. A T2 depende da rota e do formato do corpo.

### Contexto que você precisa

**O campo real é `ProfileName`, não `Name`.** O modelo `Contact` (`internal/models/models.go`) tem `ProfileName string \`json:"profile_name"\`` e **não tem** campo `Name`. A resposta da API (`ContactResponse`, `contacts.go:26`) expõe o mesmo valor em **dois** campos, `name` e `profile_name`, ambos preenchidos a partir de `c.ProfileName` (`contacts.go:199-200`). O frontend lê `contact.name`. Ao gravar, grave `profile_name`.

**Existe máscara, e ela é uma armadilha de perda de dados.** Quando a organização tem `mask_phone_numbers` ligado nas configurações (`ShouldMaskPhoneNumbers`), o nome é passado por `MaskIfPhoneNumber`, que o substitui por `****1234` **se o nome parecer um telefone** — o que acontece quando o contato nunca informou nome. Se a tela pré-preencher o campo com esse valor mascarado e o usuário salvar, o número real vira `****1234` no banco, irreversivelmente.

Por isso este endpoint **recusa qualquer nome contendo `*`**. Nome de gente não tem asterisco, e a guarda fecha a classe inteira de erro independentemente do que o cliente envie. É barato e definitivo.

O papel `agent` tem hoje `contacts:read` e nada mais (`roles.go:333`, com o comentário `// Contacts (read only)`).

- [ ] **Step 1: Escreva os testes que falham**

Crie `internal/handlers/contact_name_test.go`:

```go
package handlers_test

import (
	"testing"

	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func TestUpdateContactName_RequiresPermission(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	role := testutil.CreateTestRoleWithKeys(t, app.DB, org.ID, "sem-renomear",
		[]string{"chat:read", "chat:write", "contacts:read"})
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))
	contact := testutil.CreateTestContact(t, app.DB, org.ID)

	req := testutil.NewJSONRequest(t, map[string]any{"name": "Nome Novo"})
	testutil.SetPathParam(req, "id", contact.ID.String())
	testutil.SetAuthContext(req, org.ID, user.ID)

	require.NoError(t, app.UpdateContactName(req))
	assert.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(req))
}

func TestUpdateContactName_RenamesForPermittedUser(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	role := testutil.CreateTestRoleWithKeys(t, app.DB, org.ID, "atendente",
		[]string{"chat:read", "chat:write", "contacts:read", "contacts.name:write"})
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))
	contact := testutil.CreateTestContact(t, app.DB, org.ID)

	req := testutil.NewJSONRequest(t, map[string]any{"name": "Maria da Silva"})
	testutil.SetPathParam(req, "id", contact.ID.String())
	testutil.SetAuthContext(req, org.ID, user.ID)

	require.NoError(t, app.UpdateContactName(req))
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var reloaded models.Contact
	require.NoError(t, app.DB.First(&reloaded, "id = ?", contact.ID).Error)
	assert.Equal(t, "Maria da Silva", reloaded.ProfileName)
}

// Nenhum outro campo do contato pode mudar. Um endpoint estreito que mexe
// em mais de um campo deixa de ser estreito.
func TestUpdateContactName_TouchesNothingElse(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	role := testutil.CreateTestRoleWithKeys(t, app.DB, org.ID, "atendente",
		[]string{"chat:read", "chat:write", "contacts:read", "contacts.name:write"})
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))
	contact := testutil.CreateTestContact(t, app.DB, org.ID)

	var before models.Contact
	require.NoError(t, app.DB.First(&before, "id = ?", contact.ID).Error)

	req := testutil.NewJSONRequest(t, map[string]any{"name": "Outro Nome"})
	testutil.SetPathParam(req, "id", contact.ID.String())
	testutil.SetAuthContext(req, org.ID, user.ID)
	require.NoError(t, app.UpdateContactName(req))
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var after models.Contact
	require.NoError(t, app.DB.First(&after, "id = ?", contact.ID).Error)
	assert.Equal(t, before.PhoneNumber, after.PhoneNumber)
	assert.Equal(t, before.AssignedUserID, after.AssignedUserID)
	assert.Equal(t, before.TeamID, after.TeamID)
	assert.Equal(t, before.WhatsAppAccount, after.WhatsAppAccount)
}

// A permissão nova NÃO pode furar a visibilidade de conversa que a Fase 1
// construiu: renomear contato que o usuário não enxerga seria acesso lateral.
func TestUpdateContactName_DeniedForInvisibleContact(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	agentRole := testutil.CreateAgentRole(t, app.DB, org.ID)
	owner := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&agentRole.ID))
	outsiderRole := testutil.CreateTestRoleWithKeys(t, app.DB, org.ID, "renomeador-de-fora",
		[]string{"chat:read", "chat:write", "contacts:read", "contacts.name:write"})
	outsider := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&outsiderRole.ID))
	enableStrictVisibility(t, app, org.ID)

	contact := testutil.CreateTestContact(t, app.DB, org.ID)
	require.NoError(t, app.DB.Model(contact).Update("assigned_user_id", owner.ID).Error)

	var before models.Contact
	require.NoError(t, app.DB.First(&before, "id = ?", contact.ID).Error)

	req := testutil.NewJSONRequest(t, map[string]any{"name": "Nao Deveria Renomear"})
	testutil.SetPathParam(req, "id", contact.ID.String())
	testutil.SetAuthContext(req, org.ID, outsider.ID)

	require.NoError(t, app.UpdateContactName(req))
	assert.Equal(t, fasthttp.StatusForbidden, testutil.GetResponseStatusCode(req))

	var after models.Contact
	require.NoError(t, app.DB.First(&after, "id = ?", contact.ID).Error)
	assert.Equal(t, before.ProfileName, after.ProfileName, "o nome nao pode ter mudado")
}

// Guarda contra perda de dados: com mascara ligada, o nome exibido pode ser
// ****1234. Salvar isso gravaria a mascara por cima do numero real.
func TestUpdateContactName_RejectsMaskedValue(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	role := testutil.CreateTestRoleWithKeys(t, app.DB, org.ID, "atendente",
		[]string{"chat:read", "chat:write", "contacts:read", "contacts.name:write"})
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))
	contact := testutil.CreateTestContact(t, app.DB, org.ID)

	req := testutil.NewJSONRequest(t, map[string]any{"name": "*******1234"})
	testutil.SetPathParam(req, "id", contact.ID.String())
	testutil.SetAuthContext(req, org.ID, user.ID)

	require.NoError(t, app.UpdateContactName(req))
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

func TestUpdateContactName_RejectsEmpty(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	role := testutil.CreateTestRoleWithKeys(t, app.DB, org.ID, "atendente",
		[]string{"chat:read", "chat:write", "contacts:read", "contacts.name:write"})
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&role.ID))
	contact := testutil.CreateTestContact(t, app.DB, org.ID)

	req := testutil.NewJSONRequest(t, map[string]any{"name": "   "})
	testutil.SetPathParam(req, "id", contact.ID.String())
	testutil.SetAuthContext(req, org.ID, user.ID)

	require.NoError(t, app.UpdateContactName(req))
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}
```

`enableStrictVisibility` e `CreateAgentRole` já existem e são usados em `internal/handlers/occurrences_test.go:40-49` — não os redeclare.

- [ ] **Step 2: Rode e confirme que falham**

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc" && TEST_DATABASE_URL='postgres://postgres:postgres@127.0.0.1:5432/whatomate_test?sslmode=disable' TEST_REDIS_URL='redis://127.0.0.1:6379/1' go test ./internal/handlers/ -run 'TestUpdateContactName' -count=1 -v
```

Esperado: não compila, porque `app.UpdateContactName` não existe.

Se aparecer `ok` com `no tests to run`, **as variáveis de ambiente não chegaram**.

- [ ] **Step 3: Declare o recurso e a permissão**

Em `internal/models/roles.go`, no bloco de constantes, junto de `ResourceContacts`:

```go
	ResourceContacts                = "contacts"
	ResourceContactName             = "contacts.name"
```

Em `DefaultPermissions()`, logo após as entradas de `ResourceContacts`:

```go
		// Renomear o contato, separado de contacts:write para que o atendente
		// possa corrigir o nome sem ganhar o contato inteiro.
		{Resource: ResourceContactName, Action: ActionWrite, Description: "Rename contacts"},
```

Em `agentPermissions`, junto da linha de contatos:

```go
		// Contacts (read only)
		"contacts:read",
		// Renomear: agilidade no atendimento, sem abrir o resto do contato
		"contacts.name:write",
```

- [ ] **Step 4: Escreva o handler**

Em `internal/handlers/contacts.go`, junto dos outros handlers estreitos de contato. Confirme que `strings` já está importado no arquivo; se não estiver, acrescente.

```go
// UpdateContactNameRequest carrega o único campo que este endpoint aceita.
type UpdateContactNameRequest struct {
	Name string `json:"name"`
}

// UpdateContactName renomeia o contato e nada mais.
//
// Endpoint próprio em vez de um caminho dentro de UpdateContact: um endpoint,
// um portão. Um condicional dentro do update largo teria de acertar quais
// campos ignorar, e erraria em silêncio no dia em que alguém acrescentasse um.
func (a *App) UpdateContactName(r *fastglue.Request) error {
	orgID, userID, err := a.requireAuth(r, models.ResourceContactName, models.ActionWrite)
	if err != nil {
		return nil
	}

	contactID, err := parsePathUUID(r, "id", "contact")
	if err != nil {
		return nil
	}

	contact, err := findByIDAndOrg[models.Contact](a.DB, r, contactID, orgID, "Contact")
	if err != nil {
		return nil
	}

	// A permissão não pode furar a visibilidade: renomear contato que o
	// usuário não enxerga seria acesso lateral ao escopo de conversa.
	if !a.canInteractWithConversation(userID, orgID, contact) {
		return r.SendErrorEnvelope(fasthttp.StatusForbidden,
			"You do not have access to this conversation", nil, "")
	}

	var req UpdateContactNameRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "name is required", nil, "")
	}

	// Com mask_phone_numbers ligado, um nome que parece telefone é exibido
	// como ****1234. Salvar esse valor gravaria a máscara por cima do número
	// real, sem volta. Nome de gente não tem asterisco.
	if strings.Contains(name, "*") {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest,
			"name cannot contain masked characters", nil, "")
	}

	if err := a.DB.Model(contact).Update("profile_name", name).Error; err != nil {
		a.Log.Error("Failed to rename contact", "error", err, "contact_id", contactID)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError,
			"Failed to rename contact", nil, "")
	}

	return r.SendEnvelope(map[string]any{
		"id":           contactID,
		"name":         name,
		"profile_name": name,
	})
}
```

- [ ] **Step 5: Registre a rota**

Em `cmd/whatomate/main.go`, junto das outras rotas estreitas de contato (perto de `g.PUT("/api/contacts/{id}/tags", ...)`):

```go
	g.PUT("/api/contacts/{id}/name", app.UpdateContactName)
```

- [ ] **Step 6: Rode e confirme que passam**

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc" && TEST_DATABASE_URL='postgres://postgres:postgres@127.0.0.1:5432/whatomate_test?sslmode=disable' TEST_REDIS_URL='redis://127.0.0.1:6379/1' go test ./internal/handlers/ -run 'TestUpdateContactName' -count=1 -v
```

Esperado: os seis passam.

- [ ] **Step 7: Teste de mutação na guarda de visibilidade**

Comente o bloco `if !a.canInteractWithConversation(...)` do handler e rode:

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc" && TEST_DATABASE_URL='postgres://postgres:postgres@127.0.0.1:5432/whatomate_test?sslmode=disable' TEST_REDIS_URL='redis://127.0.0.1:6379/1' go test ./internal/handlers/ -run 'TestUpdateContactName_DeniedForInvisibleContact' -count=1
```

Esperado: **FALHA**. Restaure e rode o Step 6 de novo, confirmando `git diff internal/handlers/contacts.go` sem a mutação.

- [ ] **Step 8: Escreva o teste do backfill**

Em `internal/database/permissions_backfill_test.go`, acrescente:

```go
func TestBackfillContactName_GrantsFromContactsWrite(t *testing.T) {
	db := testutil.SetupTestDB(t)
	cleanAll(t, db)
	require.NoError(t, database.SeedPermissionsAndRoles(db))

	org := testutil.CreateTestOrganization(t, db)
	role := testutil.CreateTestRoleWithKeys(t, db, org.ID, "gestor",
		[]string{"contacts:read", "contacts:write"})

	require.NoError(t, database.BackfillContactNamePermission(db, testLog()))

	assert.Contains(t, roleKeys(t, db, role.ID), "contacts.name:write")
}

// A AMPLIACAO deliberada: quem atende passa a poder renomear, mesmo sem
// contacts:write. Difere de proposito da regra de equivalencia do CRM.
func TestBackfillContactName_GrantsFromChatWrite(t *testing.T) {
	db := testutil.SetupTestDB(t)
	cleanAll(t, db)
	require.NoError(t, database.SeedPermissionsAndRoles(db))

	org := testutil.CreateTestOrganization(t, db)
	role := testutil.CreateTestRoleWithKeys(t, db, org.ID, "atendente",
		[]string{"chat:read", "chat:write", "contacts:read"})

	require.NoError(t, database.BackfillContactNamePermission(db, testLog()))

	assert.Contains(t, roleKeys(t, db, role.ID), "contacts.name:write")
}

func TestBackfillContactName_SkipsRoleWithNeither(t *testing.T) {
	db := testutil.SetupTestDB(t)
	cleanAll(t, db)
	require.NoError(t, database.SeedPermissionsAndRoles(db))

	org := testutil.CreateTestOrganization(t, db)
	role := testutil.CreateTestRoleWithKeys(t, db, org.ID, "so-relatorios",
		[]string{"analytics:read"})

	require.NoError(t, database.BackfillContactNamePermission(db, testLog()))

	keys := roleKeys(t, db, role.ID)
	assert.NotContains(t, keys, "contacts.name:write")
	assert.Equal(t, []string{"analytics:read"}, keys)
}

func TestBackfillContactName_IsIdempotent(t *testing.T) {
	db := testutil.SetupTestDB(t)
	cleanAll(t, db)
	require.NoError(t, database.SeedPermissionsAndRoles(db))

	org := testutil.CreateTestOrganization(t, db)
	role := testutil.CreateTestRoleWithKeys(t, db, org.ID, "atendente",
		[]string{"chat:read", "chat:write"})

	require.NoError(t, database.BackfillContactNamePermission(db, testLog()))
	first := len(roleKeys(t, db, role.ID))
	require.NoError(t, database.BackfillContactNamePermission(db, testLog()))
	assert.Equal(t, first, len(roleKeys(t, db, role.ID)), "rodar duas vezes duplicou vinculos")
}

func TestBackfillContactName_NeverRemovesAnything(t *testing.T) {
	db := testutil.SetupTestDB(t)
	cleanAll(t, db)
	require.NoError(t, database.SeedPermissionsAndRoles(db))

	org := testutil.CreateTestOrganization(t, db)
	role := testutil.CreateTestRoleWithKeys(t, db, org.ID, "atendente",
		[]string{"chat:read", "chat:write", "contacts:read"})

	require.NoError(t, database.BackfillContactNamePermission(db, testLog()))

	keys := roleKeys(t, db, role.ID)
	assert.Contains(t, keys, "chat:read")
	assert.Contains(t, keys, "chat:write")
	assert.Contains(t, keys, "contacts:read")
}

func TestBackfillContactName_SkipsOrganisationAlreadyMigrated(t *testing.T) {
	db := testutil.SetupTestDB(t)
	cleanAll(t, db)
	require.NoError(t, database.SeedPermissionsAndRoles(db))

	migrada := testutil.CreateTestOrganization(t, db)
	testutil.CreateTestRoleWithKeys(t, db, migrada.ID, "ja-tem",
		[]string{"chat:read", "contacts.name:write"})
	depois := testutil.CreateTestRoleWithKeys(t, db, migrada.ID, "criado-depois",
		[]string{"chat:read", "chat:write"})

	pendente := testutil.CreateTestOrganization(t, db)
	pendenteRole := testutil.CreateTestRoleWithKeys(t, db, pendente.ID, "atendente",
		[]string{"chat:read", "chat:write"})

	require.NoError(t, database.BackfillContactNamePermission(db, testLog()))

	assert.NotContains(t, roleKeys(t, db, depois.ID), "contacts.name:write",
		"organizacao ja migrada deveria ser pulada inteira")
	assert.Contains(t, roleKeys(t, db, pendenteRole.ID), "contacts.name:write",
		"organizacao pendente deveria ser processada")
}
```

`roleKeys` (`permissions_backfill_test.go:21`), `testLog` (`:16`, um logger silencioso de nível Error) e `cleanAll` (`database_test.go:18`) **já existem no pacote — não os redeclare.**

O teste de organização já migrada tem **duas** organizações de propósito: com uma só, a consulta de pendentes volta vazia e a função retorna antes do `INSERT`, e o teste não exercitaria a guarda. Foi exatamente esse o defeito encontrado no backfill do CRM.

- [ ] **Step 9: Escreva o backfill**

Em `internal/database/permissions_backfill.go`, ao lado de `BackfillOccurrencePermissions`, seguindo a mesma forma (SQL única, `ON CONFLICT DO NOTHING`, `deleted_at IS NULL`, guarda correlacionada por organização):

```go
// BackfillContactNamePermission concede contacts.name:write aos papéis que
// já renomeiam contatos hoje e aos que atendem conversas.
//
// ATENÇÃO — esta regra difere DE PROPÓSITO da de BackfillOccurrencePermissions.
// Lá a regra era equivalência exata: ninguém ganhava capacidade nova. Aqui a
// segunda origem, chat:write, é uma AMPLIAÇÃO deliberada — o produto pediu que
// quem atende possa corrigir o nome do contato sem depender de um gestor.
//
// Puramente aditivo, como o outro: nunca revoga nada, então um rollback
// continua funcionando com as permissões antigas intactas.
func BackfillContactNamePermission(db *gorm.DB, lo logf.Logger) error {
	var seeded int64
	if err := db.Model(&models.Permission{}).
		Where("resource = ? AND action = ?", models.ResourceContactName, models.ActionWrite).
		Count(&seeded).Error; err != nil {
		return fmt.Errorf("failed to count the contact name permission: %w", err)
	}
	if seeded == 0 {
		lo.Warn("contacts.name permission not seeded yet, did nothing")
		return nil
	}

	res := db.Exec(`
		INSERT INTO role_permissions (custom_role_id, permission_id)
		SELECT DISTINCT r.id, target.id
		FROM custom_roles r
		JOIN role_permissions rp ON rp.custom_role_id = r.id
		JOIN permissions src ON src.id = rp.permission_id
		CROSS JOIN permissions target
		WHERE r.deleted_at IS NULL
		  AND target.resource = ? AND target.action = ?
		  AND (
		    (src.resource = ? AND src.action = ?)
		    OR (src.resource = ? AND src.action = ?)
		  )
		  AND NOT EXISTS (
		    SELECT 1
		    FROM custom_roles r2
		    JOIN role_permissions rp2 ON rp2.custom_role_id = r2.id
		    JOIN permissions p2 ON p2.id = rp2.permission_id
		    WHERE r2.organization_id = r.organization_id
		      AND r2.deleted_at IS NULL
		      AND p2.resource = ?
		  )
		ON CONFLICT DO NOTHING`,
		models.ResourceContactName, models.ActionWrite,
		models.ResourceContacts, models.ActionWrite,
		models.ResourceChat, models.ActionWrite,
		models.ResourceContactName,
	)
	if res.Error != nil {
		return fmt.Errorf("failed to grant the contact name permission: %w", res.Error)
	}

	if res.RowsAffected == 0 {
		lo.Info("contact name backfill: nothing pending")
		return nil
	}
	lo.Info("contact name backfill complete", "links_granted", res.RowsAffected)
	return nil
}
```

O `SELECT DISTINCT` importa: um papel que tenha **as duas** origens casaria duas vezes e tentaria inserir o mesmo vínculo em duplicidade dentro da mesma sentença, que o `ON CONFLICT` não cobre.

- [ ] **Step 10: Ligue no boot**

Em `cmd/whatomate/main.go`, dentro do bloco `if *migrate`, logo depois da chamada de `BackfillOccurrencePermissions`:

```go
		// Mesma janela: roda antes do ListenAndServe, então nenhuma requisição
		// chega antes de os papéis estarem corrigidos.
		if err := database.BackfillContactNamePermission(db, lo); err != nil {
			lo.Fatal("Contact name permission backfill failed", "error", err)
		}
```

- [ ] **Step 11: Rode tudo e faça o teste de mutação da guarda**

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc" && TEST_DATABASE_URL='postgres://postgres:postgres@127.0.0.1:5432/whatomate_test?sslmode=disable' TEST_REDIS_URL='redis://127.0.0.1:6379/1' go test ./internal/database/ -run 'TestBackfill' -count=1
```

Esperado: `ok`, com os do CRM e os novos.

Remova a cláusula `AND NOT EXISTS (...)` da guarda por organização e rode:

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc" && TEST_DATABASE_URL='postgres://postgres:postgres@127.0.0.1:5432/whatomate_test?sslmode=disable' TEST_REDIS_URL='redis://127.0.0.1:6379/1' go test ./internal/database/ -run 'TestBackfillContactName_SkipsOrganisationAlreadyMigrated' -count=1
```

Esperado: **FALHA**. Restaure e confirme `git diff internal/database/permissions_backfill.go` limpo.

- [ ] **Step 12: Build, vet e commit**

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc" && go build ./... && go vet ./... && TEST_DATABASE_URL='postgres://postgres:postgres@127.0.0.1:5432/whatomate_test?sslmode=disable' TEST_REDIS_URL='redis://127.0.0.1:6379/1' go test ./internal/handlers/ ./internal/models/ ./internal/database/ -p 1 -count=1
```

```bash
git add internal/ cmd/ && git commit -m "feat(contacts): let agents rename a contact without granting the whole record"
```

---

## Task 2: Copiar e renomear no painel de informações do contato

**Files:**
- Modify: `frontend/src/services/api.ts`
- Modify: `frontend/src/components/chat/ContactInfoPanel.vue`
- Modify: `frontend/src/i18n/locales/en.json` e `pt-BR.json`

**Interfaces:**
- Consumes: `PUT /api/contacts/{id}/name` com corpo `{ "name": "..." }`, da T1.
- Produces: nada.

### Contexto que você precisa

O painel mostra o nome em `ContactInfoPanel.vue:274` (`{{ contact.name || contact.phone_number }}`) e o telefone em `:278`. O componente recebe `contact: Contact` como prop e emite `close` e `tagsUpdated` (`:66-74`).

O padrão de copiar já existe na Fase 1, em `OccurrenceDetailView.vue:214-218`: o componente `IconButton` de `@/components/shared`, com `:icon="Copy"` e `:label`, mais `toast.success(t('common.copiedToClipboard'))`. A chave `common.copiedToClipboard` **existe nos dois locales** — reaproveite, não crie.

O gate de permissão do painel já é feito assim: `authStore.hasPermission('contacts', 'write')` em `:89`.

- [ ] **Step 1: Escreva o teste E2E que falha**

Crie `frontend/e2e/tests/chat/contact-info.spec.ts`:

```ts
import { test, expect, request as playwrightRequest } from '@playwright/test'
import { loginAsAdmin } from '../../helpers'
import { ApiHelper } from '../../helpers/api'
import { ChatPage } from '../../pages'
import { createTestScope } from '../../framework'

const scope = createTestScope('contact-info')

test.describe('Contact info panel', () => {
  let contactId: string
  let phone: string

  test.beforeAll(async () => {
    const ctx = await playwrightRequest.newContext()
    const api = new ApiHelper(ctx)
    await api.loginAsAdmin()
    phone = scope.phone()
    const contact = await api.createContact(phone, scope.name('info'))
    contactId = contact.id
    await ctx.dispose()
  })

  test('copies the phone number and renames the contact', async ({ page, context }) => {
    await context.grantPermissions(['clipboard-read', 'clipboard-write'])
    await loginAsAdmin(page)

    const chat = new ChatPage(page)
    await chat.goto(contactId)
    await chat.openContactInfo()

    const shown = (await page.locator('#contact-info-phone').innerText()).trim()
    await page.locator('#contact-info-copy-phone').click()
    const clipboard = await page.evaluate(() => navigator.clipboard.readText())
    expect(clipboard).toBe(shown)

    const novo = scope.name('renomeado')
    await page.locator('#contact-info-edit-name').click()
    await page.locator('#contact-info-name-input').fill(novo)
    await page.locator('#contact-info-save-name').click()

    await expect(page.locator('#contact-info-name')).toHaveText(novo)
  })
})
```

Os métodos usados são os reais do page object: `goto(contactId)` (`ChatPage.ts:35`) leva direto à conversa daquele contato, e `openContactInfo()` (`:155`) abre o painel. **Não existe `openFirstConversation`** — não invente. O `beforeAll` que cria o contato pela API e guarda o id segue o padrão de `frontend/e2e/tests/chat/conversation-notes.spec.ts:40-52`.

O prefixo de `scope` é o que a limpeza global usa para achar o que apagar depois.

- [ ] **Step 2: Rode e confirme que falha**

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc/frontend" && TEST_DATABASE_URL='postgres://postgres:postgres@127.0.0.1:5432/whatomate_e2e?sslmode=disable' BASE_URL=http://localhost:3000 npx playwright test e2e/tests/chat/contact-info.spec.ts --workers=1
```

Esperado: falha por não encontrar `#contact-info-copy-phone`.

- [ ] **Step 3: Acrescente o método no serviço**

Em `frontend/src/services/api.ts`, no `contactsService`, junto de `updateTags`:

```ts
  updateName: (id: string, name: string) =>
    api.put(`/contacts/${id}/name`, { name }),
```

- [ ] **Step 4: Acrescente as chaves de i18n**

Em `en.json`, no bloco `"contacts"`:

```json
    "copyName": "Copy name",
    "copyPhone": "Copy phone number",
    "editName": "Edit name",
    "saveName": "Save name",
    "cancelEdit": "Cancel",
    "nameUpdated": "Contact renamed",
```

Em `pt-BR.json`, no mesmo bloco:

```json
    "copyName": "Copiar nome",
    "copyPhone": "Copiar telefone",
    "editName": "Editar nome",
    "saveName": "Salvar nome",
    "cancelEdit": "Cancelar",
    "nameUpdated": "Contato renomeado",
```

- [ ] **Step 5: Implemente no painel**

Em `ContactInfoPanel.vue`, no bloco de script, acrescente ao que já existe:

```ts
import { Copy, Pencil, Check, X } from 'lucide-vue-next'
import { IconButton } from '@/components/shared'
import { contactsService } from '@/services/api'
import { toast } from 'vue-sonner'
import { getErrorMessage } from '@/lib/api-utils'

const canRenameContact = computed(() => authStore.hasPermission('contacts.name', 'write'))

const isEditingName = ref(false)
const nameDraft = ref('')
const isSavingName = ref(false)

const displayName = computed(() => props.contact.name || props.contact.phone_number)

function startEditName() {
  nameDraft.value = props.contact.name || ''
  isEditingName.value = true
}

function copyText(value: string) {
  navigator.clipboard.writeText(value)
  toast.success(t('common.copiedToClipboard'))
}

async function saveName() {
  const novo = nameDraft.value.trim()
  if (!novo) return
  isSavingName.value = true
  try {
    await contactsService.updateName(props.contact.id, novo)
    emit('nameUpdated', novo)
    isEditingName.value = false
    toast.success(t('contacts.nameUpdated'))
  } catch (e) {
    toast.error(getErrorMessage(e, t('common.failedSave', { resource: t('resources.contact') })))
  } finally {
    isSavingName.value = false
  }
}
```

`startEditName` preenche a partir de `contact.name`, **não** de `displayName`: `displayName` cai para o telefone quando não há nome, e pré-preencher o campo com o telefone convidaria o atendente a salvá-lo como nome. Com a máscara ligada esse telefone ainda viria como `****1234`, que o backend recusa — mas a interface não deve chegar lá.

Acrescente `nameUpdated` aos emits existentes:

```ts
const emit = defineEmits<{
  close: []
  tagsUpdated: [tags: string[]]
  nameUpdated: [name: string]
}>()
```

Confirme que `computed`, `ref` e o `t` do `useI18n` já estão no arquivo; use os que existem em vez de reimportar.

No template, troque o bloco do cabeçalho (`:273-280`) por:

```vue
          <div v-if="!isEditingName" class="flex items-center gap-1">
            <h4 id="contact-info-name" class="font-medium">{{ displayName }}</h4>
            <IconButton
              id="contact-info-copy-name"
              :icon="Copy"
              :label="$t('contacts.copyName')"
              class="h-6 w-6"
              @click="copyText(displayName)"
            />
            <IconButton
              v-if="canRenameContact"
              id="contact-info-edit-name"
              :icon="Pencil"
              :label="$t('contacts.editName')"
              class="h-6 w-6"
              @click="startEditName"
            />
          </div>
          <div v-else class="flex items-center gap-1">
            <Input
              id="contact-info-name-input"
              v-model="nameDraft"
              class="h-8 w-44"
              :disabled="isSavingName"
              @keyup.enter="saveName"
            />
            <IconButton
              id="contact-info-save-name"
              :icon="Check"
              :label="$t('contacts.saveName')"
              class="h-6 w-6"
              :disabled="isSavingName"
              @click="saveName"
            />
            <IconButton
              :icon="X"
              :label="$t('contacts.cancelEdit')"
              class="h-6 w-6"
              @click="isEditingName = false"
            />
          </div>
          <div class="flex items-center gap-1 text-sm text-muted-foreground mt-1">
            <Phone class="h-3 w-3" />
            <span id="contact-info-phone">{{ contact.phone_number }}</span>
            <IconButton
              id="contact-info-copy-phone"
              :icon="Copy"
              :label="$t('contacts.copyPhone')"
              class="h-6 w-6"
              @click="copyText(contact.phone_number)"
            />
          </div>
```

Se `Input` ainda não estiver importado no arquivo, importe de `@/components/ui/input`.

- [ ] **Step 6: Faça o pai reagir ao nome novo**

`ContactInfoPanel` recebe o contato como prop e não pode alterá-lo. A chamada à API já aconteceu no painel; falta a store refletir localmente, exatamente como faz com as tags.

Em `frontend/src/stores/contacts.ts`, ao lado de `updateContactTags` (`:404-414`), acrescente a gêmea:

```ts
  function updateContactName(contactId: string, name: string) {
    const contact = contacts.value.find(c => c.id === contactId)
    if (contact) {
      contact.name = name
      contact.profile_name = name
    }
    if (currentContact.value?.id === contactId) {
      currentContact.value = { ...currentContact.value, name, profile_name: name }
    }
  }
```

Os **dois** campos são atualizados porque a API devolve o mesmo valor em `name` e `profile_name`, e partes diferentes da interface leem um ou outro — deixar só um sincronizado produz um nome que muda no painel e não muda na lista.

Exponha `updateContactName` no retorno da store, junto de `updateContactTags`.

Em `ChatView.vue:2727-2732`, acrescente o ouvinte ao lado do que já existe, seguindo a mesma forma inline:

```vue
      @name-updated="(name) => contactsStore.updateContactName(contactsStore.currentContact!.id, name)"

- [ ] **Step 7: Rode e confirme que passa**

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc/frontend" && TEST_DATABASE_URL='postgres://postgres:postgres@127.0.0.1:5432/whatomate_e2e?sslmode=disable' BASE_URL=http://localhost:3000 npx playwright test e2e/tests/chat/ --workers=1
```

Esperado: passa, e os testes de chat que já existiam continuam passando.

- [ ] **Step 8: Verifique e commite**

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc/frontend" && npm run typecheck && npm run i18n:keys && npx eslint src/components/chat/ContactInfoPanel.vue src/services/api.ts && npm run build
```

```bash
git add frontend/src frontend/e2e && git commit -m "feat(contacts): copy and rename from the contact info panel"
```

---

## Task 3: Tooltips no menu recolhido e rótulos no card

**Files:**
- Modify: `frontend/src/components/layout/AppLayout.vue`
- Modify: `frontend/src/components/crm/OccurrenceCard.vue`
- Modify: `frontend/src/i18n/locales/en.json` e `pt-BR.json`

**Interfaces:**
- Consumes: nada.
- Produces: nada.

### Contexto que você precisa

`AppLayout.vue` já tem `isCollapsed` (`:28`) e encolhe a barra para `md:w-16` (`:145`). O item de navegação é um `RouterLink` (`:196-213`) cujo rótulo já vira `md:sr-only` quando recolhido — o leitor de tela recebe o nome, o usuário que enxerga não.

O `TooltipProvider` **já envolve a aplicação inteira** em `App.vue:12`, então basta usar `Tooltip`/`TooltipTrigger`/`TooltipContent` aqui. Nenhum provider novo.

Nenhuma string nova para os tooltips: reaproveite `$t(item.name)`, que é o mesmo rótulo do menu expandido.

- [ ] **Step 1: Escreva o teste E2E que falha**

Crie `frontend/e2e/tests/layout/sidebar-tooltips.spec.ts`:

```ts
import { test, expect } from '@playwright/test'
import { loginAsAdmin } from '../../helpers'

test.describe('Collapsed sidebar', () => {
  test('shows the menu name on hover when collapsed', async ({ page }) => {
    await loginAsAdmin(page)

    // Recolhe a barra pelo próprio controle da interface.
    await page.getByRole('button', { name: /collapse sidebar|recolher/i }).click()

    const chatLink = page.getByRole('menuitem').filter({ hasText: '' }).first()
    await chatLink.hover()

    await expect(page.getByRole('tooltip')).toBeVisible()
  })

  test('does not show tooltips while expanded', async ({ page }) => {
    await loginAsAdmin(page)

    const chatLink = page.getByRole('menuitem').first()
    await chatLink.hover()

    await expect(page.getByRole('tooltip')).toHaveCount(0)
  })
})
```

O rótulo acessível do botão que recolhe vem de `$t('nav.collapseSidebar')` (`AppLayout.vue:167`), que vale **"Collapse sidebar"** em `en` e **"Recolher barra lateral"** em `pt-BR` — verificado nos dois arquivos. A expressão regular acima cobre os dois, então o teste não fica preso ao locale em que o navegador subir.

O **segundo teste é o que importa**: sem ele, a correção passaria igual se o tooltip ficasse sempre montado, aparecendo por cima de um rótulo que já está visível.

- [ ] **Step 2: Rode e confirme que falha**

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc/frontend" && TEST_DATABASE_URL='postgres://postgres:postgres@127.0.0.1:5432/whatomate_e2e?sslmode=disable' BASE_URL=http://localhost:3000 npx playwright test e2e/tests/layout/sidebar-tooltips.spec.ts --workers=1
```

Esperado: o primeiro falha por não haver tooltip; o segundo passa desde já.

- [ ] **Step 3: Envolva o item de navegação**

Em `AppLayout.vue`, importe do `@/components/ui/tooltip`:

```ts
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
```

E envolva o `RouterLink` do item, mantendo o link exatamente como está por dentro:

```vue
              <template v-for="item in section.items" :key="item.path">
                <Tooltip v-if="isCollapsed" :delay-duration="150">
                  <TooltipTrigger as-child>
                    <RouterLink ... >
                      ... conteúdo atual, sem alteração ...
                    </RouterLink>
                  </TooltipTrigger>
                  <TooltipContent side="right">{{ $t(item.name) }}</TooltipContent>
                </Tooltip>
                <RouterLink v-else ... >
                  ... conteúdo atual, sem alteração ...
                </RouterLink>
```

Duplicar o `RouterLink` em dois ramos é feio. Se preferir, extraia o conteúdo do link para um `<template>` reutilizável ou um componente pequeno no mesmo arquivo — o que **não** pode é o tooltip ficar montado quando a barra está expandida, porque aí ele aparece por cima do rótulo que já está visível. O segundo teste do Step 1 guarda exatamente isso.

- [ ] **Step 4: Rode e confirme que os dois passam**

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc/frontend" && TEST_DATABASE_URL='postgres://postgres:postgres@127.0.0.1:5432/whatomate_e2e?sslmode=disable' BASE_URL=http://localhost:3000 npx playwright test e2e/tests/layout/ --workers=1
```

- [ ] **Step 5: Acrescente as chaves dos rótulos do card**

Em `en.json`, no bloco `"occurrences"`:

```json
    "cardContact": "Contact",
    "cardAssignee": "Assignee",
```

Em `pt-BR.json`:

```json
    "cardContact": "Contato",
    "cardAssignee": "Responsável",
```

- [ ] **Step 6: Rotule as duas linhas do card**

Em `frontend/src/components/crm/OccurrenceCard.vue`, troque as duas linhas de contato e responsável por:

```vue
    <p class="text-xs mt-1 truncate text-white/50 light:text-muted-foreground">
      <span class="text-white/30 light:text-gray-400">{{ $t('occurrences.cardContact') }}:</span>
      {{ occurrence.contact_name }}
    </p>
    <p class="text-xs mt-0.5 truncate text-white/40 light:text-muted-foreground">
      <span class="text-white/30 light:text-gray-400">{{ $t('occurrences.cardAssignee') }}:</span>
      {{ occurrence.assigned_user_name || $t('occurrences.unassigned') }}
    </p>
```

O rótulo fica mais fraco que o valor de propósito: quem lê o quadro busca o nome, não a palavra "Contato".

- [ ] **Step 7: Confirme que o quadro não quebrou**

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc/frontend" && TEST_DATABASE_URL='postgres://postgres:postgres@127.0.0.1:5432/whatomate_e2e?sslmode=disable' BASE_URL=http://localhost:3000 npx playwright test e2e/tests/crm/ --workers=1
```

Esperado: os 23 testes de CRM continuam passando. Se algum casar texto do card por igualdade exata, ele vai reclamar do rótulo novo — nesse caso ajuste o teste para casar o **nome**, que é o que ele quer provar.

- [ ] **Step 8: Verifique e commite**

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc/frontend" && npm run typecheck && npm run i18n:keys && npx eslint src/components/layout/AppLayout.vue src/components/crm/OccurrenceCard.vue && npm run build
```

```bash
git add frontend/src frontend/e2e && git commit -m "feat(ui): name the icons in the collapsed sidebar and label the board card"
```

---

## Verificação final

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc" && go build ./... && go vet ./... && TEST_DATABASE_URL='postgres://postgres:postgres@127.0.0.1:5432/whatomate_test?sslmode=disable' TEST_REDIS_URL='redis://127.0.0.1:6379/1' go test ./internal/handlers/ ./internal/models/ ./internal/database/ -p 1 -count=1
```

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc/frontend" && npm run typecheck && npm run i18n:keys && npm run build && TEST_DATABASE_URL='postgres://postgres:postgres@127.0.0.1:5432/whatomate_e2e?sslmode=disable' BASE_URL=http://localhost:3000 npx playwright test e2e/tests/ --workers=1
```

## Pré-condição de deploy

A mesma da permissão do CRM, e pela mesma razão: o contêiner de produção precisa subir com `-migrate`, senão `BackfillContactNamePermission` não roda e ninguém ganha a permissão nova. Já verificado em produção (`docker-compose.yml:23` traz `-migrate`), então nada novo a checar — mas se essa linha mudar, esta funcionalidade some junto com a do CRM.

## Riscos

| Risco | Mitigação |
|---|---|
| Salvar o nome mascarado por cima do telefone real | Recusa de `*` no handler, com teste; e a interface pré-preenche de `contact.name`, nunca do valor exibido |
| Renomear virar furo na visibilidade | `canInteractWithConversation` no handler, com teste de mutação (T1, Step 7) |
| Endpoint estreito virar largo | Corpo de um campo só; qualquer campo novo exige decisão explícita |
| Tooltip aparecer com a barra expandida | `v-if="isCollapsed"` e o segundo teste do T3 Step 1 |
| Papel com as duas origens duplicar vínculo | `SELECT DISTINCT` no backfill |
