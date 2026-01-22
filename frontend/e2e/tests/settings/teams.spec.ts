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
  let tablePage: TablePage
  let dialogPage: DialogPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    await page.goto('/settings/teams')
    await page.waitForLoadState('networkidle')

    tablePage = new TablePage(page)
    dialogPage = new DialogPage(page)
  })

  test('should open members dialog', async ({ page }) => {
    // Create a team first
    const team = createTeamFixture()

    await tablePage.clickAddButton()
    await dialogPage.waitForOpen()
    await dialogPage.fillField('Name', team.name)
    await dialogPage.submit()
    await dialogPage.waitForClose()

    // Find the team and click manage members button
    await tablePage.search(team.name)
    const row = await tablePage.getRow(team.name)

    // Click the UserPlus button to manage members
    const manageMembersButton = row.getByRole('button').filter({ has: page.locator('svg') }).first()
    await manageMembersButton.click()

    // Verify members dialog opens
    const membersDialog = page.getByRole('dialog')
    await expect(membersDialog).toBeVisible()
    await expect(membersDialog.getByRole('heading', { name: /Team Members/i })).toBeVisible()
  })

  test('should add member to team and display name correctly', async ({ page }) => {
    // Create a team
    const team = createTeamFixture()

    await tablePage.clickAddButton()
    await dialogPage.waitForOpen()
    await dialogPage.fillField('Name', team.name)
    await dialogPage.submit()
    await dialogPage.waitForClose()

    // Open members dialog
    await tablePage.search(team.name)
    const row = await tablePage.getRow(team.name)

    // Click manage members button (UserPlus icon in actions)
    const actionButtons = row.locator('button')
    const manageMembersButton = actionButtons.nth(0) // First action button is manage members
    await manageMembersButton.click()

    // Wait for members dialog
    const membersDialog = page.getByRole('dialog')
    await expect(membersDialog).toBeVisible()

    // Find available users section and add one as agent
    const addAsAgentButton = membersDialog.getByRole('button', { name: /Add as Agent/i }).first()

    if (await addAsAgentButton.isVisible()) {
      await addAsAgentButton.click()

      // Wait for the member to be added
      await page.waitForTimeout(500)

      // Verify member appears in Current Members section with actual name (not "?")
      const currentMembersSection = membersDialog.locator('text=Current Members').locator('..')
      const memberItems = currentMembersSection.locator('.flex.items-center.justify-between')

      // Check that at least one member exists
      const memberCount = await memberItems.count()
      expect(memberCount).toBeGreaterThan(0)

      // Verify the member name is displayed (not showing "?")
      const firstMemberName = memberItems.first().locator('p.font-medium')
      const nameText = await firstMemberName.textContent()
      expect(nameText).toBeTruthy()
      expect(nameText).not.toBe('?')
      expect(nameText?.trim()).not.toBe('')
    }

    // Close dialog
    await membersDialog.getByRole('button', { name: 'Close' }).first().click()
  })

  test('should remove member from team', async ({ page }) => {
    // Create a team
    const team = createTeamFixture()

    await tablePage.clickAddButton()
    await dialogPage.waitForOpen()
    await dialogPage.fillField('Name', team.name)
    await dialogPage.submit()
    await dialogPage.waitForClose()

    // Open members dialog
    await tablePage.search(team.name)
    const row = await tablePage.getRow(team.name)
    const actionButtons = row.locator('button')
    await actionButtons.nth(0).click()

    const membersDialog = page.getByRole('dialog')
    await expect(membersDialog).toBeVisible()

    // Add a member first
    const addAsAgentButton = membersDialog.getByRole('button', { name: /Add as Agent/i }).first()

    if (await addAsAgentButton.isVisible()) {
      await addAsAgentButton.click()
      await page.waitForTimeout(500)

      // Get initial member count
      const currentMembersHeading = membersDialog.getByText(/Current Members \(\d+\)/)
      const initialText = await currentMembersHeading.textContent()
      const initialCount = parseInt(initialText?.match(/\((\d+)\)/)?.[1] || '0')

      // Find and click remove button (UserMinus icon)
      const removeMemberButton = membersDialog.locator('button').filter({ has: page.locator('svg.text-destructive') }).first()

      if (await removeMemberButton.isVisible()) {
        await removeMemberButton.click()
        await page.waitForTimeout(500)

        // Verify member count decreased or "No members yet" message appears
        const updatedHeading = membersDialog.getByRole('heading', { name: /Current Members/ })
        await expect(updatedHeading).toBeVisible()

        const updatedText = await updatedHeading.textContent()
        if (updatedText?.includes('Current Members')) {
          const updatedCount = parseInt(updatedText?.match(/\((\d+)\)/)?.[1] || '0')
          expect(updatedCount).toBeLessThan(initialCount)
        }
      }
    }

    // Close dialog
    await membersDialog.getByRole('button', { name: 'Close' }).first().click()
  })

  test('should display member role badge', async ({ page }) => {
    // Create a team
    const team = createTeamFixture()

    await tablePage.clickAddButton()
    await dialogPage.waitForOpen()
    await dialogPage.fillField('Name', team.name)
    await dialogPage.submit()
    await dialogPage.waitForClose()

    // Open members dialog
    await tablePage.search(team.name)
    const row = await tablePage.getRow(team.name)
    await row.locator('button').nth(0).click()

    const membersDialog = page.getByRole('dialog')
    await expect(membersDialog).toBeVisible()

    // Add a member as agent
    const addAsAgentButton = membersDialog.getByRole('button', { name: /Add as Agent/i }).first()

    if (await addAsAgentButton.isVisible()) {
      await addAsAgentButton.click()
      await page.waitForTimeout(500)

      // Verify role badge is displayed
      const roleBadge = membersDialog.locator('.text-xs').filter({ hasText: /agent|manager/i }).first()
      await expect(roleBadge).toBeVisible()
    }

    await membersDialog.getByRole('button', { name: 'Close' }).first().click()
  })
})
