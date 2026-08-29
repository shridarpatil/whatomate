import { Page, Locator, expect } from '@playwright/test'
import { BasePage } from './BasePage'

/**
 * Page object for the CRM occurrences feature: the chat-panel widget that
 * opens/lists a contact's cases (ContactOccurrencesPanel.vue), the case
 * detail view (stage + assignee selectors, timeline, note composer), and
 * the Settings > Occurrence Stages pipeline config.
 */
export class OccurrencesPage extends BasePage {
  // Chat panel
  readonly openPanelButton: Locator
  readonly panel: Locator
  readonly titleInput: Locator

  // Detail view
  readonly stageSelect: Locator
  readonly assigneeSelect: Locator
  readonly sendProtocolButton: Locator
  readonly noteInput: Locator
  readonly addNoteButton: Locator

  // Settings > Occurrence Stages
  readonly stagesHeading: Locator
  readonly alertDialog: Locator

  // --- Lista e quadro ---

  readonly listView: Locator
  readonly boardView: Locator
  readonly boardColumns: Locator
  readonly stageFilterSelect: Locator

  constructor(page: Page) {
    super(page)
    this.openPanelButton = page.locator('#occurrences-button')
    this.panel = page.locator('#occurrences-panel')
    this.titleInput = this.panel.getByPlaceholder('Title')

    // The detail view has exactly two comboboxes, in DOM order: the stage
    // selector (header actions slot, rendered first by DetailPageLayout)
    // and the assignee selector (details card, in the main slot below it).
    this.stageSelect = page.locator('button[role="combobox"]').first()
    this.assigneeSelect = page.locator('button[role="combobox"]').nth(1)
    this.sendProtocolButton = page.getByRole('button', { name: 'Send protocol' })
    this.noteInput = page.getByPlaceholder('Write a note...')
    this.addNoteButton = page.getByRole('button', { name: 'Add note' })

    this.stagesHeading = page.locator('h1').filter({ hasText: 'Occurrence Stages' })
    this.alertDialog = page.locator('[role="alertdialog"]')

    this.listView = page.locator('#occurrences-list')
    this.boardView = page.locator('#occurrences-board')
    this.boardColumns = page.locator('[data-board-column]')
    this.stageFilterSelect = page.locator('#occurrences-stage-filter')
  }

  // --- Lista e quadro ---

  async gotoList() {
    await this.page.goto('/crm/occurrences')
    await this.page.waitForLoadState('networkidle')
  }

  async switchToBoard() {
    // The ToggleGroup renders plain buttons (role="group" on the wrapper,
    // not role="radiogroup"), not ARIA radios — reka-ui's ToggleGroupItem
    // does not set role="radio" on its items.
    await this.page.getByRole('button', { name: 'Board' }).click()
  }

  async switchToList() {
    await this.page.getByRole('button', { name: 'List' }).click()
  }

  boardColumn(stageName: string): Locator {
    return this.page.locator(`[data-board-column="${stageName}"]`)
  }

  /** O número no cabeçalho da coluna (col.total), não a contagem de cartões
   * renderizados — as duas só coincidem quando a coluna não está paginada. */
  boardColumnCount(stageName: string): Locator {
    return this.boardColumn(stageName).locator('[data-board-column-count]')
  }

  boardCard(protocol: string): Locator {
    return this.page.locator('[data-board-card]').filter({ hasText: protocol })
  }

  /** Arrasta um cartão do quadro para a coluna da etapa indicada.
   *
   * Nunca mira o centro geométrico da caixa da coluna: colunas esticam para
   * a altura da mais alta do quadro (align-items: stretch), então uma coluna
   * vazia ao lado de uma com dezenas de cartões residuais de outro teste
   * pode ter uma caixa de milhares de pixels cujo centro cai bem fora da
   * viewport — o próprio `dragTo` não rola até lá, e o drop nunca acontece.
   *
   * Por padrão solta perto do topo (sempre visível). `atBottom` solta logo
   * abaixo do último cartão já existente na coluna — usado quando o teste
   * precisa que a posição bruta do drop discorde da ordem esperada por
   * `opened_at`, em vez de coincidir com ela por acaso (ver I-3). */
  async dragCardToColumn(protocol: string, stageName: string, opts: { atBottom?: boolean } = {}) {
    const card = this.boardCard(protocol)
    await expect(card).toBeVisible({ timeout: 10000 })
    const column = this.boardColumn(stageName)
    const box = await column.boundingBox()
    if (!box) throw new Error(`board column "${stageName}" has no bounding box`)

    if (opts.atBottom) {
      const existing = column.locator('[data-board-card]')
      const lastBox = (await existing.count()) > 0 ? await existing.last().boundingBox() : null
      if (lastBox) {
        // O ponto bruto (logo abaixo do último cartão) ainda mira a coluna,
        // nunca um cartão específico: um alvo fora da própria caixa do
        // cartão falha no teste de "recebe eventos de ponteiro" do
        // Playwright e trava em retry infinito. Mas o deslocamento é
        // grampeado à fatia da coluna hoje visível na viewport — numa coluna
        // esticada pelas dezenas de cartões de uma irmã (align-items:
        // stretch) ou que ela mesma acumule com o tempo, a posição bruta
        // pode cair a milhares de pixels dali, e dragTo não rola até um
        // ponto arbitrário fora da tela.
        const viewport = this.page.viewportSize()
        const rawY = lastBox.y - box.y + lastBox.height + 10
        const visibleMaxY = viewport ? viewport.height - box.y - 10 : rawY
        const y = Math.max(0, Math.min(rawY, visibleMaxY, box.height - 10))
        await card.dragTo(column, { targetPosition: { x: box.width / 2, y } })
        return
      }
    }

    await card.dragTo(column, { targetPosition: { x: box.width / 2, y: Math.min(box.height / 2, 100) } })
  }

