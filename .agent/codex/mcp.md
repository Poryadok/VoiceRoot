# Codex MCP Notes

Cursor global MCP setup was checked on this machine. The portable facts are:

- `codegraph`: stdio server, command `codegraph serve --mcp --path <workspace>`.
- `penpot`: remote MCP stream at `design.penpot.app`; the URL contains a local
  user token and must stay out of git.

## Voice Usage

- Use CodeGraph only as a structure/navigation helper. Repository docs remain
  the source of product and architecture truth.
- CodeGraph queries should avoid generated protobuf noise. Include concrete
  service paths and symbols, for example:
  `src/backend/chat/internal/grpcsvc/list_chats.go EnrichListChats`.
- For Penpot, follow the `penpot-voice` skill. The Voice file id is documented
  there and in `docs/design/penpot-setup.md`.
- If Penpot tools fail with auth or plugin errors, ask the user to connect the
  Penpot MCP server. Do not guess geometry.
- Penpot MCP is fragile on large Voice pages. Use micro-queries: verify
  `currentFile/currentPage`, list only `page.root.children`, then inspect one
  known board id one shallow level at a time. Avoid full recursive walks and
  broad `shapeStructure`/`findShape` calls on whole pages.
- After a 504 or timeout, do not repeat the identical heavy call. Reopen/reload
  the Penpot file, disconnect/connect MCP from the Penpot file menu, keep the
  plugin window open, and restart/reconnect the MCP client if simple reads still
  hang.

## Secret Handling

Do not commit MCP tokens or URLs containing tokens. Local Codex MCP entries live
in `$CODEX_HOME/config.toml` or `~/.codex/config.toml`. Use `mcp.example.toml`
as the portable template.
