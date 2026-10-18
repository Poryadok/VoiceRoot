# ExecPlan: Admin ordinary permissions and Owner authority

## Purpose

Allow a Space Admin to manage lower custom roles by default while preserving
Owner identity gates, node deny precedence and hierarchy restrictions.

## Context

- Accepted TT decision `BE-068-ASTRA-2026-10-18-v1`, BE-068 c3 and attachment.
- Canon: `features/roles.md`, `microservices/role-service.md`,
  `features/spaces.md`, `DATA_MODEL.md`, `DATA_STORES.md`, `TESTING.md`.
- Admin currently omits `SPACE_MANAGE_ROLES`; the misleading
  `OwnerExclusiveMask` helper describes an ordinary delegable bit as ownership.
- `UpdateRole` checks the old target position but not a proposed new position.
- Ordinary Role RPCs already authorize and mutate under one per-Space lock.
- Role authority snapshots pin schema 16; migration 17 changes data only and
  must remain compatible with the same fixed catalog/trigger/function proof.

## Scope

Correct new Admin seeds and existing system Admin rows, validate proposed role
positions, clarify the canon and add focused regressions. Preserve assignment
versus CRUD permissions, generic Owner mutation denial, Space fences and custom
roles/overrides. No lifecycle activation, new permission bit, deploy or acceptance.

## Milestones

- [x] Demonstrate a failing Admin seed regression before production edits.
- [x] Implement seed correction, proposed-position validation and additive
  migration `role_db/000017_admin_role_management`.
- [ ] Prove Admin/custom/Owner composition, hierarchy and mutation denial;
  migration preservation/idempotence and authority-source compatibility.
- [ ] Complete scoped checks, independent security review and exact-head CI.

## Detailed Steps

1. Test new Admin equals all ordinary bits; replace stale seed-only ownership
   assertions with genuine Owner/ordinary-node-deny cases.
2. Exercise real Role gRPC management for Admin and delegated managers; deny
   equal/higher existing or proposed positions, preserve state/events on denial,
   and retain dedicated Owner-only generic mutation guards.
3. Seed Admin with `AllMask`; remove the misleading exclusive-mask helper.
   Check proposed `UpdateRole.position` using existing hierarchy policy inside
   the same ordinary transaction as authorization and mutation.
4. Migration 17 ORs only bit 2 into actual system Admin rows lacking it, using
   existing durable policy-revision triggers. Never replace the full mask.
   DOWN refuses because it cannot safely infer which legacy rows already held
   that bit; rollback requires a separately reviewed forward correction.
5. Permit only known clean schema 16/17 for unchanged Role authority catalogs;
   choose a fixed catalog from an untrusted version hint and repeat the exact
   guard inside the locked repeatable-read transaction. Unknown/dirty fails.
6. Use pinned migration-driver integration to exercise 16→17, preservation,
   repeated UP, refused DOWN, revision advancement and source-read readiness.

## Validation

- `go test ./permissions -count=1` supplies meaningful local RED/GREEN.
- `go test -short ./...`, `go vet ./...`, pinned `golangci-lint run ./...`.
- Hosted full Role PostgreSQL integration and exact-head required CI. Local
  Docker is unavailable; skipped tests are not runtime evidence.
- Existing generic Owner, foreign-Space and lifecycle-fence suites remain
  required integration evidence; Space owner-only lifecycle code is unchanged.

## Progress

- [x] Read accepted Astra contract and actual hierarchy/transaction/catalog code.
- [x] Seed RED observed: expected `0x3ffffffffff`, actual `0x3fffffffffb`.
- [x] Seed correction, transactional proposed-position check, migration and
  focused integration tests implemented; first short pass: 140 tests.
- [x] Full Role short suite: 337 tests passed across 11 packages; `go vet ./...`
  and `golangci-lint run ./...` passed. PostgreSQL tests are excluded by short.
- [ ] PostgreSQL migration and hierarchy outcomes require hosted execution;
  local Docker is unavailable, so they have not been claimed as RED or PASS.
- [ ] Review, CI and delivery.

## Decisions

`SPACE_MANAGE_ROLES` is delegable lower-role CRUD, while Owner authority derives
from genuine ownership and dedicated protocols. Equal ordinary masks do not
confer ownership. Admin follows ordinary union and explicit node deny precedence.

## Risks And Follow-Ups

Migration changes Admin's default authority intentionally. Proposed-position
validation must ship with it. Preserve existing freeze/retirement and Owner
transfer gates. Actual database proof and final runtime acceptance remain open
until executed; source tests alone do not complete A2.
