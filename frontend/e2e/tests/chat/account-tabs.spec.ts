import { test, expect, Page, Route } from '@playwright/test'
import { loginAsAdmin } from '../../helpers'
import { ChatPage } from '../../pages'

/**
 * Multi-account tabs: when a contact has messages from multiple WhatsApp
 * accounts, tabs should appear below the chat header to let the agent
 * switch between accounts.
 *
 * These tests use route interception to simulate multi-account and
 * single-account contacts without requiring real WhatsApp accounts.
 */

const CONTACT_ID = '00000000-0000-0000-0000-000000000001'

function makeMessage(overrides: Record<string, any> = {}) {
  return {
    id: crypto.randomUUID(),
    contact_id: CONTACT_ID,
    direction: 'incoming',
    message_type: 'text',
    content: { body: 'Hello' },
    status: 'delivered',
    whatsapp_account: 'account-1',
    created_at: '2026-02-16T10:00:00Z',
    updated_at: '2026-02-16T10:00:00Z',
    ...overrides,
  }
}

const MULTI_ACCOUNT_MESSAGES = [
  makeMessage({ whatsapp_account: 'account-1', created_at: '2026-02-16T10:00:00Z' }),
  makeMessage({ whatsapp_account: 'account-2', created_at: '2026-02-16T10:01:00Z' }),
  makeMessage({ whatsapp_account: 'account-1', direction: 'outgoing', created_at: '2026-02-16T10:02:00Z' }),
  makeMessage({ whatsapp_account: 'account-2', direction: 'incoming', created_at: '2026-02-16T10:03:00Z' }),
]

const SINGLE_ACCOUNT_MESSAGES = [
  makeMessage({ whatsapp_account: 'account-1', created_at: '2026-02-16T10:00:00Z' }),
  makeMessage({ whatsapp_account: 'account-1', direction: 'outgoing', created_at: '2026-02-16T10:01:00Z' }),
]

const CONTACT = {
  id: CONTACT_ID,
  phone_number: '+1234567890',
  name: 'Test Multi Account',
  profile_name: 'Test Multi Account',
  status: 'active',
  unread_count: 0,
  whatsapp_account: 'account-1',
  created_at: '2026-02-16T09:00:00Z',
  updated_at: '2026-02-16T09:00:00Z',
}

function messagesEnvelope(messages: any[], accountFilter?: string) {
  const filtered = accountFilter
    ? messages.filter(m => m.whatsapp_account === accountFilter)
    : messages
  return {
    status: 'success',
    data: {
      messages: filtered,
      total: filtered.length,
      page: 1,
      limit: 50,
      has_more: false,
    },
  }
}

function contactsEnvelope() {
  return {
    status: 'success',
    data: {
      contacts: [CONTACT],
      total: 1,
      page: 1,
      limit: 50,
    },
  }
}

function contactEnvelope() {
  return { status: 'success', data: CONTACT }
}

/**
 * Intercept contacts + messages API to inject mock data.
 * Returns a function to update the messages fixture mid-test.
 */
async function setupMockRoutes(page: Page, messages: any[]) {
  let currentMessages = messages

  // Mock contacts list
  await page.route('**/api/contacts?*', async (route: Route) => {
    await route.fulfill({ json: contactsEnvelope() })
  })
  await page.route('**/api/contacts', async (route: Route) => {
    if (route.request().method() === 'GET') {
      await route.fulfill({ json: contactsEnvelope() })
    } else {
      await route.continue()
    }
  })

  // Mock single contact
  await page.route(`**/api/contacts/${CONTACT_ID}`, async (route: Route) => {
    if (route.request().method() === 'GET') {
      await route.fulfill({ json: contactEnvelope() })
    } else {
      await route.continue()
    }
  })

  // Mock messages - respect account filter query param
  await page.route(`**/api/contacts/${CONTACT_ID}/messages*`, async (route: Route) => {
    if (route.request().method() === 'GET') {
      const url = new URL(route.request().url())
      const account = url.searchParams.get('account') || undefined
      await route.fulfill({ json: messagesEnvelope(currentMessages, account) })
    } else {
      await route.continue()
    }
  })

  // Mock mark-read, session, etc. to avoid 404s
  await page.route(`**/api/contacts/${CONTACT_ID}/session`, async (route: Route) => {
    await route.fulfill({ json: { status: 'success', data: null } })
  })

  return {
    setMessages: (msgs: any[]) => { currentMessages = msgs },
  }
}

