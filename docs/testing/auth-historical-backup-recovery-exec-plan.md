# ExecPlan: GI-T77 Auth matching-backup recovery fixture

## Purpose

Qualify isolated historical Auth recovery from a backup produced by the owned target database. Source catalog refusal is not recovery permission.

## Context

Sources: Auth README database section; docs/architecture/authority-source-revisions.md; game-integrations-exec-plan.md T77; GI-T77-AUTH-RECOVERY current criteria; docs/TESTING.md Java checks. Existing AuthMigrationCatalogJdbcIntegrationTest proves refusal but never restores a backup. Historical Flyway SDK-only releases must retain their original loader history.

## Scope

Add test-only backup/recovery orchestration and PostgreSQL integration coverage, plus a recovery runbook. No production repair, history mutation, activation, baseline, force, synthetic canonical migration, secrets, DEV/STAGING use or Federation work. Historical fixture readiness means matching loader validation/no-op only; current production readiness is independently unqualified.

## Milestones

- [x] RED: recovery tests require a missing backup helper; removing the archive gate subsequently makes the tamper regression fail (1 failure, expected throwable absent).
- [x] GREEN: target-generated archive bound to exact catalog and evidence restores only into empty owned database.
- [x] Negative: mismatched catalog and modified archive refuse before target restore; source evidence stays unchanged.
- [x] RED: actual Flyway BASELINE was accepted before preflight (1 failure, expected throwable absent); explicit SQL-only full-history preflight added.
- [x] GREEN: reject synthetic baseline as backup evidence.
- [ ] Relevant catalog regression checks, independent exact-head review, commit/push/PR.

## Detailed Steps

1. Test generates historical SDK-only Flyway history using the existing fixture, seeds identity/conversion/owner-receipt evidence, captures full history and produces pg_dump archive inside the owned container.
2. Refuse current catalog before any restore. Restore only matching archived catalog to a separate empty owned database; validate restored history and all evidence before loader readiness.
3. Reject tampered archive and preserve maintenance on every failure. Do not rename installed versions or edit histories.
4. Document operator containment and scope of this proof.

## Validation

Run mvn -B -Dtest=AuthMigrationCatalogJdbcIntegrationTest,AuthMigrationCatalogContractTest test in src/backend/auth. Real PostgreSQL 16/Testcontainers; failures must not leak identity, receipt or credential values.

## Progress

- [x] Task claimed by Anton-22; treehouse 57 leased; current source read.
- [x] Initial scoped checks: 7 JDBC + 4 contract tests passed without failures/errors/skips.
- [x] Read-only independent working-diff review: no material findings in original helper/tests.
- [x] Runbook added with exact scope and operator containment.
- [x] Final scoped Maven checks: 8 JDBC + 4 catalog contract tests, 0 failures/errors/skips; JDBC suite 58.39 seconds. Baseline and tamper REDs both transitioned to GREEN.
- [x] Read-only reviewer checked SQL-only preflight and runbook; no material findings.
- [ ] Exact-head review and PR lifecycle.

## Decisions

Recovery is a database/runtime bundle restore, never in-place history repair. Keep helper in test source: this evidence does not authorize online maintenance tooling.

## Risks And Follow-Ups

Archive digest binds bytes inside one fixture, not an external operator signature. Actual historical runtime binary provenance and serving admission, DEV/STAGING rollout and independent final acceptance remain separate gates. Parent T77 stays open.

Graphify update via the documented full executable path stayed silent past the bounded attempt and was interrupted; refresh remains unverified. No generated graph edits are included.
