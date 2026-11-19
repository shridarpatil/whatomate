import type { Locator } from '@playwright/test'

/** Pick the first option of a native <select> whose text matches `text` (partial match). */
export async function selectOptionMatching(select: Locator, text: RegExp): Promise<void> {
  const value = await select.locator('option').filter({ hasText: text }).first().getAttribute('value')
  if (value === null) throw new Error(`No option matching ${text}`)
  await select.selectOption(value)
}
