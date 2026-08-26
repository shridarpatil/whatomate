# CRM de ocorrências com protocolo — Plano de Implementação (Fase 1)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Entregar o núcleo do CRM — ocorrência por contato com protocolo gerado, etapas configuráveis, timeline e criação a partir da conversa — sem alterar comportamento de nada que já roda em produção.

**Architecture:** Quatro tabelas novas registradas no AutoMigrate existente. Handlers seguem o molde de `conversation_notes.go`: `requireAuth` com `chat:read`/`chat:write`, depois o gate de conversa (`canViewConversation` / `canInteractWithConversation`). Listagens escopam por subconsulta de contatos visíveis, nunca por join. Frontend com store Pinia, painel irmão do de notas no chat, e telas de lista/detalhe/configuração.

**Tech Stack:** Go 1.25 + fastglue + GORM + Postgres · Vue 3 `<script setup>` + TypeScript + Pinia + Tailwind + reka-ui · Playwright.

**Spec:** [`docs/superpowers/specs/2026-08-15-crm-ocorrencias-protocolo-design.md`](../specs/2026-08-15-crm-ocorrencias-protocolo-design.md) (commit `d80c428`)

## Global Constraints

- **Branch:** `feature/crm-ocorrencias-protocolo`, saída de `development`. Nunca commitar em `main` nem em `development`.
- **Nada de produção muda.** Nenhuma coluna adicionada a `contacts`, `agent_transfers` ou `messages`. Não tocar no fluxo do chatbot, no `sla_processor.go`, no ciclo de transferência, no painel de notas nem no `SendOutgoingMessage`.
- **Nenhuma permissão nova.** Handlers usam `models.ResourceChat` com `models.ActionRead` / `models.ActionWrite`. Configuração de etapas usa `models.ResourceSettingsGeneral` + `models.ActionWrite`.
- **Sem Kanban e sem SLA** nesta fase. Não embutir `SLATracking` ainda. Sem WebSocket.
- **Sem refatoração fora de escopo.** Não reorganizar arquivos existentes.
- **Formato do protocolo:** `2026-000123` — `YYYY-NNNNNN`, sequencial por organização, reiniciando por ano.
- **Etapas padrão semeadas:** `Aberto` (is_initial) → `Em análise` → `Aguardando cliente` → `Resolvido` (is_closing).
- **i18n:** toda string visível entra em `frontend/src/i18n/locales/pt-BR.json` **e** `en.json`, mantidos paralelos linha a linha.
- **E2E rodam em inglês**, então seletores usam os rótulos de `en.json`.

### Comandos de verificação

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc" && go build ./... && go vet ./...
```

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc" && TEST_DATABASE_URL='postgres://postgres:postgres@127.0.0.1:5432/whatomate_test?sslmode=disable' go test ./internal/handlers/ -run Occurrence -v
```

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc/frontend" && npm run typecheck && npm run i18n:keys
```

**Atenção sobre os testes Go:** sem `TEST_DATABASE_URL` os testes de banco **pulam em silêncio** e a suíte reporta `ok`. Um `ok` sem a variável não prova nada. Postgres roda em Docker (`whatc-pg`), usuário `postgres`, senha `postgres`.

**Atenção sobre `npm run lint`:** está configurado com `--fix` e reescreve arquivos do repositório inteiro. Use `npx eslint <arquivos>` sem `--fix`.

**Erro pré-existente conhecido no typecheck:** `AccountDetailView.vue(172,45) business_calling_enabled`. É o único aceitável.

---

## File Structure

| Arquivo | Responsabilidade | Ação |
|---|---|---|
| `internal/models/occurrences.go` | As quatro entidades | Criar |
| `internal/database/postgres.go` | Registro no AutoMigrate | Modificar |
| `internal/handlers/occurrence_protocol.go` | Numeração atômica e semeadura de etapas | Criar |
| `internal/handlers/occurrence_stages.go` | CRUD de etapas + regras de integridade | Criar |
| `internal/handlers/occurrences.go` | CRUD de ocorrência, etapa, timeline | Criar |
| `internal/handlers/occurrence_send.go` | Envio do protocolo + janela de 24h | Criar |
| `cmd/whatomate/main.go` | Rotas | Modificar |
| `frontend/src/services/api.ts` | Tipos e cliente | Modificar |
| `frontend/src/stores/occurrences.ts` | Estado | Criar |
| `frontend/src/components/chat/ContactOccurrencesPanel.vue` | Painel no chat | Criar |
| `frontend/src/views/chat/ChatView.vue` | Monta o painel | Modificar |
| `frontend/src/views/crm/OccurrencesView.vue` | Lista | Criar |
| `frontend/src/views/crm/OccurrenceDetailView.vue` | Detalhe + timeline | Criar |
| `frontend/src/views/settings/OccurrenceStagesView.vue` | Configuração | Criar |
| `frontend/src/router/index.ts` | Rotas do frontend | Modificar |
| `frontend/src/i18n/locales/{pt-BR,en}.json` | Textos | Modificar |
| `frontend/e2e/pages/OccurrencesPage.ts` | Page object | Criar |
| `frontend/e2e/tests/crm/occurrences.spec.ts` | E2E | Criar |

## Ordem de dependência

```
T1 modelos + numeração (GATE 1)
 └─> T2 etapas + integridade (GATE 7)
      └─> T3 criar/listar + autorização (GATES 2, 3)
           ├─> T4 detalhe/etapa/eventos (GATES 4, 6)
           │    └─> T5 envio de protocolo (GATE 5)
           └─> T6 cliente de API + store
                ├─> T7 painel no chat
                ├─> T8 lista + detalhe
                └─> T9 configuração de etapas
                     └─> T10 E2E
                          └─> T11 verificação final
```

T2 depende de T1 porque precisa do modelo `OccurrenceStage`. T3 depende de T2 porque uma ocorrência nasce numa etapa inicial que precisa existir. T5 depende de T4 porque o envio grava um evento na timeline. Todo o frontend depende de T3 (contrato de API estável). T10 depende de todo o frontend.

## Os sete gates obrigatórios

| # | Gate | Onde é testado |
|---|---|---|
| 1 | Concorrência da geração do protocolo | T1 |
| 2 | Autorização positiva e negativa | T3 |
| 3 | Exceção de visibilidade do responsável | T3 |
| 4 | Autorização em detalhe, eventos, etapa e envio | T4, T5 |
| 5 | Janela de 24h no backend | T5 |
| 6 | Fechamento e reabertura | T4 |
| 7 | Integridade das etapas | T2 |

---

## Task 1: Modelos, migração e numeração atômica do protocolo

**Contexto:** Esta é a tarefa de maior risco técnico do plano. A numeração precisa ser atômica sob concorrência — duas ocorrências abertas no mesmo instante não podem receber o mesmo protocolo. `COUNT(*) + 1` falha exatamente quando o movimento aumenta.

**Files:**
- Create: `internal/models/occurrences.go`
- Modify: `internal/database/postgres.go` (função `GetMigrationModels`, no fim da lista, antes de `AuditLog`)
- Create: `internal/handlers/occurrence_protocol.go`
- Test: `internal/handlers/occurrence_protocol_test.go`

**Interfaces:**
- Consumes: `models.BaseModel`, `testutil.SetupTestDB`.
- Produces:
  - `models.Occurrence`, `models.OccurrenceStage`, `models.OccurrenceEvent`, `models.OccurrenceCounter`
  - `models.OccurrenceEventType` com as constantes `OccurrenceEventOpened`, `OccurrenceEventNote`, `OccurrenceEventStageChange`, `OccurrenceEventAssignment`, `OccurrenceEventProtocolSent`, `OccurrenceEventClosed`
  - `func (a *App) nextProtocolNumber(tx *gorm.DB, orgID uuid.UUID, year int) (string, error)`
  - `func (a *App) ensureDefaultStages(orgID uuid.UUID) error`
  - `func (a *App) initialStage(orgID uuid.UUID) (*models.OccurrenceStage, error)`

- [ ] **Step 1: Criar os modelos**

Criar `internal/models/occurrences.go`:

```go
package models

import (
	"time"

	"github.com/google/uuid"
)

// OccurrenceEventType identifies what a timeline entry records.
type OccurrenceEventType string

const (
	OccurrenceEventOpened       OccurrenceEventType = "opened"
	OccurrenceEventNote         OccurrenceEventType = "note"
	OccurrenceEventStageChange  OccurrenceEventType = "stage_change"
	OccurrenceEventAssignment   OccurrenceEventType = "assignment"
	OccurrenceEventProtocolSent OccurrenceEventType = "protocol_sent"
	OccurrenceEventClosed       OccurrenceEventType = "closed"
)

// OccurrencePriority ranks how urgent a case is.
type OccurrencePriority string

const (
	OccurrencePriorityLow    OccurrencePriority = "low"
	OccurrencePriorityNormal OccurrencePriority = "normal"
	OccurrencePriorityHigh   OccurrencePriority = "high"
	OccurrencePriorityUrgent OccurrencePriority = "urgent"
)

// OccurrenceStage is one configurable column of the org's pipeline.
type OccurrenceStage struct {
	BaseModel
	OrganizationID uuid.UUID `gorm:"type:uuid;index;not null" json:"organization_id"`
	Name           string    `gorm:"size:100;not null" json:"name"`
	Color          string    `gorm:"size:20;default:'#6b7280'" json:"color"`
	Position       int       `gorm:"not null;default:0" json:"position"`
	IsInitial      bool      `gorm:"default:false" json:"is_initial"`
	IsClosing      bool      `gorm:"default:false" json:"is_closing"`
}

func (OccurrenceStage) TableName() string { return "occurrence_stages" }

// Occurrence is one tracked case for a contact. Its ProtocolNumber is the
// human-readable identifier the customer keeps and quotes back.
type Occurrence struct {
	BaseModel
	OrganizationID uuid.UUID `gorm:"type:uuid;index;not null;uniqueIndex:idx_occ_org_protocol" json:"organization_id"`
	ContactID      uuid.UUID `gorm:"type:uuid;index;not null" json:"contact_id"`

	// Total unique index, deliberately NOT partial on soft delete: a deleted
	// protocol must never be reissued, because the customer still has the
	// number written down.
	ProtocolNumber string `gorm:"size:20;not null;uniqueIndex:idx_occ_org_protocol" json:"protocol_number"`

	Title       string             `gorm:"size:255;not null" json:"title"`
	Description string             `gorm:"type:text" json:"description"`
	StageID     uuid.UUID          `gorm:"type:uuid;index;not null" json:"stage_id"`
	Priority    OccurrencePriority `gorm:"size:20;not null;default:'normal'" json:"priority"`

	AssignedUserID *uuid.UUID `gorm:"type:uuid;index" json:"assigned_user_id,omitempty"`
	TeamID         *uuid.UUID `gorm:"type:uuid;index" json:"team_id,omitempty"`
	OpenedByUserID uuid.UUID  `gorm:"type:uuid;not null" json:"opened_by_user_id"`

	OpenedAt time.Time  `gorm:"autoCreateTime" json:"opened_at"`
	ClosedAt *time.Time `json:"closed_at,omitempty"`

	// SourceTransferID records which attendance spawned this occurrence. It is
	// traceability only — the lifecycles stay independent, so closing the chat
	// attendance never closes the occurrence.
	SourceTransferID *uuid.UUID `gorm:"type:uuid;index" json:"source_transfer_id,omitempty"`

	// Relations
	Contact      *Contact         `gorm:"foreignKey:ContactID" json:"contact,omitempty"`
	Stage        *OccurrenceStage `gorm:"foreignKey:StageID" json:"stage,omitempty"`
	AssignedUser *User            `gorm:"foreignKey:AssignedUserID" json:"assigned_user,omitempty"`
}

