# Fleet audit 2026-11-18 — TT tracker ↔ GitHub reality ↔ CI debt registry

- **Agent:** OpenChamber session (model qwen3.8-flash-next-coder, working dir
  `D:\Git\Voice`, branch `master`), audit nick **TT-Fleet-Audit-20261118**,
  gate-actor label **Codex-PR-Merge**.
- **Scope:** reconcile active TT cards (voice project, server profile) against
  GitHub `Poryadok/VoiceRoot` reality (merged PRs, commit ancestry, check rollups,
  open PRs); cross-check `docs/todo/ci.md` debt registry against live red-CI
  classes at head; reconcile stale claims/custodies; report product-side blockers
  without fixing them.
- **Method:** `gh` API (PR lists, files API, `statusCheckRollup` snapshots — no
  CI-log endpoint chasing), `git merge-base` ancestry against `origin/master`,
  `tt` CLI (`task show`, `resource list`, comments/checkpoints/releases), OpenChamber
  `session.list` for worker-session liveness. Local `master` was 88 commits behind
  `origin/master` and had user uncommitted edits — audit used `origin/master` as the
  tip reference; local files were not touched.

## Ground truth (verified 2026-11-18)

- `origin/master` tip: **`d4af0992db2a50fac8eb94758e1020ad315f8be9`** = PR **#1166**
  (UIV3 Chat Info, branch `codex/uiv3-chat-info-r2`, head `696c5414…`), merged
  2026-11-17T19:00:39Z. **Zero open PRs** (`gh pr list --state open` → `[]`).
