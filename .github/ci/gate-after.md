# CI-GATE-10M: first observed after run

This is a preliminary implementation measurement, not acceptance of a
ten-minute SLO. Only one completed wide PR head is available. The PR's earlier
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
