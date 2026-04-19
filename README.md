# ClawMachine

**Give any AI a homelab.** MCP server for [WAGMIOS](https://github.com/mentholmike/wagmios) — exposes Docker management as Model Context Protocol tools with scoped API keys.

Works with Claude Code, Cursor, GitHub Copilot, VS Code, Gemini, and any MCP-compatible client.

---

## Why

AI agents that manage Docker need access to the Docker socket — which means full root on the host. One wrong `docker rm` and production data is gone.

ClawMachine solves this by sitting between the AI and Docker, routing all requests through [WAGMIOS](https://github.com/mentholmike/wagmios)'s scoped API:

- **Scope-gated tools** — If the API key doesn't have `containers:delete`, the `delete_container` tool doesn't exist. The AI literally can't call it.
- **Audit trail** — Every API call is tracked per-key in WAGMIOS.
- **Multi-machine** — One MCP server, multiple WAGMIOS instances, each with its own key and scopes.

The AI never touches the Docker socket. The AI never gets sudo. The enforcement is in the key, not the prompt.

---

## How It Works

```
┌──────────────┐     MCP (stdio/SSE)     ┌──────────────┐     REST (scoped key)    ┌──────────────┐     Docker API     ┌────────┐
│  AI Client   │ ───────────────────────→ │  ClawMachine │ ──────────────────────→ │   WAGMIOS    │ ────────────────→ │ Docker │
│ (Claude,etc) │                          │  MCP Server  │                         │  REST API    │                   │ Daemon │
└──────────────┘                          └──────────────┘                         └──────────────┘                   └────────┘
```

1. You install [WAGMIOS](https://github.com/mentholmike/wagmios) on your machine and create a scoped API key
2. You run ClawMachine, pointing it at WAGMIOS with that key
3. ClawMachine reads the key's scopes and only registers tools the key permits
4. Your AI client sees a tailored set of Docker management tools — nothing more

---

## Quick Start

### 1. Install WAGMIOS

```bash
curl -O https://raw.githubusercontent.com/mentholmike/wagmios/main/docker-compose.yaml
docker compose up -d
```

WAGMIOS runs at `http://localhost:5179` (API) and `http://localhost:5174` (UI).

### 2. Create an API Key

In WAGMIOS UI → Settings → Agent Permissions:

- Name: `my-agent`
- Enable: `containers:read`, `containers:write`, `images:read`, `marketplace:read`
- Leave `containers:delete` **off** unless you want the AI to be able to remove containers
- Copy the key: `wag_live_abc123...`

### 3. Install ClawMachine

**Binary (Go):**
```bash
go install github.com/mentholmike/clawmachine/cmd/clawmachine@latest
```

**Docker:**
```bash
docker run -i itzmizzle/clawmachine -api-url http://host.docker.internal:5179 -api-key wag_live_abc123
```

### 4. Configure Your AI Client

**Claude Code** (`~/.claude/claude_desktop_config.json`):
```json
{
  "mcpServers": {
    "clawmachine": {
      "command": "clawmachine",
      "args": ["-api-url", "http://localhost:5179", "-api-key", "wag_live_abc123"]
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
      "args": ["-api-url", "http://localhost:5179", "-api-key", "wag_live_abc123"]
    }
  }
}
```

**Docker MCP Toolkit** (Docker Desktop 4.62+):
```json
{
  "servers": {
    "clawmachine": {
      "command": "docker",
      "args": ["run", "-i", "itzmizzle/clawmachine", "-api-url", "http://host.docker.internal:5179", "-api-key", "wag_live_abc123"],
      "type": "stdio"
    }
  }
}
```

### 5. Use It

```
You: "What containers am I running?"
→ list_containers
   "3 containers: nginx-proxy (running), postgres-db (running), jellyfin (stopped)"

You: "Delete postgres"
→ "I don't have a delete_container tool — your API key is missing the containers:delete scope."

You: "Install Ollama"
→ browse_marketplace → install_app → start_app
   "Ollama is installed and running on port 11434."
```

---

## Tools

Tools are dynamically registered based on API key scopes. If the key doesn't have the required scope, the tool doesn't exist.

| Scope Required | Tools | Description |
|---|---|---|
| *(any key)* | `check_scopes` | Show key label, prefix, and granted scopes |
| *(any key)* | `system_info` | Docker version, API version, OS/arch |
| *(any key)* | `system_metrics` | CPU, memory, disk usage, container counts |
| `containers:read` | `list_containers` | All containers (running + stopped) |
| `containers:read` | `container_logs` | Container log output (configurable tail) |
| `containers:read` | `container_config` | Full container config (env, volumes, ports) |
| `containers:write` | `start_container` | Start a stopped container |
| `containers:write` | `stop_container` | Stop a running container |
| `containers:write` | `restart_container` | Restart a container |
| `containers:write` | `create_container` | Create a new container (image, name, env, ports, volumes) |
| `containers:delete` | `delete_container` | Permanently delete a container (irreversible) |
| `images:read` | `list_images` | All Docker images on the host |
| `images:write` | `pull_image` | Pull an image from a registry |
| `images:write` | `delete_image` | Delete an image (irreversible) |
| `marketplace:read` | `browse_marketplace` | Browse 34+ self-hosted apps |
| `marketplace:read` | `get_marketplace_app` | App details (description, categories, compose) |
| `marketplace:read` | `list_installed_apps` | Apps installed via marketplace |
| `marketplace:write` | `install_app` | Download and install a marketplace app |
| `marketplace:write` | `start_app` | Start an installed app (docker compose up) |

---

## Multi-Instance Mode

Manage multiple machines from one MCP server. Create a config file:

```json
{
  "instances": {
    "nas": {
      "url": "http://192.168.1.10:5179",
      "key": "wag_live_aaa",
      "label": "Homelab NAS"
    },
    "vps": {
      "url": "https://vps.example.com:5179",
      "key": "wag_live_bbb",
      "label": "VPS"
    }
  }
}
```

Run with:
```bash
clawmachine -config instances.json -transport sse -sse-addr :8080
```

In multi-instance mode:
- Every tool gets a `host` parameter (e.g. `host="nas"`)
- A `list_hosts` tool shows all configured instances with labels and scopes
- Each host's key has its own scope restrictions — the NAS key can have `containers:delete` while the VPS key doesn't

```
You: "Restart Nginx on the NAS and check images on the VPS"
→ restart_container(host="nas", id="nginx-proxy")
→ list_images(host="vps")
```

---

## Transport Modes

| Mode | Use Case | Command |
|------|----------|---------|
| **stdio** | Local AI clients (Claude Code, Cursor) | `clawmachine -api-url ... -api-key ...` |
| **SSE** | Remote clients, web-based agents | Add `-transport sse -sse-addr :8080 -sse-base-url http://your-host:8080` |

### Environment Variables

Flags can be set via environment variables:
- `WAGMIOS_API_URL` — WAGMIOS backend URL
- `WAGMIOS_API_KEY` — WAGMIOS API key

---

## Security Model

| | Raw Docker Access | ClawMachine + WAGMIOS |
|---|---|---|
| Permissions | All or nothing | Granular scopes per key |
| Audit trail | Docker daemon logs (noisy) | WAGMIOS activity feed (per-key) |
| Deletion safety | Agent can `docker rm -f` anything | Key must have `containers:delete` scope |
| Multi-tenant | One socket, everyone shares | Separate keys, separate scopes |
| Blast radius | Entire host | Scoped to key permissions |

The enforcement is **in the key, not the prompt**. Even if an AI decides to call `delete_container` without asking, the call fails at the WAGMIOS API level if the key doesn't have the scope.

---

## Architecture

```
cmd/clawmachine/main.go       Entry point, single/multi routing
internal/config/
  config.go                    Single-instance config
  multi.go                     Multi-instance config loader
internal/wagmios/
  client.go                    WAGMIOS REST API client
internal/mcp/
  server.go                    Single-instance MCP server (18 tools)
  multi.go                     Multi-instance MCP server (19 tools)
```

**Stack:** Go 1.25, [mcp-go](https://github.com/mark3labs/mcp-go) v0.48.0, MCP protocol version 2024-11-05

---

## Docker

```bash
# Build
docker build -t itzmizzle/clawmachine .

# Run (stdio)
docker run -i itzmizzle/clawmachine -api-url http://host.docker.internal:5179 -api-key wag_live_xxx

# Run (SSE)
docker run -p 8080:8080 itzmizzle/clawmachine \
  -api-url http://host.docker.internal:5179 -api-key wag_live_xxx \
  -transport sse -sse-addr :8080 -sse-base-url http://localhost:8080
```

Multi-arch images (amd64 + arm64) are built and pushed to `itzmizzle/clawmachine` on Docker Hub via GitHub Actions on every push to main and on version tags.

---

## Development

```bash
git clone https://github.com/mentholmike/clawmachine.git
cd clawmachine
go build ./cmd/clawmachine/
go vet ./...
```

---

## License

MIT

---

## Related

- [WAGMIOS](https://github.com/mentholmike/wagmios) — Self-hosted Docker management platform
- [mcp-go](https://github.com/mark3labs/mcp-go) — Go MCP SDK
- [Docker MCP Catalog](https://hub.docker.com/mcp) — Curated MCP server catalog