# ExecPlan: A1 staging NATS persistent storage migration

## Purpose

Convert staging NATS JetStream storage from pod-local `emptyDir` to persistent storage without losing the current streams, messages, or intentionally retained consumers. Keep cutover blocked until storage capability and data-preservation evidence are reviewed.

## Context

- Docs: `docs/DEPLOYMENT.md`, `docs/OPERATIONS.md`, `docs/TESTING.md`, `deploy/staging/README.md`.
- Current state: `deploy/staging/infra.yaml` runs NATS as a Deployment with `/data` on `emptyDir`; production's StatefulSet PVC is reference only.
- Known staging data: 7 streams, 566 legacy dynamic consumers, 8 retained messages, and an application consumer cap of 512.
- Dependencies: #465 at `3438dd636288fbfdbb9d4ebf73efca3874baff52`; #473 remains separate and must precede any activation that depends on its topology.
- Constraint: no live staging/production apply, secret operations, or pod restart/cutover in this task.

## Scope

- In: staging NATS storage resources, read-only cluster storage preflight, migration guardrails, tests, and operator documentation.
- Out: #473 auth/JWT/ACL paths, other services, and live cluster changes.
- Gap: staging storage class, provisioner capacity, complete consumer classification, and approved replay/idempotency semantics are not evidenced. Cutover must fail closed until these are supplied.

## Milestones

- [x] Add failing contract and migration-guard tests.
- [x] Add an isolated PVC-backed candidate while preserving the current emptyDir source and stable Service.
- [x] Exclude the live source Deployment from ordinary infra apply so image-tag changes cannot restart its emptyDir pod.
- [x] Add exact source census/state matching, candidate restore checks, and a selector-only accepted cutover/rollback gate.
- [x] Require an empty candidate before restore and an exact seven-stream set at acceptance.
- [x] Document export, quiesce/fence, validation, rollback, and blockers.
- [ ] Run final focused static checks and update graph.

## Detailed Steps

1. Keep checkout on a task branch based on the current `origin/master` dependency commit.
2. Test source-preserving candidate topology, fabricated-consumer rejection, census drift detection, and rollback-before-mutation context checks.
3. Add a read-only storage-class/PVC capability preflight; do not apply a PVC or workload.
4. Keep candidate isolated until exact census/config/sequence/message-hash/replay/TLS acceptance; switch only the stable Service selector after acceptance.
5. Run only focused shell/static contract checks; no cluster access or runtime suites.

## Validation

- [x] Focused staging NATS contract and guard tests.
- [ ] `graphify update .` after final code edits.
- Hosted CI proof remains required before merge.

## Progress

- [x] Read required repo, deployment, testing, and Treehouse instructions; lease isolated worktree.
- [x] Refresh lease to `3438dd636288fbfdbb9d4ebf73efca3874baff52` and create task branch.
- [x] Implement and validate fail-closed migration slice.
- [ ] Refresh graph and record CI/PR handoff state.

## Decisions

- Do not recreate the 566 legacy consumers. Their count exceeds the 512 cap and classification/semantics are absent; require reviewed inventory and explicit evidence before proceeding.
- Do not assume staging storage class or available capacity from production's 10Gi PVC.
- Migration bundle hashes prove the exported archive is unchanged; stream-level sequence/hash and replay/idempotency proof remain operator evidence because this repository defines no canonical application stream hashing or replay harness.

## Risks And Follow-Ups

- Candidate TLS identity and eight retained-message hash proofs require service-owner evidence. The guard does not treat archive hashes alone as proof of restored payload contents.
- Staging capability and data inventory evidence must be gathered by an authorized operator before migration can proceed.
- No runtime NATS restore or Kubernetes apply was performed; end-to-end copy and rollback remain unverified until the protected staging contexts and complete evidence bundle exist.
