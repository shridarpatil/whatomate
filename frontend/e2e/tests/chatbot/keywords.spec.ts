import { test, expect } from '@playwright/test'
import { loginAsAdmin } from '../../helpers'
import { KeywordsPage } from '../../pages'

test.describe('Keyword Rules Management', () => {
  let keywordsPage: KeywordsPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    keywordsPage = new KeywordsPage(page)
    await keywordsPage.goto()
  })

  test('should display keywords page', async () => {
    await keywordsPage.expectPageVisible()
    await expect(keywordsPage.addButton).toBeVisible()
  })

  test('should have back button', async () => {
    await expect(keywordsPage.backButton).toBeVisible()
  })

  test('should have search input', async () => {
    await expect(keywordsPage.searchInput).toBeVisible()
  })

  test('should open create keyword rule dialog', async () => {
    await keywordsPage.openCreateDialog()
    await keywordsPage.expectDialogVisible()
    await expect(keywordsPage.dialog).toContainText('Keyword Rule')
  })

  test('should show required fields in create dialog', async () => {
    await keywordsPage.openCreateDialog()
    await expect(keywordsPage.dialog.locator('label').filter({ hasText: /Keywords/i })).toBeVisible()
    await expect(keywordsPage.dialog.locator('label').filter({ hasText: /Match Type/i })).toBeVisible()
    await expect(keywordsPage.dialog.locator('label').filter({ hasText: /Response Type/i })).toBeVisible()
  })

  test('should show validation error for empty keywords', async () => {
    await keywordsPage.openCreateDialog()
    await keywordsPage.submitDialog()
    await keywordsPage.expectToast('keyword')
  })

  test('should show validation error for empty response', async () => {
    await keywordsPage.openCreateDialog()
    await keywordsPage.dialog.locator('input#keywords').fill('hello, hi')
    await keywordsPage.submitDialog()
    await keywordsPage.expectToast('response')
  })

  test('should create a text keyword rule', async () => {
    const keyword = `test${Date.now()}`

    await keywordsPage.openCreateDialog()
    await keywordsPage.fillKeywordForm({
      keywords: keyword,
      matchType: 'contains',
      responseType: 'text',
      response: 'Hello! How can I help you?',
      priority: 10
    })
    await keywordsPage.submitDialog()

    await keywordsPage.expectToast('created')
    await keywordsPage.expectRuleExists(keyword)
  })

  test('should create a transfer keyword rule', async () => {
    const keyword = `agent${Date.now()}`

    await keywordsPage.openCreateDialog()
    await keywordsPage.fillKeywordForm({
      keywords: keyword,
      matchType: 'exact',
      responseType: 'transfer',
      response: 'Connecting you with an agent...'
    })
    await keywordsPage.submitDialog()

    await keywordsPage.expectToast('created')
    await keywordsPage.expectRuleExists(keyword)
  })

  test('should create keyword rule with multiple keywords', async () => {
    const keywords = `hello${Date.now()}, hi${Date.now()}, hey${Date.now()}`

    await keywordsPage.openCreateDialog()
    await keywordsPage.fillKeywordForm({
      keywords: keywords,
      response: 'Welcome! How can I assist you today?'
    })
    await keywordsPage.submitDialog()

    await keywordsPage.expectToast('created')
  })

  test('should create keyword rule with regex match type', async () => {
    const keyword = `regex${Date.now()}`

    await keywordsPage.openCreateDialog()
    await keywordsPage.fillKeywordForm({
      keywords: keyword,
      matchType: 'regex',
      response: 'Pattern matched!'
    })
    await keywordsPage.submitDialog()

    await keywordsPage.expectToast('created')
  })

  test('should edit existing keyword rule', async () => {
    // First create a rule
    const keyword = `edit${Date.now()}`

    await keywordsPage.openCreateDialog()
    await keywordsPage.fillKeywordForm({
      keywords: keyword,
      response: 'Original response'
    })
    await keywordsPage.submitDialog()
    await keywordsPage.expectToast('created')
    await keywordsPage.dismissToast('created')

    // Edit the rule
    await keywordsPage.editRule(keyword)
    await keywordsPage.dialog.locator('textarea#response').fill('Updated response')
    await keywordsPage.submitDialog('Update')

    await keywordsPage.expectToast('updated')
  })

  test('should delete keyword rule', async ({ page }) => {
    // First create a rule
    const keyword = `delete${Date.now()}`

    await keywordsPage.openCreateDialog()
    await keywordsPage.fillKeywordForm({
      keywords: keyword,
      response: 'To be deleted'
    })
    await keywordsPage.submitDialog()
    await keywordsPage.expectToast('created')
    await keywordsPage.dismissToast('created')

    // Delete the rule
    await keywordsPage.deleteRule(keyword)
    await expect(keywordsPage.alertDialog).toContainText('cannot be undone')
    await keywordsPage.confirmDelete()

    // Wait for delete to complete - check for toast or rule disappearing
    const toast = page.locator('[data-sonner-toast]').filter({ hasText: /deleted/i })
    await toast.waitFor({ state: 'visible', timeout: 10000 }).catch(() => {})

    // Verify rule is gone
    await keywordsPage.expectRuleNotExists(keyword)
  })

  test('should cancel keyword rule deletion', async () => {
    // First create a rule
    const keyword = `cancel${Date.now()}`

    await keywordsPage.openCreateDialog()
    await keywordsPage.fillKeywordForm({
      keywords: keyword,
      response: 'Should not be deleted'
    })
    await keywordsPage.submitDialog()
    await keywordsPage.expectToast('created')
    await keywordsPage.dismissToast('created')

    // Try to delete but cancel
    await keywordsPage.deleteRule(keyword)
    await keywordsPage.cancelDelete()

    // Rule should still exist
    await keywordsPage.expectRuleExists(keyword)
  })

  test('should cancel keyword rule creation', async () => {
    await keywordsPage.openCreateDialog()
    await keywordsPage.dialog.locator('input#keywords').fill('cancelled')
    await keywordsPage.cancelDialog()
    await keywordsPage.expectDialogHidden()
  })

  test('should search keyword rules', async () => {
    // Create a rule with unique keyword
    const uniqueKeyword = `unique${Date.now()}`

    await keywordsPage.openCreateDialog()
    await keywordsPage.fillKeywordForm({
      keywords: uniqueKeyword,
      response: 'Unique response'
    })
    await keywordsPage.submitDialog()
    await keywordsPage.expectToast('created')

    // Search for the keyword
    await keywordsPage.search(uniqueKeyword)
    await keywordsPage.expectRuleExists(uniqueKeyword)
  })

  test('should show match type badge in rule card', async ({ page }) => {
    const keyword = `badge${Date.now()}`

    await keywordsPage.openCreateDialog()
    await keywordsPage.fillKeywordForm({
      keywords: keyword,
      matchType: 'exact',
      response: 'Exact match response'
    })
    await keywordsPage.submitDialog()
    await keywordsPage.expectToast('created')

    const card = keywordsPage.getRuleCard(keyword)
    await expect(card).toContainText('exact')
  })

  test('should show priority in rule card', async ({ page }) => {
    const keyword = `priority${Date.now()}`

    await keywordsPage.openCreateDialog()
    await keywordsPage.fillKeywordForm({
      keywords: keyword,
      response: 'Priority response',
      priority: 50
    })
    await keywordsPage.submitDialog()
    await keywordsPage.expectToast('created')

    const card = keywordsPage.getRuleCard(keyword)
    await expect(card).toContainText('50')
  })

  test('should show transfer badge for transfer rules', async ({ page }) => {
    const keyword = `transfer${Date.now()}`

    await keywordsPage.openCreateDialog()
    await keywordsPage.fillKeywordForm({
      keywords: keyword,
      responseType: 'transfer',
      response: 'Transferring...'
    })
    await keywordsPage.submitDialog()
    await keywordsPage.expectToast('created')

    const card = keywordsPage.getRuleCard(keyword)
    await expect(card).toContainText('Transfer')
  })

  test('should show active/inactive status badge', async ({ page }) => {
    const keyword = `status${Date.now()}`

    await keywordsPage.openCreateDialog()
    await keywordsPage.fillKeywordForm({
      keywords: keyword,
      response: 'Status response'
    })
    await keywordsPage.submitDialog()
    await keywordsPage.expectToast('created')

    const card = keywordsPage.getRuleCard(keyword)
    await expect(card).toContainText('Active')
  })
})

