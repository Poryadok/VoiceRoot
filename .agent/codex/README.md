# Codex Migration Map

This directory records the Cursor-to-Codex migration for Voice agent context.
It is intentionally small: `.agent/AGENTS.md` remains the shared source of truth,
and root `AGENTS.md` is the Codex-native entrypoint.

## Migrated Cursor Project Context

| Cursor source | Codex target |
| --- | --- |
| `.cursor/rules/voice-project.mdc` | root `AGENTS.md` + `.agent/AGENTS.md` |
| `.cursor/rules/git-safety.mdc` | root `AGENTS.md`, `docs/CONTRIBUTING.md`, installed git hooks |
| `.cursor/rules/debug-backend-logs.mdc` | `.agent/AGENTS.md`, `docs/TESTING.md` |
| `.cursor/rules/codegraph.mdc` | `mcp.md` and local Codex MCP config |
| `.cursor/rules/voice-design.mdc` | `docs/design/*`, copied/adapted Codex skills |
| `.cursor/rules/voice-fleet-captain.mdc` | `crew-profiles.md`, `.agent/fleet/*`, local `tmp/fleet/*` state |
| `.cursor/skills/*` | copied/adapted into `C:\Users\Sergey\.codex\skills` |
| `.cursor/agents/voice-*.md` | summarized in `crew-profiles.md` |
| `~/.cursor/mcp.json` | local Codex `config.toml` MCP entries; secrets stay local |
| `~/.cursor/hooks.json` and `~/.cursor/permissions.json` | repo git hooks plus Codex/developer git safety rules |

## Non-Portable Cursor State

- VS Code UI preferences from Cursor (`theme`, keybinding `ctrl+i`, HTTP/2 UI
  toggle) are editor behavior, not Codex agent context.
- Penpot MCP credentials are local secrets and must not be committed.
- Cursor extension caches under `~/.cursor/extensions` are not agent rules.

## Native Codex Shape

- Project entrypoint: `AGENTS.md`.
- Shared agent canon: `.agent/AGENTS.md`.
- Living plans: `.agent/execplans/` using `.agent/PLANS.md`.
- Fleet docs/templates: `.agent/fleet/`.
- Live fleet state: `tmp/fleet/` (ignored; do not commit).
- Reusable Codex skills: `C:\Users\Sergey\.codex\skills`.
- Local Codex MCP config: `C:\Users\Sergey\.codex\config.toml`.
