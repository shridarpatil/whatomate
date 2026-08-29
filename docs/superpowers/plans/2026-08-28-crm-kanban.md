# CRM Fase 2 — Quadro Kanban: plano de implementação

> **Para agentes executores:** SUB-SKILL OBRIGATÓRIA: use `superpowers:subagent-driven-development` (recomendado) ou `superpowers:executing-plans` para implementar tarefa a tarefa. Os passos usam caixas (`- [ ]`) para acompanhamento.

**Objetivo:** entregar um modo de exibição em quadro na tela de ocorrências, com colunas por etapa e mudança de etapa por arrastar.

**Arquitetura:** o quadro é uma **projeção** do recurso `occurrence`, não um recurso novo. Não há tabela, coluna, endpoint agregador nem abstração de "quadro" no backend. Cada coluna é uma chamada independente ao `GET /api/occurrences` que já existe, com `stage_id` fixo, e o `total` da resposta vira a contagem do cabeçalho. A única alteração de backend da fase inteira é um parâmetro novo, `closed_since`.

**Stack:** Go 1.25 + fastglue + GORM (backend); Vue 3 `<script setup>` + TypeScript + Pinia + Tailwind + reka-ui (frontend); Playwright (E2E).

**Spec:** `docs/superpowers/specs/2026-08-28-crm-kanban-design.md` (aprovada; §3 corrigida em `8e6d6d7`).

## Restrições globais

Valem para **todas** as tarefas. Cada tarefa herda esta seção implicitamente.

- **Nenhuma tabela nova, nenhuma coluna nova, nenhum endpoint novo.** `closed_since` é a única alteração de backend da fase.
- **Sem `position`, sem ordenação manual dentro da coluna.** A ordem é `opened_at DESC`, igual à lista.
- **Sem SLA, sem WebSocket, sem reordenar colunas arrastando.**
- **Não alterar** `visibleOccurrences`, `resolveAssignee`, `loadAuthorizedOccurrence` nem qualquer lógica de visibilidade.
- **Não tocar** em `sla_processor.go`, `chatbot_processor.go`, `agent_transfers.go`, `conversation_notes.go`, `messages.go`, `contacts.go`, `conversation_visibility.go`, `occurrence_protocol.go`, `occurrence_send.go`.
- **Nada novo é instalado.** `vuedraggable@4.1.0` já é dependência (`frontend/package.json:49`), com peer `vue ^3.0.1`. Esta fase é o primeiro uso dela no repositório.
- **Toda string visível existe nos dois locales**, `pt-BR.json` e `en.json`, mantidos paralelos.
- Os testes de banco em Go **pulam em silêncio** sem `TEST_DATABASE_URL` **e** `TEST_REDIS_URL`, e a suíte reporta `ok` sem ter rodado nada. Sempre passe as duas.
- **Não rode `npm run lint`** — ele tem `--fix` e reescreve o repositório inteiro. Use `npx eslint <arquivos>`.
- Erro pré-existente único e aceitável no `typecheck`: `AccountDetailView.vue(172,45) business_calling_enabled`. Qualquer outro erro é seu.

## Estrutura de arquivos

| Arquivo | Responsabilidade | Tarefa |
|---|---|---|
| `internal/handlers/occurrences.go` | modificar: parse de `closed_since` em `ListOccurrences` | T1 |
| `internal/handlers/occurrences_test.go` | modificar: testes do filtro | T1 |
| `frontend/src/composables/useOccurrenceViewMode.ts` | criar: a preferência lista/quadro em `localStorage` | T2 |
| `frontend/src/views/crm/OccurrencesView.vue` | modificar: seletor de modo e troca de projeção | T2, T3 |
| `frontend/src/components/crm/OccurrenceCard.vue` | criar: o cartão | T3 |
| `frontend/src/components/crm/OccurrenceBoard.vue` | criar: colunas, carga paralela, erro por coluna, carregar mais | T3; arrastar em T4 |
| `frontend/src/stores/occurrences.ts` | modificar: `fetchColumn` (T3) e `moveStage` (T4) | T3, T4 |
| `frontend/src/i18n/locales/{pt-BR,en}.json` | modificar: strings de cada tarefa | T2, T3, T4 |
| `frontend/e2e/pages/OccurrencesPage.ts` | modificar: seletores do quadro | T2, T3, T4 |
| `frontend/e2e/tests/crm/occurrence-board.spec.ts` | criar: cobertura do quadro | T2, T3, T4 |

`frontend/src/components/crm/` ainda não existe e nasce na T3.

---

## Task 1: Backend — o parâmetro `closed_since`

**Files:**
- Modify: `internal/handlers/occurrences.go` (dentro de `ListOccurrences`, logo após o bloco do `open`)
- Test: `internal/handlers/occurrences_test.go`

**Interfaces:**
- Consumes: nada de tarefas anteriores.
- Produces: `GET /api/occurrences?closed_since=<RFC3339>` filtrando por `closed_at IS NOT NULL AND closed_at >= <valor>`, com **400** quando o valor não faz parse. O frontend depende desse contrato a partir da T3.

### Contexto que você precisa

`ListOccurrences` hoje aceita `stage_id`, `contact_id`, `protocol` e `open`. O bloco do `open` é assim:

```go
	if open := string(r.RequestCtx.QueryArgs().Peek("open")); open == "true" {
		query = query.Where("occurrences.closed_at IS NULL")
	}
```

Repare que ele testa `open == "true"`. Portanto **`open=false` não significa "só fechadas"** — significa *sem filtro nenhum*, e traz a lista inteira. É por isso que a coluna de fechamento do quadro usa `closed_since` sozinho, nunca `open=false`.

**Uma divergência deliberada do padrão da casa.** `audit_logs.go:56` ignora em silêncio uma data que não faz parse. Aqui isso seria perigoso: descartar `closed_since` calado transformaria a coluna "fechadas nos últimos 7 dias" na lista inteira, incluindo as abertas — exatamente o quadro errado que a spec §3 alerta. Este filtro **rejeita com 400**.

`time` já está importado em `occurrences.go`. No arquivo de teste você precisa acrescentá-lo.

- [ ] **Step 1: Escreva os testes que falham**

Acrescente ao final de `internal/handlers/occurrences_test.go`, e acrescente `"time"` ao bloco de imports:

```go
// A borda é inclusiva: fechada exatamente no instante do corte entra.
// O relógio é fixo no teste para o caso de borda não ficar intermitente.
func TestOccurrences_ClosedSinceIncludesBoundary(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
	contact := testutil.CreateTestContact(t, app.DB, org.ID)
	stage, err := app.InitialStageForTest(org.ID)
	require.NoError(t, err)

	cut := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)

	occ := models.Occurrence{
		OrganizationID: org.ID, ContactID: contact.ID, Title: "Na borda",
		StageID: stage.ID, OpenedByUserID: user.ID,
	}
	require.NoError(t, app.CreateOccurrenceForTest(&occ))
	require.NoError(t, app.DB.Model(&models.Occurrence{}).
		Where("id = ?", occ.ID).Update("closed_at", cut).Error)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetQueryParam(req, "closed_since", cut.Format(time.RFC3339))
	require.NoError(t, app.ListOccurrences(req))
	require.Equal(t, fasthttp.StatusOK, testutil.GetResponseStatusCode(req))

	var resp struct {
		Data struct {
			Occurrences []struct {
				ID string `json:"id"`
			} `json:"occurrences"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
	require.Len(t, resp.Data.Occurrences, 1)
	assert.Equal(t, occ.ID.String(), resp.Data.Occurrences[0].ID)
}

