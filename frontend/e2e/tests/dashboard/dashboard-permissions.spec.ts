import { test, expect } from '@playwright/test'
import { ApiHelper, loginAsAdmin } from '../../helpers'
import {
  createTestScope,
  createUserWithPermissions,
  loginAs,
  SUPER_ADMIN,
  type TestUserHandle,
} from '../../framework'

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
      await page.waitForSelector('.card-depth', { timeout: 10000 })
      const firstWidget = page.locator('.card-depth').first()
      await firstWidget.hover()

      await expect(firstWidget.locator('button[title="Edit widget"]')).toBeVisible()
      await expect(firstWidget.locator('button[title="Delete widget"]')).toBeVisible()
    })
  })

  test.describe('User with Analytics Write Permission', () => {
    const scope = createTestScope('dashboard-analytics-write')
    let api: ApiHelper
    let user: TestUserHandle

    test.beforeAll(async ({ request }) => {
      api = new ApiHelper(request)
      await api.login(SUPER_ADMIN.email, SUPER_ADMIN.password)
      user = await createUserWithPermissions(api, scope, {
        permissions: [
          { resource: 'analytics', action: 'read' },
          { resource: 'analytics', action: 'write' },
        ],
      })
    })

    test.afterAll(async () => {
      await api.deleteUser(user.user.id).catch(() => {})
      await api.deleteRole(user.role.id).catch(() => {})
    })

    test('user with analytics:write can see Add Widget button', async ({ page }) => {
      await loginAs(page, user)
      await page.goto('/')
      await page.waitForLoadState('networkidle')

      const addButton = page.locator('button').filter({ hasText: /Add Widget/i })
      await expect(addButton).toBeVisible({ timeout: 10000 })
    })

    test('user with analytics:write can see edit button on widget hover', async ({ page }) => {
      await loginAs(page, user)
      await page.goto('/')
      await page.waitForLoadState('networkidle')
      await page.waitForSelector('.card-depth', { timeout: 10000 })

      const firstWidget = page.locator('.card-depth').first()
      await firstWidget.hover()
      await expect(firstWidget.locator('button[title="Edit widget"]')).toBeVisible()
    })
  })

  test.describe('User with Analytics Delete Permission', () => {
    const scope = createTestScope('dashboard-analytics-delete')
    let api: ApiHelper
    let user: TestUserHandle

    test.beforeAll(async ({ request }) => {
      api = new ApiHelper(request)
      await api.login(SUPER_ADMIN.email, SUPER_ADMIN.password)
      user = await createUserWithPermissions(api, scope, {
        permissions: [
          { resource: 'analytics', action: 'read' },
          { resource: 'analytics', action: 'delete' },
        ],
      })
    })

    test.afterAll(async () => {
      await api.deleteUser(user.user.id).catch(() => {})
      await api.deleteRole(user.role.id).catch(() => {})
    })

    test('user with analytics:delete can see delete button on widget hover', async ({ page }) => {
      await loginAs(page, user)
      await page.goto('/')
      await page.waitForLoadState('networkidle')
      await page.waitForSelector('.card-depth', { timeout: 10000 })

      const firstWidget = page.locator('.card-depth').first()
      await firstWidget.hover()
      await expect(firstWidget.locator('button[title="Delete widget"]')).toBeVisible()
    })

    test('user with analytics:delete but NOT analytics:write cannot see Add Widget button', async ({ page }) => {
      await loginAs(page, user)
      await page.goto('/')
      await page.waitForLoadState('networkidle')

      const addButton = page.locator('button').filter({ hasText: /Add Widget/i })
      await expect(addButton).not.toBeVisible()
    })

    test('user with analytics:delete but NOT analytics:write cannot see edit button', async ({ page }) => {
      await loginAs(page, user)
      await page.goto('/')
      await page.waitForLoadState('networkidle')
      await page.waitForSelector('.card-depth', { timeout: 10000 })

      const firstWidget = page.locator('.card-depth').first()
      await firstWidget.hover()
      await expect(firstWidget.locator('button[title="Edit widget"]')).not.toBeVisible()
    })
  })

  test.describe('User with Analytics Read Only', () => {
    const scope = createTestScope('dashboard-analytics-read')
    let api: ApiHelper
    let user: TestUserHandle

    test.beforeAll(async ({ request }) => {
      api = new ApiHelper(request)
      await api.login(SUPER_ADMIN.email, SUPER_ADMIN.password)
      user = await createUserWithPermissions(api, scope, {
        permissions: [{ resource: 'analytics', action: 'read' }],
      })
    })

    test.afterAll(async () => {
      await api.deleteUser(user.user.id).catch(() => {})
      await api.deleteRole(user.role.id).catch(() => {})
    })

    test('user with only analytics:read cannot see Add Widget button', async ({ page }) => {
      await loginAs(page, user)
      await page.goto('/')
      await page.waitForLoadState('networkidle')

      const addButton = page.locator('button').filter({ hasText: /Add Widget/i })
      await expect(addButton).not.toBeVisible()
    })

    test('user with only analytics:read cannot see edit button on widget hover', async ({ page }) => {
      await loginAs(page, user)
      await page.goto('/')
      await page.waitForLoadState('networkidle')
      await page.waitForSelector('.card-depth', { timeout: 10000 })

      const firstWidget = page.locator('.card-depth').first()
      await firstWidget.hover()
      await expect(firstWidget.locator('button[title="Edit widget"]')).not.toBeVisible()
    })

    test('user with only analytics:read cannot see delete button on widget hover', async ({ page }) => {
      await loginAs(page, user)
      await page.goto('/')
      await page.waitForLoadState('networkidle')
      await page.waitForSelector('.card-depth', { timeout: 10000 })

      const firstWidget = page.locator('.card-depth').first()
      await firstWidget.hover()
      await expect(firstWidget.locator('button[title="Delete widget"]')).not.toBeVisible()
    })

    test('user with only analytics:read can still view dashboard and widgets', async ({ page }) => {
      await loginAs(page, user)
      await page.goto('/')
      await page.waitForLoadState('networkidle')

      await expect(page.locator('h1')).toContainText('Dashboard')
      await page.waitForSelector('.card-depth', { timeout: 10000 })
      const widgets = page.locator('.card-depth')
      expect(await widgets.count()).toBeGreaterThan(0)
    })
  })

  test.describe('User with Full Analytics Permissions', () => {
    const scope = createTestScope('dashboard-analytics-full')
    let api: ApiHelper
    let user: TestUserHandle

    test.beforeAll(async ({ request }) => {
      api = new ApiHelper(request)
      await api.login(SUPER_ADMIN.email, SUPER_ADMIN.password)
      user = await createUserWithPermissions(api, scope, {
        permissions: [
          { resource: 'analytics', action: 'read' },
          { resource: 'analytics', action: 'write' },
          { resource: 'analytics', action: 'delete' },
        ],
      })
    })

    test.afterAll(async () => {
      await api.deleteUser(user.user.id).catch(() => {})
      await api.deleteRole(user.role.id).catch(() => {})
    })

    test('user with full analytics permissions can see all widget controls', async ({ page }) => {
      await loginAs(page, user)
      await page.goto('/')
      await page.waitForLoadState('networkidle')

      const addButton = page.locator('button').filter({ hasText: /Add Widget/i })
      await expect(addButton).toBeVisible({ timeout: 10000 })

      await page.waitForSelector('.card-depth', { timeout: 10000 })
      const firstWidget = page.locator('.card-depth').first()
      await firstWidget.hover()

      await expect(firstWidget.locator('button[title="Edit widget"]')).toBeVisible()
      await expect(firstWidget.locator('button[title="Delete widget"]')).toBeVisible()
    })
  })
})
