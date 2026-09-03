import { test, expect, request as playwrightRequest } from '@playwright/test'
import { loginAsAdmin } from '../../helpers'
import { ApiHelper } from '../../helpers/api'
import { ChatPage } from '../../pages'
import { createTestScope } from '../../framework'

const scope = createTestScope('contact-info')

test.describe('Contact info panel', () => {
  let contactId: string
  let phone: string

  test.beforeAll(async () => {
    const ctx = await playwrightRequest.newContext()
    const api = new ApiHelper(ctx)
    await api.loginAsAdmin()
    phone = scope.phone()
    const contact = await api.createContact(phone, scope.name('info'))
    contactId = contact.id
    await ctx.dispose()
  })

  test('copies the phone number and renames the contact', async ({ page, context }) => {
    await context.grantPermissions(['clipboard-read', 'clipboard-write'])
    await loginAsAdmin(page)

    const chat = new ChatPage(page)
    await chat.goto(contactId)
    await chat.openContactInfo()

    const shown = (await page.locator('#contact-info-phone').innerText()).trim()
    await page.locator('#contact-info-copy-phone').click()
    const clipboard = await page.evaluate(() => navigator.clipboard.readText())
    expect(clipboard).toBe(shown)

    const novo = scope.name('renomeado')
    await page.locator('#contact-info-edit-name').click()
    await page.locator('#contact-info-name-input').fill(novo)
    await page.locator('#contact-info-save-name').click()

    await expect(page.locator('#contact-info-name')).toHaveText(novo)
    // The store syncs both `name` and `profile_name` because the panel reads
    // one and the conversation list (and other ChatView spots) read the
    // other — assert the list picks up the rename too, same load, no reload.
    await expect(
      page.locator('[data-testid="conversation-item"]').filter({ hasText: novo })
    ).toBeVisible()
  })

  test('switching contacts while the rename box is open does not rename the wrong one', async ({ page }) => {
    await loginAsAdmin(page)

    const ctx = await playwrightRequest.newContext()
    const api = new ApiHelper(ctx)
    await api.loginAsAdmin()
    const nameA = scope.name('switch-a')
    const nameB = scope.name('switch-b')
    const contactA = await api.createContact(scope.phone(), nameA)
    const contactB = await api.createContact(scope.phone(), nameB)

    try {
      const chat = new ChatPage(page)
      await chat.goto(contactA.id)
      await chat.openContactInfo()

      await page.locator('#contact-info-edit-name').click()
      await page.locator('#contact-info-name-input').fill(scope.name('draft-for-a'))

      // Switch to contact B from the conversation list while A's rename box
      // is still open — the original repro. Search narrows the list to our
      // scoped contacts so the click doesn't depend on list ordering.
      await chat.searchContacts(nameB)
      await page.locator('[data-testid="conversation-item"]').filter({ hasText: nameB }).click()
      await page.waitForLoadState('networkidle')

      // The rename box must not survive the contact switch.
      await expect(page.locator('#contact-info-name-input')).toHaveCount(0)
      await expect(page.locator('#contact-info-name')).toHaveText(nameB)

      // If the box were still open (bug present) this clicks Save, which
      // posts the stale "draft-for-a" text to contact B's id.
      const saveButton = page.locator('#contact-info-save-name')
      if (await saveButton.count() > 0) {
        await saveButton.click()
      }

      const contacts = await api.getContacts()
      const stillB = contacts.find((c: any) => c.id === contactB.id)
      expect(stillB?.name).toBe(nameB)
    } finally {
      await api.deleteContact(contactA.id)
      await api.deleteContact(contactB.id)
      await ctx.dispose()
    }
  })
})
