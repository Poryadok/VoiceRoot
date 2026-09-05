# ExecPlan: T064 Role Reorder Hierarchy — RED

## Purpose

Produce a reviewable RED-only commit that demonstrates the current `RoleService.ReorderRoles` hierarchy bypass. The future implementation must prevent a non-owner role manager from reordering Owner, Admin, or any role at/equal-to/above the actor's top position, or from placing a target at/equal-to/above that top position. Owner full reordering remains supported.

## Context

- Docs: `docs/microservices/role-service.md` defines the Owner → Admin → Moderator → Member → Guest hierarchy and says `SPACE_MANAGE_ROLES` applies only below the actor's position. `docs/features/roles.md` confirms hierarchy affects role actions. `docs/todo/backend.md` records that `ReorderRoles` skips hierarchy for system/managed roles.
- Code: `src/backend/role/internal/grpcsvc/roles_manage.go` authorizes only non-managed targets through `CanEditRole`; `src/backend/role/internal/store/roles.go` applies positions transactionally. Existing gRPC integration tests are in `src/backend/role/internal/grpcsvc/roles_custom_integration_test.go` and `roles_manage_integration_test.go`.
- Existing regression: `TestReorderRoles_UpdatesOrder` proves an Owner can reorder the complete role list, including managed roles. It must remain green once implementation begins.
- Constraints: T064 explicitly requires a RED-only turn. Do not change production code, protobufs, migrations, generated code, or role policy outside indisputable hierarchy violations. No Docker or network commands.

## Scope

- In: an ExecPlan and focused role gRPC integration tests for the documented/explicit T064 hierarchy rules; exact error-code and persistent-position assertions; owner regression reuse.
- Out: implementation; any decision on lower managed roles; API/schema changes; event behaviour; reordering semantics beyond the requested prohibitions.
- Documentation gap: the docs call `SPACE_MANAGE_ROLES` a policy for roles below the actor but do not define whether a non-owner may reorder lower managed roles. T064 tests must neither authorize nor deny that case.

## Milestones

- [x] Confirm clean WT5 at `a3b06b52` and create `fix/role-reorder-hierarchy`.
- [x] Read role documentation, existing hierarchy tests, role store and gRPC paths.
- [x] Add RED tests for the indisputable prohibitions and regression coverage.
- [x] Compile the focused test package, record the Docker-free validation limitation and expected failures, and obtain independent test review.
- [ ] Commit only the ExecPlan and test files; leave the worktree clean and do not push.

## Detailed Steps

1. Use the existing role-manager-at-position-two fixture so the actor has `SPACE_MANAGE_ROLES` but is not Owner. Snapshot positions through `ListRoles` before each denied request.
2. Add table-driven denied calls whose ordered list includes Owner, Admin, or a role whose existing position is equal to/higher than the actor's top position. Assert `PermissionDenied` and an unchanged complete position map.
3. Add denied calls that move a target role to a resulting position equal to/higher than the actor's top position, even when the target began lower. Assert `PermissionDenied` and an unchanged position map.
4. Add a missing-`SPACE_MANAGE_ROLES` denial case, a mixed valid/invalid batch atomicity case, and malformed/wrong-space cases. Each must assert no persisted position change.
5. Retain `TestReorderRoles_UpdatesOrder` as the owner full-reorder regression rather than inventing new owner policy.
6. Run the narrow gRPC test package without starting Docker. Expected failures must be attributable to the current hierarchy bypass, then an independent reviewer checks that tests add no policy for lower managed roles and no production edits exist.
7. Commit the RED artifacts in one commit. Do not implement GREEN, push, or return/modify another worktree in this task.

## Validation

- [x] `go test ./internal/grpcsvc -run '^$' -count=1` in `src/backend/role` compiles the focused package without starting the Docker-backed integration tests.
- [x] Expected RED failures are statically demonstrated from `ReorderRoles`: managed targets skip `CanEditRole`; lower custom targets are checked only before their resulting position is assigned; the store transaction is reached for a mixed custom+Owner batch.
- [x] Fresh independent read-only review found no critical issue after the `>= actor top` matrix was expanded to cover both equality and strictly-higher result positions, and confirmed lower managed-role policy is untested.
- [x] `gofmt` and `git diff --check` pass; `git status --short` contains only the ExecPlan and intended test file prior to commit.

## Progress

- [x] Documentation and existing tests inspected.
- [x] RED tests authored; `gofmt` and `git diff --check` pass.
- [x] Focused package compiles with no tests run; dynamic integration execution is deliberately excluded by the no-Docker/no-network constraint.
- [x] First independent review identified omitted strictly-higher final-position coverage; it was added. A fresh re-review found no critical findings.
- [ ] RED commit created locally.

## Decisions

- Restrict assertions to `PermissionDenied` plus durable position invariance: these are the explicit T064 acceptance rules and do not rely on undocumented error text.
- Reuse the existing Owner full-order integration test as the owner regression, because it already includes managed roles and verifies the requested bypass.
- Do not test how lower managed roles behave for a non-owner: documentation and task both leave that policy unresolved.
- Do not execute the database-backed integration tests in this RED-only turn: their harness starts a PostgreSQL testcontainer, which would violate the explicit no-Docker/no-network constraint. Compile the package with `-run '^$'`; the pre-GREEN logical failures are instead identified directly from the request path.

## Risks And Follow-Ups

- Current `ReorderRoles` authorizes non-managed targets individually but skips `CanEditRole` for managed targets; planned RED tests should expose that bypass.
- A future GREEN change must validate every request before invoking the transactional store update, including final destination positions; it must preserve the unresolved lower-managed-role decision.
- Database-backed integration tests may require a pre-existing local test dependency. If unavailable, record the infrastructure failure separately; do not use Docker/network to create it.
