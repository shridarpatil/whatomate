# Oat UI Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace Tailwind + shadcn-vue and every non-essential frontend package with vendored Oat UI, native HTML and a few small local modules, leaving runtime deps at `vue`, `vue-router`, `pinia`, `vue-i18n`, `@vue-flow/*`.

**Architecture:** Oat (CSS + ES-module JS) is vendored under `frontend/src/vendor/oat/` and loaded globally from day one, with Tailwind's preflight disabled and shadcn's CSS variables renamed so both can coexist on the `oat-ui` branch. Cross-cutting swaps (HTTP, toasts, icons, debounce) land first as drop-in local modules; then screens move to Oat one area at a time; the last task deletes Tailwind, `components/ui` and the removed packages.

**Tech Stack:** Vue 3 + TypeScript, Vite 7, Vitest 4 (node / happy-dom per file), Playwright, Oat UI (vendored, pinned commit `9e9338096548347644b5c4c5f800e5af058440fe`).

**Spec:** `docs/superpowers/specs/2026-11-12-oat-ui-migration-design.md`

## Global Constraints

- All work on branch `oat-ui`; merge to `main` only after Task 19. Merge `main` into `oat-ui` at least weekly, porting frontend changes into migrated markup.
- Runtime `dependencies` at the end: `vue`, `vue-router`, `pinia`, `vue-i18n`, `@vue-flow/core`, `@vue-flow/background`, `@vue-flow/controls`, `@vue-flow/minimap`. Never add an npm package.
- Oat's default look: no theme overrides, no `<style>` blocks, no app CSS except vendored Oat and `frontend/src/assets/layout.css` (layout-only, no colour/border/shadow/radius/font). vue-flow's own CSS is allowed.
- Static `style="..."` attributes are forbidden; `:style` only for runtime values (widget spans/heights, chart sizes, `color-scheme` on `<html>`). Chart SVG colours use Oat variables, e.g. `fill="var(--primary)"`.
- Classes in templates must be defined by vendored Oat CSS, `layout.css`, or start with `vue-flow`. JS hooks use refs or `data-*` attributes, never classes.
- Supported browsers: Chrome/Edge 123+, Firefox 120+, Safari 17.5+ (Vite `build.target: ['chrome123','edge123','firefox120','safari17.5']`).
- All visible strings go through vue-i18n; new keys are added to `en.json` and, with the English text as placeholder, to `ar/es/hi/pt/ta.json`.
- Multi-tenant: `X-Organization-ID` from `localStorage.selected_organization_id` on every API request, unchanged.
- Both flow editors keep vue-flow canvas behaviour (pan, zoom, drag, connect, minimap, controls) unchanged.
- Commit messages: one short subject line, no body.

### Shared commands

- Unit: `cd frontend && npx vitest run <file>`
- Build: `cd frontend && npm run build` (expect exit 0)
- Lint: `cd frontend && npm run lint`
- Oat check: `cd frontend && node scripts/oat-check.mjs <paths...>` (expect `oat-check: clean`, exit 0) — created in Task 1.
- E2E for an area: Postgres + Redis running and `config.toml` present (see `.github/workflows/e2e-tests.yml` for test config), then from repo root `make build-prod && (./whatomate server -config config.toml -migrate &)`, then `cd frontend && BASE_URL=http://localhost:8080 TEST_DATABASE_URL=<test db url> npx playwright test e2e/tests/<area>` (expect all passed). Stop the server afterwards.

### Shadcn → Oat mapping (used by every screen task)

| Old | New |
|---|---|
| `Button` (default / `variant="secondary"` / `destructive` / `outline` / `ghost`; `size="sm"`/`lg`/`icon`) | `<button>` / `data-variant="secondary"` / `data-variant="danger"` / `class="outline"` / `class="ghost"`; `class="small"`/`large`/`icon` (icon-only buttons need `aria-label`). `as-child` router links → `<RouterLink class="button ...">` |
| `Card*` | `<article class="card">` with `<header>`, body, `<footer>` |
| `Badge`, `TagBadge` | `<span class="badge" data-variant="success|warning|danger|secondary">` |
| `Input`, `Textarea`, `Label` | `<label data-field>Label <input> <small data-hint>…</small></label>`; errors `<div class="error" role="status">` + `aria-invalid` |
| `Select*` | native `<select>`; searchable ones → `Combobox` (Task 5) |
| `Switch`, `Checkbox`, `RadioGroup` | `<input type="checkbox" role="switch">`, `<input type="checkbox">`, `<input type="radio">` in a `<fieldset>` |
| `Dialog*`, `AlertDialog*`, `Sheet` | `Modal` (Task 5); alert variant via `alert` prop |
| `Popover*`, `DropdownMenu*` | `<ot-dropdown>` + `popovertarget`/`popover`, items `role="menuitem"` |
| `Command*` (+ `Popover`) | `Combobox` (Task 5) |
| `Tabs*` | `<ot-tabs>` with `role="tablist"/"tab"/"tabpanel"`; when the active tab is state-driven (route/query/v-model), render the same roles without `ot-tabs` and bind `aria-selected` / `hidden` |
| `Tooltip*` | `title` attribute (`data-tooltip-placement` if needed) |
| `ScrollArea` | `<div class="scroll-y">` |
| `Separator` | `<hr>` |
| `Collapsible*`, `Accordion*` | `<details><summary>` |
| `Table*` | `<div class="table"><table><thead>/<tbody>` |
| `Skeleton`, `Spinner` | `<div role="status" class="skeleton line|box">`, `aria-busy="true"` (+ `data-spinner="small|large"`) |
| `Avatar*` | `<figure data-variant="avatar"><img>` or `<abbr>` initials |
| `Alert` | `<div role="alert" data-variant="...">` |
| `Progress` | `<progress>` |
| `Breadcrumb*` | `<nav aria-label="Breadcrumb"><ol class="unstyled hstack">` |
| Tailwind layout | Oat utilities (`flex`, `flex-col`, `hstack`, `vstack`, `gap-*`, `items-center`, `justify-*`, `mb-*`/`mt-*`, `p-4`, `w-100`, `row`/`col-*`, `container`, `text-light(er)`, `align-*`) + `layout.css`; all other Tailwind classes (colours, borders, sizes, typography, `dark:`/`light:`) are deleted |
| `lucide-vue-next` | same-named components from `@/lib/icons` (Task 4); drop `class="h-4 w-4"`-style sizing, use `:size` |
| `vue-sonner` | `@/lib/toast` (Task 3) |

