# Fleet audit 2026-11-17 (12:00 sweep) — TT tracker ↔ GitHub reality ↔ CI debt registry

- **Agent:** OpenChamber session (model qwen3.8-flash-next-coder, working dir
  `D:\Git\Voice`, branch `master`), audit nick **TT-Fleet-Audit-20261117**,
  write-actor label **Codex-PR-Merge** (resume/status/checkpoint writes) and
  **TT-Fleet-Audit-20261117** (custody reset, comments). This pass acted on
  re-anchor comments left by the 2026-11-17T05:20 pass of the same nick.
- **Scope:** cross-check TT (project `voice`) card statuses against repo reality
  (open PRs, `origin/master` ancestry, merged-but-not-advanced cards), reconcile
  stale claims/owners backed by dead worker sessions, advance merged cards with
  real merged-SHA/PR evidence only, verify the `docs/todo/ci.md` red-class debt
  registry, and report discrepancies. No product fixes, no deployments, no
  fabricated evidence, no deployment/test/acceptance records invented.
- **Method:** `gh pr list/view` (open/merged sets, `mergeCommit` fields), `git`
  (`git log --oneline origin/master` merge-subject scan, `git branch -a
  --contains`) for ancestry of merged commits; `tt` CLI (`task show`,
  `task comments`, resume/status/comment/checkpoint, `help`) against the
  tracker; OpenChamber `session.list` for worker-session liveness. Local
  checkout untouched; the 2026-11-17 earlier audit notes about a broken inline
  PowerShell `$`-loop were superseded by writing all commands to `.ps1` files
  run with `powershell -NoProfile -File`.

## Ground truth (verified 2026-11-17 during this pass)

- **Zero open PRs** in `Poryadok/VoiceRoot` (`gh pr list --state open` → empty);
  therefore "open PRs without evidence comments" is vacuously satisfied.
