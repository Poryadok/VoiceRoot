# Protected STAGING cohort preservation before prerequisites

Implement the root-assigned ordering correction to the protected deployment
workflow (docs/DEPLOYMENT.md, STAGING resource fencing; docs/DATA_STORES.md and
docs/OPERATIONS.md preservation requirements). The existing Session and global
operation lock own every request. No live actions are authorized by this work.

## Ownership and design

WT29 owns only new staging_cohort_preservation*.py producer, paused-stage loader
and tests. WT25 owns existing Session/bridge/workflow and fixed shared STAGING
archive wrappers. The approved parent coordinator remains frozen. Shared DEV
validators must remain unchanged; STAGING uses actual STAGING context and root
catalog/metadata references, never relabelled DEV authority.

Read-only source/candidate/precut preparation precedes the typed preservation
phase. The fixed producer closes writers, binds the actual catalog and affected
stores, captures and qualifies their backup, and returns AWAITING_OFF_NODE.
The existing authorize request verifies actual remote ciphertext/readback and
isolated restore before the parent prerequisite effects can run. The Session
remains active and workloads remain fenced between requests; no OS lock is held
across workflow upload. Each subsequent request reacquires the same root lock.

## Verification sequence

- Drive durable fixed cut effects with tests: PENDING before submit, exact
  response-bound completion, lost reply never adopted/replayed.
- Verify actual pre-cut image/UID/spec custody and paused-stage continuation;
  reject foreign UID, generation/spec drift, missing terminal proof and claim
  changes. No active recapture after draining writers.
- Exercise fixed PG catalog/dump transport and exact receipt/metadata binding.
- Join shared archive/restore wrappers and actual RootJournal/Session with
  synthetic Kubernetes/PG boundaries explicitly identified.
- Publish focused checkpoints, then obtain independent review. Real STAGING,
  operator inputs, installed helper eligibility and live restore are separate.

## Progress

The WT29 lease and clean published parent were verified. This branch starts at
Stage afe56bab84ee. Interfaces agreed with WT25 include prepare_inputs,
bind_cohort_preservation, validate_backup_binding, produce_dump, load_qualified,
capture_stage and revalidate_paused. Implementation and tests are in progress.
