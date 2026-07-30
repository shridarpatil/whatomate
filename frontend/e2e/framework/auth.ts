/**
 * UI login helpers used by the new framework.
 *
 * These deliberately do not wrap the existing `loginAsAdmin` from
 * `helpers/auth.ts` — that one targets `admin@test.com`, which lacks
 * permissions like `analytics:read` and silently lands users on the
 * "not allowed" page (cost ~30 minutes of debugging earlier this month).
 *
 * For framework-level tests prefer `loginAsSuperAdmin` (admin@admin.com,
 * always succeeds) or `loginAs(page, customCreds)` for permission-scoped
 * users created via `createUserWithPermissions`.
 */

import type { Page } from '@playwright/test'

export const SUPER_ADMIN = {
  email: 'admin@admin.com',
  password: 'admin',
} as const

export interface Credentials {
  email: string
  password: string
}

export async function loginAs(page: Page, creds: Credentials): Promise<void> {
  // Clear any existing session to ensure the router doesn't intercept the /login
  // navigation and redirect us away if the browser already has an active session.
  await page.context().clearCookies()
  try {
    await page.evaluate(() => window.localStorage.clear())
  } catch {
    // Ignore if on about:blank
  }

  await page.goto('/login')
  await page.locator('input[type="email"], input[name="email"]').fill(creds.email)
  await page.locator('input[type="password"], input[name="password"]').fill(creds.password)
  await page.locator('button[type="submit"]').click()
  await page.waitForURL((url) => !url.pathname.includes('/login'), { timeout: 10_000 })
}

export async function loginAsSuperAdmin(page: Page): Promise<void> {
  await loginAs(page, SUPER_ADMIN)
}