func (Occurrence) TableName() string { return "occurrences" }

// OccurrenceEvent is one entry on an occurrence's timeline. Manual notes and
// automatic events share this table on purpose: two tables drift, and a
// timeline built from two queries has to interleave them anyway.
type OccurrenceEvent struct {
	BaseModel
	OrganizationID uuid.UUID           `gorm:"type:uuid;index;not null" json:"organization_id"`
	OccurrenceID   uuid.UUID           `gorm:"type:uuid;index;not null" json:"occurrence_id"`
	Type           OccurrenceEventType `gorm:"size:30;not null" json:"type"`
	Content        string              `gorm:"type:text" json:"content"`
	Metadata       JSONB               `gorm:"type:jsonb;default:'{}'" json:"metadata"`
	CreatedByID    *uuid.UUID          `gorm:"type:uuid" json:"created_by_id,omitempty"` // nil = system

	CreatedBy *User `gorm:"foreignKey:CreatedByID" json:"created_by,omitempty"`
}

func (OccurrenceEvent) TableName() string { return "occurrence_events" }

// OccurrenceCounter holds the per-org, per-year protocol sequence. It does not
// embed BaseModel: the natural key is (organization_id, year), and a uuid PK
// would invite a second row for the same pair.
type OccurrenceCounter struct {
	OrganizationID uuid.UUID `gorm:"type:uuid;primaryKey" json:"organization_id"`
	Year           int       `gorm:"primaryKey" json:"year"`
	LastSeq        int       `gorm:"not null;default:0" json:"last_seq"`
}

func (OccurrenceCounter) TableName() string { return "occurrence_counters" }
```

- [ ] **Step 2: Registrar no AutoMigrate**

Em `internal/database/postgres.go`, dentro de `GetMigrationModels()`, imediatamente antes da linha `{"AuditLog", &models.AuditLog{}},`, inserir:

```go
		// CRM de ocorrências
		{"OccurrenceStage", &models.OccurrenceStage{}},
		{"Occurrence", &models.Occurrence{}},
		{"OccurrenceEvent", &models.OccurrenceEvent{}},
		{"OccurrenceCounter", &models.OccurrenceCounter{}},