### Per-screen task procedure (Tasks 6–18)

Each screen task repeats these steps for its file list:

1. Rewrite each listed file per the mapping table; delete Tailwind classes; remove imports from `@/components/ui/*`.
2. Update the area's e2e page objects / helpers (`frontend/e2e/pages`, `e2e/helpers`, `e2e/framework`) for changed markup, preferring `getByRole` / `getByLabel`.
3. Run Oat check on the listed files → `oat-check: clean`.
4. Run Build → exit 0.
5. Run E2E for the listed areas → all passed.
6. Open every listed route in the browser in light and dark mode; all actions work, nothing overflows the viewport.
7. Commit.

## Review Focus

1. Many requests 401 at once (token expired while the dashboard loads ~10 widgets) → exactly one `/auth/refresh` call, every original request retried once and resolved; a failed refresh logs out once. Test: Task 2 `concurrent 401s share one refresh`.
2. Network failure or 30s timeout → rejected `HttpError` with no `response`; `getErrorMessage` returns the caller's default message, not "undefined". Test: Task 2 `timeout rejects without response` + `getErrorMessage falls back`.
3. Saved dashboard layouts mixing legacy (`grid_w === 0`) and positioned widgets, including overlaps → stable order, legacy placed after positioned, reorder never changes any `grid_w`/`grid_h`. Test: Task 14 `orderWidgets` + `reorder preserves sizes`.
4. Charts given empty series, one point, all-zero values or a single pie slice → valid SVG with no `NaN` in any attribute. Test: Task 12 `degenerate inputs produce no NaN`.
5. Modal closed by Escape or backdrop click → `update:open` emitted with `false`, so the parent can reopen it. Test: Task 5 `Escape syncs open state`.

---

### Task 1: Foundation — vendored Oat, coexistence, layout sheet, theme, checker

**Files:**
- Create: `frontend/src/vendor/oat/css/*.css` (all 24 files from upstream `src/css/` at the pinned commit), `frontend/src/vendor/oat/js/*.js` (all files from upstream `src/js/`), `frontend/src/vendor/oat/LICENSE`, `frontend/src/vendor/oat/VERSION` (commit hash + date)
- Create: `frontend/src/vendor/oat/oat.css` — `@import` lines for the CSS files in the order of upstream `Makefile` `CSS_FILES`
- Create: `frontend/src/assets/layout.css`
- Create: `frontend/scripts/oat-check.mjs`, `frontend/scripts/oat-check.spec.ts`
- Create: `frontend/src/composables/useColorMode.spec.ts`
- Modify: `frontend/src/main.ts`, `frontend/vite.config.ts`, `frontend/vitest.config.ts`, `frontend/tailwind.config.cjs`, `frontend/src/assets/index.css`, `frontend/src/composables/useColorMode.ts`, `frontend/src/components/shared/FlowCanvas.vue`, the 7 files importing `useDebounceFn` from `@vueuse/core` (`composables/useSearchPagination.ts`, `views/chatbot/AIContextsView.vue`, `views/chatbot/ChatbotFlowsView.vue`, `views/settings/CustomActionsView.vue`, `views/settings/TeamDetailView.vue`, `views/settings/TeamsView.vue`, `views/settings/WebhooksView.vue`), `frontend/package.json`

**Interfaces:**
- Produces: global Oat CSS + `window.ot` (from `vendor/oat/js/index.js`); `layout.css` classes `scroll-y scroll-x overflow-hidden h-full h-screen min-h-0 min-w-0 flex-1 shrink-0 flex-wrap sticky-top relative absolute-fill truncate`; `resolveScheme(mode: ColorMode, systemDark: boolean): 'light' | 'dark'` exported from `useColorMode.ts`; CLI `node scripts/oat-check.mjs <paths...>`; `findViolations(file: string, source: string, allowed: Set<string>): string[]` exported from `oat-check.mjs`.

- [ ] **Step 1: Write failing tests**

`scripts/oat-check.spec.ts` (add `'scripts/**/*.spec.ts'` to `vitest.config.ts` `include`):
- `flags unknown static class`: `findViolations('a.vue', '<div class="flex bg-red-500">', new Set(['flex']))` → one entry containing `bg-red-500`.
- `allows vue-flow prefix`: `class="vue-flow__node"` with empty set → `[]`.
- `checks string literals in :class`: `:class="ok ? 'badge' : 'text-muted-foreground'"` with `{'badge'}` → one entry containing `text-muted-foreground`.
- `flags removed imports`: source with `from '@/components/ui/button'`, `from 'lucide-vue-next'`, `from 'vue-sonner'`, `from 'axios'`, `from '@vueuse/core'`, `from 'reka-ui'` → six entries.
- `flags style blocks and static style attrs`: `<style scoped>` and `style="color:red"` → two entries; `:style="{ height: h + 'px' }"` → none.

