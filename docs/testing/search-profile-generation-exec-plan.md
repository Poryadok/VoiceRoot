# ExecPlan: Search profile projection generations

## Purpose

Allow Search to rebuild its User-authoritative profile projection in an isolated
generation, atomically promote or roll back the serving route, and keep both
the active and rollback generations current after promotion.

## Context

- Docs: `docs/microservices/search-service.md`, `docs/PLAN.md`,
  `docs/DEPLOYMENT.md`, `docs/TESTING.md`, and `docs/CONTRIBUTING.md`.
- Base implementation: merged PR #428 (`b79cf33f`), which owns the frozen v1
  User snapshot/journal/checkpoint transport and revisioned Search projection.
- Constraints: Search owns only its database/configuration. User RPCs,
  protobufs, generated clients, and the normal v1 principal contract remain
  unchanged. No public activation endpoint is introduced.

## Scope

- In: Search migration `000008`, generation-aware profile projection storage,
  bootstrap/retry/runtime wiring, route-aware profile reads, desired-generation
  deployment configuration, and focused tests.
- Out: User/proto changes, raw-key fallback, public activation, and rebuilding
  message/chat/space indexes.

## Milestones

- [ ] Add failing tests for route validation, atomic promotion/rollback, and
  generation-aware profile persistence.
- [ ] Add expand-first schema seeded with generation 1 and a durable route.
- [ ] Make snapshot/replay write a specified generation and use a PostgreSQL
  advisory lock to admit only one desired-generation rebuilder.
- [ ] Promote only a ready generation under the route lock; retain the prior
  active generation as rollback and dual-apply journal events to both.
- [ ] Route profile reads only to the active generation and fail closed for an
  invalid/unavailable route.
- [ ] Wire optional `SEARCH_USER_PROJECTION_DESIRED_GENERATION` in Compose,
  staging, and production; update this plan and graph data.

## Validation

- [ ] Focused Search unit tests demonstrate RED then green route invariants.
- [ ] Hosted CI runs Search package and migration/integration coverage on the
  exact pushed head (Windows is intentionally not used for Go/Compose/race).

## Progress

- [x] Lease `833b8383558f318f735c52d22a9264ca` acquired at
  `D:\\Git\\Voice\\.treehouse\\Voice-d5628e\\3\\Voice`.
- [x] Branch `feature/search-generation-rebuild` created at
  `b79cf33f935affdf0b0bde17ded6b5c8a9bf3445`.
- [x] Read the governing docs and #428 implementation.
- [ ] RED tests and implementation.

## Decisions

- Existing data is generation 1, preserving deploy compatibility.
- A durable route, not an environment value, selects serving data; environment
  only requests one rebuild target. This makes an unset desired value continue
  serving the current active generation.

## Risks And Follow-Ups

- A completed hosted CI run is required for integration proof because local
  Windows Go/Compose/race execution is out of scope for this task.
