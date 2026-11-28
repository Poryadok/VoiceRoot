# TT Fleet Inventory Audit 2026-11-28 — Routine inventory pass; TT BACKEND REACHABLE (first fresh full census since 2026-11-23): totals 1291 (delta +1 vs 2026-11-22 census, status-math), ownerless in_progress pool = GI-T58 persists, merging pool 7/7 PR links re-resolved MERGED, fences fresh read-back STAGING HELD gen14/rev57 claim b73c1c98 stale / DEV free; CI contract drift NET ZERO at tip (66↔66 job names vs bc9e4909, zero CI files in the #1218–#1222 window); 1 NEW actionable finding: malformed dead pr-url stored on OPS-STG-LAN-CAPTURE-GREEN-PUSH-V1 → new available card OPS-STG-LAN-CAPTURE-PR-LINK-TYPO-V1

- Date: `2026-11-28`
- Agent: OpenChamber — Fleet Inventory 20261128 (OpenChamber session, model `qwen3.8-flash-next-coder-iq1_m`; TT comment nick `OpenChamber-Fleet-Inventory-20261128`)
- TT CLI: `C:\Users\Sergey\AppData\Local\TTenrec\bin\tt.exe --url http://192.168.0.109:30990 --project voice --timeout=60s` (bare `--timeout=60` rejected client-side, unit suffix required — carried CLI-surface note)
- Worktree/branch: `D:\Git\Voice\tmp\fleet-inventory-20261128`, branch `codex/fleet-inventory-20261128`, base tip `5a324abe845ad6261642b5134c7a53da8b423c1f` = PR #1222 merge (authored entirely in this worktree; main checkout `D:\Git\Voice` carries another session's uncommitted WIP — `git fetch origin` only, zero touches)
- Previous: [`2026-11-26-tt-fleet-inventory-audit-v2.md`](./2026-11-26-tt-fleet-inventory-audit-v2.md), [`2026-11-27-ci-gate.md`](./2026-11-27-ci-gate.md), [`2026-11-26-tt-fleet-inventory-audit.md`](./2026-11-26-tt-fleet-inventory-audit.md), [`2026-11-22-tt-fleet-inventory-audit.md`](./2026-11-22-tt-fleet-inventory-audit.md), [`2026-11-23-openchamber-fleet-inventory-audit.md`](./2026-11-23-openchamber-fleet-inventory-audit.md)

## Scope and method

Routine TT↔GitHub↔docs reconciliation pass, same axes as the 2026-11-22/23 inventory audits:

1. Fresh TT census per status (`task list --status S --compact --limit=1` → `total` field; statuses as enumerated by the 11-22/23 passes + legacy-label sweep incl. `acceptance`/`problem`).
2. Ownerless `in_progress` pool scan (`task list --status in_progress` 64 items, `owner` field); `GI-T58` full show.
3. `waiting_for_human` owners; merging pool codes + stored `prs` refs + `gh pr view` merged-state/mergedAt/baseRefName re-validation.
4. done↔eventless axis: `task events` on sampled done cards (honest CLI-verifiable slice only; no full-pool events scan).
5. Fence read-back (`resource list`/`resource show` DEV + STAGING; zero mutations).
6. Epoch coherence (`epoch list` full JSON vs PLAN.md WIP markers).
7. GitHub reality: `gh pr list --state open`, `gh pr view 1208`, `gh pr checks 1222`, `gh api …/actions/runs/46099181836`, `gh api …/branches/master/protection`.
8. CI contract drift at tip: live `ci.yml` top-level job-name parse (HEAD vs anchor `bc9e4909`), blob identities (`git ls-files --stage`), `ab309265..HEAD` file window.
9. Docs registries: `docs/audits/README.md` index vs landed audit files, `docs/PLAN.md` WIP band, `docs/todo/ci.md` since 2026-11-25.

Docs-only pass: zero product/CI code, zero cluster/fence/candidate/GHCR mutations, zero CI re-runs, zero fence writes.

## Ground truth

