import { test, expect } from '@playwright/test'
import { loginAsAdmin } from '../../helpers'
import { ApiHelper } from '../../helpers/api'
import { createTestScope } from '../../framework'
import { OccurrencesPage } from '../../pages'

const scope = createTestScope('crm-realtime')

async function createContact(api: ApiHelper): Promise<string> {
  await api.loginAsAdmin()
  const contact = await api.createContact(scope.phone(), scope.name('contact'))
  return contact.id
}

async function createOccurrenceViaApi(api: ApiHelper, contactId: string, title: string) {
  const res = await api.post('/api/occurrences', { contact_id: contactId, title })
  if (!res.ok()) throw new Error(`Failed to create occurrence: ${await res.text()}`)
  return (await res.json()).data as { id: string; protocol_number: string; stage_id: string }
}

/**
 * Tempo real no CRM (spec 2026-09-03-crm-put-e-tempo-real-design.md, §2 e
 * §4). A ação "de outro agente" é sempre uma chamada de API direta — ver a
 * nota no topo deste arquivo do plano de implementação para o porquê.
 */
test.describe('CRM realtime', () => {
  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
  })

  test('a stage change from another agent moves the card on an open board without a reload', async ({ page, request }) => {
    const api = new ApiHelper(request)
    const contactId = await createContact(api)
    const occ = await createOccurrenceViaApi(api, contactId, scope.name('live-move'))

    const occurrencesPage = new OccurrencesPage(page)
    await occurrencesPage.gotoList()
    await occurrencesPage.switchToBoard()
    await expect(occurrencesPage.boardCard(occ.protocol_number)).toBeVisible()
    // Dá tempo do WebSocket desta página terminar de conectar antes da
    // mudança de etapa sair pela API.
    await page.waitForTimeout(1000)

    const stageRes = await api.get('/api/occurrence-stages')
    if (!stageRes.ok()) throw new Error(`Failed to fetch stages: ${await stageRes.text()}`)
    const { stages } = (await stageRes.json()).data as { stages: Array<{ id: string; name: string }> }
    const target = stages.find(s => s.name === 'Em análise')
    if (!target) throw new Error('etapa "Em análise" não encontrada no pipeline padrão')

    const moveRes = await api.put(`/api/occurrences/${occ.id}/stage`, { stage_id: target.id })
    if (!moveRes.ok()) throw new Error(`Failed to change stage: ${await moveRes.text()}`)

    await expect(occurrencesPage.cardInColumn('Em análise', occ.protocol_number)).toBeVisible({ timeout: 10000 })
    await expect(occurrencesPage.cardInColumn('Aberto', occ.protocol_number)).toBeHidden()
  })

  test('an occurrence created by another agent only bumps the column total, not the card list', async ({ page, request }) => {
    const api = new ApiHelper(request)

    const occurrencesPage = new OccurrencesPage(page)
    await occurrencesPage.gotoList()
    await occurrencesPage.switchToBoard()
    await expect(occurrencesPage.boardColumn('Aberto')).toBeVisible()
    const before = Number(await occurrencesPage.boardColumnCount('Aberto').innerText())
    await page.waitForTimeout(1000)

    const contactId = await createContact(api)
    const occ = await createOccurrenceViaApi(api, contactId, scope.name('counter-only'))

    await expect.poll(
      async () => Number(await occurrencesPage.boardColumnCount('Aberto').innerText()),
      { timeout: 10000 },
    ).toBeGreaterThan(before)
    await expect(occurrencesPage.boardCard(occ.protocol_number)).toBeHidden()
  })

  test('a title change from another agent updates an open detail page without a reload', async ({ page, request }) => {
    const api = new ApiHelper(request)
    const contactId = await createContact(api)
    const occ = await createOccurrenceViaApi(api, contactId, scope.name('before-title'))

    await page.goto(`/crm/occurrences/${occ.id}`)
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(1000)

    const updateRes = await api.put(`/api/occurrences/${occ.id}`, {
      title: 'Titulo mudou ao vivo', priority: 'normal',
    })
    if (!updateRes.ok()) throw new Error(`Failed to update occurrence: ${await updateRes.text()}`)

    await expect(page.getByRole('heading', { name: 'Titulo mudou ao vivo' })).toBeVisible({ timeout: 10000 })
  })

  test('an event on a different occurrence does not touch the open detail page', async ({ page, request }) => {
    const api = new ApiHelper(request)
    const contactId = await createContact(api)
    const watchedTitle = scope.name('watched')
    const watched = await createOccurrenceViaApi(api, contactId, watchedTitle)
    const other = await createOccurrenceViaApi(api, contactId, scope.name('other'))

    await page.goto(`/crm/occurrences/${watched.id}`)
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(1000)

    const noteRes = await api.post(`/api/occurrences/${other.id}/events`, { content: 'Nota na ocorrência errada' })
    if (!noteRes.ok()) throw new Error(`Failed to add note: ${await noteRes.text()}`)

    // Dá tempo do broadcast chegar (ou não) antes de verificar que nada mudou.
    await page.waitForTimeout(1500)
    await expect(page.getByText('Nota na ocorrência errada')).toHaveCount(0)
    await expect(page.getByRole('heading', { name: watchedTitle })).toBeVisible()
  })

  test('the list view ignores realtime events entirely', async ({ page, request }) => {
    const api = new ApiHelper(request)
    const contactId = await createContact(api)

    const occurrencesPage = new OccurrencesPage(page)
    await occurrencesPage.gotoList()
    await expect(occurrencesPage.listView).toBeVisible()
    await page.waitForTimeout(1000)

    const occ = await createOccurrenceViaApi(api, contactId, scope.name('list-no-realtime'))

    // A lista não processa nenhum evento de WebSocket (spec §2, "Na lista:
    // nada") — só aparece depois de uma navegação real, que recarrega do REST.
    await page.waitForTimeout(1500)
    await expect(page.getByText(occ.protocol_number)).toHaveCount(0)

    await occurrencesPage.gotoList()
    await expect(page.getByText(occ.protocol_number)).toBeVisible()
  })
})
