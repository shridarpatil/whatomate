import { test, expect } from '@playwright/test'
import { ApiHelper } from '../../helpers'
import {
  createTestScope,
  createUserWithPermissions,
  loginAs,
  SUPER_ADMIN,
  type TestUserHandle,
} from '../../framework'

/**
 * Same regression as tests/settings/transfers-permissions.spec.ts —
 * rewritten on the new framework. Demonstrates:
 *   - createTestScope for prefixed names.
 *   - createUserWithPermissions for one-call permission-scoped users.
 *   - loginAs for credential-based UI login.
 */
const scope = createTestScope('transfers-permission-tabs')

test.describe('Transfers tabs respect transfers:write permission', () => {
  let api: ApiHelper
  let fullAccess: TestUserHandle
  let agentOnly: TestUserHandle

  test.beforeAll(async ({ request }) => {
    api = new ApiHelper(request)
    await api.login(SUPER_ADMIN.email, SUPER_ADMIN.password)

    // userSlug values intentionally avoid "agent"/"manager"/"admin" — those
    // would generate role names that collide with the seeded system roles
    // when other specs do `select.filter({ hasText: 'Agent' })` (substring,
    // case-insensitive).
    fullAccess = await createUserWithPermissions(api, scope, {
      userSlug: 'tx-full',
      permissions: [
        { resource: 'chat', action: 'read' },
        { resource: 'transfers', action: 'read' },
        { resource: 'transfers', action: 'write' },
        { resource: 'transfers', action: 'pickup' },
      ],
    })

    agentOnly = await createUserWithPermissions(api, scope, {
      userSlug: 'tx-pickup',
      permissions: [
        { resource: 'chat', action: 'read' },
        { resource: 'transfers', action: 'read' },
        { resource: 'transfers', action: 'pickup' },
      ],
    })
  })

  test.afterAll(async () => {
    // Best-effort cleanup via existing API endpoints. CI gives a fresh DB
    // per run, so this isn't load-bearing — just a courtesy for local devs.
    await api.deleteUser(fullAccess.user.id).catch(() => {})
    await api.deleteUser(agentOnly.user.id).catch(() => {})
    await api.deleteRole(fullAccess.role.id).catch(() => {})
    await api.deleteRole(agentOnly.role.id).catch(() => {})
  })

  test('user with transfers:write sees all three tabs', async ({ page }) => {
    await loginAs(page, fullAccess)
    await page.goto('/chatbot/transfers')
    await page.waitForLoadState('networkidle')

    const tablist = page.locator('[role="tablist"]')
    await expect(tablist).toBeVisible({ timeout: 5000 })
    await expect(page.getByRole('tab', { name: /My Transfers/i })).toBeVisible()
    await expect(page.getByRole('tab', { name: /Queue/i })).toBeVisible()
    await expect(page.getByRole('tab', { name: /History/i })).toBeVisible()
  })

  test('user without transfers:write sees agent-only view (no tabs)', async ({ page }) => {
    await loginAs(page, agentOnly)
    await page.goto('/chatbot/transfers')
    await page.waitForLoadState('networkidle')

    await expect(page.locator('[role="tablist"]')).toHaveCount(0)
  })
})
