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
  })
})
