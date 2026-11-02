# PR gate latency and historical audit

CI-GATE-10M separates current-candidate validation from historical release
provenance. The policy does not change already-running PR #875 or rerun it.

| Obligation | Consumer / trigger | Evidence |
| --- | --- | --- |
| Exact current PR parents, clean complete merge tree, immutable SQL tree entries, source-disabled regression guard | Every PR, required `current-diff-contract` | base/head/target, validator digest, changed paths, diagnostics artifact |
| F05–F12 provisioning/docs and all original CI-script checks | Path-selected four `ci-script-tests` shards, required as a matrix | job/step logs, unchanged failure propagation |
| Unit/widget discovery, serial isolation | Four required `flutter-unit-tests` file shards on the original Flutter selector | canonical inventory, shard receipts, test logs and per-shard failed golden artifacts |
| F13 historical source ownership, pinned checkpoints, aliases, octopus/nested contribution fixtures | Nightly and manual full CI; production deployment reuses an unaffected successful nightly baseline or runs a fresh audit; an explicit operator override requires a reason | full log and checkout SHA, retained 14 days even on failure; override reason in deployment summary |
| Auth/permissions, proto compatibility, affected service integration, Compose/A1 proofs | Existing selected mandatory jobs | existing logs/artifacts and runtime evidence |

Historical replay is explicitly a different guarantee from current-tree checks.
Moving it does not assert that lexical activation checks prove arbitrary program
semantics. The bounded guard refuses additions to prohibited root wiring and
scans full modified source-disabled lifecycle files; ordinary service tests
remain responsible for runtime behavior. Shipped migrations can only be extended
additively, except the exact reviewed nine-entry Native/P3 repair transition
from source `e409cd05c40e857b61109513eb2154f968970c02` (record
`f87ee933aa1f599284e1932e44bf32b4e2af48ba`). The validator pins both original
and repaired mode/type/blob entries and requires the whole package: four DOWN
rollback race fixes (Auth9, Messaging25/26, Role13), and the five existing Voice
composition entries (`main.go`, `logger.go`, `ordinary_runtime.go`,
`party_outbox_runtime.go`, `space_media_runtime.go`). This is no future file or
migration allowance: byte, mode, path, deletion, baseline or partial-package
drift rejects; unrelated paths still undergo normal checks. The four rollback
fixes retain admitted work by fencing writers before probing. Auth9 and
Messaging25/26 keep guard and DROP in one DO statement; Role13 requires its
existing whole-migration transaction, tested through one SQL execution. Existing
Native A3 roster relay and roomlifecycle activation remain prohibited. Exact
source entries authorize no runtime/deployment result. An authorized historical renumber needs a separate reviewed policy
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

The PR `affected-check-plan` job produces an advisory comparison artifact using
the exact base, head, merge candidate and branch name. The planner records both
sides of renames and deletions, reuses the Go and Docker selectors, and compares
its selection with the existing path-filter outputs. Separately, PR routing uses
the planner to widen unclassified code or unsupported filter grammar before job
selection. The required gate recomputes the plan at the candidate and rejects
missing, skipped, cancelled or failed selected jobs, in addition to the existing
gate. When the event base is stale, its ancestry and the candidate's actual
parents and merge tree are checked before using the actual base. Job-level
success does not prove every required Go matrix case ran; the inventory and
member receipts described below are still required. Neither the comparison nor
this gate is a local preflight receipt or release qualification. When CI script
checks are selected, local preflight runs the named gate shell and Python test
scripts and a source-bound subset of router cases with retained logs. The executable
gate/router safety surface is pinned and changes require a fresh isolation audit;
drift refuses execution. Shell fixture cases that might reach a real Docker
daemon or Go tool remain excluded until their execution environment is isolated;
the full script matrix remains `not_run`. The local
preflight runs the snapshot's current-diff validator against a synthetic commit
with the captured HEAD as parent and records its output and source identity.
That diagnostic checks the local tree against the requested base; it does not
establish GitHub merge-candidate parents or merge-tree parity and cannot make
the preflight pass. For a clean
checkout of the exact candidate with Git, Bash, jq and Python available:

```bash
python3 scripts/ci/affected_check_plan.py --base "$BASE_SHA" --head "$HEAD_SHA" \
  --candidate "$CANDIDATE_SHA" --head-ref "$HEAD_REF"
```

