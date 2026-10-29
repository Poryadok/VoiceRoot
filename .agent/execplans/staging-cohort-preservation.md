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

The producer now implements fixed cut CAS, complete PostgreSQL catalog and dump
receipts, source-bound shared archive/restore calls, owned network-none clone
resources, native NATS cut transport, encrypted capsule, prepare/upload and
off-node continuation entrypoints. A finite source-plan store projection is
captured before the cut and rechecked with physical PVC/PV identities. The
only admitted auxiliary restart is the existing source-qualified P3 publisher,
after off-node restore, with replica-only CAS and fresh owned Pod/TLS proof.

Native Linux verification: six RootJournal/Session cut, catalog and isolated
command negative tests plus one real PostgreSQL16 dump/full-catalog restore
composition pass (7/7, 20.802s). The latter includes a nonempty business table,
roles, postgres and template1, actual encrypted archive and network-none clone
cleanup. Kubernetes transport, source admission, engine identity and paused
view are explicit test fixtures; PostgreSQL SQL/dumps, Docker and restore are
real. This is not live STAGING or installed helper qualification.

Remaining before activation: independent producer review, actual bridge/parent
joined prepare/upload/authorize test, native NATS/CMS execution in that joined
fixture, protected auxiliary successor negatives, historical/paused successor
integration, and the independently owned native Space migration qualification.
The shared STAGING restore deliberately rejects changed Space until that native
adapter exists. Source projection review/credential successor checks also
remain part of the joint gate. No live claim or deployment has been performed.

The actual CMS composition subsequently exposed a binding mismatch: decrypt
requires the private cohort member hashes while GitHub custody accepts exactly
seven public fields. The producer now retains the complete private binding and
projects only those seven fields for remote verification. Actual CMS decryption
and a second isolated PG16 full-catalog restore pass (24.482s); native NATS and
GitHub remain explicitly synthetic boundaries in this test. Seven focused unit
tests pass, including missing off-node proof, lost auxiliary CAS reply, altered
image and exact replica-only response. Off-node receipt/cleanup records are
independently root-rechecked before auxiliary resume and qualification loading.
The fixed NativeRuntime now accepts the shared exact healthz readiness command as one bounded owned-copy operation; lost terminal replies remain PENDING without replay. Eight actual RootSession unit checks pass. The native NATS transport fixture is being joined separately from the completed actual PG/CMS proof.

An owned shared-volume Linux runner now executes actual pinned NATS 2.12.12 with a populated file stream. Closed capture, first isolated restore, and actual native readback through a second isolated copy pass (1/1, 4.622s), with real RootSession and command permits. Source/Kubernetes/paused admission remains synthetic; this fixture has no consumers and does not claim native consumer decoder coverage. The runner and its named volume are test-owned; no live PVC is mounted.

The PG/CMS fixture now enters the actual qualify_offnode Actions/Session continuation and load_qualified, rather than invoking decrypt alone. Actual encrypted readback and second PG restore pass (23.782s); repeated qualification performs no second remote/restore operation, and altered PENDING offnode receipt is rejected. GitHub retrieval, NATS readback, paused/Kube/source admission and absence of an auxiliary remain explicit external fixtures in this test.

The readonly PostgreSQL transport now pins search_path inside its transaction.
An actual PG16 public function overload and inherited role search_path falsify
the old query (RED 2.232s); the fixed complete PG/CMS test passes (30.706s).
Exact retained logs and source hashes are in ignored
tmp/preservation-pg-review/evidence.json. Independent scoped review closed this
P2 on commit 1a8afe02e8f4; it did not clear later producer additions.

Pre-cut actor/NATS planning authorities are now root-captured and included in
the preservation plan. A fixed ScratchRuntime requalifies signed actor and
bootstrap authentication only in disposable empty network-none brokers, under
the permanent Root Session preservation marker and one-use exact command/body
permits. Every command and owned cleanup is durable; lost replies remain
PENDING. Actual signed actor/bootstrap proof and populated native NATS copy
tests plus ten focused unit tests pass together (12/12, 21.065s). An additional
foreign-command/lost-reply boundary test passes (1/1). Crypto/source/Kubernetes
admission fixtures remain explicit; no live credential or data mount is used.

Closed NATS copies now invoke the existing closed_copy_preflight and compare
the frozen source plan/binding/scripts, while allowing the actual closed census
to differ from the planning observation. This callback's source-authority
comparison is unit-tested; the populated native-copy fixture explicitly mocks
that authority boundary and the separate scratch fixture executes real auth.
The fixed load_rollout_continuation transfers only the existing qualified cut,
custody, provenance, contract and paused events. Its actual PG/CMS Root Session
join and altered-event refusal pass (26.721s), with NATS bytes still synthetic.

Remaining: independent review of this immutable producer delta, actual Stage
bridge continuation composition, and the separately owned native Space24
adapter/tool publication. No native Space rehearsal or live rollout readiness
is asserted by these tests. Manual AST refresh remains deferred to the root
owner's already-running repository graph process.
