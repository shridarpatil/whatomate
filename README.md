<a href="https://zerodha.tech"><img src="https://zerodha.tech/static/images/github-badge.svg" align="right" alt="Zerodha Tech Badge" /></a>

# Ritam Bharat

Modern, open-source WhatsApp Business Platform — single binary backend with an embed-able Vue frontend. This README documents the full feature set, architecture, local development, production deployment (including ngrok for Meta webhooks), configuration reference, rebranding checklist, and commercial/licensing notes.

<!-- TOC -->
1. Introduction
2. Features (complete)
3. Architecture & major components
4. Quickstart — Local (Docker + dev)
5. Production deployment (binary, Docker, reverse proxy, SSL)
6. Webhooks and ngrok (Meta verification)
7. Configuration reference highlights
8. Developer workflow & CLI
9. Rebranding checklist (file-by-file)
10. Commercializing and licensing (AGPL-3.0 notes)
11. Security & operational checklist
12. Troubleshooting & FAQ
13. Contributing

## 1. Introduction

Ritam Bharat is a full-featured WhatsApp Business Platform designed for running multi-tenant WhatsApp operations: messaging, templating, campaigns, chatbots, agent inbox, analytics, and optional WhatsApp calling (WebRTC/IVR). It ships as a Go backend (single binary in production) and a Vue.js frontend (Vite during development). Everything required to run locally or in production is included or documented.

## 2. Features (complete)

- Multi-tenant support: multiple organizations with isolated data, configs, and API keys.
- Users, roles & permissions: customizable roles, per-resource action permissions, super-admins and org admins.
- WhatsApp Cloud API integration: add Meta credentials per WhatsApp account and send/receive messages.
- Incoming & outgoing messages: text, images, video, documents, interactive messages.
- Real-time chat: WebSocket support for agent inbox and live updates.
- Message templates: create, manage, sync, and publish templates to Meta.
- Bulk campaigns: scheduled/batch sending with retries and reporting.
- Chatbot automation: rule-based, flow-based, and AI-augmented (OpenAI/Anthropic/Google) responders and flow builder.
- Canned responses & slash commands: quick replies with dynamic placeholders.
- Contact import/export and segmentation.
- Agent workspaces: assign conversations, transfers, SLA processing, and chat transcripts.
- Analytics dashboard: message counts, engagement, campaign performance, agent metrics.
- WhatsApp voice calling & IVR (optional): WebRTC bridging, IVR flows, DTMF input, recording and transfers.
- Storage adapters: local filesystem or S3-compatible storage for uploads and call recordings.
- Queue & worker model: background workers for campaigns, media processing, retries.
- Webhook management: configure outbound webhooks for message and system events.
- Audit logs and analytics for compliance.

## 3. Architecture & major components

- Backend: Go, built with Fastglue; main entry: [cmd/whatomate/main.go](cmd/whatomate/main.go).
- Frontend: Vue 3 + Vite in `/frontend/` (dev) and embedded static build when building production binary.
- Database: PostgreSQL (schema via GORM). Migration happens automatically on `-migrate`.
- Cache / ephemeral storage: Redis (sessions, rate limits, transient state).
- Background workers: run as separate `whatomate worker` processes or embedded workers in the `server` command.
- Webhooks: single verification & event endpoint mounted at `/api/webhook` (see handlers in `internal/handlers/webhook.go`).
- Media storage: local `uploads/` or S3 (configured via `config.toml`).
- Reverse proxy: recommended (Caddy or Nginx) for HTTPS and hostname routing in production.

Diagram (logical):

- Browser (agent/frontend) <-> Backend API (HTTP + WebSocket)
- Backend reads/writes -> Postgres
- Backend uses Redis for sessions/rate-limits
- Workers consume queues (Redis/DB) for background jobs
- Optional: S3 for uploads/recordings
- Meta WhatsApp Cloud <-> Backend via webhook + outgoing REST calls