test.describe('Keyword Rules - Match Types', () => {
  let keywordsPage: KeywordsPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    keywordsPage = new KeywordsPage(page)
    await keywordsPage.goto()
  })

  test('should have Contains match type option', async () => {
    await keywordsPage.openCreateDialog()
    await keywordsPage.dialog.locator('button[role="combobox"]').first().click()
    await expect(keywordsPage.page.locator('[role="option"]').filter({ hasText: 'Contains' })).toBeVisible()
  })

  test('should have Exact Match type option', async () => {
    await keywordsPage.openCreateDialog()
    await keywordsPage.dialog.locator('button[role="combobox"]').first().click()
    await expect(keywordsPage.page.locator('[role="option"]').filter({ hasText: 'Exact Match' })).toBeVisible()
  })

  test('should have Regex match type option', async () => {
    await keywordsPage.openCreateDialog()
    await keywordsPage.dialog.locator('button[role="combobox"]').first().click()
    await expect(keywordsPage.page.locator('[role="option"]').filter({ hasText: 'Regex' })).toBeVisible()
  })
})

test.describe('Keyword Rules - Response Types', () => {
  let keywordsPage: KeywordsPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    keywordsPage = new KeywordsPage(page)
    await keywordsPage.goto()
  })

  test('should have Text Response type option', async () => {
    await keywordsPage.openCreateDialog()
    // Response Type is the second combobox
    const responseTypeCombobox = keywordsPage.dialog.locator('button[role="combobox"]').nth(1)
    await responseTypeCombobox.click()
    await expect(keywordsPage.page.locator('[role="option"]').filter({ hasText: 'Text Response' })).toBeVisible()
  })

  test('should have Transfer to Agent type option', async () => {
    await keywordsPage.openCreateDialog()
    const responseTypeCombobox = keywordsPage.dialog.locator('button[role="combobox"]').nth(1)
    await responseTypeCombobox.click()
    await expect(keywordsPage.page.locator('[role="option"]').filter({ hasText: 'Transfer to Agent' })).toBeVisible()
  })

  test('should show Add Button option for text responses', async () => {
    await keywordsPage.openCreateDialog()
    await expect(keywordsPage.dialog.getByRole('button', { name: /Add Button/i })).toBeVisible()
  })

  test('should hide Add Button option for transfer responses', async () => {
    await keywordsPage.openCreateDialog()
    await keywordsPage.selectOption('Response Type', 'Transfer to Agent')
    await expect(keywordsPage.dialog.getByRole('button', { name: /Add Button/i })).not.toBeVisible()
  })
})

