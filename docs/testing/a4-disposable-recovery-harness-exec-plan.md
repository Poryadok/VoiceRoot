# ExecPlan: A4 disposable recovery harness

## Purpose

Provide one local/disposable-environment command that records recovery evidence
without claiming the A4 product smoke or writing to a source environment. The
G1 operator will use its evidence bundle when external access and the approved
RPO/RTO are available.

## Context

- `docs/PLAN.md` A4 requires a source backup restored only to an isolated
  target, durable-store coverage, configuration validation, admission evidence,
  and observability evidence.
- `docs/DATA_STORES.md` owns the durable-store inventory.
- `docs/OPERATIONS.md` and `docs/DEPLOYMENT.md` require recovery to avoid
  writing over the source.
- Existing `voice-db-runtime-provisioning-contract_test.sh` proves only the
  source-disabled `voice_db` provisioning seam; it is not an A4 recovery drill.

## Scope

- In: a shell harness that validates explicit source/target separation and
  bundles supplied config, admission, and observability evidence with a
  durable-store manifest.
- Out: database restore execution, Compose/staging runtime, credentials,
  RPO/RTO acceptance, and any final-alpha claim.

## Milestones

- [x] RED: contract test specifies the required safety checks and evidence
  bundle.
- [x] GREEN: harness produces a deterministic dry-run bundle and rejects an
  identical source/target endpoint.
- [ ] Validate the focused contract test and CI script-test reachability.

## Validation

- [x] `make a4-disposable-recovery-harness-test` proves a deterministic
  dry-run bundle and same-authority rejection.
- [ ] `make ci-script-tests` proves the target remains reachable from CI
  (currently blocked locally because the Git Bash environment lacks `jq`).

## Progress

- [x] Scope selected after confirming no existing A4-wide harness.
- [x] RED oracle added.
- [x] Harness and GREEN evidence added.
- [x] Focused Make target passed through the repository Git Bash.
- [x] Full script-test attempt stopped before this target on the missing local
  `jq` prerequisite.

## Decisions

- The first slice is evidence preparation rather than live restore execution:
  the documented destructive boundary needs operator-provided isolated target
  credentials and a separately approved cutover.

## Risks And Follow-Ups

- A real target backup/restore drill remains a G1/staging action and must set
  accepted RPO/RTO before it can be reported as alpha evidence.