```

A ordem importa: `OccurrenceStage` antes de `Occurrence`, porque `Occurrence.StageID` referencia a primeira.

- [ ] **Step 3: Escrever o teste de concorrência (GATE 1)**

Criar `internal/handlers/occurrence_protocol_test.go`:

```go
package handlers_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// GATE 1. Protocol numbering must be atomic. COUNT(*)+1 passes a serial test
// and collides in production exactly when volume rises, so the test that
// matters is the concurrent one.
func TestOccurrenceProtocol_UniqueUnderConcurrency(t *testing.T) {
	app := testutil.SetupTestApp(t)
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

// A virada de ano começa em 000001, não continua a sequência do ano anterior.
func TestOccurrenceProtocol_ResetsOnNewYear(t *testing.T) {
	app := testutil.SetupTestApp(t)
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
	app := testutil.SetupTestApp(t)
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
```

- [ ] **Step 4: Rodar o teste e confirmar que falha**

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc" && TEST_DATABASE_URL='postgres://postgres:postgres@127.0.0.1:5432/whatomate_test?sslmode=disable' go test ./internal/handlers/ -run TestOccurrenceProtocol -v
```

Esperado: **falha de compilação** — `app.EnsureDefaultStagesForTest`, `app.InitialStageForTest`, `app.CreateOccurrenceForTest` e `app.NextProtocolNumberForTest` não existem.

- [ ] **Step 5: Implementar a numeração e a semeadura**

Criar `internal/handlers/occurrence_protocol.go`:

```go
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
```

- [ ] **Step 6: Expor os hooks de teste**

Criar `internal/handlers/occurrence_export_test.go`:

```go
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
```

Se `testutil.SetupTestApp`, `testutil.CreateTestOrganization` ou `testutil.CreateTestUser` não existirem com essas assinaturas, leia `test/testutil/fixtures.go` e use os nomes reais — **não crie helpers novos**.

- [ ] **Step 7: Rodar o teste e confirmar que passa**

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc" && TEST_DATABASE_URL='postgres://postgres:postgres@127.0.0.1:5432/whatomate_test?sslmode=disable' go test ./internal/handlers/ -run TestOccurrenceProtocol -v
```

Esperado: 3 testes passando.

- [ ] **Step 8: Teste de mutação — provar que o teste de concorrência detecta a regressão**

Trocar temporariamente o corpo de `nextProtocolNumber` pela versão ingênua:

```go
	var seq int64
	tx.Model(&models.Occurrence{}).Where("organization_id = ?", orgID).Count(&seq)
	return fmt.Sprintf("%d-%06d", year, seq+1), nil
```

Rodar o mesmo comando do Step 7. Esperado: `TestOccurrenceProtocol_UniqueUnderConcurrency` **falha** com protocolo duplicado. Reverter em seguida e confirmar que volta a passar. Registrar no relatório que a mutação foi feita e revertida — um teste que passa com a lógica removida não é teste.

- [ ] **Step 9: Build, vet e commit**

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc" && go build ./... && go vet ./...
```

```bash
git add internal/models/occurrences.go internal/database/postgres.go internal/handlers/occurrence_protocol.go internal/handlers/occurrence_protocol_test.go internal/handlers/occurrence_export_test.go
git commit -m "feat(crm): occurrence models and atomic protocol numbering"
```

---

## Task 2: Etapas — CRUD e regras de integridade (GATE 7)

**Contexto:** As etapas são a coluna vertebral do pipeline. Sem as regras de integridade, o primeiro `DELETE` numa etapa em uso deixa ocorrências apontando para nada, e remover a última etapa de fechamento torna impossível concluir qualquer caso.

**Files:**
- Create: `internal/handlers/occurrence_stages.go`
- Modify: `cmd/whatomate/main.go` (junto ao bloco de rotas de Conversation Notes, `/api/contacts/{id}/notes`)
- Test: `internal/handlers/occurrence_stages_test.go`

**Interfaces:**
- Consumes: `models.OccurrenceStage`, `a.ensureDefaultStages` (T1).
- Produces: rotas `/api/occurrence-stages`; `OccurrenceStageResponse` com campos `id`, `name`, `color`, `position`, `is_initial`, `is_closing`.

**Regras de integridade a implementar:**

1. Excluir etapa referenciada por alguma ocorrência → **409**.
2. Excluir a última etapa com `is_closing` → **409**.
3. Excluir a etapa `is_initial` → **409**.
4. Marcar uma etapa como `is_initial` desmarca a anterior (exatamente uma por organização).
5. Desmarcar `is_closing` da última etapa de fechamento → **409**.

- [ ] **Step 1: Escrever os testes de integridade (GATE 7)**

Criar `internal/handlers/occurrence_stages_test.go`:

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

// GATE 7. Uma etapa em uso não pode ser excluída — senão a ocorrência aponta
// para uma linha que não existe mais.
func TestOccurrenceStages_DeleteInUseIsRejected(t *testing.T) {
	app := testutil.SetupTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	contact := testutil.CreateTestContact(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	require.NoError(t, app.EnsureDefaultStagesForTest(org.ID))
	stage, err := app.InitialStageForTest(org.ID)
	require.NoError(t, err)

	occ := models.Occurrence{
		OrganizationID: org.ID, ContactID: contact.ID, Title: "Em uso",
		StageID: stage.ID, OpenedByUserID: user.ID,
	}
	require.NoError(t, app.CreateOccurrenceForTest(&occ))

	req := testutil.NewAuthedRequest(t, app, user, org, "DELETE",
		"/api/occurrence-stages/"+stage.ID.String())
	testutil.SetPathParam(req, "id", stage.ID.String())

	require.NoError(t, app.DeleteOccurrenceStage(req))
	assert.Equal(t, fasthttp.StatusConflict, req.RequestCtx.Response.StatusCode())

	var count int64
	app.DB.Model(&models.OccurrenceStage{}).Where("id = ?", stage.ID).Count(&count)
	assert.EqualValues(t, 1, count, "a etapa não pode ter sido apagada")
}

// Sem etapa de fechamento não existe como concluir uma ocorrência.
func TestOccurrenceStages_DeleteLastClosingIsRejected(t *testing.T) {
	app := testutil.SetupTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	require.NoError(t, app.EnsureDefaultStagesForTest(org.ID))

	var closing models.OccurrenceStage
	require.NoError(t, app.DB.Where("organization_id = ? AND is_closing = ?", org.ID, true).
		First(&closing).Error)

	req := testutil.NewAuthedRequest(t, app, user, org, "DELETE",
		"/api/occurrence-stages/"+closing.ID.String())
	testutil.SetPathParam(req, "id", closing.ID.String())

	require.NoError(t, app.DeleteOccurrenceStage(req))
	assert.Equal(t, fasthttp.StatusConflict, req.RequestCtx.Response.StatusCode())
}

// Exatamente uma etapa inicial por organização.
func TestOccurrenceStages_SettingInitialUnsetsPrevious(t *testing.T) {
	app := testutil.SetupTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	require.NoError(t, app.EnsureDefaultStagesForTest(org.ID))

	var other models.OccurrenceStage
	require.NoError(t, app.DB.Where("organization_id = ? AND is_initial = ?", org.ID, false).
		Order("position ASC").First(&other).Error)

	req := testutil.NewAuthedRequestWithBody(t, app, user, org, "PUT",
		"/api/occurrence-stages/"+other.ID.String(),
		map[string]any{"name": other.Name, "color": other.Color, "is_initial": true})
	testutil.SetPathParam(req, "id", other.ID.String())

	require.NoError(t, app.UpdateOccurrenceStage(req))
	assert.Equal(t, fasthttp.StatusOK, req.RequestCtx.Response.StatusCode())

	var initialCount int64
	app.DB.Model(&models.OccurrenceStage{}).
		Where("organization_id = ? AND is_initial = ?", org.ID, true).Count(&initialCount)
	assert.EqualValues(t, 1, initialCount, "deve haver exatamente uma etapa inicial")
}
```

Se os helpers `testutil.NewAuthedRequest`, `NewAuthedRequestWithBody` ou `SetPathParam` não existirem com esses nomes, leia `test/testutil/http.go` e use os reais.

- [ ] **Step 2: Rodar e confirmar falha**

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc" && TEST_DATABASE_URL='postgres://postgres:postgres@127.0.0.1:5432/whatomate_test?sslmode=disable' go test ./internal/handlers/ -run TestOccurrenceStages -v
```

Esperado: falha de compilação — os handlers não existem.

- [ ] **Step 3: Implementar os handlers de etapa**

Criar `internal/handlers/occurrence_stages.go`:

```go
package handlers

import (
	"github.com/google/uuid"
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
```

- [ ] **Step 4: Registrar as rotas**

Em `cmd/whatomate/main.go`, logo depois do bloco de Conversation Notes (as quatro linhas `/api/contacts/{id}/notes`), inserir:

```go
	// CRM — etapas de ocorrência
	g.GET("/api/occurrence-stages", app.ListOccurrenceStages)
	g.POST("/api/occurrence-stages", app.CreateOccurrenceStage)
	g.PUT("/api/occurrence-stages/{id}", app.UpdateOccurrenceStage)
	g.DELETE("/api/occurrence-stages/{id}", app.DeleteOccurrenceStage)
```

- [ ] **Step 5: Rodar os testes e confirmar que passam**

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc" && TEST_DATABASE_URL='postgres://postgres:postgres@127.0.0.1:5432/whatomate_test?sslmode=disable' go test ./internal/handlers/ -run TestOccurrenceStages -v
```

Esperado: 3 testes passando.

- [ ] **Step 6: Build, vet e commit**

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc" && go build ./... && go vet ./...
```

```bash
git add internal/handlers/occurrence_stages.go internal/handlers/occurrence_stages_test.go cmd/whatomate/main.go
git commit -m "feat(crm): configurable occurrence stages with integrity rules"
```

---

## Task 3: Criar e listar ocorrências, com autorização (GATES 2 e 3)

**Contexto:** Aqui entra o gate mais delicado. Este código já teve quatro rodadas de correção de vazamento de visibilidade, e em todas o buraco estava num endpoint que ninguém lembrou de proteger. A listagem **não** pode usar `join` com `contacts`: `scopeVisibleConversations` escreve `id` e `assigned_user_id` sem qualificar, então num join sobre `occurrences` resolveria para as colunas erradas e devolveria dados errados sem erro nenhum.

**Files:**
- Create: `internal/handlers/occurrences.go`
- Modify: `cmd/whatomate/main.go`
- Test: `internal/handlers/occurrences_test.go`

**Interfaces:**
- Consumes: `a.insertOccurrenceWithProtocol`, `a.initialStage` (T1); `a.canViewConversation`, `a.canInteractWithConversation`, `a.scopeVisibleConversations`.
- Produces: `OccurrenceResponse` com `id`, `protocol_number`, `contact_id`, `contact_name`, `title`, `description`, `stage_id`, `stage_name`, `priority`, `assigned_user_id`, `assigned_user_name`, `opened_at`, `closed_at`, `source_transfer_id`; rotas `POST/GET /api/occurrences` e `GET /api/contacts/{id}/occurrences`.

- [ ] **Step 1: Escrever os testes de autorização (GATES 2 e 3)**

Criar `internal/handlers/occurrences_test.go`:

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

// GATE 2 (positivo). Quem enxerga a conversa cria a ocorrência.
func TestOccurrences_CreateAllowedForVisibleContact(t *testing.T) {
	app := testutil.SetupTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)
	contact := testutil.CreateTestContactWith(t, app.DB, org.ID,
		testutil.WithContactAssignedUser(user.ID))

	req := testutil.NewAuthedRequestWithBody(t, app, user, org, "POST", "/api/occurrences",
		map[string]any{"contact_id": contact.ID.String(), "title": "Troca de produto"})

	require.NoError(t, app.CreateOccurrence(req))
	assert.Equal(t, fasthttp.StatusOK, req.RequestCtx.Response.StatusCode())

	var occ models.Occurrence
	require.NoError(t, app.DB.Where("contact_id = ?", contact.ID).First(&occ).Error)
	assert.NotEmpty(t, occ.ProtocolNumber, "o protocolo deve ter sido gerado")
	assert.NotEqual(t, "", occ.StageID.String())
}

// GATE 2 (negativo). Quem NÃO enxerga a conversa recebe 403 — e nenhuma linha
// é criada. Este é o teste que teria pego os quatro vazamentos anteriores.
func TestOccurrences_CreateDeniedForInvisibleContact(t *testing.T) {
	app := testutil.SetupTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	owner := testutil.CreateTestUser(t, app.DB, org.ID)
	outsider := testutil.CreateTestAgentWithoutContactsRead(t, app.DB, org.ID)
	testutil.EnableStrictVisibility(t, app.DB, org.ID)

	contact := testutil.CreateTestContactWith(t, app.DB, org.ID,
		testutil.WithContactAssignedUser(owner.ID))

	req := testutil.NewAuthedRequestWithBody(t, app, outsider, org, "POST", "/api/occurrences",
		map[string]any{"contact_id": contact.ID.String(), "title": "Não deveria abrir"})

	require.NoError(t, app.CreateOccurrence(req))
	assert.Equal(t, fasthttp.StatusForbidden, req.RequestCtx.Response.StatusCode())

	var count int64
	app.DB.Model(&models.Occurrence{}).Where("contact_id = ?", contact.ID).Count(&count)
	assert.EqualValues(t, 0, count, "nenhuma ocorrência pode ter sido criada")
}

// GATE 2 (negativo, listagem). A lista não mostra ocorrência de contato invisível.
func TestOccurrences_ListHidesInvisibleContacts(t *testing.T) {
	app := testutil.SetupTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	owner := testutil.CreateTestUser(t, app.DB, org.ID)
	outsider := testutil.CreateTestAgentWithoutContactsRead(t, app.DB, org.ID)
	testutil.EnableStrictVisibility(t, app.DB, org.ID)

	contact := testutil.CreateTestContactWith(t, app.DB, org.ID,
		testutil.WithContactAssignedUser(owner.ID))
	stage, err := app.InitialStageForTest(org.ID)
	require.NoError(t, err)

	occ := models.Occurrence{
		OrganizationID: org.ID, ContactID: contact.ID, Title: "Privada",
		StageID: stage.ID, OpenedByUserID: owner.ID,
	}
	require.NoError(t, app.CreateOccurrenceForTest(&occ))

	req := testutil.NewAuthedRequest(t, app, outsider, org, "GET", "/api/occurrences")
	require.NoError(t, app.ListOccurrences(req))

	body := string(req.RequestCtx.Response.Body())
	assert.NotContains(t, body, occ.ProtocolNumber,
		"a ocorrência de um contato invisível não pode aparecer na lista")
}

// GATE 3. A exceção do responsável: quem tem a ocorrência atribuída a enxerga,
// mesmo sem enxergar o contato. Sem isso dá para atribuir um caso que a pessoa
// não consegue abrir.
func TestOccurrences_AssigneeSeesOwnEvenWhenContactInvisible(t *testing.T) {
	app := testutil.SetupTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	owner := testutil.CreateTestUser(t, app.DB, org.ID)
	assignee := testutil.CreateTestAgentWithoutContactsRead(t, app.DB, org.ID)
	testutil.EnableStrictVisibility(t, app.DB, org.ID)

	contact := testutil.CreateTestContactWith(t, app.DB, org.ID,
		testutil.WithContactAssignedUser(owner.ID))
	stage, err := app.InitialStageForTest(org.ID)
	require.NoError(t, err)

	occ := models.Occurrence{
		OrganizationID: org.ID, ContactID: contact.ID, Title: "Atribuída a mim",
		StageID: stage.ID, OpenedByUserID: owner.ID, AssignedUserID: &assignee.ID,
	}
	require.NoError(t, app.CreateOccurrenceForTest(&occ))

	req := testutil.NewAuthedRequest(t, app, assignee, org, "GET", "/api/occurrences")
	require.NoError(t, app.ListOccurrences(req))

	body := string(req.RequestCtx.Response.Body())
	assert.Contains(t, body, occ.ProtocolNumber,
		"o responsável precisa enxergar a própria ocorrência")
}
```

Os helpers `CreateTestAgentWithoutContactsRead`, `EnableStrictVisibility` e `WithContactAssignedUser` podem não existir. **Leia `test/testutil/fixtures.go` e `internal/handlers/conversation_visibility_test.go`** e reutilize os equivalentes que já existem lá — a suíte de visibilidade já monta exatamente esse cenário. Não invente helper novo se houver um pronto.

- [ ] **Step 2: Rodar e confirmar falha**

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc" && TEST_DATABASE_URL='postgres://postgres:postgres@127.0.0.1:5432/whatomate_test?sslmode=disable' go test ./internal/handlers/ -run TestOccurrences -v
```

Esperado: falha de compilação.

- [ ] **Step 3: Implementar criar e listar**

Criar `internal/handlers/occurrences.go`:

```go
package handlers

import (
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

// CreateOccurrenceRequest is the body for opening a case.
type CreateOccurrenceRequest struct {
	ContactID        string  `json:"contact_id"`
	Title            string  `json:"title"`
	Description      string  `json:"description"`
	Priority         string  `json:"priority"`
	AssignedUserID   *string `json:"assigned_user_id"`
	SourceTransferID *string `json:"source_transfer_id"`
}

// OccurrenceResponse is the API shape of an occurrence.
type OccurrenceResponse struct {
	ID               uuid.UUID  `json:"id"`
	ProtocolNumber   string     `json:"protocol_number"`
	ContactID        uuid.UUID  `json:"contact_id"`
	ContactName      string     `json:"contact_name"`
	Title            string     `json:"title"`
	Description      string     `json:"description"`
	StageID          uuid.UUID  `json:"stage_id"`
	StageName        string     `json:"stage_name"`
	Priority         string     `json:"priority"`
	AssignedUserID   *uuid.UUID `json:"assigned_user_id,omitempty"`
	AssignedUserName string     `json:"assigned_user_name,omitempty"`
	OpenedAt         time.Time  `json:"opened_at"`
	ClosedAt         *time.Time `json:"closed_at,omitempty"`
	SourceTransferID *uuid.UUID `json:"source_transfer_id,omitempty"`
}

func occurrenceToResponse(o models.Occurrence) OccurrenceResponse {
	resp := OccurrenceResponse{
		ID:               o.ID,
		ProtocolNumber:   o.ProtocolNumber,
		ContactID:        o.ContactID,
		Title:            o.Title,
		Description:      o.Description,
		StageID:          o.StageID,
		Priority:         string(o.Priority),
		AssignedUserID:   o.AssignedUserID,
		OpenedAt:         o.OpenedAt,
		ClosedAt:         o.ClosedAt,
		SourceTransferID: o.SourceTransferID,
	}
	if o.Contact != nil {
		resp.ContactName = o.Contact.ProfileName
	}
	if o.Stage != nil {
		resp.StageName = o.Stage.Name
	}
	if o.AssignedUser != nil {
		resp.AssignedUserName = o.AssignedUser.FullName
	}
	return resp
}

// visibleOccurrences scopes a query on the occurrences table to what the user
// may see.
//
// Subquery, NOT a join: scopeVisibleConversations writes `id`,
// `assigned_user_id` and `contacts.*` unqualified, so joined onto occurrences
// it would resolve `id` to occurrences.id and silently return wrong rows. This
// mirrors ListAgentTransfers and the chatbot session listing.
func (a *App) visibleOccurrences(query *gorm.DB, userID, orgID uuid.UUID) *gorm.DB {
	visibleContacts := a.scopeVisibleConversations(
		a.DB.Model(&models.Contact{}).Where("organization_id = ?", orgID).Select("id"),
		userID, orgID)

	// The OR is the assignee exception: a case assigned to you is always yours
	// to open, even when the contact itself is outside your scope.
	return query.Where("occurrences.contact_id IN (?) OR occurrences.assigned_user_id = ?",
		visibleContacts, userID)
}

// CreateOccurrence opens a case and issues its protocol.
func (a *App) CreateOccurrence(r *fastglue.Request) error {
	orgID, userID, err := a.requireAuth(r, models.ResourceChat, models.ActionWrite)
	if err != nil {
		return nil
	}

	var req CreateOccurrenceRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}
	if req.Title == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "title is required", nil, "")
	}

	contactID, err := uuid.Parse(req.ContactID)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid contact_id", nil, "")
	}

	contact, err := findByIDAndOrg[models.Contact](a.DB, r, contactID, orgID, "Contact")
	if err != nil {
		return nil
	}

	// GATE 2: opening a case is interacting with the conversation.
	if !a.canInteractWithConversation(userID, orgID, contact) {
		return r.SendErrorEnvelope(fasthttp.StatusForbidden,
			"You do not have access to this conversation", nil, "")
	}

	stage, err := a.initialStage(orgID)
	if err != nil {
		a.Log.Error("Failed to resolve initial stage", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError,
			"Failed to resolve initial stage", nil, "")
	}

	priority := models.OccurrencePriority(req.Priority)
	if priority == "" {
		priority = models.OccurrencePriorityNormal
	}

	occ := models.Occurrence{
		OrganizationID: orgID,
		ContactID:      contactID,
		Title:          req.Title,
		Description:    req.Description,
		StageID:        stage.ID,
		Priority:       priority,
		OpenedByUserID: userID,
	}
	if req.AssignedUserID != nil && *req.AssignedUserID != "" {
		if id, err := uuid.Parse(*req.AssignedUserID); err == nil {
			occ.AssignedUserID = &id
		}
	}
	if req.SourceTransferID != nil && *req.SourceTransferID != "" {
		if id, err := uuid.Parse(*req.SourceTransferID); err == nil {
			occ.SourceTransferID = &id
		}
	}

	if err := a.insertOccurrenceWithProtocol(&occ); err != nil {
		a.Log.Error("Failed to create occurrence", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError,
			"Failed to create occurrence", nil, "")
	}

	occ.Stage = stage
	occ.Contact = contact
	return r.SendEnvelope(occurrenceToResponse(occ))
}

// ListOccurrences returns the cases the user may see, newest first.
func (a *App) ListOccurrences(r *fastglue.Request) error {
	orgID, userID, err := a.requireAuth(r, models.ResourceChat, models.ActionRead)
	if err != nil {
		return nil
	}

	pg := parsePaginationWithDefaults(r, 30, 100)

	query := a.DB.Model(&models.Occurrence{}).
		Where("occurrences.organization_id = ?", orgID)
	query = a.visibleOccurrences(query, userID, orgID)

	if stageID := string(r.RequestCtx.QueryArgs().Peek("stage_id")); stageID != "" {
		query = query.Where("occurrences.stage_id = ?", stageID)
	}
	if contactID := string(r.RequestCtx.QueryArgs().Peek("contact_id")); contactID != "" {
		query = query.Where("occurrences.contact_id = ?", contactID)
	}
	if protocol := string(r.RequestCtx.QueryArgs().Peek("protocol")); protocol != "" {
		query = query.Where("occurrences.protocol_number ILIKE ?", "%"+protocol+"%")
	}
	if open := string(r.RequestCtx.QueryArgs().Peek("open")); open == "true" {
		query = query.Where("occurrences.closed_at IS NULL")
	}

	var total int64
	query.Count(&total)

	var occurrences []models.Occurrence
	if err := query.
		Preload("Contact").Preload("Stage").Preload("AssignedUser").
		Order("occurrences.opened_at DESC").
		Limit(pg.Limit).Offset(pg.Offset).
		Find(&occurrences).Error; err != nil {
		a.Log.Error("Failed to list occurrences", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError,
			"Failed to list occurrences", nil, "")
	}

	result := make([]OccurrenceResponse, len(occurrences))
	for i, o := range occurrences {
		result[i] = occurrenceToResponse(o)
	}

	return r.SendEnvelope(map[string]any{
		"occurrences": result,
		"total":       total,
		"has_more":    len(occurrences) == pg.Limit,
	})
}

// ListContactOccurrences feeds the chat panel with one contact's cases.
func (a *App) ListContactOccurrences(r *fastglue.Request) error {
	orgID, userID, err := a.requireAuth(r, models.ResourceChat, models.ActionRead)
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
	if !a.canViewConversation(userID, orgID, contact) {
		return r.SendErrorEnvelope(fasthttp.StatusForbidden,
			"You do not have access to this conversation", nil, "")
	}

	var occurrences []models.Occurrence
	if err := a.DB.Where("organization_id = ? AND contact_id = ?", orgID, contactID).
		Preload("Stage").Preload("AssignedUser").
		Order("opened_at DESC").Find(&occurrences).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError,
			"Failed to list occurrences", nil, "")
	}

	result := make([]OccurrenceResponse, len(occurrences))
	for i, o := range occurrences {
		o.Contact = contact
		result[i] = occurrenceToResponse(o)
	}

	return r.SendEnvelope(map[string]any{"occurrences": result})
}
```

Acrescentar `"gorm.io/gorm"` ao bloco de imports.

- [ ] **Step 4: Registrar as rotas**

Em `cmd/whatomate/main.go`, junto ao bloco criado na Task 2:

```go
	// CRM — ocorrências
	g.GET("/api/occurrences", app.ListOccurrences)
	g.POST("/api/occurrences", app.CreateOccurrence)
	g.GET("/api/contacts/{id}/occurrences", app.ListContactOccurrences)
```

- [ ] **Step 5: Rodar os testes e confirmar que passam**

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc" && TEST_DATABASE_URL='postgres://postgres:postgres@127.0.0.1:5432/whatomate_test?sslmode=disable' go test ./internal/handlers/ -run TestOccurrences -v
```

Esperado: 4 testes passando.

- [ ] **Step 6: Teste de mutação no gate de visibilidade**

Comentar temporariamente a condição `if !a.canInteractWithConversation(...)` em `CreateOccurrence` e rodar de novo. Esperado: `TestOccurrences_CreateDeniedForInvisibleContact` **falha**. Reverter, confirmar verde, e registrar no relatório.

- [ ] **Step 7: Build, vet e commit**

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc" && go build ./... && go vet ./...
```

```bash
git add internal/handlers/occurrences.go internal/handlers/occurrences_test.go cmd/whatomate/main.go
git commit -m "feat(crm): create and list occurrences behind the conversation gate"
```

---

## Task 4: Detalhe, atualização, mudança de etapa e timeline (GATES 4 e 6)

**Contexto:** Todo endpoint novo repete o gate. É repetitivo de propósito: nas quatro rodadas de vazamento anteriores deste código, o buraco esteve sempre num endpoint que ficou de fora.

**Files:**
- Modify: `internal/handlers/occurrences.go` (acrescentar handlers no fim)
- Modify: `cmd/whatomate/main.go`
- Test: `internal/handlers/occurrences_detail_test.go`

**Interfaces:**
- Consumes: `OccurrenceResponse`, `occurrenceToResponse`, `a.visibleOccurrences` (T3).
- Produces: `func (a *App) loadAuthorizedOccurrence(r *fastglue.Request, orgID, userID uuid.UUID, needInteract bool) (*models.Occurrence, error)`; rotas de detalhe, update, stage e events; `OccurrenceEventResponse`.

- [ ] **Step 1: Escrever os testes (GATES 4 e 6)**

Criar `internal/handlers/occurrences_detail_test.go`:

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

// GATE 6. Entrar numa etapa de fechamento preenche closed_at; voltar limpa.
func TestOccurrences_CloseAndReopen(t *testing.T) {
	app := testutil.SetupTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)
	contact := testutil.CreateTestContactWith(t, app.DB, org.ID,
		testutil.WithContactAssignedUser(user.ID))

	initial, err := app.InitialStageForTest(org.ID)
	require.NoError(t, err)

	var closing models.OccurrenceStage
	require.NoError(t, app.DB.Where("organization_id = ? AND is_closing = ?", org.ID, true).
		First(&closing).Error)

	occ := models.Occurrence{
		OrganizationID: org.ID, ContactID: contact.ID, Title: "Fecha e reabre",
		StageID: initial.ID, OpenedByUserID: user.ID,
	}
	require.NoError(t, app.CreateOccurrenceForTest(&occ))

	// Fecha
	req := testutil.NewAuthedRequestWithBody(t, app, user, org, "PUT",
		"/api/occurrences/"+occ.ID.String()+"/stage",
		map[string]any{"stage_id": closing.ID.String()})
	testutil.SetPathParam(req, "id", occ.ID.String())
	require.NoError(t, app.ChangeOccurrenceStage(req))
	assert.Equal(t, fasthttp.StatusOK, req.RequestCtx.Response.StatusCode())

	var closed models.Occurrence
	require.NoError(t, app.DB.First(&closed, occ.ID).Error)
	require.NotNil(t, closed.ClosedAt, "closed_at deve ser preenchido na etapa de fechamento")

	// Reabre
	req2 := testutil.NewAuthedRequestWithBody(t, app, user, org, "PUT",
		"/api/occurrences/"+occ.ID.String()+"/stage",
		map[string]any{"stage_id": initial.ID.String()})
	testutil.SetPathParam(req2, "id", occ.ID.String())
	require.NoError(t, app.ChangeOccurrenceStage(req2))

	var reopened models.Occurrence
	require.NoError(t, app.DB.First(&reopened, occ.ID).Error)
	assert.Nil(t, reopened.ClosedAt, "reabrir deve limpar closed_at")

	// Ambas as transições ficaram registradas
	var changes int64
	app.DB.Model(&models.OccurrenceEvent{}).
		Where("occurrence_id = ? AND type = ?", occ.ID, models.OccurrenceEventStageChange).
		Count(&changes)
	assert.EqualValues(t, 2, changes, "cada mudança de etapa vira um evento")
}

// GATE 4. Cada endpoint carrega o próprio gate. Um só desprotegido já vaza.
func TestOccurrences_EveryEndpointIsAuthorized(t *testing.T) {
	app := testutil.SetupTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	owner := testutil.CreateTestUser(t, app.DB, org.ID)
	outsider := testutil.CreateTestAgentWithoutContactsRead(t, app.DB, org.ID)
	testutil.EnableStrictVisibility(t, app.DB, org.ID)

	contact := testutil.CreateTestContactWith(t, app.DB, org.ID,
		testutil.WithContactAssignedUser(owner.ID))
	stage, err := app.InitialStageForTest(org.ID)
	require.NoError(t, err)

	occ := models.Occurrence{
		OrganizationID: org.ID, ContactID: contact.ID, Title: "Privada",
		StageID: stage.ID, OpenedByUserID: owner.ID,
	}
	require.NoError(t, app.CreateOccurrenceForTest(&occ))

	cases := []struct {
		name    string
		handler func(*fastglue.Request) error
		method  string
		body    map[string]any
	}{
		{"detalhe", app.GetOccurrence, "GET", nil},
		{"atualizar", app.UpdateOccurrence, "PUT", map[string]any{"title": "x"}},
		{"mudar etapa", app.ChangeOccurrenceStage, "PUT", map[string]any{"stage_id": stage.ID.String()}},
		{"listar eventos", app.ListOccurrenceEvents, "GET", nil},
		{"criar evento", app.CreateOccurrenceEvent, "POST", map[string]any{"content": "nota"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var req *fastglue.Request
			if tc.body == nil {
				req = testutil.NewAuthedRequest(t, app, outsider, org, tc.method,
					"/api/occurrences/"+occ.ID.String())
			} else {
				req = testutil.NewAuthedRequestWithBody(t, app, outsider, org, tc.method,
					"/api/occurrences/"+occ.ID.String(), tc.body)
			}
			testutil.SetPathParam(req, "id", occ.ID.String())

			require.NoError(t, tc.handler(req))
			assert.Equal(t, fasthttp.StatusForbidden, req.RequestCtx.Response.StatusCode(),
				"%s deve recusar quem não enxerga o contato", tc.name)
		})
	}
}

// Nota adicionada aparece na timeline.
func TestOccurrences_NoteAppearsOnTimeline(t *testing.T) {
	app := testutil.SetupTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)
	contact := testutil.CreateTestContactWith(t, app.DB, org.ID,
		testutil.WithContactAssignedUser(user.ID))
	stage, err := app.InitialStageForTest(org.ID)
	require.NoError(t, err)

	occ := models.Occurrence{
		OrganizationID: org.ID, ContactID: contact.ID, Title: "Com nota",
		StageID: stage.ID, OpenedByUserID: user.ID,
	}
	require.NoError(t, app.CreateOccurrenceForTest(&occ))

	req := testutil.NewAuthedRequestWithBody(t, app, user, org, "POST",
		"/api/occurrences/"+occ.ID.String()+"/events",
		map[string]any{"content": "Aguardando NF do fornecedor"})
	testutil.SetPathParam(req, "id", occ.ID.String())

	require.NoError(t, app.CreateOccurrenceEvent(req))
	assert.Equal(t, fasthttp.StatusOK, req.RequestCtx.Response.StatusCode())

	list := testutil.NewAuthedRequest(t, app, user, org, "GET",
		"/api/occurrences/"+occ.ID.String()+"/events")
	testutil.SetPathParam(list, "id", occ.ID.String())
	require.NoError(t, app.ListOccurrenceEvents(list))

	assert.Contains(t, string(list.RequestCtx.Response.Body()), "Aguardando NF do fornecedor")
}
```

Acrescentar `"github.com/zerodha/fastglue"` aos imports.

- [ ] **Step 2: Rodar e confirmar falha**

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc" && TEST_DATABASE_URL='postgres://postgres:postgres@127.0.0.1:5432/whatomate_test?sslmode=disable' go test ./internal/handlers/ -run "TestOccurrences_(CloseAndReopen|EveryEndpoint|NoteAppears)" -v
```