For a planner-selected Chat integration, Windows local preflight can optionally
run one private Testcontainers case when all five `--chat-*` creator, archive,
digest and image-ID arguments are supplied. It leaves the integration matrix
`not_run` and the preflight incomplete. Cleanup uses the owned journal and
retains uncertainty rather than touching foreign resources. The diagnostic
assumes a trusted Docker administrator and cooperative other users; it does not
enforce exclusive daemon/socket access or authenticate helper, image, tool and
dependency provenance. The host Go dependency acquisition is not supervised.
One selected case does not prove general concurrent Chat acceptance or permit
PR handoff. The preflight consumes an in-process result; its journal path and
digest fields are shape-checked rather than externally revalidated receipts.
Linux remains unsupported.

Compose E2E and the attachment restart proof explicitly build current Compose
images before starting the stack with --no-build. An isolated Buildx builder
uses per-service, per-platform content caches for build preparation; cached
layers do not restore runtime state or replace the required smoke and restart
checks. Build failures remain fatal. Hosted cold and warm timings are needed
to establish the effect on job latency.

Compose E2E's host Go commands restore module downloads and compiled objects
using the explicit Gateway, Notification and shared pkg `go.sum` paths.
The monorepo has no root `go.sum`; the default setup-go path cannot restore
this cache. All Go test invocations retain `-count=1`, so tests execute again
against current source and the current Compose stack. See
[setup-go monorepo cache inputs](https://github.com/actions/setup-go/blob/v5/README.md#caching-dependency-files-and-build-outputs).
The first run with this explicit dependency key can be cold and seeds later
restores; it does not establish warm host-cache performance. The isolated proof
`python3 scripts/ci/compose_go_cache_integration.py` checks repeated execution
with a warm compilation cache and failure after a source mutation.

## PR scope and image verification

On selected PRs, Auth Maven/Testcontainers and the Auth image build execute in
independent jobs against the same checked-out candidate. The gate requires both
to succeed. Master retains the existing sequential test, image load, smoke and
publish path. A cold image cache still performs the full image build.

Path detection uses dorny v4 `some-with-excludes`: positive globs select inputs
and negative globs exclude them. Docs-only changes still require the current
diff contract. Mixed docs/code changes retain checks for their code inputs.

Known independent proof workflows and diagnostic/latency helpers are excluded
from broad **PR** product fanout, but still select all four mandatory CI-script
shards. Their dedicated workflow triggers remain intact. Unknown CI paths,
`ci.yml`, the filter configuration, Makefile and shared wiring retain broad
checks. The effective `ci_global` output restores the original broad CI-script
selection on master/non-PR events and is passed to both matrix resolvers.

PR attachment restart proof has its own input filter: unrelated database
migrations no longer select it. File/auth/identity/message inputs, relevant
migrations, shared runtime/Compose wiring and the proof itself retain pre-merge
coverage. Each database migration directory also selects its own service filter,
so migration-only PRs retain that service's unit/integration checks instead of
depending on an incidental unrelated proof. Master retains the original A1
selector. Renderer PR image verification
uses its actual package/generated-module/context inputs rather than `run_pkg`;
master still publishes the renderer on every code SHA. Neither change moves a
required selected proof after merge.

Go consumer test expansion remains in place. `docker_go_services` separately
selects directly changed services and consumers of their source `COPY` inputs
in the actual Dockerfiles. Their backend-go jobs still run tests and selected
image builds; S2S-only consumers run tests without unrelated image builds.
Generated modules, shared pkg, module manifests, context exclusions and global
wiring conservatively select all images/checks. Unsupported COPY grammar or ADD
widens selection. Post-merge staging build/promotion policy is unchanged.

The PR Go integration matrix retains the existing RPC-neighbour and Docker
source-consumer selections and adds event subscribers and dedicated invalidation
consumers from [CONTRACT_MATRIX](../../docs/CONTRACT_MATRIX.md). Auth, shared packages, proto,
shared build inputs and global CI/runtime wiring conservatively select all Go
service integrations until a narrower verified dependency inventory exists.
Migration paths select their owning service through the existing path filters;
the same RPC and event consumer expansion then applies. This routing does not
by itself prove that a required test executed: opt-in skips and Testcontainers
availability must be verified separately.

When Analytics is in the affected Go integration matrix, the dedicated PR
ClickHouse job also runs. It checks the uncached JSON test stream for an actual
successful `TestClickHouseInsertBatch` execution; a skipped or missing case
cannot satisfy the job. The full Go matrix still needs its own case inventory
and member receipts.

When Story is in the PR integration matrix, `story-lfp-integration` runs both
Story LFP consent packages against a job-owned PostgreSQL service. The verifier
requires the named store and protected transport inventory, successful package exits,
and no skipped tests or subtests in those packages. Raw Go events are retained
on failure; a successful run also retains a candidate/run/attempt-bound receipt.
This is the Story LFP inventory, not the complete Go matrix inventory.

When User is in the PR integration matrix, `user-association-integration` runs
the native identity-association package against its own PostgreSQL service.
All current named package tests are inventoried and skips are rejected. The
same job separately runs the protected capture transport with owned PostgreSQL
and Redis, requiring all named authority and canonical SQL cohort subtests, plus
the full-cohort and archive/raw-erasure cases, with no skips. A final focused
User admission-eligibility transport proof uses the same owned services. The
job also requires the native primary and secondary profile-birth capture cases
against owned PostgreSQL without skips. These
fixtures do not activate the source-disabled production path.

When Notification is in the PR integration matrix,
`notification-quota-integration` runs the complete `chatquota` package against
a job-owned Redis service. The named package inventory must pass without skips,
including the real-Redis concurrent quota test; its raw events and receipt are
retained under the same evidence rules.

When Role is in the PR integration matrix, `role-migration-integration` runs
the migration 13 rollback writer-fence, retained-evidence, stale-snapshot and
empty round-trip cases against an owned PostgreSQL service. The named cases
and subtests in `scripts/ci/role_migration_required.json` must pass with no
skips; this is separate from current-diff migration immutability validation.
The same job also requires the actual Role version 20 PostgreSQL bootstrap
transport test with an owned Redis service. Its historical schema fixture is
not evidence for migrations after version 20.

When Bot is in the PR integration matrix, `bot-membership-integration` requires
both native membership transition and protected cleanup cases against owned
PostgreSQL, with no skips in the focused runs.

When Voice is in the PR integration matrix, `voice-ledger-integration` requires
the real Redis ledger atomicity case against owned Redis. Source-disabled later
slice crash harness seams are not represented by this focused proof.

When Messaging is in the PR integration matrix, `messaging-threadlist-integration`
requires its PostgreSQL immutable-node and rollback test without skips.

When Space is in the PR integration matrix, `space-autovacuum-integration`
requires the tagged maintenance-window test against owned PostgreSQL, with a
real autovacuum worker and no skipped test. This does not replace the separate
Space native catalog or account-ban proofs.

When Chat is in the PR integration matrix, `chat-lifecycle-integration` requires
the tagged realtime lifecycle storage test against owned PostgreSQL without skips.
When Realtime is in that matrix, `realtime-lifecycle-integration` requires its
tagged fanout and protected transport tests against owned PostgreSQL without skips.
When Role is in that matrix, `role-space-receipt-integration` requires the two
tagged protected receipt producer tests with real Role and Space fixtures. The
separate interactive cutover oracle remains unresolved and is not claimed by them.
The affected `t31-critical` job calls the existing T31 Compose workflow, including
the GIS-to-Chat lost-response retry proof, and its result is required by the PR
gate. The standalone workflow remains enabled for comparison.
The current Compose Gateway starts the wider backend dependency set, so backend
changes remain conservatively selected until that runtime scope is reduced.
When File is in the matrix, its required cases include the real GIF codec tests;
the File runner installs FFmpeg and treats an unavailable codec as a failure.
Its PDF renderer and upload-to-thumbnail runtime tests also require Poppler and
cannot pass by skipping a missing renderer.

Each Go PR integration matrix member records its uncached Go JSON event stream
and a diagnostic summary bound to the candidate, run and attempt. The summary
includes named pass/skip/fail counts and skipped cases and preserves a nonzero
test exit. `backend-go-integration-evidence` downloads one artifact per selected
member, validates the exact member set, candidate/run/attempt, summary and raw
stream digest, requires a clean exact-candidate source checkout, and requires
each listed case in `go_integration_required.json`
to run and pass. This initial inventory covers observed untagged integration
cases and an Analytics adapter sentinel; it is not a complete inventory of
native, tagged, opt-in, subtest or cross-service proofs. Those require their own
explicit evidence and cannot be inferred from this aggregate.
The source audit fails on an unclassified or stale `*_integration_test.go`
entrypoint. Its explicit unresolved exceptions report open coverage gaps; they
do not make those tests pass and it does not cover other filename conventions.
The baseline also requires the named Space Bot installation add/remove
subtests and tombstone case using the qualified native 45/50/51 fixture; it
does not claim coverage of the tagged Space account-ban suite.

PR changes to Subscription source/migrations or protobufs also select Auth tests
because Auth consumes Subscription events and compiles contracts. The separate
`auth_test_impact` filter does not select Auth image builds or deployments.

Offline routing tests prove selector/gate agreement and required-result failure
propagation, not application runtime correctness or latency improvement. Keep
hosted exact-head CI and the existing measurement methodology below.

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