// Fechada antes do corte fica de fora.
func TestOccurrences_ClosedSinceExcludesOlder(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
	contact := testutil.CreateTestContact(t, app.DB, org.ID)
	stage, err := app.InitialStageForTest(org.ID)
	require.NoError(t, err)

	cut := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)

	occ := models.Occurrence{
		OrganizationID: org.ID, ContactID: contact.ID, Title: "Velha",
		StageID: stage.ID, OpenedByUserID: user.ID,
	}
	require.NoError(t, app.CreateOccurrenceForTest(&occ))
	require.NoError(t, app.DB.Model(&models.Occurrence{}).
		Where("id = ?", occ.ID).Update("closed_at", cut.Add(-time.Second)).Error)

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetQueryParam(req, "closed_since", cut.Format(time.RFC3339))
	require.NoError(t, app.ListOccurrences(req))

	var resp struct {
		Data struct {
			Occurrences []struct {
				ID string `json:"id"`
			} `json:"occurrences"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
	assert.Empty(t, resp.Data.Occurrences)
}

// Aberta (closed_at NULL) nunca entra, por mais antigo que seja o corte.
func TestOccurrences_ClosedSinceExcludesOpen(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
	contact := testutil.CreateTestContact(t, app.DB, org.ID)
	stage, err := app.InitialStageForTest(org.ID)
	require.NoError(t, err)

	occ := models.Occurrence{
		OrganizationID: org.ID, ContactID: contact.ID, Title: "Aberta",
		StageID: stage.ID, OpenedByUserID: user.ID,
	}
	require.NoError(t, app.CreateOccurrenceForTest(&occ))

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetQueryParam(req, "closed_since", "2000-01-01T00:00:00Z")
	require.NoError(t, app.ListOccurrences(req))

	var resp struct {
		Data struct {
			Occurrences []struct {
				ID string `json:"id"`
			} `json:"occurrences"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
	assert.Empty(t, resp.Data.Occurrences)
}

// Valor impossível de interpretar é recusado, não ignorado: ignorar
// transformaria a coluna de fechadas na lista inteira.
func TestOccurrences_ClosedSinceRejectsInvalidValue(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetQueryParam(req, "closed_since", "ontem")
	require.NoError(t, app.ListOccurrences(req))
	assert.Equal(t, fasthttp.StatusBadRequest, testutil.GetResponseStatusCode(req))
}

// closed_since combina com stage_id: cada coluna do quadro é uma etapa.
func TestOccurrences_ClosedSinceCombinesWithStageFilter(t *testing.T) {
	app := newTestApp(t)
	org := testutil.CreateTestOrganization(t, app.DB)
	adminRole := testutil.CreateAdminRole(t, app.DB, org.ID)
	user := testutil.CreateTestUser(t, app.DB, org.ID, testutil.WithRoleID(&adminRole.ID))
	contact := testutil.CreateTestContact(t, app.DB, org.ID)
	require.NoError(t, app.EnsureDefaultStagesForTest(org.ID))

	var stages []models.OccurrenceStage
	require.NoError(t, app.DB.Where("organization_id = ?", org.ID).
		Order("position ASC").Find(&stages).Error)
	require.GreaterOrEqual(t, len(stages), 2)
	wanted, other := stages[0], stages[1]

	cut := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)
	closedAt := cut.Add(time.Hour)

	for _, s := range []models.OccurrenceStage{wanted, other} {
		occ := models.Occurrence{
			OrganizationID: org.ID, ContactID: contact.ID, Title: "Fechada em " + s.Name,
			StageID: s.ID, OpenedByUserID: user.ID,
		}
		require.NoError(t, app.CreateOccurrenceForTest(&occ))
		require.NoError(t, app.DB.Model(&models.Occurrence{}).
			Where("id = ?", occ.ID).Update("closed_at", closedAt).Error)
	}

	req := testutil.NewGETRequest(t)
	testutil.SetAuthContext(req, org.ID, user.ID)
	testutil.SetQueryParam(req, "closed_since", cut.Format(time.RFC3339))
	testutil.SetQueryParam(req, "stage_id", wanted.ID.String())
	require.NoError(t, app.ListOccurrences(req))

	var resp struct {
		Data struct {
			Occurrences []struct {
				StageID string `json:"stage_id"`
			} `json:"occurrences"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(testutil.GetResponseBody(req), &resp))
	require.Len(t, resp.Data.Occurrences, 1)
	assert.Equal(t, wanted.ID.String(), resp.Data.Occurrences[0].StageID)
}
```

- [ ] **Step 2: Rode e confirme que falham**

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc" && TEST_DATABASE_URL='postgres://postgres:postgres@127.0.0.1:5432/whatomate_test?sslmode=disable' TEST_REDIS_URL='redis://127.0.0.1:6379/1' go test ./internal/handlers/ -run 'TestOccurrences_ClosedSince' -count=1 -v
```

Esperado: os cinco FALHAM. Os quatro primeiros porque o parâmetro é ignorado e a ocorrência aparece (ou não) errado; `RejectsInvalidValue` porque devolve 200 em vez de 400.

Se aparecer `ok` com `no tests to run` ou tudo passando de primeira, **as variáveis de ambiente não chegaram** — confira as duas antes de seguir.

- [ ] **Step 3: Implemente o filtro**

Em `internal/handlers/occurrences.go`, logo **depois** do bloco do `open` e **antes** do `var total int64`:

```go
	// O quadro pede os casos fechados a partir de um corte que o cliente
	// calcula e envia absoluto. Ao contrário dos filtros de audit_logs, um
	// valor ilegível é recusado em vez de ignorado: descartá-lo em silêncio
	// transformaria a coluna "fechadas recentemente" na lista inteira.
	if v := string(r.RequestCtx.QueryArgs().Peek("closed_since")); v != "" {
		since, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest,
				"closed_since must be an RFC3339 timestamp", nil, "")
		}
		query = query.Where("occurrences.closed_at IS NOT NULL AND occurrences.closed_at >= ?", since)
	}
```

- [ ] **Step 4: Rode e confirme que passam**

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc" && TEST_DATABASE_URL='postgres://postgres:postgres@127.0.0.1:5432/whatomate_test?sslmode=disable' TEST_REDIS_URL='redis://127.0.0.1:6379/1' go test ./internal/handlers/ -run 'TestOccurrence' -count=1
```

Esperado: `ok`. Os cinco novos passam **e** os 29 anteriores continuam passando.

- [ ] **Step 5: Teste de mutação no filtro**

Prove que o teste de borda realmente segura a condição. Troque temporariamente o `>=` por `>`:

```go
		query = query.Where("occurrences.closed_at IS NOT NULL AND occurrences.closed_at > ?", since)
```

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc" && TEST_DATABASE_URL='postgres://postgres:postgres@127.0.0.1:5432/whatomate_test?sslmode=disable' TEST_REDIS_URL='redis://127.0.0.1:6379/1' go test ./internal/handlers/ -run 'TestOccurrences_ClosedSinceIncludesBoundary' -count=1
```

Esperado: **FALHA**. Se passar, o teste de borda não vale nada e precisa ser corrigido antes de seguir.

Agora remova a segunda condição inteira:

```go
		query = query.Where("occurrences.closed_at IS NOT NULL")
```

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc" && TEST_DATABASE_URL='postgres://postgres:postgres@127.0.0.1:5432/whatomate_test?sslmode=disable' TEST_REDIS_URL='redis://127.0.0.1:6379/1' go test ./internal/handlers/ -run 'TestOccurrences_ClosedSinceExcludesOlder' -count=1
```

Esperado: **FALHA**. Depois **restaure a versão correta do Step 3** e rode o Step 4 de novo para confirmar que voltou ao verde.

- [ ] **Step 6: Build, vet e commit**

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc" && go build ./... && go vet ./...
```

```bash
git add internal/handlers/occurrences.go internal/handlers/occurrences_test.go && git commit -m "feat(crm): filter occurrences by closed_since for the board's closing columns"
```

---

## Task 2: O seletor de modo lista/quadro

**Files:**
- Create: `frontend/src/composables/useOccurrenceViewMode.ts`
- Modify: `frontend/src/views/crm/OccurrencesView.vue`
- Modify: `frontend/src/i18n/locales/pt-BR.json`, `frontend/src/i18n/locales/en.json`
- Modify: `frontend/e2e/pages/OccurrencesPage.ts`
- Test: `frontend/e2e/tests/crm/occurrence-board.spec.ts` (criar)

**Interfaces:**
- Consumes: nada da T1.
- Produces: `useOccurrenceViewMode(): { mode: Ref<'list' | 'board'> }`, exportando também o tipo `OccurrenceViewMode`. A T3 substitui o marcador do modo quadro pelo componente real.

### Contexto que você precisa

O quadro **não ganha rota própria**. `/crm/occurrences` recebe um seletor no cabeçalho e troca de projeção na mesma tela.

A preferência vai para `localStorage`, seguindo `useColorMode`, `useDateRange` e `useResizablePanel`. Leia `frontend/src/composables/useResizablePanel.ts:38-59` antes de escrever: ele mostra o padrão da casa de `try/catch` em volta de **toda** leitura e escrita, porque em janela privada o acessador lança em vez de devolver vazio.

Use `ToggleGroup` (`@/components/ui/toggle-group`), não um `Select`. Dois motivos: um grupo de dois botões é o controle certo para duas opções mutuamente exclusivas, e a tela já tem um `Select` de etapa — o page object dos E2E indexa comboboxes por ordem no DOM (`OccurrencesPage.ts:36-37`), então acrescentar outro combobox é criar fragilidade à toa.

- [ ] **Step 1: Escreva o teste E2E que falha**

Crie `frontend/e2e/tests/crm/occurrence-board.spec.ts`. Os imports e o `beforeEach` seguem exatamente `frontend/e2e/tests/crm/occurrences.spec.ts` — `test` e `expect` vêm do Playwright, não do `framework`, e o `framework` só fornece o `createTestScope`:

```ts
import { test, expect } from '@playwright/test'
import { loginAsAdmin } from '../../helpers'
import { ApiHelper } from '../../helpers/api'
import { createTestScope } from '../../framework'
import { ChatPage, OccurrencesPage } from '../../pages'

const scope = createTestScope('crm-board')

/** Um contato por teste: a suíte roda em paralelo e contagens por organização
 * ficariam intermitentes, mas um painel por contato e um protocolo único não. */
async function createContact(api: ApiHelper): Promise<string> {
  await api.loginAsAdmin()
  const contact = await api.createContact(scope.phone(), scope.name('contact'))
  return contact.id
}

test.describe('CRM occurrence board', () => {
  let occurrencesPage: OccurrencesPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    occurrencesPage = new OccurrencesPage(page)
  })

  test('the view mode preference survives a reload', async ({ page }) => {
    await occurrencesPage.gotoList()
    await expect(occurrencesPage.listView).toBeVisible()

    await occurrencesPage.switchToBoard()
    await expect(occurrencesPage.boardView).toBeVisible()

    await page.reload()
    await page.waitForLoadState('networkidle')
    await expect(occurrencesPage.boardView).toBeVisible()

    await occurrencesPage.switchToList()
    await page.reload()
    await page.waitForLoadState('networkidle')
    await expect(occurrencesPage.listView).toBeVisible()
  })
})
```

Use sempre `scope.name(...)` para nomear o que os testes criam. É o prefixo por onde a limpeza global encontra e apaga os registros; nomes montados à mão com `Date.now()` vazam para o banco.

Acrescente ao `frontend/e2e/pages/OccurrencesPage.ts`, dentro da classe:

```ts
  // --- Lista e quadro ---

  readonly listView: Locator
  readonly boardView: Locator
