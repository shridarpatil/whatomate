import { test, expect } from '@playwright/test'
import { loginAsAdmin } from '../../helpers'

test.describe('Collapsed sidebar', () => {
  test('shows the menu name on hover when collapsed', async ({ page }) => {
    await loginAsAdmin(page)

    // Recolhe a barra pelo próprio controle da interface.
    await page.getByRole('button', { name: /collapse sidebar|recolher/i }).click()

    const chatLink = page.getByRole('menuitem').first()
    // textContent ignora a visibilidade CSS (o span sr-only está oculto, não
    // removido), então lê o rótulo de forma confiável independente do estado
    // recolhido. Isso também deixa o teste independente de qual item vem
    // primeiro no menu, então ele não apodrece se a ordem mudar.
    const label = ((await chatLink.textContent()) ?? '').trim()

    await chatLink.hover()

    // O conteúdo visível do tooltip (reka-ui) não carrega role="tooltip" —
    // esse role fica só no span sr-only oculto usado para aria-describedby.
    // O wrapper do popper é o seletor confiável para o balão visível.
    const tooltip = page.locator('[data-reka-popper-content-wrapper]')
    await expect(tooltip).toBeVisible()
    await expect(tooltip).toContainText(label)
  })

  test('does not show tooltips while expanded', async ({ page }) => {
    await loginAsAdmin(page)

    const chatLink = page.getByRole('menuitem').first()
    await chatLink.hover()
    // A tooltip usa :delay-duration="150"; espera passar do delay para que a
    // asserção prove que o guard v-if="isCollapsed" funciona, em vez de
    // passar trivialmente porque o poll rodou antes do popper poder montar.
    await page.waitForTimeout(600)

    await expect(page.locator('[data-reka-popper-content-wrapper]')).toHaveCount(0)
  })

  test('shows the Settings tooltip on hover when collapsed', async ({ page }) => {
    await loginAsAdmin(page)

    await page.getByRole('button', { name: /collapse sidebar|recolher/i }).click()

    const settingsLink = page.locator('a[role="menuitem"][href="/settings"]')
    const label = ((await settingsLink.textContent()) ?? '').trim()

    // hover() espera a animação de largura (transition-all duration-300)
    // terminar antes de considerar o elemento acionável. Mas o próprio hover
    // é um salto longo (topo -> ícone fixo no rodapé), e às vezes não gera um
    // pointermove que o reka-ui reconheça depois que a animação assenta, então
    // a grace area do tooltip não abre (~1-em-8 flake). Ler o boundingBox só
    // DEPOIS do hover, e mover 1px, força um pointermove novo sobre a
    // coordenada final já estável.
    await settingsLink.hover()
    const box = (await settingsLink.boundingBox())!
    await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2 + 1)
    await page.waitForTimeout(600)

    const tooltip = page.locator('[data-reka-popper-content-wrapper]')
    await expect(tooltip).toBeVisible()
    await expect(tooltip).toContainText(label)
  })
})
