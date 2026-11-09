# Whatomate MCP Connector — Design

Date: 09/11/2026 · Status: approved by owner ("you need to build it", opted out of questions)

## Intent

Let Claude (claude.ai custom connector, Claude Desktop, Claude Code) set up and use Whatomate
the way a person does from the web UI: WhatsApp accounts, templates, contacts, chats, campaigns,
chatbot flows, RSVP events, lead ads, catalogs, users/roles/teams, settings, analytics — everything.

Owner's requirement: **Claude may do everything a person can do from the frontend.**

## Goals

- One remote MCP server, built into the existing Go backend, at `POST {public_url}/mcp`.
- Full parity with the frontend: every authenticated `/api/*` route a screen can call is reachable.
- Claude acts **as the connecting user, in one organisation, with that user's role**. All existing
  permission checks, org scoping, audit logs, campaign guards and validation apply unchanged,
  because tool calls go through the exact same HTTP handler chain as the browser.
- Works as a claude.ai custom connector (OAuth 2.1 + dynamic client registration) and in
  Claude Code / Desktop with an existing API key (`X-API-Key: whm_…`).
- A Settings page shows the connector URL, setup steps, and the user's connected AI apps with revoke.

## Non-goals

- No new business features; the connector only exposes what exists.
- No streaming/SSE, no server→client notifications, no MCP resources/prompts (tools only).
- No WebSocket/live chat push; Claude polls via list endpoints.
- The dormant chatbot graph engine stays dormant.

## Anti-goals (failure even if it "works")

- Claude gaining any permission the user does not have, or reaching another organisation's data.
- Claude completing its own OAuth consent, changing the user's password, or minting login sessions.
- Tool list so large it floods Claude's context (300+ tools).
- Silent drift: a new route added later that the connector cannot reach, unnoticed.
- Changing behaviour of existing endpoints or the browser login flow.

## Architecture (builder's decisions, made by Claude)

### Package layout

New package `internal/mcp/` (keep files small, ~150–200 lines each):

- `server.go` — fastglue handler for `/mcp`: JSON-RPC 2.0, batch not required.
  Methods: `initialize`, `notifications/initialized` (202, no body), `ping`, `tools/list`, `tools/call`.
  Unknown method → JSON-RPC error -32601. Protocol version: echo the client's requested version if it is
  one of `2025-06-18`, `2025-03-26`, `2024-11-05`; else answer `2025-06-18`. Stateless: no
  `Mcp-Session-Id` required (may issue one, never required). `GET /mcp` and `DELETE /mcp` → 405.
  Server info name `whatomate`, instructions string explaining: acting user/org, call
  `whatomate_list_areas` → `whatomate_describe_area` → `whatomate_call`; dates are ISO in the API
  but shown to people as dd/mm/yyyy; adding/importing RSVP guests never messages anyone, sending is explicit.
- `auth.go` — resolves the caller from the `/mcp` request:
  1. `X-API-Key` → validated with the existing `validateAPIKey` logic (export a helper from middleware).
     Org = key's org; switching org not allowed for API keys.
  2. `Authorization: Bearer wmcp_at_…` → MCP OAuth access token → grant (user, org).
  3. Otherwise 401 with `WWW-Authenticate: Bearer resource_metadata="{base}/.well-known/oauth-protected-resource"`.
- `dispatch.go` — executes an API call **in-process** through the same fasthttp handler the server
  uses (store the final wrapped handler — `ReportAPIErrors(corsWrapper(g.Handler()))` or `g.Handler()` —
  on a field the MCP package can reach after `setupRoutes`). Build a fresh `fasthttp.RequestCtx`
  (method, URI with base path + query, JSON or multipart body, headers). Auth header:
  - OAuth grant: a freshly minted internal app JWT (5 min expiry) carrying the same claims a browser
    session would get for that user in that org (role from `UserOrganization` for non-super-admins,
    exactly like `SwitchOrg`). Reuse `generateAccessToken` logic via an exported App method.
  - API key: forward the same `X-API-Key`.
  Add header `X-Whatomate-Client: mcp` (informational). Remote address 127.0.0.1.
  Apply rate limiting like any API request (no exemption).
- `catalog.go` + `catalog_<area>.go` — the endpoint catalog (see below).
- `tools.go` — tool definitions and handlers.
- `result.go` — response shaping: JSON pretty-compact, truncate at 60 000 chars with a note
  ("truncated — use page/limit/search filters"); text/CSV returned as text (same cap); other binary →
  `{content_type, bytes, note}`; HTTP ≥ 400 → tool result `isError: true` with status + message from the
  error envelope.
- `oauth_*.go` — OAuth 2.1 authorization server (below).

### Tools (small fixed list; parity through the catalog)

Discovery and generic access:

