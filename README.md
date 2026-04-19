# ClawMachine

**Give any AI a homelab.** MCP server for [WAGMIOS](https://github.com/mentholmike/wagmios) — exposes Docker management as Model Context Protocol tools.

Works with Claude, ChatGPT, Cursor, VS Code Copilot, and any MCP-compatible client.

## How It Works

```
AI Client (Claude, ChatGPT, Cursor, etc.)
        │
        ▼ MCP Protocol (stdio / SSE)
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

### Single Instance (stdio)

```bash
clawmachine -api-url http://localhost:5179 -api-key wag_live_yourkey
```

### Single Instance (SSE / remote)

```bash
clawmachine -api-url http://localhost:5179 -api-key wag_live_yourkey \
  -transport sse -sse-addr :8080 -sse-base-url http://localhost:8080
```

### Multi-Instance

Manage multiple machines from one MCP server:

```bash
clawmachine -config config.json -transport sse -sse-addr :8080 -sse-base-url http://localhost:8080
```

Config file (`config.json`):
```json
{
  "instances": {
    "nas": {
      "url": "http://192.168.1.10:5179",
      "key": "wag_live_xxxxxxxxxxxx",
      "label": "Homelab NAS"
    },
    "vps": {
      "url": "https://vps.example.com:5179",
      "key": "wag_live_yyyyyyyyyyyy",
      "label": "VPS"
    }
  }
}
```

In multi-instance mode, every tool gets a `host` parameter to route to the right machine. `list_hosts` shows all configured instances with their scopes.

### Environment Variables

Flags can also be set via env vars:
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

### Multi-Instance

In multi-instance mode, all tools gain a `host` parameter plus an extra `list_hosts` tool:
- `list_hosts` — show all configured instances with labels and scopes
- Every other tool routes to the specified host and validates its scopes

### Example: Install Jellyfin

```
User: "Install Jellyfin on my homelab"

→ browse_marketplace          (find jellyfin app_id)
→ install_app(app_id="jellyfin")
→ start_app(app_id="jellyfin", container_name="jellyfin", compose_path="...")
✓ Jellyfin is installed and running on port 8096
```

### Example: Multi-Machine

```
User: "Restart Nginx on the NAS and check images on the VPS"

→ restart_container(host="nas", id="nginx-proxy")
→ list_images(host="vps")
✓ Nginx restarted on NAS. VPS has 5 images.
```

## Development

```bash
git clone https://github.com/mentholmike/clawmachine.git
cd clawmachine
go build ./cmd/clawmachine/
go test ./...
```

## Architecture

```
cmd/clawmachine/main.go    Entry point, flag parsing
internal/config/           Configuration (single + multi-instance)
internal/wagmios/          WAGMIOS REST API client
internal/mcp/              MCP server (single + multi-instance)
  server.go                 Single-instance MCP server
  multi.go                  Multi-instance MCP server
```

## License

MIT

## Related

- [WAGMIOS](https://github.com/mentholmike/wagmios) — Self-hosted Docker management platform
- [mcp-go](https://github.com/mark3labs/mcp-go) — Go MCP SDK