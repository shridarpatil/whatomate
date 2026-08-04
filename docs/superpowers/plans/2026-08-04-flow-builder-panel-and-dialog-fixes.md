# Construtor de fluxo e modais — Plano de Implementação

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Consertar os botões "Ficar"/"Sair" do construtor de fluxo, impedir que modais estourem o próprio cartão (e que listas longas fiquem inalcançáveis), e tornar o painel direito do construtor recolhível e redimensionável.

**Architecture:** Três frentes independentes no frontend. (1) Um erro de contrato props/emits corrigido numa linha. (2) Uma blindagem de CSS grid nos dois componentes base de diálogo, mais o ajuste dos três diálogos de seleção do chat que hoje vazam. (3) Um composable novo `useResizablePanel` que guarda largura/colapso em `localStorage`, consumido pelo `ChatbotFlowBuilderView`.

**Tech Stack:** Vue 3 (`<script setup>`, TypeScript), Tailwind CSS, reka-ui (primitivos de Dialog/AlertDialog/ScrollArea), Vue Flow, vue-i18n, Playwright (E2E).

**Spec:** [`docs/superpowers/specs/2026-08-04-flow-builder-panel-and-dialog-fixes-design.md`](../specs/2026-08-04-flow-builder-panel-and-dialog-fixes-design.md)

> **Corrigido durante a execução — este plano não descreve mais o código final.** As revisões acharam defeitos reais no código de exemplo abaixo, e o dono do produto aprovou divergir do plano. O que mudou:
>
> - **Task 4** — `watch([width, collapsed], persist)` gravava no `localStorage` a cada `pointermove`. Agora o watcher pula enquanto `isDragging` e o `onEnd` persiste uma vez. `onHandlePointerDown` também ganhou guarda de re-entrância (`|| isDragging.value`), sem a qual um segundo ponteiro registrava um par de listeners duplicado com base defasada. Depois, `onScopeDispose` passou a persistir um arraste interrompido por desmontagem, `toggle()` limpa `isDragging`, e o handle recebe `focus()` no `pointerdown`.
> - **Task 5** — o separador ganhou `aria-valuenow`/`aria-valuemin`/`aria-valuemax` (padrão *window splitter* do WAI-ARIA), com `320`/`720` extraídos para as constantes `PANEL_MIN_WIDTH`/`PANEL_MAX_WIDTH`. E o mais importante: `onNodeClick` agora chama `expandPanel()` diretamente — o `watch(selectedNodeId)` sozinho só dispara na *mudança* de id, então reclicar o nó já selecionado com o painel recolhido não fazia nada.
> - **Task 2** — `DialogScrollContent.vue` tinha a mesma trilha de grid não blindada e também foi corrigido; o plano só citava dois primitivos.
>
> A fonte da verdade do comportamento é o código, não este arquivo.

## Global Constraints

- **Branch:** `feature/flow-builder-panel-and-dialog-fixes` (já criada a partir de `development`). Nunca commitar em `main` nem em `development`.
- **Diretório de trabalho:** todos os comandos rodam em `frontend/`.
- **Sistema em produção.** Nenhuma refatoração além do descrito. Não tocar em `useUnsavedChangesGuard.ts`. Não adicionar guard de rota ao construtor de fluxo. Não refatorar `ChatbotFlowBuilderView.vue` (961 linhas) nem `ChatView.vue`.
- **Não existe infraestrutura de testes unitários neste projeto.** `vitest` está no `package.json` mas sem config, sem script e sem nenhum teste — **não** monte vitest, isso é escopo alheio. A disciplina de teste do repo é Playwright E2E.
- **i18n:** toda string visível ou `aria-label` novo entra em `src/i18n/locales/pt-BR.json` **e** `src/i18n/locales/en.json`. Os dois arquivos são paralelos linha a linha; mantenha assim.
- **Os testes E2E rodam em inglês**, então os seletores usam os rótulos de `en.json`.
- **Limites do painel:** largura padrão `420`, mínimo `320`, máximo `720`. Chave de `localStorage`: `flow-builder-panel`.

### Como rodar as verificações

```bash
cd frontend && npm run typecheck
```

```bash
cd frontend && npm run lint
```

```bash
cd frontend && npm run i18n:keys
```

Os E2E precisam da stack completa no ar (Postgres + backend Go em `http://localhost:8080`), porque `e2e/global-setup.ts` cria usuários de teste direto no banco:

```bash
cd frontend && BASE_URL=http://localhost:8080 npx playwright test e2e/tests/chatbot/flow-builder.spec.ts --project=chromium
```

**Se a stack não estiver disponível**, escreva e commite os testes assim mesmo, e **diga explicitamente que os E2E não foram executados**. Não afirme que passaram. `typecheck`, `lint` e `i18n:keys` são obrigatórios em toda tarefa e não dependem da stack.

---

## File Structure

