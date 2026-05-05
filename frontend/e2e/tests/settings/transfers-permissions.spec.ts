import { test, expect, Page } from '@playwright/test'
import { ApiHelper, generateUniqueName, generateUniqueEmail } from '../../helpers'

/**
 * Regression: a user on a CUSTOM role (not literally named admin/manager)
 * with transfers:read + transfers:write should see all three tabs on the
 * Transfers page. Before the role-name-to-permission fix, the tabs were
 * gated on `userRole === 'admin' || userRole === 'manager'`, so custom
 * roles silently saw the agent-only view.
 */

async function loginWithCredentials(page: Page, email: string, password: string) {
  await page.goto('/login')
  await page.locator('input[name="email"], input[type="email"]').fill(email)
  await page.locator('input[name="password"], input[type="password"]').fill(password)
  await page.locator('button[type="submit"]').click()
  await page.waitForURL((url) => !url.pathname.includes('/login'), { timeout: 10000 })
}

test.describe('Transfers tabs respect transfers:write permission', () => {
  // Role names deliberately avoid the words "Agent" / "Manager" so they
  // don't collide with users.spec.ts's hasText('Agent') filter on the role
  // dropdown — leftover soft-deleted rows would otherwise break that test.
  const customRoleName = generateUniqueName('E2E Tx Full')
  const fullAccessEmail = generateUniqueEmail('e2e-transfers-write')
  const readOnlyEmail = generateUniqueEmail('e2e-transfers-read')
  const password = 'Password123!'

  let api: ApiHelper
  let fullAccessRoleId: string
  let readOnlyRoleId: string
  let fullAccessUserId: string
  let readOnlyUserId: string

  test.beforeAll(async ({ request }) => {
    api = new ApiHelper(request)
    await api.login('admin@admin.com', 'admin')

    // Custom role with full transfers access — name deliberately not "admin"/"manager"
    const fullPerms = await api.findPermissionKeys([
      { resource: 'chat', action: 'read' },
      { resource: 'transfers', action: 'read' },
      { resource: 'transfers', action: 'write' },
      { resource: 'transfers', action: 'pickup' },
    ])
    const fullRole = await api.createRole({
      name: customRoleName,
      description: 'Custom role with full transfer permissions for e2e',
      permissions: fullPerms,
    })
    fullAccessRoleId = fullRole.id

    const fullUser = await api.createUser({
      email: fullAccessEmail,
      password,
      full_name: 'E2E Full Transfer Access',
      role_id: fullAccessRoleId,
    })
    fullAccessUserId = fullUser.id

    // Custom role with only read+pickup — should see agent-only view
    const readPerms = await api.findPermissionKeys([
      { resource: 'chat', action: 'read' },
      { resource: 'transfers', action: 'read' },
      { resource: 'transfers', action: 'pickup' },
    ])
    const readRole = await api.createRole({
      name: generateUniqueName('E2E Tx Read'),
      description: 'Custom role with read-only transfers for e2e',
      permissions: readPerms,
    })
    readOnlyRoleId = readRole.id

    const readUser = await api.createUser({
      email: readOnlyEmail,
      password,
      full_name: 'E2E Read-Only Transfer',
      role_id: readOnlyRoleId,
    })
    readOnlyUserId = readUser.id
  })

  test.afterAll(async () => {
    if (fullAccessUserId) await api.deleteUser(fullAccessUserId).catch(() => {})
    if (readOnlyUserId) await api.deleteUser(readOnlyUserId).catch(() => {})
    if (fullAccessRoleId) await api.deleteRole(fullAccessRoleId).catch(() => {})
    if (readOnlyRoleId) await api.deleteRole(readOnlyRoleId).catch(() => {})
  })

  test('custom role with transfers:write sees all three tabs', async ({ page }) => {
    await loginWithCredentials(page, fullAccessEmail, password)
    await page.goto('/chatbot/transfers')
    await page.waitForLoadState('networkidle')

    // Tabs only render in the admin/manager view; with the role-name-based
    // check they would have been hidden for this custom role.
    const tablist = page.locator('[role="tablist"]')
    await expect(tablist).toBeVisible({ timeout: 5000 })
    await expect(page.getByRole('tab', { name: /My Transfers/i })).toBeVisible()
    await expect(page.getByRole('tab', { name: /Queue/i })).toBeVisible()
    await expect(page.getByRole('tab', { name: /History/i })).toBeVisible()
  })

  test('custom role without transfers:write sees agent-only view (no tabs)', async ({ page }) => {
    await loginWithCredentials(page, readOnlyEmail, password)
    await page.goto('/chatbot/transfers')
    await page.waitForLoadState('networkidle')

    // Agent view has no tabs — they go straight to their own transfers list.
    await expect(page.locator('[role="tablist"]')).toHaveCount(0)
  })
})
