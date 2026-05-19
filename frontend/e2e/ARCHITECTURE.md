# E2E Test Architecture

This document codifies the conventions for end-to-end tests under `frontend/e2e/`.
It exists because several non-obvious patterns and gotchas have already cost
debugging time and will keep doing so unless they're written down.

If something below contradicts existing code, the document is the spec — fix
the code.

---

## Layout

```
frontend/e2e/
├── ARCHITECTURE.md         <-- this file
├── global-setup.ts         Logs in as the default super-admin once and
│                           creates admin@test.com / manager@test.com /
│                           agent@test.com if missing.
├── helpers/
│   ├── api.ts              ApiHelper — typed wrapper over Playwright's
│   │                       APIRequestContext with cookie + CSRF handling.
│   ├── audit.ts            verifyAuditLogged() — polls /api/audit-logs
│   │                       for a given (resource_type, resource_id, action).
│   ├── auth.ts             login() / loginAsAdmin() / logout() / TestUser
│   │                       constants.
│   ├── detail-page.ts      Helpers for the standard Detail page layout.
│   ├── fixtures.ts         generateUniqueEmail/Name + per-resource factory
│   │                       fixtures (UserFixtures, TeamFixtures, …).
│   └── index.ts            Re-export barrel.
├── pages/                  Page Object Model — one class per logical view.
│   ├── BasePage.ts
│   ├── TablePage.ts        Reusable list-page operations (search, addBtn,
│   │                       deleteRow, sorting helpers).
│   ├── DialogPage.ts       Generic dialog form helpers (fillField,
│   │                       selectOption, submit, waitForOpen/Close).
│   ├── LoginPage.ts
│   └── …Page.ts            One per major view.
└── tests/                  Specs grouped by feature area.
    ├── auth/
    ├── settings/
    ├── chatbot/
    ├── calling/
    ├── chat/
    └── …
```

---

## Two test styles

**Default: page-based (UI-driven).** This is an admin-heavy product — most
defects show up in render state, watchers, permission gates, and form
flow. Bugs of that shape are invisible to API-only tests. Examples from
the recent past that API-only would have missed entirely:

- Custom-role users seeing the wrong tabs because of a role-name vs
  permission gate.
- A date picker losing its calendar grid after Apply because of a
  serialize/restore mismatch.
- A chat unread divider racing with a watcher and disappearing.
- A flow's transfer step printing "Connecting you to an agent..." right
  before its sibling printed "We're closed".

If a behavior has any UI surface, test it through the UI. Layer API
side-channel assertions inside that test for things the UI can't show
(audit log entries, server-side flags, rate-limit counters).

### Page-based (`{ page }` fixture, optionally + `{ request }`)

Use for anything user-visible: list/detail/CRUD flows, search, filters,
permission-gated UI variations, dialogs, toasts, real-time updates, role
changes, error rendering.

```ts
test('admin can create + see + delete a webhook', async ({ page, request }) => {
  const api = new ApiHelper(request)
  await api.login('admin@admin.com', 'admin')

  await loginAsAdmin(page)
  await page.goto('/settings/webhooks')

  // UI flow
  await page.getByRole('button', { name: /Add Webhook/i }).click()
  await page.getByLabel('Name').fill('alerts')
  await page.getByLabel('URL').fill('https://example.com/hook')
  await page.getByRole('button', { name: /Create/i }).click()
  await expect(page.getByText('alerts')).toBeVisible()

  // API side-channel: confirm audit log fires for the same flow
  const { data: list } = await api.get('/api/webhooks')
  const wh = list.body.data.webhooks.find((w: any) => w.name === 'alerts')
  await verifyAuditLogged(request, 'webhook', wh.id, 'created')
})
```

The UI test owns the scenario. The API call only verifies the side effect
that doesn't appear in the DOM. Don't replace the UI flow with the API
flow.

### API-only (`{ request }` fixture) — narrower role

Use when there's **no UI surface** for what's being tested. Examples:

- Webhook signature verification (incoming Meta webhooks have no UI).
- HMAC + replay protection on outbound webhooks.
- Permission negative tests on raw endpoints (`agent_role hits POST
  /api/users → 403`).
- Response-shape / contract assertions for the public API.
- Rate-limit response headers and 429 behavior.

