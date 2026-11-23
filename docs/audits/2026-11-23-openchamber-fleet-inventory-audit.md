# 2026-11-23 TT fleet inventory audit (OpenChamber-Fleet-Inventory-20261123)

Agent: OpenChamber-Fleet-Inventory-20261123 (OpenChamber session, model qwen3.8-flash-next-coder-iq1_m). TT write-actor nick for the pass: `OpenChamber-Fleet-Inventory-20261123`.

## Scope and method

Routine TT-fleet inventory audit (TT ↔ GitHub reality ↔ docs registries), same axes as the 2026-11-21/22 passes:

- TT census per status via `tt task list --status X --compact` (TT CLI at `http://192.168.0.109:30990`, project `voice`); unfiltered `task list` errors (server workflow-2 profile). Legacy status names (`todo`, `doing`, `pr_open`, `stg_doing`, `stg_ready_for_accept`) queried and returned total 0 — label drift, not missing data.
- `task show` full JSON for the merging pool, wfh trio, done pool `events[]`, fences via `resource show STAGING_SERVER`/`resource list` — raw bytes captured via node `spawn` (PowerShell `>` redirect writes UTF-16 and corrupts captured JSON; that was a live self-correction during this pass).
- GitHub reality: `gh pr view` JSON for 5 open PRs and 7 merging-card PRs (`--json number,state,mergedAt`); `docker manifest inspect ghcr.io/poryadok/voiceroot/<name>:<tag>` (public GHCR, no login) for the 7-changed tag-pin check and content-address `name@sha256:<digest>` resolves for 16 unchanged + renderer; negative sanity probe (tampered digest ref) must NOT resolve.
- CI contract: parsed all job keys of live `.github/workflows/ci.yml` at capture tip and re-derived `.github/ci/branch-protection-checklist.md` job-set; read `scripts/verify-required-jobs.sh` + `gate-baseline.json`; staging image catalog `scripts/ci/staging-image-catalog.json` (23 names) + nats-hub-config-renderer = 24 gate refs.
- Delivery worktree: `D:/Git/Voice/tmp/tt-fleet-inventory-audit-20261123`, branch `codex/tt-fleet-inventory-audit-20261123` from `origin/master` at `2285c041c4f0` (created with `git worktree add`; the first attempt used invalid `git worktree create` and silently created nothing — corrected, see Self-corrections).

