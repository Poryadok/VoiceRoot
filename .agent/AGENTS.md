# Voice Project Instructions

Voice is a Discord-like messenger with voice chat and built-in matchmaking. Product and feature details must come from this repository only. Do not invent missing behavior; if the docs do not define something, ask the user or record the gap where appropriate.

## Sources Of Truth

| Need | File |
| --- | --- |
| Vision, audience, matchmaking | `docs/PROJECT.md` |
| Feature catalog and details | `docs/FEATURES.md`, `docs/features/` |
| Glossary: chat, channel, space, account, etc. | `docs/GLOSSARY.md` |
| Services, boundaries, contracts | `docs/MICROSERVICES.md`, `docs/microservices/*.md` |
| Database and Redis inventory by service | `docs/DATA_STORES.md` |
| Data model rules: IDs, cross-service references, common fields | `docs/DATA_MODEL.md` |
| Cross-cutting technical requirements: JWT, rate limits, WS/reconnect, push, voice | `docs/ARCHITECTURE_REQUIREMENTS.md` |
| SLO, degradation, releases, DB migrations | `docs/OPERATIONS.md` |
| Environments, deployment, CI to staging/prod | `docs/DEPLOYMENT.md` |
| Tests, local checks, CI composition | `docs/TESTING.md` |
| Git, branches, PR, process | `docs/CONTRIBUTING.md` |
| Local machine setup (toolchain, hooks, compose) | `docs/DEV_SETUP.md` |
| Repositories, monorepo, protos | `docs/REPOSITORIES.md` |
| Existing code vs planned work | `docs/PLAN.md` (implementation status) |
| Documentation gaps | `docs/TODO.md` |

## File Reading

- Russian documentation is UTF-8. In PowerShell, read Markdown docs with `Get-Content -Raw -Encoding UTF8 <path>`.
- This applies to `.agent/*.md`, `docs/**/*.md`, root `README.md`, and service `README.md` files.
- Avoid plain `Get-Content <path>` for documentation; it may render Cyrillic as mojibake and corrupt understanding.

## Fleet (Multitask / parallel agents)

- Lite fleet workflow: `.agent/fleet/README.md`, local backlog `tmp/fleet/backlog.md`, captain skill `.agent/codex/skills/voice-fleet-captain/SKILL.md`, rule `.agent/codex/rules/voice-fleet-captain.mdc`.
- Crew agents: `.agent/codex/agents/voice-*.md` (gateway, realtime, chat/messaging, go-backend, java-auth, flutter, protos, verify, designer). Cursor mirrors live in `.cursor/agents/`.
- Parallel **code** isolation: treehouse (`treehouse.toml`, skill `.agent/codex/skills/treehouse/SKILL.md`) — one crew ↔ one worktree path; not mixed with Cursor `/worktree` on the same task.
- Delegate bounded, independent work when it improves quality or saves time,
  including review within one service. Read-only reviewers may inspect the
  working checkout; parallel code writers need separate worktrees.
- In Codex, use subagent tools for internal work. Create a user-visible task only
  when the user explicitly requests one. Keep inter-agent messages readable.

## Development Workflow

