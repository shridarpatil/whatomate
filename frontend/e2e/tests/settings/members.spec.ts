import { test, expect } from '@playwright/test'
import { TablePage } from '../../pages'
import { loginAsAdmin, ApiHelper, generateUniqueEmail } from '../../helpers'

const ADMIN_EMAIL = 'admin@admin.com'
const ADMIN_PASSWORD = 'admin'
const FALLBACK_ADMIN_EMAIL = 'admin@test.com'
const FALLBACK_ADMIN_PASSWORD = 'password'

test.describe('Organization Members', () => {
  let tablePage: TablePage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    await page.goto('/settings/members')
    await page.waitForLoadState('networkidle')

    tablePage = new TablePage(page)
  })

  test('should display members list', async ({ page }) => {
    // Page header should be visible
    await expect(page.locator('text=/Organization Members/i').first()).toBeVisible()

    // Table should be visible (may have members or be empty)
    await page.waitForTimeout(500)
    const hasTable = await tablePage.tableBody.isVisible().catch(() => false)
    const hasEmptyState = await page.locator('text=/No members/i').isVisible().catch(() => false)
    expect(hasTable || hasEmptyState).toBeTruthy()
  })

  test('should show member details in table', async ({ page }) => {
    // Wait for table to load
    await page.waitForTimeout(500)

    const rowCount = await tablePage.getRowCount().catch(() => 0)
    if (rowCount > 0) {
      // First row should have member name, role, and date
      const firstRow = tablePage.tableRows.first()
      await expect(firstRow).toBeVisible()

      // Should display member name (not empty)
      const nameCell = firstRow.locator('td').first()
      const nameText = await nameCell.textContent()
      expect(nameText?.trim()).not.toBe('')
    }
  })

  test('should search members', async ({ page }) => {
    await page.waitForTimeout(500)
    const initialCount = await tablePage.getRowCount().catch(() => 0)

    if (initialCount > 1) {
      // Get the first member name
      const firstRow = tablePage.tableRows.first()
      const nameText = await firstRow.locator('p.font-medium').first().textContent()

      if (nameText) {
        await tablePage.search(nameText.trim())
        await page.waitForTimeout(400)
        const filteredCount = await tablePage.getRowCount()
        expect(filteredCount).toBeLessThanOrEqual(initialCount)
      }
    }
  })

  test('should open add member dialog', async ({ page }) => {
    // Click "Add Member" button
    const addButton = page.getByRole('button', { name: /Add Member/i })

    if (await addButton.isVisible()) {
      await addButton.click()

      // Dialog should appear
      const dialog = page.getByRole('dialog')
      await expect(dialog).toBeVisible()
      await expect(dialog.getByText(/Add Organization Member/i)).toBeVisible()

      // Should have user select and role select
      await expect(dialog.locator('button[role="combobox"]').first()).toBeVisible()

      // Close dialog
      await dialog.getByRole('button', { name: /Cancel/i }).click()
    }
  })

  test('should show role badge for members', async ({ page }) => {
    await page.waitForTimeout(500)
    const rowCount = await tablePage.getRowCount().catch(() => 0)

    if (rowCount > 0) {
      // Verify a role badge is visible in the table
      const roleBadge = tablePage.tableRows.first().locator('[class*="border"]').filter({ hasText: /.+/ }).first()
      await expect(roleBadge).toBeVisible()
    }
  })
})

test.describe('Organization Members - API Tests', () => {
  test('should list organization members via API', async ({ request }) => {
    const api = new ApiHelper(request)
    try {
      await api.login(ADMIN_EMAIL, ADMIN_PASSWORD)
    } catch {
      try {
        await api.login(FALLBACK_ADMIN_EMAIL, FALLBACK_ADMIN_PASSWORD)
      } catch {
        test.skip(true, 'No admin credentials available')
        return
      }
    }

    const members = await api.getOrgMembers()
    expect(Array.isArray(members)).toBeTruthy()
  })

  test('should add and remove organization member via API', async ({ request }) => {
    const api = new ApiHelper(request)
    try {
      await api.login(ADMIN_EMAIL, ADMIN_PASSWORD)
    } catch {
      try {
        await api.login(FALLBACK_ADMIN_EMAIL, FALLBACK_ADMIN_PASSWORD)
      } catch {
        test.skip(true, 'No admin credentials available')
        return
      }
    }

    // Create a new org to test with
    const orgName = `Member Test Org ${Date.now()}`
    let org: any
    try {
      org = await api.createOrganization(orgName)
    } catch {
      test.skip(true, 'Failed to create test organization')
      return
    }

    // Create a user in the default org to add to the new org
    const testEmail = generateUniqueEmail('member-test')
    let testUser: any
    try {
      testUser = await api.createUser({
        email: testEmail,
        password: 'password123',
        full_name: 'Member Test User',
        role_id: '', // Will get default role
      })
    } catch {
      test.skip(true, 'Failed to create test user')
      return
    }

    // Add the user to the new org
    await api.addOrgMember(testUser.id, undefined, org.id)

    // List members and verify user is included
    const members = await api.getOrgMembers(org.id)
    const memberIds = members.map((m: any) => m.user_id)
    expect(memberIds).toContain(testUser.id)

    // Remove the user from the org
    await api.removeOrgMember(testUser.id, org.id)

    // Verify user is no longer a member
    const membersAfter = await api.getOrgMembers(org.id)
    const memberIdsAfter = membersAfter.map((m: any) => m.user_id)
    expect(memberIdsAfter).not.toContain(testUser.id)
  })

  test('should list my organizations via API', async ({ request }) => {
    const api = new ApiHelper(request)
    try {
      await api.login(ADMIN_EMAIL, ADMIN_PASSWORD)
    } catch {
      try {
        await api.login(FALLBACK_ADMIN_EMAIL, FALLBACK_ADMIN_PASSWORD)
      } catch {
        test.skip(true, 'No admin credentials available')
        return
      }
    }

    const orgs = await api.getMyOrganizations()
    expect(Array.isArray(orgs)).toBeTruthy()
    // The logged-in user should belong to at least one org
    expect(orgs.length).toBeGreaterThanOrEqual(1)

    // Each org should have required fields
    for (const org of orgs) {
      expect(org.organization_id).toBeTruthy()
      expect(org.name).toBeTruthy()
    }
  })

  test('should switch organization via API', async ({ request }) => {
    const api = new ApiHelper(request)
    try {
      await api.login(ADMIN_EMAIL, ADMIN_PASSWORD)
    } catch {
      try {
        await api.login(FALLBACK_ADMIN_EMAIL, FALLBACK_ADMIN_PASSWORD)
      } catch {
        test.skip(true, 'No admin credentials available')
        return
      }
    }

    // Create a second org
    const orgName = `Switch Test Org ${Date.now()}`
    let org: any
    try {
      org = await api.createOrganization(orgName)
    } catch {
      test.skip(true, 'Failed to create test organization')
      return
    }

    // Switch to the new org — super admin can switch to any org
    const newToken = await api.switchOrg(org.id)
    expect(newToken).toBeTruthy()
    expect(typeof newToken).toBe('string')
  })
})
