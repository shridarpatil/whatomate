# Frontend migration to Oat UI — design

Date: 2026-11-12

## Intent

Shrink the frontend's npm supply chain. The current UI stack (Tailwind, shadcn-vue/reka-ui and their helpers, chart.js, icon packs, utility libraries) pulls in a large dependency tree that keeps producing security/vulnerability reports. The goal is the **smallest set of runtime dependencies that keeps the app working**. A new look is not the goal; adopting Oat's look is an acceptable side effect.

[Oat](https://github.com/knadh/oat) is a zero-dependency, ~10KB (minified) semantic HTML/CSS/JS UI library. It styles semantic tags directly and provides a few web components (tabs, dropdown, tooltip, toast, sidebar, tag input, upload).

## Goals

- Every screen in `frontend/src` is styled with Oat, semantic HTML and small local components. This includes the chatbot flow builder and the IVR flow editor (see below).
- Charts are hand-drawn SVG Vue components.
- Runtime `dependencies` at the end: `vue`, `vue-router`, `pinia`, `vue-i18n`, `@vue-flow/*`. Nothing else.
- All user-facing functionality keeps working, with one intended exception: dashboard widgets can no longer be resized by dragging (see Dashboard).
- Light and dark mode (via Oat's own theme), plus the existing theme switcher (light / dark / system), keep working.
- The Playwright e2e suite passes against the production binary at the end.

## Non-goals

- Changing the backend, API, routing structure, stores or i18n setup.
- Redesigning page layouts or flows beyond what is needed to drop the old components.
- Replacing build and test tooling. Vite, TypeScript, vue-tsc, Vitest, Playwright, ESLint, Prettier, happy-dom and vite-plugin-compression stay as devDependencies, since they don't ship to users.

## Anti-goals (failure even if it "works")

- Pulling in a new npm UI, chart, icon or utility package to fill a gap. Gaps are filled with native HTML or small local code.
- Leaving Tailwind, reka-ui or lucide in the tree "just for" one area at merge time.
- Changing how the vue-flow canvases behave (pan, zoom, drag, connect, minimap, controls).
- Merging a half-migrated UI to `main`.

## Packages

**Removed (runtime):** `reka-ui`, `vaul-vue`, `vee-validate`, `class-variance-authority`, `clsx`, `tailwind-merge`, `tailwindcss-animate`, `vue-sonner`, `chart.js`, `vue-chartjs`, `grid-layout-plus`, `lucide-vue-next`, `@radix-icons/vue`, `@tanstack/vue-query`, `@vueuse/core`, `@internationalized/date`, `axios`, `vue3-emoji-picker` (used by the chat composer in `views/chat/ChatView.vue`; replacement TBD, see Open questions).

**Removed (dev):** `tailwindcss`, `postcss`, `autoprefixer` (remove `postcss.config.*` / `tailwind.config.*` once nothing needs them).

**Kept:** `vue`, `vue-router`, `pinia`, `vue-i18n`, `@vue-flow/core`, `@vue-flow/background`, `@vue-flow/controls`, `@vue-flow/minimap`.

## Decisions made with the user

1. **Oat replaces only the styling/component layer.** Vue 3, vue-router, pinia and vue-i18n stay.
2. **Charts:** hand-drawn SVG components, no chart library. Used by `views/dashboard/DashboardView.vue`, `views/analytics/AgentAnalyticsView.vue` and `views/analytics/MetaInsightsView.vue` (currently Line/Bar/Pie/Doughnut via `lib/charts.ts`).
3. **Flow editors** (chatbot flow builder, IVR flow editor, shared `components/shared/FlowCanvas.vue`, the node components under `components/chatbot/nodes` and `components/calling/nodes`, plus `ChatNodeProperties.vue` and `PanelConfigEditor.vue`) **keep vue-flow** and its canvas behaviour. Their panels, node cards, toolbars and dialogs move to Oat like every other screen.
4. **Non-styling packages are removed too.** Keep only what's essential.
5. **Dashboard:** replace `grid-layout-plus` with a fixed responsive grid. Users can still reorder widgets but can't resize them by dragging. Existing saved layouts must still load; order is derived from the saved positions.
   - *Claude's call:* the grid is span-aware (CSS grid, 12 columns): each widget keeps its saved `grid_w`/`grid_h` as column/row spans, and widgets with `grid_w === 0` get the same auto-placement defaults as `computeGridLayout` today. On reorder, `saveLayout` writes only the new order/positions and **never overwrites stored `grid_w`/`grid_h`**, so the change is reversible.
6. **Rollout:** migrate page by page on **one long-lived branch**, merged to `main` once the whole migration is finished.
7. **Oat's look, no custom CSS.** Screens take Oat's default look; no attempt to match today's design (glass cards, gradients, `light:` variant, custom chat bubble colours all go). Layout uses semantic markup, Oat's components and **Oat's own utility classes** (`.flex`, `.gap-*`, `.p-*`, `.col-*`, etc.). No app stylesheets, no `<style>` blocks, no theme overrides. Note: Oat's utilities share names with some Tailwind classes, so the "no Tailwind classes" check means "no class that isn't defined by Oat or vue-flow".
   - **One layout-only sheet is allowed** (user decision): `frontend/src/assets/layout.css`, about 15 helper classes for what Oat cannot express (scroll containers, full/fixed height panes, flex grow/shrink, sticky, truncation, positioning). It holds no colours, borders, shadows, radii, fonts or other decoration; everything visual comes from Oat.
   - Other unavoidable exceptions, kept to a minimum: inline `style` bindings for genuinely dynamic values (e.g. dashboard `grid-column: span N`, chart dimensions) and SVG presentation attributes in charts that reference Oat's CSS variables (`fill="var(--primary)"`). vue-flow keeps its own shipped stylesheet.
   - **Later user decision: compact density + brand primary** in `frontend/src/assets/theme.css` (an `@layer theme-overrides` loaded after Oat, before `layout.css`). It overrides only Oat's size tokens (spacing, type scale with a fixed app heading scale and 14px body, sidebar width, grid gap), square compact icon buttons where Oat hardcodes the size, and `--primary`/`--primary-foreground`/`--ring` set to emerald. No other theme changes (other colours, borders, radii, shadows, fonts stay Oat's).

## Calls made by Claude (user can override)

- **Oat is vendored**, not installed from npm: copy `oat.min.css` / `oat.min.js` (or the `src/css` + `src/js` sources) from a pinned upstream commit into the repo (e.g. `frontend/src/vendor/oat/`), with the commit hash and the MIT licence recorded beside them. Upgrades are deliberate copies.
- **Icons:** a local `Icon.vue` holding the SVG paths for only the icons actually used, copied from Lucide (ISC licence, attribution kept in the file). Usage stays close to today's (`<Icon name="trash" />`).
- **HTTP:** `axios` (used in `services/api.ts`, `lib/api-utils.ts`, `components/layout/OrganizationSwitcher.vue`) is replaced by a thin `fetch` wrapper. It must preserve today's interceptor behaviour: base URL, auth/CSRF headers, `X-Organization-ID`, 401 handling/refresh, error shape seen by callers, JSON and multipart/file uploads, and blob downloads (exports).
- **Toasts:** `vue-sonner` (59 call sites) is replaced by Oat's toast behind a small local `toast` module with the same call shape (`toast.success/error/info/...`), so call sites barely change.
- **Gaps in Oat** are filled with native elements first (`<dialog>`, `<details>`, `<select>`, `<input type="date">`, `popover` attribute) and small local components only where needed: combobox/searchable select, date-range picker (`components/shared/DateRangePicker.vue`), command palette (if used), slider, sheet/side drawer, context menu, toggle group, resizable panes (if used).
- **Theme:** Oat's default theme is used as-is (neutral primary, system font); see decision 7. The emerald brand colour and custom fonts go. Oat drives dark mode with `color-scheme` + `light-dark()`, so the theme switcher sets `color-scheme` on `<html>` (light / dark / light dark).
- **Coexistence during migration:** Oat ships inside `@layer`, and Tailwind 3's base output is unlayered, so Tailwind's preflight would override Oat on migrated pages. While both are loaded, the builder must isolate them (e.g. scope Tailwind's preflight away from migrated subtrees, or scope Oat under a wrapper on migrated pages), and remove the scoping when Tailwind is deleted. The exact mechanism is the builder's choice.
- **Suggested order:** foundation (vendor Oat, theme, `Icon.vue`, toast module, fetch wrapper, local gap components, `AppLayout`/navigation, shared components in `components/shared`) → auth → settings pages → campaigns/templates → analytics + dashboard with SVG charts → contacts → chat → calling → flow editors → delete `components/ui`, Tailwind and the removed packages.

- **Emoji picker:** replaced by a small fixed grid of common emojis inside an Oat dropdown in the chat composer (no search/categories), plus the OS emoji keyboard as usual.
- **Keeping up with `main`:** merge `main` into the branch regularly (at least weekly) and port frontend changes into the migrated markup as part of each merge. Areas with active development on `main` (chat, contacts, calling, chatbot) are migrated late to reduce rework.
- **Browsers:** Oat's requirements are accepted as the supported range: Chrome/Edge 123+, Firefox 120+, Safari 17.5+. No polyfills beyond what Oat ships. Set Vite `build.target` accordingly.

## Constraints

- Multi-tenant UI: the org switcher and `X-Organization-ID` behaviour must survive the HTTP-layer change unchanged.
- i18n: all visible strings keep going through vue-i18n; no hardcoded English introduced.
- Accessibility semantics the e2e suite relies on must still be meaningful. The suite (`frontend/e2e`) currently targets shadcn/reka internals heavily: `button[role="combobox"]` + `[role="option"]` (selects), `[data-sonner-toast]`, `button[role="switch"]`, `[role="alertdialog"]`, `getByRole('tab')`. These selectors **will** change. Update the page objects/helpers in `e2e/pages` and `e2e/helpers` to the new markup, preferring role/label-based locators.
- E2E runs against the production binary with the embedded frontend (see CLAUDE.md), not the Vite dev server.
- The frontend is embedded in the Go binary via `internal/frontend/dist`; no backend change is expected.

## Scale (for planning)

- 342 `.vue` files, ~40k lines; 110 files import from `@/components/ui`; ~4,500 `class=` attributes using Tailwind.
- 47 shadcn component folders in `components/ui` to retire.
- `lucide-vue-next` in 112 files; `vue-sonner` in 59.

## Done when

- `npm ls --prod` shows only the kept packages and their own transitive deps.
- No Tailwind classes, `components/ui` imports, or removed-package imports remain (grep-clean).
- `npm run build`, `npm run test:unit` and `npm run lint` pass; the e2e suite passes against `make build-prod` output.
- Both flow editors behave as before on the canvas.
- Manual pass over every route in light and dark mode.
- No `<style>` blocks or app CSS files remain outside the vendored Oat files and `assets/layout.css` (vue-flow's own CSS excepted).

## Left to the builder

- Exact local component APIs, file layout under `src/`, CSS organisation.
- SVG chart internals (axes, tooltips, legends, responsiveness). They should cover what the three views show today: line, bar, pie/doughnut, with hover values.
- How reorder works on the dashboard (drag handles via native HTML5 drag-and-drop, or move up/down controls).
- Per-page commit granularity on the branch.
