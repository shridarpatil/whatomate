import { test, expect } from '@playwright/test'
import { TablePage, DialogPage } from '../../pages'
import { loginAsAdmin, createUserFixture } from '../../helpers'

test.describe('Users Management', () => {
  let tablePage: TablePage
  let dialogPage: DialogPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    await page.goto('/settings/users')
    await page.waitForLoadState('networkidle')

    tablePage = new TablePage(page)
    dialogPage = new DialogPage(page)
  })

  test('should display users list', async ({ page }) => {
    // Should show table with users
    await expect(tablePage.tableBody).toBeVisible()
    // At least the admin user should exist
    const rowCount = await tablePage.getRowCount()
    expect(rowCount).toBeGreaterThan(0)
  })

  test('should search users', async ({ page }) => {
    await tablePage.search('admin')
    // Should filter results
    await page.waitForTimeout(500)
    await tablePage.expectRowExists('admin')
  })

  test('should open create user dialog', async ({ page }) => {
    await tablePage.clickAddButton()
    await dialogPage.waitForOpen()
    await expect(dialogPage.dialog).toBeVisible()
  })

  test('should create a new user', async ({ page }) => {
    const newUser = createUserFixture()

    await tablePage.clickAddButton()
    await dialogPage.waitForOpen()

    await dialogPage.fillField('Email', newUser.email)
    await dialogPage.fillField('Name', newUser.fullName)
    await dialogPage.fillField('Password', newUser.password)
    await dialogPage.selectOption('Role', 'Agent')

    await dialogPage.submit()
    await dialogPage.waitForClose()

    // Verify user appears in list
    await tablePage.search(newUser.email)
    await tablePage.expectRowExists(newUser.email)
  })

  test('should show validation error for invalid email', async ({ page }) => {
    await tablePage.clickAddButton()
    await dialogPage.waitForOpen()

    await dialogPage.fillField('Email', 'invalid-email')
    await dialogPage.fillField('Name', 'Test User')
    await dialogPage.fillField('Password', 'password123')

    await dialogPage.submit()

    // Should show validation error and stay open
    await expect(dialogPage.dialog).toBeVisible()
  })

  test('should edit existing user', async ({ page }) => {
    // First create a user to edit
    const user = createUserFixture()

    await tablePage.clickAddButton()
    await dialogPage.waitForOpen()
    await dialogPage.fillField('Email', user.email)
    await dialogPage.fillField('Name', user.fullName)
    await dialogPage.fillField('Password', user.password)
    await dialogPage.selectOption('Role', 'Agent')
    await dialogPage.submit()
    await dialogPage.waitForClose()

    // Now edit the user
    await tablePage.search(user.email)
    await tablePage.editRow(user.email)
    await dialogPage.waitForOpen()

    const updatedName = 'Updated User Name'
    await dialogPage.fillField('Name', updatedName)
    await dialogPage.submit()
    await dialogPage.waitForClose()

    // Verify update
    await tablePage.expectRowExists(updatedName)
  })

  test('should delete user', async ({ page }) => {
    // First create a user to delete
    const user = createUserFixture({ fullName: 'User To Delete' })

    await tablePage.clickAddButton()
    await dialogPage.waitForOpen()
    await dialogPage.fillField('Email', user.email)
    await dialogPage.fillField('Name', user.fullName)
    await dialogPage.fillField('Password', user.password)
    await dialogPage.selectOption('Role', 'Agent')
    await dialogPage.submit()
    await dialogPage.waitForClose()

    // Search for the user
    await tablePage.search(user.email)
    await tablePage.expectRowExists(user.email)

    // Delete the user
    await tablePage.deleteRow(user.email)

    // Verify deletion
    await tablePage.clearSearch()
    await tablePage.search(user.email)
    await tablePage.expectRowNotExists(user.email)
  })

  test('should cancel user creation', async ({ page }) => {
    await tablePage.clickAddButton()
    await dialogPage.waitForOpen()

    await dialogPage.fillField('Email', 'cancelled@test.com')
    await dialogPage.cancel()

    await dialogPage.waitForClose()
    // User should not be created
    await tablePage.search('cancelled@test.com')
    await tablePage.expectRowNotExists('cancelled@test.com')
  })
})

test.describe('Users - Role-based Access', () => {
  test.skip('agent should not access users page', async ({ page }) => {
    // Skip: Role-based access control may be implemented differently
    // This test should be updated based on actual RBAC implementation
  })
})