- Before coding, read the relevant `docs/` files from the table and the user task.
- Keep the main checkout on current `master`: `git fetch origin` + `git merge origin/master` (no rebase). After creating a commit, push it promptly unless the user explicitly asks to keep it local.
- Shared workflows live in `.agent/workflows/`. For full documentation-first TDD workflow, use `.agent/codex/skills/tdd-code-workflow/SKILL.md` (mirrors `.agent/workflows/tdd-code-workflow/SKILL.md` for repo-local portability). Cursor loads a stub from `.cursor/skills/tdd-code-workflow/SKILL.md`.
- Product design (Penpot mocks, tokens, UX review): Codex profile `.agent/codex/agents/voice-designer.md` and skill `.agent/codex/skills/penpot-voice/SKILL.md`. Visual canon is `docs/design/` + `design/tokens/voice.tokens.json`; do not invent product behavior.
- If the user **explicitly** asks to use that TDD workflow / skill, follow the canonical `SKILL.md` **strictly** (written plan before production code, delegation where the tool supports it, red–green–refactor, review loops, final checklist) — see «Strict mode» in that file and «Явный запрос скилла» in `docs/TESTING.md`.
- For substantial or ambiguous work, write and maintain an ExecPlan following `.agent/PLANS.md`. Plans must be self-contained, tied to repository documentation, and updated as work progresses.
- Prefer TDD when implementing behavior defined in docs: behavior tests from docs, minimal implementation, relevant tests green, then refactor.
- For a failing or disputed test, first compare the expected behavior with the documentation. Do not weaken a test just to match incorrect code.
- Change test expectations only when the specification in `docs/` was updated or the user task explicitly changes the requirement.
- If the docs have a gap, use `docs/TODO.md` or ask the user. Do not add product behavior without a repository-backed source.
- For the full TDD cycle, exceptions, and source priority, see `docs/TESTING.md`, especially the "Порядок разработки (TDD)" section.

## Autonomy And Skill Precedence

- Carry action requests through implementation and verification within their
  authorized scope. Resolve routine technical choices from context; preserve
  earlier authorization and continue independent work while awaiting needed input.
- Explicit user instructions override skill guidelines. Apply `docs/TESTING.md`
  scope and exceptions without adding an approval step. An explicit product
  requirement change authorizes matching docs, plan, test, and code updates;
  unresolved product decisions still require input or a documented gap.
- When a skill requires a pause, identify the exact `SKILL.md` and quote the
  relevant rule with a link. Explain the actual unresolved decision. Complete
  available preparation before requesting any necessary final approval.
- Treat follow-up corrections as steering the active task. A status question
  does not cancel it; carry progress and remaining work across context compaction.
- Run required checks for the affected scope. Add meaningful behavior tests;
  documentation or mechanical edits do not need tests that only mirror wording.
  Broaden or repeat passing checks only after changes, failures, or new concerns.

## Architectural Boundaries

- Plan vs code: some services in the repository are stubs or old names. Before judging readiness or estimating scope, read `docs/PLAN.md`. The target service map is in `docs/MICROSERVICES.md`.
- WebSocket delivery belongs to Realtime Service, not Messaging. The event stream uses sequence `s` and `resume`.
- Loading missed messages belongs to REST/API Messaging through Gateway with a cursor per `chat_id`; do not implement this as "catch up everything over WS".
- Auth target architecture is Java (`VoiceAuthService`). Most other documented services are Go. The client is Flutter, including web. Do not move Auth to Go without an explicit user decision.
- Default branch is `master`, not `main`. GitHub repository names use PascalCase. See `docs/REPOSITORIES.md` for monorepo and proto rules.
- Do not expand `docs/` or rewrite the feature structure unless the user explicitly asks. This file should provide navigation and boundaries, not duplicate long specifications.

## Language And Token Use

- Communicate with the user in Russian by default.
- Use English for code identifiers, commands, API names, error messages, commit messages, and standard engineering terms when translation would reduce precision.
- Keep internal planning notes and implementation terminology concise; prefer English technical wording where practical.
- Be brief in routine status updates and final answers. Skip filler, greetings, and generic preambles.
- Lead with the result, why it matters, verification, and material limitations.
  Prefer clear paragraphs; use lists for steps or parallel items, without stock
  phrases or unnecessary headings.
- `caveman`, `brief`, or `low-token` means terse engineering mode, not weaker reasoning.
- In low-token mode, keep output compact and use `Changed`, `Checked`, and `Risk` when reporting code work.
- For architecture, security, migrations, production incidents, and code reviews, prefer concise but clear explanations over ultra-compressed answers.
- Keep agent memory and rule files compressed: short rules, exact conditions, no repeated policy prose.