```ts
test('agent role cannot create users', async ({ request }) => {
  const api = new ApiHelper(request)
  await api.login('agent@test.com', 'password')
  const resp = await api.post('/api/users', { email: 'x@x.com', ... })
  expect(resp.status()).toBe(403)
})
```

Before reaching for API-only, ask: is there a UI flow that hits this
endpoint? If yes, prefer the UI test and add the API assertion as a
side-channel. API-only is the exception, not the default.

### Why this is a flip from earlier guidance

Previous versions of this doc defaulted to API-only because page tests
are slower and flakier. Both are true, but the alternative — discovering
UI bugs in production — is worse for an admin product whose value lives
in the UI. The framework changes outlined in
[the e2e plan](#standard-coverage-matrix-per-resource-coming-soon)
address the slowness and flakiness directly (per-spec cleanup, shared
fixtures, `data-testid` rollout, prod-build runs locally), so UI-default
costs less than it used to.

---

## Standard coverage matrix per resource (coming soon)

A future PR will introduce `defineCrudSpec(config)` — a generator that
emits the standard set of UI-driven tests for any admin-style resource
(list, detail, create, edit, delete, search, filter). The intent is that
adding a new resource gives you ~12 baseline tests for free, all
UI-driven, with API side-channels for audit log assertions.

When that lands, every CRUD-shaped resource should use it; bespoke specs
become the exception for chat, calling, and other non-CRUD views.

---

## Fixture lifetime — the rule that has bitten everyone

Playwright's `request` fixture is **the same APIRequestContext for the whole
test run** (`beforeEach` + body + `afterEach`). Cookies and headers persist.

**This means: do NOT call `api.login()` more than once on the same test's
`request` context.** The second call sends the `whm_access` cookie set by
the first, which trips CSRF middleware (login itself doesn't send
`X-CSRF-Token`). You'll see:

```
{"status":"error","message":"CSRF token mismatch"}
```

### Correct pattern — log in once, share via outer scope

```ts
test.describe('Webhooks', () => {
  let api: ApiHelper

  test.beforeEach(async ({ request }) => {
    api = new ApiHelper(request)
    await api.login('admin@admin.com', 'admin')
    // …seed data…
  })

  test.afterEach(async () => {
    // Reuse the same api — its csrfToken is still set.
    await cleanup(api)
  })

  test('does the thing', async () => {
    // Use `api` directly. Do NOT instantiate a new ApiHelper here
    // and call login() again.
    await api.post('/api/webhooks', { ... })
  })
})
```

### Anti-pattern — DO NOT do this

```ts
test('does the thing', async ({ request }) => {
  const api = new ApiHelper(request)
  await api.login('admin@admin.com', 'admin')   // ❌ if beforeEach already logged in
  ...
})
```

---

## Authentication

Two test users. Pick deliberately.

| User                | Source                       | Use for                                             |
| ------------------- | ---------------------------- | --------------------------------------------------- |
| `admin@admin.com` / `admin` | Created by Go migrations (`-migrate`). Always exists. | Default. API setup, seeding, anything that doesn't care which org. |
| `admin@test.com` / `password` | Created by `global-setup.ts`. Exists only after a successful global setup. | Tests that need a stable, predictable test user with role-mapped permissions. |

**`api.loginAsAdmin()` uses `admin@test.com`** — only call it if you know
global-setup ran cleanly. For most tests, prefer `api.login('admin@admin.com', 'admin')`.

The page-based `loginAsAdmin(page)` helper (in `helpers/auth.ts`) also uses
`admin@test.com`.

---

## ApiHelper conventions

```ts
const api = new ApiHelper(request)
await api.login(email, password)            // sets cookies + csrfToken

await api.get(path, extraHeaders?)          // GET (no CSRF needed)
await api.post(path, data, extraHeaders?)   // POST + X-CSRF-Token
await api.put(path, data, extraHeaders?)    // PUT + X-CSRF-Token
await api.del(path, extraHeaders?)          // DELETE + X-CSRF-Token
```

**Today, `ApiHelper.{post,put,del}` does NOT throw on non-2xx.** Tests that
don't explicitly assert the response status will see silent failures cascade
into mysterious assertion errors three lines later.

**Always** check the response or use a strict variant:

```ts
const resp = await api.post('/api/webhooks', { name: 'x' })
expect(resp.ok(), `POST failed: ${resp.status()} ${await resp.text()}`).toBe(true)
const wh = (await resp.json()).data
```

(Phase 1 of the e2e architecture plan changes the helper to throw by default;
once that lands, this section will be simplified.)

### Negative-path tests

For tests that *expect* an error response, assert the status explicitly:

```ts
const resp = await api.put('/api/settings/sso/okta', { client_id: 'x' })
expect(resp.status()).toBe(400)
```

---

## Test data

### Seed via API, not via UI

UI seeding is slow and flaky. The ApiHelper has factory methods:
`createUser`, `createContact`, `createWhatsAppAccount`, `createTemplate`,
`createOrganization`, etc. Add new ones there as resources are added.

### Use `generateUniqueEmail()` / `generateUniqueName()`

```ts
import { generateUniqueEmail, generateUniqueName } from '../../helpers'
const email = generateUniqueEmail('webhook-test')   // unique per call
const name = generateUniqueName('Hook')             // unique per call
```

**Never hard-code emails or names** unless the test specifically asserts
on a fixed value (e.g. `admin@admin.com`).

### State accumulates across runs (today)

There is **no per-test database cleanup**. Every test relies on the
uniqueness of `Date.now()` / `uuid` to avoid collisions. Tables like
`organizations`, `sso_providers`, `webhooks` accumulate rows across runs.

Implications:
- Don't write tests that assert exact row counts on shared tables.
- For tests that need a clean slate (e.g. SSO list assertions), explicitly
  delete via the API in `beforeEach`.

(Phase 2 introduces a per-test cleanup fixture.)

---

## Cross-org tests

Switching orgs persists across the test (cookies live on `request`). If you
switch into a new org, **always switch back** before the test ends —
otherwise `afterEach` cleanup runs against the wrong org.

```ts
test('cross-org isolation', async () => {
  const newOrg = await api.createOrganization(`Iso ${Date.now()}`)
  await api.switchOrg(newOrg.id)
  // …assertions in the new org…
  await api.switchOrg(originalOrgId)   // ALWAYS restore
})
```

---

## Audit-log assertions

For any test that does Create/Update/Delete on an org-scoped resource, verify
the audit trail:

```ts
import { verifyAuditLogged } from '../../helpers'

const wh = (await api.post('/api/webhooks', { ... })).data
await verifyAuditLogged(request, 'webhook', wh.id, 'created')
await verifyAuditLogged(request, 'webhook', wh.id, 'created', {
  expectedFields: ['name', 'url'],   // optional: which diff fields to require
})
```

`LogAudit` runs in a goroutine on the server, so the helper polls (3s
default). If a future handler skips the audit hook, the test fails loudly
within seconds.

There is one known gap: **`internal/handlers/contacts.go` does not call
`audit.LogAudit`** (the relevant test in `audit-trail.spec.ts` is
`test.fixme`'d). This will be fixed as part of Phase 3.

---

## Soft-delete + unique-index landmine

GORM's `gorm.DeletedAt` (in `BaseModel`) makes `db.Delete()` a soft delete
by default. **Many unique indexes in this codebase don't include
`deleted_at`**, so a soft-deleted row blocks re-creating one with the same
key.

We hit this with `sso_providers` (now fixed via `Unscoped().Delete()` in
the handler). Other suspicious indexes — verify before assuming they're
safe:

- `idx_wa_org_name` on `whatsapp_accounts (organization_id, name)`
- `idx_user_org` on `user_organizations (user_id, organization_id)`
- (Phase 4 sweeps the schema for the full list.)

If your test deletes and re-creates a resource with the same name/key,
expect a 500 from the re-create unless the handler hard-deletes.

---

## Selectors

Today the suite uses `getByText(/.../)`, role-based queries, and class-name
chains. These are brittle: text changes (i18n, copy edits) and Tailwind
restructures break them.

Preferred order:
1. `getByRole('button', { name: 'Save' })` — semantic, robust.
2. `data-testid` attributes — stable, explicit.
3. Class chains / nth-child — last resort, leave a comment.

When adding new specs, **add `data-testid` to the elements you query**
rather than relying on text. Rolling out `data-testid` across the existing
views is Phase 5; until then, follow the pattern above for new code.

---

## Page Object Model

One class per major view, extending `BasePage`. Reuse `TablePage` for
list-page operations and `DialogPage` for forms.

```ts
// pages/MyView.ts
import { BasePage } from './BasePage'
import { Locator, Page } from '@playwright/test'

export class MyViewPage extends BasePage {
  readonly addButton: Locator

  constructor(page: Page) {
    super(page)
    this.addButton = page.getByRole('button', { name: /Add/i })
  }

  async open() {
    await this.page.goto('/my-view')
    await this.page.waitForLoadState('networkidle')
  }
}
```

Add new POMs to `pages/index.ts`. Tests should never instantiate raw
`page.locator(...)` for repeated queries — encapsulate in a POM.

---

## Local vs CI

| | Local (vite dev) | CI (vite preview / built) |
| --- | --- | --- |
| `BASE_URL` | `http://localhost:3000` (dev server) | served from prod build |
| `page.goto` cold-start | seconds; `load` event hangs forever due to HMR | fast, deterministic |
| Recommended `waitUntil` | `'domcontentloaded'` (what `helpers/auth.ts` uses) | default `'load'` works |
| Browser test stability | flaky | reliable |

**The login.spec.ts test currently fails locally on the dev server.** It's
not a regression — it's environmental. Devs running e2e locally should
either:
1. Run only API-based tests (`--grep-invert "(page|UI|view|navigates)"`).
2. Or run against a production build:

```bash
cd frontend
npm run build
npx vite preview --port 3000 &
BASE_URL=http://localhost:3000 npx playwright test
```

(Phase 6 wraps this into a `npm run test:e2e:local` script.)

---

## Running

```bash
# All tests, against whatever's at BASE_URL (defaults to localhost:3000).
BASE_URL=http://localhost:3000 npx playwright test

# Single spec, serial.
BASE_URL=http://localhost:3000 npx playwright test e2e/tests/settings/webhooks.spec.ts -j 1

# Filter by test name regex.
npx playwright test --grep "audit log"

# Inverse: skip page-based tests in dev.
npx playwright test --grep-invert "page renders|navigates to|view shows"

# Show the HTML report after a run.
npx playwright show-report
```

---

## Common pitfalls — fast lookup

| Symptom | Cause | Fix |
| --- | --- | --- |
| `CSRF token mismatch` on login | Calling `api.login()` twice on the same `request` context. | Log in once in `beforeEach`, share via outer scope. |
| 500 on PUT after DELETE | Soft-delete + unique index without `deleted_at`. | Use `Unscoped().Delete()` in the handler, or change the index. |
| `Login failed` in tests using `api.loginAsAdmin()` | `admin@test.com` not created (global-setup failed). | Use `api.login('admin@admin.com', 'admin')` instead. |
| `expect(...).toBeDefined()` fails right after PUT | Silent server error; `api.put` doesn't throw on non-2xx. | Add `expect(resp.ok(), await resp.text()).toBe(true)` before reading data. |
| `page.goto` 30 s timeout on `/login` | Vite dev server cold-start; `load` event hangs. | Use prod build via `vite preview`, or `waitUntil: 'domcontentloaded'`. |
| `should record a change to "role_id"` fails on user update | Audit diff records the preloaded relation's JSON tag (`role`), not the FK column. | Don't assert on a specific field name unless verified — drop `expectedFields` or use `role`. |
| Tests pass alone, fail in suite | Cross-test state pollution (cookies, DB rows, current org). | Switch back to original org; explicitly clean in `beforeEach`. |

---

## Adding a new spec

1. **Default to UI-driven** (`{ page }` fixture). Reach for API-only only
   when there is no UI surface — see [Two test styles](#two-test-styles).
2. Use `admin@admin.com`/`admin` for setup; reach for `admin@test.com` only
   if you specifically need its role-mapped permissions. (Page-based admin
   flows that need `analytics:read` on the dashboard route will fail under
   `admin@test.com`.)
3. Generate unique names/emails — never hard-code.
4. Check API responses with `expect(resp.ok())`.
5. If the test does CRUD, call `verifyAuditLogged()` after each mutation.
6. If switching orgs, restore the original at the end of the test.
7. Add new resource helpers to `ApiHelper` rather than inlining `request.post`.
8. Use POM classes (in `pages/`) for repeated UI queries.
9. Tag genuinely-broken-but-known cases as `test.fixme` with a comment
   linking to the issue/PR. Never silently skip.
