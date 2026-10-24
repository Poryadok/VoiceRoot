# CI-GATE-10M: preliminary after observations

This is a preliminary implementation measurement, not acceptance of a
ten-minute SLO. The first implementation phase has one completed wide PR head. The PR's earlier
development heads are shown below as cancelled observations, not quick passes.

| Head / CI run | Outcome | Gate wall | Initial queue | Execution envelope |
| --- | --- | --- | --- | --- |
| `309db86599e932b21dbf07fba2b6afb7a2c8d3c3` / [40079510504](https://github.com/Poryadok/VoiceRoot/actions/runs/40079510504) | success | 15m44s | 30s | 15m14s |
| `a3f21912a2bbe24eb427416e153c601fbb140009` / [40079247757](https://github.com/Poryadok/VoiceRoot/actions/runs/40079247757) | cancelled after developer corrections | censored | 35s | censored |
| `7c8ed10d229c5f100dd6a0486c2b655295a48908` / [40078888241](https://github.com/Poryadok/VoiceRoot/actions/runs/40078888241) | cancelled after developer corrections | censored | 31s | censored |
| `3f2fa0a222f15474ac0a38c0f4660348072971ce` / [40078821079](https://github.com/Poryadok/VoiceRoot/actions/runs/40078821079) | cancelled after developer corrections | censored | 4s | censored |

All four are GitHub attempt 1; they are successive source heads, not retries of
one head. Cancelled workflows are censored even if their final aggregator
reported failure. Their raw failed step outcomes remain diagnostic evidence.

The completed sample is CI/global with historical-validator and workflow
changes plus a frontend Docker contract-test change. Historical audit was
selected and passed, so this expensive change category is not excluded.
Successful after n=1 mean/p95 are both 944 seconds; one observation is not a
representative percentile estimate. The [baseline](gate-baseline.md) has
CI/global success n=3 mean20m12s/p95 23m34s and all-success n=16 mean9m10s;
different cohorts must not be blended to claim the target.

Observed current jobs (duration excludes their earlier queue):

| Job | Duration | Outcome |
| --- | --- | --- |
| current-diff-contract | 9s | PASS |
| voice-history-audit | 27s | PASS |
| ci-script-tests / policy | 90s | PASS |
| ci-script-tests / runtime | 99s | PASS |
| ci-script-tests / staging | 329s | PASS |
| ci-script-tests / proofs | 28s | PASS |
| flutter-unit-tests / slowest shard | 9m12s | PASS |
| compose-e2e | 14m20s | PASS |

The last Flutter shard completed at workflow+10m43s including queue; the wide
baseline Flutter job in run39385343073 took18m11s. These observations suggest
the split removes a substantial serial path; they are not a controlled
performance experiment. Compose's current stack build takes378s and its
all-feature smoke392s, making it the remaining broad PR critical path.

Next evidence: collect at least ten representative distinct PR heads, including
global/CI changes, preserve cancellations and failed runs, and publish scope
groups with queue/execution/attempts. Ordinary-component after latency and the
overall ten-minute average remain unverified. No PR #875 workflow was changed,
cancelled or rerun. Integrate this separate PR after that existing work, as the
task requests; an already-running old gate is not replaced by this policy.

## First Compose cache observation

Source 555ae546fc04f58ffe827709a990b39746958bac,
[run40081841614](https://github.com/Poryadok/VoiceRoot/actions/runs/40081841614),
attempt1, completed PASS. This is another CI/global head from the same PR, not a new
representative PR. Keep this cache phase separate from the preceding implementation
phase: n=1, mean/p95=1100s (18m20s), initial queue67s, execution envelope1033s (17m13s).
It is above the target and slower than the first observation.

Compose build/load296s plus startup66s compares with previous stack build/start378s;
smoke491s compares with392s; Bake post-processing37s and builder cleanup18s also belong
to gate wall-clock. Initial queue rose from30s to67s. The last Flutter shard waited252s
after its job was created and completed at workflow+13m46s. These uncontrolled runs
cannot attribute the overall difference solely to caching.

The build log contains472 CACHED records and successful per-target exporters. The
private Notification diagnostic rebuild/preflight consumed only about9.5s, so it is not
the observed smoke bottleneck. Initial host Notification Go download/compilation
consumed about100s; Gateway live tests themselves reported191s. Build layers and host
compilation may be reused, but all live tests must run again with count=1. An explicit
host dependency cache now hashes the actual Gateway, Notification and shared pkg module
sums. Its first hosted run seeds a cold cache; no ten-minute acceptance is claimed.

## Subsequent censored development heads

These pushes superseded development runs through ordinary PR concurrency; no manual
cancellation or rerun was requested. Neither workflow contributes a short terminal
duration to the SLO mean.

| Source head | Run | Outcome | Initial queue |
| --- | --- | --- | --- |
| 72745aa1e9e4771ecbf4cd6e40654f8c1b2499cd | [40080795912](https://github.com/Poryadok/VoiceRoot/actions/runs/40080795912) | cancelled; censored | 4.0s |
| 96aed431f921d8529a5e0f7e1aab4cb734966143 | [40081751202](https://github.com/Poryadok/VoiceRoot/actions/runs/40081751202) | cancelled; censored | 33.0s |

## Warm layer-cache experiment and removal

Source2032aa0262bffbd30529187936d4a5fa599a89b3,
[run40083676615](https://github.com/Poryadok/VoiceRoot/actions/runs/40083676615),
attempt1, PASS: gate1304s (21m44s), initial queue4s, execution envelope1300s. Compose
separately waited230s for a runner; that wait remains in gate wall-clock. This is
another head of the same CI/global PR, not an independent representative PR.

Bake build/load304s plus startup70s did not improve materially over original378s. Its
post-processing34s and builder cleanup18s add to wall-clock. Smoke488s passed. Logs
show508 CACHED records but also two recoverable web cache blob-not-found errors,
followed by142.7s base extraction and70.2s Flutter rebuilding; the web cache exporter
took53s including preparation. Cache storage existed and all25 exports completed, which
did not establish a useful latency gain.

The final implementation removes the Bake layer-cache consumer and its helpers,
restoring the original Compose build/start command. The independently proven host Go
cache fix remains: setup-go explicitly hashes Gateway, Notification and shared pkg
module sums, all Go live test commands rerun with count=1, and the isolated warm-cache
source-mutation proof must fail after a source change. This first hosted Go cache key
was cold-seeded; subsequent benefit is still preliminary. All experimental runs remain
visible above; none is discarded to improve the mean.

The cleanup candidate is validated on its exact head before integration. Its final
run link and per-head raw timing summary are published in
[PR880](https://github.com/Poryadok/VoiceRoot/pull/880) and the CI-GATE-10M tracker
attachments, avoiding a self-referential report commit that restarts the gate.
