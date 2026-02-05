import { test, expect, Page } from '@playwright/test'
import { loginAsAdmin, login, ApiHelper, generateUniqueName, generateUniqueEmail } from '../../helpers'

// Helper to get visible sidebar menu items
async function getSidebarMenuItems(page: Page): Promise<string[]> {
  const items: string[] = []
  const navLinks = page.locator('aside nav a, aside nav button[class*="justify-start"]')
  const count = await navLinks.count()
  for (let i = 0; i < count; i++) {
    const text = await navLinks.nth(i).textContent()
    if (text && text.trim()) items.push(text.trim().toLowerCase())
  }
  return items
}

// Helper to login with specific credentials
async function loginWithCredentials(page: Page, email: string, password: string) {
  await page.goto('/login')
  await page.locator('input[name="email"], input[type="email"]').fill(email)
  await page.locator('input[name="password"], input[type="password"]').fill(password)
  await page.locator('button[type="submit"]').click()
  await page.waitForURL((url) => !url.pathname.includes('/login'), { timeout: 10000 })
}

test.describe('Custom Role with Limited Permissions', () => {
  const roleName = generateUniqueName('E2E Limited Role')
  const userEmail = generateUniqueEmail('e2e-limited')
  const userPassword = 'Password123!'

  let api: ApiHelper
  let roleId: string
  let userId: string

  test.beforeAll(async ({ request }) => {
    api = new ApiHelper(request)
    await api.loginAsAdmin()

    // Create role with only chat read permission (very limited)
    const permissions = await api.findPermissionKeys([
      { resource: 'chat', action: 'read' },
    ])

    const role = await api.createRole({
      name: roleName,
      description: 'E2E test role with limited permissions',
      permissions,
    })
    roleId = role.id

    // Create user with the custom role
    const user = await api.createUser({
      email: userEmail,
      password: userPassword,
      full_name: 'E2E Limited User',
      role_id: roleId,
    })
    userId = user.id
  })

  test.afterAll(async () => {
    // Cleanup
    if (userId) await api.deleteUser(userId).catch(() => {})
    if (roleId) await api.deleteRole(roleId).catch(() => {})
  })

  test('user with limited role sees only permitted menu items', async ({ page }) => {
    await loginWithCredentials(page, userEmail, userPassword)
    await page.waitForSelector('aside nav')
    await page.waitForTimeout(500)

    const menuItems = await getSidebarMenuItems(page)

    // Should see Chat (has chat:read permission)
    expect(menuItems.some(item => item.includes('chat'))).toBeTruthy()

    // Should NOT see Settings (no settings permissions)
    expect(menuItems.some(item => item.includes('settings'))).toBeFalsy()

    // Should NOT see Analytics/Dashboard (no analytics permissions)
    expect(menuItems.some(item => item.includes('analytics') || item.includes('dashboard'))).toBeFalsy()
  })

  test('user with limited role is redirected from unauthorized pages', async ({ page }) => {
    await loginWithCredentials(page, userEmail, userPassword)

    // Try to access settings directly
    await page.goto('/settings')
    await page.waitForLoadState('networkidle')

    // Should be redirected away from settings
    expect(page.url()).not.toContain('/settings')
  })

  test('user with limited role can access permitted pages', async ({ page }) => {
    await loginWithCredentials(page, userEmail, userPassword)

    // Should be able to access chat
    await page.goto('/chat')
    await page.waitForLoadState('networkidle')

    expect(page.url()).toContain('/chat')
    await expect(page.locator('body')).not.toContainText('forbidden', { ignoreCase: true })
  })

  test('user lands on first accessible page after login', async ({ page }) => {
    await loginWithCredentials(page, userEmail, userPassword)

    // Should land on chat (first accessible route based on permissions)
    expect(page.url()).toContain('/chat')
  })
})

