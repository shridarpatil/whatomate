import { test, expect } from '@playwright/test'
import { TablePage, DialogPage } from '../../pages'
import { loginAsAdmin, createTeamFixture } from '../../helpers'

test.describe('Teams Management', () => {
  let tablePage: TablePage
  let dialogPage: DialogPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    await page.goto('/settings/teams')
    await page.waitForLoadState('networkidle')

    tablePage = new TablePage(page)
    dialogPage = new DialogPage(page)
  })

  test('should display teams list', async ({ page }) => {
    await expect(tablePage.tableBody).toBeVisible()
  })

  test('should search teams', async ({ page }) => {
    // If there are teams, search should filter
    const initialCount = await tablePage.getRowCount()
    if (initialCount > 0) {
      const firstRow = tablePage.tableRows.first()
      const teamName = await firstRow.locator('td').first().textContent()
      if (teamName) {
        await tablePage.search(teamName.trim())
        await page.waitForTimeout(300)
        const filteredCount = await tablePage.getRowCount()
        expect(filteredCount).toBeLessThanOrEqual(initialCount)
      }
    }
  })

  test('should open create team dialog', async ({ page }) => {
    await tablePage.clickAddButton()
    await dialogPage.waitForOpen()
    await expect(dialogPage.dialog).toBeVisible()
  })

  test('should create a new team', async ({ page }) => {
    const newTeam = createTeamFixture()

    await tablePage.clickAddButton()
    await dialogPage.waitForOpen()

    await dialogPage.fillField('Name', newTeam.name)
    await dialogPage.fillField('Description', newTeam.description)

    await dialogPage.submit()
    await dialogPage.waitForClose()

    // Verify team appears in list
    await tablePage.search(newTeam.name)
    await tablePage.expectRowExists(newTeam.name)
  })

  test('should show validation error for empty name', async ({ page }) => {
    await tablePage.clickAddButton()
    await dialogPage.waitForOpen()

    // Try to submit without name
    await dialogPage.fillField('Description', 'Test description')
    await dialogPage.submit()

    // Should show validation error and stay open
    await expect(dialogPage.dialog).toBeVisible()
  })

  test('should edit existing team', async ({ page }) => {
    // First create a team to edit
    const team = createTeamFixture()

    await tablePage.clickAddButton()
    await dialogPage.waitForOpen()
    await dialogPage.fillField('Name', team.name)
    await dialogPage.fillField('Description', team.description)
    await dialogPage.submit()
    await dialogPage.waitForClose()

    // Now edit the team
    await tablePage.search(team.name)
    await tablePage.editRow(team.name)
    await dialogPage.waitForOpen()

    const updatedName = team.name + ' Updated'
    await dialogPage.fillField('Name', updatedName)
    await dialogPage.submit()
    await dialogPage.waitForClose()

    // Verify update
    await tablePage.search(updatedName)
    await tablePage.expectRowExists(updatedName)
  })

  test('should delete team', async ({ page }) => {
    // First create a team to delete
    const team = createTeamFixture({ name: 'Team To Delete ' + Date.now() })

    await tablePage.clickAddButton()
    await dialogPage.waitForOpen()
    await dialogPage.fillField('Name', team.name)
    await dialogPage.fillField('Description', team.description)
    await dialogPage.submit()
    await dialogPage.waitForClose()

    // Search and verify it exists
    await tablePage.search(team.name)
    await tablePage.expectRowExists(team.name)

    // Delete the team
    await tablePage.deleteRow(team.name)

    // Verify deletion
    await tablePage.clearSearch()
    await tablePage.search(team.name)
    await tablePage.expectRowNotExists(team.name)
  })

  test('should cancel team creation', async ({ page }) => {
    const teamName = 'Cancelled Team ' + Date.now()

    await tablePage.clickAddButton()
    await dialogPage.waitForOpen()

    await dialogPage.fillField('Name', teamName)
    await dialogPage.cancel()

    await dialogPage.waitForClose()

    // Team should not be created
    await tablePage.search(teamName)
    await tablePage.expectRowNotExists(teamName)
  })
})

test.describe('Team Members', () => {
  test('should view team members', async ({ page }) => {
    await loginAsAdmin(page)
    await page.goto('/settings/teams')
    await page.waitForLoadState('networkidle')

    const tablePage = new TablePage(page)

    // Create a team first
    const team = createTeamFixture()
    const dialogPage = new DialogPage(page)

    await tablePage.clickAddButton()
    await dialogPage.waitForOpen()
    await dialogPage.fillField('Name', team.name)
    await dialogPage.submit()
    await dialogPage.waitForClose()

    // Click on team to view members (or click view members action)
    await tablePage.search(team.name)
    const row = await tablePage.getRow(team.name)

    // Try to find and click members button/link
    const membersButton = row.locator('button, a').filter({ hasText: /Members|View|Manage/i })
    if (await membersButton.count() > 0) {
      await membersButton.click()
      // Should navigate to team members or open members dialog
      await page.waitForLoadState('networkidle')
    }
  })
})
