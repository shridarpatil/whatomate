import { test, expect } from '@playwright/test'
import { loginAsAdmin } from '../../helpers'
import { ContactsPage } from '../../pages'
import * as path from 'path'
import * as fs from 'fs'
import * as os from 'os'

test.describe('Contacts Management', () => {
  let contactsPage: ContactsPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    contactsPage = new ContactsPage(page)
    await contactsPage.goto()
  })

  test('should display contacts page', async () => {
    await contactsPage.expectPageVisible()
    await expect(contactsPage.addButton).toBeVisible()
  })

  test('should open create contact dialog', async () => {
    await contactsPage.openCreateDialog()
    await contactsPage.expectDialogVisible()
    await expect(contactsPage.dialog).toContainText('Create Contact')
  })

  test('should show validation error for empty phone', async () => {
    await contactsPage.openCreateDialog()
    await contactsPage.submitDialog()
    await contactsPage.expectToast(/required/i)
  })

  test('should create a new contact', async () => {
    const phoneNumber = `91${Date.now().toString().slice(-10)}`
    const contactName = `Test Contact ${Date.now()}`

    await contactsPage.openCreateDialog()
    await contactsPage.fillContactForm(phoneNumber, contactName)
    await contactsPage.submitDialog()

    await contactsPage.expectToast(/created/i)
    await contactsPage.expectContactExists(phoneNumber)
  })

  test('should create contact with just phone number', async () => {
    const phoneNumber = `91${Date.now().toString().slice(-10)}`

    await contactsPage.openCreateDialog()
    await contactsPage.fillContactForm(phoneNumber)
    await contactsPage.submitDialog()

    await contactsPage.expectToast(/created/i)
    await contactsPage.expectContactExists(phoneNumber)
  })

  test('should edit existing contact', async ({ page }) => {
    // First create a contact
    const phoneNumber = `91${Date.now().toString().slice(-10)}`
    const originalName = `Original Name ${Date.now()}`

    await contactsPage.openCreateDialog()
    await contactsPage.fillContactForm(phoneNumber, originalName)
    await contactsPage.submitDialog()

    await contactsPage.expectToast(/created/i)
    await contactsPage.dismissToast(/created/i)

    // Wait for contact to appear
    await contactsPage.expectContactExists(phoneNumber)

    // Edit the contact via detail page
    const newName = `Updated Name ${Date.now()}`
    await contactsPage.editContact(phoneNumber)

    // Update profile name on the detail page
    const nameInput = page.locator('div.space-y-1\\.5:has(> label:has-text("Profile Name")) input').first()
    await nameInput.fill(newName)
    await page.waitForTimeout(300)

    // Save
    const saveBtn = page.getByRole('button', { name: /^Save$/i })
    await expect(saveBtn).toBeVisible({ timeout: 5000 })
    await saveBtn.click()
    await page.waitForLoadState('networkidle')

    // Verify in list
    await page.goto('/settings/contacts')
    await page.waitForLoadState('networkidle')
    await contactsPage.expectContactExists(newName)
  })

  test('should delete contact', async ({ page }) => {
    // First create a contact
    const phoneNumber = `91${Date.now().toString().slice(-10)}`
    const contactName = `Delete Contact ${Date.now()}`

    await contactsPage.openCreateDialog()
    await contactsPage.fillContactForm(phoneNumber, contactName)
    await contactsPage.submitDialog()

    await contactsPage.expectToast(/created/i)
    await contactsPage.dismissToast(/created/i)

    // Wait for contact to appear
    await contactsPage.expectContactExists(phoneNumber)

    // Delete the contact
    await contactsPage.deleteContact(phoneNumber)
    await contactsPage.confirmDelete()

    await contactsPage.expectToast(/deleted/i)
  })

  test('should search contacts', async ({ page }) => {
    // First create a contact with unique name
    const phoneNumber = `91${Date.now().toString().slice(-10)}`
    const uniqueName = `UniqueSearch${Date.now()}`

    await contactsPage.openCreateDialog()
    await contactsPage.fillContactForm(phoneNumber, uniqueName)
    await contactsPage.submitDialog()

    await contactsPage.expectToast(/created/i)
    await contactsPage.dismissToast(/created/i)

    // Search for the contact by name
    await contactsPage.search(uniqueName)
    await contactsPage.expectContactExists(uniqueName)

    // Search by phone number
    await contactsPage.search(phoneNumber)
    await contactsPage.expectContactExists(phoneNumber)
  })

  test('should prevent duplicate phone numbers', async ({ page }) => {
    const phoneNumber = `91${Date.now().toString().slice(-10)}`

    // Create first contact
    await contactsPage.openCreateDialog()
    await contactsPage.fillContactForm(phoneNumber, 'First Contact')
    await contactsPage.submitDialog()
    await contactsPage.expectToast(/created/i)

    // Wait for toast to disappear
    await page.locator('[data-sonner-toast]').waitFor({ state: 'hidden', timeout: 10000 })

    // Try to create duplicate
    await contactsPage.openCreateDialog()
    await contactsPage.fillContactForm(phoneNumber, 'Duplicate Contact')
    await contactsPage.submitDialog()

    await contactsPage.expectToast(/already exists/i)
  })

  test('should cancel contact creation', async () => {
    await contactsPage.openCreateDialog()
    await contactsPage.fillContactForm('919999999999', 'Cancelled Contact')
    await contactsPage.cancelDialog()
    await contactsPage.expectDialogHidden()
  })
})

