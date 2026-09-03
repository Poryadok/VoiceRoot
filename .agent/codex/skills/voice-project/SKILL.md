---
name: voice-project
description: 'Voice monorepo project context for Codex: sources of truth, architecture boundaries, Cursor migration map, git safety, fleet/worktree conventions, and local verification. Use for any substantial work in D:\Git\Voice.'
---

# Voice Project

Use this skill for substantial work in `D:\Git\Voice`. The repository-native Codex entrypoint is `AGENTS.md`; the cross-agent canon is `.agent/AGENTS.md`.

## First Reads

1. Read `AGENTS.md` in the repository root.
2. Read `.agent/AGENTS.md` completely.
3. Read `docs/PLAN.md` before judging readiness or implementation status.
4. Read only the relevant feature/service docs for the current task.

Use UTF-8 when reading Markdown from PowerShell:

```powershell
Get-Content -Raw -Encoding UTF8 .agent\AGENTS.md
```

## Key Boundaries

- Product behavior comes from repository docs only. Do not invent missing behavior.
- Auth is Java/Spring in `src/backend/auth/`; do not port it to Go without explicit user direction.
- Realtime WebSocket flow belongs to `src/backend/realtime/`.
- Missed message history catch-up belongs to Messaging REST/API through Gateway with a cursor per `chat_id`.
- Federation is deferred unless the user explicitly changes scope.
- Node.js for CI/frontend is 24.

## Migrated Cursor Context

- Cursor rules and agents are mapped in `.agent/codex/`.
- Cursor skills were copied into Codex skills where useful.
- Cursor global MCP setup was migrated locally for CodeGraph and Penpot; tokens stay local and must not be committed.

## Related Codex Skills

- `tdd-code-workflow` for strict docs-first TDD when explicitly invoked or required by the task.
- `voice-git-workflow` for branch, commit, PR, merge, and no-history-rewrite rules.
- `voice-project-full-verification` for whole-repo sign-off.
- `go-microservice-task-evaluation`, `java-microservice-task-evaluation`, and `flutter-web-client-testing` for focused verification.
- `penpot-voice` and `penpot-mockup-audit` for design MCP work.
- `voice-fleet-captain` and `treehouse` for parallel worktree orchestration.

