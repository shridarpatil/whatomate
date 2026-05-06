/**
 * Reusable test bodies for standard CRUD-shaped flows.
 *
 * Each function returns a Playwright test body — the spec author calls
 * `test('name', body(opts))` so they keep control over test names,
 * skip/only flags, and any pre-test setup.
 *
 * The helpers favour expressive selectors (role-based with text patterns)
 * over data-testids until that rollout happens. Once data-testids exist
 * on high-traffic admin views, helpers can opt into them.
 *
 *   test('list view loads', listLoadsBody({
 *     url: '/settings/webhooks',
 *     user: () => testUser,
 *     addButton: /Add Webhook/i,
 *   }))
 */

import { expect, type Locator, type Page } from '@playwright/test'
import { loginAs } from './auth'

export interface UserRef {
  email: string
  password: string
}

interface BaseOpts {
  url: string
  user: () => UserRef
}

/** Body for "list view loads" — smoke check on a list page. */
export function listLoadsBody(opts: BaseOpts & {
  addButton?: RegExp
  expectVisible?: RegExp[]
}): (args: { page: Page }) => Promise<void> {
  return async ({ page }) => {
    await loginAs(page, opts.user())
    await page.goto(opts.url)
    await page.waitForLoadState('networkidle')

    if (opts.addButton) {
      // .first() — pages sometimes render the button twice (header + empty-state).
      await expect(
        page.getByRole('button', { name: opts.addButton }).first(),
      ).toBeVisible({ timeout: 5000 })
    }
    for (const pattern of opts.expectVisible ?? []) {
      await expect(page.getByText(pattern).first()).toBeVisible({ timeout: 5000 })
    }
  }
}

/** Body for "permission-denied user can't see admin button". */
export function expectAddButtonHidden(opts: BaseOpts & {
  addButton: RegExp
}): (args: { page: Page }) => Promise<void> {
  return async ({ page }) => {
    await loginAs(page, opts.user())
    await page.goto(opts.url)
    await page.waitForLoadState('networkidle')

    await expect(
      page.getByRole('button', { name: opts.addButton }),
    ).toHaveCount(0, { timeout: 5000 })
  }
}

/**
 * One of three matchers — pick whichever the form supports.
 *
 * `label` works only when the UI library actually associates Label and
 * Input via a `for` attribute. Many shadcn-style components don't, in
 * which case use `placeholder` or `locator`.
 */
export type FieldFill = (
  | { label: string | RegExp }
  | { placeholder: string | RegExp }
  | { locator: (page: Page) => Locator }
) & { value: string }

function resolveFieldLocator(page: Page, field: FieldFill): Locator {
  if ('locator' in field) return field.locator(page)
  if ('label' in field) return page.getByLabel(field.label).first()
  return page.getByPlaceholder(field.placeholder).first()
}

/**
 * Body for "create flow" — opens add dialog, fills fields, submits,
 * expects the row to appear in the list.
 *
 * Assumes the dialog-based CRUD pattern (users, roles, canned responses,
 * tags, etc.). For detail-page-based create flows (webhooks, templates,
 * campaigns) write a bespoke test or extend this helper with a mode
 * parameter when the need is real.
 *
 * Doesn't try to be clever about field types — string fill is the only
 * flow handled today. Selects, file uploads, etc. need bespoke tests.
 */
export function createFlowBody(opts: BaseOpts & {
  addButton: RegExp
  fields: FieldFill[]
  submitButton?: RegExp
  /** A substring of the new row's identifier to look for in the list. */
  expectRow: string | RegExp
  /** Toast text to look for. Default: any toast containing /created|added/i. */
  successToast?: RegExp
}): (args: { page: Page }) => Promise<void> {
  return async ({ page }) => {
    await loginAs(page, opts.user())
    await page.goto(opts.url)
    await page.waitForLoadState('networkidle')

    await page.getByRole('button', { name: opts.addButton }).first().click()

    for (const field of opts.fields) {
      await resolveFieldLocator(page, field).fill(field.value)
    }

    const submit = opts.submitButton ?? /Save|Create|Add|Submit/i
    await page.getByRole('button', { name: submit }).last().click()

    const toast = opts.successToast ?? /created|added/i
    await expect(
      page.locator('[data-sonner-toast], [role="status"], [role="alert"]')
        .filter({ hasText: toast }),
    ).toBeVisible({ timeout: 5000 })

    await expect(page.getByText(opts.expectRow).first()).toBeVisible({ timeout: 5000 })
  }
}