## 4. Quickstart — Local

Recommended local setup is Docker Compose (fast) or running backend + frontend separately for development.

Docker (quick):

```bash
# Copy sample config and .env then start
cp config.example.toml config.toml
cp docker/.env.example .env
docker compose -f docker/docker-compose.yml up -d

# Visit http://localhost:8080 (login: admin@admin.com / admin)
```

Developer mode (backend + frontend separately):

```bash
# Backend (with migrations)
make run-migrate

# Frontend (separate terminal)
cd frontend
npm install
npm run dev

# Frontend dev runs on :3000 and proxies /api to :8080
```

Binary (production local):

```bash
make build-prod
./whatomate server -migrate -config config.toml
```

## 5. Production deployment (overview)

Options:
- Docker Compose (single host) — uses `docker/docker-compose.yml`.
- Single binary (recommended minimal): `make build-prod` produces an executable with embedded frontend.

Essential production steps:
1. Set `app.environment = "production"` and `app.debug = false` in `config.toml`.
2. Provide `app.encryption_key` (32+ chars) and strong `jwt.secret`.
3. Run database & redis (managed services or local containers).
4. Use a reverse proxy (Caddy recommended for automatic TLS) to serve `https://yourdomain/` and forward to the API on `localhost:8080`.
5. Enable cookie domain and secure cookies in `[cookie]` when serving under your domain.
6. Ensure backups for Postgres and uploads, and optionally enable snapshots.

Example (Caddyfile):

```
yourdomain.com {
  reverse_proxy localhost:8080
}
```

DigitalOcean quick notes (budget-minded):
- Droplet: start with 2 vCPU / 4GB RAM (~$24/mo).
- Use DO managed Postgres for production if you prefer managed backups & HA.
- Your $200 DO credit covers several months depending on droplet size.

### Auto-deploy on GitHub push

This repository includes [deploy-production workflow](.github/workflows/deploy-production.yml) that deploys automatically to your server when you push to `main` or `deploy/do-app`.

Set these GitHub Actions repository secrets:
- `PROD_HOST` (example: `142.93.210.66`)
- `PROD_USER` (example: `root`)
- `PROD_SSH_KEY` (private key content for SSH auth)
- `PROD_APP_DIR` (optional, defaults to `/root/whatomate`)

What the workflow does:
1. Connects to the droplet over SSH.
2. Checks out the pushed branch and pulls latest changes.
3. Runs `docker compose up -d --build --remove-orphans`.
4. Runs a health check on `http://127.0.0.1:8080/health`.

You can also run it manually from GitHub Actions with `workflow_dispatch` and choose the branch.

## 6. Webhooks and ngrok (Meta verification)

The project exposes the Meta webhook endpoints on:

```
GET  /api/webhook    # verification (hub.challenge)
POST /api/webhook    # events
```

Verification: Meta will call the GET endpoint with query params `hub.mode`, `hub.verify_token`, and `hub.challenge`. The app checks the token against (in order):
- `whatsapp.webhook_verify_token` in `config.toml` (global token)
- Per-account `webhook_verify_token` stored when adding a WhatsApp account via the UI

Set a verify token in `config.toml` example:

```toml
[whatsapp]
webhook_verify_token = "my-super-secret-webhook-token-123"
api_version = "v18.0"
base_url = "https://graph.facebook.com"
```

Using ngrok (develop/demo):

```bash
# Start your local server on 8080 (docker compose or ./whatomate)
ngrok http 8080

# Use the HTTPS forwarding URL that ngrok provides as your Meta callback:
https://<ngrok-id>.ngrok-free.dev/api/webhook

# Use the same verify token value in Meta as in config.toml or per-account settings.
```

Important: do not expose the raw Postgres/Redis ports publicly—keep them bound to localhost or the internal network.

## 7. Configuration reference highlights

