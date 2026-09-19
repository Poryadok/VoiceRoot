# Voice Codex Instructions

This file is the Codex-native project entrypoint. The canonical shared agent
instructions live in `.agent/AGENTS.md`; read that file first for any non-trivial
work in this repository.

## Required First Reads

- `.agent/AGENTS.md` for project sources of truth, language, workflow, and
  architectural boundaries.
- `docs/PLAN.md` before judging feature status, selecting a milestone, or
  opening new product WIP. For a fully specified implementation slice, the
  dispatcher supplies the exact milestone section and relevant feature/service
  docs; workers do not load the whole PLAN or fleet chat history by default.
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

- Apply the autonomy, skill precedence, delegation, and verification rules in
  `.agent/AGENTS.md`. Project model setup and official GPT-6 Astra guidance are
  linked from `.agent/codex/README.md`.
- Use `rtk` for every shell command, for example `rtk git status` and
  `rtk go test ./...`, so terminal output is token-optimized before it reaches
  the agent. Use `rtk proxy <command>` only when unfiltered output is required.
- When the `codegraph_explore` MCP tool is available, use it before raw search
  for code structure, callers/callees, and change impact. Keep repository docs
  as the source of product and architecture truth.
- Use repository documentation as the source of product behavior. Do not invent
  missing product or API behavior; ask the user or record a gap in the proper
  `docs/todo/*.md` file.
- Start fleet work with 2–4 PR lifecycle owners. Expand promptly only for a
  ready independent deliverable with a named consumer, non-overlapping write
  scope, recorded role/model/effort/context rationale, and no scarce-resource
  conflict. Do not target a raw worker count; proven useful parallelism may
  exceed four.
- Every fleet brief names one accounting role, model, effort, context source,
  and checkpoint. Use focused briefs and `fork_turns="none"` by default;
  full-history forks need a recorded reason. Astra is not a default fleet model:
  reserve it for named architecture, security, concurrency, or cross-service
  uncertainty and return ordinary implementation to Terra or Luna afterward.
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
- Keep the main checkout on an up-to-date `master`: after remote changes, run
  `git fetch origin` and `git merge origin/master`, never rebase.
- Do not rebase, amend, force-push, bypass hooks, run `git reset --hard`, or use
  history rewrite tools unless the user explicitly requests that exact action.
- Sync with `git fetch origin` and `git merge origin/master`.
- After creating a commit, push it promptly unless the user explicitly asks to
  keep it local.
- Merge PRs with merge commits; do not squash by default.

## Cursor Migration Notes

Cursor-specific rules, skills, agents, MCP setup, and hooks were inventoried and
mapped for Codex in `.agent/codex/`. Project-owned Codex skills live in
`.agent/codex/skills/`, crew profiles in `.agent/codex/agents/`, and portable
MCP notes/examples in `.agent/codex/mcp.md`. Treat `.agent/` as the cross-agent
canon and `.cursor/` as the Cursor adapter layer.

## graphify

This project has a knowledge graph at graphify-out/ with god nodes, community structure, and cross-file relationships.
The local CLI is `C:\\Users\\Sergey\\AppData\\Roaming\\Python\\Python312\\Scripts\\graphify.exe`; use that path when `graphify` is not on `PATH`.

When the user types `/graphify`, use the installed graphify skill or instructions before doing anything else.

Rules:
- For every codebase question and before every code edit, call
  `mcp__codegraph__codegraph_explore` first. It is a deferred Codex tool, so
  call it directly even when it is absent from the initial tool manifest; do
  not inspect `ALL_TOOLS`, run a CLI probe, or use raw search first. Pass the
  concrete symbol, file path, or question and `projectPath = "D:\\Git\\Voice"`.
- If that direct MCP call returns a tool-not-found error, use
  `rtk codegraph explore "<question or symbols>" --path .` for the same
  question. Do not make any other availability checks.
- Use Graphify after CodeGraph only for broad cross-language or documentation
  context: run `graphify query "<question>"`; use `graphify path "<A>" "<B>"`
  for relationships and `graphify explain "<concept>"` for focused concepts.
  These return a scoped subgraph, usually much smaller than GRAPH_REPORT.md or
  raw grep output.
- Dirty graphify-out/ files are expected after hooks or incremental updates; dirty graph files are not a reason to skip graphify. Only skip graphify if the task is about stale or incorrect graph output, or the user explicitly says not to use it.
- If graphify-out/wiki/index.md exists, use it for broad navigation instead of raw source browsing.
- Read graphify-out/GRAPH_REPORT.md only for broad architecture review or when query/path/explain do not surface enough context.
- After modifying code, run `graphify update .` to keep the graph current (AST-only, no API cost).