Esperado: falha de compilação.

- [ ] **Step 3: Implementar o carregador autorizado e os handlers**

Acrescentar ao fim de `internal/handlers/occurrences.go`:

```go
// UpdateOccurrenceRequest is the body for editing a case's editable fields.
type UpdateOccurrenceRequest struct {
	Title          string  `json:"title"`
	Description    string  `json:"description"`
	Priority       string  `json:"priority"`
	AssignedUserID *string `json:"assigned_user_id"`
}

// ChangeStageRequest moves a case to another stage.
type ChangeStageRequest struct {
	StageID string `json:"stage_id"`
}

// OccurrenceEventRequest adds a manual note to the timeline.
type OccurrenceEventRequest struct {
	Content string `json:"content"`
}

// OccurrenceEventResponse is the API shape of a timeline entry.
type OccurrenceEventResponse struct {
	ID            uuid.UUID  `json:"id"`
	Type          string     `json:"type"`
	Content       string     `json:"content"`
	Metadata      any        `json:"metadata"`
	CreatedByID   *uuid.UUID `json:"created_by_id,omitempty"`
	CreatedByName string     `json:"created_by_name,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

// loadAuthorizedOccurrence resolves the occurrence from the path and applies the
// conversation gate. Every occurrence endpoint goes through it — the repetition
// is the point: in this codebase's four visibility leaks, the hole was always an
// endpoint someone forgot to gate.
//
// needInteract asks for write access; false checks view access only.
func (a *App) loadAuthorizedOccurrence(r *fastglue.Request, orgID, userID uuid.UUID, needInteract bool) (*models.Occurrence, error) {
	occurrenceID, err := parsePathUUID(r, "id", "occurrence")
	if err != nil {
		return nil, errEnvelopeSent
	}

	occ, err := findByIDAndOrg[models.Occurrence](a.DB, r, occurrenceID, orgID, "Occurrence")
	if err != nil {
		return nil, errEnvelopeSent
	}

	var contact models.Contact
	if err := a.DB.Where("id = ? AND organization_id = ?", occ.ContactID, orgID).
		First(&contact).Error; err != nil {
		_ = r.SendErrorEnvelope(fasthttp.StatusNotFound, "Contact not found", nil, "")
		return nil, errEnvelopeSent
	}

	// The assignee exception, mirroring visibleOccurrences.
	isAssignee := occ.AssignedUserID != nil && *occ.AssignedUserID == userID

	allowed := isAssignee
	if !allowed {
		if needInteract {
			allowed = a.canInteractWithConversation(userID, orgID, &contact)
		} else {
			allowed = a.canViewConversation(userID, orgID, &contact)
		}
	}
	if !allowed {
		_ = r.SendErrorEnvelope(fasthttp.StatusForbidden,
			"You do not have access to this occurrence", nil, "")
		return nil, errEnvelopeSent
	}

	occ.Contact = &contact
	return occ, nil
}

