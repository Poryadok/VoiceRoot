# PR gate latency and historical audit

CI-GATE-10M separates current-candidate validation from historical release
provenance. The policy does not change already-running PR #875 or rerun it.

| Obligation | Consumer / trigger | Evidence |
| --- | --- | --- |
| Exact current PR parents, clean complete merge tree, immutable SQL tree entries, source-disabled regression guard | Every PR, required `current-diff-contract` | base/head/target, validator digest, changed paths, diagnostics artifact |
| F05–F12 provisioning/docs and all original CI-script checks | Path-selected four `ci-script-tests` shards, required as a matrix | job/step logs, unchanged failure propagation |
| Unit/widget discovery, serial isolation | Four required `flutter-unit-tests` shards on the original Flutter selector | test logs and per-shard failed golden artifacts |
| F13 historical source ownership, pinned checkpoints, aliases, octopus/nested contribution fixtures | Required master gate before release/staging; nightly; every manual dispatch; PRs changing the historical validator/scope/base helpers; named release branches | full log and checkout SHA, retained 14 days even on failure |
| Auth/permissions, proto compatibility, affected service integration, Compose/A1 proofs | Existing selected mandatory jobs | existing logs/artifacts and runtime evidence |

Historical replay is explicitly a different guarantee from current-tree checks.
Moving it does not assert that lexical activation checks prove arbitrary program
semantics. The bounded guard refuses additions to prohibited root wiring and
scans full modified source-disabled lifecycle files; ordinary service tests
remain responsible for runtime behavior. Shipped migrations can only be extended
additively. An authorized historical renumber needs a separate reviewed policy
change, not an environment override.

The four Make shards preserve all 70 original shell/Python recipe commands;
policy adds current-diff, routing and latency regression tests. Full local
`make ci-script-tests` still defaults to full historical replay; PR script parity
uses `make -j4 ci-script-tests-pr` plus the explicit current-diff invocation in
[TESTING](../../docs/TESTING.md). Flutter's native file sharding retains full
discovery and does not introduce a hand-maintained test allowlist.
Go service short/integration jobs depend only on `changes`: they consume the
checked-out shared package source, not artifacts of `backend-go-pkg`. The
aggregator still requires the selected package job to succeed, while independent
service checks no longer wait for its execution and another runner allocation.

No validator-result cache is used. Every candidate is checked again, including
changed event parents and validator source. Dirty tracked worktrees fail; the
artifact is diagnostic, never accepted as authority on another run. Existing
Go/Flutter/dependency and Docker content caches still execute validation and
respect their toolchain/content identities. No `continue-on-error` or synthetic
success is introduced.

Compose E2E regenerates a build-only Bake plan from the current effective
three-file Compose configuration. The helper checks the complete target
inventory and exact context/Dockerfile/args/platform/tags, including Compose's
implicit project-service names. Raw runtime configuration stays in memory;
the transient build-only definition and plan use private permissions. The
official Bake action uses `source: .` and loads every current image before
`up --no-build`; all original smoke tests still execute. BuildKit content keys
invalidate changed inputs, independently of cache scope names. Per-target
`compose-e2e-*` writers cannot replace the existing `backend-*` cache objects;
matching backend caches can be read. Export is optional and bounded to30s
per target; build/load/test failures remain fatal.

The network-free proof `python3 scripts/ci/compose_bake_cache_integration.py`
checks the actual implicit tag, unchanged-input cache hit, and changed loaded
image ID/content after mutating a `COPY` source. It needs Docker/Buildx and
creates only disposable images/stopped containers. The seven offline plan
tests are part of `make ci-script-tests-pr`. This local proof does not assert
a GHA cache hit or transport performance; hosted measurements establish those.
See [native Bake Compose support](https://docs.docker.com/build/bake/compose-file/)
and [GHA cache scopes](https://docs.docker.com/build/cache/backends/gha/).

## Measurement

[Observed baseline](gate-baseline.md) includes 18 heads across 17 branches:
16 success, one failure, one incomplete. Successful mean 9.17 minutes/p95 23.57;
CI/global success subset n=3 mean 20.20/p95 23.57. Including the completed
failure gives n=17 mean 12.48/p95 65.43. This distinct-branch sample reduces
repeated-head cancellation bias; it is not a volume-weighted estimate.
The successful global/CI group is reported separately and never discarded.

The new policy's after results are **preliminary until at least ten distinct
representative PR heads have completed**. Do not claim the ten-minute target
from local test speed, one successful run or the aggregator's own duration.
Known remaining broad-path costs include Compose startup+smoke and Space
integration; cold builds and runner contention must remain visible.
The [first observed wide after run](gate-after.md) passed in15m44s; it does
not establish the ten-minute target. It includes every selected mandatory job
and the historical-validator exception, together with prior cancelled heads.

Reproduce future measurements without modifying runs:

```bash
python3 scripts/ci/gate-latency.py sample.json tmp/ci-gate-measurement
```

Manifest shape (use real run IDs and a declared scope for every sample):

```json
{"repository":"Poryadok/VoiceRoot","samples":[{"run_id":39385343073,"scope":"CI/global"}]}
```

The collector saves raw run metadata and all job attempts, then reports
`created_at` → final-attempt `ci-gate.completed_at`, initial queue, execution
envelope, job waits/step costs, attempts, mean and nearest-rank p95. Wall-clock
includes retry gaps. Attempt execution sums parallel jobs and is explicitly
not wall-clock. Job created→started can include dependency wait; do not label
all of it runner queue. Missing gates, cancellation and in-progress runs stay
visible and censored; terminal failures have their own outcome and enter the
all-terminal group. Use one row per distinct PR head, retain global/CI and
slow exceptions, and record selection dates/methodology with the report.
