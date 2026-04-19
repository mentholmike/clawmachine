# ClawMachine Sprint Board

## Phase 1 — Core MCP Server

| ID | Task | Status | Notes |
|---|---|---|---|
| SCRUM-001 | Initialize Go module & project structure | ✅ Done | go.mod, cmd/clawmachine, internal/{mcp,wagmios,config,tools} |
| SCRUM-002 | MCP server foundation (stdio transport, tool registration, handshake) | ✅ Done | server.NewMCPServer, registerTools, ServeStdio/SSE |
| SCRUM-003 | `check_scopes` MCP tool | ✅ Done | Maps to GET /api/auth/status |
| SCRUM-004 | Container MCP tools (list, start, stop, restart, logs, config, create, delete) | ✅ Done | 8 tools, scope-gated |
| SCRUM-005 | Image MCP tools (list, pull, delete) | ✅ Done | 3 tools, scope-gated |
| SCRUM-006 | Marketplace MCP tools (browse, details, installed, create, start) | ✅ Done | 5 tools, scope-gated |
| SCRUM-007 | Scope-aware tool filtering | ✅ Done | Only exposes tools the API key has permissions for |
| SCRUM-010 | System & key management tools | ✅ Done | system_info, system_metrics |

## Phase 2 — Remote & Multi-Machine

| ID | Task | Status | Notes |
|---|---|---|---|
| SCRUM-008 | SSE transport for remote MCP clients | 📋 Todo | server.NewSSEServer wired, needs testing |
| SCRUM-009 | Multi-machine routing | 📋 Todo | host param or multi-server config |

## Build Issues (Active)

- [ ] Fix `req.Params.Arguments` type assertion — `Arguments` is `any`, need `.(map[string]interface{})` cast before indexing. Three handlers have broken brace nesting from the edit.