| Tool | Purpose |
|---|---|
| `whatomate_whoami` | user, email, current org, role name, permission list (from `GET /api/me`) |
| `whatomate_list_organizations` | orgs the user belongs to (`GET /api/me/organizations`) |
| `whatomate_switch_organization` | OAuth grants only: verify membership (same rules as `SwitchOrg`), update the grant's org. API key → error explaining keys are org-fixed |
| `whatomate_list_areas` | areas (contacts, chats/messages, templates, campaigns, rsvp, chatbot, flows, accounts, lead ads, catalogs, users, roles, teams, tags, canned responses, webhooks, custom actions, api keys, analytics, widgets, calls/IVR, settings, audit logs, admin…) each with one-line description and endpoint count |
| `whatomate_describe_area(area)` | every endpoint in the area: method, path template, summary, required permission, query params, JSON body schema, notes (side-effects, e.g. "sends WhatsApp messages") |
| `whatomate_call(method, path, query?, body?)` | call any catalogued JSON endpoint. Path must match a catalog entry (path templates with `{id}` filled). Rejects excluded/unknown routes |
| `whatomate_upload(method, path, file_base64, filename, content_type?, file_field?, fields?)` | multipart endpoints (media, CSV imports, template samples, guest import, flow/IVR media, business profile photo). Max decoded size 16 MB |

Featured typed tools (thin wrappers over catalog entries, with proper input schemas) for the
most common jobs, so Claude does not need discovery for everyday work:
`whatomate_search_contacts`, `whatomate_list_conversations`/chat list, `whatomate_get_messages`,
`whatomate_send_text`, `whatomate_send_template`, `whatomate_list_templates`, `whatomate_list_accounts`,
`whatomate_list_campaigns`, `whatomate_get_campaign`, `whatomate_list_rsvp_events`,
`whatomate_get_rsvp_results`, `whatomate_list_chatbot_flows`, `whatomate_dashboard_analytics`.
A catalog entry marks itself featured with `Tool: "whatomate_…"`; the tool's input schema = path params
+ query params + body schema of that entry. Implementer picks the exact endpoints per name from the routes.

Annotations: GET-only tools `readOnlyHint: true`; `whatomate_call`/`whatomate_upload` and featured
send/start/delete tools `destructiveHint: true, openWorldHint: true`. Claude clients ask the user before
destructive tools by default.

### Endpoint catalog

Go data, one file per area: `Entry{Area, Method, Path, Summary, Permission, Query []Param,
Body map[string]any (JSON schema), Multipart *MultipartSpec, SideEffects string, Tool string}`.
Summaries/body schemas are derived from the handler request structs and the frontend `api.ts` calls.
Body schemas list real field names and types; enums where the handler validates them.

**Parity guard test** (`internal/mcp/catalog_parity_test.go`): parse `cmd/whatomate/main.go`
for every `g.GET/POST/PUT/PATCH/DELETE("/api/…")` registration and assert each is either in the
catalog or in the explicit exclusion list; also assert no catalog entry points at a non-existent route.
Adding a route without cataloguing it fails CI.

**Excluded routes** (session/infra plumbing, or unsafe for an agent):
`/api/auth/*` (login, register, refresh, logout, switch-org, ws-token, SSO), `/api/webhook*` (Meta
callbacks), `/api/custom-actions/redirect*`, `/api/me/password`, `/public/*`, `/ws`, `/health`, `/ready`,
`/api/embedded-signup/config`, and the MCP OAuth consent endpoints themselves. Everything else is included,
including super-admin areas (they still require super-admin server-side).

### OAuth 2.1 authorization server

Endpoints (outside `/api` so the global auth `Before` hook skips them; add them explicitly to that hook's
skip list only if needed):

- `GET /.well-known/oauth-protected-resource` (also `/.well-known/oauth-protected-resource/mcp`) — RFC 9728:
  `resource = {base}/mcp`, `authorization_servers = [{base}]`.
- `GET /.well-known/oauth-authorization-server` — RFC 8414: issuer, `authorization_endpoint {base}/oauth/authorize`,
  `token_endpoint {base}/oauth/token`, `registration_endpoint {base}/oauth/register`,
  `response_types_supported [code]`, `grant_types_supported [authorization_code, refresh_token]`,
  `code_challenge_methods_supported [S256]`, `token_endpoint_auth_methods_supported [none]`.
- `POST /oauth/register` — RFC 7591 dynamic client registration (public clients). Accept `client_name`,
  `redirect_uris` (https anywhere, or `http://localhost|127.0.0.1:*`). Store client. Rate limited by IP.
- `GET /oauth/authorize` — validate client, redirect_uri (exact match), `response_type=code`,
  `code_challenge` S256 required, `state`, optional `resource`. Then redirect the browser to the
  frontend consent page `{base}{base_path}/oauth/consent?<original params>`.
