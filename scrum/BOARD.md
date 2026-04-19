# ClawMachine Sprint Board

## Phase 1 — Core MCP Server ✅ COMPLETE

| ID | Task | Status | Notes |
|---|---|---|---|
| SCRUM-001 | Initialize Go module & project structure | ✅ Done | go.mod, cmd/clawmachine, internal/{mcp,wagmios,config} |
| SCRUM-002 | MCP server foundation (stdio transport, tool registration, handshake) | ✅ Done | server.NewMCPServer, registerTools, ServeStdio |
| SCRUM-003 | `check_scopes` MCP tool | ✅ Done | Maps to GET /api/auth/status |
| SCRUM-004 | Container MCP tools (8 tools) | ✅ Done | list, logs, config, start, stop, restart, create, delete |
| SCRUM-005 | Image MCP tools (3 tools) | ✅ Done | list, pull, delete |
| SCRUM-006 | Marketplace MCP tools (5 tools) | ✅ Done | browse, details, installed, install, start |
| SCRUM-007 | Scope-aware tool filtering | ✅ Done | Only exposes tools the API key permits |
| SCRUM-010 | System tools (info, metrics) | ✅ Done | 2 tools, available to any key |

## Phase 2 — Remote & Multi-Machine ✅ COMPLETE

| ID | Task | Status | Notes |
|---|---|---|---|
| SCRUM-008 | SSE transport for remote MCP clients | ✅ Done | WithBaseURL, KeepAlive, configurable addr |
| SCRUM-009 | Multi-machine routing | ✅ Done | -config flag, MultiServer, host param on all tools, list_hosts tool |

## Summary

- **18 single-instance tools** + **19 multi-instance tools** (18 + list_hosts)
- **stdio + SSE** transport with KeepAlive
- **Multi-instance** mode via JSON config
- **Scope-gated** — only tools the key permits are registered
- **Go 1.25**, mcp-go v0.48.0
- **Builds clean**, go vet passes