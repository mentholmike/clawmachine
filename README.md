# ClawMachine

**Give any AI a homelab.** MCP server for [WAGMIOS](https://github.com/mentholmike/wagmios) — exposes WAGMIOS Docker management as Model Context Protocol tools.

Works with Claude, ChatGPT, Cursor, VS Code Copilot, and any MCP-compatible client.

## How It Works

```
AI Client (Claude, ChatGPT, etc.)
        │
        ▼ MCP Protocol (stdio/SSE)
┌─────────────────┐
│   ClawMachine   │  ← This repo
│   MCP Server    │
└────────┬────────┘
         │ REST API (scoped key)
         ▼
┌─────────────────┐
│    WAGMIOS      │  ← Your homelab
│   REST API      │
└─────────────────┘
```

ClawMachine connects to a WAGMIOS instance and exposes its REST API as MCP tools. The API key's scopes control which tools appear — if the key doesn't have `containers:delete`, the `delete_container` tool won't exist.

## Install

```bash
go install github.com/mentholmike/clawmachine/cmd/clawmachine@latest
```

## Usage

### stdio (local clients like Claude Code, Cursor)

```bash
clawmachine -api-url http://localhost:5179 -api-key wag_live_yourkey
```

### SSE (remote clients)

```bash
clawmachine -api-url http://localhost:5179 -api-key wag_live_yourkey -transport sse -sse-addr :8080
```

### Environment Variables

Flags can also be set via environment variables:

- `WAGMIOS_API_URL` — WAGMIOS backend URL
- `WAGMIOS_API_KEY` — WAGMIOS API key

### Client Configuration

**Claude Code** (`~/.claude/claude_desktop_config.json`):
```json
{
  "mcpServers": {
    "clawmachine": {
      "command": "clawmachine",
      "args": ["-api-url", "http://localhost:5179", "-api-key", "wag_live_yourkey"]
    }
  }
}
```

**Cursor** (`.cursor/mcp.json`):
```json
{
  "mcpServers": {
    "clawmachine": {
      "command": "clawmachine",
      "args": ["-api-url", "http://localhost:5179", "-api-key", "wag_live_yourkey"]
    }
  }
}
```

## Tools

Tools are dynamically registered based on your API key's scopes:

| Scope | Tools |
|-------|-------|
| any | `check_scopes`, `system_info`, `system_metrics` |
| `containers:read` | `list_containers`, `container_logs`, `container_config` |
| `containers:write` | `start_container`, `stop_container`, `restart_container`, `create_container` |
| `containers:delete` | `delete_container` |
| `images:read` | `list_images` |
| `images:write` | `pull_image`, `delete_image` |
| `marketplace:read` | `browse_marketplace`, `get_marketplace_app`, `list_installed_apps` |
| `marketplace:write` | `install_app`, `start_app` |

### Example: Install Jellyfin

```
User: "Install Jellyfin on my homelab"

→ browse_marketplace          (find jellyfin app_id)
→ install_app(app_id="jellyfin")
→ start_app(app_id="jellyfin", container_name="jellyfin", compose_path="...")
✓ Jellyfin is installed and running on port 8096
```

## Multi-Machine

Point ClawMachine at different WAGMIOS instances for different machines:

```bash
# NAS
clawmachine -api-url http://192.168.1.10:5179 -api-key $NAS_KEY

# VPS
clawmachine -api-url https://vps.example.com:5179 -api-key $VPS_KEY
```

Each instance gets its own tools based on that machine's key scopes.

## Development

```bash
git clone https://github.com/mentholmike/clawmachine.git
cd clawmachine
go build ./cmd/clawmachine/
```

## License

MIT

## Related

- [WAGMIOS](https://github.com/mentholmike/wagmios) — Self-hosted Docker management platform
- [mcp-go](https://github.com/mark3labs/mcp-go) — Go MCP SDK