| Arquivo | Responsabilidade | Ação |
|---|---|---|
| `src/views/chatbot/ChatbotFlowBuilderView.vue` | Tela do construtor: corrige o contrato do diálogo (Task 1) e consome o painel redimensionável (Task 5) | Modificar |
| `src/components/ui/dialog/DialogContent.vue` | Primitivo base de diálogo: trilha do grid blindada | Modificar |
| `src/components/ui/alert-dialog/AlertDialogContent.vue` | Idem, para alert dialogs | Modificar |
| `src/views/chat/ChatView.vue` | Três diálogos de seleção (responsável, transferir p/ agente, transferir p/ time) | Modificar |
| `src/composables/useResizablePanel.ts` | **Novo.** Estado + persistência + arraste de um painel lateral redimensionável. Única responsabilidade, sem acoplamento com o construtor de fluxo | Criar |
| `src/composables/index.ts` | Barrel de composables | Modificar |
| `src/i18n/locales/pt-BR.json`, `src/i18n/locales/en.json` | Rótulos do painel | Modificar |
| `e2e/pages/FlowsPage.ts` | Page object `ChatbotFlowBuilderPage` — novos locators | Modificar |
| `e2e/tests/chatbot/flow-builder.spec.ts` | Cobertura do diálogo e do painel | Modificar |

---

## Task 1: Consertar os botões "Ficar" e "Sair"

**Contexto:** `UnsavedChangesDialog.vue` declara `props: { open }` e `emits: ['stay', 'leave']`. O construtor de fluxo o consome com `v-model:open` (escuta `update:open`, que nunca é emitido) e `@confirm` (nunca emitido). Resultado: os dois botões são inertes. As outras 11 telas que usam esse componente já estão no contrato correto.

**Files:**
- Modify: `frontend/src/views/chatbot/ChatbotFlowBuilderView.vue:959`
- Modify: `frontend/e2e/pages/FlowsPage.ts` (classe `ChatbotFlowBuilderPage`, a partir da linha 326)
- Test: `frontend/e2e/tests/chatbot/flow-builder.spec.ts`

**Interfaces:**
- Consumes: `cancelDialogOpen: Ref<boolean>` (já existe, linha 92) e `confirmCancel(): void` (já existe, linha 650 — fecha o diálogo e faz `router.push('/chatbot/flows')`).
- Produces: locators `cancelButton`, `unsavedDialog`, `stayButton`, `leaveButton` em `ChatbotFlowBuilderPage`, consumidos pelas Tasks 1 e 5.

- [ ] **Step 1: Adicionar os locators ao page object**

Em `frontend/e2e/pages/FlowsPage.ts`, dentro da classe `ChatbotFlowBuilderPage`, logo depois do método `addNode` (que termina na linha 343), inserir:

```typescript
  /** Header "Cancel" button — opens the unsaved-changes dialog when the flow is dirty. */
  get cancelButton() {
    return this.page.getByRole('button', { name: /^Cancel$/ })
  }

  /** The unsaved-changes alert dialog (reka-ui sets role="alertdialog"). */
  get unsavedDialog() {
    return this.page.getByRole('alertdialog')
  }

  /** "Stay" button inside the unsaved-changes dialog. */
  get stayButton() {
    return this.unsavedDialog.getByRole('button', { name: /^Stay$/ })
  }

  /** "Leave" button inside the unsaved-changes dialog. */
  get leaveButton() {
    return this.unsavedDialog.getByRole('button', { name: /^Leave$/ })
  }
```

- [ ] **Step 2: Escrever os testes que falham**

Em `frontend/e2e/tests/chatbot/flow-builder.spec.ts`, acrescentar ao **final** do arquivo:

```typescript
test.describe('Chatbot Flow Builder - unsaved changes dialog', () => {
  let builder: ChatbotFlowBuilderPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    builder = new ChatbotFlowBuilderPage(page)
    await builder.gotoNew()
    // Adding a node is what marks the flow dirty.
    await builder.addNode('Text')
  })

  test('"Stay" keeps the author in the builder', async ({ page }) => {
    await builder.cancelButton.click()
    await expect(builder.unsavedDialog).toBeVisible()

    await builder.stayButton.click()

    await expect(builder.unsavedDialog).toBeHidden()
    await expect(page).toHaveURL(/\/chatbot\/flows\/new/)
    await expect(builder.paletteToolbar).toBeVisible()
  })

  test('"Leave" navigates back to the flow list', async ({ page }) => {
    await builder.cancelButton.click()
    await expect(builder.unsavedDialog).toBeVisible()

    await builder.leaveButton.click()

    await expect(page).toHaveURL(/\/chatbot\/flows$/)
  })
})
```

- [ ] **Step 3: Rodar os testes e confirmar que falham**

```bash
cd frontend && BASE_URL=http://localhost:8080 npx playwright test e2e/tests/chatbot/flow-builder.spec.ts -g "unsaved changes dialog" --project=chromium
```

Esperado: **2 falhas**. O diálogo abre, mas os cliques em "Stay"/"Leave" não têm efeito — `toBeHidden()` e `toHaveURL()` estouram por timeout.

Se a stack não estiver no ar, registre isso e siga para o Step 4.

- [ ] **Step 4: Corrigir o contrato**

Em `frontend/src/views/chatbot/ChatbotFlowBuilderView.vue`, trocar a linha 959.

De:

```vue
    <UnsavedChangesDialog v-model:open="cancelDialogOpen" @confirm="confirmCancel" />
```