test.describe('Multi-Account Tabs', () => {
  let chatPage: ChatPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    chatPage = new ChatPage(page)
  })

  test('should show account tabs for multi-account contact', async ({ page }) => {
    await setupMockRoutes(page, MULTI_ACCOUNT_MESSAGES)
    await chatPage.goto(CONTACT_ID)
    await page.waitForTimeout(500)

    // Should show tabs for both accounts
    const tabs = chatPage.accountTabs
    await expect(tabs).toHaveCount(2)
    await expect(chatPage.getAccountTab('account-1')).toBeVisible()
    await expect(chatPage.getAccountTab('account-2')).toBeVisible()
  })

  test('should NOT show account tabs for single-account contact', async ({ page }) => {
    await setupMockRoutes(page, SINGLE_ACCOUNT_MESSAGES)
    await chatPage.goto(CONTACT_ID)
    await page.waitForTimeout(500)

    // Should not show tabs when only one account
    const tabs = chatPage.accountTabs
    await expect(tabs).toHaveCount(0)
  })

  test('should highlight the active account tab', async ({ page }) => {
    await setupMockRoutes(page, MULTI_ACCOUNT_MESSAGES)
    await chatPage.goto(CONTACT_ID)
    await page.waitForTimeout(500)

    // One tab should have the active (emerald) styling
    const activeTab = chatPage.activeAccountTab
    await expect(activeTab).toHaveCount(1)

    // The most recent incoming message is from account-2, so it should be selected
    await expect(activeTab).toHaveText('account-2')
  })

  test('should have visible inactive tab styling', async ({ page }) => {
    await setupMockRoutes(page, MULTI_ACCOUNT_MESSAGES)
    await chatPage.goto(CONTACT_ID)
    await page.waitForTimeout(500)

    const inactiveTabs = chatPage.inactiveAccountTabs
    await expect(inactiveTabs).toHaveCount(1)

    // Inactive tab should be visible (not transparent/invisible)
    const inactiveTab = inactiveTabs.first()
    await expect(inactiveTab).toBeVisible()
    // Check it has a background class for visibility
    const classes = await inactiveTab.getAttribute('class') || ''
    expect(classes).toContain('bg-white')
  })

  test('should switch account when clicking inactive tab', async ({ page }) => {
    await setupMockRoutes(page, MULTI_ACCOUNT_MESSAGES)
    await chatPage.goto(CONTACT_ID)
    await page.waitForTimeout(500)

    // Default active is account-2 (most recent incoming)
    await expect(chatPage.activeAccountTab).toHaveText('account-2')

    // Click account-1 tab
    await chatPage.switchAccount('account-1')
    await page.waitForTimeout(300)

    // Now account-1 should be active
    await expect(chatPage.activeAccountTab).toHaveText('account-1')
  })

  test('should fetch filtered messages when switching account', async ({ page }) => {
    await setupMockRoutes(page, MULTI_ACCOUNT_MESSAGES)
    await chatPage.goto(CONTACT_ID)
    await page.waitForTimeout(500)

    // Set up a request listener to verify the account filter is sent
    const messageRequests: string[] = []
    page.on('request', (req) => {
      if (req.url().includes('/messages') && req.method() === 'GET') {
        messageRequests.push(req.url())
      }
    })

    // Switch to account-1
    await chatPage.switchAccount('account-1')
    await page.waitForTimeout(300)

    // Verify a messages request was made with account=account-1
    const filtered = messageRequests.find(url => url.includes('account=account-1'))
    expect(filtered).toBeTruthy()
  })

  test('should not overlap with chat header', async ({ page }) => {
    await setupMockRoutes(page, MULTI_ACCOUNT_MESSAGES)
    await chatPage.goto(CONTACT_ID)
    await page.waitForTimeout(500)

    // Get the chat header and account tabs bounding boxes
    const header = page.locator('.h-14.flex-shrink-0').first()
    const tabsContainer = chatPage.getAccountTab('account-1').locator('..')

    const headerBox = await header.boundingBox()
    const tabsBox = await tabsContainer.boundingBox()

    // Both should exist
    expect(headerBox).toBeTruthy()
    expect(tabsBox).toBeTruthy()

    if (headerBox && tabsBox) {
      // Tabs should start at or below where the header ends (no overlap)
      expect(tabsBox.y).toBeGreaterThanOrEqual(headerBox.y + headerBox.height - 1)
    }
  })
})