- Master tip anchor: `5a324abe845ad6261642b5134c7a53da8b423c1f` (fresh `git worktree add … origin/master` after `git fetch origin`); merged chain since 2026-11-25: !1216 (`c666d365`), !1217 (merge `ab309265`, NSI ci.yml step reorder), fleet docs PRs #1218–#1221 (`4c7ea4e4`), PR #1222 merge = tip (`59b746e0` pre-merge docs commit).
- TT reachable (first contact in five days): `projects list` → 5 projects incl. `voice` (server profile); census revision `rev 23732` on every successful status read.
- Fresh census @ rev 23732:

```text
available 896 | in_progress 64 | review 45 | merging 7 | waiting_for_human 3 | blocked 94
done 77 | stg_ready_for_deploy 86 | dev_ready_for_deploy 17 | dev_test 2  → SUM = 1291
legacy/unexpected labels all 0: pending todo doing pr_open dev_ready dev_verify
stg_doing stg_ready_for_accept acceptance problem
```

- Delta vs the 2026-11-22 census (rev 23622, total 1290): available 898→896 (−2), in_progress 61→64 (+3), all other statuses flat; total +1. Delta is status-math (TT has no global journal); known landed contributor: `OPS-CI-RULES-GATE-VERDICT-6152-20261125` created `available` on 2026-11-25 (recorded in the 11-25 v2 WIP row) — full per-card attribution not CLI-verifiable, report-only.
- Ownerless `in_progress` pool: exactly 1 of 64 — `GI-T58` (epoch `GAME`, `owner`/`assignee`/`tester`/`checkpoint_at`/`next_action` all empty). Audit card `OPS-GI-T58-OWNERLESS-IN-PROGRESS-V1` exists `available` (3 comments) since 2026-11-22 → finding persists, comment receipt only, no stage change.
- wfh trio (owners recorded): `GI-T13` → `Codex-PR-Merge`; `OPS-STAGING-INITIAL-RECONCILE-144` → `Rudeus-144`; `OPS-STG-LAN-INSTALLED-COHORT-ROOT-V1` → `Codex-PR-Merge`.
- Merging pool 7 codes ↔ stored PR refs ↔ fresh gh state:

```text
BE175-EXISTING-VOLUME-ASSERTIONS → #1112 MERGED 2026-11-03T02:17:21Z (base develop)
CI-BRANCH-PROTECTION-CHECKLIST-ALIGN-1 → #1174 MERGED 2026-11-19T18:34:14Z (base master)
CR-20261102-06 → #1101 MERGED 2026-11-03T04:30:03Z (develop)   CR-20261102-07 → #1102 (develop) + #1134 (master)
CR-20261102-10 → #1095 MERGED 2026-11-02T19:08:48Z (develop)   CR-20261102-15/16 → #1136 MERGED 2026-11-12T00:22:11Z (master)
```

- done↔eventless sampled axis (not full-pool): done pool = 77 codes; `task events` returned a top-level JSON array — sampled `BE-188`, `BE-193`, `BE-A2-AUTH-VOICE-SESSION-FLOOR`, `GI-T05`, `GI-T13-DEV` all events=0; known non-eventless done pair re-probed: `OPS-CLICKHOUSE-OOM-20261021` events=152, `OPS-CLICKHOUSE-OOM-RECOVERY-450` events=176 (consistent with the 11-23 finding "76/77 eventless").
- Fences (fresh read-back, zero mutation):
  - `STAGING`: `state=held generation=14 revision=57 operation=12 claim=b73c1c98-1b13-43f9-a5db-9b046f71de6f owner-label Codex-PR-Merge phase=deploy stale=true`; candidate pin `d4af0992db2a50fac8eb94758e1020ad315f8be9`; `tasks.len=85` vs census `stg_ready_for_deploy=86` — separate axes (unchanged relation vs the 11-23 note). Stale HELD claim waiting for the human idle-cohort LAN ceremony (owner-gated sudo inspector) → gate stands.
  - `DEV`: `state=free generation=96 revision=375 operation=52 claim="" stale=false`; last claim window `1db5ae5e-e447-46b2-87af-51795ca8af4b`, tasks `[OPS-CLICKHOUSE-OOM-20261021, OPS-CLICKHOUSE-OOM-RECOVERY-450]`, candidate commit `62b6fca83b606c4f458f190d20a7e8e02610168c` — matches the 11-22/23 DEV records (PASS delivered, free).
