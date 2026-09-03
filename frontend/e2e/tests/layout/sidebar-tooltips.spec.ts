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

    await expect(page.locator('[data-reka-popper-content-wrapper]')).toHaveCount(0)
  })
})