- `config.example.toml` holds every setting and is the single source of truth for server config.
- Important security fields:
  - `app.encryption_key` — AES-256 key to encrypt API secrets at rest (required in production).
  - `jwt.secret` — strong JWT signing secret (32+ chars).
  - `[whatsapp].webhook_verify_token` — global verify token (optional if you use per-account tokens).
  - `[cookie].domain` and `cookie.secure` — set when using a domain and HTTPS.

Environment variables are supported with the `WHATOMATE_` prefix (e.g. `WHATOMATE_DATABASE_HOST`). See `internal/config/config.go` for mapping.

## 8. Developer workflow & CLI

- Build backend: `make build`
- Build prod binary (embedded frontend): `make build-prod`
- Run with migrations: `./whatomate server -migrate -config config.toml`
- Run workers only: `./whatomate worker -workers=4`
- Run tests: `make test`

Key API routes (see `cmd/whatomate/main.go` for exact bindings):
- `GET /api/webhook` — Meta verification challenge
- `POST /api/webhook` — Meta events
- `/api/webhooks` — CRUD for outbound webhooks (managed in UI)

## 9. Rebranding checklist (file-by-file)

To rebrand the product (visible name, logos, and internal references):

- Visual & docs
  - `docs/src/assets/logo-light.svg` and `docs/src/assets/logo-dark.svg` — replace SVG text/logo
  - `README.md` and `docs/` content — update product name, links and examples
  - `frontend/index.html` and global app title in `frontend/src/` — update meta/title

- Code & binaries
  - `cmd/whatomate/main.go` — change CLI help text and app name strings
  - `internal/handlers/webhook_dispatch.go` and other internal logs/user-agent strings — update `RitamBharat-Webhook/1.0` if desired
  - Docker image name references in `docker/docker-compose.yml` and `docker/Dockerfile*` if you build your own images

- Configuration & package
  - `config.example.toml` — change default `app.name`
  - `frontend/package.json` / `docs/package.json` — update display names if needed

Notes: The project is AGPL-3.0 — see licensing notes below before distributing a modified network service.

## 10. Commercializing & licensing (AGPL-3.0 notes)

This repository is licensed under the GNU Affero General Public License v3.0. Key implications:
- You can sell hosted services built with this software and charge for hosting, support, or customization.
- If you run a modified version that users interact with over a network, you must make the source of the modified version available to those users under AGPL-3.0.
- Before offering a hosted SaaS or distributing modified binaries, consult legal counsel to ensure compliance with AGPL requirements and to decide on support/dual-licensing strategies.

Business model tips:
- Offer managed hosting (setup + monthly), SLA tiers, onboarding & template setup as premium services.
- Keep a conduit for secure storage of Meta tokens — you must rotate and secure them.

## 11. Security & operational checklist

- Set `app.encryption_key` and `jwt.secret` to long, randomly generated values before production.
- Do not use default admin login in production — change immediately.
- Use HTTPS (Caddy or Let’s Encrypt) and secure cookies.
- Limit API access via CORS and a strong firewall.
- Scale workers and database connections according to load. Monitor CPU, memory, and Redis latency.

## 12. Troubleshooting & FAQ

- Frontend can’t reach API in dev: ensure backend running on `:8080` and start frontend from `/frontend` with `npm run dev`.
- Webhook verification failing: ensure verify token in Meta matches `config.toml` or per-account token and you’re using the HTTPS ngrok URL (if using ngrok).
- Port conflicts: change `server.port` in `config.toml`.

## 13. Contributing

Please read [CONTRIBUTING.md](CONTRIBUTING.md) before opening pull requests. Tests live in `internal/*_test.go` and can be run via `make test`.

---

If you want, I can now:
- Apply a tailored rebrand (replace logos + app name) across the repo.
- Create a `DEPLOYMENT.md` with step-by-step DigitalOcean droplet commands using your $200 credit.
- Produce an `ONBOARDING.md` for first 10 customers and pricing tiers.

Which of these should I do next?
