# AGENTS

Read this file before starting any complex task in this repository.

## Repo Map

- Backend entrypoint: [cmd/whatomate/main.go](/e:/xampp/htdocs/bu-so/whatomate/cmd/whatomate/main.go)
- Backend handlers: [internal/handlers](/e:/xampp/htdocs/bu-so/whatomate/internal/handlers)
- Data models: [internal/models/models.go](/e:/xampp/htdocs/bu-so/whatomate/internal/models/models.go)
- Database setup and migrations: [internal/database](/e:/xampp/htdocs/bu-so/whatomate/internal/database)
- Frontend app: [frontend/src](/e:/xampp/htdocs/bu-so/whatomate/frontend/src)
- Settings views: [frontend/src/views/settings](/e:/xampp/htdocs/bu-so/whatomate/frontend/src/views/settings)
- Shared frontend components: [frontend/src/components](/e:/xampp/htdocs/bu-so/whatomate/frontend/src/components)
- API client layer: [frontend/src/services/api.ts](/e:/xampp/htdocs/bu-so/whatomate/frontend/src/services/api.ts)
- Docker assets: [docker](/e:/xampp/htdocs/bu-so/whatomate/docker)
- MCP connector (remote MCP server + OAuth 2.1): [internal/mcp](/e:/xampp/htdocs/bu-so/whatomate/internal/mcp)
- External API docs: [EXTERNAL_MESSAGE_API.md](/e:/xampp/htdocs/bu-so/whatomate/EXTERNAL_MESSAGE_API.md), [EXTERNAL_TEMPLATE_SEND_API.md](/e:/xampp/htdocs/bu-so/whatomate/EXTERNAL_TEMPLATE_SEND_API.md)
- MCP (Claude) connector: [internal/mcp](/e:/xampp/htdocs/bu-so/whatomate/internal/mcp), docs [MCP_CONNECTOR.md](/e:/xampp/htdocs/bu-so/whatomate/MCP_CONNECTOR.md)

## Working Rules

- Keep frontend and backend changes aligned when an API contract changes.
- Prefer small, focused files. If a file grows beyond roughly 150-200 lines because of a new feature, split it.
- Update this file or [README.md](/e:/xampp/htdocs/bu-so/whatomate/README.md) when adding a new workflow, root-level doc, or architectural pattern that future agents need to know.
- Do not assume Docker publishing conventions from memory; use [DOCKER_HUB_PUBLISHING.md](/e:/xampp/htdocs/bu-so/whatomate/DOCKER_HUB_PUBLISHING.md).

## Verification

- Frontend typecheck: `cd frontend && npm run typecheck`
- Frontend build: `cd frontend && npm run build`
- Backend tests: `go test ./...`
- Handler-focused tests: `go test ./internal/handlers -count=1`

If the local shell does not have Go installed, note that explicitly instead of claiming backend verification.

- DB-backed Go tests share one database (`TEST_DATABASE_URL`, plus `TEST_REDIS_URL`) and run in parallel. Give fixtures unique names (WhatsApp account names are globally unique) and scope assertions to your own org/fixtures. Tests of intentionally global behaviour (super-admin listings, cross-org chat repair scans) must use `newIsolatedTestApp` / `testutil.SetupIsolatedTestDB`, which gives the test a private, freshly migrated schema. Remember `Contact.IsActive` has a gorm default of `true`: create, then update to `false`.
- Frontend unit tests: `cd frontend && npx vitest run`. User-facing dates go through `formatDate` / `formatDateTime` in `frontend/src/lib/utils.ts` (dd/mm/yyyy, dd/mm/yyyy HH:mm).

## Current Important Conventions