test.describe('Contacts Import/Export', () => {
  let contactsPage: ContactsPage
  let tempDir: string

  test.beforeAll(() => {
    tempDir = fs.mkdtempSync(path.join(os.tmpdir(), 'e2e-contacts-'))
  })

  test.afterAll(() => {
    // Cleanup temp directory
    if (tempDir && fs.existsSync(tempDir)) {
      fs.rmSync(tempDir, { recursive: true })
    }
  })

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    contactsPage = new ContactsPage(page)
    await contactsPage.goto()
  })

  test('should open import/export dialog', async () => {
    await contactsPage.openImportExportDialog()
    await expect(contactsPage.importExportDialog).toBeVisible()
    await expect(contactsPage.importExportDialog).toContainText('Import/Export')
  })

  test('should show export columns', async () => {
    await contactsPage.openImportExportDialog()

    // Should show column checkboxes
    await expect(contactsPage.importExportDialog.locator('label').filter({ hasText: 'Phone Number' })).toBeVisible()
    await expect(contactsPage.importExportDialog.locator('label').filter({ hasText: 'Name' })).toBeVisible()
  })

  test('should switch between import and export tabs', async () => {
    await contactsPage.openImportExportDialog()

    // Should start on export tab
    await expect(contactsPage.importExportDialog.getByRole('button', { name: /Export CSV/i })).toBeVisible()

    // Switch to import tab
    await contactsPage.switchToImportTab()
    await expect(contactsPage.importExportDialog.getByRole('button', { name: /Import CSV/i })).toBeVisible()

    // Switch back to export tab
    await contactsPage.switchToExportTab()
    await expect(contactsPage.importExportDialog.getByRole('button', { name: /Export CSV/i })).toBeVisible()
  })

  test('should show required columns info on import tab', async () => {
    await contactsPage.openImportExportDialog()
    await contactsPage.switchToImportTab()

    // Should show required columns info
    await expect(contactsPage.importExportDialog).toContainText('Required columns')
    await expect(contactsPage.importExportDialog).toContainText('Phone Number')
  })

  test('should import contacts from CSV', async ({ page }) => {
    // Create a test CSV file
    const timestamp = Date.now()
    const csvContent = `Phone Number,Name
91${timestamp.toString().slice(-10)},Import Test 1
91${(timestamp + 1).toString().slice(-10)},Import Test 2`

    const csvPath = path.join(tempDir, `test-import-${timestamp}.csv`)
    fs.writeFileSync(csvPath, csvContent)

    await contactsPage.openImportExportDialog()
    await contactsPage.switchToImportTab()
    await contactsPage.uploadImportFile(csvPath)
    await contactsPage.clickImportButton()

    // Should show import results
    await contactsPage.expectImportResult(2, 0)
  })

  test('should update existing contacts on import with flag', async ({ page }) => {
    // First create a contact
    const phoneNumber = `91${Date.now().toString().slice(-10)}`
    const originalName = `Original Import ${Date.now()}`

    await contactsPage.openCreateDialog()
    await contactsPage.fillContactForm(phoneNumber, originalName)
    await contactsPage.submitDialog()
    await contactsPage.expectToast(/created/i)

    // Wait for toast to disappear completely before continuing
    await page.locator('[data-sonner-toast]').waitFor({ state: 'hidden', timeout: 10000 })

    // Create CSV with same phone number but different name
    const newName = `Updated Via Import ${Date.now()}`
    const csvContent = `Phone Number,Name
${phoneNumber},${newName}`

    const csvPath = path.join(tempDir, `test-update-${Date.now()}.csv`)
    fs.writeFileSync(csvPath, csvContent)

    await contactsPage.openImportExportDialog()
    await contactsPage.switchToImportTab()
    await contactsPage.toggleUpdateOnDuplicate()
    await contactsPage.uploadImportFile(csvPath)
    await contactsPage.clickImportButton()

    // Should show 0 created, 1 updated
    await contactsPage.expectImportResult(0, 1)
  })

  test('should cancel import/export dialog', async () => {
    await contactsPage.openImportExportDialog()
    await contactsPage.closeImportExportDialog()
    await expect(contactsPage.importExportDialog).not.toBeVisible()
  })
})

