# Canonical application package capture

For a reviewed full-build request, use the successful **master push** CI run,
its exact source SHA and attempt. The request must be included in that push;
cycle 1 may advance to cycle 2, and either cycle may be retried with a new
request ID. A request does not establish that a deployment cycle completed.
The script checks all 23 application
build/push jobs, the renderer, no image promotion, no automatic staging deploy,
the canonical stack lock, and the registry's immutable linux/amd64 identities.

From a clean checkout containing this reviewed verifier, on Linux:

```sh
GH_TOKEN="$GITHUB_TOKEN" python3 -B scripts/dev/canonical_application_package.py \
  --source-sha "$SOURCE_SHA" --run-id "$CI_RUN_ID" --attempt "$CI_ATTEMPT" \
  --output /secure/existing-directory/application-package.json
```

The output path must not already exist. The receipt records the exact source,
verifier revision, request, run/attempt, lock artifact/hash and immutable images;
it is written only after the complete capture and rechecks succeed. The source
authority may accept a later master under its existing backend/CI/catalog
equivalence rule. That rule **does not assert frontend equivalence**: the
receipt always identifies the original source and its exact image digests.

The receipt is an input to a separately fenced rollout, not deployment or
runtime qualification. DEV and STAGING must consume the same image identities;
the STAGING authority's independently resolved image map must exactly equal the
retained package map. Revalidate at use, preserve the request's rollout and
recovery evidence, and separately qualify migrations, credentials, external
assets, rollback and environment-owned frontend runtime configuration. No
secrets belong in this receipt. The required interval and source selection for
a later cycle are operator/workflow responsibilities.
