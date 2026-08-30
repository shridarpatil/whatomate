import { test, expect } from '@playwright/test'
import { createTestScope, createUserWithPermissions, loginAs } from '../../framework'
import { ApiHelper } from '../../helpers/api'
import { ChatPage } from '../../pages'

const scope = createTestScope('crm-perms')

/** Fresh contact per test, same rationale as crm/occurrences.spec.ts: the
 * suite runs fullyParallel so tests must not share state. */
async function createContact(api: ApiHelper): Promise<string> {
  await api.loginAsAdmin()
  const contact = await api.createContact(scope.phone(), scope.name('contact'))
  return contact.id
}

test.describe('CRM permissions', () => {
  test('a role without occurrences cannot reach the CRM', async ({ page, request }) => {
    const api = new ApiHelper(request)
    await api.loginAsAdmin()
    const user = await createUserWithPermissions(api, scope, {
      userSlug: 'sem-crm',
      permissions: [
        { resource: 'chat', action: 'read' },
        { resource: 'chat', action: 'write' },
      ],
    })

    await loginAs(page, user)
    await page.goto('/crm/occurrences')
    await page.waitForLoadState('networkidle')

    // Nem o item de menu, nem a tela.
    await expect(page.locator('#occurrences-list')).toBeHidden()
    await expect(page.locator('#occurrences-board')).toBeHidden()
    expect(page.url()).not.toContain('/crm/occurrences')
  })

  // The ticket icon in the conversation header used to be implicitly gated by
  // the CRM route requiring 'chat'. Now occurrences has its own permission,
  // so this is the one place left where a chat-only agent could still reach
  // the CRM panel (and get an erroring one, since GET
  // /api/contacts/{id}/occurrences requires occurrences:read).
  test('a chat-only role does not see the occurrences button on an open conversation', async ({ page, request }) => {
    const api = new ApiHelper(request)
    await api.loginAsAdmin()
    const contactId = await createContact(api)
    const user = await createUserWithPermissions(api, scope, {
      userSlug: 'so-chat',
      permissions: [
        { resource: 'chat', action: 'read' },
        { resource: 'chat', action: 'write' },
      ],
    })
    // Without contacts:read, visibility is scoped to assigned/transferred
    // contacts (scopeVisibleConversations) — assign it so the conversation
    // actually opens for this user instead of silently 404ing.
    await api.put(`/api/contacts/${contactId}/assign`, { user_id: user.user.id })

    try {
      await loginAs(page, user)
      const chatPage = new ChatPage(page)
      await chatPage.goto(contactId)
      // Proves the conversation actually opened (not a vacuous pass because
      // the whole header failed to render): the sibling notes button, which
      // carries no occurrences gating, must be visible.
      await expect(page.locator('#notes-button')).toBeVisible()

      await expect(page.locator('#occurrences-button')).toBeHidden()
    } finally {
      // global-cleanup.ts deletes E2E users before E2E contacts; a contact
      // left assigned to a since-deleted user violates fk_contacts_assigned_user
      // on the next run. Unassign so this test doesn't leave that behind.
      await api.put(`/api/contacts/${contactId}/assign`, { user_id: null })
    }
  })

  test('a CRM role cannot reach the stage settings, in the menu or by URL', async ({ page, request }) => {
    const api = new ApiHelper(request)
    await api.loginAsAdmin()
    const user = await createUserWithPermissions(api, scope, {
      userSlug: 'so-crm',
      permissions: [
        { resource: 'chat', action: 'read' },
        { resource: 'chat', action: 'write' },
        { resource: 'occurrences', action: 'read' },
        { resource: 'occurrences', action: 'write' },
      ],
    })

    await loginAs(page, user)

    await page.goto('/crm/occurrences')
    await page.waitForLoadState('networkidle')
    await expect(page.locator('#occurrences-list')).toBeVisible()

    // Sem occurrences.stages, a secao "Settings" inteira some do menu (ja que
    // so existe, para este papel, por causa do item "Occurrence Stages") —
    // essa e a asserção load-bearing que pegaria alguém alargando
    // `childPermissions` displicentemente no futuro. (Uma asserção separada de
    // "Occurrence Stages" nao provaria nada aqui: os filhos do submenu só
    // renderizam quando o pai está ativo — AppLayout.vue:216 — e o usuário
    // está em /crm/occurrences neste ponto, entao a contagem seria 0 de
    // qualquer forma.)
    await expect(page.getByRole('menuitem', { name: 'Settings', exact: true })).toHaveCount(0)

    await page.goto('/settings/occurrence-stages')
    await page.waitForLoadState('networkidle')
    expect(page.url()).not.toContain('/settings/occurrence-stages')
  })

  // O caso positivo, e o que teria pego o bug que quase foi para o plano:
  // a guarda de rota checa a ação `read` com a ação FIXA, então um papel que
  // pode editar etapas mas não tem occurrences.stages:read seria barrado da
  // própria tela que administra.
  test('a stage-admin role reaches the stage settings, in the menu and by URL', async ({ page, request }) => {
    const api = new ApiHelper(request)
    await api.loginAsAdmin()
    const user = await createUserWithPermissions(api, scope, {
      userSlug: 'gestor-funil',
      permissions: [
        { resource: 'chat', action: 'read' },
        { resource: 'chat', action: 'write' },
        { resource: 'occurrences', action: 'read' },
        { resource: 'occurrences', action: 'write' },
        { resource: 'occurrences.stages', action: 'read' },
        { resource: 'occurrences.stages', action: 'write' },
        { resource: 'occurrences.stages', action: 'delete' },
      ],
    })

    await loginAs(page, user)

    await page.goto('/settings/occurrence-stages')
    await page.waitForLoadState('networkidle')

    expect(page.url()).toContain('/settings/occurrence-stages')
    await expect(page.locator('h1').filter({ hasText: 'Occurrence Stages' })).toBeVisible()
    // PageHeader (o h1 acima) fica fora do ramo de erro do componente — um 403
    // em GET /api/occurrence-stages (gate real: occurrences:read, nao a
    // permissao de stages) deixaria o h1 visivel e este teste verde mesmo
    // assim. A linha da etapa semeada "Aberto" e a prova de que a tela
    // funcionou de fato.
    await expect(page.getByRole('cell', { name: 'Aberto' })).toBeVisible()

    // E pelo menu: este papel nao tem nenhuma permissao settings.*, entao o
    // item pai "Settings" resolve direto para o unico filho que ele pode
    // abrir (AppLayout.vue calcula esse effectivePath), e a linha "Occurrence
    // Stages" aparece expandida por baixo dele.
    const settingsLink = page.getByRole('menuitem', { name: 'Settings', exact: true })
    await expect(settingsLink).toBeVisible()
    await expect(settingsLink).toHaveAttribute('href', '/settings/occurrence-stages')
    await expect(page.getByRole('menuitem', { name: 'Occurrence Stages' })).toBeVisible()
  })

  // navigationOrder (router/index.ts) nao tinha entrada para /crm/occurrences:
  // getFirstAccessibleRoute caia em todos os ramos e mandava esse papel pro
  // fallback '/profile'. Esse persona (occurrences sem chat) so passou a
  // existir com o modulo de CRM ganhando permissao propria — todo teste
  // anterior concede chat:read, o que escondia o bug.
  test('a CRM-only role (no chat) lands on the CRM after login', async ({ page, request }) => {
    const api = new ApiHelper(request)
    await api.loginAsAdmin()
    const user = await createUserWithPermissions(api, scope, {
      userSlug: 'so-ocorrencias',
      permissions: [
        { resource: 'occurrences', action: 'read' },
        { resource: 'occurrences', action: 'write' },
      ],
    })

    await loginAs(page, user)
    await page.waitForLoadState('networkidle')

    expect(page.url()).toContain('/crm/occurrences')
    await expect(page.locator('#occurrences-list')).toBeVisible()
  })
})
