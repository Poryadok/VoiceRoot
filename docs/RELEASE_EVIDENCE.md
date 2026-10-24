# Release evidence and recovery lessons

Use this guide for CI diagnosis, older PR review, acceptance readers and staging
rollouts. It complements [TESTING](TESTING.md), [DEPLOYMENT](DEPLOYMENT.md) and
[CONTRIBUTING](CONTRIBUTING.md); it does not replace deployment authorization or
source, image, storage and runtime guards.

## Bind evidence to the source and operation

Record the tested commit, workflow run and attempt, selected jobs, artifacts and
deployed image digests. Branch names and PR titles are not source bindings. If
local main is behind or contains unrelated changes, preserve that work and use
exact Git revisions or the owner's worktree. A stale code index does not override
current source.

In the rollout protocol, request/report `report_run_id` identifies deployment;
`source_authority.run_id` identifies source CI. Do not invent a deployment run ID
in checkpoint context. Bind finish evidence to its actual journal, request nonce,
operation, source, attempt and canonical request hash.

Helper installation, runtime readiness and application acceptance are separate
outcomes. An installed helper does not prove application deployment. Where the
documented provisional protocol is available, `PROVISIONAL_READY` is not `PASS`:
use its original CI attempt and separate runtime acceptance. Never emulate this
by bypassing ordinary guards. A completed prepare report is a request snapshot,
not necessarily the latest operation state; inspect finish/acceptance evidence.

## Verify the intended scenario

A green aggregate gate does not prove a required scenario executed or passed.
Inspect its named job, selection and skip conditions at the exact source. Older
Notification work had a failed Compose case despite a green aggregate gate;
another PR skipped its relevant Compose check. Correct coverage or collect the
missing proof; never relabel a skip as acceptance. Nonverbose suite output does
not justify invented per-test PASS lines.

Negative tests need a successful control through the same setup. A Space tree
audit rollback test could pass because a closed database action constraint
rejected insertion before the intended injected failure. Update the registry
through an additive migration, prove insertion succeeds, then isolate the failure
and verify rollback. A negative result caused by a different error proves nothing
about the intended failure path.

Confirm that the selected gate actually invokes the regression helper; static
fixture checks do not exercise its runtime assertions. Separate download/runner/
dependency failures from a compiled assertion failure before assigning a fix.
Use the exact failed checkout and base diff, including blob identity where needed;
adjacent `git blame` lines or a passing unrelated master control do not attribute
the failure.

A successful HTTP history read or WS handshake does not prove delivery. Observe
the receiver's subscription acknowledgement and the exact sent message ID. Keep
a failed strict trace-correlation assertion separate from otherwise successful
functional observations; do not relabel the aggregate exit as E2E success.

## Inspect CI jobs and producer provenance

