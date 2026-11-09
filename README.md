<a href="https://zerodha.tech"><img src="https://zerodha.tech/static/images/github-badge.svg" align="right" alt="Zerodha Tech Badge" /></a>

# Whatomate

Modern, open-source WhatsApp Business Platform. Single binary app.

![Dashboard](docs/public/images/dashboard-light.png#gh-light-mode-only)
![Dashboard](docs/public/images/dashboard-dark.png#gh-dark-mode-only)

## Features

- **Multi-tenant Architecture**
  Support multiple organizations with isolated data and configurations.

- **Granular Roles & Permissions**
  Customizable roles with fine-grained permissions. Create custom roles, assign specific permissions per resource (users, contacts, templates, etc.), and control access at the action level (read, create, update, delete). Super admins can manage multiple organizations.

- **WhatsApp Cloud API Integration**
  Connect with Meta's WhatsApp Business API for messaging, including optional MM Lite routing controls for marketing templates after account onboarding. Settings -> Accounts also includes Meta-backed phone verification, phone registration, and two-step PIN management. Meta does not expose an API to disable two-step verification.

- **Real-time Chat**
  Live messaging with WebSocket support for instant communication, unread counters, and unread-only filtering.

- **External Message Ingestion**
  Persist externally-sent WhatsApp messages without calling Meta, with super-admin routing by `phone_number_id` to the correct organization/account.

- **Super Admin Chat Repair Tool**
  Settings includes a super-admin-only repair panel for reviewing legacy misrouted chats across existing message history, with safe moves and explicit manual approval for merge-required cases.

- **Template Management**
  Create and manage message templates approved by Meta.

- **Bulk Campaigns**
  Send campaigns to multiple contacts with retry support for failed messages, append new recipients to existing campaigns, and export per-campaign XLSX delivery reports.

- **Chatbot Automation**
  Keyword-based auto-replies, conversation flows with branching logic, and AI-powered responses (OpenAI, Anthropic, Google).

- **Canned Responses**
  Pre-defined quick replies with slash commands (`/shortcut`) and dynamic placeholders.

- **Voice Calling & IVR**
  Incoming and outgoing WhatsApp calls with IVR menus, DTMF routing, call transfers to agent teams, hold music, and call recording. See [calling docs](https://shridarpatil.github.io/whatomate/features/calling/).

- **Analytics Dashboard**
  Track messages, engagement, and campaign performance.

<details>
<summary>View more screenshots</summary>

![Dashboard](docs/public/images/dashboard-light.png#gh-light-mode-only)
![Dashboard](docs/public/images/dashboard-dark.png#gh-dark-mode-only)
![Chatbot](docs/public/images/chatbot-light.png#gh-light-mode-only)
![Chatbot](docs/public/images/chatbot-dark.png#gh-dark-mode-only)
![Agent Analytics](docs/public/images/agent-analytics-light.png#gh-light-mode-only)
![Agent Analytics](docs/public/images/agent-analytics-dark.png#gh-dark-mode-only)
![Conversation Flow Builder](docs/public/images/conversation-flow-light.png#gh-light-mode-only)
![Conversation Flow Builder](docs/public/images/conversation-flow-dark.png#gh-dark-mode-only)
![Templates](docs/public/images/11-templates.png)
![Campaigns](docs/public/images/13-campaigns.png)

</details>

## Installation

### Docker

The compose setup can build the app directly from [`jba-nitinjain/whatomate`](https://github.com/jba-nitinjain/whatomate.git) instead of pulling a prebuilt image.

```bash
# Download compose file, sample config, and env file
curl -LO https://raw.githubusercontent.com/jba-nitinjain/whatomate/main/docker/docker-compose.yml
curl -LO https://raw.githubusercontent.com/jba-nitinjain/whatomate/main/config.example.toml
curl -L https://raw.githubusercontent.com/jba-nitinjain/whatomate/main/docker/.env.example -o .env

# Copy and edit config
cp config.example.toml config.toml
# Edit .env to set PostgreSQL credentials and timezone

# Build from the repo and run services
docker compose up -d
```

Go to `http://localhost:8080` and login with `admin@admin.com` / `admin`

#### Docker Hub Publishing

When publishing images to Docker Hub, always push a multi-arch image that includes both:

- `linux/amd64`
- `linux/arm64`

Preferred shortcut:

```bash
make docker-push
```

Example:

```bash
docker buildx build \
  --builder multiarch-builder \
  --platform linux/amd64,linux/arm64 \
  -f docker/Dockerfile \
  --cache-from type=registry,ref=nikyjain/whatomate:buildcache \
  --cache-to type=registry,ref=nikyjain/whatomate:buildcache,mode=max,oci-mediatypes=true,image-manifest=true,compression=gzip \
  -t nikyjain/whatomate:latest \
  -t nikyjain/whatomate:<tag> \
  --push .
```

For the standard repeatable workflow, see [DOCKER_HUB_PUBLISHING.md](./DOCKER_HUB_PUBLISHING.md). Agents should start from [AGENTS.md](./AGENTS.md).

__________________

### Binary

Download the [latest release](https://github.com/shridarpatil/whatomate/releases) and extract the binary.

```bash
# Copy and edit config
cp config.example.toml config.toml

# Run with migrations
./whatomate server -migrate
```

Go to `http://localhost:8080` and login with `admin@admin.com` / `admin`

__________________

### Build from Source

```bash
git clone https://github.com/shridarpatil/whatomate.git
cd whatomate

# Production build (single binary with embedded frontend)
make build-prod
./whatomate server -migrate
```

See [configuration docs](https://shridarpatil.github.io/whatomate/getting-started/configuration/) for detailed setup options.

Any `config.toml` key can be overridden with an environment variable named `WHATOMATE_<SECTION>__<KEY>`, using a **double underscore** between levels (single underscores stay part of the name):

```bash
export WHATOMATE_DATABASE__HOST=db.internal
export WHATOMATE_WHATSAPP__APP_ID=123456789
export WHATOMATE_DEFAULT_ADMIN__EMAIL=admin@example.com
```

The older single-underscore form (`WHATOMATE_DATABASE_HOST`) is still accepted for every known key so existing deployments keep their settings; a deprecation warning is printed at startup, and the `__` form wins when both are set. Rename to the `__` form when convenient.

Rollbar can be enabled either in `config.toml` under `[rollbar]` or with plain environment variables:

```bash
export ROLLBAR_ACCESS_TOKEN=your_post_server_item_token
export ROLLBAR_CLIENT_ACCESS_TOKEN=your_post_client_item_token
export ROLLBAR_ENVIRONMENT=production
```

- `ROLLBAR_ACCESS_TOKEN` reports backend panics and API request failures.
- `ROLLBAR_CLIENT_ACCESS_TOKEN` is optional and enables browser-side Vue/Axios error reporting in the embedded frontend.
- For standalone frontend development, the browser token can also be set through `frontend/.env` using `VITE_ROLLBAR_ACCESS_TOKEN`.

## CLI Usage

```bash
./whatomate server              # API + 1 worker (default)
./whatomate server -workers=0   # API only
./whatomate worker -workers=4   # Workers only (for scaling)
./whatomate version             # Show version
```

## Developers

The backend is written in Go ([Fastglue](https://github.com/zerodha/fastglue)) and the frontend is Vue.js 3 with shadcn-vue.
- If you are interested in contributing, please read [CONTRIBUTING.md](./CONTRIBUTING.md) first.
- External integration docs:
  [External Message Persistence API](./EXTERNAL_MESSAGE_API.md) and [External Template Send API](./EXTERNAL_TEMPLATE_SEND_API.md)
- Claude / MCP connector (set up and use Whatomate from Claude): [MCP_CONNECTOR.md](./MCP_CONNECTOR.md)

```bash
# Development setup
make run-migrate    # Backend (port 8080)
cd frontend && npm run dev   # Frontend (port 3000)
```

## License

See [LICENSE](LICENSE) for details.
