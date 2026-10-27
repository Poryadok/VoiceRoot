# A4 capability evidence gate

For A5 isolated Windows runner package input, see
[`windows-test-feed/README.md`](windows-test-feed/README.md). Its disposable
signature/feed preparation does not qualify native install/update or G1 trust.

`capability_manifest.py` implements the offline package boundary from PLAN A4/G1.
It emits a deterministic negative-suite plan and validates source-bound runner
observations. It never executes product probes, changes flags, deploys, or attests
the truth of external observations. A successful compile is not qualification.

Start with `capability-manifest.example.json`. Its placeholder SHA deliberately
fails validation. Replace the service inventories with the complete **observed**
canonical service IDs from the exact release package: images, rollout, readiness
and alerts. The single-service sample is illustrative, not a deployable profile.
Federation and aliases/unrecognized services are rejected from every inventory.
Do not infer that a service absent from this file is absent from a real cluster.

The configuration projection is an operator assertion from the effective config
validator, not an environment inspector: production/staging only, no development
defaults, no test checkout. This tool rejects contradictory declarations but does
not discover secrets, environment defaults or runtime drift. Supply the separate
effective-config validator evidence with the release package. Enabling a paid
capability requires a later versioned profile and its G2 gates, not editing this
alpha policy. `unfinished_rich_types` covers unsupported rich payload types;
basic A1 attachments remain in the free path.

Generate the suite using the immutable source resolved independently by the
protected release caller (never derive the expected SHA from the input itself):

```bash
python3 scripts/release/capability_manifest.py candidate.json \
  --expected-source-sha "$RELEASE_SOURCE_SHA" > negative-suite.json
```

The suite contains UI, REST and WS cases for every disabled entry, including
Federation. Concrete runners own feature selectors, routes, protocol requests,
authorized fixtures and expected deny/absence assertions. A case means the
feature remains unavailable **and produces no prohibited effects**. Missing
transport support must be proved as absence/unreachability; it is not a SKIP.
No guessed product endpoints or synthetic PASS results are generated here.

Runners produce one report with this shape, using the generated digest and all
18 case IDs. Each observation must reference portable actual evidence containing
its build/deployed-source identity and deny/no-effects assertions:

```json
{
  "schema_version": 1,
  "source_sha": "<full release SHA>",
  "manifest_sha256": "<generated candidate digest>",
  "observations": [
    {"case_id": "paid.rest", "result": "PASS", "evidence_uri": "artifact://actual-run/paid-rest.json"}
  ]
}
```

The shortened example is intentionally incomplete and fails qualification. The
verifier rejects missing/duplicate/extra cases, FAIL/SKIP, local paths, credential
URIs and source/digest mismatches. Unknown JSON fields, duplicate keys and
non-finite numbers fail closed. Validation emits no input values on failure.

```bash
python3 scripts/release/capability_manifest.py candidate.json \
  --expected-source-sha "$RELEASE_SOURCE_SHA" --results actual-observations.json
```

Exit 0 with `result=PASS` means **offline evidence completeness only**. It does
not replace runner execution, protected artifact provenance, independent review,
CI, deployment or appointed acceptance. The A4 operator qualification consumes
this output alongside effective config, admission, compatibility and product
proof; runtime adapter wiring and actual alpha negative qualification remain
open until independently executed. Existing deploy paths are not relaxed.

Regression tests run in `make ci-script-tests`, or directly:

```bash
python3 -m unittest discover -s scripts/release -p capability_manifest_test.py
```

## A4 client compatibility package

`client_compatibility.py` compiles the PLAN A4/A5/G1 matrix and checks offline
evidence completeness. The operator supplies exact observed tuples and immutable
previous/current artifacts. This repository does not yet declare qualified
Windows/browser support. No sample tuple or unit-test fixture constitutes support.

The manifest has exactly these fields:

- `schema_version`: integer `1`; `source_sha`: exact current release commit;
  `matrix_version`: explicit revision; `regression_sampling_policy`: nonempty
  documented policy for **extended regression only**.
- `backends`: `previous` and `current`, each with `source_sha` and nonzero
  `artifact_sha256` of the complete immutable backend package. Current source
  must equal the release source. Retain the package's service-image inventory.