`composables/useColorMode.spec.ts`:
- `resolveScheme('system', true) === 'dark'`, `resolveScheme('system', false) === 'light'`, `resolveScheme('light', true) === 'light'`, `resolveScheme('dark', false) === 'dark'`.

- [ ] **Step 2: Run tests, verify they fail**

Run: `cd frontend && npx vitest run scripts/oat-check.spec.ts src/composables/useColorMode.spec.ts`
Expected: FAIL (modules / exports missing)

- [ ] **Step 3: Implement**

- `oat-check.mjs`: allowed set = every `.name` class selector parsed from `src/vendor/oat/css/*.css` and `src/assets/layout.css`; scans `.vue`/`.ts` files under the given paths; checks static `class="..."` tokens, quoted string tokens inside `:class="..."`, removed-package imports (`@/components/ui/`, `lucide-vue-next`, `vue-sonner`, `axios`, `@vueuse/core`, `reka-ui`, `vaul-vue`, `vee-validate`, `class-variance-authority`, `clsx`, `tailwind-merge`, `chart.js`, `vue-chartjs`, `grid-layout-plus`, `@radix-icons/vue`, `@tanstack/vue-query`, `@internationalized/date`, `vue3-emoji-picker`), `<style` blocks and static `style="`. Prints `file: reason` per violation and exits 1, else prints `oat-check: clean`.
- `layout.css` (inside `@layer utilities`, exactly these rules): `.scroll-y{overflow-y:auto;min-height:0}` `.scroll-x{overflow-x:auto}` `.overflow-hidden{overflow:hidden}` `.h-full{height:100%}` `.h-screen{height:100dvh}` `.min-h-0{min-height:0}` `.min-w-0{min-width:0}` `.flex-1{flex:1 1 0%}` `.shrink-0{flex-shrink:0}` `.flex-wrap{flex-wrap:wrap}` `.sticky-top{position:sticky;top:0;z-index:1}` `.relative{position:relative}` `.absolute-fill{position:absolute;inset:0}` `.truncate{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}`.
- Coexistence: `tailwind.config.cjs` `corePlugins: { preflight: false }`; rename every shadcn colour variable (`--background --foreground --card --card-foreground --popover --popover-foreground --primary --primary-foreground --secondary --secondary-foreground --muted --muted-foreground --accent --accent-foreground --destructive --destructive-foreground --border --input --ring --radius` and any `--chart-*`/`--sidebar-*`) to an `--sh-` prefix in both `index.css` (definitions and uses) and `tailwind.config.cjs`, so Oat's variables of the same names win.
- `main.ts`: import `./vendor/oat/oat.css`, `./vendor/oat/js/index.js`, `./assets/layout.css` before `./assets/index.css`; remove `VueQueryPlugin` and `@tanstack/vue-query`.
- `vite.config.ts`: `vue({ template: { compilerOptions: { isCustomElement: tag => tag.startsWith('ot-') } } })`; `build.target` from Global Constraints.
- `useColorMode.ts`: add `resolveScheme`; `updateTheme` also sets `document.documentElement.style.colorScheme` to the resolved scheme (keep the existing `light`/`dark` class toggling until Task 19).
- `FlowCanvas.vue`: move the four vue-flow CSS `@import`s from `<style>` to script `import`s; delete the `<style>` block; if any non-import rule there sized the canvas, replace it with `layout.css` classes in the template.
- Replace `useDebounceFn` with `debounce` from `@/lib/utils` in the 7 files.
- `package.json`: remove `@tanstack/vue-query`.

- [ ] **Step 4: Run tests, verify they pass**

Run: `cd frontend && npx vitest run scripts/oat-check.spec.ts src/composables/useColorMode.spec.ts`
Expected: PASS

- [ ] **Step 5: Build and smoke**

Run Build → exit 0. Run `node scripts/oat-check.mjs src/assets` → clean. Start `npm run dev`, log in, toggle the theme: app still usable (Oat base styles visible on unstyled elements is expected), both flow editors still render the canvas.

- [ ] **Step 6: Commit**

```bash
git add -A frontend && git commit -m "feat(ui): vendor oat and add coexistence foundation"
```

---

### Task 2: HTTP client replacing axios

**Files:**
- Create: `frontend/src/lib/http.ts`, `frontend/src/lib/http.spec.ts`, `frontend/src/lib/api-utils.spec.ts`
- Modify: `frontend/src/services/api.ts:1-150` (client + interceptors; service methods unchanged), `frontend/src/lib/api-utils.ts`, `frontend/src/components/layout/OrganizationSwitcher.vue` (comment mentions axios only — update wording), `frontend/package.json` (remove `axios`)

**Interfaces:**
- Produces (`lib/http.ts`):
  - `interface HttpConfig { params?: Record<string, unknown>; headers?: Record<string, string>; responseType?: 'json' | 'blob' | 'arraybuffer' | 'text'; timeout?: number; signal?: AbortSignal }`
  - `interface HttpResponse<T = any> { data: T; status: number; headers: Headers }`
  - `class HttpError<T = any> extends Error { response?: { status: number; data: T }; config: RequestConfig; isHttpError: true }`
  - `interface RequestConfig extends HttpConfig { method: string; url: string; data?: unknown; _retry?: boolean }`
  - `interface HttpClient { request<T>(c: RequestConfig): Promise<HttpResponse<T>>; get<T>(url, c?); delete<T>(url, c?); post<T>(url, data?, c?); put<T>(url, data?, c?); patch<T>(url, data?, c?); baseURL: string }`
  - `createHttpClient(opts: { baseURL: string; timeout: number; onRequest?: (c: RequestConfig) => void; onError?: (e: HttpError, retry: (c: RequestConfig) => Promise<HttpResponse>) => Promise<HttpResponse> }): HttpClient`
  - `isHttpError(e: unknown): e is HttpError`
