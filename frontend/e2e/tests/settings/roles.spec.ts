import { test, expect } from '@playwright/test'
import { TablePage, DialogPage } from '../../pages'
import { loginAsAdmin } from '../../helpers'

test.describe('Roles Management', () => {
  let tablePage: TablePage
  let dialogPage: DialogPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    await page.goto('/settings/roles')
    await page.waitForLoadState('networkidle')

    tablePage = new TablePage(page)
    dialogPage = new DialogPage(page)
  })

  test('should display roles list', async ({ page }) => {
    // Should show table with roles
    await expect(tablePage.tableBody).toBeVisible()
    // System roles (admin, manager, agent) should exist
    const rowCount = await tablePage.getRowCount()
    expect(rowCount).toBeGreaterThan(0)
  })

  test('should show system roles with badges', async ({ page }) => {
    // System roles should have a "System" badge
    await expect(page.locator('text=System').first()).toBeVisible()
  })

  test('should search roles', async ({ page }) => {
    await tablePage.search('admin')
    await page.waitForTimeout(500)
    await tablePage.expectRowExists('admin')
  })

  test('should open create role dialog', async ({ page }) => {
    await tablePage.clickAddButton()
    await dialogPage.waitForOpen()
    await expect(dialogPage.dialog).toBeVisible()
    // Should show permissions section (label inside dialog)
    await expect(dialogPage.dialog.locator('label').filter({ hasText: 'Permissions' })).toBeVisible()
  })

  test('should create a new custom role', async ({ page }) => {
    const roleName = `Test Role ${Date.now()}`

    await tablePage.clickAddButton()
    await dialogPage.waitForOpen()

    await dialogPage.fillField('Name', roleName)
    await dialogPage.fillField('Description', 'A custom test role for E2E testing')

    // Select some permissions - click a checkbox in the permissions accordion
    const permissionCheckbox = dialogPage.dialog.locator('button[role="checkbox"]').first()
    if (await permissionCheckbox.isVisible()) {
      await permissionCheckbox.click()
    }

    await dialogPage.submit()
    await dialogPage.waitForClose()

    // Verify role appears in list
    await tablePage.search(roleName)
    await tablePage.expectRowExists(roleName)
  })

  test('should require role name', async ({ page }) => {
    await tablePage.clickAddButton()
    await dialogPage.waitForOpen()

    // Try to submit without name
    await dialogPage.fillField('Description', 'Role without name')
    await dialogPage.submit()

    // Should show error and stay open
    await expect(page.locator('text=Name is required').or(page.locator('text=required'))).toBeVisible()
  })

  test('should view system role permissions (read-only)', async ({ page }) => {
    // Find and click edit on a system role
    await tablePage.search('admin')
    await tablePage.editRow('admin')
    await dialogPage.waitForOpen()

    // Dialog title should indicate view mode for system roles
    const dialogTitle = dialogPage.dialog.getByRole('heading')
    await expect(dialogTitle.filter({ hasText: 'View Role' })).toBeVisible()

    // Should show message about system roles being read-only
    await expect(dialogPage.dialog.locator('text=System roles cannot be modified')).toBeVisible()

    // Close the view dialog (uses "Close" button, not "Cancel")
    await dialogPage.dialog.getByRole('button', { name: 'Close' }).first().click()
    await dialogPage.waitForClose()
  })

  test('should edit custom role', async ({ page }) => {
    // First create a role to edit
    const originalName = `Edit Role ${Date.now()}`

    await tablePage.clickAddButton()
    await dialogPage.waitForOpen()
    await dialogPage.fillField('Name', originalName)
    await dialogPage.fillField('Description', 'Original description')
    await dialogPage.submit()
    await dialogPage.waitForClose()

    // Now edit the role
    await tablePage.search(originalName)
    await tablePage.editRow(originalName)
    await dialogPage.waitForOpen()

    const updatedName = `Updated Role ${Date.now()}`
    await dialogPage.fillField('Name', updatedName)
    await dialogPage.fillField('Description', 'Updated description')
    await dialogPage.submit()
    await dialogPage.waitForClose()

    // Verify update
    await tablePage.search(updatedName)
    await tablePage.expectRowExists(updatedName)
  })

  test('should delete custom role', async ({ page }) => {
    // First create a role to delete
    const roleName = `Delete Role ${Date.now()}`

    await tablePage.clickAddButton()
    await dialogPage.waitForOpen()
    await dialogPage.fillField('Name', roleName)
    await dialogPage.submit()
    await dialogPage.waitForClose()

    // Search and delete
    await tablePage.search(roleName)
    await tablePage.expectRowExists(roleName)
    await tablePage.deleteRow(roleName)

    // Verify deletion
    await tablePage.clearSearch()
    await tablePage.search(roleName)
    await tablePage.expectRowNotExists(roleName)
  })

  test('should not allow deleting system roles', async ({ page }) => {
    // System roles should not have a delete button
    await tablePage.search('admin')

    // The delete button should be hidden or disabled for system roles
    const deleteButton = page.locator(`tr:has-text("admin") button:has-text("Delete")`)
    await expect(deleteButton).not.toBeVisible().catch(async () => {
      // If visible, it should be disabled
      await expect(deleteButton).toBeDisabled()
    })
  })

  test('should show delete confirmation when deleting custom role', async ({ page }) => {
    // Create a role to delete
    const roleName = `Role To Delete ${Date.now()}`

    await tablePage.clickAddButton()
    await dialogPage.waitForOpen()
    await dialogPage.fillField('Name', roleName)
    await dialogPage.submit()
    await dialogPage.waitForClose()

    // Search for the role
    await tablePage.search(roleName)
    await tablePage.expectRowExists(roleName)

    // Click delete and verify confirmation dialog appears
    const deleteButton = page.locator(`tr:has-text("${roleName}") button:has(svg.text-destructive)`)
    await deleteButton.click()

    // Should show confirmation dialog
    const alertDialog = page.locator('[role="alertdialog"]')
    await expect(alertDialog).toBeVisible()
    await expect(alertDialog).toContainText('delete')

    // Cancel to clean up
    await alertDialog.getByRole('button', { name: 'Cancel' }).click()
  })

  test('should toggle default role flag', async ({ page }) => {
    // Create a role and set it as default
    const roleName = `Default Role ${Date.now()}`

    await tablePage.clickAddButton()
    await dialogPage.waitForOpen()
    await dialogPage.fillField('Name', roleName)

    // Toggle the default switch
    const defaultSwitch = page.locator('button[role="switch"]#is_default, [id="is_default"]')
    await defaultSwitch.click()

    await dialogPage.submit()
    await dialogPage.waitForClose()

    // Verify the role shows default badge (the badge div, not the role name)
    await tablePage.search(roleName)
    const defaultBadge = page.locator(`tr:has-text("${roleName}") .rounded-full`).filter({ hasText: 'Default' })
    await expect(defaultBadge).toBeVisible()
  })

  test('should cancel role creation', async ({ page }) => {
    await tablePage.clickAddButton()
    await dialogPage.waitForOpen()

    await dialogPage.fillField('Name', 'Cancelled Role')
    await dialogPage.cancel()

    await dialogPage.waitForClose()
    // Role should not be created
    await tablePage.search('Cancelled Role')
    await tablePage.expectRowNotExists('Cancelled Role')
  })

  test('should display permission count in role list', async ({ page }) => {
    // Roles should show permission count
    const permissionBadge = page.locator('tr td >> text=/\\d+/').first()
    await expect(permissionBadge).toBeVisible()
  })

  test('should navigate to roles from settings', async ({ page }) => {
    // Go to settings first
    await page.goto('/settings')
    await page.waitForLoadState('networkidle')

    // Click on Roles card/link
    await page.locator('text=Roles').click()

    // Should be on roles page
    await expect(page).toHaveURL(/\/settings\/roles/)
    await expect(page.locator('h1:has-text("Roles")')).toBeVisible()
  })
})

