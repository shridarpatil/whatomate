import { test, expect } from '@playwright/test'
import { loginAsAdmin, verifyAuditLogged, ApiHelper, TEST_USERS } from '../../helpers'
import { CannedResponsesPage } from '../../pages'
import { createTestScope } from '../../framework'

const scope = createTestScope('canned-responses')

test.describe('Canned Responses Management', () => {
  let cannedResponsesPage: CannedResponsesPage

  test.beforeEach(async ({ page, request }) => {
    await loginAsAdmin(page)
    // Authenticate the API request fixture so verifyAuditLogged can hit /api/audit-logs.
    await new ApiHelper(request).login(TEST_USERS.admin.email, TEST_USERS.admin.password)
    cannedResponsesPage = new CannedResponsesPage(page)
    await cannedResponsesPage.goto()
  })

  test('should display canned responses page', async () => {
    await cannedResponsesPage.expectPageVisible()
    await expect(cannedResponsesPage.addButton).toBeVisible()
  })

  test('should navigate to create detail page', async ({ page }) => {
    await cannedResponsesPage.openCreate()
    expect(page.url()).toContain('/settings/canned-responses/new')
    await expect(page.getByRole('heading', { name: 'Create Canned Response' })).toBeVisible()
  })

  test('should show validation error when saving empty form', async () => {
    await cannedResponsesPage.openCreate()
    // Type then clear to flip the dirty flag and reveal Save.
    await cannedResponsesPage.fillResponseForm('temp', 'temp')
    await cannedResponsesPage.fillResponseForm('', '')
    await cannedResponsesPage.saveDetail()
    await cannedResponsesPage.expectToast('required')
  })

  test('should create a new canned response and audit-log it', async ({ page, request }) => {
    const responseName = scope.name('create')
    const responseContent = 'Hello! Thank you for contacting us.'

    await cannedResponsesPage.openCreate()
    await cannedResponsesPage.fillResponseForm(responseName, responseContent, 'create-sc', 'Greetings')
    await cannedResponsesPage.saveDetail()

    await cannedResponsesPage.expectToast('created')

    // After save we should land on /settings/canned-responses/:id
    await page.waitForURL(/\/settings\/canned-responses\/[a-f0-9-]+$/)
    const id = page.url().split('/').pop()!
    expect(id).toMatch(/^[a-f0-9-]+$/)

    await verifyAuditLogged(request, 'canned_response', id, 'created', {
      expectedFields: ['name', 'content'],
    })

    await cannedResponsesPage.goto()
    await cannedResponsesPage.expectResponseExists(responseName)
  })

  test('should edit existing canned response and audit-log diff', async ({ page, request }) => {
    const responseName = scope.name('edit')
    const updatedContent = 'Updated content body'

    // Create
    await cannedResponsesPage.openCreate()
    await cannedResponsesPage.fillResponseForm(responseName, 'Original content')
    await cannedResponsesPage.saveDetail()
    await cannedResponsesPage.expectToast('created')
    await page.waitForURL(/\/settings\/canned-responses\/[a-f0-9-]+$/)
    const id = page.url().split('/').pop()!

    // Update content on the same detail page
    const textarea = page.locator('div.space-y-1\\.5:has(> label:has-text("Content")) textarea').first()
    await textarea.fill(updatedContent)
    await page.waitForTimeout(300)
    await cannedResponsesPage.saveDetail()
    await cannedResponsesPage.expectToast('updated')

    await verifyAuditLogged(request, 'canned_response', id, 'updated', {
      expectedFields: ['content'],
    })
  })

  test('should delete canned response from detail page and audit-log it', async ({ page, request }) => {
    const responseName = scope.name('delete')

    await cannedResponsesPage.openCreate()
    await cannedResponsesPage.fillResponseForm(responseName, 'To be deleted')
    await cannedResponsesPage.saveDetail()
    await cannedResponsesPage.expectToast('created')
    await page.waitForURL(/\/settings\/canned-responses\/[a-f0-9-]+$/)
    const id = page.url().split('/').pop()!

    // Dismiss the created toast so it doesn't intercept clicks.
    await cannedResponsesPage.dismissToast('created')

    await cannedResponsesPage.deleteFromDetail()
    await cannedResponsesPage.expectToast('deleted')
    await page.waitForURL(/\/settings\/canned-responses$/)

    await verifyAuditLogged(request, 'canned_response', id, 'deleted', {
      expectedFields: ['name'],
    })
  })

  test('should delete canned response from row action', async ({ page }) => {
    const responseName = scope.name('row-delete')

    await cannedResponsesPage.openCreate()
    await cannedResponsesPage.fillResponseForm(responseName, 'Row delete content')
    await cannedResponsesPage.saveDetail()
    await cannedResponsesPage.expectToast('created')
    await cannedResponsesPage.dismissToast('created')

    await cannedResponsesPage.goto()
    await cannedResponsesPage.search(responseName)
    await cannedResponsesPage.expectResponseExists(responseName)

    await cannedResponsesPage.deleteResponse(responseName)
    await cannedResponsesPage.confirmDelete()
    await cannedResponsesPage.expectToast('deleted')
  })

  test('should filter by category', async () => {
    await cannedResponsesPage.filterByCategory('Greetings')
  })

  test('should search canned responses', async ({ page }) => {
    const uniqueText = scope.name('search')

    await cannedResponsesPage.openCreate()
    await cannedResponsesPage.fillResponseForm(uniqueText, 'Search test content')
    await cannedResponsesPage.saveDetail()
    await cannedResponsesPage.expectToast('created')

    await cannedResponsesPage.goto()
    await cannedResponsesPage.search(uniqueText)
    await expect(page.locator('body')).toContainText(uniqueText)
  })

  test('should show audit log on the detail page', async ({ page }) => {
    const responseName = scope.name('audit-view')

    await cannedResponsesPage.openCreate()
    await cannedResponsesPage.fillResponseForm(responseName, 'Audit view content')
    await cannedResponsesPage.saveDetail()
    await cannedResponsesPage.expectToast('created')
    await page.waitForURL(/\/settings\/canned-responses\/[a-f0-9-]+$/)

    // Activity Log panel is rendered for existing responses.
    await expect(page.getByText('Activity Log')).toBeVisible({ timeout: 10000 })
  })
})