- `services/api.ts` keeps exporting `api` (now `HttpClient`) and `getRequestHeaders`; `api.defaults.baseURL` call site becomes `api.baseURL`.
- `api-utils.ts`: `AxiosResponse` → `HttpResponse`, `isAxiosError` → `isHttpError`; exported function names unchanged.

- [ ] **Step 1: Write failing tests** (`lib/http.spec.ts`, stub `globalThis.fetch` with `vi.fn`)

- `get serializes params`: `get('/x', { params: { page: 2, search: 'a b', empty: undefined } })` → fetch URL `${base}/x?page=2&search=a+b` (undefined/null dropped).
- `json body and content-type`: `post('/x', { a: 1 })` → body `'{"a":1}'`, header `Content-Type: application/json`, `credentials: 'include'`.
- `FormData passes through`: `post('/x', fd)` → body is the same `FormData`, no `Content-Type` header set.
- `responseType blob/arraybuffer`: resolves `data` as `Blob` / `ArrayBuffer`.
- `non-2xx rejects HttpError`: 422 with `{message:'bad'}` → rejects, `e.response.status === 422`, `e.response.data.message === 'bad'`, `isHttpError(e) === true`.
- `timeout rejects without response`: fetch never resolves, `timeout: 10`, fake timers → rejects `HttpError` with `response === undefined`.
- `onRequest can add headers` (used for org + CSRF).

In `services/api.ts`-level tests (same file, import `api` after setting `localStorage`/`document.cookie` in a happy-dom environment, `// @vitest-environment happy-dom`):
- `adds org header always and CSRF only on mutating`: `selected_organization_id='org-1'`, cookie `whm_csrf=tok` → GET has `X-Organization-ID: org-1` and no `X-CSRF-Token`; POST/PUT/PATCH/DELETE have both.
- `401 refreshes then retries once`: first call 401, `/auth/refresh` 200, retry 200 → resolves retry data; fetch calls = 3.
- `concurrent 401s share one refresh`: 5 parallel GETs all 401 then 200 → exactly one `/auth/refresh` call, all 5 resolve.
- `auth endpoints never refresh`: `post('/auth/login')` 401 → rejects, no refresh call.
- `failed refresh logs out`: refresh 401 → localStorage `user`/`auth_token`/`refresh_token` cleared, `window.location.href` ends with `/login`.

`lib/api-utils.spec.ts`:
- `getErrorMessage reads response message`: HttpError with `response.data.message='x'` → `'x'`.
- `getErrorMessage falls back`: HttpError without response, default `'D'` → `'D'`.

- [ ] **Step 2: Run tests, verify they fail**

Run: `cd frontend && npx vitest run src/lib/http.spec.ts src/lib/api-utils.spec.ts`
Expected: FAIL (module missing)

- [ ] **Step 3: Implement `createHttpClient` in `lib/http.ts`; rewire `services/api.ts`**

Timeout via `AbortController` (combine with caller `signal`). JSON responses parsed when `content-type` includes `json`, else text; empty body → `null`. In `api.ts`, port the existing interceptor logic verbatim into `onRequest` / `onError` (refresh mutex, `navigator.locks` serialisation, logout redirect using `basePath`); `refreshAccessToken` uses raw `fetch` POST with `credentials: 'include'`. Keep all exported service objects and method signatures unchanged.

- [ ] **Step 4: Run tests, verify they pass**

Run: `cd frontend && npx vitest run src/lib/http.spec.ts src/lib/api-utils.spec.ts`
Expected: PASS

- [ ] **Step 5: Verify whole app**

Run `grep -rn "axios" frontend/src` → no matches. Run Build → exit 0. Run E2E for `auth settings campaigns` → all passed (covers CRUD, uploads, exports).

- [ ] **Step 6: Commit**

```bash
git add -A frontend && git commit -m "feat(ui): replace axios with fetch client"
```

---

### Task 3: Toasts on Oat

**Files:**
- Create: `frontend/src/lib/toast.ts`, `frontend/src/lib/toast.spec.ts`
- Modify: all 59 files importing from `vue-sonner` (import path only), `frontend/src/App.vue` (remove `Toaster`), `frontend/e2e/pages/BasePage.ts`, `frontend/e2e/helpers/detail-page.ts`, `frontend/e2e/framework/crud.ts`, `frontend/package.json` (remove `vue-sonner`)

**Interfaces:**
- Produces: `toast(message: string, opts?: ToastOptions)` with `toast.success/error/info/warning(message, opts?)`; `interface ToastOptions { description?: string; duration?: number }`. Maps to `window.ot.toast(text, title, { variant, duration, placement: 'top-right' })`: with `description`, title = `message`, text = `description`; without, text = `message`, no title. Variants: success→`success`, error→`danger`, warning→`warning`, info/plain→`info`.
- E2E toast locator everywhere: `.toast-container output` (Oat renders each toast as `<output class="toast">`).

- [ ] **Step 1: Write failing tests** (`toast.spec.ts`, stub `window.ot.toast` with `vi.fn`)

