# TT-fleet audit 2026-11-21 — Inventory audit at TT rev 23536, master tip `7d7808a7`

> **Consumers**: Fleet dispatcher — fresh inventory baseline (per-status TT census, PR-lifecycle cross-check, owner-gated pools) and actionable-gap enumeration; owners — fence/epoch verdicts for dispatch decisions; CI controllers — contract-drift verdict at tip. Evidence-only audit; no status transitions on foreign cards, no fence mutation, no CI runs triggered.

Label: `TT-Fleet-Audit-20261121`. Tracker: `tt --url http://192.168.0.109:30990 --project voice`, `--timeout=30s`. Worktree: fresh OpenChamber worktree `codex/tt-fleet-audit-20261121` at `origin/master` `7d7808a773364c06dfa3f4a696e66ef75b21c0e3` (clean at audit; the main `D:\Git\Voice` checkout stayed on stale, heavily-dirty `1da2a4dc` — not merged, not forced, left untouched per fleet rule).
Model: qwen3.8-flash-next-coder-iq1_m.

## Scope

- Routine TT-fleet inventory audit: statuses enumeration, per-card PR reference, links, done/deploy records, waiting_for_human owners, epoch/PLAN WIP alignment, resource fences.
- GitHub PR ↔ TT consistency (open PRs vs TT `merging`/`review`/`in_progress`).
- CI contract drift: required-check lists (branch-protection checklist, gate-policy, `verify-required-jobs.sh`) vs live `.github/workflows/ci.yml` job graph at tip.
- STG rollout GHCR digest gate: `scripts/staging/verify-staging-images.sh` presence check against the package pin in the STAGING fence purpose.

## Ground truth

- Tracker revision: 23536 (stable across all fresh reads this audit).
- GitHub: `gh pr list --repo Poryadok/VoiceRoot --state open` = `[]`; master tip = PR #1186 merge `7d7808a7` (2026-11-20T17:46:35Z) — `docs: LAN retirement authority drift — root cause + gate verdict + action path (plan note)`.
- Status totals (fresh per-status queries): available 904, in_progress 50, review 45, merging 7, dev_ready_for_deploy 17, dev_deploying 0, dev_ready_for_test 0, dev_test 2, stg_ready_for_deploy 86, stg_deploying 0, stg_ready_for_test 0, stg_test 0, acceptance 0, waiting_for_human 3, blocked 94, done 77, archived 0. Legacy statuses (todo/doing/pr_open/stg_doing/stg_ready_for_accept/testing/deploy) = 0 — the vocabulary drift cleanup of 2026-11-18/19 still holds.
- PR-lifecycle cross-check (58 unique PR refs across 50 in_progress + 45 review + 7 merging, owner fields complete — 0 ownerless in all three pools): 56/58 `MERGED`, 2/58 `CLOSED` unmerged: `DELIVER-BE085-PR915` (review, !915, owner `Codex-Status-Audit-20261101`, known since 2026-11-19) and `UIV3-OVERLAY-CALL-OUTGOING` (review, !1131, owner `Ursula-7`) — new observation this audit.
- waiting_for_human = 3, all with owners and recorded decisions (see A-axes below).
- Fences (fresh `resource list`): DEV state `free`, **revision 375, generation 96**, phase `prepare`, fence purpose verbatim "Independent DEV verification of CH OOM cards 2026-11-21: read-only observation + record V12 + recheck/accept under this claim; zero cluster mutation"; outcome `PASS`, candidate `62b6fca8` with a 47-image observed map (evidence: fresh `voice-dev` kubectl re-observation 2026-11-21, ConfigMap `voice-clickhouse-memory-policy-v2` uid `a472c3a4…` rv 4371975, CH OOM class does not reproduce); DEV fence record carries deployment record `DEV-CHFIX-62B6FCA-20261121-V14` — the dev-side delivery path is fenced, observed and recorded; DEV claim `6b500f31-0137-492a-93a3-7c6e6ee80aef` present in the fence claim fields with outcome PASS. STAGING HELD by `Codex-PR-Merge`, claim `b73c1c98-1b13-43f9-a5db-9b046f71de6f`, generation 14, **revision 57** (55 on 2026-11-19 → +2: LAN ceremony attempt today + fence-record append), phase `deploy`, `stale:true`, pending verbatim "LAN human root ceremony ATTEMPTED 2026-11-21 and FAILED CLOSED: controller.Blocked `operator_retirement_binding_invalid` inside the media-abort terminal closure proof of `rollout-0e2be62a3326` (route `upgrade_idle_installed_cohort_v1`; frames at tip `fd97cec7`: installer.py L575/L455 → _upgrade_idle_transition L324 → operator_retirement.py idle_state L1451 → v…)", candidate = package pin `d4af0992` with a 23-image data-preserving map (purpose verbatim: "16 unchanged live digests + 7 services admin,auth,developer-portal,notification,realtime,user,web to exact :d4af0992 GHCR digests (published durably by CI publish run 44329726561 on master 3c2a24e3, 22/22 publish jobs success, gate 24/24)"); waits: STAGING reconcile wait `ready:false`, DEV wait cancelled.
- Epochs (fresh `epoch list`): A1 (Active — cards in progress/review/merging/dev/STG), GAME (Parallel active — 11 done+), A2 (Next, 5 cards), **A4 — TT title still `Queued` while PLAN.md line 29 says `Parallel active` (evidence 2026-11-20)**; A3, A5–A7 Queued.
- GHCR gate (fresh `docker manifest inspect --verbose` today): package pin `:d4af0992db2a50fac8eb94758e1020ad315f8be9` — **24/24 present** (23 catalog + `nats-hub-config-renderer`).

