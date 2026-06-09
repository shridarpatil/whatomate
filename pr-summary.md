# Whatomate API Documentation Updates PR Summary

This pull request updates the API Reference documentation to match the actual Go backend implementation in Whatomate. It registers missing endpoints, removes outdated references, adds pagination query parameters, defines exact response envelopes, and includes realistic examples.

## Key Changes Summary

### 1. Core System & Administration (Subagent 1 & Main Agent)
* **API Keys (`api-keys.mdx`):**
  * Documented missing `GET /api/api-keys/{id}` and `PUT /api/api-keys/{id}` (is_active toggle).
  * Added query parameters (`search`, `page`, `limit`) and updated the response structure to reflect pagination.
* **Authentication (`authentication.mdx`):**
  * Updated examples to clarify cookie-based JWT flow, logout session invalidation, and organization switching.
* **Organizations (`organizations.mdx`):**
  * Corrected organization settings endpoints from `/api/organizations/settings` to `/api/org/settings`.
  * Added calling settings fields (`calling_enabled`, `max_call_duration`, `transfer_timeout_secs`, `hold_music_file`, `ringback_file`).
  * Documented the `POST /api/org/audio` endpoint for uploading organization-level hold music/ringbacks.
* **Roles & Permissions (`roles.mdx`):**
  * Documented pagination query parameters on List Roles and corrected the response schema.
* **Teams (`teams.mdx`):**
  * Documented the entire Teams and Team Members CRUD API (which was previously undocumented in the sidebar config).
  * Added Teams reference page to Astro Starlight sidebar.
* **Webhooks (`webhooks.mdx`):**
  * Added documentation for the complete Webhook Management API: List, Get, Create, Update, Delete, and Test Webhook.
* **Custom Actions (`custom-actions.mdx`):**
  * Added pagination query parameters and documented the custom action redirect endpoint (`GET /api/custom-actions/redirect/{token}`).
* **Users (`users.mdx`):**
  * Corrected the availability update path to `PUT /api/me/availability` and documented List Users query parameters.

---

### 2. Contacts, Messaging, Templates & Automation (Subagent 2)
* **Contacts (`contacts.mdx`):**
  * Documented `/api/groups` and detailed contact session parameters (assign, tags, stop session).
* **Messages (`messages.mdx`):**
  * Documented reactions (`POST /api/contacts/{id}/messages/{message_id}/reaction`), media message sending, and media serving.
* **Templates (`templates.mdx`):**
  * Documented syncing, creation, submission/publishing, and media upload for template header samples.
* **Flows (`flows.mdx`):**
  * Updated Flows documentation to cover CRUD, saving to Meta, duplicate, sync, publish, and deprecate.
* **Campaigns (`campaigns.mdx`):**
  * Documented bulk campaign CRUD, control actions (start, pause, cancel, retry-failed), recipient imports/lists, and media uploads.
* **Chatbot (`chatbot.mdx`):**
  * Updated chatbot settings, keyword rules, flow graphs, AI contexts, sessions, and agent transfer picking (`POST /api/chatbot/transfers/pick`).
* **Canned Responses (`canned-responses.mdx`):**
  * Documented interactive buttons configurations schema.

---

### 3. Accounts, Calling, Catalogs & Analytics (Subagent 3)
* **Accounts (`accounts.mdx`):**
  * Added new model fields, `/api/accounts/{id}/subscribe`, and WhatsApp Business Profile retrieval, updates, and profile picture uploads.
* **Calling (`calling.mdx` - NEW):**
  * Documented Voice Calling APIs including IVR Flows, Call Logs (hold, resume, recording), Call Transfers, Outgoing calls (WebRTC connect, call permission status, ICE servers).
  * Added Calling reference to Astro Starlight sidebar.
* **Catalogs (`catalogs.mdx` - NEW):**
  * Documented Catalogs and Products APIs including catalog lists, sync, and product CRUD.
  * Added Catalogs reference to Astro Starlight sidebar.
* **Analytics (`analytics.mdx`):**
  * Corrected response shapes and documented Agent Analytics and Meta WhatsApp Analytics.

## Verification
* Ran `npm run build` inside the `docs` directory of the worktree. All Astro routes built successfully.