- `origin/master` tip at pass time: **`44bd3c768c852007791d52d3a3fbf551a854c628`**
  (PR **#1162**, merged 2026-11-16). Product pin **`62b6fca…`** (PR **#1157**) =
  the DEV/STAGING observed commit; DEV ceremony 39@62b6fca pending tester
  receipts (live claim, untouched by audit).
- Merged-on-master set verified as ancestors of `origin/master` (git log merge
  subjects + `branch -a --contains`): PR **#1134** `3d1cc99849d97540117e160cbc4607f9e5da1d40`
  (branch `codex/port-cr07-membership-pagination`, merged 2026-11-12T00:22:07Z),
  PR **#1135** `bdc9ec8772a2638cabb7ef0442c0fbfac9744a24`
  (`codex/port-cr1214-session-rotation`, 2026-11-11T15:51:32Z), PR **#1136**
  `8e3c5bf74f6a6e7c5393cf19bfc374bf1498e18e` (`codex/port-cr1516-attachment-lifecycle`,
  2026-11-12T00:22:11Z), PR **#1137** `4c75a58cd164508b12f1038315ccd5283b446878`
  (`codex/port-be059-squad-teardown-test`, 2026-11-12T00:22:16Z), PR **#1089**
  `1e9378a8e312752b3d002c10658b4166e72d7dec` (base master, merged 2026-11-07T16:13:58Z).
- **Develop-only limbo:** PR **#1125** merge `36b2b27525d9c0b13d9c30860969255db3b4b29e`
  (2026-11-07T15:27:32Z) and PR **#1123** merge `41ef933dcde792b005cc2245bc34641f374a5dd2`
  (2026-11-08T15:26:59Z) are on `origin/develop` (tip `c9be8f5b…`) and **not**
  ancestors of `origin/master`; no promotion PR for them exists (zero open PRs).
- Card↔PR mapping corroborated by branch names: PR #1134 → CR-20261102-07,
  PR #1135 → CR-20261102-12/13/14, PR #1136 → CR-20261102-15/16, PR #1137 →
  BE-059 (branches `port-cr07`, `port-cr1214`, `port-cr1516`, `port-be059`).
- STAGING observation (recorded by prior tester pass, quoted): 7/23 app images
  still pending data-preserving rollout at the observed pin; DEV resource claim
  live (Codex-PR-Merge, OPS-CLICKHOUSE-OOM pair, deployment
  `DEV-CHFIX-62B6FCA-20261117-V1`).
- TT stage counts after this pass: `review` 43, `merging` 6, `dev_ready_for_deploy`
  18, `in_progress` 51, `stg_ready_for_deploy` ~82, `waiting_for_human` 1
  (GI-T13); other stages empty.

## Findings

1. **Merged-but-not-advanced cards (reconciled this pass):** CR-20261102-07/12/13/14/15/16
   and BE-059 sat at `review` with `owner=""` while their fix PRs (#1134–#1137) were
   already merged onto `origin/master` (ancestry verified). Stale evidence claims on
   those cards pointed at develop-only original PRs (#1096/#1099/#1102/#1107 etc.).
2. **CI-AFFECTED-CHECK-PLAN stuck `in_progress`, owner empty:** its PR #1089 is
   merged on master (`1e9378a8`), but the hosted current-head ci-gate criterion
   remains genuinely pending (ci-gate red per `docs/todo/ci.md`) → reset, not advanced.
3. **Develop-only limbo review cards:** BE-STORY-FILE-MTLS-CLIENT (PR #1125) and
   A1-GIS-REV32-VERIFIER (PR #1123) hold at `review` with merged-into-develop
   evidence; fix not on master, no promotion PR. Honest hold + audit comments.
4. **`docs/todo/ci.md` registry gap:** the develop-only debt list enumerates
   #1093–#1104, #1106–#1113, #1117 but **omits #1123 and #1125**, which are
   merged-into-develop with no promotion PR — reported to dispatcher, docs not
   edited by this audit. Registry otherwise consistent: three red classes
   (ci-gate, full-validation-aggregation, local-ci-parity), tip matches
   `44bd3c76`, !1160/!1161 wording confirmed (merge snapshots carry ci-gate
   FAILURE rollups, both actually merged `269f642b`/`9d810e9`), !1105/!1131
   CLOSED UNMERGED ✓, "No open PRs remain" ✓.
5. **35 stale `in_progress` cards with `owner=""`** (dead-worker custody drift):
   A1-DEV-AUTH-USER-DB-CREDENTIAL-ISOLATION, A1-NATIVE-WINDOW-AUTOVACUUM,
   A1-REALTIME-COMPOSE-MIGRATIONS, A2-USER-CAPTURE-* (codex-a2-rollout), BE-019,
   CL-022, 20+ UIV3 Ursula-* cards, etc. Evidence of staleness: OpenChamber
   `session.list` — 40 sessions, only this audit (and main "войс доделе") busy,
   the rest idle.
6. **82 stale `stg_ready_for_deploy` cards** (owner TT-DevTester-20261116, assignee
   Codex-PR-Merge) vs STAGING observation showing 7/23 images still pending the
   data-preserving rollout; advancing would require fabricating task-bound STG
   records → report only.
7. **Unpushed local branch** `codex/marusya210-pr1057-repair` — work not represented
   on any remote.
8. Promotion PRs **!1105/!1131 closed unmerged** (consistent with ci.md). Three
   `merging` cards (BE175-EXISTING-VOLUME-ASSERTIONS ← PR #1112 `a68a8fba`,
   CR-20261102-06 ← PR #1101 `a73d8a2d`, CR-20261102-10 ← PR #1095 `cabd1fce`)
   are accurately `merging` awaiting dedicated develop→master promotion.
9. Liveness caveat: TT actor labels are not verified identities; staleness judged
   by recorded SHA vs merged ancestry plus idle OpenChamber sessions.

## Actions taken (receipt-verified)

All writes via `tt` CLI, `--project voice`; ASCII strings; receipts under
`%LOCALAPPDATA%\Temp\opencode` (`ttwrite.ps1`, `ttcheckpoint.ps1`,
`ttreset5.ps1`, `ttcomments.ps1`, `ttverify2.ps1`).

- **`task resume` ×7** (actor Codex-PR-Merge): CR-20261102-07/12/13/14/15/16, BE-059
  (reclaim custody of `owner=""` `review` cards; reason = audit reconciliation,
  next-action = post-transition plan).
- **`task status` ×12:** CR-20261102-12/13/14 + BE-059 → `merging` → `dev_ready_for_deploy`
  (basis: real merged PRs #1135/#1137 on master **and** ci-gate ledger rows recorded
  in `docs/todo/ci.md`); CR-20261102-07/15/16 → `merging` only (basis: real merged
  PRs #1134/#1136 on master; `dev_ready_for_deploy` withheld — no ci-gate ledger
  receipt for those merged PORT PRs, open red-class debt at tip `44bd3c76`);
  CI-AFFECTED-CHECK-PLAN → `available` (after **1 `task resume-custody`** with
  `--expected-owner=` reclaiming the empty owner).
- **`task checkpoint` ×7** (actor Codex-PR-Merge): set explicit post-transition
  `next_action` on all 7 reconciled cards. Failed attempts: 2 `task checkpoint`
  calls rejected on the two unowned `review` cards (owner claim required) →
  replaced by comments.
- **`task comment` ×10** (nick TT-Fleet-Audit-20261117): 7× `[RECONCILE 2026-11-17]`
  (review→merging[→dev_ready] with merged PR/SHA receipts), 1× `[RESET 2026-11-17]`
  on CI-AFFECTED-CHECK-PLAN (in_progress→available, pending criteria re-anchored),
  2× `[AUDIT 2026-11-17]` audit-hold comments on BE-STORY-FILE-MTLS-CLIENT and
  A1-GIS-REV32-VERIFIER (develop-only limbo, PR SHAs, registry-gap note).
- Stage shift verified post-pass (`ttverify2.ps1` + `task show` scan): review 50→43,
  merging 3→6, dev_ready_for_deploy 14→18, in_progress 52→51. Post-state re-check:
  12/13/14/BE-059 = `dev_ready_for_deploy` (owner Codex-PR-Merge), 07/15/16 =
  `merging`, two limbo cards = `review` (no owner), CI-AFFECTED = `available`.

No status transition, deployment, test, acceptance or resource record was invented;
every transition names a real merged PR + verified ancestor SHA in its reason.

## Verdict

After the pass: the seven merged-fix cards no longer sit at `review` behind dead
custody; the four with ci-gate ledger receipts are `dev_ready_for_deploy` (awaiting
the genuine DEV deployment ceremony + tester `task test`), the three without receipts
wait at `merging` with next_action demanding the hosted ci-gate ledger receipt at
current head; CI-AFFECTED-CHECK-PLAN is back `available` with re-anchored criteria;
the two develop-only-limbo cards stay `review` with explicit audit comments. The
`docs/todo/ci.md` registry is consistent at tip `44bd3c76` except the newly found
gap (#1123/#1125 missing from the develop-only debt list). Remaining stale custody
(35 `in_progress`, 82 `stg_ready_for_deploy`) is reported, not bulk-reset.

## Deliberately not done

- **35 stale `owner=""` `in_progress` cards** — reported only; bulk reset would
  erase per-card owner decisions the dispatcher must make.
- **82 `stg_ready_for_deploy` cards** — not advanced; STAGING rollout genuinely
  pending (7/23 images); fabricating STG task-bound records is forbidden.
- **BE-STORY-FILE-MTLS-CLIENT / A1-GIS-REV32-VERIFIER** — not advanced (fix not on
  master); no promotion PRs opened for #1123/#1125 (write scope + product decision).
- **3 `merging` cards (BE175, CR-06, CR-10)** — left as-is, accurately awaiting
  dedicated develop→master promotion per `docs/todo/ci.md`.
- **GI-T13 `waiting_for_human`** — legitimate human Google OAuth consent gate.
- **DEV claim ceremony** (OPS-CLICKHOUSE-OOM pair, `DEV-CHFIX-62B6FCA-20261117-V1`)
  — live, untouched; STAGING claim free/observation untouched.
- **`docs/todo/ci.md` edit** — registry gap reported to dispatcher, not edited here.
- **Unpushed `codex/marusya210-pr1057-repair`** branch — report only.

## Self-corrections

- An earlier mid-pass inline PowerShell `$`-loop ancestry check produced false
  **NOT-ON-MASTER** artifacts for PRs #1134–#1137/#1089; superseded in this pass by
  a `.ps1` file run (`git log --oneline origin/master` merge-subject scan +
  `git branch -a --contains`), which confirmed all five as master ancestors —
  the verdicts above use the corrected results.
- `gh pr list --search "number:1093..1162"` is invalid; the merged PR set came from
  `--state closed` (merged included) with jq filtering on `mergeCommit.oid`.

## Evidence pointers

- TT receipts: dashboard `/?project=voice&task=<CODE>#comment-cN`, nicks
  TT-Fleet-Audit-20261117 / actors Codex-PR-Merge.
- Receipt files (ephemeral temp): `ttwrite.ps1`, `ttcheckpoint.ps1`,
  `ttreset5.ps1`, `ttcomments.ps1`, `ttverify2.ps1`, `gitcheck.ps1` under
  `%LOCALAPPDATA%\Temp\opencode`.
- gh/git: merge-commit SHAs and dates above; `git branch -a --contains` outputs
  showing `remotes/origin/master` for `3d1cc998…`/`bdc9ec87…`/`8e3c5bf7…`/
  `4c75a58c…` vs only `remotes/origin/develop` for `36b2b275…`/`41ef933d…`.
- Session liveness: OpenChamber `session.list` (40 sessions; only this audit
  busy) — recorded during the pass.