// GetOccurrence returns one case.
func (a *App) GetOccurrence(r *fastglue.Request) error {
	orgID, userID, err := a.requireAuth(r, models.ResourceChat, models.ActionRead)
	if err != nil {
		return nil
	}
	occ, err := a.loadAuthorizedOccurrence(r, orgID, userID, false)
	if err != nil {
		return nil
	}
	a.DB.Preload("Stage").Preload("AssignedUser").First(occ, occ.ID)
	return r.SendEnvelope(occurrenceToResponse(*occ))
}

// UpdateOccurrence edits title, description, priority and assignee.
func (a *App) UpdateOccurrence(r *fastglue.Request) error {
	orgID, userID, err := a.requireAuth(r, models.ResourceChat, models.ActionWrite)
	if err != nil {
		return nil
	}
	occ, err := a.loadAuthorizedOccurrence(r, orgID, userID, true)
	if err != nil {
		return nil
	}

	var req UpdateOccurrenceRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}
	if req.Title == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "title is required", nil, "")
	}

	updates := map[string]any{
		"title":       req.Title,
		"description": req.Description,
	}
	if req.Priority != "" {
		updates["priority"] = req.Priority
	}

	assigneeChanged := false
	if req.AssignedUserID != nil {
		if *req.AssignedUserID == "" {
			updates["assigned_user_id"] = nil
			assigneeChanged = occ.AssignedUserID != nil
		} else if id, err := uuid.Parse(*req.AssignedUserID); err == nil {
			updates["assigned_user_id"] = id
			assigneeChanged = occ.AssignedUserID == nil || *occ.AssignedUserID != id
		}
	}

	if err := a.DB.Model(occ).Updates(updates).Error; err != nil {
		a.Log.Error("Failed to update occurrence", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError,
			"Failed to update occurrence", nil, "")
	}

	if assigneeChanged {
		a.DB.Create(&models.OccurrenceEvent{
			OrganizationID: orgID,
			OccurrenceID:   occ.ID,
			Type:           models.OccurrenceEventAssignment,
			CreatedByID:    &userID,
		})
	}

	return r.SendEnvelope(occurrenceToResponse(*occ))
}

// ChangeOccurrenceStage moves a case and records the transition. Entering a
// closing stage stamps closed_at; leaving one clears it — that is how reopening
// works, without a separate endpoint.
func (a *App) ChangeOccurrenceStage(r *fastglue.Request) error {
	orgID, userID, err := a.requireAuth(r, models.ResourceChat, models.ActionWrite)
	if err != nil {
		return nil
	}
	occ, err := a.loadAuthorizedOccurrence(r, orgID, userID, true)
	if err != nil {
		return nil
	}

	var req ChangeStageRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}
	stageID, err := uuid.Parse(req.StageID)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid stage_id", nil, "")
	}

	target, err := findByIDAndOrg[models.OccurrenceStage](a.DB, r, stageID, orgID, "Stage")
	if err != nil {
		return nil
	}

	var from models.OccurrenceStage
	a.DB.Where("id = ?", occ.StageID).First(&from)

	updates := map[string]any{"stage_id": target.ID}
	if target.IsClosing {
		now := time.Now()
		updates["closed_at"] = &now
	} else {
		updates["closed_at"] = nil
	}

	if err := a.DB.Model(occ).Updates(updates).Error; err != nil {
		a.Log.Error("Failed to change occurrence stage", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError,
			"Failed to change stage", nil, "")
	}

	eventType := models.OccurrenceEventStageChange
	a.DB.Create(&models.OccurrenceEvent{
		OrganizationID: orgID,
		OccurrenceID:   occ.ID,
		Type:           eventType,
		Content:        from.Name + " → " + target.Name,
		Metadata:       models.JSONB{"from_stage_id": from.ID.String(), "to_stage_id": target.ID.String()},
		CreatedByID:    &userID,
	})

	if target.IsClosing {
		a.DB.Create(&models.OccurrenceEvent{
			OrganizationID: orgID,
			OccurrenceID:   occ.ID,
			Type:           models.OccurrenceEventClosed,
			CreatedByID:    &userID,
		})
	}

	occ.Stage = target
	return r.SendEnvelope(occurrenceToResponse(*occ))
}

// ListOccurrenceEvents returns the timeline, oldest first.
func (a *App) ListOccurrenceEvents(r *fastglue.Request) error {
	orgID, userID, err := a.requireAuth(r, models.ResourceChat, models.ActionRead)
	if err != nil {
		return nil
	}
	occ, err := a.loadAuthorizedOccurrence(r, orgID, userID, false)
	if err != nil {
		return nil
	}

	var events []models.OccurrenceEvent
	if err := a.DB.Where("organization_id = ? AND occurrence_id = ?", orgID, occ.ID).
		Preload("CreatedBy").Order("created_at ASC").Find(&events).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError,
			"Failed to list events", nil, "")
	}

	result := make([]OccurrenceEventResponse, len(events))
	for i, e := range events {
		result[i] = OccurrenceEventResponse{
			ID:          e.ID,
			Type:        string(e.Type),
			Content:     e.Content,
			Metadata:    e.Metadata,
			CreatedByID: e.CreatedByID,
			CreatedAt:   e.CreatedAt,
		}
		if e.CreatedBy != nil {
			result[i].CreatedByName = e.CreatedBy.FullName
		}
	}

	return r.SendEnvelope(map[string]any{"events": result})
}

