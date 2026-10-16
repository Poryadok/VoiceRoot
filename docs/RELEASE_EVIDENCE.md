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

## Bound visual and runtime claims

Branch-scoped captures may not run on an integration branch. Reuse accepted images
only when relevant source/fixtures are proven unchanged; otherwise collect fresh
source-bound captures. Record artifact hashes and which images were inspected
versus reused by byte equality.

Fake-player screenshots do not prove decoding, and an image's asset manifest does
not prove public browser delivery. Verify the actual consumer required by the
task and state each observation's limits. Preserve failed attempts separately
instead of blending their evidence with a later successful source.
