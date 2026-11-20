# CI full-gate verdict 2026-11-20 — authoritative full-profile master dispatch at head `aab06d19`

- **Agent:** OpenChamber session (model qwen3.8-flash-next-coder-iq1_m, working dir
  `D:\Git\Voice`, branch `master`), audit nick **CI-Full-Gate-Verdict-20261120**.
- **Scope:** honest verdict for the authoritative full-profile `workflow_dispatch` of
  `.github/workflows/ci.yml` (workflow id 263230665, `profile=full`) run #6077 / id **44660093277**
  at `origin/master` head `aab06d19098ebc0f381f991751205fa44b771092`; classification of all reds
  against the documented debt-class registry (`docs/todo/ci.md` blob at `origin/master`); artifact
  root cause for the one red outside every registered class; no product code, workflow, inventory,
  or gate bytes touched.
- **Method:** `gh api` (`/actions/runs/44660093277`, `/actions/runs/44660093277/jobs` paginated,
  per-job steps endpoint), artifact download `gh run download 44660093277 -n go-integration-matchmaking-1`
  and offline parse of `go-integration-events.jsonl` (2010 events), `git cat-file blob` reads of
  `docs/todo/ci.md`, `src/backend/matchmaking/internal/grpcsvc/match_test.go`, and
  `.github/workflows/ci.yml` at `origin/master`, `git log` for the test-file history,
  job-verdict cross-check against prior runs 44329501457 / 44581878923 / 44589215159.

## Ground truth (verified 2026-11-20T02:20Z–04:0xZ)

- Run **44660093277**: `status completed`, `conclusion failure`, head `aab06d19098e…` (docs PR #1175
  merge commit; product bytes identical to tip `7d699cfd…` — `git diff --name-only aab06d19..7d699cfd`
  lists only `docs/STAGING_SERVER.md`, `docs/audits/README.md`, `docs/audits/2026-11-20-dev-ch-oom-reobservation.md`),
  created `2026-11-20T02:20:37Z`; **133 jobs = 111 success / 12 skipped / 10 failure**.
- REDS (10, with job ids where resolved): `backend-go (role)`; `backend-go-integration (space)`;
  `backend-go-integration-pr (matchmaking)` job **136036022408**; `backend-go-integration-pr (space)`
  job 136036022359; `compose-e2e`; `full-validation-aggregation` job 136064185949; `local-ci-parity`;
  `role-space-receipt-integration`; `space-native-installer-integration`; `voice-history-audit`.
- SKIPPED (12): `affected-check-plan`, `backend-go-integration-evidence`, `ci-gate`, `ci-skip-gate`,
  `current-diff-contract`, `deploy-staging`, `minio-mc-image-publish`, `minio-server-image-publish`,
  `staging-images-dispatch-publish`, `staging-images-promote`, `staging-images-push`,
  `staging-stack-lock`. `ci-gate` skipped — not red — so no branch-protection claim change.
- Registry-documented reds (9/10): Space/Role Go ban-catalog + native50 pin chain family
  (`backend-go (role)`, `backend-go-integration (space)`, `backend-go-integration-pr (space)`,
  `role-space-receipt-integration`, `space-native-installer-integration`, `compose-e2e`),
  chronic F13 `voice-history-audit` residual, aggregators/parity propagators
  (`full-validation-aggregation` over the same reds incl. the new class, `local-ci-parity` mirror
  over Space Go reds) — families registered in `docs/todo/ci.md` (2026-11-15 CI-rollup receipt +
  2026-11-19 tip verdict).