Capture anchors: TT census revision 23672; `origin/master` tip at census time `0d43054a7f8b1ed10e2a52e1a4c84243c0e0966c` (= merge of PR #1199, the 2026-11-22 inventory-audit docs PR). Delivery base `2285c041c4f0` (PR #1197 merge): between the anchors only UIV3 merges landed (`#1193`, `#1195`, `#1197` + their branch commits); `git diff 0d43054a..2285c041 -- .github` is EMPTY, so every CI-contract conclusion below holds for both anchors.

## Ground truth

- Census (TT rev 23672, total 1290): `available` 897, `blocked` 94, `in_progress` 62, `waiting_for_human` 3, `review` 45, `merging` 7, `dev_ready_for_deploy` 17, `dev_deploying` 0, `dev_ready_for_test` 0, `dev_test` 2 (`OPS-CLICKHOUSE-OOM-20261021`, `OPS-CLICKHOUSE-OOM-RECOVERY-450`), `stg_ready_for_deploy` 86, `stg_deploying` 0, `stg_ready_for_test` 0, `stg_test` 0, `acceptance` 0, `done` 77, `archived` 0, legacy statuses 0. Delta vs 2026-11-22 inventory pass (rev 23622): in_progress 61→62, available 898→897 — status-math only, explained by ongoing claims; done/blocked/review/merging/stg_ready/stg_* unchanged.
- Epoch list coherent: `H(0)…DEFERRED(10)` present, titles reconciled (A4 title already reconciled 2026-11-21, no action owed).
- `waiting_for_human` trio (owners recorded, no transitions made):
  1. `GI-T13` — needs a human Google OAuth client registration step; its `events[]` show claim/transition history; owner-gated.
  2. `OPS-STAGING-INITIAL-RECONCILE-144` — epoch A1; its closure is subsumed by the STG rollout ceremony still in flight (fence holds it); owner-gated.
  3. `OPS-STG-LAN-INSTALLED-COHORT-ROOT-V1` — owner `Codex-PR-Merge`, euid-0 LAN node gate; frozen rerun BANNED (authority-family drift root cause `docs/audits/2026-11-20-lan-retirement-authority-drift.md`); next_action per checkpoint e4; its new blocker is the all-green full-graph master push, tracked by card `OPS-STG-LAN-CAPTURE-GREEN-PUSH-V1` (`in_progress`).
- DEV fence (`resource show`, raw): `state: free`, operation 52 outcome PASS, released `2026-11-20T13:43:06Z` (CH OOM pair, deployment record `DEV-CHFIX-62B6FCA-20261121-V14`, commit `62b6fca83b606c4f458f190d20a7e8e02610168c`), `"stale": false`.
- STAGING fence (`STAGING_SERVER`, raw bytes, rev 57): `operation 12`, `generation 14`, `state: held`, claim `b73c1c98-1b13-43f9-a5db-9b046f71de6f`, owner `Codex-PR-Merge`, phase `deploy`, `"stale": true`. Fence `purpose`: data-preserving STG rollout at package pin `d4af0992db2a50fac8eb94758e1020ad315f8be9` over product pin `62b6fca83b606c4f458f190d20a7e8e02610168c`; `tasks.len=85` (0-indexed 0..84, first entry `OPS-STAGING-INITIAL-RECONCILE-144`).
- Fence `pending` (verbatim head): LAN human root ceremony ATTEMPTED 2026-11-21 and FAILED CLOSED: `controller.Blocked operator_retirement_binding_invalid` inside the media-abort terminal closure proof of rollout `0e2be62a3326`; code chain at tip `fd97cec7`: `installer.py:575/:455` → `operator_retirement.py` `idle_state:1451` → `verify_terminal:263` → `verify_media_abort_terminal:1354` → `media_target_templates:1401` → fail `:41`. Zero cluster mutation observed. Fence `next_action` is human: `sudo python3 /tmp/insp2026.py 2>&1 | tee /root/insp-20261121.log` (inspector sha256 `df845ebf…` as recorded in the fence).
- Pins: package pin `d4af0992db2a50fac8eb94758e1020ad315f8be9`, product pin `62b6fca83b606c4f458f190d20a7e8e02610168c`; STG merge chain PRs #1163–#1171; PR #1167 MERGED (prior sessions).
- Open PRs at census time: `#1191` (UIV3 groupsettings, mergeable UNKNOWN), `#1193`, `#1195`, `#1197`, `#1200` — all map to `in_progress` UIV3 cards; consistent, no finding. By delivery base `#1193/#1195/#1197` MERGED (drift recorded above).
- Merging pool (all 7 carry `prs[]` in `task show` full JSON; `gh pr view` today confirms MERGED; fresh `mergedAt`): `BE175-EXISTING-VOLUME-ASSERTIONS`→#1112 (2026-11-03T02:17:21Z), `CI-BRANCH-PROTECTION-CHECKLIST-ALIGN-1`→#1174 (2026-11-19T18:34:14Z), `CR-20261102-06`→#1101 (2026-11-03T04:30:03Z), `CR-20261102-07`→#1102 (2026-11-02T19:03:22Z) + #1134 (2026-11-12T00:22:07Z), `CR-20261102-10`→#1095 (2026-11-02T19:08:48Z), `CR-20261102-15/16`→#1136 (2026-11-12T00:22:11Z).
- Done-vs-deploy events audit: of 77 `done` cards, 76 have `events[]` empty (zero structured `test`/`accept`/`recheck`/`impact_detected`/`deployment` records); only `GI-T56` has 5 `impact_detected` events. Free-form evidence comments exist on some (e.g. `PL-A1-DELETE`, `H-STAGING-ACCESS`) but no linked records. The records subsystem is exercised in the live inventory only by the OPS-CLICKHOUSE-OOM pair and the two fences.
- CI contract at tip: `branch-protection-checklist.md` (re-derived 2026-11-19 at `bc9e4909…`) claims a 66-job graph; live `ci.yml` at capture tip has exactly 66 job keys; checklist tiers cover all 66; jobs↔tiers name-set diff NET ZERO. `verify-required-jobs.sh` + `gate-baseline.json` (17-row baseline) read for contract support. No ci.yml adds/removals between `bc9e4909` and delivery base (`git diff -- .github` empty 0d43054a..2285c041).
- GHCR gate (fence candidate, raw `staging_fence_raw.json` candidate/observed maps, 23 names each; renderer 24th ref `ghcr.io/poryadok/voiceroot/nats-hub-config-renderer`):

  | image @ `:d4af0992` | candidate digest | observed live digest | changed? |
  |---|---|---|---|
  | admin | `44fa715b09adf98995206bddfa1c41552f037d5abefa8dbb09ebb565aa8e043d` | `a295059a24e9699facbe9c282494006e4cc1638d5bb0ec455f3cef6f4072ab1b` | changed |
  | auth | `5a12dc03230f9915b95450c464b7a396110ba71739a50374829d943887058fe5` | `089d43761aa50beec3db35ea6f6587bf40bd930fc29561cfcecde047f9615036` | changed |
  | developer-portal | `d64e732ec9b092953098c95d099efe8f6e440443f5ba6d78febd0176d1dbac36` | `37d62d3b9be7c69b0f0b7cac7d9e7fc2ac6d049c7cde103d04fbac7fab239659` | changed |
  | notification | `bf40d51e1377b37eadb7de6b80d4ae5e0dae0add466cb30bb126e4aa8bc050c0` | `48b36a7ccf97d8b542f8831000ab431f64ee054560b87782a9d23d1497f0b5b9` | changed |
  | realtime | `f4ea05d42dec151bf179652350d41512fd4a4dd625012e141fb465089785798d` | `e3cc4ae60599ac53412437d433e708b79d22e726f9431fa316ebe1b4606c1d17` | changed |
  | user | `e1e7b9fbdafdd1aedfa9f5cd5b2dff01e37d7fa0f659d82201e68670294d8ca4` | `b0b31eae4595a31adc7ce50dd9ef7dd03896625d2f5792d7f9c7e922b91ff72c` | changed |
  | web | `3256a81713817e6c3b9d3195a49a13d8ee1771f20f28d5e093bc96dd56aa8d24` | `5b89fdb8fc99319433c0e515c8792daba05667eae2baf11bc4211fe44c075e00` | changed |
  | analytics | `06b99ee3d80398c63bccf996e49da24b6f69ae8fb2b40694f9021b7be982bc9b` | equal | unchanged |
  | bot | `0e35c2cc650b362cfb44266aa6b4a757ff441fbda45a3ce6664b8ea48dff9b14` | equal | unchanged |
  | chat | `fa69b966c80eca6c343369458918635d8845c9a5bec773c6d00d66a547fbd39e` | equal | unchanged |
  | file | `423f85ea76a8f09011b08e9f3ffa9bc58f902d435d62fe26d7b39886329385bd` | equal | unchanged |
  | gameintegration | `2e35958e21c9e1a6763d424451ec3ad9d96544f385570bef0187de91f787bfe1` | equal | unchanged |
  | gateway | `3f9b9e74a91c74c713b934d045d8bb7a46f7784f10324aa96f99a703550d7f2a` | equal | unchanged |
  | matchmaking | `ffd23809f17302606f7d4530fb3515fb080bdd8543b1e56a2e6568cba732ed1d` | equal | unchanged |
  | messaging | `54fad661e746423612bfd2027f30f30d9f6c0d865a8e8dec9017bc37410b6ab7` | equal | unchanged |
  | moderation | `0a132705f571aa0eb833ed202d3c34ac991c176da6364151c29139da7823fa83` | equal | unchanged |
  | role | `32d40d8d96c2282e751496e067f3721b54dd6797613560a4e82495a72f8e1e9e` | equal | unchanged |
  | search | `eb491ee4e35d231d357d48dd5d31f42002c101c3664d40ec1d420cc5fe3c91a5` | equal | unchanged |
  | social | `9df4c88fb571f460f09eba38a0b914f341e5bbad0953f69500a52910318db396` | equal | unchanged |
  | space | `f656cb93983912335b8754a65e17e6998f66105434b304e8108321fd0a2540c5` | equal | unchanged |
  | story | `29048fdd5bfc2be52bdae3a7290258ce1ac5afa29e6de6214a22d70f78a264c2` | equal | unchanged |
  | subscription | `90209d61611f9457c1bea8d627f3af56903965849b14b420a14c542738db579a` | equal | unchanged |
  | voice | `be797abed2b740c286d1cfcf53ffce221f08665c169761b2788071c4e53745ce` | equal | unchanged |
  | renderer (nats-hub-config-renderer @ `:d4af0992`) | `5489892bad61614f935ae250dda561ed3501b1ab5c78e8de72499fd5aa3b9e64` | present | gate ref |

  Gate verdict: **PASS 24/24** — 7 changed verified at `:d4af0992` tag-pin (tag digest == candidate digest for all 7); 16 unchanged verified by resolving the content-address reference `name@sha256:<candidate>` on GHCR (present ⇒ the promote-live candidate is exactly the live cluster image, drift-0 by design) AND candidate == observed per the raw fence maps; renderer present at `:d4af0992`; negative sanity probe (deliberately tampered digest ref) did NOT resolve (`sanityNegFalsePositive: false`).

## Findings

1. **Actionable — done↔records consistency gap: 76/77 `done` cards are eventless.** No structured test/accept/recheck/impact/deployment records back the `done` state for 76 of 77 done cards (evidence above). Done-vs-deploy consistency for legacy done cards is unauditable on the records axis. No duplicate finding existed (`task search` by code prefixes found nothing). → new `available` card `OPS-DONE-CARDS-EVENTLESS-RECEIPTS-20261123` (epoch A1, P2, EVIDENCE+DECISION in description; decision keeps 76/77 `done` and forbids fabricating receipts; reopen only via `task invalidate` with evidence).
2. **Stale `merging` pool (7/7 PRs MERGED, cards still `merging`)** — re-verified today with fresh `gh pr view` mergedAt values above. This finding is ALREADY registered by card `OPS-MERGING-CARDS-STAGE-RECONCILE-20261120` (still `available`) → action = receipt comment on that card with today's re-validation, NOT a duplicate card. Note (report-only): fresh gh `mergedAt` values (#1095/#1102 merged 2026-11-02, #1101/#1112 2026-11-03, #1134/#1136 2026-11-12, #1174 2026-11-19) do not all match the dates quoted in the 2026-11-20 card description; today's receipt cites fresh gh JSON without reconciling the older claims.
3. **wfh trio honest** — all three owners recorded (GI-T13 human OAuth-client registration; OPS-STAGING-INITIAL-RECONCILE-144 closure subsumed by STG ceremony; OPS-STG-LAN-INSTALLED-COHORT-ROOT-V1 owner `Codex-PR-Merge` with euid-0 node gate and BANNED frozen rerun). No transitions owed by audit.
4. **Fences**: DEV free, op 52 PASS delivered (`DEV-CHFIX-62B6FCA-20261121-V14`), `"stale": false`. STAGING HELD at rev 57/gen 14 with claim `b73c1c98…` and `"stale": true` — staleness evidence: recorded SHA (package pin `d4af0992…`) vs live master ancestry; the LAN capture precondition (all-green full-graph master push) still does not exist; fence `next_action` is human node-side (sudo inspector run above).
5. **CI contract drift: NET ZERO** (66/66 checklist↔live ci.yml at both anchors; verifier 34 names ⊆ live; 0 adds/removals since `bc9e4909`). Tip push at capture anchor (`ci-gate` + siblings) red family already registered in `docs/todo/ci.md` (lines 22–25 region) by CI-Debt-Family-Root-Causes-20261122 — no new red classes, no new `docs/todo/ci.md` rows.
6. **Report-only CLI-surface drifts**: (a) `tt journal` does not exist in the current CLI — prior passes used `resource list/show` waits+events as the substitute; (b) legacy status names return total 0 (workflow-2 server profile label drift). Neither is data loss; no card owed (TT CLI is not a repo product surface).
7. **Report-only census/fence membership note**: STAGING fence `tasks.len=85` (raw bytes) vs status census `stg_ready_for_deploy` 86 — the fence task list includes `OPS-STAGING-INITIAL-RECONCILE-144` (`waiting_for_human`), so the two sets are different axes; not treated as an inconsistency and no action owed.
8. **Report-only tip drift during the pass**: capture anchor tip `0d43054a` → delivery base `2285c041` (PRs #1193/#1195/#1197 merged during the pass; `git diff -- .github` between anchors EMPTY, CI verdicts unaffected).

## Actions taken

- TT (all receipts cite this audit path; nick `OpenChamber-Fleet-Inventory-20261123`):
  1. receipt comment on `OPS-MERGING-CARDS-STAGE-RECONCILE-20261120` — re-validation of all 7 merging cards with today's fresh `gh pr view` mergedAt values (finding 2, no duplicate card).
  2. new `available` card `OPS-DONE-CARDS-EVENTLESS-RECEIPTS-20261123` (finding 1; epoch A1, P2, EVIDENCE + DECISION + audit anchor in description).
- Repo: `docs/audits/2026-11-23-openchamber-fleet-inventory-audit.md` (this file), one index row in `docs/audits/README.md`, one dated WIP line in `docs/PLAN.md` audit-log band. Docs-only PR, merged with merge commit.
- No CI job classes added to `docs/todo/ci.md` (finding 5: already registered family).

## Verdict

After the pass: fleet inventory is consistent on the census/PR/fence/epoch axes at the recorded anchors; the STG rollout gate is honestly READY at the package pin (`:d4af0992` GHCR gate PASS 24/24, digests byte-stable against the HELD fence candidate map), and the STG rollout itself remains an honest HOLD because its LAN root precondition (all-green full-graph master push after full-validation debt flips) still does not exist — that HOLD is not a gate failure. CI contract is drift-clean (NET ZERO). One new actionable finding registered as one new card; one previously registered finding re-validated by comment. wfh trio and fences are faithfully recorded; no fabricated deploy/CI/acceptance evidence anywhere.

## Deliberately not done

- STG rollout / CI push / promote: not fired — rollout gate READY but root precondition absent (all-green full-graph master push does not exist; tip push reds are registered debt family in `docs/todo/ci.md`); firing CI would not fix the leaves and is out of audit duty.
- LAN node ceremony: owner-gated euid-0 step (sudo inspector run in fence `next_action`); frozen rerun BANNED.
- No status transitions on foreign cards (wfh trio, `GI-T13`, the 7 `merging` cards, 76/77 `done` pool): stage reconciliation stays with `OPS-MERGING-CARDS-STAGE-RECONCILE-20261120`; done-pool reopen only via `task invalidate` with evidence.
- `owned-enrollment` infra red and open PRs untouched; delivery PR is docs-only.
- Main checkout untouched (dirty state of other sessions, HEAD at its own commit); this audit was authored and merged exclusively in its own worktree.

## Self-corrections

- `git worktree create` is not a valid subcommand; the first worktree attempt created nothing and file writes to relative paths silently went nowhere. Corrected to `git worktree add … -b codex/<label> origin/master` and absolute-path file operations; all three repo edits were re-applied and verified against disk in the new worktree.
- PowerShell `>` redirect wrote UTF-16 and broke captured TT JSON earlier; re-captured the STAGING fence raw bytes via node spawn (`staging_fence_raw.json`) — the digest tables above come from those raw bytes and supersede the earlier ad-hoc `ghcr_gate2.ps1` digest table, whose 64-hex tails were wrong.
- Fence `tasks.len=85` per raw bytes; the census axis (`stg_ready_for_deploy` 86) is a different set — both recorded as separate axes, not reconciled into one number.
- Earlier `waiting_for_human`/`merging` date claims on the 2026-11-20 card do not all match today's fresh `gh pr view` mergedAt JSON; this pass cites only fresh gh values and does not silently "fix" the older card claim (report-only).