```

no bloco de campos, e no `constructor`:

```ts
    this.listView = page.locator('#occurrences-list')
    this.boardView = page.locator('#occurrences-board')
```

e como métodos:

```ts
  async gotoList() {
    await this.page.goto('/crm/occurrences')
    await this.page.waitForLoadState('networkidle')
  }

  async switchToBoard() {
    await this.page.getByRole('radio', { name: 'Board' }).click()
  }

  async switchToList() {
    await this.page.getByRole('radio', { name: 'List' }).click()
  }
```

- [ ] **Step 2: Rode e confirme que falha**

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc/frontend" && npx playwright test e2e/tests/crm/occurrence-board.spec.ts --workers=1
```

Esperado: FALHA em `#occurrences-list` não existir. O backend precisa estar de pé apontando para o banco que o `global-setup` semeia — se os testes falharem em massa por dados ausentes, é ambiente, não código.

- [ ] **Step 3: Crie o composable**

`frontend/src/composables/useOccurrenceViewMode.ts`:

```ts
import { ref, watch, type Ref } from 'vue'

export type OccurrenceViewMode = 'list' | 'board'

const STORAGE_KEY = 'occurrences:view-mode'

/**
 * A preferência lista/quadro, lembrada por dispositivo. Modo de exibição é o
 * tipo de preferência que faz sentido variar entre a mesa e o celular, então
 * ela vive no localStorage e não no perfil do usuário.
 *
 * Qualquer coisa ilegível lá cai na lista, que é o modo que existia antes do
 * quadro. Leitura e escrita ficam dentro de try/catch porque em janela privada
 * o próprio acessador lança, em vez de devolver vazio.
 */
export function useOccurrenceViewMode(): { mode: Ref<OccurrenceViewMode> } {
  const mode = ref<OccurrenceViewMode>('list')

  try {
    const saved = localStorage.getItem(STORAGE_KEY)
    if (saved === 'list' || saved === 'board') {
      mode.value = saved
    }
  } catch {
    // Entrada ausente ou bloqueada — o padrão acima já vale.
  }

  watch(mode, value => {
    try {
      localStorage.setItem(STORAGE_KEY, value)
    } catch {
      // Modo privado ou cota estourada — a preferência só não sobrevive.
    }
  })

  return { mode }
}
```