  /** O cartão de um protocolo dentro de uma coluna específica. Vazio — logo,
   * `toBeHidden()` — quando o cartão não está naquela coluna. */
  cardInColumn(stageName: string, protocol: string): Locator {
    return this.boardColumn(stageName).locator('[data-board-card]').filter({ hasText: protocol })
  }

  // --- Chat panel ---

  async openPanel() {
    if (!(await this.panel.isVisible())) {
      await this.openPanelButton.click()
      await this.panel.waitFor({ state: 'visible' })
    }
  }

  /** Opens the panel, fills the title and submits. "New occurrence" labels
   * both the header toggle and the form's submit button, so .first()/.last()
   * disambiguate by DOM order rather than text. */
  async createOccurrence(title: string) {
    await this.openPanel()
    await this.panel.getByRole('button', { name: 'New occurrence' }).first().click()
    await this.titleInput.fill(title)
    await this.panel.getByRole('button', { name: 'New occurrence' }).last().click()
    // The form closes only after the create request resolves. Wait for that
    // here so a second call right after this one finds a clean, closed form
    // instead of racing the toggle button against the still-open one.
    await this.titleInput.waitFor({ state: 'hidden' })
  }

  getOccurrenceCard(text: string | RegExp): Locator {
    return this.panel.locator('a').filter({ hasText: text })
  }

  /** Reads the protocol number off a panel card matched by its title. */
  async getProtocolNumber(cardText: string): Promise<string> {
    const card = this.getOccurrenceCard(cardText)
    await expect(card).toBeVisible({ timeout: 10000 })
    return (await card.locator('.font-mono').first().innerText()).trim()
  }

  async openOccurrenceDetail(cardText: string) {
    await this.getOccurrenceCard(cardText).click()
    await this.page.waitForURL(/\/crm\/occurrences\/[0-9a-f-]+$/)
    await this.page.waitForLoadState('networkidle')
  }

  // --- Detail view ---

  async changeStage(stageName: string) {
    await this.stageSelect.click()
    await this.page.locator('[role="option"]').filter({ hasText: stageName }).click()
  }

  async changeAssignee(userName: string) {
    await this.assigneeSelect.click()
    await this.page.locator('[role="option"]').filter({ hasText: userName }).click()
  }

  async removeAssignee() {
    await this.changeAssignee('Unassigned')
  }

  async addNote(content: string) {
    await this.noteInput.fill(content)
    await this.addNoteButton.click()
  }

  get timelineItems(): Locator {
    return this.page.locator('.pl-9.pb-6')
  }

  timelineEntry(text: string | RegExp): Locator {
    return this.timelineItems.filter({ hasText: text })
  }

  // --- Settings > Occurrence Stages ---

  async gotoStagesSettings() {
    await this.page.goto('/settings/occurrence-stages')
    await this.page.waitForLoadState('networkidle')
  }

  getStageRow(name: string): Locator {
    return this.page.locator('tbody tr').filter({ hasText: name })
  }

  /** Clicks the row's delete action (last button in the actions cell) and
   * confirms in the alert dialog. Leaves the dialog open on failure — the
   * app only closes it on a successful delete. */
  async deleteStage(name: string) {
    const row = this.getStageRow(name)
    await expect(row).toBeVisible({ timeout: 10000 })
    await row.locator('td:last-child button').last().click()
    await this.alertDialog.waitFor({ state: 'visible' })
    await this.alertDialog.getByRole('button', { name: /^Delete$/i }).click()
  }

  async expectToast(text: string | RegExp) {
    const toast = this.page.locator('[data-sonner-toast]').filter({ hasText: text })
    await expect(toast).toBeVisible({ timeout: 5000 })
    return toast
  }
}