Para:

```vue
    <UnsavedChangesDialog
      :open="cancelDialogOpen"
      @stay="cancelDialogOpen = false"
      @leave="confirmCancel"
    />
```

Nada mais muda. `confirmCancel()` já fecha o diálogo e navega.

- [ ] **Step 5: Rodar os testes e confirmar que passam**

```bash
cd frontend && BASE_URL=http://localhost:8080 npx playwright test e2e/tests/chatbot/flow-builder.spec.ts -g "unsaved changes dialog" --project=chromium
```

Esperado: **2 passes**.

- [ ] **Step 6: Typecheck e lint**

```bash
cd frontend && npm run typecheck && npm run lint
```

Esperado: sem erros.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/views/chatbot/ChatbotFlowBuilderView.vue frontend/e2e/pages/FlowsPage.ts frontend/e2e/tests/chatbot/flow-builder.spec.ts
git commit -m "fix(flow-builder): wire UnsavedChangesDialog to its real stay/leave contract"
```

---

## Task 2: Blindar a trilha do grid nos diálogos base

**Contexto:** `DialogContent` e `AlertDialogContent` são CSS grid sem `grid-template-columns` declarado. A coluna implícita usa a função `auto`, cujo **mínimo** é a maior contribuição mínima entre os itens — e isso **pode ultrapassar a largura do contêiner**. Basta um filho que não encolhe (texto com `white-space: nowrap`, por exemplo) para a trilha crescer e todo o conteúdo vazar para fora do cartão. `grid-cols-[minmax(0,1fr)]` fixa o mínimo em `0` e elimina a classe inteira do problema.

Compatível com quem sobrescreve o display: `cn()` usa `tailwind-merge`, então um consumidor que passe `flex flex-col` (como o preview do fluxo em `ChatbotFlowBuilderView.vue:941`) substitui `grid` e a regra `grid-cols-*` fica inerte.

**Files:**
- Modify: `frontend/src/components/ui/dialog/DialogContent.vue:32`
- Modify: `frontend/src/components/ui/alert-dialog/AlertDialogContent.vue:30`

**Interfaces:**
- Consumes: nada.
- Produces: nenhuma API nova. A Task 3 depende deste comportamento para que os diálogos do chat parem de vazar.

- [ ] **Step 1: Blindar o `DialogContent`**

Em `frontend/src/components/ui/dialog/DialogContent.vue`, na string de classes da linha 32, trocar o trecho inicial.

De:

```
'fixed left-1/2 top-1/2 z-50 grid w-full max-w-lg -translate-x-1/2 -translate-y-1/2 gap-4 border bg-background p-6
```

Para:

```
'fixed left-1/2 top-1/2 z-50 grid grid-cols-[minmax(0,1fr)] w-full max-w-lg -translate-x-1/2 -translate-y-1/2 gap-4 border bg-background p-6
```

O restante da string (sombras, ring, animações, `sm:rounded-lg`) fica intacto.

- [ ] **Step 2: Blindar o `AlertDialogContent`**

Em `frontend/src/components/ui/alert-dialog/AlertDialogContent.vue`, aplicar exatamente a mesma troca na string de classes da linha 30.

De:

```
'fixed left-1/2 top-1/2 z-50 grid w-full max-w-lg -translate-x-1/2 -translate-y-1/2 gap-4 border bg-background p-6
```

Para:

```
'fixed left-1/2 top-1/2 z-50 grid grid-cols-[minmax(0,1fr)] w-full max-w-lg -translate-x-1/2 -translate-y-1/2 gap-4 border bg-background p-6
```

- [ ] **Step 3: Typecheck e lint**

```bash
cd frontend && npm run typecheck && npm run lint
```

Esperado: sem erros.

- [ ] **Step 4: Confirmar que nenhum diálogo existente regrediu**

```bash
cd frontend && BASE_URL=http://localhost:8080 npx playwright test e2e/tests/chatbot e2e/tests/chat --project=chromium
```

Esperado: mesmo resultado de antes da mudança (nenhuma falha nova). Esta é uma mudança em componente compartilhado — é o passo que protege as outras telas.

Se a stack não estiver no ar, registre isso explicitamente.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/ui/dialog/DialogContent.vue frontend/src/components/ui/alert-dialog/AlertDialogContent.vue
git commit -m "fix(ui): keep dialog grid track from overflowing its own card"
```

---

## Task 3: Corrigir os três diálogos de seleção do chat

**Contexto:** `ChatView.vue` tem três diálogos com a mesma estrutura e o mesmo defeito duplo:

1. **Vazamento horizontal** — cada item da lista é um `Button` (flex) com `<span class="truncate">`. `truncate` aplica `white-space: nowrap`; como item flex, o `min-width` do span é `auto`, então sua contribuição mínima é o **texto inteiro**. Com "Supervisor Vitória da Conquista" + badge "Supervisor de Unidade", o botão exige ~390px dentro de um cartão de 384px (`max-w-sm`). Sem `min-w-0` o `truncate` nunca trunca — ele empurra.
2. **Rolagem morta** — `ScrollArea` com `max-h-[280px]` num `ScrollAreaRoot` de altura automática. O `ScrollAreaViewport` do reka-ui tem classe `h-full`; como o pai tem `height: auto` (só `max-height`), a porcentagem resolve como `auto` e o viewport **cresce em vez de rolar**. O `overflow-hidden` do Root corta o excesso. Com ~8 usuários cabe por pouco; a partir de ~10 os últimos ficam **inalcançáveis**.

