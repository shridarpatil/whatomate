import { test, expect } from '@playwright/test'
import { ApiHelper } from '../../helpers'
import { createTestScope, createUserWithPermissions, loginAs, SUPER_ADMIN, type TestUserHandle } from '../../framework'

// Organization settings can only be changed with settings.general:write.
test.describe('Organization settings permissions', () => {
  const scope = createTestScope('settings-general-read')
  let admin: ApiHelper
  let user: TestUserHandle

  test.beforeAll(async ({ request }) => {
    admin = new ApiHelper(request)
    await admin.login(SUPER_ADMIN.email, SUPER_ADMIN.password)
    user = await createUserWithPermissions(admin, scope, {
      permissions: [{ resource: 'settings.general', action: 'read' }],
    })
  })

  test.afterAll(async () => {
    await admin.deleteUser(user.user.id).catch(() => {})
    await admin.deleteRole(user.role.id).catch(() => {})
  })

  test('read-only user cannot change settings through the API', async ({ playwright }) => {
    const ctx = await playwright.request.newContext()
    const api = new ApiHelper(ctx)
    await api.login(user.email, user.password)
    // Send the current values back, so nothing changes even if the check regresses.
    const current = (await (await api.get('/api/org/settings')).json()).data
    const { timezone, calling_enabled } = current.settings
    for (const body of [{ name: current.name }, { timezone }, { calling_enabled }]) {
      const res = await api.put('/api/org/settings', body)
      expect(res.status(), JSON.stringify(body)).toBe(403)
    }
    await ctx.dispose()
  })

  test('read-only user sees settings without Save buttons', async ({ page }) => {
    await loginAs(page, user)
    await page.goto('/settings')
    await expect(page.locator('#org_name')).toBeVisible({ timeout: 10000 })
    await expect(page.getByRole('button', { name: 'Save Changes' })).toHaveCount(0)
  })
})