- Matchmaking root cause (artifact `go-integration-matchmaking-1`, `go-integration-events.jsonl`
  `event_sha256 1b3cbda8ca0522cf4d507f6cb7809872f40da021ad15d9455b8c3f743b5f95f7`, counts
  fail=1 pass=380 skip=0, exit_code 1): the single FAIL is
  `TestMatchDeadline_ConcurrentAcceptDeclineAndSweeperConverge` (1.13s) in package
  `voice/backend/matchmaking/internal/grpcsvc`; assertion `match_test.go:488`
  `require.Equal(t, codes.FailedPrecondition, status.Code(result.err))` on the **accept** branch of
  the concurrent accept/decline/sweeper deadline race printed
  `expected: 0x9` / `actual: 0xd` — gRPC status 9 FailedPrecondition expected, 13 Internal returned.
  `match_test.go` unchanged since `f7408b65979b…` (2026-10-16); job `runs-on: ubuntu-latest`
  (unpinned by the PR #1149 docker-class pin), Ubuntu 26.04 migration window closed 2026-11-19.
- Cross-run job verdicts (matched via jobs endpoints): `backend-go-integration-pr (matchmaking)`
  GREEN at 44329501457, 44581878923, 44589215159; `backend-go (matchmaking)` and
  `backend-go-integration (matchmaking)` GREEN in 44660093277 itself.
- Greens relevant to registry state in 44660093277: `backend-auth` + `backend-auth-image` +
  `chat-auth-java-integration` success (auth-java class proven green at tip — was "unproven
  (not re-run)" in the 2026-11-19 tip verdict); `t31-critical / compose-https-claim-inbox-ack`
  success (red at #6058/#6059); `ci-script-tests` 4/4 matrix sub-jobs success; A1 trio
  (`a1-attachment-restart-proof`, `a1-e2e`, `a1-flutter-profile-handoff`) success;
  gameintegration trio (`backend-go (gameintegration)`, `backend-go-integration (gameintegration)`,
  `backend-go-integration-pr (gameintegration)`) success — third consecutive green at fresh heads
  (#6058, #6059, this dispatch).

## Findings

1. **New red class: `backend-go-integration-pr (matchmaking)` deadline arbitration** — root cause
   above; NOT in the registry (`docs/todo/ci.md` blob has zero `matchmaking` class entries),
   three prior runs green, test unchanged ⇒ first-appearance red, root cause unresolved
   (deadline-race contention flake on the fresh Ubuntu 26.04 runner vs real product regression).
   Registered in `docs/todo/ci.md` by this PR with a TT card `CI-MATCHMAKING-DEADLINE-INTERNAL-20261120`
   (epoch A1, `available`). Reproduction at a fresh head owed before any product-code touch.
2. **auth-java entry retired** per its own policy ("Retire this entry when the class is green"):
   the class ran GREEN at this head (`backend-auth`, `backend-auth-image`,
   `chat-auth-java-integration` all success in 44660093277).
3. **`backend-go-integration (gameintegration)` class: third consecutive green at three distinct
   fresh heads** (#6058, #6059, this full dispatch at `aab06d19`) — still no independent greens at
   the same head and no reproduction, so the entry and TT card
   `CI-GAMEINTEGRATION-AUTHORITY-TLS-20261119` stay as recorded (neither retired nor re-proven).
4. **t31 HTTPS claim-inbox E2E receiver-DB proof ran GREEN at this head** (job conclusion success)
   where it was red at #6058/#6059 heads — consistent with the runner-drift record (migration window
   closed 2026-11-19) and pin `ubuntu-24.04` behaviour; entry stays open on its own debt evidence,
   no verdict flip claimed from a single green.
5. Verdict head is one docs merge behind the tip at this writing (`7d699cfd` = #1176 docs merge):
   the head delta is docs-only (verified via `git diff --name-only`), so the CI verdict transfers to
   the current tip with respect to product bytes; a fresh full dispatch on the new tip is not implied
   green by this record.

## Actions taken (receipts)

- TT `task create`: **1** — `CI-MATCHMAKING-DEADLINE-INTERNAL-20261120` (epoch A1, status `available`,
  workflow 2, full root-cause description). Refused attempt noted: `--problem` rejected
  (`problem explanation requires waiting_for_human status`) — card created without it.
- TT card state changes: **0** elsewhere; no `claim`/`test`/`accept`/`recheck`; no resource
  acquire/release on DEV or STAGING.
- GitHub: zero dispatches, zero pushes to product paths; one docs-only PR (this file +
  `docs/todo/ci.md` class section + auth-java retirement note + `docs/audits/README.md` index row).
- LAN cluster: untouched in this pass (read-only prior pass documented in
  `docs/audits/2026-11-20-dev-ch-oom-reobservation.md`).

## Verdict

At head `aab06d19…` the authoritative full-profile CI gate is **FAILURE with 9 reds inside
documented debt classes and exactly 1 new class** — the matchmaking deadline-arbitration
`Internal` (card `CI-MATCHMAKING-DEADLINE-INTERNAL-20261120`, available). The auth-java class is
proven green at tip and its registry entry retired. Publication evidence (publish run
44329726561 + GHCR gate 24/24 at `:d4af0992…`) and the live STAGING fence claim
`b73c1c98-1b13-43f9-a5db-9b046f71de6f` (gen 14, rev 55, deploy phase) are unchanged by this record;
the LAN root rollout ceremony remains the user's euid=0 step.

## Deliberately not done

- No CI re-dispatch or retry-chasing; verdict recorded as-is (retry policy per entry: no blind retries).
- No retirement of the gameintegration entry/card: greens at distinct fresh heads, not the same head.
- No retirement of the t31 entry: single green, entry stays on its debt evidence.
- No product-code, ci.yml, inventory, or gate-script bytes touched; no claim release or
  deployment-record changes.