A correção transforma o modal numa coluna flex com teto de altura, o que dá altura definida ao `ScrollArea` e faz a rolagem funcionar de verdade.

Os três diálogos:

| Linha | Diálogo |
|---|---|
| 2846 | Definir responsável (`isAssignDialogOpen`) |
| 2911 | Transferir para agente (`isTransferAgentDialogOpen`) |
| 2953 | Transferir para time (`isTransferTeamDialogOpen`) |

**Files:**
- Modify: `frontend/src/views/chat/ChatView.vue:2847-2906` (definir responsável)
- Modify: `frontend/src/views/chat/ChatView.vue:2912-2948` (transferir p/ agente)
- Modify: `frontend/src/views/chat/ChatView.vue:2954-2983` (transferir p/ time)

**Interfaces:**
- Consumes: a blindagem da Task 2.
- Produces: nada. É correção puramente visual.

- [ ] **Step 1: Corrigir o diálogo "Definir responsável"**

Em `frontend/src/views/chat/ChatView.vue`, aplicar quatro edições dentro do bloco que começa na linha 2847.

**1a.** Linha 2847 — dar teto de altura e virar coluna flex:

```vue
      <DialogContent class="max-w-md max-h-[85vh] flex flex-col">
```

**1b.** Linha 2848 — impedir que o cabeçalho encolha:

```vue
        <DialogHeader class="shrink-0">
```

**1c.** Linha 2854 — o corpo propaga a altura definida para o filho (`min-h-0` é o que permite o item flex encolher abaixo do conteúdo):

```vue
        <div class="py-4 space-y-3 flex-1 min-h-0 flex flex-col">
```

**1d.** Linha 2882 — `ScrollArea` com altura definida vinda do flex, em vez do `max-h` que não rola:

```vue
          <ScrollArea class="flex-1 min-h-0">
```

**1e.** Linha 2892 — deixar o `truncate` realmente truncar:

```vue
                <span class="truncate min-w-0 flex-1 text-left">{{ user.full_name }}</span>
```

- [ ] **Step 2: Corrigir o diálogo "Transferir para agente"**

Mesmo bloco de edições, agora a partir da linha 2912.

**2a.** Linha 2912:

```vue
      <DialogContent class="max-w-md max-h-[85vh] flex flex-col">
```

**2b.** Linha 2913:

```vue
        <DialogHeader class="shrink-0">
```

**2c.** Linha 2917:

```vue
        <div class="py-4 space-y-3 flex-1 min-h-0 flex flex-col">
```

**2d.** Linha 2929:

```vue
          <ScrollArea class="flex-1 min-h-0">
```

**2e.** Linha 2940:

```vue
                <span class="truncate min-w-0 flex-1 text-left">{{ user.full_name }}</span>
```

- [ ] **Step 3: Corrigir o diálogo "Transferir para time"**

Mesmo bloco de edições, agora a partir da linha 2954. Este item de lista não tem badge, mas tem o mesmo `truncate` sem `min-w-0` e o mesmo `ScrollArea` que não rola.

**3a.** Linha 2954:

```vue
      <DialogContent class="max-w-md max-h-[85vh] flex flex-col">
```

**3b.** Linha 2955:

```vue
        <DialogHeader class="shrink-0">
```

**3c.** Linha 2959:

```vue
        <div class="py-4 space-y-3 flex-1 min-h-0 flex flex-col">
```

**3d.** Linha 2964:

```vue
          <ScrollArea class="flex-1 min-h-0">
```

**3e.** Linha 2975:

```vue
                <span class="truncate min-w-0 flex-1 text-left">{{ team.name }}</span>
```

- [ ] **Step 4: Typecheck e lint**

```bash
cd frontend && npm run typecheck && npm run lint
```

Esperado: sem erros.

- [ ] **Step 5: Conferir visualmente no navegador**

Esta é uma correção de CSS: ela precisa de olho, não só de asserção. Com a stack no ar, abrir uma conversa e o menu "Definir responsável" numa organização com nomes longos (os "Supervisor <cidade>" servem) e confirmar:

1. o `Select` de papel e o campo de busca ficam **dentro** do cartão branco, sem sobrar nas laterais;
2. os badges de papel aparecem **inteiros**, e nomes muito longos terminam em reticências;
3. com mais itens do que cabem na altura, a lista **rola** e o último item é alcançável;
4. o modal nunca fica mais alto que a janela.

Repetir para "Transferir para agente" e "Transferir para time".

Registre o que foi conferido. Se a stack não estiver no ar, diga que a verificação visual não foi feita — não presuma.

- [ ] **Step 6: Rodar os E2E do chat para garantir que nada quebrou**

```bash
cd frontend && BASE_URL=http://localhost:8080 npx playwright test e2e/tests/chat --project=chromium
```