- [ ] **Step 4: Acrescente as strings nos dois locales**

Dentro do bloco `"occurrences"` de `frontend/src/i18n/locales/en.json`:

```json
    "viewList": "List",
    "viewBoard": "Board",
    "viewModeLabel": "View mode",
```

E no mesmo bloco de `frontend/src/i18n/locales/pt-BR.json`:

```json
    "viewList": "Lista",
    "viewBoard": "Quadro",
    "viewModeLabel": "Modo de exibição",
```

- [ ] **Step 5: Ligue o seletor na tela**

Em `frontend/src/views/crm/OccurrencesView.vue`, acrescente aos imports do `<script setup>`:

```ts
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { useOccurrenceViewMode } from '@/composables/useOccurrenceViewMode'
import { List, LayoutGrid } from 'lucide-vue-next'
```

e depois de `const store = useOccurrencesStore()`:

```ts
const { mode } = useOccurrenceViewMode()
```

No template, dentro do `CardHeader` que já abriga os filtros, ao lado do `Select` de etapa, acrescente o seletor. O filtro de etapa **só aparece no modo lista** — num quadro de etapas, um seletor de etapa não significa nada:

```vue
              <Select v-if="mode === 'list'" v-model="stageFilter" @update:model-value="onStageFilterChange">
```

(é o `Select` que já existe; só ganha o `v-if`)

E logo depois do fechamento dele:

```vue
              <ToggleGroup v-model="mode" type="single" :aria-label="$t('occurrences.viewModeLabel')">
                <ToggleGroupItem value="list" :aria-label="$t('occurrences.viewList')">
                  <List class="h-4 w-4 mr-1.5" />
                  {{ $t('occurrences.viewList') }}
                </ToggleGroupItem>
                <ToggleGroupItem value="board" :aria-label="$t('occurrences.viewBoard')">
                  <LayoutGrid class="h-4 w-4 mr-1.5" />
                  {{ $t('occurrences.viewBoard') }}
                </ToggleGroupItem>
              </ToggleGroup>
```

Envolva a `DataTable` que já existe num contêiner identificável e acrescente o marcador do quadro logo depois:

```vue
            <div v-if="mode === 'list'" id="occurrences-list">
              <DataTable ... />
            </div>
            <div v-else id="occurrences-board" class="p-4 text-sm text-muted-foreground">
              {{ $t('occurrences.viewBoard') }}
            </div>
```

Mantenha a `DataTable` exatamente como está, com todos os `<template #cell-...>` e o `@page-change`. Ela só passa a morar dentro da `div`.

O marcador é provisório: a T3 o substitui pelo componente real, preservando o `id`.

- [ ] **Step 6: Verifique**

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc/frontend" && npm run typecheck && npm run i18n:keys && npx eslint src/composables/useOccurrenceViewMode.ts src/views/crm/OccurrencesView.vue && npm run build
```

Esperado: passa, com o único erro conhecido do `AccountDetailView`.

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc/frontend" && npx playwright test e2e/tests/crm/occurrence-board.spec.ts --workers=1
```

Esperado: PASSA.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/composables/useOccurrenceViewMode.ts frontend/src/views/crm/OccurrencesView.vue frontend/src/i18n/locales/pt-BR.json frontend/src/i18n/locales/en.json frontend/e2e/pages/OccurrencesPage.ts frontend/e2e/tests/crm/occurrence-board.spec.ts && git commit -m "feat(crm): add a list/board view toggle to the occurrences screen"
```

---

## Task 3: O quadro estático — colunas, contagens e carregar mais

**Files:**
- Create: `frontend/src/components/crm/OccurrenceCard.vue`
- Create: `frontend/src/components/crm/OccurrenceBoard.vue`
- Modify: `frontend/src/stores/occurrences.ts`
- Modify: `frontend/src/views/crm/OccurrencesView.vue`
- Modify: `frontend/src/i18n/locales/pt-BR.json`, `frontend/src/i18n/locales/en.json`
- Modify: `frontend/e2e/pages/OccurrencesPage.ts`
- Test: `frontend/e2e/tests/crm/occurrence-board.spec.ts`

**Interfaces:**
- Consumes: `closed_since` da T1; `mode` da T2.
- Produces: `fetchColumn(params: Record<string, string>): Promise<{ occurrences: Occurrence[]; total: number }>` no store; `OccurrenceCard.vue` com props `{ occurrence: Occurrence; disabled?: boolean }`; `OccurrenceBoard.vue` com prop `{ protocol?: string }`. A T4 acrescenta arrastar a este mesmo componente.

### Contexto que você precisa

**Por que o store ganha uma função nova.** `fetchOccurrences` escreve em `occurrences.value`, um array **único e compartilhado** que a lista possui. O quadro tem N colunas carregando em paralelo; se todas escrevessem ali, a última resposta apagaria as outras. `fetchColumn` **devolve** a página em vez de guardá-la, e cada coluna fica com o próprio estado no componente.

**Regra de cada coluna** (§5 da spec), escrita explicitamente porque uma leitura errada produz o quadro errado:

| Coluna | Parâmetros |
|---|---|
| Etapa normal (`is_closing = false`) | `stage_id=<etapa>` + `open=true` |
| Etapa de fechamento (`is_closing = true`) | `stage_id=<etapa>` + `closed_since=<agora − 7 dias>` |

**Pode haver mais de uma etapa de fechamento.** A Fase 1 exige *pelo menos uma*, não exatamente uma — uma organização pode ter "Resolvido" e "Cancelado". A regra vale para **cada** etapa com `is_closing = true`. Nenhuma coluna é tratada como especial.

**Nunca use `open=false`** para montar a coluna de fechamento. O handler testa `open == "true"`, então `open=false` é *sem filtro* e traria a lista inteira. `closed_since` sozinho já implica fechada, pela primeira metade da condição.

Os 7 dias são **constante do frontend**, e o valor enviado é um instante absoluto em RFC3339 calculado pelo relógio do cliente. O backend não sabe o que são "7 dias" — ele recebe uma data e compara.

**Carga paralela com falha isolada.** As chamadas saem juntas, e o tempo de abertura é o da mais lenta, não a soma. Se uma coluna falhar, ela mostra o próprio erro com "tentar de novo" e as outras seguem funcionando. Um quadro inteiro derrubado por uma requisição é pior que um quadro com uma coluna avisando que falhou.

As chaves de prioridade no i18n são **camelCase**: `priorityLow`, `priorityNormal`, `priorityHigh`, `priorityUrgent`. Não existe `priority_low`.

- [ ] **Step 1: Escreva os testes E2E que falham**

Acrescente ao `frontend/e2e/tests/crm/occurrence-board.spec.ts`, dentro do `describe`:

```ts
  test('the board shows one column per stage with its own count', async () => {
    await occurrencesPage.gotoList()
    await occurrencesPage.switchToBoard()

    await expect(occurrencesPage.boardView).toBeVisible()
    // ensureDefaultStages semeia quatro etapas, com nomes em português
    // independentemente do idioma da interface.
    await expect(occurrencesPage.boardColumns).toHaveCount(4)
    await expect(occurrencesPage.boardColumn('Aberto')).toBeVisible()
    await expect(occurrencesPage.boardColumn('Resolvido')).toBeVisible()
  })

  test('the stage filter is gone in board mode', async () => {
    await occurrencesPage.gotoList()
    await expect(occurrencesPage.stageFilterSelect).toBeVisible()

    await occurrencesPage.switchToBoard()
    await expect(occurrencesPage.stageFilterSelect).toBeHidden()
  })

  test('a column with more than a page of occurrences loads more', async ({ request }) => {
    const api = new ApiHelper(request)
    const contactId = await createContact(api)

    // 26 abertas na etapa inicial: uma a mais que a página de 25.
    for (let i = 0; i < 26; i++) {
      const res = await api.post('/api/occurrences', {
        contact_id: contactId,
        title: scope.name(`bulk-${i}`),
      })
      if (!res.ok()) throw new Error(`Failed to create occurrence: ${await res.text()}`)
    }

    await occurrencesPage.gotoList()
    await occurrencesPage.switchToBoard()

    const column = occurrencesPage.boardColumn('Aberto')
    const cards = column.locator('[data-board-card]')

    // A primeira página é sempre 25, pelo limite, havendo pelo menos isso.
    await expect(cards).toHaveCount(25)

    await column.getByRole('button', { name: 'Load More' }).click()

    // Depois de carregar mais, passa de 25. Não asserimos um número exato: a
    // coluna é da organização inteira e a suíte roda em paralelo, então
    // qualquer contagem fechada ficaria intermitente.
    await expect.poll(async () => cards.count()).toBeGreaterThan(25)
  })
