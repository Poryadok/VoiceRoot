# ExecPlan: T064 Role Reorder Hierarchy — GREEN

## Purpose

Make `RoleService.ReorderRoles` enforce the documented role hierarchy without
changing edit, assignment, or event policy. An Owner keeps the existing ability
to reorder any supplied role list; a non-Owner with `SPACE_MANAGE_ROLES` may
only reorder targets and destinations strictly below their highest role.

## Context

- Docs: `docs/microservices/role-service.md` defines position as hierarchy and
  says `SPACE_MANAGE_ROLES` applies only to roles below the actor. The same
  hierarchy is confirmed in `docs/features/roles.md`.
- RED evidence: `.agent/execplans/t064-role-reorder-hierarchy-red.md` and the
  accepted gRPC integration cases in
  `src/backend/role/internal/grpcsvc/roles_manage_integration_test.go`.
- Code: `ReorderRoles` currently skips `CanEditRole` for managed roles, while
  `RoleStore.ReorderRoles` is the only method that begins a transaction.
- Constraint: lower managed-role policy is undocumented; T064 must not decide
  it. Do not change `CanEditRole`, Assign/Revoke, protobufs, migrations,
  ordering-list semantics, or events.

## Scope

- In: a reorder-specific authorization check that recognises Owner, derives a
  non-Owner actor's top position, checks every existing target position, and
  checks every resulting store position before the store call.
- Out: duplicate validation, full-list expansion, lower managed-role policy,
  role-edit policy, T063 assignment/revocation work, and event publication.

## Milestones

- [x] Confirm RED tests and governing documentation.
- [x] Add the smallest dedicated reorder authorization implementation.
- [x] Run cached focused checks and inspect the final diff.
- [x] Obtain an independent read-only implementation review.
- [x] Commit the GREEN change locally; do not push.

## Detailed Steps

1. Keep `requireManageRoles` as the permission gate; retain its Owner bypass
   through the existing effective-mask rules.
2. Before any store transaction, parse every requested UUID and fetch every
   target, rejecting malformed or cross-space IDs exactly as the RED tests
   require.
3. Use a reorder-specific check, rather than changing `CanEditRole`, to let an
   Owner pass unchanged. For a non-Owner, reject any target whose current
   position is at or above the actor's top position and reject any list index
   whose resulting `len(ids)-1-index` position is at or above that top.
4. Call `RoleStore.ReorderRoles` only after all validation succeeds. Do not
   mutate the request list, add event emission, or decide lower managed-role
   behaviour.
5. Run `gofmt`, cached `go test` focused on `TestReorderRoles_`, package
   compilation with `-run '^$'`, and `git diff --check`. If the database
   integration harness cannot run without Docker/network, record that as an
   environment limitation rather than changing test scope.

## Validation

- [x] `GOPROXY=off go test -short ./internal/grpcsvc -count=1` passes.
- [x] `GOPROXY=off go test ./internal/grpcsvc -run '^$' -count=1` compiles the
  focused package without starting integration tests.
- [x] `gofmt -w src/backend/role/internal/grpcsvc/roles_manage.go` and
  `git diff --check` leave no formatting or whitespace error.
- [x] A fresh reviewer found no hierarchy bypass, partial-write path, or
  out-of-scope policy change.
- [x] `GOPROXY=off go test ./internal/grpcsvc -run '^TestReorderRoles_' -count=1
  -timeout 60s` was attempted. It timed out in Testcontainers while its local
  PostgreSQL container repeatedly lacked `5432/tcp`, before a test assertion
  ran; no Docker or network remediation was attempted.

## Progress

- [x] Documentation, RED tests, store transaction boundary, and existing
  `CanEditRole` semantics inspected.
- [x] GREEN authorization parses and resolves the complete batch, then checks
  all target and resulting positions before the transactional store call.
- [x] Cached short test and no-test compile checks pass; the full focused
  integration run is blocked only by the local Testcontainers/PostgreSQL port
  failure recorded above.
- [x] Fresh independent read-only review found no Critical, High, or Medium
  finding and confirmed no T063 scope, events, or list-semantics change.

## Decisions

- A separate reorder-specific check is required because `CanEditRole` correctly
  rejects all managed roles for update/delete; changing it would expand policy
  outside T064.
- Final positions use the existing store contract: the ID at index `i` receives
  `len(ordered_role_ids)-1-i`. This validates the RED promotion cases without
  changing list semantics or expanding it to every role in the space.
- Lower managed roles receive no special new branch. The checks are position
  based only, preserving the explicitly unresolved policy.

## Risks And Follow-Ups

- The focused tests are PostgreSQL/testcontainers integration tests. Cached
  dependencies alone do not guarantee a reachable Docker daemon or image; any
  such failure is environmental and must be reported separately.
- Duplicate and incomplete reorder-list semantics predate T064 and are outside
  this narrowly approved change.
