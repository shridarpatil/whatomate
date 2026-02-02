import { test, expect } from '@playwright/test'
import { loginAsAdmin } from '../../helpers'
import { AIContextsPage } from '../../pages'

test.describe('AI Contexts Management', () => {
  let aiContextsPage: AIContextsPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    aiContextsPage = new AIContextsPage(page)
    await aiContextsPage.goto()
  })

  test('should display AI contexts page', async () => {
    await aiContextsPage.expectPageVisible()
    await expect(aiContextsPage.addButton).toBeVisible()
  })

  test('should have back button', async () => {
    await expect(aiContextsPage.backButton).toBeVisible()
  })

  test('should open create AI context dialog', async () => {
    await aiContextsPage.openCreateDialog()
    await aiContextsPage.expectDialogVisible()
    await expect(aiContextsPage.dialog).toContainText('AI Context')
  })

  test('should show required fields in create dialog', async () => {
    await aiContextsPage.openCreateDialog()
    await expect(aiContextsPage.dialog.locator('label').filter({ hasText: /Name/i })).toBeVisible()
    await expect(aiContextsPage.dialog.locator('label').filter({ hasText: /Type/i })).toBeVisible()
    await expect(aiContextsPage.dialog.locator('label').filter({ hasText: /Content/i })).toBeVisible()
  })

  test('should show validation error for empty name', async () => {
    await aiContextsPage.openCreateDialog()
    await aiContextsPage.submitDialog()
    await aiContextsPage.expectToast('name')
  })

  test('should create a static AI context', async () => {
    const contextName = `FAQ Context ${Date.now()}`

    await aiContextsPage.openCreateDialog()
    await aiContextsPage.fillStaticContextForm({
      name: contextName,
      content: 'This is our product FAQ. We offer various services...',
      priority: 20
    })
    await aiContextsPage.submitDialog()

    await aiContextsPage.expectToast('created')
    await aiContextsPage.expectContextExists(contextName)
  })

  test('should create AI context with trigger keywords', async () => {
    const contextName = `Keyword Context ${Date.now()}`

    await aiContextsPage.openCreateDialog()
    await aiContextsPage.fillStaticContextForm({
      name: contextName,
      triggerKeywords: 'faq, help, support',
      content: 'This context is triggered by specific keywords.'
    })
    await aiContextsPage.submitDialog()

    await aiContextsPage.expectToast('created')
    await aiContextsPage.expectContextExists(contextName)
  })

  test('should create an API AI context', async () => {
    const contextName = `API Context ${Date.now()}`

    await aiContextsPage.openCreateDialog()
    await aiContextsPage.fillApiContextForm({
      name: contextName,
      content: 'Use this data to answer questions:',
      apiUrl: 'https://api.example.com/context',
      apiMethod: 'GET'
    })
    await aiContextsPage.submitDialog()

    await aiContextsPage.expectToast('created')
    await aiContextsPage.expectContextExists(contextName)
  })

  test('should show validation error for API context without URL', async () => {
    await aiContextsPage.openCreateDialog()
    await aiContextsPage.dialog.locator('input#name').fill('API Context')
    await aiContextsPage.selectOption('Type', 'API Fetch')
    await aiContextsPage.submitDialog()

    await aiContextsPage.expectToast('URL')
  })

  test('should edit existing AI context', async () => {
    // First create a context
    const contextName = `Edit Context ${Date.now()}`

    await aiContextsPage.openCreateDialog()
    await aiContextsPage.fillStaticContextForm({
      name: contextName,
      content: 'Original content'
    })
    await aiContextsPage.submitDialog()
    await aiContextsPage.expectToast('created')
    await aiContextsPage.dismissToast('created')

    // Edit the context
    await aiContextsPage.editContext(contextName)
    await aiContextsPage.dialog.locator('textarea#static_content').fill('Updated content')
    await aiContextsPage.submitDialog('Update')

    await aiContextsPage.expectToast('updated')
  })

  test('should delete AI context', async () => {
    // First create a context
    const contextName = `Delete Context ${Date.now()}`

    await aiContextsPage.openCreateDialog()
    await aiContextsPage.fillStaticContextForm({
      name: contextName,
      content: 'To be deleted'
    })
    await aiContextsPage.submitDialog()
    await aiContextsPage.expectToast('created')
    await aiContextsPage.dismissToast('created')

    // Delete the context
    await aiContextsPage.deleteContext(contextName)
    await expect(aiContextsPage.alertDialog).toContainText('cannot be undone')
    await aiContextsPage.confirmDelete()

    await aiContextsPage.expectToast('deleted')
  })

  test('should cancel AI context deletion', async () => {
    // First create a context
    const contextName = `Cancel Delete ${Date.now()}`

    await aiContextsPage.openCreateDialog()
    await aiContextsPage.fillStaticContextForm({
      name: contextName,
      content: 'Should not be deleted'
    })
    await aiContextsPage.submitDialog()
    await aiContextsPage.expectToast('created')
    await aiContextsPage.dismissToast('created')

    // Try to delete but cancel
    await aiContextsPage.deleteContext(contextName)
    await aiContextsPage.cancelDelete()

    // Context should still exist
    await aiContextsPage.expectContextExists(contextName)
  })

  test('should cancel AI context creation', async () => {
    await aiContextsPage.openCreateDialog()
    await aiContextsPage.dialog.locator('input#name').fill('Cancelled Context')
    await aiContextsPage.cancelDialog()
    await aiContextsPage.expectDialogHidden()
  })

  test('should show context type badge in table row', async () => {
    const contextName = `Type Badge ${Date.now()}`

    await aiContextsPage.openCreateDialog()
    await aiContextsPage.fillStaticContextForm({
      name: contextName,
      content: 'Static content'
    })
    await aiContextsPage.submitDialog()
    await aiContextsPage.expectToast('created')

    const row = aiContextsPage.getContextRow(contextName)
    await expect(row).toContainText('Static')
  })

  test('should show priority in table row', async () => {
    const contextName = `Priority Badge ${Date.now()}`

    await aiContextsPage.openCreateDialog()
    await aiContextsPage.fillStaticContextForm({
      name: contextName,
      content: 'Content with priority',
      priority: 50
    })
    await aiContextsPage.submitDialog()
    await aiContextsPage.expectToast('created')

    const row = aiContextsPage.getContextRow(contextName)
    await expect(row).toContainText('50')
  })

  test('should show trigger keywords in table row', async () => {
    const contextName = `Keywords Badge ${Date.now()}`
    const keywords = 'faq, help'

    await aiContextsPage.openCreateDialog()
    await aiContextsPage.fillStaticContextForm({
      name: contextName,
      triggerKeywords: keywords,
      content: 'Content with keywords'
    })
    await aiContextsPage.submitDialog()
    await aiContextsPage.expectToast('created')

    const row = aiContextsPage.getContextRow(contextName)
    await expect(row).toContainText('faq')
    await expect(row).toContainText('help')
  })

  test('should show active/inactive status in table row', async () => {
    const contextName = `Status Badge ${Date.now()}`

    await aiContextsPage.openCreateDialog()
    await aiContextsPage.fillStaticContextForm({
      name: contextName,
      content: 'Active context'
    })
    await aiContextsPage.submitDialog()
    await aiContextsPage.expectToast('created')

    const row = aiContextsPage.getContextRow(contextName)
    await expect(row).toContainText('Active')
  })
})