- Conversation flow reply options support explicit reply-button and list-picker modes via step `input_config.reply_mode`; runtime routing uses option IDs for both button and list replies, and new flows should be scoped with `whatsapp_account` when account-specific.
- External message ingestion supports super-admin routing by `phone_number_id` to the correct organization/account.
- Legacy misrouted chats can be reviewed from the super-admin-only Settings UI using all message history, with safe moves plus explicit manual approval for merge-required cases.
- The chat repair backend flows are preview/scan/apply under `/api/admin/chat-repair`; opening the tab remains read-only, and the explicit `Scan Candidates` action auto-applies safe move candidates, including same-location chats whose active messages still drift from the resolved organization/account.
- Multi-organisation membership is managed from Settings → Organisation Members (`/settings/organization-members`), backed by `/api/organizations/members` and gated by the existing `organizations` permission (`read` to view, `assign` to add, change role or remove). Member update/remove endpoints are keyed by the member's `user_id`; creating brand-new user accounts stays on Settings → Users.
- WhatsApp onboarding now has a resumable hybrid wizard under Settings → Accounts, backed by `/api/accounts/onboarding/sessions/*` plus super-admin-only app-level Meta config at `/api/settings/meta-onboarding`. Keep new onboarding work session-first and preserve the legacy `/api/accounts` CRUD/manual editor for break-glass maintenance.
- RSVP events support `guest_list` and `open_keyword` admission. `RSVPResponse` is the unified guest/response roster: `rsvp_started_at` distinguishes never-started guests from incomplete flows, reminders target only never-started guests, and incomplete flows remain under the re-prompt workflow. Adding or importing guests never sends a message; invitations and reminders are explicit actions.
- RSVP result tallies expose independent Member Attendance and Spouse Attendance counts. An attending spouse without a valid configured spouse-mobile answer is counted in both Spouse Attending and Spouse Pending until the contact is completed; this does not change the member's attendance count.
- Manual “all not started” reminders accept response-ID exclusions so organizers can remove recipients from the searchable preview without changing the guest roster. Reminder template variables use saved per-placeholder mappings (member/event tokens or fixed text) and are resolved per recipient; scheduled reminders persist those mappings while continuing to evaluate all not-started guests at execution time.
- Manual and scheduled RSVP reminders create source-linked bulk campaigns. RSVP owns recipient eligibility and variable resolution; the campaign queue owns delivery and progress. The worker must recheck that each guest is still not started immediately before sending and synchronize each campaign recipient outcome back to `RSVPReminderDelivery`. Do not allow recipient edits, deletion, or resend on RSVP-linked campaigns; initiate a fresh reminder from RSVP Results instead.
- Meta Lead Ads: Pages connect under Settings → Lead Ads (`/api/lead-sources`, `/api/lead-mappings`, `/api/leads`, all gated by the `lead_ads` permission resource incl. `lead_ads:export` for the `leads` export table). Meta calls the public `/api/webhook/leadgen`; every `LeadSource` must have an App Secret and the webhook fails closed (403) when a page's source has none or the `X-Hub-Signature-256` HMAC does not match. The webhook only queues `Lead` rows (idempotent on `leadgen_id`); the `LeadProcessor` worker (`internal/handlers/lead_processor*.go`) fetches answers from Graph whenever `field_data` is empty, resolves org-scoped form mappings, upserts the contact with targeted column updates and sends the template at most once via the `sending` status marker (a lead stuck in `sending` is never auto-resent; operators re-queue it). Page/form uniqueness uses partial unique indexes over live rows in `database.getIndexes`, and leads hold no DB foreign keys to contacts/messages/templates so existing hard-delete jobs keep working.