// CreateOccurrenceEvent adds a manual note to the timeline.
func (a *App) CreateOccurrenceEvent(r *fastglue.Request) error {
	orgID, userID, err := a.requireAuth(r, models.ResourceChat, models.ActionWrite)
	if err != nil {
		return nil
	}
	occ, err := a.loadAuthorizedOccurrence(r, orgID, userID, true)
	if err != nil {
		return nil
	}

	var req OccurrenceEventRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}
	if req.Content == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "content is required", nil, "")
	}

	event := models.OccurrenceEvent{
		OrganizationID: orgID,
		OccurrenceID:   occ.ID,
		Type:           models.OccurrenceEventNote,
		Content:        req.Content,
		CreatedByID:    &userID,
	}
	if err := a.DB.Create(&event).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError,
			"Failed to create event", nil, "")
	}

	return r.SendEnvelope(OccurrenceEventResponse{
		ID:          event.ID,
		Type:        string(event.Type),
		Content:     event.Content,
		CreatedByID: event.CreatedByID,
		CreatedAt:   event.CreatedAt,
	})
}
```

- [ ] **Step 4: Registrar as rotas**

```go
	g.GET("/api/occurrences/{id}", app.GetOccurrence)
	g.PUT("/api/occurrences/{id}", app.UpdateOccurrence)
	g.PUT("/api/occurrences/{id}/stage", app.ChangeOccurrenceStage)
	g.GET("/api/occurrences/{id}/events", app.ListOccurrenceEvents)
	g.POST("/api/occurrences/{id}/events", app.CreateOccurrenceEvent)
```

- [ ] **Step 5: Rodar os testes e confirmar que passam**

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc" && TEST_DATABASE_URL='postgres://postgres:postgres@127.0.0.1:5432/whatomate_test?sslmode=disable' go test ./internal/handlers/ -run TestOccurrences -v
```

Esperado: todos os testes de ocorrência passando, incluindo os 5 subtestes de autorização.

- [ ] **Step 6: Build, vet e commit**

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc" && go build ./... && go vet ./...
```

```bash
git add internal/handlers/occurrences.go internal/handlers/occurrences_detail_test.go cmd/whatomate/main.go
git commit -m "feat(crm): occurrence detail, stage transitions and timeline"
```

---

## Task 5: Envio do protocolo com validação da janela de 24h (GATE 5)

**Contexto:** A janela de 24h **não é validada em lugar nenhum do sistema hoje** — ela é calculada e devolvida como `service_window_open`, e o `SendMessage` envia para a Meta de qualquer forma. Este endpoint é o primeiro a recusar. É uma inconsistência deliberada, decidida pelo dono do produto para não alterar comportamento em produção.

**Files:**
- Create: `internal/handlers/occurrence_send.go`
- Modify: `cmd/whatomate/main.go`
- Test: `internal/handlers/occurrence_send_test.go`

**Interfaces:**
- Consumes: `a.loadAuthorizedOccurrence` (T4); `a.SendOutgoingMessage`, `OutgoingMessageRequest`, `MessageSendOptions` (existentes).
- Produces: rota `POST /api/occurrences/{id}/send-protocol`.

- [ ] **Step 1: Escrever os testes (GATE 5)**

Criar `internal/handlers/occurrence_send_test.go`:

```go
package handlers_test

import (
	"testing"
	"time"

	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// GATE 5. Fora da janela de 24h o envio é recusado com 422, antes de qualquer
// chamada à Meta.
func TestOccurrenceSendProtocol_RejectedOutsideServiceWindow(t *testing.T) {
	app := testutil.SetupTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)

	old := time.Now().Add(-30 * time.Hour)
	contact := testutil.CreateTestContactWith(t, app.DB, org.ID,
		testutil.WithContactAssignedUser(user.ID))
	require.NoError(t, app.DB.Model(contact).Update("last_inbound_at", old).Error)

	stage, err := app.InitialStageForTest(org.ID)
	require.NoError(t, err)
	occ := models.Occurrence{
		OrganizationID: org.ID, ContactID: contact.ID, Title: "Fora da janela",
		StageID: stage.ID, OpenedByUserID: user.ID,
	}
	require.NoError(t, app.CreateOccurrenceForTest(&occ))

	req := testutil.NewAuthedRequest(t, app, user, org, "POST",
		"/api/occurrences/"+occ.ID.String()+"/send-protocol")
	testutil.SetPathParam(req, "id", occ.ID.String())

	require.NoError(t, app.SendOccurrenceProtocol(req))
	assert.Equal(t, fasthttp.StatusUnprocessableEntity, req.RequestCtx.Response.StatusCode())

	var sent int64
	app.DB.Model(&models.OccurrenceEvent{}).
		Where("occurrence_id = ? AND type = ?", occ.ID, models.OccurrenceEventProtocolSent).
		Count(&sent)
	assert.EqualValues(t, 0, sent, "nenhum evento de envio pode ser gravado")
}

// Contato que nunca enviou mensagem também está fora da janela.
func TestOccurrenceSendProtocol_RejectedWhenNeverInbound(t *testing.T) {
	app := testutil.SetupTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	user := testutil.CreateTestUser(t, app.DB, org.ID)
	contact := testutil.CreateTestContactWith(t, app.DB, org.ID,
		testutil.WithContactAssignedUser(user.ID))

	stage, err := app.InitialStageForTest(org.ID)
	require.NoError(t, err)
	occ := models.Occurrence{
		OrganizationID: org.ID, ContactID: contact.ID, Title: "Sem inbound",
		StageID: stage.ID, OpenedByUserID: user.ID,
	}
	require.NoError(t, app.CreateOccurrenceForTest(&occ))

	req := testutil.NewAuthedRequest(t, app, user, org, "POST",
		"/api/occurrences/"+occ.ID.String()+"/send-protocol")
	testutil.SetPathParam(req, "id", occ.ID.String())

	require.NoError(t, app.SendOccurrenceProtocol(req))
	assert.Equal(t, fasthttp.StatusUnprocessableEntity, req.RequestCtx.Response.StatusCode())
}

// GATE 4 aplicado ao envio: quem não enxerga o contato não envia.
func TestOccurrenceSendProtocol_DeniedForInvisibleContact(t *testing.T) {
	app := testutil.SetupTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	owner := testutil.CreateTestUser(t, app.DB, org.ID)
	outsider := testutil.CreateTestAgentWithoutContactsRead(t, app.DB, org.ID)
	testutil.EnableStrictVisibility(t, app.DB, org.ID)

	contact := testutil.CreateTestContactWith(t, app.DB, org.ID,
		testutil.WithContactAssignedUser(owner.ID))
	require.NoError(t, app.DB.Model(contact).Update("last_inbound_at", time.Now()).Error)

	stage, err := app.InitialStageForTest(org.ID)
	require.NoError(t, err)
	occ := models.Occurrence{
		OrganizationID: org.ID, ContactID: contact.ID, Title: "Privada",
		StageID: stage.ID, OpenedByUserID: owner.ID,
	}
	require.NoError(t, app.CreateOccurrenceForTest(&occ))

	req := testutil.NewAuthedRequest(t, app, outsider, org, "POST",
		"/api/occurrences/"+occ.ID.String()+"/send-protocol")
	testutil.SetPathParam(req, "id", occ.ID.String())

	require.NoError(t, app.SendOccurrenceProtocol(req))
	assert.Equal(t, fasthttp.StatusForbidden, req.RequestCtx.Response.StatusCode())
}
```

- [ ] **Step 2: Rodar e confirmar falha**

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc" && TEST_DATABASE_URL='postgres://postgres:postgres@127.0.0.1:5432/whatomate_test?sslmode=disable' go test ./internal/handlers/ -run TestOccurrenceSendProtocol -v
```

Esperado: falha de compilação.

- [ ] **Step 3: Implementar o envio**

Criar `internal/handlers/occurrence_send.go`:

```go
package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

// serviceWindowOpen reports whether the customer messaged within the last 24
// hours. Outside that window WhatsApp only accepts templates.
//
// The same expression exists inline in contacts.go, where it feeds
// service_window_open on the contact payload. It is duplicated rather than
// extracted because extracting it would edit a production file for no
// behavioural gain — see the spec's "no production change" constraint.
func serviceWindowOpen(contact *models.Contact) bool {
	return contact.LastInboundAt != nil && time.Since(*contact.LastInboundAt) < 24*time.Hour
}

// SendOccurrenceProtocol sends the protocol number to the customer.
//
// This is the only endpoint in the system that enforces the 24-hour window.
// SendMessage does not — it computes the window, reports it, and sends anyway.
// The inconsistency is deliberate and recorded in the spec: enforcing it
// globally would change production behaviour.
func (a *App) SendOccurrenceProtocol(r *fastglue.Request) error {
	orgID, userID, err := a.requireAuth(r, models.ResourceChat, models.ActionWrite)
	if err != nil {
		return nil
	}

	occ, err := a.loadAuthorizedOccurrence(r, orgID, userID, true)
	if err != nil {
		return nil
	}

	contact := occ.Contact
	if !serviceWindowOpen(contact) {
		return r.SendErrorEnvelope(fasthttp.StatusUnprocessableEntity,
			"The 24-hour service window is closed; only templates can be sent", nil, "")
	}

	var account models.WhatsAppAccount
	if err := a.DB.Where("organization_id = ? AND name = ?", orgID, contact.WhatsAppAccount).
		First(&account).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest,
			"WhatsApp account not found for this contact", nil, "")
	}

	body := fmt.Sprintf("Seu protocolo de atendimento é %s. Guarde este número para consultas futuras.",
		occ.ProtocolNumber)

	if _, err := a.SendOutgoingMessage(context.Background(), OutgoingMessageRequest{
		Account: &account,
		Contact: contact,
		Type:    models.MessageTypeText,
		Content: body,
	}, DefaultSendOptions()); err != nil {
		a.Log.Error("Failed to send protocol", "error", err, "occurrence", occ.ID)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError,
			"Failed to send protocol", nil, "")
	}

	a.DB.Create(&models.OccurrenceEvent{
		OrganizationID: orgID,
		OccurrenceID:   occ.ID,
		Type:           models.OccurrenceEventProtocolSent,
		Content:        occ.ProtocolNumber,
		CreatedByID:    &userID,
	})

	return r.SendEnvelope(map[string]any{"sent": true, "protocol_number": occ.ProtocolNumber})
}
```

Se `DefaultSendOptions()` ou `models.MessageTypeText` tiverem nomes diferentes, leia `internal/handlers/messages.go` e use os reais. **Não altere `SendOutgoingMessage`.**

- [ ] **Step 4: Registrar a rota**

```go
	g.POST("/api/occurrences/{id}/send-protocol", app.SendOccurrenceProtocol)
```

