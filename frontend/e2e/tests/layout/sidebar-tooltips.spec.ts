import { test, expect } from '@playwright/test'
import { loginAsAdmin } from '../../helpers'

test.describe('Collapsed sidebar', () => {
  test('shows the menu name on hover when collapsed', async ({ page }) => {
    await loginAsAdmin(page)

    // A barra já nasce recolhida por padrão — nada a clicar aqui.
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

    // A barra nasce recolhida por padrão — expande pelo próprio controle da
    // interface para testar o estado oposto. Isso dispara a mesma transição
    // de largura (duration-300) que o teste do Settings já lida com: hover()
    // espera a animação parar, mas o salto de posição pode não gerar um
    // pointermove que o reka-ui reconheça depois que ela assenta. Sem o
    // nudge, um hover que não "pegou" faria este teste passar por acidente
    // — a ausência do tooltip provaria pouco se o mouse nunca chegou lá.
    await page.getByRole('button', { name: /expand sidebar|expandir/i }).click()

    const chatLink = page.getByRole('menuitem').first()
    await chatLink.hover()
    const box = (await chatLink.boundingBox())!
    await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2 + 1)
    // A tooltip usa :delay-duration="150"; espera passar do delay para que a
    // asserção prove que o guard v-if="isCollapsed" funciona, em vez de
    // passar trivialmente porque o poll rodou antes do popper poder montar.
    await page.waitForTimeout(600)

    await expect(page.locator('[data-reka-popper-content-wrapper]')).toHaveCount(0)
  })

  test('shows the Settings tooltip on hover when collapsed', async ({ page }) => {
    await loginAsAdmin(page)

    // A barra já nasce recolhida por padrão — nada a clicar aqui.
    const settingsLink = page.locator('a[role="menuitem"][href="/settings"]')
    const label = ((await settingsLink.textContent()) ?? '').trim()

    // O hover é um salto longo (topo -> ícone fixo no rodapé), e às vezes não
    // gera um pointermove que o reka-ui reconheça depois de aterrissar, então
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
