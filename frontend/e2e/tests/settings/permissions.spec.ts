import { test, expect, type Page } from '@playwright/test'
import { ApiHelper, loginAsAdmin } from '../../helpers'
import {
  createTestScope,
  createUserWithPermissions,
  loginAs,
  SUPER_ADMIN,
  type TestUserHandle,
} from '../../framework'

// Helper to read the visible sidebar menu items as plain text.
async function getSidebarMenuItems(page: Page): Promise<string[]> {
  const items: string[] = []
  const navLinks = page.locator('aside a[role="menuitem"], aside nav a, aside nav button[class*="justify-start"]')
  const count = await navLinks.count()
  for (let i = 0; i < count; i++) {
    const text = await navLinks.nth(i).textContent()
    if (text && text.trim()) items.push(text.trim().toLowerCase())
  }
  return items
}

test.describe('Custom Role with Limited Permissions', () => {
  const scope = createTestScope('permissions-limited')
  let api: ApiHelper
  let user: TestUserHandle

  test.beforeAll(async ({ request }) => {
    api = new ApiHelper(request)
    await api.login(SUPER_ADMIN.email, SUPER_ADMIN.password)
    user = await createUserWithPermissions(api, scope, {
      permissions: [{ resource: 'chat', action: 'read' }],
    })
  })

  test.afterAll(async () => {
    await api.deleteUser(user.user.id).catch(() => {})
    await api.deleteRole(user.role.id).catch(() => {})
  })

  test('user with limited role sees only permitted menu items', async ({ page }) => {
    await loginAs(page, user)
    await page.waitForSelector('aside nav')
    await page.waitForTimeout(500)

    const menuItems = await getSidebarMenuItems(page)

    expect(menuItems.some((item) => item.includes('chat'))).toBeTruthy()
    expect(menuItems.some((item) => item.includes('settings'))).toBeFalsy()
    expect(menuItems.some((item) => item.includes('analytics') || item.includes('dashboard'))).toBeFalsy()
  })

  test('user with limited role is redirected from unauthorized pages', async ({ page }) => {
    await loginAs(page, user)
    await page.goto('/settings')
    await page.waitForLoadState('networkidle')
    expect(page.url()).not.toContain('/settings')
  })

  test('user with limited role can access permitted pages', async ({ page }) => {
    await loginAs(page, user)
    await page.goto('/chat')
    await page.waitForLoadState('networkidle')
    expect(page.url()).toContain('/chat')
    await expect(page.locator('body')).not.toContainText('forbidden', { ignoreCase: true })
  })

  test('user lands on first accessible page after login', async ({ page }) => {
    await loginAs(page, user)
    expect(page.url()).toContain('/chat')
  })
})

test.describe('Role with Settings Access', () => {
  const scope = createTestScope('permissions-settings')
  let api: ApiHelper
  let user: TestUserHandle

  test.beforeAll(async ({ request }) => {
    api = new ApiHelper(request)
    await api.login(SUPER_ADMIN.email, SUPER_ADMIN.password)
    user = await createUserWithPermissions(api, scope, {
      permissions: [
        { resource: 'chat', action: 'read' },
        { resource: 'users', action: 'read' },
        { resource: 'users', action: 'create' },
        { resource: 'settings.general', action: 'read' },
      ],
    })
  })

  test.afterAll(async () => {
    await api.deleteUser(user.user.id).catch(() => {})
    await api.deleteRole(user.role.id).catch(() => {})
  })

  test('user with settings permission sees Settings menu', async ({ page }) => {
    await loginAs(page, user)
    await page.waitForSelector('aside nav')
    await page.waitForTimeout(500)

    const menuItems = await getSidebarMenuItems(page)
    expect(menuItems.some((item) => item.includes('settings'))).toBeTruthy()
  })

  test('user with users:read can access users page', async ({ page }) => {
    await loginAs(page, user)
    await page.goto('/settings/users')
    await page.waitForLoadState('networkidle')
    expect(page.url()).toContain('/settings/users')
    await expect(page.locator('table, [role="table"]').first()).toBeVisible()
  })

  test('user with users:create sees Add button', async ({ page }) => {
    await loginAs(page, user)
    await page.goto('/settings/users')
    await page.waitForLoadState('networkidle')
    const addButton = page.locator('button').filter({ hasText: /add|create/i })
    await expect(addButton.first()).toBeVisible()
  })
})

// Uses admin@test.com (the canonical admin-role user) deliberately —
// testing the admin role's behavior, not super-admin's.
test.describe('Admin vs Limited Role Comparison', () => {
  test('admin sees all menu items', async ({ page }) => {
    await loginAsAdmin(page)
    await page.waitForSelector('aside nav')
    await page.waitForTimeout(500)

    const menuItems = await getSidebarMenuItems(page)
    expect(menuItems.some((item) => item.includes('chat'))).toBeTruthy()
    expect(menuItems.some((item) => item.includes('settings'))).toBeTruthy()
  })

  test('admin can access all settings pages', async ({ page }) => {
    await loginAsAdmin(page)

    await page.goto('/settings/users')
    await page.waitForLoadState('networkidle')
    expect(page.url()).toContain('/settings/users')

    await page.goto('/settings/roles')
    await page.waitForLoadState('networkidle')
    expect(page.url()).toContain('/settings/roles')

    await page.goto('/settings')
    await page.waitForLoadState('networkidle')
    expect(page.url()).toContain('/settings')
  })
})

test.describe('Dynamic Role Updates', () => {
  const scope = createTestScope('permissions-dynamic')
  let api: ApiHelper
  let user: TestUserHandle

  test.beforeAll(async ({ request }) => {
    api = new ApiHelper(request)
    await api.login(SUPER_ADMIN.email, SUPER_ADMIN.password)
    user = await createUserWithPermissions(api, scope, {
      permissions: [{ resource: 'chat', action: 'read' }],
    })
  })

  test.afterAll(async () => {
    await api.deleteUser(user.user.id).catch(() => {})
    await api.deleteRole(user.role.id).catch(() => {})
  })

  test('user initially has limited access', async ({ page }) => {
    await loginAs(page, user)
    await page.waitForSelector('aside nav')

    const menuItems = await getSidebarMenuItems(page)
    expect(menuItems.some((item) => item.includes('chat'))).toBeTruthy()
    expect(menuItems.some((item) => item.includes('settings'))).toBeFalsy()
  })
})