- `error maps to danger`: `toast.error('Failed')` → `ot.toast('Failed', undefined, { variant: 'danger', placement: 'top-right' })`.
- `description becomes body`: `toast.success('Saved', { description: 'All good', duration: 2000 })` → `ot.toast('All good', 'Saved', { variant: 'success', duration: 2000, placement: 'top-right' })`.
- `plain toast is info`.

- [ ] **Step 2: Run, verify fail** — `npx vitest run src/lib/toast.spec.ts` → FAIL
- [ ] **Step 3: Implement `lib/toast.ts`; replace `from 'vue-sonner'` with `from '@/lib/toast'` in all files; remove `<Toaster>`; replace `[data-sonner-toast]` with `.toast-container output` in the three e2e files**
- [ ] **Step 4: Run, verify pass** — `npx vitest run src/lib/toast.spec.ts` → PASS; `grep -rn "vue-sonner\|data-sonner" frontend/src frontend/e2e` → no matches
- [ ] **Step 5: Build + E2E for `settings`** → passed
- [ ] **Step 6: Commit** — `git commit -m "feat(ui): use oat toasts"`

---

### Task 4: Local icons

**Files:**
- Create: `frontend/src/lib/icons.ts`, `frontend/src/lib/icon-data.ts` (generated once), `frontend/src/lib/icons.spec.ts`
- Modify: all files importing from `lucide-vue-next` (import path only), `frontend/package.json` (remove `lucide-vue-next`; `@radix-icons/vue` is removed in Task 19 with `components/ui`)

**Interfaces:**
- Produces: `icon-data.ts`: `export const iconData: Record<string, Array<[tag: string, attrs: Record<string, string>]>>` keyed by Lucide PascalCase name. `icons.ts`: one named export per used icon, same names as today (`Trash2`, `Plus`, … — including any `*Icon` aliases used), each a functional component rendering `<svg xmlns viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" :width="size" :height="size" aria-hidden="true">` with props `size?: number | string` (default `16`), `strokeWidth?: number | string`; extra attrs (class, aria-label) fall through. File header records Lucide v0.303.0 ISC licence and that new icons are copied by hand from lucide.dev.

- [ ] **Step 1: Write failing tests** (`icons.spec.ts`, happy-dom, render with `createApp(...).mount(div)`)
- `renders paths`: `Trash2` → `<svg>` with 5 children, width `16`.
- `size prop`: `size: 24` → width/height `24`.
- `every icon has data`: every export of `icons.ts` maps to a non-empty `iconData` entry.
- [ ] **Step 2: Run, verify fail**
- [ ] **Step 3: Generate `icon-data.ts`** with a throwaway script in the scratchpad (not committed): collect every name imported from `lucide-vue-next` under `frontend/src`, read `node_modules/lucide-vue-next/dist/esm/icons/<kebab>.js`, drop `key` attrs. **Implement `icons.ts`; replace `from 'lucide-vue-next'` with `from '@/lib/icons'`.**
- [ ] **Step 4: Run, verify pass**; `grep -rn "lucide-vue-next" frontend/src` → none
- [ ] **Step 5: Build** → exit 0; spot-check sidebar and a table page render icons
- [ ] **Step 6: Commit** — `git commit -m "feat(ui): replace lucide with local icons"`

---

### Task 5: Gap components — Modal, Combobox, DateRangePicker

**Files:**
- Create: `frontend/src/components/oat/Modal.vue`, `frontend/src/components/oat/Combobox.vue`, `frontend/src/components/oat/combobox.ts`, `frontend/src/components/oat/Modal.spec.ts`, `frontend/src/components/oat/combobox.spec.ts`
- Modify: `frontend/src/components/shared/DateRangePicker.vue` (keep its current props/emits), `frontend/e2e/pages/DialogPage.ts`, `frontend/e2e/helpers/detail-page.ts`, `frontend/e2e/pages/BasePage.ts`

**Interfaces:**
- `Modal.vue`: props `open: boolean`, `title?: string`, `description?: string`, `alert?: boolean` (sets `role="alertdialog"`), `size?: 'small' | 'large'`; emits `update:open`; slots `default`, `footer`. Renders native `<dialog>` (`showModal()` when `open` becomes true, `close()` when false; `close`/`cancel` events and backdrop click emit `update:open(false)`; backdrop click ignored when `alert`).
- `combobox.ts`: `interface ComboOption { value: string; label: string; hint?: string }`; `filterOptions(options: ComboOption[], query: string): ComboOption[]` (case-insensitive substring on label and hint; empty query returns all).
- `Combobox.vue`: props `modelValue: string | null`, `options: ComboOption[]`, `placeholder?: string`, `searchPlaceholder?: string`, `emptyText?: string`, `disabled?: boolean`, `id?: string`; emits `update:modelValue`. Trigger is a `<button>` with `aria-haspopup="listbox"`; inside `<ot-dropdown>` popover: search `<input>` + `role="listbox"` list of `role="option"` buttons; arrow keys/Enter/Escape work.
- E2E helpers: `DialogPage.fillSelect(label, option)` handles native `<select>` (`selectOption`) and Combobox (click trigger, click `getByRole('option', { name })`), keeping the legacy `button[role="combobox"]` path until Task 19. Alert dialog locator: `page.getByRole('alertdialog')`.

