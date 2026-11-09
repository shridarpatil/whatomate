# Whatomate MCP Connector (Claude)

Whatomate is a remote [MCP](https://modelcontextprotocol.io) server. Claude can use it to set up and run
Whatomate the same way a person does in the web app: WhatsApp accounts, templates, contacts, chats,
campaigns, chatbot flows, RSVP events, lead ads, catalogs, users, roles, teams, settings and analytics.

Connector URL: `https://<your-whatomate-host>/mcp` (shown in **Settings → AI Connector**).

## What Claude can do

Claude acts **as the person who connected it, inside one organisation, with that person's role**.
It can do exactly what that person can do in the web app — nothing more. Every request goes through
the same permission checks, organisation scoping, validation, campaign delivery guard and audit log as
the browser.

Not available to Claude: logging in, approving its own connection, changing your password, Meta webhooks.

Claude apps ask you before running tools that change data or send messages.

## Connect from claude.ai (or Claude Desktop)

1. In Whatomate open **Settings → AI Connector** and copy the connector URL.
2. In Claude: **Settings → Connectors → Add custom connector**, paste the URL, save, then **Connect**.
3. A Whatomate page opens. Log in if asked, pick the organisation, press **Approve**.
4. Done. To disconnect, press **Revoke** under *Connected AI apps* in Settings → AI Connector.

## Connect from Claude Code

Create an API key in **Settings → API Keys**, then:

```bash
claude mcp add --transport http whatomate https://<your-whatomate-host>/mcp --header "X-API-Key: whm_..."
```

An API key is tied to one organisation; `whatomate_switch_organization` only works for claude.ai (OAuth) connections.

## Tools

| Tool | What it does |
|---|---|
| `whatomate_whoami` | Who Claude is acting as, organisation, role, permissions |
| `whatomate_list_organizations` / `whatomate_switch_organization` | See and change the working organisation |
| `whatomate_list_areas` | Feature areas (contacts, campaigns, RSVP, chatbot, accounts, …) |
| `whatomate_describe_area` | Every action in an area with its inputs |
| `whatomate_call` | Run any catalogued action |
| `whatomate_upload` | Actions that take a file (media, CSV imports, template samples) |
| `whatomate_search_contacts`, `whatomate_send_text`, `whatomate_send_template`, `whatomate_list_campaigns`, … | Shortcuts for everyday work |

## Security model

- OAuth 2.1 with PKCE (S256) and dynamic client registration for claude.ai; API keys for Claude Code.
- Access tokens last 1 hour, refresh tokens 30 days and rotate on every use; reusing an old refresh
  token revokes the connection. Tokens are stored only as SHA-256 hashes.
- A connection stops working immediately when it is revoked, the user is disabled, or the user is removed
  from the organisation.
- Turn the whole connector off with `mcp.enabled = false` (env `WHATOMATE_MCP__ENABLED=false`).

## For developers: adding a route

Every authenticated `/api` route must be in the MCP catalog (`internal/mcp/catalog_<area>.go`) or in
`internal/mcp/excluded.go`. `internal/mcp/catalog_parity_test.go` fails when a route in
`cmd/whatomate/main.go` is in neither. Add an `Entry` with the exact path template, a plain-words summary,
the permission, query params and a JSON-schema body.
