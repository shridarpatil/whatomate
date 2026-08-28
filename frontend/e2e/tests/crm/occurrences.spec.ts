import { test, expect } from '@playwright/test'
import { loginAsAdmin } from '../../helpers'
import { ApiHelper } from '../../helpers/api'
import { createTestScope } from '../../framework'
import { ChatPage, OccurrencesPage } from '../../pages'

const scope = createTestScope('crm-occurrences')

/** Creates a fresh contact for one test so no test shares occurrence state
 * with another — the suite runs fullyParallel and org-wide "N occurrences"
 * counts would be flaky, but a contact-scoped panel and a unique protocol
 * are not. */
async function createContact(api: ApiHelper): Promise<string> {
  await api.loginAsAdmin()
  const contact = await api.createContact(scope.phone(), scope.name('contact'))
  return contact.id
}

async function getOccurrenceByProtocol(api: ApiHelper, protocol: string) {
  const res = await api.get(`/api/occurrences?protocol=${encodeURIComponent(protocol)}`)
  if (!res.ok()) throw new Error(`Failed to fetch occurrence: ${await res.text()}`)
  const body = await res.json()
  const occ = body.data?.occurrences?.[0]
  if (!occ) throw new Error(`No occurrence found for protocol ${protocol}`)
  return occ
}

async function listStages(api: ApiHelper) {
  const res = await api.get('/api/occurrence-stages')
  if (!res.ok()) throw new Error(`Failed to fetch stages: ${await res.text()}`)
  const body = await res.json()
  return body.data.stages as Array<{ id: string; name: string; is_closing: boolean }>
}

test.describe('CRM Occurrences', () => {
  let occurrencesPage: OccurrencesPage
  let chatPage: ChatPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    occurrencesPage = new OccurrencesPage(page)
    chatPage = new ChatPage(page)
  })

  test('creates an occurrence from the conversation panel and issues a protocol', async ({ page, request }) => {
    const api = new ApiHelper(request)
    const contactId = await createContact(api)
    await chatPage.goto(contactId)

    const title = scope.name('occ')
    await occurrencesPage.createOccurrence(title)

    const protocol = await occurrencesPage.getProtocolNumber(title)
    expect(protocol).toMatch(/^\d{4}-\d{6}$/)
  })

  test('moves an occurrence to another stage and logs it on the timeline', async ({ page, request }) => {
    const api = new ApiHelper(request)
    const contactId = await createContact(api)
    await chatPage.goto(contactId)

    const title = scope.name('occ')
    await occurrencesPage.createOccurrence(title)
    const protocol = await occurrencesPage.getProtocolNumber(title)

    const occ = await getOccurrenceByProtocol(api, protocol)
    const stages = await listStages(api)
    const target = stages.find(s => s.id !== occ.stage_id)
    if (!target) throw new Error('org has only one occurrence stage — cannot test a move')

    await occurrencesPage.openOccurrenceDetail(title)
    await occurrencesPage.changeStage(target.name)

    await occurrencesPage.expectToast('Stage updated')
    await expect(occurrencesPage.timelineEntry('Stage change')).toBeVisible()
  })

  test('adds a note to the timeline', async ({ page, request }) => {
    const api = new ApiHelper(request)
    const contactId = await createContact(api)
    await chatPage.goto(contactId)

    const title = scope.name('occ')
    await occurrencesPage.createOccurrence(title)
    const protocol = await occurrencesPage.getProtocolNumber(title)

    await occurrencesPage.openOccurrenceDetail(title)
    const noteText = scope.name('note')
    await occurrencesPage.addNote(noteText)

    await occurrencesPage.expectToast('Note added')
    await expect(occurrencesPage.timelineEntry(noteText)).toBeVisible()
  })

  test('refuses to delete a stage that is in use by an occurrence', async ({ page, request }) => {
    const api = new ApiHelper(request)
    const contactId = await createContact(api)
    await chatPage.goto(contactId)

    const title = scope.name('occ')
    await occurrencesPage.createOccurrence(title)
    const protocol = await occurrencesPage.getProtocolNumber(title)
    const occ = await getOccurrenceByProtocol(api, protocol)

    await occurrencesPage.gotoStagesSettings()
    await occurrencesPage.deleteStage(occ.stage_name)

    await occurrencesPage.expectToast('Stage is in use by existing occurrences')
    await expect(occurrencesPage.getStageRow(occ.stage_name)).toBeVisible()
  })

  test('a new occurrence starts assigned to its creator, and the assignee can be changed and cleared', async ({ page, request }) => {
    const api = new ApiHelper(request)
    const contactId = await createContact(api)
    await chatPage.goto(contactId)

    const title = scope.name('occ')
    await occurrencesPage.createOccurrence(title)
    const protocol = await occurrencesPage.getProtocolNumber(title)

    // The occurrence was opened by the logged-in admin (Test Admin), who
    // must be its default assignee — no case is born orphaned.
    const occ = await getOccurrenceByProtocol(api, protocol)
    expect(occ.assigned_user_name).toBe('Test Admin')

    await occurrencesPage.openOccurrenceDetail(title)
    await expect(occurrencesPage.assigneeSelect).toContainText('Test Admin')

    await occurrencesPage.changeAssignee('Test Manager')
    await occurrencesPage.expectToast('Assignee updated')
    await expect(occurrencesPage.assigneeSelect).toContainText('Test Manager')
    await expect(occurrencesPage.timelineEntry('Assignment')).toBeVisible()

    await occurrencesPage.removeAssignee()
    await occurrencesPage.expectToast('Assignee updated')
    await expect(occurrencesPage.assigneeSelect).toContainText('Unassigned')
    await expect(occurrencesPage.timelineEntry('Assignment')).toHaveCount(2)
  })
})