## Findings

### CI contract drift at tip — CLEAN, no drift

- Live graph: `.github/workflows/ci.yml` at `7d7808a7` = **66 jobs**.
- Authority `.github/ci/branch-protection-checklist.md` (rebuilt 2026-11-19 at `bc9e4909`, PR #1174 MERGED, merge commit `e5f0dba7`): Tier1 (4) + Tier1b (45) + Tier2 (4) + Tier2b (1) + Tier3 (12) covers all 66 — `notInChecklist=[]`, `clNotJobs=[]`.
- `.github/ci/verify-required-jobs.sh` enforces Tier1(4)+Tier2(4)+Tier3(12) — all names exist in the live graph, all ⊆ checklist; `ci.yml` renames since `bc9e4909` = none (diff inspected).
- `gh pr view` on a merging card shows branch-protection checks not yet enumerated (gate not started at audit moment) — not a contract drift; registry drift verdict CLEAN.
- `docs/CI.md` and `.github/BRANCH_PROTECTION.md` from the consumer contract still do not exist in the repo — the real registries are `.github/ci/*` (README contract line 7 and the 2026-11-19 F9 entry already document this; no doc action due).

### STG rollout GHCR digest gate — PASS on presence, semantics of the map recorded

- Gate = `scripts/staging/verify-staging-images.sh` presence check (`docker manifest inspect` at the 40-hex STACK_LOCK tag): **24/24 images present at `:d4af0992`** (catalog `scripts/ci/staging-image-catalog.json` = 23 + renderer) — matches the 2026-11-20 publish runs 22/22 and the fence comment `gate: 24/24`.
- Candidate map vs GHCR truth: the 7 services changed between product pin `62b6fca8` and package pin `d4af0992` (admin, auth, developer-portal, notification, realtime, user, web) promote to **exactly the `:d4af0992` GHCR digests** (7/23 match); the other 16 candidate entries carry **live cluster digests** — by design data-preserving semantics (fence purpose verbatim: `16 unchanged live digests + 7 services ... to exact :d4af0992 GHCR digests`). Not drift; recorded for the rollout ceremony.
- `:7d7808a7` (head) and `:62b6fca8` probes: `manifest unknown` for admin/web/renderer — refs published only per package pin; the gate runs against the pinned tag, so absent head/next-pin refs are not drift. A `:62b6fca8` full sweep was not run (the pinned-tag rule is the gate contract, verbatim in the fence purpose).

### Inventory drift and actionable findings

1. **PLAN.md WIP ↔ TT epoch `A4` title drift** — PLAN line 29 says `Parallel active` (evidence 2026-11-20: A4 holds 4 in_progress, 8 review, 2 merging, 37 `stg_ready_for_deploy` cards), while the TT epoch record still reads `A4 — безопасность и автономный release harness; Queued`. Action: reconciled directly via `epoch update A4 --title ... Parallel active по PLAN` (TT-authoritative record; PLAN already holds the corrected wording). No card transitions.
2. **Waiting pool**: `GI-T13` (GAME, owner `Codex-PR-Merge`) is an *agent*-bound human gate (Google OAuth real-provider admission; options A/B recorded 2026-11-20) — the card is honest (`waiting_for_human` per its own definition), but the "human" in it is a named agent owner, so it will not auto-advance; owner decision required. Report-only (registry semantics, precedent 2026-11-19 F1).
3. `OPS-STAGING-INITIAL-RECONCILE-144` (A1, owner `Рудеус-144`): `next_action` cites PR #1167 which is now `MERGED` (verified) — reconcile closure awaits the STG rollout ceremony gated by the LAN root (86 `stg_ready_for_deploy` cards held). Honest wfh.
4. `OPS-STG-LAN-INSTALLED-COHORT-ROOT-V1` (A1, owner `Codex-PR-Merge`): root cause resolved 2026-11-20 (media-target authority-family drift 23 vs 24, gameintegration era `1f6e7751…`); the 2026-11-21 ceremony attempt `--install --upgrade-idle-installed-cohort-v1` failed closed (`operator_retirement_binding_invalid`) and is recorded in the STAGING fence record; next_action = user runs node-side `sudo python3 /tmp/insp2026.py` (node `/tmp/insp2026.py` sha256 `df845ebf…`, 13022B) then the owner's reviewed-baseline capture decision (`VERIFIED_GATED_TARGET_BASELINE`) per README :94–96. Do NOT rerun the frozen route (README :91–92, fence record c11).
5. **Done ↔ deploy-record linkage**: CLI has no `deployment list` verb and full-list records carry deploy-id text for only 2/77 done cards; but the fence records prove the delivery path: DEV fence record has `DEV-CHFIX-62B6FCA-20261121-V14` (deployment record, observed 2026-11-21). Verdict: the "done without deployment record" axis is unverifiable via CLI (precondition F1 of 2026-11-19 stands); no new card — the already-`available` registry-gap card `DOC-STG-DEPLOYING-STATUS-SEMANTICS` covers the adjacent doc gap (landed in `docs/DEPLOYMENT.md` via PR #1182).
6. **Card ↔ closed-unmerged PR (new)**: `UIV3-OVERLAY-CALL-OUTGOING` (review, `!1131` CLOSED unmerged 2026-11-20, owner `Ursula-7`) — work-item card exists; precedent `DELIVER-BE085-PR915` stays report-only. Report-only.
7. **CI contract registry existence**: `docs/CI.md` + `.github/BRANCH_PROTECTION.md` absent — real registries `.github/ci/*`. Report-only (documented).
8. **CI gate debt-red**: merge-path precedent `--merge --admin` holds (see 2026-11-20 audits); no new class observed at tip this audit. Report-only.
9. **PR #1167 merged ↔ reconcile not closed**: reconcile closure needs the rollout ceremony — held by finding 4's gate. Not a second action.
10. **GHCR tag absence**: head/`62b6fca8`-tag manifests not published — not drift (pinned-tag gate). Report-only.
11. **Dev pool healthy**: `dev_ready_for_deploy` 17, `dev_deploying` 0, `dev_ready_for_test` 0, `dev_test` 2; DEV fence free; dev-side ceremony in flight via A4 cards.
12. **STAGING fence rev 57 vs 55**: +2 fence revisions from 2026-11-20 LAN attempts + record append; owner `Codex-PR-Merge`, claim `b73c1c98`, `stale:true` with recorded next action — consistent, not a violation.
13. **Epoch card distribution**: GAME 81 (11 done), A1 131, A2 5, A3 14, A4 67, A5 38, A6 10, A7 16 — all cards carry epoch assignments (0 unassigned among listed records; totals match status sums ±). Report-only.
14. **`stg_ready_for_deploy` 86** all behind the LAN root gate — consistent pool (package pins `d4af0992`), HOLD verdict.
15. **CLI capability gaps observed today**: `journal status` verb absent from the installed CLI (fence/wait history read via `resource list` records instead); unpaginated `task list --status available` (limit 0) fails with `backend returned non-JSON response (HTTP 200)` — pagination required (limit 900 worked). Registry-semantics gaps, report-only (precedent F4/F6 family).
16. **Main-checkout drift**: `D:\Git\Voice` on stale `1da2a4dc`, heavily dirty (foreign WIP) — audit ran in a clean worktree from `origin/master`; main checkout left untouched (fleet rule, not a repo defect).

### Actionable gaps → cards

- **0 new cards.** Every actionable finding either got an action this audit (epoch-title reconciliation) or maps to an already-existing card/registry: `OPS-STG-MEDIA-AUTHORITY-FAMILY-DRIFT-23-24-V1` (available), `OPS-STG-LAN-INSTALLED-COHORT-ROOT-V1` (waiting_for_human — the only unblock = owner's euid-0/inspector decision), `DOC-STG-DEPLOYING-STATUS-SEMANTICS` (available, deliverable landed), `CI-MATCHMAKING-DEADLINE-INTERNAL-20261120` (available, open CI class), waiting owners recorded. Dup-search (`task search <prefix>`) confirms no missing actionable card.

### Deliberately not done

- **No status transitions on any foreign card** (7 `merging` owned by `Codex-PR-Merge`, 45 `review` owned — no evidence this audit can advance any; the next human gate is the LAN root inspector/decision, not a card transition).
- **No fence mutation** (STAGING held by `Codex-PR-Merge` with recorded stale-true pending; the only legal next step there is the user-side inspector run + owner decision — fleet never mutates owner-held fences as part of an inventory audit).
- **No CI runs triggered** (inventory audit; tip has no branch protection set on PR #1186 — gate would fail trivially until the STG ceremony enforces tiers; drift verdict from registries is enough).
- **No `deployment create`** (delivery path proven from fence records; creating dev/stg deploy records from an audit would fabricate evidence of ceremonies not observed end-to-end).
- **No doc edits of CI/branch-protection docs** (registries current at tip; only this audit file + dated receipts).
- **Main `D:\Git\Voice` checkout left alone** (stale `1da2a4dc` + foreign dirty WIP — merge into it was aborted by design; sync is owner/fleet process, not an audit mutation).

## Actions

- TT: `epoch update A4 --title "A4 — безопасность и автономный release harness; Parallel active по PLAN"` (reconciles the epoch record with PLAN.md line 29; position unchanged).
- TT receipt comments (audit `TT-Fleet-Audit-20261121`): `CI-BRANCH-PROTECTION-CHECKLIST-ALIGN-1` — drift-clean verdict at tip 7d7808a7 (66/66, verify 66/66, renames none since `bc9e4909`); `DOC-STG-DEPLOYING-STATUS-SEMANTICS` — deliverable landed in `docs/DEPLOYMENT.md` via PR #1182 (status-registry section :248–255), audit confirms at tip; `OPS-STG-MEDIA-AUTHORITY-FAMILY-DRIFT-23-24-V1` — audit reconfirmed fence/era evidence fresh 2026-11-21, ceremony attempt today failed closed as predicted.
- Repo: audit file added; `docs/audits/README.md` index row; `docs/PLAN.md` WIP note line (2026-11-21 TT-fleet audit at rev 23536: A4 epoch reconciled, verdicts); `docs/todo/ci.md` dated receipt (contract drift CLEAN at tip, no new red class).
- Repo worktree: `codex/tt-fleet-audit-20261121` from `origin/master`, docs-only commit → PR → merged to `master`.

## Reachable now

- **User (owner-side, euid-0)**: node-side `sudo python3 /tmp/insp2026.py` (inspector run recorded in the STAGING fence record), then the reviewed-baseline capture decision (`VERIFIED_GATED_TARGET_BASELINE`, README :94–96) — this is the single gate unlocking the 86-card STG rollout; do NOT rerun the frozen `--install --upgrade-idle-installed-cohort-v1`.
- **Fleet/owners**: A4 STG ceremony cards continue in_progress/review/merging behind the LAN gate; A1 reconcile closure (`OPS-STAGING-INITIAL-RECONCILE-144`) completes after the rollout ceremony; `GI-T13` awaits owner (agent-bound) decision on options A/B; CI gate tiers stay green-only until the STG ceremony enforces the tiers (documented rule, unchanged).
- **Next audits**: the done/deploy-record linkage axis needs a future CLI `deployment list` verb (precondition F1); the `UIV3-OVERLAY-CALL-OUTGOING` ↔ !1131 pair needs owner closure (report-only until owner decides).

## Verdict

- **Inventory drift**: 1 reconciled action (epoch A4 title ↔ PLAN), 1 new observation (UIV3 ↔ closed !1131), 14 carried/known — no unexplained drift; owner-gated pools honest; fence traffic consistent (STAGING rev 57 with recorded LAN attempts, DEV delivered).
- **CI contract drift**: CLEAN at tip — checklist `bc9e4909` covers 66/66 live jobs, `verify-required-jobs.sh` enforces 66/66, no renames since rebuild; PR-open↔TT `merging`/`review` consistent (0 open PRs ↔ 0 pr_open).
- **GHCR STG gate**: PRESENT 24/24 at pinned `:d4af0992`; candidate map 7-changed-exact + 16-live by design; head/next-pin refs absent per pinned-tag gate contract — not drift.
- **Actionable gaps**: all covered by existing cards/registries or owner decisions (0 new cards); the single reachable-now lever is the user-side LAN inspector run + reviewed-baseline decision.

<!-- ttnrec:receipt kind=audit label=TT-Fleet-Audit-20261121 target=tt-fleet -->