Esperado: mesmo resultado de antes da mudança.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/views/chat/ChatView.vue
git commit -m "fix(chat): make the assign/transfer pickers fit their card and actually scroll"
```

---

## Task 4: Composable `useResizablePanel` e chaves de i18n

**Contexto:** O `ChatbotFlowBuilderView.vue` já tem 961 linhas. O estado do painel (largura, colapso, arraste, persistência) vive num composable próprio para não engordar a view e para manter uma responsabilidade por arquivo. Sem infra de teste unitário no projeto, a validação vem pelos E2E da Task 5 — por isso esta task entrega o código e as chaves, e a próxima entrega o comportamento observável.

**Files:**
- Create: `frontend/src/composables/useResizablePanel.ts`
- Modify: `frontend/src/composables/index.ts`
- Modify: `frontend/src/i18n/locales/pt-BR.json` (após a linha 2609)
- Modify: `frontend/src/i18n/locales/en.json` (após a linha 2609)

**Interfaces:**
- Consumes: nada.
- Produces:
  - `useResizablePanel(options: ResizablePanelOptions): ResizablePanel`
  - `ResizablePanelOptions = { storageKey: string; defaultWidth: number; minWidth: number; maxWidth: number }`
  - `ResizablePanel = { width: Ref<number>; collapsed: Ref<boolean>; isDragging: Ref<boolean>; toggle(): void; expand(): void; onHandlePointerDown(event: PointerEvent): void; onHandleKeydown(event: KeyboardEvent): void }`
  - Chaves i18n `flowBuilder.collapsePanel`, `flowBuilder.expandPanel`, `flowBuilder.resizePanel`.

  A Task 5 consome exatamente estes nomes.

- [ ] **Step 1: Criar o composable**

Criar `frontend/src/composables/useResizablePanel.ts` com este conteúdo:

```typescript
import { ref, watch, type Ref } from 'vue'

export interface ResizablePanelOptions {
  /** localStorage key holding `{ width, collapsed }`. */
  storageKey: string
  defaultWidth: number
  minWidth: number
  maxWidth: number
}

export interface ResizablePanel {
  width: Ref<number>
  collapsed: Ref<boolean>
  isDragging: Ref<boolean>
  toggle: () => void
  expand: () => void
  onHandlePointerDown: (event: PointerEvent) => void
  onHandleKeydown: (event: KeyboardEvent) => void
}

/** Pixels added/removed per arrow-key press on the resize handle. */
const KEYBOARD_STEP = 20

/**
 * State for a right-edge panel the user can collapse and drag-resize.
 * Width and collapsed state survive reloads via localStorage; anything
 * unreadable there falls back to the defaults rather than breaking the view.
 */
export function useResizablePanel(options: ResizablePanelOptions): ResizablePanel {
  const { storageKey, defaultWidth, minWidth, maxWidth } = options

  const clamp = (value: number) => Math.min(maxWidth, Math.max(minWidth, Math.round(value)))

  const width = ref(defaultWidth)
  const collapsed = ref(false)
  const isDragging = ref(false)

  try {
    const raw = localStorage.getItem(storageKey)
    if (raw) {
      const saved = JSON.parse(raw) as { width?: unknown; collapsed?: unknown }
      if (typeof saved.width === 'number' && Number.isFinite(saved.width)) {
        width.value = clamp(saved.width)
      }
      if (typeof saved.collapsed === 'boolean') {
        collapsed.value = saved.collapsed
      }
    }
  } catch {
    // Missing or corrupted entry — the defaults above already apply.
  }

  function persist() {
    try {
      localStorage.setItem(storageKey, JSON.stringify({ width: width.value, collapsed: collapsed.value }))
    } catch {
      // Private mode or quota exceeded — resizing still works for this session.
    }
  }

  watch([width, collapsed], persist)

  function toggle() {
    collapsed.value = !collapsed.value
  }

  function expand() {
    collapsed.value = false
  }

  function onHandlePointerDown(event: PointerEvent) {
    if (collapsed.value) return

    const handle = event.currentTarget as HTMLElement
    const startX = event.clientX
    const startWidth = width.value

    isDragging.value = true
    handle.setPointerCapture(event.pointerId)

    // The panel sits on the right edge, so dragging left widens it.
    function onMove(moveEvent: PointerEvent) {
      width.value = clamp(startWidth - (moveEvent.clientX - startX))
    }

    function onEnd() {
      isDragging.value = false
      if (handle.hasPointerCapture(event.pointerId)) {
        handle.releasePointerCapture(event.pointerId)
      }
      handle.removeEventListener('pointermove', onMove)
      handle.removeEventListener('pointerup', onEnd)
      handle.removeEventListener('pointercancel', onEnd)
    }

    handle.addEventListener('pointermove', onMove)
    handle.addEventListener('pointerup', onEnd)
    handle.addEventListener('pointercancel', onEnd)

    event.preventDefault()
  }

  function onHandleKeydown(event: KeyboardEvent) {
    if (event.key === 'ArrowLeft') {
      width.value = clamp(width.value + KEYBOARD_STEP)
      event.preventDefault()
    } else if (event.key === 'ArrowRight') {
      width.value = clamp(width.value - KEYBOARD_STEP)
      event.preventDefault()
    }
  }

  return { width, collapsed, isDragging, toggle, expand, onHandlePointerDown, onHandleKeydown }
}
```

- [ ] **Step 2: Exportar no barrel**

Em `frontend/src/composables/index.ts`, acrescentar ao final (o arquivo usa exportações nomeadas com os tipos na mesma linha — siga esse estilo; hoje termina na linha 12 com `export { useTypingNotifier } from './useTypingNotifier'`):

```typescript
export { useResizablePanel, type ResizablePanel, type ResizablePanelOptions } from './useResizablePanel'
```

- [ ] **Step 3: Adicionar as chaves em pt-BR**

Em `frontend/src/i18n/locales/pt-BR.json`, dentro do objeto `flowBuilder`, logo **depois** da linha 2609 (`"flowSettings": "Configurações do fluxo",`), inserir:

```json
    "collapsePanel": "Recolher painel",
    "expandPanel": "Expandir painel",
    "resizePanel": "Redimensionar painel",