test.describe('AI Contexts - Context Types', () => {
  let aiContextsPage: AIContextsPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    aiContextsPage = new AIContextsPage(page)
    await aiContextsPage.goto()
  })

  test('should have Static Content type option', async () => {
    await aiContextsPage.openCreateDialog()
    await aiContextsPage.dialog.locator('button[role="combobox"]').first().click()
    await expect(aiContextsPage.page.locator('[role="option"]').filter({ hasText: 'Static Content' })).toBeVisible()
  })

  test('should have API Fetch type option', async () => {
    await aiContextsPage.openCreateDialog()
    await aiContextsPage.dialog.locator('button[role="combobox"]').first().click()
    await expect(aiContextsPage.page.locator('[role="option"]').filter({ hasText: 'API Fetch' })).toBeVisible()
  })

  test('should show API configuration fields for API type', async () => {
    await aiContextsPage.openCreateDialog()
    await aiContextsPage.selectOption('Type', 'API Fetch')

    await expect(aiContextsPage.dialog.locator('label').filter({ hasText: /Method/i })).toBeVisible()
    await expect(aiContextsPage.dialog.locator('label').filter({ hasText: /API URL/i })).toBeVisible()
    await expect(aiContextsPage.dialog.locator('label').filter({ hasText: /Headers/i })).toBeVisible()
    await expect(aiContextsPage.dialog.locator('label').filter({ hasText: /Response Path/i })).toBeVisible()
  })

  test('should hide API configuration fields for static type', async () => {
    await aiContextsPage.openCreateDialog()
    // Static is default, so API fields should not be visible
    await expect(aiContextsPage.dialog.locator('input#api_url')).not.toBeVisible()
  })
})