- `clients`: `windows` and `web`, each with `previous` and `current` records
  containing semantic `version`, exact `source_sha`, and `artifact_sha256` of
  the immutable installer/Web asset package. Versions and artifact hashes must
  differ between previous/current. Preserve the actual signed feed/asset manifest
  and build provenance alongside the package.
- `tuples`: nonempty Windows **and** Web arrays represented as one list. Each
  entry has unique `id` (lowercase letters/digits/hyphens), `platform`, `os`,
  exact `os_version`/build and `arch`. Windows `os` is `Windows`; Web entries
  also require `browser` and numeric exact `browser_version` (no channel aliases,
  ranges or wildcards). Duplicate tuple identities are rejected.

```bash
python3 scripts/release/client_compatibility.py matrix.json \
  --expected-source-sha "$RELEASE_SOURCE_SHA" > compatibility-suite.json
```

Every tuple gets all four previous/current client/backend smoke combinations.
Each Windows combination also gets `version`, `forced_update`, `update_url`.
Runner assertions follow [updates.md](../../docs/features/updates.md): normal,
optional-update and below-minimum behavior; `/version` remains reachable; other
API requests are denied with426 when outdated; exact update URL reaches the
client's manifest handoff. Windows UI blocking/deferral and actual update behavior
must appear in portable runtime evidence. Web smoke checks real old/current
assets and API compatibility, including cache transition, rather than inventing
desktop `/version` UI (the Web exception in updates.md remains applicable).

Every tuple also gets a **sequence**: update previous client to current against
current backend, retain client/session, roll backend back to previous, then prove
the updated client still works. A standalone current/previous request does not
prove this rollback case. Each runner's raw report must identify observed OS,
arch, browser/build, client artifact, effective backend package, steps, assertions
and failures. No runner execution or observation truth is attested by this tool.

Pass actual observations via `--results actual.json`. Report fields are integer
`schema_version:1`, exact `source_sha`, generated `manifest_sha256`, and
`observations`. Each observation has exactly `case_id`, `result:"PASS"`, a
portable credential-free `https://` or `artifact://` `evidence_uri` without query
or fragment, and nonzero `evidence_sha256` of its raw evidence. All cases must be
present exactly once. Input duplicate keys, unknown fields, stale material,
FAIL/SKIP, missing tuples and incomplete case sets reject. Evidence hashes are
declared content identities; the protected caller independently retrieves and
verifies evidence and provenance. Never derive expected source from the candidate.

Exit0/resultPASS means **offline evidence completeness only**. Actual per-tuple
smoke, trusted install/update, protected environment custody, current CI,
independent review and appointed acceptance are separate required evidence.
Tests use synthetic versions/hashes and run in `make ci-script-tests` or:

```bash
python3 -m unittest discover -s scripts/release -p client_compatibility_test.py
```

## A4 rollout evidence completeness

`a4_rollout_evidence.py` checks that a rollout packet binds staging and
production to one caller-supplied exact source SHA, immutable service image
digests and the same migration content inventory with an explicit PASS result
and portable migration evidence digest for both environments. Staging evidence
must also declare a non-empty synthetic sample, a positive observation window,
its operator-supplied SLO rollback condition, and a separately exercised
rollback trigger using the same metric, comparator, and threshold, with
updated-client compatibility evidence.

The candidate manifest and evidence report contain content digests for the
portable raw evidence. Metric names, comparators and thresholds are supplied
by the authorized release policy; this tool does not select SLOs or inspect the
target. Use a source SHA resolved by the protected caller, never one derived
from the candidate itself:

```bash
python3 scripts/release/a4_rollout_evidence.py candidate.json evidence.json \
  --expected-source-sha "$RELEASE_SOURCE_SHA"
```

Exit 0 means **offline evidence completeness only**. It does not execute or
verify deployment, migrations, canary observations, rollback, CI, resource
custody or acceptance. Unknown fields, duplicate JSON keys, stale source,
mutable image tags, inventory drift, missing samples/windows and a rollback
measurement inconsistent with its declared comparator fail closed. No real
cluster or production authority is changed by the tool.


Focused synthetic regression tests run in `make ci-script-tests` or directly:

```bash
python3 -m unittest discover -s scripts/release -p a4_rollout_evidence_test.py
```