test.describe('Keyword Rules - Buttons', () => {
  let keywordsPage: KeywordsPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    keywordsPage = new KeywordsPage(page)
    await keywordsPage.goto()
  })

  test('should add button to keyword rule', async () => {
    await keywordsPage.openCreateDialog()
    await keywordsPage.dialog.getByRole('button', { name: /Add Button/i }).click()

    // Should show button input fields
    await expect(keywordsPage.dialog.locator('input[placeholder="Button ID"]')).toBeVisible()
    await expect(keywordsPage.dialog.locator('input[placeholder="Button Title"]')).toBeVisible()
  })

  test('should remove button from keyword rule', async () => {
    await keywordsPage.openCreateDialog()
    await keywordsPage.dialog.getByRole('button', { name: /Add Button/i }).click()

    // Verify button inputs are visible
    await expect(keywordsPage.dialog.locator('input[placeholder="Button ID"]')).toBeVisible()

    // Remove the button - the delete button is inside the button row with inputs
    const buttonRow = keywordsPage.dialog.locator('.flex.items-center.gap-2').filter({ has: keywordsPage.page.locator('input[placeholder="Button ID"]') })
    await buttonRow.locator('button').last().click()

    // Button fields should be removed
    await expect(keywordsPage.dialog.locator('input[placeholder="Button ID"]')).not.toBeVisible()
  })
})