test.describe('AI Contexts - API Configuration', () => {
  let aiContextsPage: AIContextsPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    aiContextsPage = new AIContextsPage(page)
    await aiContextsPage.goto()
  })

  test('should have GET method option', async () => {
    await aiContextsPage.openCreateDialog()
    await aiContextsPage.selectOption('Type', 'API Fetch')

    const methodCombobox = aiContextsPage.dialog.locator('button[role="combobox"]').filter({ hasText: /GET|POST/i })
    await methodCombobox.click()
    await expect(aiContextsPage.page.locator('[role="option"]').filter({ hasText: 'GET' })).toBeVisible()
  })

  test('should have POST method option', async () => {
    await aiContextsPage.openCreateDialog()
    await aiContextsPage.selectOption('Type', 'API Fetch')

    const methodCombobox = aiContextsPage.dialog.locator('button[role="combobox"]').filter({ hasText: /GET|POST/i })
    await methodCombobox.click()
    await expect(aiContextsPage.page.locator('[role="option"]').filter({ hasText: 'POST' })).toBeVisible()
  })

  test('should create API context with headers', async () => {
    const contextName = `Headers Context ${Date.now()}`

    await aiContextsPage.openCreateDialog()
    await aiContextsPage.fillApiContextForm({
      name: contextName,
      apiUrl: 'https://api.example.com/data',
      apiHeaders: '{"Authorization": "Bearer token123"}'
    })
    await aiContextsPage.submitDialog()

    await aiContextsPage.expectToast('created')
    await aiContextsPage.expectContextExists(contextName)
  })

  test('should show validation error for invalid JSON headers', async () => {
    await aiContextsPage.openCreateDialog()
    await aiContextsPage.dialog.locator('input#name').fill('Invalid Headers')
    await aiContextsPage.selectOption('Type', 'API Fetch')
    await aiContextsPage.dialog.locator('input#api_url').fill('https://api.example.com')
    await aiContextsPage.dialog.locator('textarea#api_headers').fill('invalid json')
    await aiContextsPage.submitDialog()

    await aiContextsPage.expectToast('Invalid JSON')
  })

  test('should create API context with response path', async () => {
    const contextName = `Response Path Context ${Date.now()}`

    await aiContextsPage.openCreateDialog()
    await aiContextsPage.fillApiContextForm({
      name: contextName,
      apiUrl: 'https://api.example.com/data',
      apiResponsePath: 'data.context'
    })
    await aiContextsPage.submitDialog()

    await aiContextsPage.expectToast('created')
    await aiContextsPage.expectContextExists(contextName)
  })

  test('should show API Fetch badge for API contexts', async () => {
    const contextName = `API Badge ${Date.now()}`

    await aiContextsPage.openCreateDialog()
    await aiContextsPage.fillApiContextForm({
      name: contextName,
      apiUrl: 'https://api.example.com/data'
    })
    await aiContextsPage.submitDialog()
    await aiContextsPage.expectToast('created')

    const row = aiContextsPage.getContextRow(contextName)
    await expect(row).toContainText('API Fetch')
  })
})

test.describe('AI Contexts - Priority', () => {
  let aiContextsPage: AIContextsPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    aiContextsPage = new AIContextsPage(page)
    await aiContextsPage.goto()
  })

  test('should have priority field', async () => {
    await aiContextsPage.openCreateDialog()
    await expect(aiContextsPage.dialog.locator('input#priority')).toBeVisible()
  })

  test('should create context with custom priority', async () => {
    const contextName = `Custom Priority ${Date.now()}`

    await aiContextsPage.openCreateDialog()
    await aiContextsPage.fillStaticContextForm({
      name: contextName,
      content: 'High priority content',
      priority: 100
    })
    await aiContextsPage.submitDialog()

    await aiContextsPage.expectToast('created')
    const row = aiContextsPage.getContextRow(contextName)
    await expect(row).toContainText('100')
  })
})

test.describe('AI Contexts - Enabled Toggle', () => {
  let aiContextsPage: AIContextsPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    aiContextsPage = new AIContextsPage(page)
    await aiContextsPage.goto()
  })

  test('should have enabled toggle', async () => {
    await aiContextsPage.openCreateDialog()
    await expect(aiContextsPage.dialog.locator('button[role="switch"]')).toBeVisible()
  })

  test('should toggle enabled switch', async () => {
    await aiContextsPage.openCreateDialog()

    // Toggle the switch - it starts as checked (enabled)
    const switchEl = aiContextsPage.dialog.locator('button[role="switch"]')
    await expect(switchEl).toHaveAttribute('data-state', 'checked')

    // Click to disable
    await switchEl.click()
    await expect(switchEl).toHaveAttribute('data-state', 'unchecked')

    // Click again to enable
    await switchEl.click()
    await expect(switchEl).toHaveAttribute('data-state', 'checked')
  })
})