test.describe('Roles - Permissions Selection', () => {
  test('should display permission groups in accordion', async ({ page }) => {
    await loginAsAdmin(page)
    await page.goto('/settings/roles')
    await page.waitForLoadState('networkidle')

    const tablePage = new TablePage(page)
    const dialogPage = new DialogPage(page)

    await tablePage.clickAddButton()
    await dialogPage.waitForOpen()

    // Should show permission groups (Users, Contacts, Messages, etc.)
    await expect(page.locator('text=Users').first()).toBeVisible()
    await expect(page.locator('text=Contacts').first()).toBeVisible()
  })

  test('should select all permissions in a group', async ({ page }) => {
    await loginAsAdmin(page)
    await page.goto('/settings/roles')
    await page.waitForLoadState('networkidle')

    const tablePage = new TablePage(page)
    const dialogPage = new DialogPage(page)

    await tablePage.clickAddButton()
    await dialogPage.waitForOpen()

    // Click the group checkbox to select all
    const groupCheckbox = page.locator('[data-testid="group-users-checkbox"]').or(
      page.locator('button[role="checkbox"]').first()
    )
    await groupCheckbox.click()

    // Permission count should increase
    const selectedCount = page.locator('text=/\\d+ selected/')
    await expect(selectedCount).toBeVisible()
  })
})
