# A4 capability evidence gate

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
