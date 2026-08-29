import { test, expect } from '@playwright/test'
import { loginAsAdmin } from '../../helpers'
import { ApiHelper } from '../../helpers/api'
import { createTestScope } from '../../framework'
import { ChatPage, OccurrencesPage } from '../../pages'

const scope = createTestScope('crm-board')

/** Um contato por teste: a suíte roda em paralelo e contagens por organização
 * ficariam intermitentes, mas um painel por contato e um protocolo único não. */
async function createContact(api: ApiHelper): Promise<string> {
  await api.loginAsAdmin()
  const contact = await api.createContact(scope.phone(), scope.name('contact'))
  return contact.id
}

test.describe('CRM occurrence board', () => {
  let occurrencesPage: OccurrencesPage

  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page)
    occurrencesPage = new OccurrencesPage(page)
  })

  test('the view mode preference survives a reload', async ({ page }) => {
    await occurrencesPage.gotoList()
    await expect(occurrencesPage.listView).toBeVisible()

    await occurrencesPage.switchToBoard()
    await expect(occurrencesPage.boardView).toBeVisible()

    await page.reload()
    await page.waitForLoadState('networkidle')
    await expect(occurrencesPage.boardView).toBeVisible()

    await occurrencesPage.switchToList()
    await page.reload()
    await page.waitForLoadState('networkidle')
    await expect(occurrencesPage.listView).toBeVisible()
  })
})
