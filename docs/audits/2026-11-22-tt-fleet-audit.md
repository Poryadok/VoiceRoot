# TT-fleet audit 2026-11-22 — Inventory audit at TT rev 23588, master tip `eb341dd8`

> **Consumers**: Fleet dispatcher — fresh inventory baseline (per-status TT census, PR-lifecycle cross-check, owner-gated pools) and actionable-gap enumeration; owners — fence/epoch verdicts for dispatch decisions; CI controllers — contract-drift verdict at tip; STG rollout owners — GHCR digest gate at the newest product pin. Evidence-only audit; no status transitions on foreign cards, no fence mutation, no CI runs triggered.

Label: `TT-Fleet-Audit-20261122`. Tracker: `tt --url http://192.168.0.109:30990 --project voice`, `--timeout=30s`.
Model: qwen3.8-flash-next-coder-iq1_m.
Worktree: fresh OpenChamber worktree `codex/tt-fleet-audit-20261122` at `D:\Git\Voice\tmp\tt-fleet-audit-20261122`, branch from `origin/master` `eb341dd80325cfb640bfb9b519fcb0926dbff06c` (clean at audit; the main `D:\Git\Voice` checkout stayed on stale, heavily-dirty `1da2a4dc` — not merged, not forced, left untouched per fleet rule).

## Scope

- Routine TT-fleet inventory audit: per-status TT census, owner fields across `in_progress`/`review`/`merging`, `waiting_for_human` owner decisions, epoch/PLAN WIP alignment, resource fences (DEV/STAGING hold purposes vs wait records).
- GitHub PR ↔ TT consistency at the fresh tip: open PRs vs TT pools, merging-card PR references vs PR reality.
- CI contract drift: `.github/ci/branch-protection-checklist.md` tiers and `.github/ci/verify-required-jobs.sh` (ci-gate) vs live `.github/workflows/ci.yml` job graph at tip; debt/infra classes registered in `docs/todo/ci.md`.
- STG rollout GHCR digest gate: `scripts/staging/verify-staging-images.sh` presence rule (`docker manifest inspect --verbose` at 40-hex STACK_LOCK tags) cross-checked against the STAGING fence candidate map — evaluated at both the fence pin `:d4af0992` and the newest product pin `:eb341dd8`.

## Ground truth

