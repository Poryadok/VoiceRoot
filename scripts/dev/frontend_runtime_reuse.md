# DEV frontend runtime configuration adoption and reuse

The central operator authenticates the current candidate package and platform
image digests, qualifies the actual workload/config inventory and holds the
common DEV claim, journal, fence and lock. Frontends may run before the writer
cut only after that operator qualifies them as nonwriters. This phase does not
establish that mapping or authorize DEV/STAGING by itself.

`FrontendExecutor` requires immutable `frontend_deployment_baseline` intent
with exactly `web`, `admin`, `portal`, each `{uid,generation,spec}` from the
qualified actual Deployment inventory. It compares live UID, generation and
full spec before first adoption, reuse or no-op; status-only RV changes are
allowed before the fresh UID/RV/full-spec CAS. Full specs remain in the private
journal, never the public ledger or process arguments.

For each already-adopted app, `frontend_existing_config[app]` pins the actual
immutable ConfigMap `{uid,resourceVersion,sha256}` from that qualified inventory.
The phase rereads the entire public document, checks exact current source-owned
DEV bytes, immutable identity and exact read-only Deployment projection, and
retains the receipt across the operation. A missing, mutable, changed or
unqualified object holds; it is never replaced or silently created. This is
current environment evidence and does not assert a previous candidate source.
A later P2 source/image is permitted only under the caller's separately
authenticated P2 package and its platform digest mapping.

If the actual qualified image differs from the candidate, the phase journals
and applies a full-spec CAS, then verifies the replacement ReplicaSet/Pods,
effective specs and platform image IDs and reads exact config bytes from every
Pod. If it already matches, the phase records an observation without issuing a
fake patch, but performs the same actual Pod and served-byte checks. After all
frontend effects, the owner obtains the actual new baseline for the cohort
phase. After cohort rollout/resume, `verify_current(app)` rechecks the stored
target and live Pod/image/config evidence. An uncertain response remains HOLD;
no automatic retry, rollback, lock release or live authority is provided here.

Linux focused tests: `python3 scripts/dev/frontend_runtime_reuse_test.py`,
`python3 scripts/dev/frontend_runtime_executor_test.py`, and
`python3 scripts/dev/frontend_runtime_adoption_test.py`. Kubernetes, claim and
candidate qualification in these tests are fixtures, not live DEV evidence.