Inspect the run ID, attempt, source SHA and actual job IDs/status/conclusions.
The workflow label alone does not determine every job's state. A run with zero
jobs supplies no execution evidence; inspect prerequisites and selector outputs
before concluding that a needs-gated matrix job is missing. The GitHub
[workflow-jobs API](https://docs.github.com/en/rest/actions/workflow-jobs)
defaults to 30 jobs per page: request up to 100 and follow pagination. Its per-job
log endpoint may expose completed-job logs before whole-run CLI retrieval works.
Keep expiring redirects and raw logs private and report only bounded safe excerpts.

Check actual branch requirements as well as visible checks. A green listed check
does not substitute for a required `ci-gate` that has not materialized. Pending
source-triggered CI is not a reason for a cosmetic commit, bypass or blind retry.

For PR-event-dependent tests, distinguish immutable build provenance base `B0`
from the event's current base `B`. Immediately before execution inside the actual
test environment, compare the exported event/CI/API base and the merge checkout's
first parent when that checkout is the synthetic event merge. A checkout hash
preflight alone cannot validate stale inherited environment variables. Retain
invalid-environment receipts as invalid; do not weaken the policy to admit them.

When stdout lacks the image's build digest, an authenticated Buildx `.dockerbuild`
artifact can provide producer evidence. Validate descriptor bytes/hash, repository,
revision, context/platform, run attempt and successful build-step timing; an earlier
incidental digest is not the build result. The existing reader is
[`gis_dev_first_install.py`](../scripts/dev/gis_dev_first_install.py), with fixture
coverage in [`gis_first_install_test.py`](../scripts/dev/gis_first_install_test.py).

## Test acceptance readers before privileged execution

Run the complete reader against fixtures with actual producer paths, schemas and
container/image-map shapes. Parsing and narrow helper tests miss initialization,
journal-directory and composition errors. Include negative controls for each
claimed invariant; independently review and hash-pin the final executed bytes.

Compare new state with independently pinned historical evidence. New-checkpoint
self-consistency does not establish preservation. Verify namespace, marker and
PVC name/UID/PV UID as required by the protocol, pin the old checkpoint hash, and
recheck its bytes afterward. Keep failed evidence immutable as well.

Bound Kubernetes discovery and check selected init containers and sidecars, not
only primary images. For Pod asset reads, verify ownership and reread Pod identity
around execution; replacement during the read must invalidate the observation.

## Test the executed producers

Exercise the actual CLI argv, exit category and output bytes through the adapter
and consumer. A dict-returning mock cannot establish that the installed kubectl
accepts a selector. Use a synthetic no-write client dry-run when verifying CLI
formatting; never use a private live Secret to experiment with it. Check required
safe fields in the final persisted/returned artifact, not only its classifier.

Language namespace and scope are also part of that path. Bash functions use
[dynamic scope](https://www.gnu.org/s/bash/manual/bash.html#Shell-Functions):
an output helper using `printf -v` can update a wrapper's same-named local instead
of the caller's variable. Test the actual caller variable. Python
[`exec`](https://docs.python.org/3/library/functions.html#exec) with independently
created namespaces can define distinct exception classes. Inject the generated
producer's own exception type or exercise that producer; matching class names
alone do not test its typed handler.

Reproduce Kubernetes Secret projections rather than only plain temporary
directories: the kubelet's [AtomicWriter](https://github.com/kubernetes/kubernetes/blob/master/pkg/volume/util/atomic_writer.go)
uses `..data` and timestamped backing directories. Preserve required key-count,
ownership and symlink containment checks when supporting that layout.

Startup ordering requires a real process fixture. Point the blocking dependency
at a bounded unresponsive endpoint and verify trusted HTTPS independently; handler
tests and Compose dependency parsing alone cannot prove listener availability.
For issuer tools, exercise the exact reviewed image, UID, mounts, offline/network
policy, capabilities and read-only filesystem against separate test-only state.
Host OpenSSL tests do not prove `TemporaryDirectory` works without a writable
scratch mount. Retain live journals/volumes on failure; this fixture grants no
live retry or key access. Current DEV issuer contracts are in
[`scripts/dev/README.md`](../scripts/dev/README.md).

## Make diagnostics reliable and safe

Expose bounded, validated, allowlisted reports to ordinary agents instead of
requiring broad root-journal access or a person relaying logs. Include safe stage,
reason, source/run/operation association and evidence hashes, never credentials
or unrestricted raw errors.

Collection can fail independently of the application. Sending all stdout through
`head` under `pipefail` can obscure a small valid diagnostic, exceed parser limits
or cause SIGPIPE. Filter diagnostic candidates before existing bounds, drain
unrelated output, and retain producer failures, timeouts and malformed-marker
rejection. Test large unrelated output, producer exit failure, malformed,
duplicate and excess candidates. Do not convert collection failure into empty
success.

Distinguish observations from hypotheses: failed collection does not prove
overflow caused the application error. An initial visible build argument does
not establish the final diagnostic image when a later suppressed rebuild/recreate
exists. Inspect activation before changing product behavior.

Never mark a failed journal COMPLETE/PASS merely to make an inventory match. Use
only the protocol's reviewed, separate typed disposition, bound to exact failure
and retained source/build custody. Unknown failures and partial output still
block. Do not delete journals or repin immutable old receipts to admit an upgrade.
Before retrying, inspect the saved invocation, live handle, result and current
state. An observation timeout is not proof execution stopped, and create-only
destinations cannot be retried blindly.

### Secret metadata and schema

For discovery, use a metadata-only API/table view rather than fetching a private
Secret and hoping a formatter omits its payload. Formatting errors can expose
more than the intended names/counts. Test selectors with synthetic absent/present
labels and a fake secret sentinel. Select required identity/ownership/operation
metadata in one payload-free snapshot; separate GETs can straddle replacements.
Inspect certificates only through a path established to contain public material.

After an accidental private tool-output disclosure, stop that query path, do not
repeat values or delete audit history, and distinguish observed tool exposure from
unproven external compromise. Follow the established issuer/runbook and actual
authorization for any controlled rotation; disclosure alone grants no mutation.

Preserve the live Secret type, including `Opaque` when the guarded operation
admits it. In the [Secret schema](https://kubernetes.io/docs/concepts/configuration/secret/),
`type` is a top-level resource field, not a metadata field. Verify the actual
replacement payload as well as the metadata selector; tests for these separate
boundaries live in [`media_pki_test.py`](../scripts/dev/media_pki_test.py).

### Resumable operations and immutable evidence

Freeze consumed diagnostic packets before modifying a successor: create a new
versioned sibling and retain the exact executed stdin, wrapper, manifest/freeze
and status bytes. A later edit does not preserve historical executable custody.

Continue an existing forward-only operation only through its documented journal
and issuer custody. Test compatibility with older journals under the exact
required UID/resourceVersion/public-fingerprint pins; a missing new receipt field
does not authorize restarting, deleting receipts or weakening guards.

After successful Job creation, API defaults and controller labels may make the
live full spec differ from submitted bytes. Preserve its UID, operation intent,
protected desired fields and completion receipts, classify the exact difference,
and review any narrow normalization. A failed reconciliation check after DDL does
not authorize executing the completed Job again.

Post-completion readback is distinct from in-progress resume admission. A later
Deployment resourceVersion alone does not prove a changed committed pod template:
compare the operation's UID, full template, marker, observed generation and
readiness against its immutable receipt. Record mismatches and unknown causes;
do not rewrite the receipt, retry a completed operation, or relax resume guards.

## Integrate work without losing evidence

Strict branch freshness can leave a PR BEHIND despite successful CI on a synthetic
merge checkout. Inspect protection and current eligibility. Use normal merges;
coordinate related accepted histories to avoid serial master changes repeatedly
staling other candidates. Preserve stronger current authorization/lifecycle
behavior when resolving older branches.

Pending CI with zero jobs is not necessarily abandoned: earlier jobs can still
be progressing. Inspect exact handles and timestamps. Do not claim a proven
concurrency blocker when the API only supports an inference; follow task
authorization for CI controls.

Review an old PR's actual diff, ancestry, requirements and landed equivalents.
A deadline-test title concealed a large cross-service change and migration-number
collisions. Resolve collisions with additive migrations, never by overwriting
shipped migrations. Complete useful missing work; close superseded work only
with concrete replacement evidence.

A file introduced only by the feature branch can be absent on master without an
upstream deletion. Compare the old base to the new master and inspect the common
ancestor before recreating it or claiming integration overlap. Worktree/process
custody is maintained in [CONTRIBUTING](CONTRIBUTING.md#managed-worktrees-and-process-custody).

## Bound visual and runtime claims

Branch-scoped captures may not run on an integration branch. Reuse accepted images
only when relevant source/fixtures are proven unchanged; otherwise collect fresh
source-bound captures. Record artifact hashes and which images were inspected
versus reused by byte equality.

Fake-player screenshots do not prove decoding, and an image's asset manifest does
not prove public browser delivery. Verify the actual consumer required by the
task and state each observation's limits. Preserve failed attempts separately
instead of blending their evidence with a later successful source.

Before declaring a control absent, inspect the exact hash-verified PNG at original
resolution and the expected region; matching scaled thumbnails are not independent
absence evidence. Nonmodal overlays also require keyboard access and fully visible
actions, using the incomplete-onboarding fixture that actually retains the overlay.
Pointer access or title bounds alone do not prove those conditions.