/**
 * Body for "search filters the list" — types into a search box, asserts
 * a known row appears (or doesn't).
 *
 * Most list pages use a placeholder like "Search..." for the input.
 * Pass `searchInput` to override.
 */
export function searchListBody(opts: BaseOpts & {
  searchInput?: RegExp
  query: string
  expectVisible?: string | RegExp
  expectHidden?: string | RegExp
}): (args: { page: Page }) => Promise<void> {
  return async ({ page }) => {
    await loginAs(page, opts.user())
    await page.goto(opts.url)
    await page.waitForLoadState('networkidle')

    const placeholder = opts.searchInput ?? /Search/i
    await page.getByPlaceholder(placeholder).first().fill(opts.query)
    await page.waitForTimeout(400) // debounce window in most list views

    if (opts.expectVisible) {
      await expect(page.getByText(opts.expectVisible).first()).toBeVisible({ timeout: 5000 })
    }
    if (opts.expectHidden) {
      await expect(page.getByText(opts.expectHidden)).toHaveCount(0, { timeout: 5000 })
    }
  }
}

/**
 * Body for "edit flow" — finds a row by text, clicks edit, fills new
 * values into the dialog, submits, expects success toast. Does NOT
 * assert the new value renders in the list — caller should do that
 * separately if it matters.
 */
export function editFlowBody(opts: BaseOpts & {
  rowText: string | RegExp
  /** Per-row edit affordance label. Default: /edit/i. */
  rowEditButton?: RegExp
  /** Submit button. Default: /Update|Save/i. */
  submitButton?: RegExp
  fields: FieldFill[]
  successToast?: RegExp
}): (args: { page: Page }) => Promise<void> {
  return async ({ page }) => {
    await loginAs(page, opts.user())
    await page.goto(opts.url)
    await page.waitForLoadState('networkidle')

    const row = page.getByRole('row').filter({ hasText: opts.rowText }).first()
    await expect(row).toBeVisible({ timeout: 5000 })

    const editButton = opts.rowEditButton ?? /edit/i
    await row.getByRole('button', { name: editButton }).click()

    for (const field of opts.fields) {
      const input = resolveFieldLocator(page, field)
      await input.fill('')
      await input.fill(field.value)
    }

    const submit = opts.submitButton ?? /Update|Save/i
    await page.getByRole('button', { name: submit }).last().click()

    const toast = opts.successToast ?? /updated|saved/i
    await expect(
      page.locator('[data-sonner-toast], [role="status"], [role="alert"]')
        .filter({ hasText: toast }),
    ).toBeVisible({ timeout: 5000 })
  }
}

/**
 * Body for "delete flow" — finds a row by text, clicks its delete affordance,
 * confirms in the dialog, expects the row to disappear.
 */
export function deleteFlowBody(opts: BaseOpts & {
  rowText: string | RegExp
  /** Per-row delete button label. Default: /delete|trash/i. */
  rowDeleteButton?: RegExp
  /** Confirmation dialog button label. Default: /Delete|Confirm/i. */
  confirmButton?: RegExp
}): (args: { page: Page }) => Promise<void> {
  return async ({ page }) => {
    await loginAs(page, opts.user())
    await page.goto(opts.url)
    await page.waitForLoadState('networkidle')

    const row = page.getByRole('row').filter({ hasText: opts.rowText }).first()
    await expect(row).toBeVisible({ timeout: 5000 })

    const deleteButton = opts.rowDeleteButton ?? /delete/i
    await row.getByRole('button', { name: deleteButton }).click()

    const confirm = opts.confirmButton ?? /^Delete|^Confirm/i
    await page.getByRole('button', { name: confirm }).click()

    await expect(page.getByText(opts.rowText).first()).toHaveCount(0, { timeout: 5000 })
  }
}