- [ ] **Step 1: Write failing tests**
  - `combobox.spec.ts`: `filterOptions(opts, 'ali')` returns `Alice` and `Natalie` (hint), `filterOptions(opts, '')` returns all, `filterOptions(opts, 'ALI')` same as lowercase.
  - `Modal.spec.ts` (happy-dom; polyfill `HTMLDialogElement.prototype.showModal/close` if missing): `opens when open=true` (dialog `open` attr set), `Escape syncs open state` (dispatch `cancel` → emitted `update:open` `[false]`), `alert ignores backdrop click`.
- [ ] **Step 2: Run, verify fail**
- [ ] **Step 3: Implement the three components and the e2e helper changes**
- [ ] **Step 4: Run, verify pass**
- [ ] **Step 5: Oat check on `src/components/oat src/components/shared/DateRangePicker.vue` → clean; Build; E2E `analytics`** (uses DateRangePicker) → passed
- [ ] **Step 6: Commit** — `git commit -m "feat(ui): add modal, combobox and native date range picker"`

---

### Task 6: App shell

**Files:** `frontend/src/App.vue`, `components/layout/AppLayout.vue`, `components/layout/UserMenu.vue`, `components/layout/OrganizationSwitcher.vue`, `components/layout/ThemeSwitcher.vue`, `components/LanguageSwitcher.vue`, `views/NotFoundView.vue`

**Interfaces:** Consumes Modal/Combobox (Task 5), icons (Task 4). Layout uses Oat sidebar: `<div data-sidebar-layout>` + `<header data-topnav>` + `<aside data-sidebar>` + `<main class="scroll-y">`; nav items from `navigation.ts` unchanged; mobile toggle `[data-sidebar-toggle]`.

- [ ] Steps 1–7 of the per-screen procedure. E2E areas: `auth profile`. Additional check: sidebar collapses on a 375px-wide viewport and every permitted nav item is reachable.

---

### Task 7: Shared components

**Files:** every file in `frontend/src/components/shared/` except `FlowCanvas.vue` and `DateRangePicker.vue`; `components/roles/PermissionMatrix.vue`

**Interfaces:** Public props/emits/slots of each shared component stay unchanged (35+ call sites depend on them). `ConfirmDialog`/`DeleteConfirmDialog`/`UnsavedChangesDialog` use `Modal` with `alert`; `CrudFormDialog`/`CreateContactDialog`/`ImportExportDialog` use `Modal`; `DataTable` renders `<div class="table"><table>`; `PaginationControls` uses Oat pagination markup; `IconButton` = `<button class="ghost icon" :title :aria-label>`.

- [ ] Steps 1–7. E2E areas: `settings` (exercises DataTable, CRUD dialogs, delete confirm, pagination, search).

---

### Task 8: Auth and profile

**Files:** `views/auth/LoginView.vue`, `views/auth/RegisterView.vue`, `views/auth/SSOCallbackView.vue`, `views/profile/ProfileView.vue`

- [ ] Steps 1–7. E2E areas: `auth profile i18n`.

---

### Task 9: Settings A — accounts, organisation settings

**Files:** `views/settings/SettingsView.vue`, `AccountsView.vue`, `AccountDetailView.vue`, `BusinessProfileDialog.vue`, `SSOSettingsView.vue`, `ChatbotSettingsView.vue`, `CustomActionsView.vue`; `lib/utils.ts` (`getQualityBadgeClass`, `getVerificationBadgeClass` → return an Oat badge `data-variant` value `'success' | 'warning' | 'danger' | 'secondary'`, renamed `getQualityBadgeVariant` / `getVerificationBadgeVariant`; `getAvatarGradient` deleted and its callers use plain Oat avatars)

- [ ] Steps 1–7 (update every caller of the renamed helpers; `grep -rn "BadgeClass\|getAvatarGradient" frontend/src` → none). E2E areas: `settings`.

---

### Task 10: Settings B — users, roles, teams, keys, webhooks, tags, audit, canned responses

**Files:** `views/settings/UsersView.vue`, `UserDetailView.vue`, `RolesView.vue`, `RoleDetailView.vue`, `TeamsView.vue`, `TeamDetailView.vue`, `APIKeysView.vue`, `APIKeyDetailView.vue`, `WebhooksView.vue`, `WebhookDetailView.vue`, `TagsView.vue`, `AuditLogsView.vue`, `AuditLogDetailView.vue`, `CannedResponsesView.vue`, `CannedResponseDetailView.vue`

- [ ] Steps 1–7. E2E areas: `settings`.

---

### Task 11: Campaigns, templates, WhatsApp Flows

**Files:** `views/settings/CampaignsView.vue`, `CampaignDetailView.vue`, `TemplatesView.vue`, `TemplateDetailView.vue`, `FlowsView.vue`, `components/flow-builder/FlowBuilder.vue`

- [ ] Steps 1–7. E2E areas: `campaigns flows`.

---

### Task 12: SVG chart components

**Files:**
- Create: `frontend/src/lib/chart-math.ts`, `frontend/src/lib/chart-math.spec.ts`, `frontend/src/components/charts/LineChart.vue`, `BarChart.vue`, `PieChart.vue`, `ChartTooltip.vue`

**Interfaces:**
- `chart-math.ts`:
  - `niceTicks(min: number, max: number, count?: number /* default 5 */): number[]` — ascending, includes a tick ≤ min and ≥ max, steps of 1/2/5×10ⁿ; `min === max` widens to `[0, max || 1]`.
  - `linearScale(domain: [number, number], range: [number, number]): (v: number) => number` — zero-width domain maps everything to range midpoint.
  - `linePath(points: Array<[x: number, y: number]>): string` — `M…L…`; `''` for no points; single point → `M x y` only.
  - `pieArcs(values: number[], radius: number, innerRadius?: number): Array<{ path: string; start: number; end: number; value: number }>` — zero total → `[]`; a single non-zero value draws a full ring (two half arcs).