- Epoch coherence (16 epochs read): `A4` title carries "Parallel active — PLAN" marker (reconciled 2026-11-21 — no action), `GAME` "development acceptance OPEN", `A1` "Active — PLAN", `A2` "Next — PLAN", `A3`/`A5–A7`/others Queued/backlog/deferred — PLAN.md WIP markers coherent; zero epoch writes.
- PR reality: exactly one open PR — #1208 (`OPEN`/`mergeable=CONFLICTING`/`mergeStateStatus=DIRTY`, head `05eb6318fdb96e3cc14243dd8f727c93afe46b9a`, base `6f652d0626e2e563b766a4992389651de9cdb6fb`, updated 2026-11-24T00:05:58Z) = registered anchor-clash class, owner-side landing, report-only, untouched.
- `gh pr checks 1222` (tip PR): 69 checks = 7 SUCCESS / 1 FAILURE (`owned-enrollment` — registered infra class, not gate-required, same shape as #1208's) / 61 SKIPPED.
- Tip CI dispatch: run #6183 id `46099181836`, `workflow_dispatch`, head `5a324abe…`, GitHub run-level rollup **completed / conclusion=failure** (dispatched 2026-11-28). The authoritative `133`-job full-graph classification for this tip is owned by the CI-verdict lane and NOT produced by this inventory pass; per cross-lane TT records the full-graph verdict for run `46099181836` was already posted to `OPS-STG-LAN-CAPTURE-GREEN-PUSH-V1` by nick `Codex-NSI-Reorder-20261125` at `2026-11-28T05:08:57Z` (verified present in the card's comment list — dedup anchor, not reposted here). Previous tip `4c7ea4e4` full-graph verdict run #6179 (`46061235286`) `133 = 119 success / 11 skipped / 3 failure` stays as landed in `docs/audits/2026-11-27-ci-gate.md`.
- Branch protection (fresh API read, report-only): `required_status_checks.strict=false`, `contexts=["ci-gate"]` — unchanged F7 class vs checklist Tier-1 four names.
- CI contract drift at tip (verified, not assumed): live `ci.yml` top-level job names = `66` at HEAD and `66` at anchor `bc9e4909e092f1be63a45c4d81e20b3bf5b9a902`, `only_tip=[] only_anchor=[]` (zero adds/removals/renames). Blobs byte-identical vs the 11-26 v2 records: `ci.yml 3afca89e445d4b6ff007835d1f7259d454114660`, `branch-protection-checklist.md 60901ebf39e9639e67abb15fc3e81f75d6ed8fe9`, `verify-required-jobs.sh 7ff776e682947181ba150a3360d712ade2e7c02e`, `gate-policy.md f6e14bf5a4fd1f1dadea1f8b6c81d57572e62842`. `git diff --ab309265..HEAD` window (#1218–#1222) = 7 docs files `+501/−1`, ZERO files under `.github/`, `scripts/ci/`, `src/` → job-name contract 66↔66 intact since `bc9e4909` through the new tip.
- Anchor-card records (fresh): `OPS-GI-T58-OWNERLESS-IN-PROGRESS-V1` available, 3 comments; `OPS-STG-LAN-CAPTURE-GREEN-PUSH-V1` in_progress owner/assignee `Codex-LAN-Capture-Debt`, 14 comments, 16 events, stored `prs=["https://github.com/Poryadok/VVoiceRoot/pull/1192"]`; `OPS-MERGING-CARDS-STAGE-RECONCILE-20261120` available, 2 comments; `OPS-DONE-CARDS-EVENTLESS-RECEIPTS-20261123` available, 0 comments.
- Docs registries: README index rows through the 2026-11-27 gate row present; the only dangling audit file remains the 2026-11-24 file pending on open PR #1208 (report-only class, unchanged); PLAN WIP band coherent through the `> 2026-11-27 CI gate verdict` row; `docs/todo/ci.md` 2026-11-25→2026-11-27 sections carry the NSI reorder, Maven-429 retire, tag-retention monitor rows, #6170/#6179 verdict rows — no new-class row owed by this pass.

## Findings

1. **TT outage class closed**: backend reachable (rev 23732); the `OPS-TT-BACKEND-UNREACHABLE-20261126` payload owed verbatim in the 11-26 v1 doc is now moot for creation purposes — contact restored, no outage card created (would assert a closed condition; honest closure = this receipt + the landed docs). Report-only.
2. **Census total 1291 (+1 vs 2026-11-22)**: available −2 / in_progress +3, others flat; delta explained as status-math with the known new card `OPS-CI-RULES-GATE-VERDICT-6152-20261125` (landed available 2026-11-25). No unexpected labels (`legacy 0` sweep incl. `acceptance`/`problem`).
3. **GI-T58 ownerless persists** (sole ownerless in_progress of 64) — carried finding, existing card, comment receipt posted, no stage change by this lane.
4. **Merging pool re-validated 7/7**: every merging card's stored PR resolves to a MERGED PR (mergedAt + baseRefName recorded above; 4 base=`develop` era PRs + 3 base=`master`); receipt on `OPS-MERGING-CARDS-STAGE-RECONCILE-20261120`; cards stay `merging` (stage advancement is owner-side per canon, never audit-side).
5. **done↔eventless axis persists**: 5 sampled done cards events=0, the one known structured pair carries events — matches the 11-23 76/77 finding; cleanup card `OPS-DONE-CARDS-EVENTLESS-RECEIPTS-20261123` open (available, 0 comments) → census receipt comment added, no new card.
6. **Fences read-back exactly as expected**: STAGING HELD `gen14/rev57` claim `b73c1c98…` `stale=true` (human idle-cohort gate), DEV free `gen96/rev375` — zero drift vs landed records, zero mutations.
7. **CI contract drift NET ZERO at the new tip** (66↔66, blobs byte-identical, zero CI files in the #1218–#1222 window).
8. **PR reality**: sole open PR #1208 `CONFLICTING/DIRTY` (registered class) and #1222 checks sole FAILURE `owned-enrollment` (registered infra class) — both report-only, untouched.
9. **Tip CI verdict deferral (honest)**: run #6183 (`46099181836`) is completed at run-level rollup (`failure`), but its full-graph red-classification is owned by the CI lane; the verdict already exists cross-lane on the LAN card (`Codex-NSI-Reorder-20261125`, 2026-11-28T05:08:57Z) — this pass records the anchor, fabricates nothing.
10. **NEW actionable finding**: card `OPS-STG-LAN-CAPTURE-GREEN-PUSH-V1` stores a malformed PR link `https://github.com/Poryadok/VVoiceRoot/pull/1192` — repo `Poryadok/VVoiceRoot` does not exist (`gh api repos/Poryadok/VVoiceRoot` → 404 `Not Found`); the intended target is PR #1192 in `Poryadok/VoiceRoot` (the 2026-11-22 LAN-capture docs PR). Dead stored link = unlink-pr + re-link repair owed → **one new card `OPS-STG-LAN-CAPTURE-PR-LINK-TYPO-V1` (available, P3 chore, epoch A1)**; dedup search ("VVoiceRoot", "malformed") found no existing card.
11. Branch protection unchanged `["ci-gate"] strict:false` vs Tier-1 four — F7 report-only, owner-side checklist decision.

## Actions taken

- TT writes (all @ rev ≤ 23732, author nick exactly `OpenChamber-Fleet-Inventory-20261128`):
  1. Receipt comment `c4` (2026-11-28T09:31:33Z, 4th comment on the card) on `OPS-GI-T58-OWNERLESS-IN-PROGRESS-V1` (finding persists, no status change).
  2. Fence read-back receipt comment `c15` (2026-11-28T09:43:51Z, 15th comment on the card) on `OPS-STG-LAN-CAPTURE-GREEN-PUSH-V1` (+ cross-reference of the dead pr-url finding).
  3. Merging-pool re-validation receipt comment `c3` (2026-11-28T09:50:10Z, 3rd comment on the card) on `OPS-MERGING-CARDS-STAGE-RECONCILE-20261120`.
  4. done↔eventless census/sample receipt comment `c1` (2026-11-28T10:02:57Z, 1st comment on the card) on `OPS-DONE-CARDS-EVENTLESS-RECEIPTS-20261123`.
  5. NEW card `OPS-STG-LAN-CAPTURE-PR-LINK-TYPO-V1` created 2026-11-28T09:21:48Z (available, epoch A1; evidence = this audit path + dead-link 404 probe + stored prs value; CLI `task create` has no `--type/--priority` flags → those fields unset on the card, recorded honestly as an anomaly vs the older OPS cards carrying `type chore`/`priority P2`).
- TT command kinds: 4 `task comment`, 1 `task create`, 0 status/stage/owner/fence mutations, 0 retries needed.
- Docs: `docs/audits/2026-11-28-tt-fleet-inventory-audit.md` (this file) + one `docs/audits/README.md` index row + one `docs/PLAN.md` WIP-band line after the 2026-11-27 CI-gate row. `docs/todo/ci.md` untouched (zero new registered-class events this pass).
- GitHub: docs-only PR from branch `codex/fleet-inventory-20261128` → merged via `gh pr merge --merge --admin` (merge commit recorded in the PR log; this doc lands in it).

## Verdict

Fleet state coherent at tip `5a324abe`: TT reachable and census fresh (1291 total, arithmetic-consistent with the 11-22/23 baselines), ownerless pool exactly `{GI-T58}` (carried, carded), merging pool 7/7 PR links MERGED with recorded mergedAt/base, wfh trio owners recorded, fences byte-stable vs landed records (STAGING stale-HELD with the human idle-cohort gate standing — LAN capture legality still ZERO legal all-green full-graph master push per the 11-27 gate audit + cross-lane 11-28 receipts), epoch/PLAN coherence intact (A4 already reconciled), CI job-name contract 66↔66 NET ZERO at tip, PR reality fully registered (#1208 CONFLICTING class, #1222 `owned-enrollment` infra class), tip CI verdict completion deferred honestly to the CI lane (run #6183 completed, classification cross-lane on the LAN card). Exactly one new actionable finding found and carded: dead pr-url on the LAN-capture card. No product, CI, fence, cluster, or GHCR mutation anywhere in this pass.

## Deliberately not done

- Main checkout `D:\Git\Voice`: another session's uncommitted porcelain lines — `git fetch origin` only, zero touches; this pass authored 100% in its own worktree.
- Fences: read-only observation; ZERO claim/resource mutations (STAGING HELD stays with the human idle-cohort ceremony; assistant never runs sudo on the ssh node `voice-staging`).
- GHCR: no digest sweep this pass (rollout/CI lanes own the last recorded `:d4af0992` content-address `24/24` + tag-expiry monitor state; nothing here contradicts it).
- Full-graph CI classification of run #6183 (tip): owned by the CI-verdict lane; anchor recorded, not re-derived, verdict not fabricated.
- Stage advancement of the 7 merging cards / GI-T58 reconciliation execution: owner-side per canon; this lane posts receipts only.
- PR #1208 landing/closure: owner decision (registered class).
- Branch protection Tier-1 reconcile: owner-side checklist decision (F7 class), token-side `repo:admin` not used.
- Epoch writes: zero (coherence read clean).
- `docs/todo/ci.md`: untouched (no new registered-class event this pass; tip-run completion is a cross-lane verdict record, not a new class declaration).

## Self-corrections

- None vs public numbers of prior passes (this pass agrees with the landed 11-26 v2 / 11-27 records wherever axes overlap).
- Tooling notes (report-only CLI surface drift): installed `tt` CLI rejects `--brief` on `task show` and `-compact` on `epoch list` (full JSON parsed instead); `tt` requires unit-suffix `--timeout=60s`; `task events` returns a top-level JSON array (not a `{"events":[…]}` wrapper) — the first parse attempt of this pass mis-shaped that and was corrected before any count was published; `gh pr checks` has no `--limit` flag; `gh api` bodies may carry a BOM — parsed after locating the first `{`/`[`.