```

O rótulo do botão é `Load More`, com o M maiúsculo — é a chave `occurrences.loadMore` que já existe no locale.

Acrescente ao `OccurrencesPage.ts`, nos campos:

```ts
  readonly boardColumns: Locator
  readonly stageFilterSelect: Locator
```

no `constructor`:

```ts
    this.boardColumns = page.locator('[data-board-column]')
    this.stageFilterSelect = page.locator('#occurrences-stage-filter')
```

e como método:

```ts
  boardColumn(stageName: string): Locator {
    return this.page.locator(`[data-board-column="${stageName}"]`)
  }

  boardCard(protocol: string): Locator {
    return this.page.locator('[data-board-card]').filter({ hasText: protocol })
  }
```

Para o `#occurrences-stage-filter` funcionar, acrescente esse `id` ao `SelectTrigger` do filtro de etapa em `OccurrencesView.vue`.

- [ ] **Step 2: Rode e confirme que falham**

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc/frontend" && npx playwright test e2e/tests/crm/occurrence-board.spec.ts --workers=1
```

Esperado: o teste da preferência (T2) continua passando; os dois novos FALHAM por não existir nenhum `[data-board-column]`.

- [ ] **Step 3: Acrescente `fetchColumn` ao store**

Em `frontend/src/stores/occurrences.ts`, depois de `fetchOccurrences`:

```ts
  // O quadro carrega cada coluna de forma independente e em paralelo, então
  // isto devolve a página em vez de escrever o array `occurrences`, que a
  // lista possui. Se as colunas escrevessem lá, a última resposta a chegar
  // apagaria as outras.
  async function fetchColumn(params: Record<string, string>) {
    const res = await occurrencesService.list(params)
    return { occurrences: res.data.data.occurrences, total: res.data.data.total }
  }
```

e acrescente `fetchColumn` ao objeto devolvido, junto de `fetchOccurrences`.

- [ ] **Step 4: Crie o cartão**

`frontend/src/components/crm/OccurrenceCard.vue`:

```vue
<script setup lang="ts">
import { Badge } from '@/components/ui/badge'
import { useOccurrencesStore } from '@/stores/occurrences'
import type { Occurrence } from '@/services/api'

defineProps<{ occurrence: Occurrence; disabled?: boolean }>()

const store = useOccurrencesStore()

// As chaves do i18n são camelCase; não existe `priority_low`.
const PRIORITY_KEY = {
  low: 'occurrences.priorityLow',
  normal: 'occurrences.priorityNormal',
  high: 'occurrences.priorityHigh',
  urgent: 'occurrences.priorityUrgent',
} as const
</script>

<template>
  <div
    data-board-card
    class="rounded-md border border-white/[0.08] light:border-gray-200 bg-white/[0.02] light:bg-white p-3"
    :class="disabled ? 'opacity-50 cursor-wait' : 'cursor-grab'"
  >
    <div class="flex items-center justify-between gap-2">
      <span class="font-mono text-xs text-white/50 light:text-muted-foreground">
        {{ occurrence.protocol_number }}
      </span>
      <Badge
        variant="outline"
        class="shrink-0 text-xs"
        :style="{ borderColor: store.stageColor(occurrence.stage_id), color: store.stageColor(occurrence.stage_id) }"
      >
        {{ $t(PRIORITY_KEY[occurrence.priority]) }}
      </Badge>
    </div>
    <p class="text-sm mt-1 truncate text-white light:text-gray-900">{{ occurrence.title }}</p>
    <p class="text-xs mt-1 truncate text-white/50 light:text-muted-foreground">{{ occurrence.contact_name }}</p>
    <p class="text-xs mt-0.5 truncate text-white/40 light:text-muted-foreground">
      {{ occurrence.assigned_user_name || $t('occurrences.unassigned') }}
    </p>
  </div>
</template>
```

- [ ] **Step 5: Crie o quadro**

`frontend/src/components/crm/OccurrenceBoard.vue`:

```vue
<script setup lang="ts">
import { ref, watch, onMounted } from 'vue'
import { Button } from '@/components/ui/button'
import { Spinner } from '@/components/ui/spinner'
import { useOccurrencesStore } from '@/stores/occurrences'
import type { Occurrence, OccurrenceStage } from '@/services/api'
import OccurrenceCard from './OccurrenceCard.vue'

const props = defineProps<{ protocol?: string }>()

const store = useOccurrencesStore()

/** Cartões por página, em cada coluna. */
const PAGE_SIZE = 25
/** Janela da coluna de fechamento. Constante desta fase, não configuração. */
const CLOSED_WINDOW_DAYS = 7

interface ColumnState {
  stage: OccurrenceStage
  items: Occurrence[]
  total: number
  page: number
  loading: boolean
  failed: boolean
}

const columns = ref<ColumnState[]>([])

