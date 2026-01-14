import { Page } from '@playwright/test'

export interface TestUser {
  email: string
  password: string
  role: 'admin' | 'manager' | 'agent'
}

// Test users - these should match seeded data in test database
export const TEST_USERS = {
  admin: {
    email: 'admin@test.com',
    password: 'password',
    role: 'admin' as const,
  },
  manager: {
    email: 'manager@test.com',
    password: 'password',
    role: 'manager' as const,
  },
  agent: {
    email: 'agent@test.com',
    password: 'password',
    role: 'agent' as const,
  },
}

export async function login(page: Page, user: TestUser) {
  await page.goto('/login')
  await page.locator('input[name="email"], input[type="email"]').fill(user.email)
  await page.locator('input[name="password"], input[type="password"]').fill(user.password)
  await page.locator('button[type="submit"]').click()
  // Wait for redirect to dashboard or home
  await page.waitForURL(/\/(dashboard|chat|home)?$/)
}

export async function loginAsAdmin(page: Page) {
  await login(page, TEST_USERS.admin)
}

export async function loginAsManager(page: Page) {
  await login(page, TEST_USERS.manager)
}

export async function loginAsAgent(page: Page) {
  await login(page, TEST_USERS.agent)
}

export async function logout(page: Page) {
  // Click user menu in sidebar - it's in the aside element (not nav), button contains user's email
  const userMenuButton = page.locator('aside').getByRole('button').filter({ hasText: /@/ }).first()
  await userMenuButton.click()
  // Click logout in popover
  await page.getByRole('button', { name: /Log out/i }).click()
  // Wait for redirect to login
  await page.waitForURL(/\/login/)
}

export async function isLoggedIn(page: Page): Promise<boolean> {
  // Check if we're on a protected page (not login/register)
  const url = page.url()
  return !url.includes('/login') && !url.includes('/register')
}