- `ChartSeries = { label: string; data: number[]; color?: string }`. `LineChart` / `BarChart` props: `labels: string[]`, `series: ChartSeries[]`, `height?: number` (default 240), `stacked?: boolean` (Bar only), `fill?: boolean` (Line only), `formatValue?: (v: number) => string`. `PieChart` props: `labels: string[]`, `values: number[]`, `donut?: boolean`, `formatValue?`. All are responsive (`viewBox` + `width="100%"`), show a legend when >1 series/slice, and show `ChartTooltip` with label + values on hover/focus of a point/bar/slice. Default palette cycles `var(--primary)`, `var(--success)`, `var(--warning)`, `var(--danger)`, `var(--muted-foreground)`; axes/gridlines use `var(--border)` and labels `var(--muted-foreground)`.

- [ ] **Step 1: Write failing tests** (`chart-math.spec.ts`)
  - `niceTicks(0, 87)` → `[0, 20, 40, 60, 80, 100]`; `niceTicks(3, 3)` → starts at `0`, ends ≥ `3`.
  - `linearScale([0,100],[200,0])(25)` → `150`; zero-width domain → midpoint.
  - `pieArcs([1,1,2], 50)` → 3 arcs, last `end` ≈ `2π`.
  - `degenerate inputs produce no NaN`: `linePath([])`, `linePath([[0,0]])`, `pieArcs([0,0],50)`, `pieArcs([5],50)`, `niceTicks(0,0)`, ticks/scale for all-zero data — no output string contains `NaN`, no number is `NaN`.
- [ ] **Step 2: Run, verify fail**
- [ ] **Step 3: Implement `chart-math.ts` and the four components**
- [ ] **Step 4: Run, verify pass**
- [ ] **Step 5: Oat check `src/components/charts` → clean; Build**
- [ ] **Step 6: Commit** — `git commit -m "feat(ui): add svg chart components"`

---

### Task 13: Analytics views

**Files:** `views/analytics/AgentAnalyticsView.vue`, `views/analytics/MetaInsightsView.vue`

**Interfaces:** Consumes Task 12 charts. Convert each `computed` chart.js data object to `{ labels, series }` (or `{ labels, values }` for the doughnut, `donut: true`); delete the chart.js `*ChartOptions` objects, keeping any value formatting as `formatValue`. Command/Popover filters → `Combobox`.

- [ ] Steps 1–7. E2E areas: `analytics`. Additional check: every chart from today renders with the same data (compare against `main` with the same date range).

---

### Task 14: Dashboard — fixed grid with reordering

**Files:**
- Create: `frontend/src/lib/dashboard-layout.ts`, `frontend/src/lib/dashboard-layout.spec.ts`
- Modify: `frontend/src/views/dashboard/DashboardView.vue` (remove `grid-layout-plus`, `computeGridLayout`, `<style>` block), `frontend/package.json` (remove `grid-layout-plus`)

**Interfaces:**
- `interface Placed { id: string; x: number; y: number; w: number; h: number }`
- `placeWidgets(widgets: DashboardWidget[]): Placed[]` — moved verbatim from `computeGridLayout` (positioned as-is; legacy `grid_w === 0` auto-placed below with number 3×3, chart 6×5, table/shortcuts 6×8 on 12 columns).
- `orderWidgets(widgets: DashboardWidget[]): Placed[]` — `placeWidgets` sorted by `y`, then `x`, then original index.
- `reorder(order: Placed[], from: number, to: number): Placed[]` — moves one item, then re-flows `x`/`y` left-to-right on 12 columns (new row when `x + w > 12`; row height = tallest item in the row); never changes `w`/`h`.
- Rendering: `<div class="row">`, each widget `class="col-{w}"` (clamped 1–12) with `:style="{ minHeight: h * 40 + 'px' }"`; on screens <768px every widget is `col-12` (use the existing breakpoint composable or `matchMedia`). In edit mode each widget shows "Move earlier" / "Move later" buttons (`aria-label`, new i18n keys `dashboard.moveEarlier`, `dashboard.moveLater`); each move calls `reorder` and the existing debounced `persistLayout` with the re-flowed `x`/`y` and unchanged `w`/`h`.

- [ ] **Step 1: Write failing tests** (`dashboard-layout.spec.ts`)
  - `orderWidgets` on `[B(x6,y0,w6,h5), A(x0,y0,w6,h5), L(legacy number)]` → ids `A, B, L`.
  - `overlapping saved positions keep stable order`: two widgets at the same `x,y` → original array order.
  - `reorder preserves sizes`: after `reorder(order, 2, 0)` every item's `w`/`h` equals its input; `x`/`y` form a valid flow (no item exceeds 12 columns).
  - `reorder flows to new row`: widths `[6,6,6]` → y values `[0,0,5]` when heights are 5.
- [ ] **Step 2: Run, verify fail**
- [ ] **Step 3: Implement `dashboard-layout.ts`; migrate `DashboardView.vue` (charts via Task 12, rest per mapping table)**
- [ ] **Step 4: Run, verify pass**
- [ ] **Step 5: Oat check `src/views/dashboard src/lib/dashboard-layout.ts` → clean; Build; E2E `dashboard` → passed.** Manual: a saved layout from `main` loads in the same order; moving a widget and reloading keeps the new order; widget sizes in the DB are unchanged (`grid_w`/`grid_h` before vs after).
- [ ] **Step 6: Commit** — `git commit -m "feat(ui): oat dashboard with reorderable grid"`