test.describe('Role with Settings Access', () => {
  const roleName = generateUniqueName('E2E Settings Role')
  const userEmail = generateUniqueEmail('e2e-settings')
  const userPassword = 'Password123!'

  let api: ApiHelper
  let roleId: string
  let userId: string

  test.beforeAll(async ({ request }) => {
    api = new ApiHelper(request)
    await api.loginAsAdmin()

    // Create role with settings permissions
    const permissions = await api.findPermissionKeys([
      { resource: 'chat', action: 'read' },
      { resource: 'users', action: 'read' },
      { resource: 'users', action: 'create' },
      { resource: 'settings.general', action: 'read' },
    ])

    const role = await api.createRole({
      name: roleName,
      description: 'E2E test role with settings access',
      permissions,
    })
    roleId = role.id

    const user = await api.createUser({
      email: userEmail,
      password: userPassword,
      full_name: 'E2E Settings User',
      role_id: roleId,
    })
    userId = user.id
  })

  test.afterAll(async () => {
    if (userId) await api.deleteUser(userId).catch(() => {})
    if (roleId) await api.deleteRole(roleId).catch(() => {})
  })

  test('user with settings permission sees Settings menu', async ({ page }) => {
    await loginWithCredentials(page, userEmail, userPassword)
    await page.waitForSelector('aside nav')
    await page.waitForTimeout(500)

    const menuItems = await getSidebarMenuItems(page)
    expect(menuItems.some(item => item.includes('settings'))).toBeTruthy()
  })

  test('user with users:read can access users page', async ({ page }) => {
    await loginWithCredentials(page, userEmail, userPassword)

    await page.goto('/settings/users')
    await page.waitForLoadState('networkidle')

    expect(page.url()).toContain('/settings/users')
    await expect(page.locator('table, [role="table"]').first()).toBeVisible()
  })

  test('user with users:create sees Add button', async ({ page }) => {
    await loginWithCredentials(page, userEmail, userPassword)

    await page.goto('/settings/users')
    await page.waitForLoadState('networkidle')

    // Should see Add/Create button
    const addButton = page.locator('button').filter({ hasText: /add|create/i })
    await expect(addButton.first()).toBeVisible()
  })
})

test.describe('Admin vs Limited Role Comparison', () => {
  test('admin sees all menu items', async ({ page }) => {
    await loginAsAdmin(page)
    await page.waitForSelector('aside nav')
    await page.waitForTimeout(500)

    const menuItems = await getSidebarMenuItems(page)

    expect(menuItems.some(item => item.includes('chat'))).toBeTruthy()
    expect(menuItems.some(item => item.includes('settings'))).toBeTruthy()
  })

  test('admin can access all settings pages', async ({ page }) => {
    await loginAsAdmin(page)

    // Can access users
    await page.goto('/settings/users')
    await page.waitForLoadState('networkidle')
    expect(page.url()).toContain('/settings/users')

    // Can access roles
    await page.goto('/settings/roles')
    await page.waitForLoadState('networkidle')
    expect(page.url()).toContain('/settings/roles')

    // Can access general settings
    await page.goto('/settings')
    await page.waitForLoadState('networkidle')
    expect(page.url()).toContain('/settings')
  })
})

test.describe('Dynamic Role Updates', () => {
  const roleName = generateUniqueName('E2E Dynamic Role')
  const userEmail = generateUniqueEmail('e2e-dynamic')
  const userPassword = 'Password123!'

  let api: ApiHelper
  let roleId: string
  let userId: string

  test.beforeAll(async ({ request }) => {
    api = new ApiHelper(request)
    await api.loginAsAdmin()

    // Create role with minimal permissions
    const permissions = await api.findPermissionKeys([
      { resource: 'chat', action: 'read' },
    ])

    const role = await api.createRole({
      name: roleName,
      description: 'E2E test role for dynamic updates',
      permissions,
    })
    roleId = role.id

    const user = await api.createUser({
      email: userEmail,
      password: userPassword,
      full_name: 'E2E Dynamic User',
      role_id: roleId,
    })
    userId = user.id
  })

  test.afterAll(async () => {
    if (userId) await api.deleteUser(userId).catch(() => {})
    if (roleId) await api.deleteRole(roleId).catch(() => {})
  })

  test('user initially has limited access', async ({ page }) => {
    await loginWithCredentials(page, userEmail, userPassword)
    await page.waitForSelector('aside nav')

    const menuItems = await getSidebarMenuItems(page)

    // Should see Chat
    expect(menuItems.some(item => item.includes('chat'))).toBeTruthy()

    // Should NOT see Settings
    expect(menuItems.some(item => item.includes('settings'))).toBeFalsy()
  })
})
