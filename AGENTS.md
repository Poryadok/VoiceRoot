# Voice Codex Instructions

This file is the Codex-native project entrypoint. The canonical shared agent
instructions live in `.agent/AGENTS.md`; read that file first for any non-trivial
work in this repository.

## Required First Reads

- `.agent/AGENTS.md` for project sources of truth, language, workflow, and
  architectural boundaries.
- `docs/PLAN.md` before judging whether a feature is shipped, partial, stub, or
  deferred.
- Relevant `docs/features/*`, `docs/microservices/*`, `docs/DATA_MODEL.md`,
  `docs/DATA_STORES.md`, and `docs/ARCHITECTURE_REQUIREMENTS.md` before coding
  behavior.
- `docs/TESTING.md` and `docs/CONTRIBUTING.md` before verification, commits, or
  PR work.

PowerShell documentation reads should use UTF-8, for example:

```powershell
Get-Content -Raw -Encoding UTF8 .agent\AGENTS.md
```

## Codex Workflow

- Use repository documentation as the source of product behavior. Do not invent
  missing product or API behavior; ask the user or record a gap in the proper
  `docs/todo/*.md` file.
- For substantial, ambiguous, cross-service, or risky work, maintain an ExecPlan
  using `.agent/PLANS.md`.
- If the user explicitly invokes `tdd-code-workflow`, follow the installed Codex
  skill strictly. The repository canonical workflow remains
  `.agent/workflows/tdd-code-workflow/SKILL.md`.
- Keep communication with the user in Russian by default. Use English for code,
  commit messages, command names, identifiers, and API names.

## Hard Boundaries

- Auth is Java/Spring in `src/backend/auth/`; do not port it to Go without an
  explicit user decision.
- Realtime WebSocket event flow belongs to `src/backend/realtime/`.
- Missed message history catch-up belongs to Messaging REST/API via Gateway with
  a cursor per `chat_id`; do not implement global WS catch-up.
- Federation is deferred unless the user explicitly changes scope.
- Node.js for CI/frontend is 24.

## Git Safety

- Default branch is `master`.
- Do not rebase, amend, force-push, bypass hooks, run `git reset --hard`, or use
  history rewrite tools unless the user explicitly requests that exact action.
- Sync with `git fetch origin` and `git merge origin/master`.
- Merge PRs with merge commits; do not squash by default.

## Cursor Migration Notes

Cursor-specific rules, skills, agents, MCP setup, and hooks were inventoried and
mapped for Codex in `.agent/codex/`. Treat `.agent/` as the cross-agent canon and
`.cursor/` as the Cursor adapter layer.