- [ ] **Step 5: Rodar os testes e confirmar que passam**

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc" && TEST_DATABASE_URL='postgres://postgres:postgres@127.0.0.1:5432/whatomate_test?sslmode=disable' go test ./internal/handlers/ -run TestOccurrenceSendProtocol -v
```

Esperado: 3 testes passando.

- [ ] **Step 6: Build, vet e commit**

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc" && go build ./... && go vet ./...
```

```bash
git add internal/handlers/occurrence_send.go internal/handlers/occurrence_send_test.go cmd/whatomate/main.go
git commit -m "feat(crm): send protocol to the customer, gated on the 24h window"
```

---

## Task 6: Cliente de API e store do frontend

**Files:**
- Modify: `frontend/src/services/api.ts` (no fim, junto ao bloco de `ConversationNote`)
- Create: `frontend/src/stores/occurrences.ts`

**Interfaces:**
- Consumes: as rotas de T2-T5.
- Produces: tipos `Occurrence`, `OccurrenceStage`, `OccurrenceEvent`; `occurrencesService`; store `useOccurrencesStore` com `occurrences`, `stages`, `events`, `isLoading`, `fetchOccurrences`, `fetchContactOccurrences`, `fetchStages`, `createOccurrence`, `changeStage`, `addNote`, `sendProtocol`.

- [ ] **Step 1: Acrescentar tipos e serviço**

Em `frontend/src/services/api.ts`, ao final, seguindo o estilo do bloco `ConversationNote`:

```typescript
export interface OccurrenceStage {
  id: string
  name: string
  color: string
  position: number
  is_initial: boolean
  is_closing: boolean
}

export interface Occurrence {
  id: string
  protocol_number: string
  contact_id: string
  contact_name?: string
  title: string
  description: string
  stage_id: string
  stage_name?: string
  priority: 'low' | 'normal' | 'high' | 'urgent'
  assigned_user_id?: string
  assigned_user_name?: string
  opened_at: string
  closed_at?: string
  source_transfer_id?: string
}

export interface OccurrenceEvent {
  id: string
  type: 'opened' | 'note' | 'stage_change' | 'assignment' | 'protocol_sent' | 'closed'
  content: string
  metadata?: Record<string, unknown>
  created_by_id?: string
  created_by_name?: string
  created_at: string
}

export const occurrencesService = {
  list: (params?: Record<string, string>) =>
    api.get<{ occurrences: Occurrence[]; total: number; has_more: boolean }>('/occurrences', { params }),
  get: (id: string) => api.get<Occurrence>(`/occurrences/${id}`),
  create: (data: {
    contact_id: string
    title: string
    description?: string
    priority?: string
    source_transfer_id?: string
  }) => api.post<Occurrence>('/occurrences', data),
  update: (id: string, data: { title: string; description?: string; priority?: string; assigned_user_id?: string | null }) =>
    api.put<Occurrence>(`/occurrences/${id}`, data),
  changeStage: (id: string, stageId: string) =>
    api.put<Occurrence>(`/occurrences/${id}/stage`, { stage_id: stageId }),
  listEvents: (id: string) =>
    api.get<{ events: OccurrenceEvent[] }>(`/occurrences/${id}/events`),
  addNote: (id: string, content: string) =>
    api.post<OccurrenceEvent>(`/occurrences/${id}/events`, { content }),
  sendProtocol: (id: string) =>
    api.post<{ sent: boolean; protocol_number: string }>(`/occurrences/${id}/send-protocol`),
  listForContact: (contactId: string) =>
    api.get<{ occurrences: Occurrence[] }>(`/contacts/${contactId}/occurrences`),

  listStages: () => api.get<{ stages: OccurrenceStage[] }>('/occurrence-stages'),
  createStage: (data: Partial<OccurrenceStage>) => api.post<OccurrenceStage>('/occurrence-stages', data),
  updateStage: (id: string, data: Partial<OccurrenceStage>) =>
    api.put<OccurrenceStage>(`/occurrence-stages/${id}`, data),
  deleteStage: (id: string) => api.delete(`/occurrence-stages/${id}`),
}
```

Confirme o formato de resposta lendo como `conversationNotesService` desembrulha o envelope (`data.data` ou `data`) e siga o mesmo.

- [ ] **Step 2: Criar a store**

Criar `frontend/src/stores/occurrences.ts` seguindo o estilo de `stores/notes.ts`:

```typescript
import { defineStore } from 'pinia'
import { ref } from 'vue'
import { occurrencesService, type Occurrence, type OccurrenceStage, type OccurrenceEvent } from '@/services/api'

export const useOccurrencesStore = defineStore('occurrences', () => {
  const occurrences = ref<Occurrence[]>([])
  const contactOccurrences = ref<Occurrence[]>([])
  const stages = ref<OccurrenceStage[]>([])
  const events = ref<OccurrenceEvent[]>([])
  const isLoading = ref(false)

  async function fetchStages() {
    const res = await occurrencesService.listStages()
    stages.value = res.data.data?.stages ?? res.data.stages ?? []
  }

  async function fetchOccurrences(params?: Record<string, string>) {
    isLoading.value = true
    try {
      const res = await occurrencesService.list(params)
      occurrences.value = res.data.data?.occurrences ?? res.data.occurrences ?? []
    } finally {
      isLoading.value = false
    }
  }

  async function fetchContactOccurrences(contactId: string) {
    const res = await occurrencesService.listForContact(contactId)
    contactOccurrences.value = res.data.data?.occurrences ?? res.data.occurrences ?? []
  }

  async function fetchEvents(occurrenceId: string) {
    const res = await occurrencesService.listEvents(occurrenceId)
    events.value = res.data.data?.events ?? res.data.events ?? []
  }

  async function createOccurrence(payload: {
    contact_id: string
    title: string
    description?: string
    source_transfer_id?: string
  }) {
    const res = await occurrencesService.create(payload)
    await fetchContactOccurrences(payload.contact_id)
    return res.data.data ?? res.data
  }

  async function changeStage(occurrenceId: string, stageId: string) {
    await occurrencesService.changeStage(occurrenceId, stageId)
    await fetchEvents(occurrenceId)
  }

  async function addNote(occurrenceId: string, content: string) {
    await occurrencesService.addNote(occurrenceId, content)
    await fetchEvents(occurrenceId)
  }

  async function sendProtocol(occurrenceId: string) {
    await occurrencesService.sendProtocol(occurrenceId)
    await fetchEvents(occurrenceId)
  }

  function clear() {
    occurrences.value = []
    contactOccurrences.value = []
    events.value = []
  }

  return {
    occurrences, contactOccurrences, stages, events, isLoading,
    fetchStages, fetchOccurrences, fetchContactOccurrences, fetchEvents,
    createOccurrence, changeStage, addNote, sendProtocol, clear,
  }
})
```

- [ ] **Step 3: Typecheck e commit**

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc/frontend" && npm run typecheck
```

Esperado: apenas o erro pré-existente `business_calling_enabled`.

```bash
git add frontend/src/services/api.ts frontend/src/stores/occurrences.ts
git commit -m "feat(crm): occurrences API client and store"
```

---

## Task 7: Painel de ocorrências no chat

**Contexto:** É a integração que o dono do produto pediu explicitamente. Painel **irmão** do de notas, não substituto — notas do contato são sobre a pessoa, a timeline da ocorrência é sobre o caso.

**Files:**
- Create: `frontend/src/components/chat/ContactOccurrencesPanel.vue`
- Modify: `frontend/src/views/chat/ChatView.vue`
- Modify: `frontend/src/i18n/locales/pt-BR.json` e `en.json`

**Interfaces:**
- Consumes: `useOccurrencesStore` (T6).
- Produces: componente `ContactOccurrencesPanel` com props `contactId: string`, `sourceTransferId?: string`.

- [ ] **Step 1: Acrescentar as chaves de i18n**

Em ambos os locales, dentro do objeto `chat`, na mesma posição relativa nos dois arquivos:

`pt-BR.json`:
```json
    "occurrences": "Ocorrências",
    "newOccurrence": "Nova ocorrência",
    "occurrenceTitle": "Título",
    "occurrenceDescription": "Descrição",
    "noOccurrences": "Nenhuma ocorrência para este contato",
    "protocol": "Protocolo",
    "sendProtocol": "Enviar protocolo",
    "sendProtocolWindowClosed": "Fora da janela de 24h — só é possível enviar template",
    "occurrenceTimeline": "Histórico do caso",
    "addNote": "Adicionar anotação",
```

`en.json`:
```json
    "occurrences": "Occurrences",
    "newOccurrence": "New occurrence",
    "occurrenceTitle": "Title",
    "occurrenceDescription": "Description",
    "noOccurrences": "No occurrences for this contact",
    "protocol": "Protocol",
    "sendProtocol": "Send protocol",
    "sendProtocolWindowClosed": "Outside the 24h window — only templates can be sent",
    "occurrenceTimeline": "Case history",
    "addNote": "Add note",
```

- [ ] **Step 2: Criar o painel**

Criar `frontend/src/components/chat/ContactOccurrencesPanel.vue`:

```vue
<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { useOccurrencesStore } from '@/stores/occurrences'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Plus } from 'lucide-vue-next'

const props = defineProps<{
  contactId: string
  sourceTransferId?: string
}>()

const store = useOccurrencesStore()
const isCreating = ref(false)
const newTitle = ref('')

async function load() {
  if (!props.contactId) return
  await store.fetchContactOccurrences(props.contactId)
}

async function create() {
  if (!newTitle.value.trim()) return
  await store.createOccurrence({
    contact_id: props.contactId,
    title: newTitle.value.trim(),
    source_transfer_id: props.sourceTransferId,
  })
  newTitle.value = ''
  isCreating.value = false
}

onMounted(load)
watch(() => props.contactId, load)
</script>

<template>
  <div class="flex flex-col h-full min-h-0">
    <div class="flex items-center justify-between px-4 py-3 border-b shrink-0">
      <h3 class="text-sm font-medium">{{ $t('chat.occurrences') }}</h3>
      <Button variant="ghost" size="sm" @click="isCreating = !isCreating">
        <Plus class="h-4 w-4 mr-1" />
        {{ $t('chat.newOccurrence') }}
      </Button>
    </div>

    <div v-if="isCreating" class="p-4 border-b space-y-2 shrink-0">
      <Input
        v-model="newTitle"
        :placeholder="$t('chat.occurrenceTitle')"
        @keyup.enter="create"
      />
      <Button size="sm" class="w-full" :disabled="!newTitle.trim()" @click="create">
        {{ $t('chat.newOccurrence') }}
      </Button>
    </div>

    <ScrollArea orientation="vertical" class="flex-1 min-h-0">
      <div class="p-4 space-y-2">
        <RouterLink
          v-for="occ in store.contactOccurrences"
          :key="occ.id"
          :to="`/crm/occurrences/${occ.id}`"
          class="block p-3 rounded-md border hover:bg-accent transition-colors"
        >
          <div class="flex items-center justify-between gap-2">
            <span class="font-mono text-xs text-muted-foreground">{{ occ.protocol_number }}</span>
            <Badge variant="outline" class="shrink-0 text-xs">{{ occ.stage_name }}</Badge>
          </div>
          <p class="text-sm mt-1 truncate min-w-0">{{ occ.title }}</p>
        </RouterLink>

        <p
          v-if="store.contactOccurrences.length === 0"
          class="text-sm text-muted-foreground text-center py-6"
        >
          {{ $t('chat.noOccurrences') }}
        </p>
      </div>
    </ScrollArea>
  </div>
