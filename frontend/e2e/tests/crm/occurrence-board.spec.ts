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
  let chatPage: ChatPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    occurrencesPage = new OccurrencesPage(page)
    chatPage = new ChatPage(page)
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

  test('clicking the already-active mode button is a no-op', async ({ page }) => {
    await occurrencesPage.gotoList()
    await expect(occurrencesPage.listView).toBeVisible()

    // Switch away and back so a concrete "list" preference is actually
    // persisted, not just the unwritten default — otherwise the assertion
    // below can't tell a corrupted store from a preference that was never
    // written in the first place.
    await occurrencesPage.switchToBoard()
    await occurrencesPage.switchToList()
    await expect(occurrencesPage.listView).toBeVisible()

    // reka-ui's single-select ToggleGroup deselects (emits undefined) when
    // the active item is clicked again. Clicking "List" while already in
    // list mode must not fall through to the board placeholder or corrupt
    // the stored preference.
    await occurrencesPage.switchToList()
    await expect(occurrencesPage.listView).toBeVisible()
    await expect(occurrencesPage.boardView).not.toBeVisible()

    const stored = await page.evaluate(() => localStorage.getItem('occurrences:view-mode'))
    expect(stored).toBe('list')

    await page.reload()
    await page.waitForLoadState('networkidle')
    await expect(occurrencesPage.listView).toBeVisible()
  })

  test('clicking the already-active mode button is a no-op in board mode', async ({ page }) => {
    await occurrencesPage.gotoList()
    await occurrencesPage.switchToBoard()
    await expect(occurrencesPage.boardView).toBeVisible()

    // Mirrors the list-mode assertion above: reka-ui's single-select
    // ToggleGroup deselects (emits undefined) when the active item is
    // clicked again. The bug this guards was visually silent in the board
    // branch, so it needs its own assertion, not just the list one.
    await occurrencesPage.switchToBoard()
    await expect(occurrencesPage.boardView).toBeVisible()
    await expect(occurrencesPage.listView).not.toBeVisible()

    const stored = await page.evaluate(() => localStorage.getItem('occurrences:view-mode'))
    expect(stored).toBe('board')
  })

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

    // O cartão fica travado (cursor-wait) enquanto o PUT de mudança de etapa
    // está em voo — um clique nesse intervalo é ignorado de propósito (I-1),
    // então espera a requisição resolver antes de clicar.
    await expect(occurrencesPage.boardCard(protocol)).not.toHaveClass(/cursor-wait/)

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

    // A mais velha entra primeiro (sozinha na coluna); a mais nova entra por
    // último, soltada explicitamente no rodapé — ou seja, a posição bruta do
    // drop a insere DEPOIS da mais velha. Isso é o oposto da ordem esperada
    // (opened_at DESC bota a mais nova primeiro), então a asserção abaixo só
    // passa se sortByOpenedAtDesc de fato reordenar a coluna. Um drop "no
    // meio" da coluna já cai depois de tudo que existe (ver comentário em
    // dragCardToColumn) e coincidiria com o resultado esperado mesmo sem
    // ordenação nenhuma — por isso o rodapé explícito aqui, não o centro.
    await occurrencesPage.dragCardToColumn(olderProtocol, 'Em análise')
    // Espera o primeiro PUT assentar antes do segundo arrasto: se ele resolve
    // (e o cartão perde `pending`) enquanto o segundo drag ainda está em
    // curso sobre a mesma coluna, o patch do Vue no meio do gesto derruba o
    // rastreamento do Sortable e o drop é silenciosamente ignorado — sem essa
    // espera o teste fica intermitente por essa corrida, não pela ordenação.
    await expect(occurrencesPage.boardCard(olderProtocol)).not.toHaveClass(/cursor-wait/)
    await occurrencesPage.dragCardToColumn(newerProtocol, 'Em análise', { atBottom: true })

    // sortByOpenedAtDesc roda no drop, de forma síncrona, antes do PUT de
    // mudança de etapa sair (onColumnChange ordena antes de marcar o cartão
    // como pendente) — então a ordem já está correta assim que o gesto de
    // arrastar termina, sem precisar esperar a rede. Uma leitura direta do
    // DOM é preferível a `expect.poll` quando o resultado já é determinístico.
    // A comparação é relativa (não por índice absoluto): a coluna pode ter
    // outros cartões residuais de testes anteriores na mesma suíte.
    const texts = await occurrencesPage.boardColumn('Em análise')
      .locator('[data-board-card]').allInnerTexts()
    const olderIndex = texts.findIndex(t => t.includes(olderProtocol))
    const newerIndex = texts.findIndex(t => t.includes(newerProtocol))
    expect(newerIndex).toBeGreaterThanOrEqual(0)
    expect(olderIndex).toBeGreaterThan(newerIndex)
  })

  test('a card locked by an in-flight stage change ignores clicks until it releases', async ({ page, request }) => {
    const api = new ApiHelper(request)
    const contactId = await createContact(api)
    await chatPage.goto(contactId)

    const title = scope.name('locked-click')
    await occurrencesPage.createOccurrence(title)
    const protocol = await occurrencesPage.getProtocolNumber(title)

    await occurrencesPage.gotoList()
    await occurrencesPage.switchToBoard()

    // Segura o PUT de mudança de etapa até o teste liberar explicitamente,
    // para poder clicar no cartão enquanto ele ainda está travado (pending).
    let release: () => void = () => {}
    const held = new Promise<void>(resolve => { release = resolve })
    await page.route('**/api/occurrences/*/stage', async route => {
      await held
      await route.continue()
    })

    const boardUrl = page.url()
    await occurrencesPage.dragCardToColumn(protocol, 'Em análise')

    // A requisição está retida: o cartão mostra cursor-wait (I-1) e um clique
    // aqui não deve navegar para o detalhe — é a guarda `pending.value.has`
    // em goToDetail que barra isso, não deixada por conta de nenhum outro
    // mecanismo.
    await expect(occurrencesPage.boardCard(protocol)).toHaveClass(/cursor-wait/)
    await occurrencesPage.boardCard(protocol).click()
    await page.waitForTimeout(300) // dá tempo de uma navegação indevida acontecer
    expect(page.url()).toBe(boardUrl)

    // Libera a resposta: o cartão destrava e o mesmo clique agora navega —
    // prova que a guarda não é um cadeado que nunca abre.
    release()
    await expect(occurrencesPage.boardCard(protocol)).not.toHaveClass(/cursor-wait/)
    await occurrencesPage.boardCard(protocol).click()
    await page.waitForURL(/\/crm\/occurrences\/[0-9a-f-]+$/)
  })

  test('an empty column drop zone spans most of the column, not a thin strip', async ({ request }) => {
    const api = new ApiHelper(request)
    const contactId = await createContact(api)

    // Enche "Aberto" para esticar o quadro: colunas ficam lado a lado num
    // flex container sem `items-start`, então align-items: stretch (padrão)
    // faz toda coluna herdar a altura da mais alta. É essa altura que a
    // coluna vazia ao lado precisa preencher em sua maior parte.
    for (let i = 0; i < 8; i++) {
      const res = await api.post('/api/occurrences', {
        contact_id: contactId,
        title: scope.name(`fill-${i}`),
      })
      if (!res.ok()) throw new Error(`Failed to create occurrence: ${await res.text()}`)
    }

    await occurrencesPage.gotoList()
    await occurrencesPage.switchToBoard()

    // "Aguardando cliente" nunca é destino de arrasto nesta suíte, então
    // segue vazia — é a coluna vazia medida aqui.
    const emptyColumn = occurrencesPage.boardColumn('Aguardando cliente')
    const dropZone = emptyColumn.locator('[data-board-dropzone]')
    await expect(dropZone).toBeVisible()

    const columnBox = await emptyColumn.boundingBox()
    const dropZoneBox = await dropZone.boundingBox()
    if (!columnBox || !dropZoneBox) throw new Error('missing bounding box')

    // Antes do fix I-2 o alvo de drop era `min-h-16` (64px) dentro de uma
    // coluna de centenas de pixels — uma fração ínfima. Depois, medido: 442px
    // de zona de drop numa coluna de 540px (~82%). 50% falha no comportamento
    // antigo e passa folgado no novo, sem depender de um valor exato de
    // pixels.
    expect(dropZoneBox.height / columnBox.height).toBeGreaterThan(0.5)
  })
})
