import { test, expect, Page } from '@playwright/test'
import { loginAsAdmin, ApiHelper, generateUniqueName, generateUniqueEmail } from '../../helpers'

// Helper to login with specific credentials
async function loginWithCredentials(page: Page, email: string, password: string) {
  await page.goto('/login')
  await page.locator('input[name="email"], input[type="email"]').fill(email)
  await page.locator('input[name="password"], input[type="password"]').fill(password)
  await page.locator('button[type="submit"]').click()
  await page.waitForURL((url) => !url.pathname.includes('/login'), { timeout: 10000 })
}

test.describe('Dashboard Widget Permissions', () => {
  test.describe('Admin User (with full permissions)', () => {
    test.beforeEach(async ({ page }) => {
      await loginAsAdmin(page)
      await page.goto('/')
      await page.waitForLoadState('networkidle')
    })

    test('admin can see Add Widget button', async ({ page }) => {
      const addButton = page.locator('button').filter({ hasText: /Add Widget/i })
      await expect(addButton).toBeVisible({ timeout: 10000 })
    })

    test('admin can see edit and delete buttons on widget hover', async ({ page }) => {
      // Wait for widgets to load
      await page.waitForSelector('.card-depth', { timeout: 10000 })

      // Hover over first widget
      const firstWidget = page.locator('.card-depth').first()
      await firstWidget.hover()

      // Should see edit button (pencil icon)
      const editButton = firstWidget.locator('button[title="Edit widget"]')
      await expect(editButton).toBeVisible()

      // Should see delete button (trash icon)
      const deleteButton = firstWidget.locator('button[title="Delete widget"]')
      await expect(deleteButton).toBeVisible()
    })
  })

  test.describe('User with Analytics Write Permission', () => {
    const roleName = generateUniqueName('E2E Analytics Write')
    const userEmail = generateUniqueEmail('e2e-analytics-write')
    const userPassword = 'Password123!'

    let api: ApiHelper
    let roleId: string
    let userId: string

    test.beforeAll(async ({ request }) => {
      api = new ApiHelper(request)
      await api.loginAsAdmin()

      // Create role with analytics read and write permissions
      const permissions = await api.findPermissionKeys([
        { resource: 'analytics', action: 'read' },
        { resource: 'analytics', action: 'write' },
      ])

      const role = await api.createRole({
        name: roleName,
        description: 'E2E test role with analytics write permission',
        permissions,
      })
      roleId = role.id

      // Create user with the custom role
      const user = await api.createUser({
        email: userEmail,
        password: userPassword,
        full_name: 'E2E Analytics Write User',
        role_id: roleId,
      })
      userId = user.id
    })

    test.afterAll(async () => {
      if (userId) await api.deleteUser(userId).catch(() => {})
      if (roleId) await api.deleteRole(roleId).catch(() => {})
    })

    test('user with analytics:write can see Add Widget button', async ({ page }) => {
      await loginWithCredentials(page, userEmail, userPassword)
      await page.goto('/')
      await page.waitForLoadState('networkidle')

      const addButton = page.locator('button').filter({ hasText: /Add Widget/i })
      await expect(addButton).toBeVisible({ timeout: 10000 })
    })

    test('user with analytics:write can see edit button on widget hover', async ({ page }) => {
      await loginWithCredentials(page, userEmail, userPassword)
      await page.goto('/')
      await page.waitForLoadState('networkidle')

      // Wait for widgets to load
      await page.waitForSelector('.card-depth', { timeout: 10000 })

      // Hover over first widget
      const firstWidget = page.locator('.card-depth').first()
      await firstWidget.hover()

      // Should see edit button
      const editButton = firstWidget.locator('button[title="Edit widget"]')
      await expect(editButton).toBeVisible()
    })
  })

  test.describe('User with Analytics Delete Permission', () => {
    const roleName = generateUniqueName('E2E Analytics Delete')
    const userEmail = generateUniqueEmail('e2e-analytics-delete')
    const userPassword = 'Password123!'

    let api: ApiHelper
    let roleId: string
    let userId: string

    test.beforeAll(async ({ request }) => {
      api = new ApiHelper(request)
      await api.loginAsAdmin()

      // Create role with analytics read and delete permissions (but NOT write)
      const permissions = await api.findPermissionKeys([
        { resource: 'analytics', action: 'read' },
        { resource: 'analytics', action: 'delete' },
      ])

      const role = await api.createRole({
        name: roleName,
        description: 'E2E test role with analytics delete permission',
        permissions,
      })
      roleId = role.id

      // Create user with the custom role
      const user = await api.createUser({
        email: userEmail,
        password: userPassword,
        full_name: 'E2E Analytics Delete User',
        role_id: roleId,
      })
      userId = user.id
    })

    test.afterAll(async () => {
      if (userId) await api.deleteUser(userId).catch(() => {})
      if (roleId) await api.deleteRole(roleId).catch(() => {})
    })

    test('user with analytics:delete can see delete button on widget hover', async ({ page }) => {
      await loginWithCredentials(page, userEmail, userPassword)
      await page.goto('/')
      await page.waitForLoadState('networkidle')

      // Wait for widgets to load
      await page.waitForSelector('.card-depth', { timeout: 10000 })

      // Hover over first widget
      const firstWidget = page.locator('.card-depth').first()
      await firstWidget.hover()

      // Should see delete button
      const deleteButton = firstWidget.locator('button[title="Delete widget"]')
      await expect(deleteButton).toBeVisible()
    })

    test('user with analytics:delete but NOT analytics:write cannot see Add Widget button', async ({ page }) => {
      await loginWithCredentials(page, userEmail, userPassword)
      await page.goto('/')
      await page.waitForLoadState('networkidle')

      const addButton = page.locator('button').filter({ hasText: /Add Widget/i })
      await expect(addButton).not.toBeVisible()
    })

    test('user with analytics:delete but NOT analytics:write cannot see edit button', async ({ page }) => {
      await loginWithCredentials(page, userEmail, userPassword)
      await page.goto('/')
      await page.waitForLoadState('networkidle')

      // Wait for widgets to load
      await page.waitForSelector('.card-depth', { timeout: 10000 })

      // Hover over first widget
      const firstWidget = page.locator('.card-depth').first()
      await firstWidget.hover()

      // Should NOT see edit button
      const editButton = firstWidget.locator('button[title="Edit widget"]')
      await expect(editButton).not.toBeVisible()
    })
  })

  test.describe('User with Analytics Read Only', () => {
    const roleName = generateUniqueName('E2E Analytics Read Only')
    const userEmail = generateUniqueEmail('e2e-analytics-readonly')
    const userPassword = 'Password123!'

    let api: ApiHelper
    let roleId: string
    let userId: string

    test.beforeAll(async ({ request }) => {
      api = new ApiHelper(request)
      await api.loginAsAdmin()

      // Create role with only analytics read permission
      const permissions = await api.findPermissionKeys([
        { resource: 'analytics', action: 'read' },
      ])

      const role = await api.createRole({
        name: roleName,
        description: 'E2E test role with analytics read-only permission',
        permissions,
      })
      roleId = role.id

      // Create user with the custom role
      const user = await api.createUser({
        email: userEmail,
        password: userPassword,
        full_name: 'E2E Analytics Read Only User',
        role_id: roleId,
      })
      userId = user.id
    })

    test.afterAll(async () => {
      if (userId) await api.deleteUser(userId).catch(() => {})
      if (roleId) await api.deleteRole(roleId).catch(() => {})
    })

    test('user with only analytics:read cannot see Add Widget button', async ({ page }) => {
      await loginWithCredentials(page, userEmail, userPassword)
      await page.goto('/')
      await page.waitForLoadState('networkidle')

      const addButton = page.locator('button').filter({ hasText: /Add Widget/i })
      await expect(addButton).not.toBeVisible()
    })

    test('user with only analytics:read cannot see edit button on widget hover', async ({ page }) => {
      await loginWithCredentials(page, userEmail, userPassword)
      await page.goto('/')
      await page.waitForLoadState('networkidle')

      // Wait for widgets to load
      await page.waitForSelector('.card-depth', { timeout: 10000 })

      // Hover over first widget
      const firstWidget = page.locator('.card-depth').first()
      await firstWidget.hover()

      // Should NOT see edit button
      const editButton = firstWidget.locator('button[title="Edit widget"]')
      await expect(editButton).not.toBeVisible()
    })

    test('user with only analytics:read cannot see delete button on widget hover', async ({ page }) => {
      await loginWithCredentials(page, userEmail, userPassword)
      await page.goto('/')
      await page.waitForLoadState('networkidle')

      // Wait for widgets to load
      await page.waitForSelector('.card-depth', { timeout: 10000 })

      // Hover over first widget
      const firstWidget = page.locator('.card-depth').first()
      await firstWidget.hover()

      // Should NOT see delete button
      const deleteButton = firstWidget.locator('button[title="Delete widget"]')
      await expect(deleteButton).not.toBeVisible()
    })

    test('user with only analytics:read can still view dashboard and widgets', async ({ page }) => {
      await loginWithCredentials(page, userEmail, userPassword)
      await page.goto('/')
      await page.waitForLoadState('networkidle')

      // Should see dashboard
      await expect(page.locator('h1')).toContainText('Dashboard')

      // Should see widgets
      await page.waitForSelector('.card-depth', { timeout: 10000 })
      const widgets = page.locator('.card-depth')
      expect(await widgets.count()).toBeGreaterThan(0)
    })
  })

  test.describe('User with Full Analytics Permissions', () => {
    const roleName = generateUniqueName('E2E Analytics Full')
    const userEmail = generateUniqueEmail('e2e-analytics-full')
    const userPassword = 'Password123!'

    let api: ApiHelper
    let roleId: string
    let userId: string

    test.beforeAll(async ({ request }) => {
      api = new ApiHelper(request)
      await api.loginAsAdmin()

      // Create role with all analytics permissions
      const permissions = await api.findPermissionKeys([
        { resource: 'analytics', action: 'read' },
        { resource: 'analytics', action: 'write' },
        { resource: 'analytics', action: 'delete' },
      ])

      const role = await api.createRole({
        name: roleName,
        description: 'E2E test role with full analytics permissions',
        permissions,
      })
      roleId = role.id

      // Create user with the custom role
      const user = await api.createUser({
        email: userEmail,
        password: userPassword,
        full_name: 'E2E Analytics Full User',
        role_id: roleId,
      })
      userId = user.id
    })

    test.afterAll(async () => {
      if (userId) await api.deleteUser(userId).catch(() => {})
      if (roleId) await api.deleteRole(roleId).catch(() => {})
    })

    test('user with full analytics permissions can see all widget controls', async ({ page }) => {
      await loginWithCredentials(page, userEmail, userPassword)
      await page.goto('/')
      await page.waitForLoadState('networkidle')

      // Should see Add Widget button
      const addButton = page.locator('button').filter({ hasText: /Add Widget/i })
      await expect(addButton).toBeVisible({ timeout: 10000 })

      // Wait for widgets to load
      await page.waitForSelector('.card-depth', { timeout: 10000 })

      // Hover over first widget
      const firstWidget = page.locator('.card-depth').first()
      await firstWidget.hover()

      // Should see both edit and delete buttons
      const editButton = firstWidget.locator('button[title="Edit widget"]')
      await expect(editButton).toBeVisible()

      const deleteButton = firstWidget.locator('button[title="Delete widget"]')
      await expect(deleteButton).toBeVisible()
    })
  })
})
