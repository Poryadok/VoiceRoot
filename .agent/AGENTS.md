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
| Repositories, monorepo, protos | `docs/REPOSITORIES.md` |
| Existing code vs planned work | `docs/PLAN.md` |
| Documentation gaps | `docs/TODO.md` |

## Development Workflow

- Before coding, read the relevant `docs/` files from the table and the user task.
- For substantial or ambiguous work, write and maintain an ExecPlan following `.agent/PLANS.md`. Plans must be self-contained, tied to repository documentation, and updated as work progresses.
- Prefer TDD when implementing behavior defined in docs: behavior tests from docs, minimal implementation, relevant tests green, then refactor.
- For a failing or disputed test, first compare the expected behavior with the documentation. Do not weaken a test just to match incorrect code.
- Change test expectations only when the specification in `docs/` was updated or the user task explicitly changes the requirement.
- If the docs have a gap, use `docs/TODO.md` or ask the user. Do not add product behavior without a repository-backed source.
- For the full TDD cycle, exceptions, and source priority, see `docs/TESTING.md`, especially the "Порядок разработки (TDD)" section.

## Architectural Boundaries

- Plan vs code: some services in the repository are stubs or old names. Before judging readiness or estimating scope, read `docs/PLAN.md`. The target service map is in `docs/MICROSERVICES.md`.
- WebSocket delivery belongs to Realtime Service, not Messaging. The event stream uses sequence `s` and `resume`.
- Loading missed messages belongs to REST/API Messaging through Gateway with a cursor per `chat_id`; do not implement this as "catch up everything over WS".
- Auth target architecture is Java (`VoiceAuthService`). Most other documented services are Go. The client is Flutter, including web. Do not move Auth to Go without an explicit user decision.
- Default branch is `master`, not `main`. GitHub repository names use PascalCase. See `docs/REPOSITORIES.md` for monorepo and proto rules.
- Do not expand `docs/` or rewrite the feature structure unless the user explicitly asks. This file should provide navigation and boundaries, not duplicate long specifications.