/**
 * Etapa normal mostra as abertas; etapa de fechamento mostra as fechadas na
 * janela recente. Nunca use `open=false` para a segunda: o handler testa
 * `open == "true"`, então `false` significa *sem filtro* e traria tudo.
 *
 * O corte vai absoluto, em RFC3339, calculado aqui — o backend não sabe o que
 * são "7 dias", ele recebe uma data e compara.
 */
function columnParams(stage: OccurrenceStage, page: number): Record<string, string> {
  const params: Record<string, string> = {
    stage_id: stage.id,
    page: String(page),
    limit: String(PAGE_SIZE),
  }

  if (stage.is_closing) {
    params.closed_since = new Date(Date.now() - CLOSED_WINDOW_DAYS * 24 * 60 * 60 * 1000).toISOString()
  } else {
    params.open = 'true'
  }

  if (props.protocol) params.protocol = props.protocol

  return params
}

async function loadColumn(col: ColumnState, page: number) {
  col.loading = true
  col.failed = false
  try {
    const { occurrences, total } = await store.fetchColumn(columnParams(col.stage, page))
    col.items = page === 1 ? occurrences : [...col.items, ...occurrences]
    col.total = total
    col.page = page
  } catch {
    // Falha é isolada por coluna: esta avisa, as outras seguem funcionando.
    col.failed = true
  } finally {
    col.loading = false
  }
}

/**
 * Dispara todas as colunas juntas. O tempo de abertura é o da chamada mais
 * lenta, não a soma. `loadColumn` trata o próprio erro, então o Promise.all
 * nunca rejeita — é isso que mantém o isolamento por coluna.
 */
async function loadAll() {
  columns.value = store.stages.map(stage => ({
    stage,
    items: [],
    total: 0,
    page: 1,
    loading: true,
    failed: false,
  }))
  await Promise.all(columns.value.map(col => loadColumn(col, 1)))
}

function hasMore(col: ColumnState): boolean {
  return col.items.length < col.total
}

onMounted(async () => {
  if (store.stages.length === 0) await store.fetchStages()
  await loadAll()
})

watch(() => props.protocol, loadAll)
</script>

<template>
  <div id="occurrences-board" class="flex gap-4 overflow-x-auto p-4">
    <div
      v-for="col in columns"
      :key="col.stage.id"
      :data-board-column="col.stage.name"
      class="flex w-72 shrink-0 flex-col rounded-lg border border-white/[0.08] light:border-gray-200 bg-white/[0.02] light:bg-gray-50"
    >
      <div class="flex items-center justify-between gap-2 border-b border-white/[0.08] light:border-gray-200 p-3">
        <span class="flex items-center gap-2 text-sm font-medium">
          <span class="h-2.5 w-2.5 rounded-full" :style="{ backgroundColor: col.stage.color }" />
          {{ col.stage.name }}
        </span>
        <span class="text-xs text-muted-foreground">{{ col.total }}</span>
      </div>

      <div class="flex flex-col gap-2 p-2 min-h-24">
        <div v-if="col.failed" class="p-3 text-center">
          <p class="text-xs text-muted-foreground">{{ $t('occurrences.columnLoadFailed') }}</p>
          <Button variant="outline" size="sm" class="mt-2" @click="loadColumn(col, 1)">
            {{ $t('common.retryLoad') }}
          </Button>
        </div>

        <template v-else>
          <OccurrenceCard v-for="occ in col.items" :key="occ.id" :occurrence="occ" />

          <div v-if="col.loading" class="flex justify-center p-3">
            <Spinner class="h-4 w-4" />
          </div>

          <p v-else-if="col.items.length === 0" class="p-3 text-center text-xs text-muted-foreground">
            {{ $t('occurrences.columnEmpty') }}
          </p>

          <Button
            v-if="hasMore(col) && !col.loading"
            variant="ghost"
            size="sm"
            @click="loadColumn(col, col.page + 1)"
          >
            {{ $t('occurrences.loadMore') }}
          </Button>
        </template>
      </div>
    </div>
  </div>
</template>
```

`Spinner` é exportado de `@/components/ui/spinner` com esse nome (`index.ts:1`), então o import acima está correto como está.

- [ ] **Step 6: Acrescente as strings nos dois locales**

**`occurrences.loadMore` já existe** (`en.json:2032`, "Load More") e o quadro a reaproveita. Não crie de novo.

Só duas chaves são novas. Em `en.json`, no bloco `"occurrences"`:

```json
    "columnLoadFailed": "Could not load this column",
    "columnEmpty": "Nothing here",
```

Em `pt-BR.json`:

```json
    "columnLoadFailed": "Não foi possível carregar esta coluna",
    "columnEmpty": "Nada aqui",
```

- [ ] **Step 7: Troque o marcador pelo quadro**

Em `OccurrencesView.vue`, importe:

```ts
import OccurrenceBoard from '@/components/crm/OccurrenceBoard.vue'
```

e substitua a `div` provisória da T2:

```vue
            <OccurrenceBoard v-else :protocol="searchQuery || undefined" />
```

O `id="occurrences-board"` agora vive dentro do componente, então a `div` provisória some inteira.

- [ ] **Step 8: Verifique**

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc/frontend" && npm run typecheck && npm run i18n:keys && npx eslint src/components/crm/OccurrenceBoard.vue src/components/crm/OccurrenceCard.vue src/stores/occurrences.ts src/views/crm/OccurrencesView.vue && npm run build
```

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc/frontend" && npx playwright test e2e/tests/crm/occurrence-board.spec.ts --workers=1
```

Esperado: os três testes passam.

- [ ] **Step 9: Commit**

```bash
git add frontend/src/components/crm frontend/src/stores/occurrences.ts frontend/src/views/crm/OccurrencesView.vue frontend/src/i18n/locales/pt-BR.json frontend/src/i18n/locales/en.json frontend/e2e && git commit -m "feat(crm): render the occurrence board with one column per stage"
```

---

## Task 4: Arrastar entre colunas

**Files:**
- Modify: `frontend/src/components/crm/OccurrenceBoard.vue`
- Modify: `frontend/src/stores/occurrences.ts`
- Modify: `frontend/src/i18n/locales/pt-BR.json`, `frontend/src/i18n/locales/en.json`
- Modify: `frontend/e2e/pages/OccurrencesPage.ts`
- Test: `frontend/e2e/tests/crm/occurrence-board.spec.ts`

**Interfaces:**
- Consumes: tudo da T3.
- Produces: `moveStage(occurrenceId: string, stageId: string): Promise<void>` no store. Nada depende desta tarefa.

### Contexto que você precisa

**Por que o store ganha `moveStage` em vez de reusar `changeStage`.** O `changeStage` que já existe chama `fetchEvents` depois de gravar, porque a tela de detalhe precisa recarregar a timeline. No quadro isso é uma requisição desperdiçada por arrasto **e** sobrescreve `events.value`, que pertence ao detalhe. `moveStage` grava e para por aí.

**A reversão não depende do estado visual.** Ao iniciar o arrasto, guarde explicitamente `{ occurrenceId, fromStageId }`. Se a requisição falhar, o cartão volta usando esses valores guardados — não "de onde ele parece ter vindo". Interface otimista que não desfaz é interface que mente.

**Um cartão com requisição em voo não aceita novo arrasto.** É a regra mais simples que resolve: sem ela, dois movimentos rápidos no mesmo cartão podem chegar fora de ordem e a reversão restaura o estado errado. Nada de sistema de sincronização — só travar aquele cartão enquanto a requisição dele está em voo.

**Soltar na própria coluna é no-op**: nenhuma requisição, nenhum evento de timeline, `closed_at` intocado. Isso sai de graça porque só o evento `added` do vuedraggable dispara ação, e mover dentro da mesma lista emite `moved`, não `added` — mas há também uma guarda explícita comparando as etapas, porque a spec exige o comportamento, não a sorte da biblioteca.

**A ordem é `opened_at DESC`, sempre.** Depois de um movimento bem-sucedido *e* depois de uma reversão, a coluna é reordenada. Não existe posição manual.

- [ ] **Step 1: Escreva os testes E2E que falham**

Acrescente ao `occurrence-board.spec.ts`:

Acrescente `ChatPage` ao `beforeEach` do describe, como faz o spec da Fase 1:

```ts
  let chatPage: ChatPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    occurrencesPage = new OccurrencesPage(page)
    chatPage = new ChatPage(page)
  })