- Tracker revision: 23588 (identical across all fresh reads this audit; 23536 at the 2026-11-21 audit).
- GitHub: master tip = PR #1190 merge `eb341dd8…` (UIV3 stories code merge). Merged since 2026-11-21 audit: `#1187` (TT-fleet audit docs), `#1188` (audit Actions fix), `#1189` (LAN reviewed-baseline successor docs), `#1190`. Open PRs = 2: `#1191` (head `codex/uiv3-panel-chat-groupsettings-20261121`, worker card `UIV3-PANEL-CHAT-GROUPSETTINGS` `in_progress` — MERGEABLE but mergeStateStatus `BLOCKED`, head run 44938332145 reds = documented debt family incl `ci-gate`) and `#1192` (head `docs/lan-capture-satisfiability-20261121`, another session's docs PR + its audit `docs/audits/2026-11-21-lan-capture-source-green-push-satisfiability.md` + card `OPS-STG-LAN-CAPTURE-GREEN-PUSH-V1`; its PR-head runs 44956661185/44956661317 have exactly one red: `owned-enrollment` infra class).
- Status totals (fresh per-status queries at rev 23588): available 900, in_progress 56, review 45, merging 7, dev_ready_for_deploy 17, dev_deploying 0, dev_ready_for_test 0, dev_test 2, stg_ready_for_deploy 86, stg_deploying 0, stg_ready_for_test 0, stg_test 0, acceptance 0, waiting_for_human 3, blocked 94, done 77, archived 0. Legacy statuses (`todo`/`doing`/`pr_open`/`stg_doing`/`stg_ready_for_accept`/`testing`/`deploy`) = 0 — vocabulary cleanup still holds.
- Pool owner check (58 cards across `in_progress` 56 + `review` 45 + `merging` 7): exactly **1 ownerless** — `GI-T58` (GAME epoch, T-series), all others carry owner/assignee. PR-reference scan across pool card fields: 13 unique PR refs → 11 MERGED, 1 OPEN (`!1191` ↔ `UIV3-PANEL-CHAT-GROUPSETTINGS`, consistent), 1 CLOSED-unmerged (`!915` ↔ `DELIVER-BE085-PR915`, carried precedent since 2026-11-19).
- `waiting_for_human` = 3, all with recorded owner decisions: `GI-T13` (GAME, owner `Codex-PR-Merge`, real-provider admission options recorded 2026-11-20); `OPS-STAGING-INITIAL-RECONCILE-144` (A1, owner `Рудеус-144`, reconcile closure gated by STG rollout ceremony); `OPS-STG-LAN-INSTALLED-COHORT-ROOT-V1` (A1, owner `Codex-PR-Merge`, reviewed-baseline capture decision owed).
- Fences (fresh `resource list`): DEV state `free`, generation 96, revision 375, purpose "Independent DEV verification of CH OOM cards 2026-11-21…", claim `6b500f31-0137-492a-93a3-7c6e6ee80aef` released outcome `PASS`, fence record carries deployment record `DEV-CHFIX-62B6FCA-20261121-V14`. STAGING HELD by `Codex-PR-Merge`, claim `b73c1c98-1b13-43f9-a5db-9b046f71de6f`, generation 14, revision 57, `stale:true`, phase `deploy` — **unchanged since the 2026-11-21 audit (no new fence record this window)**; purpose verbatim: data-preserving STG rollout at package pin `d4af0992…` (product pin `62b6fca8…`) with candidate map = 16 unchanged live digests + 7 services (`admin,auth,developer-portal,notification,realtime,user,web`) to exact `:d4af0992` GHCR digests; STAGING reconcile wait `ready:false`; DEV wait cancelled.
- Epochs (fresh `epoch list` at rev 23588): `A1` pos 1 "…Active — PLAN", `GAME` pos 1 "…development acceptance OPEN", `A2` pos 2 "Next — PLAN", `A4` pos 4 "Parallel active — PLAN" (title already reconciled by the 2026-11-21 audit via `epoch update A4` — no action this audit), `A3/A5/A6/A7` Queued. Epoch totals (all statuses): A1 217, GAME 81, A4 163.
- GHCR gate (fresh `docker manifest inspect --verbose` on `ghcr.io/poryadok/voiceroot/*`): **24/24 present at `:d4af0992db2a50fac8eb94758e1020ad315f8be9`** (23 catalog images from `scripts/ci/staging-image-catalog.json` + `nats-hub-config-renderer`) and **24/24 present at the newest product pin `:eb341dd80325cfb640bfb9b519fcb0926dbff06c`**.
- CI job graph (fresh at tip): `.github/workflows/ci.yml` = 66 top-level jobs, byte-identical job-key set vs checklist rebuild `bc9e4909` (zero adds/removals since the checklist rebuild, PR #1174).
- Tip master-push CI run 44934578838 (event `push`, head `eb341dd8`): 132 jobs = 103 success / 19 skipped / 10 failure; `staging-stack-lock` SUCCESS (artifact 14720981236), `staging-images-promote` + 22× `staging-images-dispatch-publish` published all 24 images at `:eb341dd8` data-preserving.

## Findings

### CI contract drift at tip — CLEAN, no drift

- Live graph 66 jobs == 66 at `bc9e4909` (fresh key-set diff: zero adds/removals since the checklist rebuild).
- `.github/ci/branch-protection-checklist.md` (rebuilt 2026-11-19 at `bc9e4909`, PR #1174, merge `e5f0dba7…`) covers all 66 live jobs (`live-not-in-checklist = []`); the only checklist name tokens not in the ci.yml job set are `markdown-link-check` (defined in the separate workflow `.github/workflows/docs-link-check.yml`, not ci.yml drift) and `tier3-only` (profile label).
- `.github/ci/verify-required-jobs.sh` (the `ci-gate` verifier) enforces 34 distinct conditional job names — all present in the live 66-job graph.
- Tip full master-push run 44934578838: all 10 reds are already-registered debt families (`ci-gate`, `full-validation-aggregation`, `local-ci-parity`, `compose-e2e` native50 chain, `space-native-installer-integration`, `role-space-receipt-integration`, `backend-go (role)`, `backend-go-integration-pr (space)`, `backend-go-integration (space)` Space/Role ban-catalog shard, `a1-attachment-restart-proof` a1-* flake family) — no new class. `owned-enrollment` infra class (missing CHFIX target8 refs) re-observed FAIL at PR-head runs 44956661185/44956661317 — infra class already registered in `docs/todo/ci.md`, docs-only merge precedent applies.
- `docs/CI.md` and `.github/BRANCH_PROTECTION.md` named by the consumer contract still do not exist at tip — real registries are `.github/ci/*` (precedent F9, report-only, no doc action).

### STG rollout GHCR digest gate — PASS on presence at both pins, semantics recorded

- Gate rule = presence at the pinned 40-hex STACK_LOCK tag. **Fence pin `:d4af0992`: 24/24 PRESENT**; the 7 fence-candidate "changed" services match their pinned digests exactly (7/7): `admin 44fa715b…`, `auth 5a12dc03…`, `developer-portal d64e732e…`, `notification bf40d51e…`, `realtime f4ea05d4…`, `user e1e7b9fb…`, `web 3256a817…`; the other 16 candidate entries carry live-cluster digests by design (data-preserving semantics in the fence purpose verbatim).
- **Newest product pin `:eb341dd8` (tip merge #1190): 24/24 PRESENT** — the code-touching master merge fired the gated `staging-stack-lock` (artifact 14720981236) and promote published a full data-preserving 24-image set at the new tag: the 7 changed services got fresh tip digests (`admin 25e8bb69…`, `auth 885d1ce2…`, `developer-portal df35b662…`, `notification b0a56524…`, `realtime 33cff769…`, `user 74484f00…`, `web 5b563564…`), the 16 unchanged keep live-cluster digests. Tag-name ≠ live digest for unchanged images is by design, not drift.
- Note for digests queries: `admin@sha256:<live>` digest-refs can silently resolve to unrelated newer manifests — the sweep used pinned-tag descriptors (`@:40-hex` + `--verbose`), the gate-contract form.
- Absence of other historical tags was not re-probed (pinned-tag rule is the gate contract).

### Inventory drift and actionable findings

1. **NEW actionable: `GI-T58` ownerless `in_progress`** (GAME epoch, "T58 — Закрыть app-conversation consent — notification/event matrix"): `owner`/`assignee`/`tester`/`checkpoint_at`/`next_action`/`link` all empty at rev 23588; its event history only shows 2026-11-01 `impact_detected` refutations by `Codex-Status-Audit-20261101` (erasure-review of the GAME T-series historical `done`, evidence `erasure-review/gi2-done-review.json`), last comment 2026-11-01T16:46Z. The 2026-11-21 audit claimed "0 ownerless in all three pools" — that check missed this card (present since 2026-11-01). New `available` card `OPS-GI-T58-OWNERLESS-IN-PROGRESS-V1` created (decision: dispatcher/owner reconciles claim state — claim by GAME sprint actor or move to `available`; audit made no transition on the foreign card).
2. **PR ↔ pools consistent**: the only 2 open PRs are accounted — `#1191` ↔ in_progress worker card `UIV3-PANEL-CHAT-GROUPSETTINGS` (BLOCKED mergeable under the debt family incl `ci-gate`; report-only, debt classes registered); `#1192` is another session's docs PR whose card `OPS-STG-LAN-CAPTURE-GREEN-PUSH-V1` is `in_progress` (created 2026-11-21T20:15Z by `Codex-LAN-Capture-Debt`); not mine to merge or duplicate.
3. **Merging pool (7) honest**: `BE175-EXISTING-VOLUME-ASSERTIONS` (awaiting master promotion of PR #1112 source `4dce5e0…`), `CI-BRANCH-PROTECTION-CHECKLIST-ALIGN-1` (PR #1174 MERGED `e5f0dba7…`, awaiting acceptance record), `CR-20261102-06/07/10/15/16` (develop-era PORT cards awaiting promotions/ci-gate receipts) — none falsely claims an open PR; no drift. Report-only.
4. **LAN root capture-source gate updated**: `source_authority.py` capture-source precondition per `docs/audits/2026-11-21-lan-capture-source-green-push-satisfiability.md` (other session, card `OPS-STG-LAN-CAPTURE-GREEN-PUSH-V1`) = all-green full-graph master push (or qualifying historical CI row); 42 push runs 2026-10-17..2026-11-21 had zero all-green, and the fresh tip push 44934578838 is still `ci-gate`-red (+9 debt reds) — the full publish at `:eb341dd8` is publication evidence, **not** a gate green. `OPS-STG-LAN-INSTALLED-COHORT-ROOT-V1` / `OPS-STG-REVIEWED-BASELINE-SUCCESSOR-V1` stay gated; do NOT rerun the frozen `--install --upgrade-idle-installed-cohort-v1` route (failed closed 2026-11-21, recorded in fence record).
5. **Fence axis consistent**: STAGING HELD `Codex-PR-Merge` gen 14/rev 57 `stale:true` — unchanged since the 2026-11-21 audit (that audit already read rev 57; no new fence traffic this window); DEV free gen 96/rev 375 with claim `6b500f31…` outcome PASS + deployment record `DEV-CHFIX-62B6FCA-20261121-V14` — dev-side delivery path fenced, observed, recorded. No fence violation; no fence mutation by this audit.
6. **Done ↔ deployment-record linkage**: CLI has no `deployment list`/`journal` verb; full-list records carry deploy-id text for only 2/77 `done` cards; the fence records prove the recent delivery path (DEV side). Verdict: this axis remains unverifiable via CLI (precedent F1); adjacent doc-gap card `DOC-STG-DEPLOYING-STATUS-SEMANTICS` already `available` (deliverable landed in `docs/DEPLOYMENT.md` via PR #1182). Report-only.
7. **`stg_ready_for_deploy` 86**: all HOLD behind the LAN root owner decision (package pins `:d4af0992`), consistent pool (precedent). Report-only.
8. **Census deltas vs the 2026-11-21 audit (rev 23536 → 23588, total tasks 1285 → 1287)**: available 904→900 (−4), in_progress 50→56 (+6): one verified newly created card (`OPS-STG-LAN-CAPTURE-GREEN-PUSH-V1`, created in_progress 2026-11-21T20:15Z) + claim transitions from `available` (UIV3 stories PR #1190/#1191 workers claimed by owner sessions); review 45, merging 7, wfh 3, done 77, blocked 94 unchanged. No unexplained pool movement.
9. **CI debt/matchmaking classes**: `CI-MATCHMAKING-DEADLINE-INTERNAL-20261120` not reproduced at tip push (matchmaking/matchmaking-contract + backend-go twins success) — stays registered per its retry policy (independent greens at the same head owed for a flake verdict). No new class this audit.
10. **Epoch/PLAN coherence**: `A4` TT title already `Parallel active — PLAN` (reconciled 2026-11-21) — no epoch action due; `A1` Active, GAME Parallel active, `A2` Next match PLAN. Report-only.
11. **`waiting_for_human` honest**: all 3 cards owner-bound with recorded decisions; the single reachable lever remains the LAN root owner-side path (inspector run + reviewed-baseline capture), itself blocked by the all-green master-push prerequisite (finding 4). Report-only.
12. **CLI capability gaps persist**: no `journal`/`deployment list` verbs; unpaginated `task list --status available` (limit 0) fails with `backend returned non-JSON response (HTTP 200)` — pagination mandatory (limit 900 used). Registry-semantics, report-only (precedent F4/F6 family).
13. **Main-checkout drift**: `D:\Git\Voice` on stale `1da2a4dc` + foreign dirty WIP — audit ran in a clean worktree from `origin/master`; main checkout left untouched (fleet rule, not a repo defect). Report-only.

### Actionable gaps — cards

- **1 new `available` card**: `OPS-GI-T58-OWNERLESS-IN-PROGRESS-V1` (finding 1; EVIDENCE + DECISION + priority in its description, audit path in evidence). Duplicate search `task search GI-T58` / `OWNERLESS` / `OWNER-RECONCILE` / `OPS-GI-T58` found no existing card for this finding.
- All other findings map to existing cards/registries or owner decisions: `OPS-STG-LAN-CAPTURE-GREEN-PUSH-V1` (available prerequisite work, owned by the LAN-capture session), `OPS-STG-REVIEWED-BASELINE-SUCCESSOR-V1`, `OPS-STG-LAN-INSTALLED-COHORT-ROOT-V1` (waiting_for_human owner decision), `DOC-STG-DEPLOYING-STATUS-SEMANTICS` (available), `CI-MATCHMAKING-DEADLINE-INTERNAL-20261120` (available), debt classes in `docs/todo/ci.md`.

### Deliberately not done

- **No status transitions on any foreign card** (the ownerless `GI-T58` repair is tracked as the new card, not executed here; 7 `merging` cards owned, 45 `review` owned — no evidence this audit can advance any).
- **No epoch update** (A4 already reconciled 2026-11-21; fresh read confirms).
- **No fence mutation** (STAGING held by `Codex-PR-Merge` with recorded stale-true pending; legal next step is owner-side inspector + capture decision, gated by the debt family).
- **No CI runs triggered** (inventory audit; drift verdict derived from registries + tip runs already dispatched).
- **No `deployment create`** (delivery path proven from fence records; fabricating deploy records from an audit is forbidden).
- **PR #1192 not touched** (another session's open docs PR; its merge decision belongs to that session).
- **Main `D:\Git\Voice` checkout left alone** (stale + foreign dirty WIP; sync is owner/fleet process, not an audit mutation).

## Actions

- TT: 1 new `available` card `OPS-GI-T58-OWNERLESS-IN-PROGRESS-V1` (A1 epoch, evidence cites this audit path). TT receipt comments (nick `TT-Fleet-Audit-20261122`, audit path cited in each): `GI-T58`, `OPS-GI-T58-OWNERLESS-IN-PROGRESS-V1`, `OPS-STG-LAN-CAPTURE-GREEN-PUSH-V1`, `OPS-STG-LAN-INSTALLED-COHORT-ROOT-V1`, `OPS-STG-REVIEWED-BASELINE-SUCCESSOR-V1`, `OPS-STAGING-INITIAL-RECONCILE-144`, `CI-BRANCH-PROTECTION-CHECKLIST-ALIGN-1`, `DOC-STG-DEPLOYING-STATUS-SEMANTICS`, `CI-MATCHMAKING-DEADLINE-INTERNAL-20261120`, `GI-T13` — 10 comments, verdicts per finding above.
- Repo: audit file added; `docs/audits/README.md` index row; `docs/PLAN.md` dated WIP note line; `docs/todo/ci.md` dated tip-audit receipt (contract drift CLEAN + tip push run classes + `owned-enrollment` infra re-observation).
- Repo worktree: `codex/tt-fleet-audit-20261122` from `origin/master`, docs-only commit → PR → merged to `master`.

## Reachable now

- **Owner/dispatcher (GI)**: reconcile the ownerless `in_progress` `GI-T58` per the new card (claim by GAME sprint actor or transition to `available`); GAME acceptance stays open per PLAN, independent of this registry repair.
- **Owner (LAN root)**: unchanged chain — fix the full-validation debt family (`ci-gate`, `full-validation-aggregation`, `local-ci-parity`, Space/Role proofs) and land an all-green full-graph master push; then euid-0 fresh capture bind (`VERIFIED_GATED_TARGET_BASELINE`) per `2026-11-21-lan-capture-source-green-push-satisfiability.md` / `2026-11-21-lan-reviewed-baseline-successor.md`. The tip merge already proves the repo-side precondition fires (lock artifact + 24 publish at `:eb341dd8`), but `ci-gate` red keeps the capture source unsatisfied. Do NOT rerun the frozen idle route.
- **Fleet**: UIV3 worker `#1191` in flight (its merge will fire another tip-pin data-preserving publish); LAN-capture session owns PR #1192 merge; this audit's docs PR is merged.

## Verdict

- **Inventory drift**: 1 new actionable finding (ownerless `GI-T58`, correcting the prior audit's "0 ownerless" statement) → 1 new `available` card; pools otherwise consistent (1 ownerless total, wfh 3 owners recorded, merging pool honest, both open PRs accounted); census deltas explained.
- **CI contract drift**: CLEAN at tip `eb341dd8` — 66/66 checklist coverage of live ci.yml jobs, verifier 34 names ⊆ live, zero job adds/removals since `bc9e4909`; tip push run has only registered debt reds; `owned-enrollment` infra class re-confirmed.
- **GHCR STG gate**: PRESENT 24/24 at both the fence package pin `:d4af0992` (fence candidate 7/7 exact digests) and the newest product pin `:eb341dd8` (tip push 44934578838 fired gated `staging-stack-lock` + 24-image data-preserving publish; 16 unchanged live digests by design).
- **Actionable gaps**: 1 new card; everything else carried/covered by existing cards, registries, or owner decisions (LAN root gate still the single deepest blocker, unchanged in kind).

## Self-corrections

- The 2026-11-21 audit's statement "owner fields complete — 0 ownerless in all three pools" was inaccurate: this audit's fresh scan at rev 23588 finds exactly one ownerless card in `in_progress` (`GI-T58`), whose presence in that pool is provable back to its 2026-11-01 event history. This audit's finding 1 and card supersede that earlier statement.
- The 2026-11-21 audit's GHCR gate was evaluated only at `:d4af0992`; this audit additionally evaluates the newest product pin `:eb341dd8` (24/24) — an extension, not a correction.

<!-- ttnrec:receipt kind=audit label=TT-Fleet-Audit-20261122 target=tt-fleet -->