- Frontend `/oauth/consent` (Vue, public route that requires login: if no session, go to login and come back) —
  shows "<client_name> wants to use Whatomate as <user email>", organisation picker (user's orgs, current preselected),
  plain-language list of what it can do ("everything your role allows in this organisation, including sending
  WhatsApp messages"), Approve / Deny buttons.
- `POST /api/oauth/consent` (normal JWT session auth, excluded from the MCP catalog) — body: original params +
  `organization_id` + `approve`. Verifies membership; on approve stores a one-time code in Redis
  (5 min, bound to client, redirect_uri, code_challenge, user, org, resource) and returns `{redirect_url}`
  (`redirect_uri?code=…&state=…`); on deny returns redirect with `error=access_denied`.
- `POST /oauth/token` (form-encoded) — `authorization_code` (+ `code_verifier` PKCE check, client & redirect match,
  single use) → creates grant + `access_token` (`wmcp_at_` + 32 random bytes hex, 1 h) +
  `refresh_token` (`wmcp_rt_…`, 30 days, rotated on each use; reuse of an old refresh token revokes the grant).
  `refresh_token` grant → new pair. Errors per RFC 6749 JSON.

Storage (gorm models, added to migrations and `GetMigrationModels`):
- `MCPOAuthClient{ID, ClientID(unique), ClientName, RedirectURIs(jsonb), CreatedAt}`
- `MCPGrant{ID, UserID, OrganizationID, ClientID, ClientName, CreatedAt, LastUsedAt, RevokedAt}`
- `MCPToken{ID, GrantID, Kind(access|refresh), TokenHash(sha256 hex, unique), ExpiresAt, UsedAt}`
Tokens are never stored in plain text. A grant is invalid if revoked, user inactive, or (non-super-admin)
membership of the org removed — checked on every `/mcp` request.

Management API (session-auth, catalogued as read/revoke only so Claude can list but the consent stays human):
- `GET /api/mcp/grants` — the current user's connected apps (client name, org name, created, last used).
- `DELETE /api/mcp/grants/{id}` — revoke own grant (deletes its tokens).
- `GET /api/mcp/info` — connector URL (`{base}/mcp`) for the settings page.

Base URL: `server.public_url` when set, else derived from the request (scheme from `X-Forwarded-Proto`,
host from `X-Forwarded-Host`/Host) — same approach the app already uses for request-derived links.
`server.base_path` is honoured for the frontend consent URL.

Config: `mcp.enabled` (default `true`). When false, `/mcp`, `/oauth/*` and well-known endpoints return 404.

### Frontend

- `/oauth/consent` view (see above). Uses existing auth store / login redirect.
- Settings → **AI Connector** (`/settings/ai-connector`, visible to every logged-in user):
  - Connector URL with copy button.
  - Step-by-step for claude.ai (Settings → Connectors → Add custom connector → paste URL → Connect → approve),
    and for Claude Code (`claude mcp add --transport http whatomate <url> --header "X-API-Key: whm_…"`, linking to API Keys page).
  - "Connected AI apps" table: app name, organisation, connected on, last used (dd/mm/yyyy, dd/mm/yyyy HH:mm via
    `formatDate`/`formatDateTime`), Revoke (confirm dialog). Keep `RefreshButton` + `useViewRefresh`.
- API client additions in `frontend/src/services/api.ts`.

### Docs

- `MCP_CONNECTOR.md` at repo root: what it is, setup for claude.ai and Claude Code, security model,
  how to add a new route to the catalog.
- AGENTS.md: repo map entry + convention bullet (catalog parity rule, in-process dispatch, OAuth tables).
- README link.

## Constraints

- Go 1.26, fastglue/fasthttp, gorm/Postgres, Redis. No net/http MCP SDK; hand-rolled JSON-RPC is small.
- Module path `github.com/nikyjain/whatomate`. Owner rule: existing behaviour must not change.
- Files ~150–200 lines; split catalog by area.
- DB tests follow AGENTS.md rules (unique fixture names, scoped assertions).
- User-facing dates dd/mm/yyyy.

## Verification

- Unit: JSON-RPC methods, protocol negotiation, tool list stable, result truncation, path validation
  (rejects excluded/unknown/`..`), PKCE S256, redirect-URI exact match, code single-use, refresh rotation +
  reuse detection, revoked grant → 401, org switch membership check, API-key path.
- Integration (DB-backed): `tools/call whatomate_call GET /api/contacts` as an OAuth grant returns only that org's
  contacts; a user lacking a permission gets `isError` 403 from the real handler.
- Parity test passes.
- `go build ./...`, `go build -tags chatbotgraph ./...`, `go test ./internal/mcp/... ./internal/handlers -count=1`,
  `golangci-lint` if available, `cd frontend && npm run typecheck && npx vitest run && npm run build`.

## Left to the builder

Exact featured-endpoint picks, catalog wording, file splits, error message wording, consent page styling
(follow existing shadcn-vue components and settings page patterns).