- Upstream sync (shridarpatil/whatomate) is merged in stages with real merges. Our module path stays `github.com/nikyjain/whatomate`. WhatsApp client sends take a `whatsapp.Recipient{Phone, BSUID}` (use `contactRecipient(contact)` in handlers). Template components: `whatsapp.BuildTemplateComponentsWithQuickReplyPayloads` is our full builder (URL params by index, quick-reply payloads, header link or media ID + filename); upstream's `BuildTemplateComponents(body, headerType, mediaID, filename)` is a thin wrapper, and `AutoButtonComponents` (FLOW tokens) is appended on every template send.
- Settings list pages follow upstream's list + detail-page structure (`/settings/<resource>/:id`, `DetailPageLayout`, `AuditLogPanel`); keep our `RefreshButton` + `useViewRefresh` on list pages. Campaigns keep our full list workspace (scheduling, resend, report export, RSVP lock) and link to the upstream detail page, which carries the same RSVP-reminder lock.
- Existing system `admin` roles are topped up with every permission at boot (`database.SyncAdminRolePermissions`), so new permission resources (e.g. `audit_logs`) never lock admins out.
- The optional per-user API rate limiter (`rate_limit.enabled`, off by default) exempts API-key requests to `/api/messages/external`, `/api/messages/template` and message status lookups, and always exempts Meta callbacks (`/api/webhook`, `/api/webhook/leadgen`, GET and POST). Missing `server.allowed_origins` in production only logs a warning (upstream makes it fatal).
- Webhook signature checks stay ours: accounts with an app secret always require a valid `X-Hub-Signature-256`; without a secret, `whatsapp.require_webhook_signature` decides reject vs warn.
- Chatbot engine (stage 2 sync): upstream's v2 graph engine files (`chatbot_graph_runner.go`, `chatbot_graph_types.go`, `chatbot_flow_migration.go`, `ChatbotFlow.Graph` column, graph-native editor components) are present but DORMANT. The step engine (`ChatbotFlowStep`, still in AutoMigrate) runs every flow and RSVP; `BackfillChatbotFlowGraph` is never called at boot and incoming messages are never routed to the graph runner. Our `ChatbotFlowBuilderView.vue` + `FlowChart.vue` + `useChatbotFlowConverter.ts` remain the editor. Do not delete `ChatbotFlowStep` or wire the graph runner without an explicit migration plan. `chatbot_graph_runner.go` (which also holds `parseChatGraph`) carries the `//go:build chatbotgraph` tag together with its tests (`chatbot_graph_runner_test.go`, `chatbot_flow_migration_test.go`), so the default build/lint does not see the unreferenced runner; `chatbot_graph_types.go` and `chatbot_flow_migration.go` stay untagged. Verify both `go build ./...` and `go build -tags chatbotgraph ./...` (and `golangci-lint run --build-tags chatbotgraph ./internal/handlers/`) when touching these files.
- Template TEXT header variables: `whatsapp.BuildTemplateComponentsWithHeaderText` wraps our full builder and prepends the header text parameter; chat sends (`header_params`), campaign recipients (`BulkMessageRecipient.HeaderParams`) and the worker all use it.
- Embedded Signup/Coexistence: our onboarding wizard stays primary. Upstream's `/api/accounts/exchange-token`, `/api/accounts/{id}/register` and public `/api/embedded-signup/config` are kept for API compatibility only (no UI). `WhatsAppAccount.IsSMB` marks Coexistence (WhatsApp Business app) numbers; the wizard detects them from the phone status (`is_on_biz_app` / `platform_type`) and skips Meta `/register`. There is one `Client.RegisterPhoneNumber(ctx, account, RegisterPhoneNumberRequest)`. The account phone-setup `/api/accounts/{id}/register-phone` and the legacy `/api/accounts/{id}/register` both skip Meta `/register` when `IsSMB` is set (register-phone answers `skipped: true`).
- Config env vars (stage 3 sync, upstream #478): levels are separated by a double underscore, `WHATOMATE_<SECTION>__<KEY>` (`WHATOMATE_DATABASE__HOST`, `WHATOMATE_WHATSAPP__APP_ID`). Legacy single-underscore names (`WHATOMATE_DATABASE_HOST`) are still honoured for every known key via `internal/config/env_compat.go` (keys derived from the `Config` struct's koanf tags), print a deprecation warning at startup, and lose to the `__` form when both are set. Keep that fallback and its tests (`TestLoad_LegacySingleUnderscoreEnvStillWorks`, `TestLoad_BothEnvFormsLoad_DoubleUnderscoreWins`).
- Owner rule: our `main` behaviour must not change. Useful upstream behaviour changes live behind opt-in settings that default to main's behaviour: `ChatbotSettings.mark_bot_handled_as_read` (pre-mark bot-handled incoming messages read / mark read after bot replies) and `ChatbotSettings.skip_transfer_outside_hours` (flow transfer steps and queue/team transfers send only the out-of-hours message outside business hours; keyword transfers keep main's business-hours check). Flow completion webhooks and `api_fetch` steps expose the sender as `{{phone_number}}` only when the flow did not collect one and never write it into `session_data`. The production Dockerfile uses `CMD ["./whatomate", "server", ...]` with no ENTRYPOINT so existing `command:` overrides keep working. Phone-node Graph lookups that request newer fields (`is_on_biz_app`, `platform_type`, `whatsapp_business_manager_messaging_limit`) retry once with main's field list on a (#100) unknown-field error. Meta Insights falls back to INR.
- Chatbot flow delete keeps removing the flow's `chatbot_flow_steps` rows (the step engine owns them) but guards it with `HasTable` (upstream #585). WhatsApp button/list labels are truncated on rune boundaries with `truncateLabel` (Meta counts characters; hi/ta/ar labels are multi-byte).
- WhatsApp product catalogs (`/api/catalogs`, `/api/products`) are gated by the `catalogs` permission resource (read / write / delete / sync); the UI lives at `/catalogs` (Messaging menu) with components in `frontend/src/components/catalogs/`. Prices travel in minor units (cents); Meta errors are returned verbatim with HTTP 502 so the UI can show them.
- Contact phones are stored in one canonical form: `contactutil.CanonicalPhone` (digits only, country code, no `+`; bare 10-digit and `0`-prefixed numbers get `91`). Look contacts up with `contactutil.FindContact` / `FindContactUnscoped` / `GetOrCreateContact`, which match every legacy form via `PhoneVariants`; never query `phone_number = ?` with a raw input. `contactutil.MergeDuplicateContacts` runs at boot (idempotent, non-fatal): it folds same-number contacts into one survivor, repoints every `contact_id` column / FK to it, and hard-deletes duplicates (or tombstones them as `merged:<id>` when a reference cannot move).
- Renaming a contact (chat header pencil, Contact Info panel, `PUT /api/contacts/{id}` with `profile_name`) sets `Contact.ProfileNameLocked`; `GetOrCreateContact` then stops overwriting the name with WhatsApp profile or campaign recipient names. Saving an empty name unlocks it.
- Campaign delivery guard (`internal/webhookhealth`): the WhatsApp webhook records, per phone number ID in Redis, the last accepted and last signature-rejected callback; the campaign worker records the first send not yet followed by an accepted callback. While a number's latest callback was rejected, or a campaign send has gone 10 minutes without any accepted callback, `StartCampaign` answers 409 with the reason, due scheduled campaigns are paused, and the worker pauses running campaigns (recipients stay pending). The reason is stored in `BulkMessageCampaign.PauseReason` and shown in the campaign UI; start/resume and manual pause clear it. Any accepted callback for the number (e.g. an incoming message) lifts the block. Unknown state or no Redis never blocks. The `Contact.BSUID` column is named `bs_uid` by gorm; raw SQL must use `bs_uid`.
- Chatbot conversation history lives at `/chatbot/sessions` (list + detail) over `GET /api/chatbot/sessions[/{id}]`. Both endpoints require `flows.chatbot:read`; the list supports `status`, `flow_id`, `search` (contact name/phone), `from`/`to` (started date) and `page`/`limit`, and responses add `flow_name` and `message_count` to the session model. Keys in `session_data` starting with `_` are engine bookkeeping and are hidden from the Collected Answers view.
- MCP connector (`internal/mcp`, spec `docs/superpowers/specs/2026-11-09-mcp-connector-design.md`): `POST /mcp` (JSON-RPC, stateless) plus an OAuth 2.1 AS (`/.well-known/oauth-*`, `/oauth/register|authorize|token`, consent via `POST /api/oauth/consent`) and `/api/mcp/info|grants`. Tool calls are dispatched in-process through the server's final fasthttp handler (`mcpServer.SetHandler` in main.go) as the connecting user: OAuth grants get a fresh 5-minute app JWT from `App.ResolveOrgUser` + `App.MintAccessToken` (SwitchOrg role rules), API keys are forwarded as `X-API-Key` (`middleware.ValidateAPIKey`). Every `/api` route registered in main.go must be catalogued in `internal/mcp/catalog_<area>.go` or listed in `mcp.ExcludedRoutes`; `TestCatalogParity` fails otherwise. OAuth tables: `mcp_oauth_clients`, `mcp_grants`, `mcp_tokens` (tokens stored as SHA-256 only; refresh reuse revokes the grant). `mcp.enabled` (default true) turns the public endpoints into 404s. Docs `MCP_CONNECTOR.md`; users see and revoke connections in Settings → AI Connector (`/settings/ai-connector`), consent page `/oauth/consent`.

## Docker Publishing Rule

Always publish the application image as:

- `nikyjain/whatomate:latest`

Always publish as multi-arch for:

- `linux/amd64`
- `linux/arm64`

Use [DOCKER_HUB_PUBLISHING.md](/e:/xampp/htdocs/bu-so/whatomate/DOCKER_HUB_PUBLISHING.md) as the source of truth.

Preferred command from repo root:

- `make docker-push`

The Docker publish path also maintains a remote Buildx cache at:

- `nikyjain/whatomate:buildcache`
