import { test, expect, request as playwrightRequest } from '@playwright/test'
import { loginAsAdmin } from '../../helpers'
import { ApiHelper } from '../../helpers/api'
import { ChatPage } from '../../pages'

const TEST_METADATA = {
  plan: 'premium',
  age: 30,
  active: true,
  address: {
    city: 'Mumbai',
    state: 'Maharashtra',
    zip: '400001',
  },
  orders: [
    { id: 'ORD-001', amount: 1500, status: 'delivered' },
    { id: 'ORD-002', amount: 2300, status: 'pending' },
  ],
  interests: ['fitness', 'tech', 'travel'],
}

test.describe('Contact Metadata Panel', () => {
  test.describe.configure({ mode: 'serial' }) // Tests share contact metadata state

  let contactId: string

  test.beforeAll(async () => {
    const reqContext = await playwrightRequest.newContext()
    const api = new ApiHelper(reqContext)
    await api.loginAsAdmin()

    // Get existing contacts or create one
    let contacts = await api.getContacts()
    if (contacts.length === 0) {
      await api.createContact(`91${Date.now().toString().slice(-10)}`, 'Metadata Test')
      contacts = await api.getContacts()
    }

    contactId = contacts[0].id

    // Set metadata on the contact
    await api.updateContact(contactId, { metadata: TEST_METADATA })
    await reqContext.dispose()
  })

  let chatPage: ChatPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    chatPage = new ChatPage(page)
  })

  async function openInfoPanel(page: import('@playwright/test').Page) {
    await chatPage.goto(contactId)

    // Open contact info panel
    const infoBtn = page.locator('#info-button')
    await infoBtn.click()

    // Wait for the panel header to appear
    await expect(page.getByText('Contact Info')).toBeVisible({ timeout: 10000 })
  }

  test('should display metadata section when contact has metadata', async ({ page }) => {
    await openInfoPanel(page)

    // "General" section should show for top-level primitives (plan, age, active)
    await expect(page.getByText('General')).toBeVisible()

    // Verify primitive values are displayed
    await expect(page.getByText('premium')).toBeVisible()
    await expect(page.getByText('30', { exact: true })).toBeVisible()
  })

  test('should display nested object metadata as key-value pairs', async ({ page }) => {
    await openInfoPanel(page)

    // "Address" section header (formatted from "address")
    await expect(page.getByText('Address')).toBeVisible()

    // Key-value pairs inside the address section
    await expect(page.getByText('Mumbai')).toBeVisible()
    await expect(page.getByText('Maharashtra')).toBeVisible()
    await expect(page.getByText('400001')).toBeVisible()
  })

  test('should display array of objects as a table', async ({ page }) => {
    await openInfoPanel(page)

    // "Orders" section header (formatted from "orders")
    await expect(page.getByText('Orders')).toBeVisible()

    // Table should show the count
    await expect(page.getByText('(2)')).toBeVisible()

    // Table column headers
    await expect(page.locator('th').getByText('Id')).toBeVisible()
    await expect(page.locator('th').getByText('Amount')).toBeVisible()
    await expect(page.locator('th').getByText('Status')).toBeVisible()

    // Table data
    await expect(page.getByText('ORD-001')).toBeVisible()
    await expect(page.getByText('1500')).toBeVisible()
    await expect(page.getByText('delivered')).toBeVisible()
    await expect(page.getByText('ORD-002')).toBeVisible()
  })

  test('should display array of primitives as badges', async ({ page }) => {
    await openInfoPanel(page)

    // "Interests" section header
    await expect(page.getByText('Interests')).toBeVisible()

    // Badges for each interest
    await expect(page.getByText('fitness')).toBeVisible()
    await expect(page.getByText('tech')).toBeVisible()
    await expect(page.getByText('travel')).toBeVisible()
  })

  test('should display boolean metadata as Yes/No badges', async ({ page }) => {
    await openInfoPanel(page)

    // The "Active" label should appear in the General section
    await expect(page.getByText('Active')).toBeVisible()

    // Boolean true should show as "Yes" badge
    await expect(page.getByText('Yes')).toBeVisible()
  })

  test('should collapse and expand metadata sections', async ({ page }) => {
    await openInfoPanel(page)

    // Click "Address" section header to collapse it
    const addressTrigger = page.locator('button').filter({ hasText: 'Address' })
    await addressTrigger.click()

    // Values should be hidden after collapse
    await expect(page.getByText('Mumbai')).toBeHidden()

    // Click again to expand
    await addressTrigger.click()

    // Values should be visible again
    await expect(page.getByText('Mumbai')).toBeVisible()
  })

  test('should format metadata labels from snake_case and camelCase', async ({ page }) => {
    // Update contact with snake_case and camelCase keys
    const reqContext = await playwrightRequest.newContext()
    const api = new ApiHelper(reqContext)
    await api.loginAsAdmin()
    await api.updateContact(contactId, {
      metadata: {
        first_name: 'John',
        lastName: 'Doe',
      },
    })
    await reqContext.dispose()

    await openInfoPanel(page)

    // snake_case "first_name" should become "First Name"
    await expect(page.getByText('First Name')).toBeVisible()

    // camelCase "lastName" should become "Last Name"
    await expect(page.getByText('Last Name')).toBeVisible()

    // Restore original metadata for other tests
    const restoreCtx = await playwrightRequest.newContext()
    const restoreApi = new ApiHelper(restoreCtx)
    await restoreApi.loginAsAdmin()
    await restoreApi.updateContact(contactId, { metadata: TEST_METADATA })
    await restoreCtx.dispose()
  })

  test('should not show metadata section when contact has no metadata', async ({ page }) => {
    // Clear metadata
    const reqContext = await playwrightRequest.newContext()
    const api = new ApiHelper(reqContext)
    await api.loginAsAdmin()
    await api.updateContact(contactId, { metadata: {} })
    await reqContext.dispose()

    await openInfoPanel(page)

    // "General" section from metadata should not be visible
    await expect(page.getByText('General')).toBeHidden()

    // Restore metadata
    const restoreCtx = await playwrightRequest.newContext()
    const restoreApi = new ApiHelper(restoreCtx)
    await restoreApi.loginAsAdmin()
    await restoreApi.updateContact(contactId, { metadata: TEST_METADATA })
    await restoreCtx.dispose()
  })
})