- Merged chain at tip: PR **#1163** `9988ec75` (auth-java debt docs) → PR **#1164**
  `6c89d18f` (BE-036 authority, head `705cdd72…`) → PR **#1165** `265975b8`
  (ClickHouse policy v2) → PR **#1166** `d4af0992`. Product pin `62b6fca8` (PR #1157);
  registry tip commit `44bd3c76` (PR #1162).
- Ancestry verdicts: `cabd1fce…` and `a68a8fba…` (develop-only merges for
  CR-20261102-10 / BE175 / CR-06 work, PRs #1101/#1112/#1095) are **not** ancestors
  of `origin/master`; `8e609115` (PR #1006) **is**.
- Head rollup of #1166: 72 checks; **FAILURE: `ci-gate`, `owned-enrollment`,
  `a1-attachment-restart-proof`**; Poryadok "All checks completed successfully" is a
  false pass on its own workflow.
- `docs/todo/ci.md` at `origin/master`: auth-java class (chat-auth-java-integration,
  backend-auth build/image dependency; consumers ci-gate + full-validation-aggregation)
  added via PR #1163; f974b501 aggregate diagnosis (ci-gate /
  full-validation-aggregation / local-ci-parity); class-3 over100 5s lease
  (`account_ban_large_integration_test.go:52`) remains open.
- STAGING observe r41 (recorded truth, quoted as-is): 33/33 deployments Available;
  16/23 app-scope images at tip; 7/23 pending rollout
  (admin, auth, developer-portal, notification, realtime, user, web);
  `STG-PKG-D4AF0992-20261117-V1` dispatch never made. STAGING resource: **still held**
  by claim of Codex-PR-Merge since 2026-11-17T19:54, state `recovery_required`
  (re-verified via `resource list` after the pass — deliberately not released, report
  only). DEV: `free`, 40/40 images matched at tip; ceremonies 38@44bd3c76 → 39@62b6fca8
  → 40@d4af0992.
- TT counts at pass time: in_progress ~51, review ~43-44, merging 6, dev_test 4,
  waiting_for_human 2 (GI-T13, OPS-STAGING-INITIAL-RECONCILE-144),
  stg_ready_for_deploy 86, dev_ready_for_deploy 17. TT revision before writes
  ~23181, after writes **23188**.

## Findings

1. **Stale card claims from dead workers** (evidence: claim SHA vs merged ancestry +
   idle OpenChamber sessions): UIV3-PANEL-CHAT-INFO, BE-036,
   OPS-DEV-UNPRIVILEGED-JOURNAL (claims 2026-11-16), A1-DEV-PR998-CANDIDATE (claim @
   `9988ec75`), A1-DEV-AUTH-USER-PROBE-DEADLINE-GC (claim @ `62b6fca8`) — holders
   Codex-PR-Merge / TT-DevTester-20261116, sessions idle.
2. **CLICKHOUSE pair dev_test custody stale**: OPS-CLICKHOUSE-OOM-20261021 /
   OPS-CLICKHOUSE-OOM-RECOVERY-450, tester TT-DevTester-20261116 idle since
   2026-11-17T10:54.
3. **STAGING claim held 24h** by idle Codex-PR-Merge session; blocks a fresh claimant
   from re-verifying the tip; report-only per audit mandate.
4. **Checkpoint tip refs stale** on CR-20261102-07/15/16: recorded tip `44bd3c76`
   (= PR #1162) while actual tip `d4af0992`.
5. **Develop-only merges kept honestly at `merging`**: PR #1101 (`a73d8a2d…`),
   `cabd1fce…`, `a68a8fba…` are not in `origin/master`; no promotion PR open —
   CR-20261102-06/10, BE175 stay `merging` (accurate).
6. **CR-20261102-16** `merging` with underlying PR #1006 (`8e609115`) already in master;
   next-action requires a hosted ci-gate ledger receipt — legitimately open (ci-gate red).
7. **UIV3-PANEL-CHAT-INFO**: deliverable on master via #1166; final DoD item
   "visible in merged UI" unmet (role_screen/chat_screen HTML unchanged at tip) → status
   kept `in_progress`, no promotion.
8. **GI-T13** `waiting_for_human`: acceptance of Google OAuth browser-consent evidence
   is human-gated; no autonomous transition legitimate.
9. **OPS-STAGING-INITIAL-RECONCILE-144**: honest `waiting_for_human` — gh token scopes
   (gist,read:org,repo,workflow) lack read/write:packages → GHCR staging dispatch
   `STG-PKG-D4AF0992-20261117-V1` cannot run from this operator; 86
   `stg_ready_for_deploy` cards wait on that rollout.
10. **Registry gap**: no TT card exists for merged docs-only PR #1163 (admissible
    documented-exception, flagged); two red classes (`owned-enrollment`,
    `a1-attachment-restart-proof`) are not registered in `docs/todo/ci.md`.
11. **Local worktree drift**: `docs/todo/ci.md` locally user-edited and 88 commits
    behind `origin/master` — not authoritative for the comparison.
12. Liveness caveat: TT actor labels are not verified identities; staleness judged by
    recorded SHA vs ancestry plus idle OpenChamber sessions.

## Actions taken (receipt-verified)

Executed via node scripts `ttbatch2.mjs`, `ttbatch3.mjs`, `ttbatch4.mjs`
(receipts `ttbatch2-res.json`, temp dir). Accepted ops:

- **16 `task comment`** (nick TT-Fleet-Audit-20261118): CR-20261102-07/06/10/15/16,
  BE175-EXISTING-VOLUME-ASSERTIONS, UIV3-PANEL-CHAT-INFO, BE-036,
  OPS-DEV-UNPRIVILEGED-JOURNAL, A1-DEV-PR998-CANDIDATE,
  A1-DEV-AUTH-USER-PROBE-DEADLINE-GC, GI-T13, OPS-STAGING-INITIAL-RECONCILE-144,
  CI-INTEGRATION-IMPACT-COVERAGE, OPS-CLICKHOUSE-OOM-20261021,
  OPS-CLICKHOUSE-OOM-RECOVERY-450.
- **1 `task checkpoint`** (actor Codex-PR-Merge): CR-20261102-07 — reconciled against
  tip `d4af0992`, replaces the 44bd3c76-era next-action.
- **2 `task pr`** (actor Codex-PR-Merge): UIV3-PANEL-CHAT-INFO → PR #1166;
  BE-036 → PR #1164.
- **5 `task release`** (stale claim reclaim, actor Codex-PR-Merge): UIV3-PANEL-CHAT-INFO,
  BE-036, OPS-DEV-UNPRIVILEGED-JOURNAL, A1-DEV-PR998-CANDIDATE,
  A1-DEV-AUTH-USER-PROBE-DEADLINE-GC.
- **2 `task release-custody`** (actor TT-Fleet-Audit-20261118, expected-owner
  TT-DevTester-20261116): OPS-CLICKHOUSE-OOM-20261021 (batch2 attempt failed → accepted
  on batch3 with fresh revision), OPS-CLICKHOUSE-OOM-RECOVERY-450 (batch2 attempt failed
  → accepted on batch4 retry with fresh revision).
- Failed attempts retried: 2 (batch2 custody pair, stale-guard/revision), 1 (batch3
  second custody, stale revision after first write). Final TT revision **23188**.

No status transitions were made anywhere; no deployment/test/acceptance records were
created; STAGING resource claim left intact; the 86 stg_ready cards and DEV ceremony
dispatch untouched; local worktree untouched.

## Verdict

For every merged-PR-related discrepancy above, the card record now carries an audit
comment anchored to verified reality; stale claims/custodies are released so fresh
workers can act; statuses were **not** advanced because ci debt (`ci-gate`,
`owned-enrollment`, `a1-attachment-restart-proof` red at head) legitimately blocks
`dev_ready_for_deploy` promotions and DoD/human gates remain genuinely open.

## Deliberately not done

- STAGING rollout `STG-PKG-D4AF0992-20261117-V1` never dispatched (needs gh
  packages scope / operator with write:packages; STAGING claim report-only).
- Promotion PRs for develop-only merges (CR-06/10, BE175) — none opened (needs
  write scope + product decision).
- Fresh-tester `task test` PASS on DEV-CHFIX-62B6FCA-20261117-V1 (CLICKHOUSE pair),
  fresh tester verdicts (dev_test), human acceptance gates (GI-T13,
  D110-PR1006-DELIVERY at `review`).
- Creating a TT card for PR #1163 or registering the two unregistered red CI classes
  in `docs/todo/ci.md` — flagged, not invented.
- Syncing the stale local `master` (88 behind) over user's uncommitted edits.

## Self-corrections

Earlier chat summary of this pass over-reported: claimed "2 custody releases for
A1-DEV-CANDIDATE-CONSISTENCY and STAGING" — **never executed** (STAGING claim is
report-only, confirmed still held); claimed PR links/checkpoints on CR-07/15/16 and
CLICKHOUSE pair beyond the receipts — only 2 `task pr` and 1 `task checkpoint`
were accepted. The numbers above come from `ttbatch2-res.json` and the post-pass
`task show` scan (`tt-post-show-20261118.json`).

## Evidence pointers

- TT receipts: dashboard `/?project=voice&task=<CODE>#comment-cN` (nicks above).
- Receipt files (ephemeral temp): `ttbatch2-res.json`, `ttbatch2/3/4.mjs`,
  `tt-post-show-20261118.json` under `%LOCALAPPDATA%\Temp\opencode`.
- gh snapshots: `rollup1166.json` (3 FAILURE of 72 checks), closed-PR merge SHAs
  above; `git merge-base` outputs for `cabd1fce…`/`a68a8fba…`.