```

- [ ] **Step 4: Adicionar as chaves em inglês**

Em `frontend/src/i18n/locales/en.json`, dentro do objeto `flowBuilder`, logo **depois** da linha 2609 (`"flowSettings": "Flow Settings",`), inserir:

```json
    "collapsePanel": "Collapse panel",
    "expandPanel": "Expand panel",
    "resizePanel": "Resize panel",
```

- [ ] **Step 5: Verificar paridade de chaves, tipos e lint**

```bash
cd frontend && npm run i18n:keys && npm run typecheck && npm run lint
```

Esperado: sem chaves faltando entre os locales, sem erros de tipo, sem erros de lint.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/composables/useResizablePanel.ts frontend/src/composables/index.ts frontend/src/i18n/locales/pt-BR.json frontend/src/i18n/locales/en.json
git commit -m "feat(ui): add useResizablePanel composable and flow-builder panel labels"
```

---

## Task 5: Painel direito recolhível e redimensionável

**Contexto:** O painel direito do construtor é hoje `<Card class="w-[420px] …">`, fixo. Em fluxos grandes isso espreme a área de edição. Passa a ter botão de recolher, faixa fina de 28px para reabrir e alça de arraste na borda esquerda.

Detalhe que evita um bug de percepção: **selecionar um nó com o painel recolhido reabre o painel**. Sem isso, clicar num nó não teria efeito visível e pareceria quebrado.

O canvas é `flex-1` e reflui sozinho; o Vue Flow já observa o próprio tamanho. **Não chame `fitView`** no toggle nem no arraste — isso reposicionaria a viewport e tiraria o autor do ponto onde ele estava editando.

**Files:**
- Modify: `frontend/src/views/chatbot/ChatbotFlowBuilderView.vue` (imports ~linha 2 e 34-39, estado ~linha 97, watch ~linha 177, template linhas 750 e 785)
- Modify: `frontend/e2e/pages/FlowsPage.ts` (classe `ChatbotFlowBuilderPage`)
- Test: `frontend/e2e/tests/chatbot/flow-builder.spec.ts`

**Interfaces:**
- Consumes: `useResizablePanel` da Task 4, com `ResizablePanel` exatamente como definido lá; chaves `flowBuilder.collapsePanel` / `expandPanel` / `resizePanel`; locators do page object criados na Task 1.
- Produces: locators `collapsePanelButton`, `expandPanelButton`, `panelResizeHandle`, `flowSettingsHeading`, `rightPanel`.

- [ ] **Step 1: Adicionar os locators do painel ao page object**

Em `frontend/e2e/pages/FlowsPage.ts`, dentro da classe `ChatbotFlowBuilderPage`, junto dos locators criados na Task 1, inserir:

```typescript
  /** Collapse toggle inside the right panel. */
  get collapsePanelButton() {
    return this.page.getByRole('button', { name: 'Collapse panel' })
  }

  /** Expand toggle on the collapsed rail. */
  get expandPanelButton() {
    return this.page.getByRole('button', { name: 'Expand panel' })
  }

  /** Drag handle on the panel's left edge. */
  get panelResizeHandle() {
    return this.page.getByRole('separator', { name: 'Resize panel' })
  }

  /** "Flow Settings" heading — only rendered while the panel is open. */
  get flowSettingsHeading() {
    return this.page.getByText('Flow Settings', { exact: true })
  }

  /** The right panel element itself (used for width assertions). */
  get rightPanel() {
    return this.page.getByTestId('flow-builder-panel')
  }
```

- [ ] **Step 2: Escrever os testes que falham**

Em `frontend/e2e/tests/chatbot/flow-builder.spec.ts`, acrescentar ao **final** do arquivo:

