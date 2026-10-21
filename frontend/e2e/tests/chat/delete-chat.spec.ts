import { test, expect, request as playwrightRequest, type APIRequestContext } from '@playwright/test'
import { ApiHelper } from '../../helpers'
import { ChatPage } from '../../pages'
import {
  createTestScope,
  createUserWithPermissions,
  loginAs,
  loginAsSuperAdmin,
  SUPER_ADMIN,
  type TestUserHandle,
} from '../../framework'

const scope = createTestScope('delete-chat')

async function superAdminApi(request: APIRequestContext): Promise<ApiHelper> {
  const api = new ApiHelper(request)
  await api.login(SUPER_ADMIN.email, SUPER_ADMIN.password)
  return api
}

test.describe('Chat contact options menu', () => {
  let noDeleteUser: TestUserHandle

  test.beforeAll(async () => {
    const ctx = await playwrightRequest.newContext()
    const api = await superAdminApi(ctx)
    // No userSlug: tests run fully parallel, so each worker runs this hook and
    // needs its own randomly named role.
    noDeleteUser = await createUserWithPermissions(api, scope, {
      permissions: [
        { resource: 'chat', action: 'read' },
        { resource: 'chat', action: 'write' },
        { resource: 'contacts', action: 'read' },
      ],
    })
    await ctx.dispose()
  })

  test.afterAll(async () => {
    const ctx = await playwrightRequest.newContext()
    const api = await superAdminApi(ctx)
    await api.deleteUser(noDeleteUser.user.id).catch(() => {})
    await api.deleteRole(noDeleteUser.role.id).catch(() => {})
    await ctx.dispose()
  })

  test('three-dot menu opens and shows contact options', async ({ page, request }) => {
    const api = await superAdminApi(request)
    const contact = await api.createContact(scope.phone(), scope.name('menu'))

    await loginAsSuperAdmin(page)
    const chatPage = new ChatPage(page)
    await chatPage.goto(contact.id)

    await chatPage.openContactOptions()
    await expect(chatPage.contactOptionsMenu.getByText('Contact Options')).toBeVisible()
    await expect(chatPage.contactOptionsMenu.getByRole('menuitem', { name: /contact details/i })).toBeVisible()
  })

  test('cancelling the confirmation keeps the chat', async ({ page, request }) => {
    const api = await superAdminApi(request)
    const contact = await api.createContact(scope.phone(), scope.name('cancel'))

    await loginAsSuperAdmin(page)
    const chatPage = new ChatPage(page)
    await chatPage.goto(contact.id)

    await chatPage.openContactOptions()
    await chatPage.deleteChatMenuItem.click()
    await expect(chatPage.deleteChatDialog).toBeVisible()
    await chatPage.deleteChatDialog.getByRole('button', { name: 'Cancel' }).click()
    await expect(chatPage.deleteChatDialog).toBeHidden()

    await expect(page).toHaveURL(new RegExp(`/chat/${contact.id}$`))
    const resp = await api.get(`/api/contacts/${contact.id}`)
    expect(resp.status()).toBe(200)
  })

  test('deleting a chat removes it and returns to the chat list', async ({ page, request }) => {
    const api = await superAdminApi(request)
    const name = scope.name('delete')
    const contact = await api.createContact(scope.phone(), name)

    await loginAsSuperAdmin(page)
    const chatPage = new ChatPage(page)
    await chatPage.goto(contact.id)

    await chatPage.openContactOptions()
    await chatPage.deleteChatMenuItem.click()
    await expect(chatPage.deleteChatDialog).toContainText(name)
    await chatPage.deleteChatDialog.getByRole('button', { name: 'Delete' }).click()

    await expect(page.getByText('Chat deleted')).toBeVisible()
    await expect(page).toHaveURL(/\/chat$/)
    await expect(page.getByText(name)).toHaveCount(0)

    // API side-channel: the contact is gone for the org.
    const resp = await api.get(`/api/contacts/${contact.id}`)
    expect(resp.status()).toBe(404)
  })

  test('delete option is hidden without contacts:delete', async ({ page, request }) => {
    const api = await superAdminApi(request)
    const contact = await api.createContact(scope.phone(), scope.name('no-perm'))

    await loginAs(page, noDeleteUser)
    const chatPage = new ChatPage(page)
    await chatPage.goto(contact.id)

    await chatPage.openContactOptions()
    await expect(chatPage.contactOptionsMenu.getByText('Contact Options')).toBeVisible()
    await expect(chatPage.deleteChatMenuItem).toHaveCount(0)
  })

  test('API rejects chat deletion without contacts:delete', async ({ request }) => {
    const adminCtx = await playwrightRequest.newContext()
    const admin = await superAdminApi(adminCtx)
    const contact = await admin.createContact(scope.phone(), scope.name('api-no-perm'))

    const userApi = new ApiHelper(request)
    await userApi.login(noDeleteUser.email, noDeleteUser.password)
    const resp = await userApi.del(`/api/contacts/${contact.id}/conversation`)
    expect(resp.status()).toBe(403)

    const stillThere = await admin.get(`/api/contacts/${contact.id}`)
    expect(stillThere.status()).toBe(200)
    await adminCtx.dispose()
  })
})
