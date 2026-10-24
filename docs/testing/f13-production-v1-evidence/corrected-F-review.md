# Corrected F complete-unit source applicability

Current d1727a21dbe7aa72385228b21be6465eac66da50; prior qualified current1c47dce37a88c2539c8dabb074d9eda46fb84999; required correction909090a34743902799306974d6f3ebabe3c02f10 (verified ancestor). Original historical composition fa1528a7c232a644218edf49811e2bb2e373105f and its startup defect remain historical facts.

Original complete unit was read directly from scripts/ci/testdata/f13-exact-j-547/historical-inputs.zip member objects/dc4a28dc896a6bc1da54fcc01aceba94eaa63fd4b1b1eb175eeb9dc8cb9c31a3, logical /fixed3071-unit/corrected-F/source-unit.json. Local unmodified extracted JSON: f54-original-unit.json. Exact complete old/current entries are in f54-d172-inventory.json, generated from the old unit and actual d172 Git ls-tree, not a handpicked changed subset.

Counts: 33 manual paths plus4 repair bindings,34 unique paths (three overlaps). 31/33 manual entries unchanged. Three/four repair entries unchanged.32/34 unique entries unchanged. Only drift:

* docs/microservices/voice-service.md: old0e717f7e9c3dad3560945127e6fea6dbe3e1f422 → current6d95089f9b55e46346b0dd722c8bf97c64a39565. Eight added lines describe the five exact signed Voice Role CheckPermission selectors using the existing private client and original two-second budget. No P3 recovery or activation rule altered.
* src/backend/voice/main.go: oldeb2c4f2b13aa361b5f8d01238546afb8d925458e → current540a91513cfd19d1499183302b3880a578951d56. Removes ordinary insecure ROLE_GRPC_ADDR permission client and assigns rolePerms to the existing protected rolegrant checker during its earlier construction. No recovery snapshot placement/effect binding change.

## Required repair preserved

Original909 fixes recovery beginning before final NativeDrain/ProjectionFence publication. Current main calls startSpaceMediaRuntime, then configured.RecoverySnapshot, then starts ResumePending loop; it does not start that recovery loop before the final media composition. Lifecycle Service.RecoverySnapshot still copies both Service and Effects; service.go, effects.go and recovery_publication_test.go are identical to the qualified repair bindings. TestRecoveryPublicationRetainsFinalBindingsDuringStartupMutation retains the positive and concurrent mutation assertions. Entire internal/spacelifecycle, internal/spacemedia, internal/spacebarrier and space_media_runtime.go are unchanged1c47→d172. No repaired startup invariant regression found.

## Evidence scope

This audit is source applicability for the whole33+4 unit, with exact all-path inventory. It is not a new runtime proof. Existing current Native A/B proof may support its actual executed source/dependency set and the changed private Role transport; it is not automatically proof of every F composition history or new retirement/Voice A3 relay activation. Stale Leave changes are outside these34 unique paths and need their separately recorded source/proof applicability. Shared/generated dependency changes likewise remain with the current whole-unit dependency audit.

The old unit explicitly has producer_only=true, runtime_pass=false, activation_authority=false and current_or_future_path_authority=false. Preserve those limits and the historical startup defect; do not relax old AUTHOR checks. A new production F54 applicability record can bind this exact current complete unit and separately reviewed evidence while retaining the original historical tuple/repair requirement. This read-only reviewer edited no source and executed no runtime/tests.