```typescript
test.describe('Chatbot Flow Builder - right panel', () => {
  let builder: ChatbotFlowBuilderPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    builder = new ChatbotFlowBuilderPage(page)
    await builder.gotoNew()
  })

  test('collapses and expands the right panel', async () => {
    await expect(builder.flowSettingsHeading).toBeVisible()

    await builder.collapsePanelButton.click()
    await expect(builder.flowSettingsHeading).toBeHidden()
    await expect(builder.expandPanelButton).toBeVisible()

    await builder.expandPanelButton.click()
    await expect(builder.flowSettingsHeading).toBeVisible()
  })

  test('selecting a node reopens a collapsed panel', async () => {
    await builder.collapsePanelButton.click()
    await expect(builder.expandPanelButton).toBeVisible()

    // Adding a node auto-selects it; the properties must not stay hidden.
    await builder.addNode('Text')
    await expect(builder.messageTextarea).toBeVisible()
  })

  test('remembers the collapsed state across reloads', async ({ page }) => {
    await builder.collapsePanelButton.click()
    await expect(builder.expandPanelButton).toBeVisible()

    await page.reload()
    await page.waitForLoadState('networkidle')

    await expect(builder.expandPanelButton).toBeVisible()
  })

  test('arrow keys on the resize handle widen and narrow the panel', async () => {
    const before = await builder.rightPanel.boundingBox()
    expect(before).not.toBeNull()

    await builder.panelResizeHandle.focus()
    // Panel is on the right edge, so ArrowLeft widens it. 3 x 20px.
    await builder.panelResizeHandle.press('ArrowLeft')
    await builder.panelResizeHandle.press('ArrowLeft')
    await builder.panelResizeHandle.press('ArrowLeft')

    const after = await builder.rightPanel.boundingBox()
    expect(after!.width).toBeGreaterThan(before!.width)
  })
})
```

- [ ] **Step 3: Rodar os testes e confirmar que falham**

```bash
cd frontend && BASE_URL=http://localhost:8080 npx playwright test e2e/tests/chatbot/flow-builder.spec.ts -g "right panel" --project=chromium
```

Esperado: **4 falhas** — nenhum dos controles existe ainda.

- [ ] **Step 4: Importar o ícone e o composable**

Em `frontend/src/views/chatbot/ChatbotFlowBuilderView.vue`:

**4a.** No bloco de ícones do `lucide-vue-next` (linhas 22-39), acrescentar `ChevronLeft` junto de `ChevronDown` e `ChevronRight`, que já estão lá:

```typescript
  ChevronDown,
  ChevronLeft,
  ChevronRight,
```

**4b.** Logo depois da linha 47 (`import PanelConfigEditor from '@/components/chatbot/PanelConfigEditor.vue'`), acrescentar:

```typescript
import { useResizablePanel } from '@/composables/useResizablePanel'
```

- [ ] **Step 5: Instanciar o painel no estado da view**

Em `frontend/src/views/chatbot/ChatbotFlowBuilderView.vue`, logo **depois** da linha 97 (`const activityOpen = ref(false)`), acrescentar:

```typescript
// Right panel geometry (collapse + drag-resize), persisted across sessions.
const {
  width: panelWidth,
  collapsed: panelCollapsed,
  isDragging: panelDragging,
  toggle: togglePanel,
  expand: expandPanel,
  onHandlePointerDown: onPanelHandlePointerDown,
  onHandleKeydown: onPanelHandleKeydown,
} = useResizablePanel({
  storageKey: 'flow-builder-panel',
  defaultWidth: 420,
  minWidth: 320,
  maxWidth: 720,
})
```

- [ ] **Step 6: Reabrir o painel quando um nó for selecionado**

Ainda em `ChatbotFlowBuilderView.vue`, logo **depois** do bloco `selectedChatNode` (que termina na linha 176) e **antes** de `function onNodeClick`, acrescentar:

```typescript
// Selecting a node must never leave the author staring at a hidden panel.
watch(selectedNodeId, (id) => {
  if (id) expandPanel()
})
```

`watch` já está importado na linha 2.

- [ ] **Step 7: Suprimir seleção de texto durante o arraste**

Em `ChatbotFlowBuilderView.vue`, trocar a linha 750.

De:

```vue
    <div class="flex-1 flex overflow-hidden">
```

Para:

```vue
    <div class="flex-1 flex overflow-hidden" :class="panelDragging && 'select-none'">
```

- [ ] **Step 8: Trocar o painel fixo pela faixa + painel redimensionável**

Em `ChatbotFlowBuilderView.vue`, substituir a linha 785 inteira.

De:

```vue
      <!-- Right panel -->
      <Card class="w-[420px] min-w-0 border-y-0 border-r-0 rounded-none shrink-0 flex flex-col">
```

Para:

