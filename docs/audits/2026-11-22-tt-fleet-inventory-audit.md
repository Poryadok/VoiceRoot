# TT-fleet inventory audit 2026-11-22 — TT rev 23622 / tip 85bbba48 (TT-Fleet-Inventory-Audit-20261122)

Agent: TT-Fleet-Inventory-Audit-20261122 (OpenChamber session, model qwen3.8-flash-next-coder-iq1_m).
Same-day third pass: precedents — `2026-11-22-tt-fleet-audit.md` (rev 23588 / tip `eb341dd8`) и `2026-11-22-ci-debt-family-root-causes.md`; findings re-verified fresh, duplicates get comments on existing cards, never second cards.

## Scope and method

- TT: `graph` (rev 23622, 1290 cards, fields code|title|status|type|epoch|owner|assignee|checkpoint_at|stale|next_action|blocked_by|depends_on|link) + `resource list` (fence state fresh at rev 23634).
- GitHub (repo `Poryadok/VoiceRoot`, worktree at `origin/master` tip `85bbba48e56f`, PR #1196 merge): open/closed PR states + heads, `gh pr checks` for the 3 open PRs, master branch-protection `required_status_checks` endpoint.
- Docs registries: `.github/workflows/ci.yml` live job set vs `.github/ci/branch-protection-checklist.md` (rebuild `bc9e4909`, PR #1174) vs `.github/ci/verify-required-jobs.sh`; `docs/CI.md`/`.github/BRANCH_PROTECTION.md` still absent (F9 precedent, registries = `.github/ci/*`).
- STG gate: `docker manifest inspect --verbose` of all 24 GHCR names from `scripts/ci/staging-image-catalog.json` (+ `nats-hub-config-renderer`) at the newest product pin and at the STAGING package pin.
- Git: ancestry `origin/master` vs develop-era shas claimed by merging cards; product-pin diff `eb341dd8`→tip.

## Ground truth

### Census at TT rev 23622

TOTAL 1290. Statuses: available 898, in_progress 61, review 45, merging 7, blocked 94, stg_ready_for_deploy 86, waiting_for_human 3, dev_ready_for_deploy 17, dev_test 2, done 77; legacy statuses (`testing`/`deploy`) 0.

Epoch totals: A1 220, A2 277, A3 75, A4 163, A5 49, A6 49, A7 44, GAME 81, H 40, BOTS 35, MOBILE 12, STORIES 22, VERIFICATION 7, UI-V3 132, POLISH 74, DEFERRED 10 (=1290).
Epoch in_progress by epoch: UI-V3 28, A1 16, A2 8, GAME 2, A4 1, A5 2, A6 1, H 1, VERIFICATION 1, POLISH 1 (=61).
Pool in_progress+review+merging = 113.

Deltas vs TT-Fleet-Audit-20261122 (rev 23588): total +3, available −2, in_progress +5 (review 45, merging 7, done 77, blocked 94, stg_ready 86, dev_ready 17, dev_test 2, wfh 3 unchanged).

### PR-lifecycle / merging-pool reality

Open PRs at tip: `#1191` head `16948359100c…` branch `codex/uiv3-panel-chat-groupsettings-20261121` (12 files), `#1193` head `954314ced1…` `codex/uiv3-panel-chat-emojipicker-20261121` (3 files), `#1195` head `d993c466…` `codex/uiv3-panel-chat-sendmenu-r2-20261121` (2 files). Closed: `#1101` head `6c7879ef`, `#1112` head `4dce5e0`, `#1174` head `badddc0d` (merged under merge commit `e5f0dba7`).

PR-head check verdicts (`gh pr checks`, runs 44938332145 / 44968151003 / 44990599621):
- `#1191`: ci-gate FAIL + reds = `backend-go (role)`, `backend-go-integration-pr (space)`, `compose-e2e`, `role-space-receipt-integration`, `space-native-installer-integration`, `backend-go-integration-pr (space)`-twin set — all the registered Space/Role ban-catalog + native50 debt families (CI-Debt-Family-Root-Causes-20261122 classes A/B); plus `owned-enrollment` FAIL (run 44938331941) infra class. `current-diff-contract` PASS.
- `#1193`, `#1195`: only `owned-enrollment` FAIL (separate runs 44968151031 / 44990599533); `ci-gate` PASS, `current-diff-contract` PASS — required-job set green at both.
- master branch protection `required_status_checks`: strict=false, contexts=["ci-gate"] — single gate, matches `.github/ci/gate-policy.md` precedent.

Merging pool (7 cards) sha claims re-proven against fresh `origin/master`: `a68a8fba…`, `a73d8a2d235e…`, `cabd1fce6d80…`, `fc101f922774…`, `4dce5e0`, `6c7879ef` all NOT-IN master (card claims honest); `ffad84570c…` (f13 source pin), `d4af0992…`, `44bd3c7` IN master.

### CI contract drift at tip `85bbba48`

- Live `ci.yml` job set = 66; checklist rebuild `bc9e4909` covers 66/66 (missing 0); adds/removals since `bc9e4909`: 0/0; `ci.yml`, verifier, gate-policy, path-filters byte-identical since `bc9e4909` (only `branch-protection-checklist.md` itself changed, +93/−24 — its own rebuild).
- `verify-required-jobs.sh`: 127 lines, ~34 enforced conditional job names, all ⊆ live graph; `current-diff-contract` required on every PR.
- Verdict: CLEAN, no drift.

### GHCR digest gate (fresh `docker manifest inspect --verbose`, docker 27.4.0)

24/24 PRESENT at newest product pin `eb341dd80325cfb640bfb9b519fcb0926dbff06c` AND at STAGING package pin `d4af0992db2a50fac8eb94758e1020ad315f8be9` (fence pin, gen14/rev57 HELD):

```
[eb341dd80325…] analytics cbd08839 | bot 75c7ac75 | chat c3dd84ba | file 4043d9c8 | gameintegration 135a06d0 | gateway 75ba1fde | matchmaking c77f49b5 | messaging 36d62f80 | moderation ab2a4c67 | notification b0a56524 | realtime 33cff769 | role 7b3e422f | search b3682af0 | social 06e11b26 | space e9778e8f | story 12161f29 | subscription ce069bca | user 74484f00 | voice b3ddd3de | auth 885d1ce2 | web 5b563564 | developer-portal df35b662 | admin 25e8bb69 | nats-hub-config-renderer 45cc2222
[d4af0992db2a…] analytics bb95be18 | bot 74fb1cf7 | chat b4ade2ed | file b9791359 | gameintegration 81e9c896 | gateway a22fdee9 | matchmaking e95364cd | messaging c69762ea | moderation 2cfcab6f | notification bf40d51e | realtime f4ea05d4 | role 38fd1eaa | search 000726cf | social 52ce0bc4 | space d8a80083 | story 47ecf11f | subscription 61bd0ac1 | user e1e7b9fb | voice 03b0d8b7 | auth 5a12dc03 | web 3256a817 | developer-portal d64e732e | admin 44fa715b | nats-hub-config-renderer 5489892b
```

- Product-pin diff `eb341dd80325…`→tip: 6 files, all docs (PLAN.md, 3 audits, audits/README.md, docs/todo/ci.md) → `eb341dd8` remains the newest product pin.
- Digest cross-check vs fence evidence: `:d4af0992` digests byte-match the HELD STAGING candidate map (`admin@sha256:44fa715b…`, `auth@sha256:5a12dc03…`, `developer-portal d64e732e…`, `notification bf40d51e…`, `realtime f4ea05d4…`, `user e1e7b9fb…`, `web 3256a817…` and 16 unchanged live digests); changed-7 at `:eb341dd8` (`admin 25e8bb69, auth 885d1ce2, developer-portal df35b662, notification b0a56524, realtime 33cff769, user 74484f00, web 5b563564`) byte-equal to the values recorded by `2026-11-22-tt-fleet-audit.md` — GHCR publication stable, no drift.
- Catalog: 23 catalog names re-read fresh from `scripts/ci/staging-image-catalog.json` at tip + renderer = 24 checked.

### Fences (fresh `resource list` at rev 23634)

- STAGING: state HELD, gen 14 / rev 57, owner `Codex-PR-Merge`, claim `b73c1c98-1b13-43f9-a5db-9b046f71de6f`, phase deploy, `stale: true`; pending records LAN ceremony attempt 2026-11-21 failed closed (`operator_retirement_binding_invalid` inside media-abort terminal closure of `rollout-0e2be62a3326`), read-only inspector `/tmp/insp2026.py` staged at node, next_action = owner euid-0 read-only inspection then reviewed successor; frozen `--install --upgrade-idle-installed-cohort-v1` rerun stays banned. Unchanged since the 2026-11-21/22 records.
- STAGING wait `6d687b5a…` actor `Рудеус-144` `ready:false` (reconcile inbox/processing/journal under legitimate operator authority) — persists.
- DEV: state free, gen 96 / rev 375, last claim `6b500f31-0137-492a-93a3-7c6e6ee80aef` op 52 `outcome PASS`, deployment `DEV-CHFIX-62B6FCA-20261121-V14`, next_action "next owner must reobserve actual version and reconcile its candidate". Unchanged since 2026-11-21 records.

### waiting_for_human (3)

- `GI-T13` (GAME): owner `Codex-PR-Merge`, stale true, next_action "continue waiting_for_human".
- `OPS-STAGING-INITIAL-RECONCILE-144` (A1): owner `Рудеус-144`, stale true — matches the open STAGING wait above.
- `OPS-STG-LAN-INSTALLED-COHORT-ROOT-V1` (A1): owner `Codex-PR-Merge`, stale true; next_action cites owner euid-0 ceremony per c14 plan + repo-side unblock = first code-merge master push fires `staging-stack-lock` + promote 24 (UIV3 PRs in flight, not yet merged).

## Findings

1. **Ownerless pool persists: `GI-T58` only** (GAME, in_progress; neither owner nor assignee in 1290-card dump; pool=113 re-scanned). Duplicate of the `2026-11-22-tt-fleet-audit.md` finding → receipt comment on `GI-T58` and on the existing remediation card `OPS-GI-T58-OWNERLESS-IN-PROGRESS-V1` (still `available`), no second card.
2. **Merging-pool honesty re-proven** (6 develop-era shas NOT-IN master, claims verbatim honest; `CR-20261102-06` names PR #1101 merged to **develop** only — #1101 is CLOSED; `BE175-EXISTING-VOLUME-ASSERTIONS` names PR #1112 source `4dce5e0` CLOSED) → receipt comments on the 7 merging cards, zero transitions by this audit.
3. **New open PRs `#1193`/`#1195` since rev 23588** with in_progress cards `UIV3-PANEL-CHAT-EMOJIPICKER` (claim `Codex-UI-EmojiPicker`) and `UIV3-PANEL-CHAT-SENDMENU` (claim `Codex-UI-SendMenu`); their required-job sets are green at PR-head (`ci-gate`/`current-diff-contract` PASS), only the registered `owned-enrollment` infra class red — merge-path precedent applies, no new class. `#1191` reds = registered debt classes A/B only.
4. **CI contract drift CLEAN** at fresh tip (66/66, verifier ⊆ live, 0 adds/removals since `bc9e4909`); `docs/CI.md`/`.github/BRANCH_PROTECTION.md` still absent — report-only F9 precedent, no card.
5. **GHCR digest gate PASS 24/24 at both pins**, digests byte-stable vs prior audit records and vs the HELD STAGING fence candidate — publication chain intact, STG `:d4af0992` HOLD unchanged.
6. **Census delta attribution is status-math only** (total +3 = in_progress +5 + available −2): TT CLI has no global journal (`journal` verb absent; per-card `task events` exists but a 113-card events scan is out of budget) — reported as a limitation, not a finding.
7. **done↔deployment cross-check is CLI-unverifiable**: 0 of 77 `done` cards expose a `deployments` property in `graph` output, and `resource list` exposes deployment ids only inside fence evidence (`DEV-CHFIX-62B6FCA-20261121-V14`, `STG-PKG-D4AF0992-…` precedent) — enumeration axis limited, honestly no claim either way.
8. **Epoch coherence vs PLAN holds**: epoch titles read fresh at rev 23622 — `A1 — Active по PLAN`, `A2 — Next по PLAN`, `A3 — Queued`, `A4 — Parallel active по PLAN`, GAME `разразённый параллельный спринт; development acceptance OPEN` — exactly the PLAN WIP table; A4 already reconciled 2026-11-21, no epoch action. `22 hours`/`1 hour 16 min` stale PLAN markers: absent already at fresh tip (re-grepped).
9. **Stale-claim markers on held cards** (pool owners `stale=true`: `ADMIN-E2E-FLAKE`, `CI-BRANCH-PROTECTION-CHECKLIST-ALIGN-1`, `CR-20261102-07/15/16`, `OPS-GITHUB-HOST-TRANSPORT`, `OPS-TREEHOUSE-CAPACITY-20261019`, `BE-222`, wfh 3, dev_test 2, stg_ready 86 cohort) — same shape as at rev 23588, custody hygiene informational, no action mandated by contract.

## Actions taken

- 0 new TT cards (every actionable finding is carried by an existing card; duplicates → comments).
- TT receipt comments (nick `TT-Fleet-Inventory-Audit-20261122`, all cite `docs/audits/2026-11-22-tt-fleet-inventory-audit.md`): `GI-T58`, `OPS-GI-T58-OWNERLESS-IN-PROGRESS-V1`, `GI-T13`, `OPS-STAGING-INITIAL-RECONCILE-144`, `OPS-STG-LAN-INSTALLED-COHORT-ROOT-V1`, `OPS-STG-LAN-CAPTURE-GREEN-PUSH-V1`, `CI-BRANCH-PROTECTION-CHECKLIST-ALIGN-1`, `UIV3-PANEL-CHAT-GROUPSETTINGS`, `UIV3-PANEL-CHAT-EMOJIPICKER`, `UIV3-PANEL-CHAT-SENDMENU`, `CR-20261102-06`, `CR-20261102-07`, `CR-20261102-10`, `CR-20261102-15`, `CR-20261102-16`, `BE175-EXISTING-VOLUME-ASSERTIONS` (16 comments total, all success).
- docs-only commit + PR from branch `codex/tt-fleet-inventory-audit-20261122` into master, merged with merge commit under the debt-red `ci-gate` precedent (docs-only diff, `current-diff-contract` green).

## Verdict

TT-fleet inventory at rev 23622 is coherent with GitHub reality at tip `85bbba48` and with PLAN WIP direction: no epoch drift, no CI contract drift (CLEAN verdict persisted a 4th time), GHCR digest gate 24/24 at both live pins with byte-stable digests, fences stable (STAGING HELD behind LAN owner decision incl. wait `ready:false`; DEV free with delivered `DEV-CHFIX-62B6FCA-20261121-V14`), 86 `stg_ready_for_deploy` HOLD unchanged, ownerless pool has exactly one card (`GI-T58`) with its remediation card open, merging-pool 7 claims honest, 3 wfh owners recorded. Zero new product or CI classes; all deltas are attributable to other sessions' claimed in-flight work (UIV3 PRs, CI-debt fix claims `Codex-CI-Debt-FixA`/`-FixB`, LAN capture claim `Codex-LAN-Capture-Debt`). This audit transitioned no statuses and touched no fences.

## Deliberately not done

- No `done↔deployments` enumeration (CLI lacks a list verb; limitation recorded above).
- No per-card `task events` scan (113 cards out of budget; delta attribution left as status math, recorded honestly).
- No rerun of frozen LAN ceremony (`--install --upgrade-idle-installed-cohort-v1` BANNED; fail-closed 2026-11-21 stays the recorded root cause).
- No status transitions, claims, deployments, PR links or CI claims of any kind beyond the read receipts above.
- CI full-master dispatch verdict not re-run this pass (night gate is its own scheduled audit; PR-head evidence cited, not extrapolated to push runs).