```

E acrescente os testes:

```ts
  test('dragging a card to another column records the change in the timeline', async ({ request }) => {
    const api = new ApiHelper(request)
    const contactId = await createContact(api)
    await chatPage.goto(contactId)

    const title = scope.name('drag')
    await occurrencesPage.createOccurrence(title)
    const protocol = await occurrencesPage.getProtocolNumber(title)

    await occurrencesPage.gotoList()
    await occurrencesPage.switchToBoard()

    await occurrencesPage.dragCardToColumn(protocol, 'Em análise')
    await expect(occurrencesPage.cardInColumn('Em análise', protocol)).toBeVisible()

    await occurrencesPage.boardCard(protocol).click()
    await expect(occurrencesPage.timelineEntry('Stage change')).toBeVisible()
  })

  test('a server failure returns the card to its original column', async ({ page, request }) => {
    const api = new ApiHelper(request)
    const contactId = await createContact(api)
    await chatPage.goto(contactId)

    const title = scope.name('rollback')
    await occurrencesPage.createOccurrence(title)
    const protocol = await occurrencesPage.getProtocolNumber(title)

    await occurrencesPage.gotoList()
    await occurrencesPage.switchToBoard()

    // A reversão só vale provada contra uma falha real do servidor.
    await page.route('**/api/occurrences/*/stage', route =>
      route.fulfill({
        status: 500,
        contentType: 'application/json',
        body: JSON.stringify({ message: 'Stage change refused by test' }),
      }),
    )

    await occurrencesPage.dragCardToColumn(protocol, 'Em análise')

    await occurrencesPage.expectToast(/Stage change refused by test/)
    await expect(occurrencesPage.cardInColumn('Aberto', protocol)).toBeVisible()
    await expect(occurrencesPage.cardInColumn('Em análise', protocol)).toBeHidden()
  })

  // Protege a decisão de §7 da spec: se alguém introduzir ordenação manual
  // sem spec, este teste fica vermelho.
  test('after a drag the column stays ordered by opened_at descending', async ({ request }) => {
    const api = new ApiHelper(request)
    const contactId = await createContact(api)
    await chatPage.goto(contactId)

    const older = scope.name('older')
    const newer = scope.name('newer')
    await occurrencesPage.createOccurrence(older)
    await occurrencesPage.createOccurrence(newer)

    const olderProtocol = await occurrencesPage.getProtocolNumber(older)
    const newerProtocol = await occurrencesPage.getProtocolNumber(newer)

    await occurrencesPage.gotoList()
    await occurrencesPage.switchToBoard()

    // Arrasta as duas para a mesma coluna, a mais velha por último. Se a
    // ordem fosse a de chegada, ela ficaria no fim; por opened_at DESC, a
    // mais nova vem primeiro.
    await occurrencesPage.dragCardToColumn(newerProtocol, 'Em análise')
    await occurrencesPage.dragCardToColumn(olderProtocol, 'Em análise')

    const texts = await occurrencesPage.boardColumn('Em análise')
      .locator('[data-board-card]').allInnerTexts()
    const olderIndex = texts.findIndex(t => t.includes(olderProtocol))
    const newerIndex = texts.findIndex(t => t.includes(newerProtocol))
    expect(newerIndex).toBeGreaterThanOrEqual(0)
    expect(olderIndex).toBeGreaterThan(newerIndex)
  })
```

Acrescente ao `OccurrencesPage.ts`:

```ts
  /** Arrasta um cartão do quadro para a coluna da etapa indicada. */
  async dragCardToColumn(protocol: string, stageName: string) {
    const card = this.boardCard(protocol)
    await expect(card).toBeVisible({ timeout: 10000 })
    await card.dragTo(this.boardColumn(stageName))
  }

  /** O cartão de um protocolo dentro de uma coluna específica. Vazio — logo,
   * `toBeHidden()` — quando o cartão não está naquela coluna. */
  cardInColumn(stageName: string, protocol: string): Locator {
    return this.boardColumn(stageName).locator('[data-board-card]').filter({ hasText: protocol })
  }
```

Não crie um `gotoChat`: o caminho da casa é `chatPage.goto(contactId)`, e o `createOccurrence`/`getProtocolNumber` do page object já cuidam de abrir o painel a partir daí.

Os nomes de etapa saem de `defaultStages` (`internal/handlers/occurrence_protocol.go:14`) e são **em português mesmo com a interface em inglês**, porque são dados semeados no banco: `Aberto`, `Em análise`, `Aguardando cliente`, `Resolvido`. Já `'Stage change'` é rótulo de interface, e a suíte roda em inglês.

- [ ] **Step 2: Rode e confirme que falham**

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc/frontend" && npx playwright test e2e/tests/crm/occurrence-board.spec.ts --workers=1
```

Esperado: os três da T2/T3 passam; os três novos FALHAM porque arrastar ainda não faz nada.

- [ ] **Step 3: Acrescente `moveStage` ao store**

Em `frontend/src/stores/occurrences.ts`, depois de `changeStage`:

```ts
  // O quadro grava a etapa e para por aí. O `changeStage` acima recarrega a
  // timeline porque a tela de detalhe precisa dela; aqui isso seria uma
  // requisição desperdiçada por arrasto e ainda sobrescreveria `events`, que
  // pertence ao detalhe.
  async function moveStage(occurrenceId: string, stageId: string) {
    await occurrencesService.changeStage(occurrenceId, stageId)
  }
```

e acrescente `moveStage` ao objeto devolvido.

- [ ] **Step 4: Implemente o arrastar**

Em `OccurrenceBoard.vue`, acrescente aos imports:

```ts
import draggable from 'vuedraggable'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import { getErrorMessage } from '@/lib/api-utils'
```