```vue
      <!-- Collapsed rail: the only way back once the panel is hidden -->
      <div
        v-if="panelCollapsed"
        class="w-7 shrink-0 border-l bg-background flex justify-center pt-2"
      >
        <Button
          variant="ghost"
          size="icon"
          class="h-7 w-7"
          :title="$t('flowBuilder.expandPanel')"
          :aria-label="$t('flowBuilder.expandPanel')"
          @click="expandPanel()"
        >
          <ChevronLeft class="h-4 w-4" />
        </Button>
      </div>

      <!-- Right panel -->
      <Card
        v-else
        data-testid="flow-builder-panel"
        class="min-w-0 border-y-0 border-r-0 rounded-none shrink-0 flex flex-col relative"
        :style="{ width: panelWidth + 'px' }"
      >
        <div
          role="separator"
          aria-orientation="vertical"
          tabindex="0"
          :aria-label="$t('flowBuilder.resizePanel')"
          :class="[
            'absolute left-0 top-0 z-10 h-full w-1.5 -ml-0.5 cursor-col-resize transition-colors',
            'hover:bg-primary/40 focus-visible:bg-primary/40 focus-visible:outline-none',
            panelDragging && 'bg-primary/60',
          ]"
          @pointerdown="onPanelHandlePointerDown"
          @keydown="onPanelHandleKeydown"
        />
        <div class="flex justify-end px-2 pt-2 shrink-0">
          <Button
            variant="ghost"
            size="icon"
            class="h-7 w-7"
            :title="$t('flowBuilder.collapsePanel')"
            :aria-label="$t('flowBuilder.collapsePanel')"
            @click="togglePanel()"
          >
            <ChevronRight class="h-4 w-4" />
          </Button>
        </div>
```

O conteúdo existente do Card (o `<div v-if="selectedChatNode …">` da linha 787 em diante e o `<ScrollArea v-else …>`) fica **exatamente como está**, agora abaixo da barrinha do toggle. O `</Card>` de fechamento também não muda.

- [ ] **Step 9: Rodar os testes e confirmar que passam**

```bash
cd frontend && BASE_URL=http://localhost:8080 npx playwright test e2e/tests/chatbot/flow-builder.spec.ts --project=chromium
```

Esperado: **todos os testes do arquivo passam**, incluindo os 4 novos e os que já existiam.

- [ ] **Step 10: Typecheck, lint e paridade de i18n**

```bash
cd frontend && npm run typecheck && npm run lint && npm run i18n:keys
```

Esperado: sem erros.

- [ ] **Step 11: Conferir o arraste no navegador**

O arraste com ponteiro não é coberto pelos E2E (só o ajuste por teclado é). Com a stack no ar, abrir um fluxo e confirmar:

1. arrastar a borda esquerda do painel para a **esquerda** o alarga; para a **direita**, estreita;
2. a largura para nos limites (não passa de ~720px nem afunda abaixo de ~320px);
3. durante o arraste nada é selecionado como texto;
4. soltar o botão fora da janela não deixa o painel "grudado" no ponteiro;
5. recarregar a página preserva a largura escolhida;
6. o canvas acompanha a largura sem que a viewport do fluxo pule de lugar.

Registre o que foi conferido. Se a stack não estiver no ar, diga que não foi verificado.

- [ ] **Step 12: Commit**

```bash
git add frontend/src/views/chatbot/ChatbotFlowBuilderView.vue frontend/e2e/pages/FlowsPage.ts frontend/e2e/tests/chatbot/flow-builder.spec.ts
git commit -m "feat(flow-builder): collapsible and resizable right panel"
```

---

## Task 6: Verificação final da branch

**Files:** nenhum arquivo novo — é o portão de saída.

**Interfaces:**
- Consumes: tudo das Tasks 1-5.
- Produces: nada.

- [ ] **Step 1: Suíte completa de verificações estáticas**

```bash
cd frontend && npm run typecheck && npm run lint && npm run i18n:keys
```

Esperado: sem erros em nenhum dos três.

- [ ] **Step 2: Suíte E2E de chatbot e chat**

```bash
cd frontend && BASE_URL=http://localhost:8080 npx playwright test e2e/tests/chatbot e2e/tests/chat --project=chromium
```

Esperado: sem falhas.

- [ ] **Step 3: Revisar o diff completo da branch**

```bash
git diff development...HEAD --stat
```

Confirme que só aparecem os arquivos listados no File Structure deste plano. Qualquer arquivo fora dessa lista é escopo que vazou — investigue antes de seguir.

- [ ] **Step 4: Relatar honestamente**

Escrever o resumo do que foi feito, citando **quais comandos rodaram e qual foi a saída real**. Se algum E2E ou verificação visual não pôde ser executado por falta da stack, diga isso de forma explícita. Não afirme que algo passou sem ter visto passar.

---

## Notas para quem revisa

Riscos conhecidos, em ordem de atenção:

1. **Task 2 mexe em componente compartilhado** (`DialogContent`/`AlertDialogContent`), usado por toda a aplicação. É uma classe de Tailwind aditiva que só impede overflow, mas é a mudança de maior alcance da branch — daí o Step 4 rodando as suítes existentes.
2. **Task 3 altera três diálogos de produção do chat**, incluindo transferência de conversa. A lógica (`transferToAgent`, `transferToTeam`, `assignContactToUser`) não é tocada — só classes CSS.
3. **Task 5 altera o layout da tela mais pesada do produto.** O conteúdo do painel é preservado literalmente; o que muda é o contêiner.

Dívida registrada no spec e **deliberadamente fora deste plano**: o construtor de fluxo não usa `useUnsavedChangesGuard` (sair pela sidebar ou fechar a aba não avisa); `confirmLeave()` navega com `window.location.href`, forçando reload completo; não houve varredura dos demais modais atrás do padrão `truncate` sem `min-w-0`.