</template>
```

**Nota de implementação:** o `ScrollArea` recebe `orientation="vertical"` de propósito. Sem isso o reka-ui monta a barra horizontal e aplica `min-width: fit-content` no conteúdo, o que anula o `truncate` — foi exatamente o bug corrigido nos diálogos do chat.

- [ ] **Step 3: Montar o painel no ChatView**

Em `frontend/src/views/chat/ChatView.vue`, ao lado do painel de notas: importar o componente, adicionar `const isOccurrencesPanelOpen = ref(false)`, um botão no cabeçalho da conversa que alterna o painel, e renderizar `<ContactOccurrencesPanel :contact-id="contactsStore.currentContact.id" :source-transfer-id="activeTransferId ?? undefined" />` quando aberto. **Não altere o painel de notas nem sua store.**

- [ ] **Step 4: Verificar**

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc/frontend" && npm run typecheck && npm run i18n:keys && npx eslint src/components/chat/ContactOccurrencesPanel.vue src/views/chat/ChatView.vue
```

Esperado: typecheck só com o erro pré-existente; i18n com as chaves batendo; eslint limpo nos arquivos novos (o `ChatView.vue` tem 4 erros `no-empty` pré-existentes nas linhas ~320/757/817/829 — esses são aceitáveis).

- [ ] **Step 5: Conferir no navegador**

Com Vite em `:3000` e backend em `:8080`, abrir uma conversa, abrir o painel, criar uma ocorrência e confirmar que o protocolo aparece no formato `2026-000001`.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/components/chat/ContactOccurrencesPanel.vue frontend/src/views/chat/ChatView.vue frontend/src/i18n/locales/pt-BR.json frontend/src/i18n/locales/en.json
git commit -m "feat(crm): occurrences panel in the chat, sibling to notes"
```

---

## Task 8: Telas de lista e detalhe

**Files:**
- Create: `frontend/src/views/crm/OccurrencesView.vue`
- Create: `frontend/src/views/crm/OccurrenceDetailView.vue`
- Modify: `frontend/src/router/index.ts`
- Modify: os dois locales

**Interfaces:**
- Consumes: `useOccurrencesStore` (T6).
- Produces: rotas `/crm/occurrences` (nome `occurrences`) e `/crm/occurrences/:id` (nome `occurrence-detail`).

- [ ] **Step 1: Criar a lista**

`OccurrencesView.vue`: cabeçalho com título, campo de busca ligado ao filtro `protocol`, seletor de etapa alimentado por `store.stages`, e tabela com protocolo (fonte monoespaçada), título, contato, etapa como `Badge`, responsável e data de abertura. Cada linha navega para o detalhe. Chamar `store.fetchStages()` e `store.fetchOccurrences()` no `onMounted`. Usar `EmptyState` se o projeto tiver um componente equivalente — verifique em `components/shared/`.

- [ ] **Step 2: Criar o detalhe**

`OccurrenceDetailView.vue`: cabeçalho com protocolo e título; seletor de etapa que chama `store.changeStage`; botão **Enviar protocolo** que chama `store.sendProtocol` e, ao receber 422, mostra `$t('chat.sendProtocolWindowClosed')` via toast; timeline renderizando `store.events` em ordem cronológica com ícone por `type`; e campo de nova anotação chamando `store.addNote`.

- [ ] **Step 3: Registrar as rotas**

Em `frontend/src/router/index.ts`, dentro do bloco de filhos do `AppLayout`, seguindo o estilo das rotas existentes:

```typescript
        {
          path: 'crm/occurrences',
          name: 'occurrences',
          component: () => import('@/views/crm/OccurrencesView.vue'),
        },
        {
          path: 'crm/occurrences/:id',
          name: 'occurrence-detail',
          component: () => import('@/views/crm/OccurrenceDetailView.vue'),
        },
```

- [ ] **Step 4: Acrescentar o item de menu**

Em `frontend/src/components/layout/` (o componente da barra lateral), acrescentar "CRM" apontando para `/crm/occurrences`, seguindo o padrão dos itens existentes.

- [ ] **Step 5: Verificar e commitar**

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc/frontend" && npm run typecheck && npm run i18n:keys && npm run build
```

```bash
git add frontend/src/views/crm/ frontend/src/router/index.ts frontend/src/components/layout/ frontend/src/i18n/locales/
git commit -m "feat(crm): occurrences list and detail views"
```

---

## Task 9: Configuração das etapas

**Files:**
- Create: `frontend/src/views/settings/OccurrenceStagesView.vue`
- Modify: `frontend/src/router/index.ts`
- Modify: os dois locales

- [ ] **Step 1: Criar a tela**

Lista das etapas ordenadas por `position`, cada uma com nome editável, seletor de cor, e marcadores `is_initial` / `is_closing`. Botões de criar e excluir. **Quando a API devolver 409, mostrar a mensagem do backend no toast** — são as regras de integridade da Task 2, e o usuário precisa saber por que a exclusão foi recusada.

- [ ] **Step 2: Registrar a rota** em `/settings/occurrence-stages`, seguindo o padrão das outras telas de settings.

- [ ] **Step 3: Verificar e commitar**

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc/frontend" && npm run typecheck && npm run i18n:keys
```

```bash
git add frontend/src/views/settings/OccurrenceStagesView.vue frontend/src/router/index.ts frontend/src/i18n/locales/
git commit -m "feat(crm): occurrence stages settings screen"
```

---

## Task 10: Cobertura E2E

**Files:**
- Create: `frontend/e2e/pages/OccurrencesPage.ts`
- Modify: `frontend/e2e/pages/index.ts`
- Create: `frontend/e2e/tests/crm/occurrences.spec.ts`

**Interfaces:**
- Produces: classe `OccurrencesPage` estendendo `BasePage`.

- [ ] **Step 1: Criar o page object**

Seguindo o estilo de `frontend/e2e/pages/FlowsPage.ts`, com getters para: painel de ocorrências no chat, botão de nova ocorrência, campo de título, cartão de ocorrência por protocolo, seletor de etapa, botão de enviar protocolo, campo de anotação e itens da timeline. Exportar de `pages/index.ts`.

- [ ] **Step 2: Escrever a spec**

Criar `frontend/e2e/tests/crm/occurrences.spec.ts` com quatro testes, usando `loginAsAdmin` como as demais specs:

1. **cria ocorrência pela conversa** — abrir conversa, abrir painel, criar, e conferir que aparece um protocolo no formato `/^\d{4}-\d{6}$/`.
2. **move de etapa** — abrir o detalhe, trocar a etapa, e conferir que a timeline ganhou uma entrada de mudança.
3. **adiciona anotação** — escrever na timeline e conferir que o texto aparece.
4. **etapa em uso não pode ser excluída** — em Settings, tentar excluir a etapa da ocorrência criada e conferir que aparece a mensagem de conflito e a etapa continua na lista.

Os seletores usam os rótulos de `en.json`, porque os E2E rodam em inglês.

- [ ] **Step 3: Rodar**

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc/frontend" && TEST_DATABASE_URL='postgres://postgres:postgres@127.0.0.1:5432/whatomate_e2e' npx playwright test e2e/tests/crm/occurrences.spec.ts --project=chromium --reporter=list
```

Esperado: 4 passando. Os E2E exigem backend em `:8080` e Postgres no ar.

- [ ] **Step 4: Commit**

```bash
git add frontend/e2e/pages/OccurrencesPage.ts frontend/e2e/pages/index.ts frontend/e2e/tests/crm/occurrences.spec.ts
git commit -m "test(crm): e2e coverage for occurrences and protocol"
```

---

## Task 11: Verificação final da branch

- [ ] **Step 1: Verificações estáticas**

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc" && go build ./... && go vet ./...
```

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc/frontend" && npm run typecheck && npm run i18n:keys && npm run build
```

- [ ] **Step 2: Suíte Go completa, com banco**

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc" && TEST_DATABASE_URL='postgres://postgres:postgres@127.0.0.1:5432/whatomate_test?sslmode=disable' go test ./internal/... 2>&1 | grep -E "^(ok|FAIL|---)"
```

Falhas pré-existentes conhecidas, que **não** são regressão: `internal/handlers` e `internal/worker` estouram por nil pointer em goroutines assíncronas sem Redis conectado. Se aparecer qualquer outra, investigue.

- [ ] **Step 3: E2E de chat e chatbot — provar que nada quebrou**

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc/frontend" && TEST_DATABASE_URL='postgres://postgres:postgres@127.0.0.1:5432/whatomate_e2e' npx playwright test e2e/tests/chat e2e/tests/chatbot e2e/tests/crm --project=chromium --reporter=list 2>&1 | tail -8
```

Falhas conhecidas e não relacionadas: `queue-pickup.spec.ts:519` (falha idêntica na `development`) e `template-header-param.spec.ts` (flake sob carga paralela, passa isolada). Qualquer outra falha é regressão desta branch.

- [ ] **Step 4: Confirmar que nada de produção foi tocado**

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc" && git diff development...HEAD --stat
```

Confirmar que **não** aparecem: `internal/handlers/sla_processor.go`, `internal/handlers/chatbot_processor.go`, `internal/handlers/agent_transfers.go`, `internal/handlers/conversation_notes.go`, `internal/models/models.go`, `internal/models/chatbot.go`, `frontend/src/components/chat/ConversationNotes.vue`, `frontend/src/stores/notes.ts`. Qualquer um deles na lista significa que o escopo vazou — investigue antes de seguir.

`internal/handlers/messages.go` também **não** deve aparecer: o envio do protocolo consome `SendOutgoingMessage`, não o altera.

- [ ] **Step 5: Relatar honestamente**

Escrever o resumo citando **quais comandos rodaram e qual foi a saída real**. Se algum E2E ou verificação não pôde ser executado, dizer isso explicitamente. Não afirmar que algo passou sem ter visto passar.

---

## Notas para quem revisa

Riscos, em ordem de atenção:

1. **A numeração do protocolo** é o único ponto com risco de corrupção de dados. O teste de concorrência e o de mutação da Task 1 são o que separa "funciona na demo" de "funciona com movimento".
2. **A autorização se repete em oito endpoints.** É proposital. O padrão a conferir é: todo handler chama `requireAuth` **e** o gate de conversa — nunca só um dos dois.
3. **A listagem usa subconsulta, não join.** Se alguém "otimizar" para join, `scopeVisibleConversations` passa a resolver `id` como `occurrences.id` e devolve dados errados sem erro. O comentário no código existe para impedir isso.

Dívidas registradas e deliberadas: o `/send-protocol` valida a janela de 24h e o `SendMessage` não; não há WebSocket nesta fase (o painel busca sob demanda); evoluir para múltiplos pipelines exigirá migração das ocorrências existentes.