test.describe('Contacts in Chat View', () => {
  // Admin always has contacts:write permission, so the add-contact button must be visible.
  // The button has aria-label="Add Contact" (via $t('chat.addContact')) for stable selection.

  test('should show add contact button in chat view', async ({ page }) => {
    await loginAsAdmin(page)
    await page.goto('/chat')
    await page.waitForLoadState('networkidle')

    const addContactButton = page.getByRole('button', { name: /add contact/i }).first()
    await expect(addContactButton).toBeVisible({ timeout: 10000 })
  })

  test('should open create contact dialog from chat view', async ({ page }) => {
    await loginAsAdmin(page)
    await page.goto('/chat')
    await page.waitForLoadState('networkidle')

    await page.getByRole('button', { name: /add contact/i }).first().click()
    const dialog = page.locator('[role="dialog"][data-state="open"]')
    await expect(dialog).toBeVisible({ timeout: 5000 })
    await expect(dialog).toContainText(/Create Contact/i)
  })

  test('should create contact from chat view and navigate to chat', async ({ page }) => {
    await loginAsAdmin(page)
    await page.goto('/chat')
    await page.waitForLoadState('networkidle')

    const phoneNumber = `91${Date.now().toString().slice(-10)}`
    const contactName = `Chat Create ${Date.now()}`

    await page.getByRole('button', { name: /add contact/i }).first().click()
    const dialog = page.locator('[role="dialog"][data-state="open"]')
    await expect(dialog).toBeVisible({ timeout: 5000 })

    await dialog.locator('input').first().fill(phoneNumber)
    await dialog.locator('input').nth(1).fill(contactName)
    await dialog.getByRole('button', { name: /Create/i }).click()

    await expect(page.locator('[data-sonner-toast]').filter({ hasText: /created/i })).toBeVisible({ timeout: 10000 })
    await page.waitForURL(/\/chat\/.*/, { timeout: 10000 })
  })
})
