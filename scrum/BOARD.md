# ClawMachine Sprint Board

## Phase 1 — Core MCP Server ✅ COMPLETE

| ID | Task | Status | Notes |
|---|---|---|---|
| SCRUM-001 | Initialize Go module & project structure | ✅ Done | go.mod, cmd/clawmachine, internal/{mcp,wagmios,config} |
| SCRUM-002 | MCP server foundation (stdio transport, tool registration, handshake) | ✅ Done | server.NewMCPServer, registerTools, ServeStdio + SSE |
| SCRUM-003 | `check_scopes` MCP tool | ✅ Done | Maps to GET /api/auth/status |
| SCRUM-004 | Container MCP tools (list, start, stop, restart, logs, config, create, delete) | ✅ Done | 8 tools, scope-gated |
| SCRUM-005 | Image MCP tools (list, pull, delete) | ✅ Done | 3 tools, scope-gated |
| SCRUM-006 | Marketplace MCP tools (browse, details, installed, create, start) | ✅ Done | 5 tools, scope-gated |
| SCRUM-007 | Scope-aware tool filtering | ✅ Done | Only exposes tools the API key has permissions for |
| SCRUM-010 | System tools (info, metrics) | ✅ Done | 2 tools, available to any key |

## Phase 2 — Remote & Multi-Machine

| ID | Task | Status | Notes |
|---|---|---|---|
| SCRUM-008 | SSE transport for remote MCP clients | 📋 Todo | NewSSEServer wired, needs integration testing |
| SCRUM-009 | Multi-machine routing | 📋 Todo | host param or per-instance config |

## Summary

- **18 MCP tools** implemented, scope-gated
- **stdio + SSE** transport support
- **Go 1.25**, mark3labs/mcp-go v0.48.0
- **Repo:** https://github.com/mentholmike/clawmachine
- **Commit:** 0284683