e no `<script setup>`, depois de `const store = ...`:

```ts
const { t } = useI18n()

/** Ocorrências com requisição em voo. Um cartão travado não aceita novo arrasto. */
const pending = ref(new Set<string>())

/**
 * A origem do arrasto, capturada no início e usada na reversão. Guardar isto
 * explicitamente é o que permite desfazer sem depender do estado visual.
 */
let dragOrigin: { occurrenceId: string; fromStageId: string } | null = null

function onDragStart(col: ColumnState, evt: { oldIndex: number }) {
  const item = col.items[evt.oldIndex]
  dragOrigin = item ? { occurrenceId: item.id, fromStageId: col.stage.id } : null
}

/** Recusa arrastar um cartão cuja própria requisição ainda está em voo. */
function canMove(evt: { draggedContext: { element: Occurrence } }): boolean {
  return !pending.value.has(evt.draggedContext.element.id)
}

function sortByOpenedAtDesc(items: Occurrence[]) {
  items.sort((a, b) => Date.parse(b.opened_at) - Date.parse(a.opened_at))
}

async function onColumnChange(toCol: ColumnState, evt: { added?: { element: Occurrence } }) {
  // Mover dentro da mesma coluna emite `moved`, não `added`: nenhuma
  // requisição sai e nenhum evento de timeline é criado.
  if (!evt.added) return

  const origin = dragOrigin
  dragOrigin = null
  if (!origin) return

  const occ = evt.added.element

  // Guarda explícita do no-op, exigida pela spec e não deixada por conta do
  // comportamento da biblioteca.
  if (origin.fromStageId === toCol.stage.id) return

  const fromCol = columns.value.find(c => c.stage.id === origin.fromStageId)

  pending.value.add(occ.id)
  try {
    await store.moveStage(occ.id, toCol.stage.id)
    sortByOpenedAtDesc(toCol.items)
    toCol.total += 1
    if (fromCol) fromCol.total = Math.max(0, fromCol.total - 1)
  } catch (e) {
    // Reversão pela origem guardada, não pelo que está na tela.
    const idx = toCol.items.findIndex(i => i.id === occ.id)
    if (idx !== -1) toCol.items.splice(idx, 1)
    if (fromCol) {
      fromCol.items.push(occ)
      sortByOpenedAtDesc(fromCol.items)
    }
    // A mensagem é a que o servidor devolveu, não um erro genérico.
    toast.error(getErrorMessage(e, t('occurrences.stageChangeFailed')))
  } finally {
    pending.value.delete(occ.id)
  }
}
```

No template, troque o bloco dos cartões pelo `draggable`:

```vue
          <draggable
            v-model="col.items"
            :group="{ name: 'occurrences' }"
            :move="canMove"
            item-key="id"
            class="flex flex-col gap-2 min-h-16"
            @start="onDragStart(col, $event)"
            @change="onColumnChange(col, $event)"
          >
            <template #item="{ element }">
              <OccurrenceCard :occurrence="element" :disabled="pending.has(element.id)" />
            </template>
          </draggable>
```

O `v-if="col.loading"`, o vazio e o "carregar mais" ficam **fora** do `draggable`, logo depois dele — só os cartões vão dentro.

Se o TypeScript reclamar de tipos do `vuedraggable`, que não traz definições próprias, crie `frontend/src/types/vuedraggable.d.ts`:

```ts
declare module 'vuedraggable' {
  import type { DefineComponent } from 'vue'
  const draggable: DefineComponent<Record<string, unknown>>
  export default draggable
}
```

- [ ] **Step 5: Nenhuma string nova**

`occurrences.stageChangeFailed` **já existe** (`en.json:2091`, "Failed to update stage"), criada na Fase 1 para o seletor de etapa do detalhe. O quadro a reaproveita: é a mesma falha, na mesma operação. Não crie chave nova e não altere o texto existente.

Este passo existe para você **não** acrescentar nada — siga direto para a verificação.

- [ ] **Step 6: Verifique**

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc/frontend" && npm run typecheck && npm run i18n:keys && npx eslint src/components/crm/OccurrenceBoard.vue src/stores/occurrences.ts && npm run build
```

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc/frontend" && npx playwright test e2e/tests/crm/occurrence-board.spec.ts --workers=1
```

Esperado: os seis testes passam.

- [ ] **Step 7: Suíte E2E completa**

O arrasto mexe na tela de ocorrências, que outros specs também visitam.

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc/frontend" && npx playwright test --workers=1
```

Esperado: nenhuma regressão. As duas falhas pré-existentes conhecidas continuam sendo as únicas aceitáveis. **Se falharem 9 testes de uma vez, suspeite do ambiente antes do código**: o sintoma clássico é o backend apontando para um banco diferente do que o `global-setup` semeia.

- [ ] **Step 8: Confira a limpeza dos E2E**

Esta fase não cria tabela, então `global-cleanup.ts` não deveria precisar de mudança: os testes usam `createTestScope('crm-board')`, e a limpeza já apaga ocorrências por contato e por usuário. Confirme mesmo assim — na Fase 1, tabela nova quebrou a limpeza **duas vezes**, e o sintoma foi contato preso por chave estrangeira.

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc" && docker exec whatc-pg psql -U postgres -d whatomate_e2e -tAc "select count(*) from occurrences where title like '%crm-board%';"
```

Esperado: `0` depois de a suíte terminar. Se sobrar, corrija `global-cleanup.ts`.

- [ ] **Step 9: Commit**

```bash
git add frontend/src frontend/e2e && git commit -m "feat(crm): move occurrences between board columns by dragging"
```

---

## Verificação final da fase

Depois da T4, com tudo verde:

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc" && go build ./... && go vet ./... && TEST_DATABASE_URL='postgres://postgres:postgres@127.0.0.1:5432/whatomate_test?sslmode=disable' TEST_REDIS_URL='redis://127.0.0.1:6379/1' go test ./internal/handlers/ -count=1
```

```bash
cd "C:/Users/Ivan Coelho/Documents/GitHub/whatc/frontend" && npm run typecheck && npm run i18n:keys && npm run build && npx playwright test --workers=1
```

## Riscos

| Risco | Mitigação |
|---|---|
| `open=false` lido como "só fechadas" | A coluna de fechamento usa apenas `closed_since` (T3, Step 5) |
| `closed_since` inválido ignorado, virando a lista inteira | 400 explícito, divergindo do padrão de `audit_logs` (T1) |
| Reversão restaurar estado errado após dois movimentos rápidos | Cartão travado enquanto a requisição dele está em voo (T4) |
| Uma coluna lenta ou com erro derrubar o quadro | Chamadas paralelas, erro tratado dentro de `loadColumn` (T3) |
| Quadro travar com volume | 25 por coluna, "carregar mais", só abertas nas etapas normais |
| Alguém introduzir `position` sem spec | O teste de ordenação por `opened_at` fica vermelho (T4) |
| Colunas escreverem por cima umas das outras | `fetchColumn` devolve em vez de gravar estado compartilhado (T3) |