---

### Task 15: Contacts and chatbot (non-editor) screens

**Files:** `views/settings/ContactsView.vue`, `views/settings/ContactDetailView.vue`, `views/chatbot/ChatbotView.vue`, `KeywordsView.vue`, `KeywordDetailView.vue`, `AIContextsView.vue`, `AIContextDetailView.vue`, `AgentTransfersView.vue`, `ChatbotFlowsView.vue`

- [ ] Steps 1–7. E2E areas: `chatbot settings` (contacts specs live under settings).

---

### Task 16: Chat

**Files:** `views/chat/ChatView.vue` (delete `<style>` block), `components/chat/CannedResponsePicker.vue`, `ContactInfoPanel.vue`, `ConversationNotes.vue`, `MediaViewerDialog.vue`, `MetadataSection.vue`, `TemplatePicker.vue`; create `frontend/src/lib/emojis.ts`; `frontend/package.json` (remove `vue3-emoji-picker`)

**Interfaces:**
- Layout: `<div class="flex h-full min-h-0">` with conversation list, thread and contact panel each a `flex-col` column; message list `class="flex-1 scroll-y"`, composer `shrink-0`. Own messages right-aligned (`justify-end`), others left; bubbles are `article.card`.
- `emojis.ts`: `export const COMMON_EMOJIS: readonly string[]` — exactly 48 common emojis (smileys, gestures, hearts, objects). Composer emoji button opens an `<ot-dropdown>` grid of `<button class="ghost small">` that calls the existing `insertEmoji`.
- `MediaViewerDialog.vue`: replace `onKeyStroke` with a `keydown` listener added in `onMounted` / removed in `onUnmounted` (same keys).

- [ ] Steps 1–7. E2E areas: `chat`. Additional checks: list, thread and panel scroll independently; composer stays pinned at the bottom; new messages auto-scroll; emoji insertion works; works at 375px width.

---

### Task 17: Calling (non-editor)

**Files:** `views/calling/CallLogsView.vue`, `CallTransfersView.vue`, `IVRFlowsView.vue`, `components/calling/ActiveCallPanel.vue`, `CallButton.vue`, `CallTransferPicker.vue`, `IVRPathTree.vue`

- [ ] Steps 1–7. E2E areas: `calling`.

---

### Task 18: Flow editors (vue-flow kept)

**Files:** `views/chatbot/ChatbotFlowBuilderView.vue`, `views/calling/IVRFlowEditorView.vue`, `components/shared/FlowCanvas.vue`, `components/chatbot/ChatNodeProperties.vue`, `PanelConfigEditor.vue`, `components/chatbot/nodes/*.vue`, `components/chatbot/flow-preview/*.vue` (delete `<style>` blocks in `InteractivePreview.vue`, `PreviewListPicker.vue`), `components/calling/IVRNodeProperties.vue`, `components/calling/nodes/*.vue`

**Interfaces:** vue-flow props, events, node types, handles and `FlowCanvas` API unchanged. Editor layout: `<div class="flex h-full min-h-0">` with canvas `flex-1 relative` and properties panel `shrink-0 scroll-y`. Nodes render as `<article class="card">` with icon + title header; handles stay vue-flow's.

- [ ] Steps 1–7. E2E areas: `chatbot calling`. Additional checks on both editors: add, drag, connect, delete nodes; pan/zoom; minimap and controls; edit properties; save and reload; chatbot preview panel runs a flow.

---

### Task 19: Remove the old stack and verify

**Files:**
- Delete: `frontend/src/components/ui/`, `frontend/src/lib/charts.ts`, `frontend/tailwind.config.cjs`, `frontend/postcss.config.cjs`, `frontend/src/assets/index.css` (move nothing; `fonts.css` and its font files are deleted too unless a vendored file needs them)
- Modify: `frontend/src/main.ts`, `frontend/src/lib/utils.ts` (remove `cn`), `frontend/src/composables/useColorMode.ts` (drop `light`/`dark` class toggling), `frontend/package.json` + `package-lock.json`, e2e helpers (drop legacy `button[role="combobox"]` branches), `CLAUDE.md` (frontend line: "Vue 3 + Oat UI (vendored in `frontend/src/vendor/oat`)"; note the no-custom-CSS rule and `scripts/oat-check.mjs`)
- Remove packages: `reka-ui`, `vaul-vue`, `vee-validate`, `class-variance-authority`, `clsx`, `tailwind-merge`, `tailwindcss-animate`, `chart.js`, `vue-chartjs`, `@radix-icons/vue`, `@vueuse/core`, `@internationalized/date`, `tailwindcss`, `postcss`, `autoprefixer`, plus any still-listed package from earlier tasks.

- [ ] **Step 1: Delete and edit as listed; `npm install` to refresh the lockfile**
- [ ] **Step 2: `cd frontend && npm ls --prod --depth=0`** → exactly `vue`, `vue-router`, `pinia`, `vue-i18n`, `@vue-flow/background`, `@vue-flow/controls`, `@vue-flow/core`, `@vue-flow/minimap`
- [ ] **Step 3: `node scripts/oat-check.mjs src`** → `oat-check: clean`
- [ ] **Step 4: `npm run test:unit && npm run lint && npm run build`** → all exit 0
- [ ] **Step 5: Full E2E** (`npx playwright test` against `make build-prod` binary) → all passed
- [ ] **Step 6: Manual pass over every route in light and dark mode**
- [ ] **Step 7: Commit** — `git commit -m "chore(ui): remove tailwind, shadcn and unused packages"`
