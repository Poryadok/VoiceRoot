# 2026-11-20 Фull CI verdicts на master — оба authoritative full-profile dispatches в одной ночь

Session: `Codex-PR-Merge` (OpenChamber session, model qwen3.8-flash-next-coder-iq1_m). Verdict-reading rule (`docs/todo/ci.md`, PR #1179): auto-subset push/PR runs are NOT verdicts; authoritative = `workflow_dispatch profile=full` с empty `publish_tag`/`publish_services` (or full-graph push run). Tonight two such runs fired on consecutive master tips.

## Samples

| Sample | Head | Run | Created UTC | Jobs | success/skipped/failure | Conclusion |
|---|---|---|---|---|---|---|
| A | `6315070bac926dece67ac71159475b79cd871a08` | #6069 = 44675790539 | 2026-11-20T04:38:41Z | 133 | 112 / 12 / 9 | FAILURE |
| B | `353656adda974050b69867a2b1b3efb1071aef79` | #6076 = 44682932169 | 2026-11-20T05:43:38Z | 133 | 111 / 12 / 10 | FAILURE |

Both dispatches carried empty publish inputs ⇒ publication jobs inert. Identical 12 SKIPPED in A and B: `affected-check-plan`, `backend-go-integration-evidence`, `ci-gate`, `ci-skip-gate`, `current-diff-contract`, `deploy-staging`, `minio-mc-image-publish`, `minio-server-image-publish`, `staging-images-dispatch-publish`, `staging-images-promote`, `staging-images-push`, `staging-stack-lock`. Publication stays a separate fenced ceremony — verdict runs never publish (rule of `docs/todo/ci.md`).

## Red families (A = 9 reds, B = 10; B \ A = {`a1-e2e`})

Every red in both verdicts sits inside the documented debt registry (`docs/todo/ci.md`):

- `backend-go (role)`, `backend-go-integration (space)`, `backend-go-integration-pr (space)` — Role native membership authority/invite decisions и Space Go ban-catalog shard (registered proof debt).
- `compose-e2e` — stable-red, native50 pin chain (registry run-#5996 entry).
- `full-validation-aggregation`, `local-ci-parity` — gate propagator и `make build-all` parity over the same primitive (registry entries, PR #1149/#1150 pins).
- `role-space-receipt-integration`, `space-native-installer-integration`, `voice-history-audit` — receipt proofs, Space native installer proof, history-audit debt (registry).
- `a1-e2e` — только в B: step `make compose-a1-multi-account-proof`, ci.yml job `a1-e2e`, already pinned `ubuntu-24.04`. Registry documents this class as debt (PR #1149 entry: «A1 multi-account compose proof») and documents the flake-flip semantics (`compose-db-init` infra-owned startup, registry verdict-reconciliation entries: A1 class flipes green↔red across authoritative runs). Green in A, red in B — documented flake flip, NOT a new class.

Zero new CI classes tonight.

## Matchmaking class — TT card `CI-MATCHMAKING-DEADLINE-INTERNAL-20261120`

- Red sample: head `aab06d19…`, run 44660093277, job 136036022408 — 1 FAIL of 381: `TestMatchDeadline_ConcurrentAcceptDeclineAndSweeperConverge`, expected `FailedPrecondition` (0x9), got `Internal` (0xd).
- Green sample 1: head `6315070b…` (run A) — trio green: `backend-go (matchmaking)` job 136087273256, `backend-go-integration-pr (matchmaking)` job 136087273358, `backend-go-integration (matchmaking)` job 136087273521.
- Green sample 2: head `353656add…` (run B) — trio green: jobs 136110506438 / 136110506548 / 136110506977.
- Correction to the class context: the job `backend-go-integration-pr` was ALREADY pinned `ubuntu-24.04` at the red head — `git cat-file blob aab06d19….github/workflows/ci.yml` line 1416 carries `runs-on: ubuntu-24.04`, same at tip. The red was observed on a pinned runner: the 26.04-alias-migration flake hypothesis does NOT apply to this class.
- Static root cause (read-only analysis, file:line evidence): the accept path yields `Internal` only via DB/redis I/O on the Expired-branch recovery projection — `match.go:171–173` returns the raw `err` before the expected `FailedPrecondition` at `:181`; the projection re-locks rows concurrently mutated by decline+sweeper (`store/matches.go:339–387` raw pgx `FOR UPDATE`/`Commit` sites). The test backdates `created_at` by EXACTLY `MatchAcceptWindow` (`match_test.go:451`, no margin; sibling test at `:336` leaves a deliberate 1s margin). Pool = `pgxpool.New` defaults with 10s connect timeout (`pkg/integrationtest/postgres.go:113`) — fixture-load contention surfaces as connect/reset-class pgx errors, consistent with the fast (1.13s) `Internal`. No deterministic product-logic route found on this path.
- Decision: retry policy keeps the card `available`; 2 independent green samples at 2 heads against 1 red. No code PR for the class — the pin already exists, «pin the job» mitigation is moot. Closure awaits either a reproduced failure with error text or an owner decision.

## Verdict и Reachable

- Two authoritative verdicts in one night: CI debt state is UNCHANGED vs the 2026-11-19 chain (`aab06d19` verdict, `docs/audits/2026-11-20-ci-full-gate-verdict.md`); every red in both runs is registry-documented.
- Consumers: no new TT cards from this audit. `CI-MATCHMAKING-DEADLINE-INTERNAL-20261120` stays open with receipts c1–c3. Nightly CI scheduler already upgraded to `profile=full` (taskId `04b70407-6ed8-469b-a167-7395bb603f76`); next full verdict tonight at current tip delivered by run #6076 itself.
