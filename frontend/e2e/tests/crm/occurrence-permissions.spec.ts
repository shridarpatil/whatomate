import { test, expect } from '@playwright/test'
import { createTestScope, createUserWithPermissions, loginAs } from '../../framework'
import { ApiHelper } from '../../helpers/api'

const scope = createTestScope('crm-perms')

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

    // Sem occurrences.stages, o item "Occurrence Stages" nao e oferecido no
    // menu (e a secao "Settings" inteira some, ja que so.existe por causa
    // dele para este papel) — e a asserção que pegaria alguém alargando
    // `childPermissions` displicentemente no futuro.
    await expect(page.getByRole('menuitem', { name: 'Occurrence Stages' })).toHaveCount(0)
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

    // E pelo menu: este papel nao tem nenhuma permissao settings.*, entao o
    // item pai "Settings" resolve direto para o unico filho que ele pode
    // abrir (AppLayout.vue calcula esse effectivePath), e a linha "Occurrence
    // Stages" aparece expandida por baixo dele.
    const settingsLink = page.getByRole('menuitem', { name: 'Settings', exact: true })
    await expect(settingsLink).toBeVisible()
    await expect(settingsLink).toHaveAttribute('href', '/settings/occurrence-stages')
    await expect(page.getByRole('menuitem', { name: 'Occurrence Stages' })).toBeVisible()
  })
})
