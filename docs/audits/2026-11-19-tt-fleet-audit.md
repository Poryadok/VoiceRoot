# TT fleet audit — 2026-11-19

**Label**: `TT-Fleet-Audit-20261119` (OpenChamber session, model `qwen3.8-flash-next-coder-iq1_m`).
**Role**: routine fleet inventory audit of the `voice` project — TT tracker ↔ GitHub reality ↔ docs registries.
This is not a re-audit of the full 16-epoch card inventory; deep epoch work belongs to epoch audits.

## Scope and method

- TT CLI: `tt.exe --url http://192.168.0.109:30990 --project voice` (per `ttenrec` skill; exit code 1 with valid JSON output is CLI/banner noise).
- GitHub: `gh api` against repo `Poryadok/VoiceRoot`, default branch `master`.
- GHCR: `docker.exe manifest inspect --verbose` on public package `ghcr.io/poryadok/voiceroot/*` — unauthenticated, no auth wall.
- Docs authorities read as ground truth: `docs/PLAN.md`, `docs/audits/README.md`, `docs/todo/ci.md`,
  `.github/ci/branch-protection-checklist.md`, `.github/ci/gate-policy.md`, `docs/TESTING.md`,
  `scripts/ci/staging-image-catalog.json`.
- TT read snapshot taken **before any write** at tracker revision **23202**.

## Ground truth at snapshot

### TT status totals (revision 23202)

```
available 898 | in_progress 50 | waiting_for_human 2 | review 45 | merging 6
dev_ready_for_deploy 4 | dev_deploying 0 | dev_ready_for_test 0 | dev_test 2
stg_ready_for_deploy 86 | stg_deploying 0 | stg_ready_for_test 0 | stg_test 0
acceptance 0 | done 77 | blocked 94
```

`tt brief` reports buckets `active 56 / blocked 99 / done 76 / queues 148`; these bucket definitions
do not match the per-status sums above (52 in-progress+human, 94 blocked, 77 done). Bucket semantics
were not audited — observation only (see F9).

### waiting_for_human — exactly 2 cards, both owner-bound

| Card | Epoch | Owner | Note |
|---|---|---|---|
| `OPS-STAGING-INITIAL-RECONCILE-144` | A1 | `Рудеус-144` | human owner; `next_action` cites superseded PR #1167 (see F2) |
| `GI-T13` | GAME | `Codex-PR-Merge` (assignee same) | owner-bound, but the bound actor is an agent, not a human (see F3) |

### PR-lifecycle pool

`merging` = 6: `BE175-EXISTING-VOLUME-ASSERTIONS`, `CR-20261102-06/07/10/15/16`.
`review` = 45, incl. `A1-GIS-REV32-VERIFIER`, `DELIVER-BE085-PR915`, `D110-PR1006-DELIVERY`,
`FLUTTER-MATCH*` goldens, `CR-20261102-08/09/11`.

### Epoch history (coherence inputs)

| Epoch | total | in_progress | wfh | review | merging | dev_rdd | dev_test | stg_rdd | done | blocked | available |
|---|---|---|---|---|---|---|---|---|---|---|---|
| `A1` | 208 | 12 | 1 | 10 | 4 | 4 | 2 | 22 | 12 | 8 | 133 |
| `A4` | 162 | 1 | 0 | 8 | 2 | 0 | 0 | 11 | 2 | 4 | 134 |
| `GAME` | 81 | 2 | 1 | 1 | 0 | 0 | 0 | 4 | 45 | 21 | 7 |

`A4` `in_progress` card is `BE-019` (assignee `astrie`, title “Auth account confirmation/restore/terminal
erasure coordinator”). Epoch count: 16.

### GitHub reality at tip

- Tip `origin/master` = `3c2a24e3112daecdc9e54b25b46d309ea35610c2` (2026-11-18T14:54:36Z).
- Open PRs: **0**. `#1167` CLOSED UNMERGED (superseded).
- Merged docs chain: `#1169` → `1b20b39873…` → `#1170` → `994983cd…` → `#1171` → `3c2a24e3…` (tip).
- Package pin `d4af0992db2a50fac8eb94758e1020ad315f8be9` = commit of PR `#1166`.
- Product pin (fence purpose): `62b6fca83b606c4f458f190d20a7e8e02610168c`.

### CI at tip (run-level actuals)

| Run | started | shape | verdict |
|---|---|---|---|
| `44329501457` | 2026-11-18T14:54:40Z, @tip | full profile push; ~150 jobs enumerated | **FAILURE** — 7 reds: `local-ci-parity`, `role-space-receipt-integration`, `backend-go (role)`, `space-native-installer-integration`, `backend-go-integration-pr (space)`, `backend-go-integration (space)`, `compose-e2e`; `ci-script-tests` ran GREEN (4 matrix sub-jobs `(staging)/(proofs)/(policy)/(runtime)`); `ci-gate`, `current-diff-contract`, `full-validation-aggregation` absent from this dispatch |
| `44329726561` | 14:55:52Z, @tip | path-filtered publish dispatch: 87 jobs = 23 SUCCESS (`changes` + 22 × `staging-images-dispatch-publish`) / 64 skipped | SUCCESS but **validation jobs all SKIPPED** (`ci-gate`, `ci-script-tests`, `full-validation-aggregation`, `current-diff-contract`, `backend-auth`, `chat-auth-java-integration`, `staging-images-promote`, `staging-stack-lock`, `deploy-staging`) — publish evidence only |
| `44426480170` | 2026-11-19T00:07:48Z | docs-only push, all jobs skipped except `changes` | inert |
| `44403401294` | “Staging deploy” workflow | `deploy` job FAILURE = `NATS_ROLLOUT=RESOURCE_FENCE_UPGRADE_REQUIRED` LAN gate; rest skipped | pending human gate (F4) |

Tip-level verdict: **not green**. The 7 reds of the full run are exactly the documented debt family of
`docs/todo/ci.md` (Space/Role Go ban-catalog shard, native50 pin chain, parity); no new red class at tip.
`ci-gate` class is *unproven* at tip (not re-run), not disproven.

### STG rollout gate — GHCR digest cross-check (re-verified 2026-11-19)

All 23 images of `scripts/ci/staging-image-catalog.json` (version 1, no digest fields; targets are
name+git-pin tag) plus `nats-hub-config-renderer` checked against `:d4af0992db2a50fac8eb94758e1020ad315f8be9`:

```
analytics bb95be18509f62fd  gameintegration 81e9c8965016fbd6  moderation 2cfcab6f9d85514a
bot       74fb1cf7cd33007a  gateway         a22fdee9883a9542  notification bf40d51e1377b37e
chat      b4ade2edf43d58fd  matchmaking     e95364cd18e5567f  realtime     f4ea05d42dec151b
file      b9791359860a1130  messaging       c69762eacb523ea5  role         38fd1eaadda1b602
social    52ce0bc4fe89fbce  search          000726cf5e92cfb7  space        d8a800838e071b1a
story     47ecf11fbf9f152a  subscription    61bd0ac13bcf0882  user         e1e7b9fbdafdd1ae
voice     03b0d8b73e749448  auth            5a12dc03230f9915  web          3256a81713817e6c
dev-portal d64e732ec9b09295  admin          44fa715b09adf989  (digests truncated to 16 hex; full in receipt)
nats-hub-config-renderer  PRESENT sha256:5489892bad61614f935ae250dda561ed3501b1ab5c78e8de72499fd5aa3b9e64
```

**24/24 PRESENT at the package pin `:d4af0992…`** (renderer additionally PRESENT at `:3c2a24e3…` with
`sha256:b51d9813…`). This independently corroborates the STG fence purpose claim “gate 24/24”. The 2026-11-18
gate-actor finding “GHCR 2/24” was time-local: images were completed later on 2026-11-18 by publish run
`44329726561` (22 dispatch-publish SUCCESS at 15:1x–15:5xZ).

### Resource fences

- STG fence **HELD**: owner `Codex-PR-Merge`, claim `b73c1c98-1b13-43f9-a5db-9b046f71de6f`, generation 14,
  resource revision 55; purpose = data-preserving STG rollout at package pin `d4af0992…` (product pin
  `62b6fca…`), 7 changed services (admin, auth, developer-portal, notification, realtime, user, web) →
  `:d4af0992` digests + 16 unchanged. Evidence verified as above (GHCR 24/24 + CI publish 22/22).
  Pending human gate: LAN run `44403401294` failed `RESOURCE_FENCE_UPGRADE_REQUIRED`; capture-manifest
  81 members digest `78f5532cfc06766e5482caf402a6a0d30c7d47255f1fc441a43149132859546b` lacks
  `nats-rollout-preservation/resource_claim.py`; root idle upgrade needs the pmd password on the voice-staging node.
- DEV fence **free** at resource revision 302, generation 80.

## Findings

### F1. `done` cards: deployment linkage not enumerable

EVIDENCE: `tt task events` returns `[]` on sampled `done` cards; `tt task show` has no deployments field.
DECISION: axis “done w/o deployment record” is **unverifiable via installed CLI**; skipped, not claimed clean.
REACHABLE: only via web UI or a future CLI verb. No action.

### F2. PR-lifecycle card evidence is stale against GitHub reality

EVIDENCE: `OPS-STAGING-INITIAL-RECONCILE-144.next_action` names PR #1167 — **closed unmerged, superseded**;
the real delivery route is merged `#1169 → #1170 → #1171` landing tip `3c2a24e3…`. `CR-20261102-06/07/10/15/16`
(`merging`) and `CR-20261102-08/09/11` (review) carry receipts of the `44bd3c76`/`d4af0992` era; tip moved twice
since. `DELIVER-BE085-PR915` references PR !915 — closed unmerged. `D110-PR1006-DELIVERY` in `review`.
DECISION: **report-only**; no status transitions and no comments on foreign-owned cards (repo practice:
prior audits recorded such deltas in audit docs, not on cards).
REACHABLE: owners should re-anchor receipts to tip `3c2a24e3…` on next stage advance.

### F3. `waiting_for_human` ownership sane

EVIDENCE: exactly 2 wfh cards, both with bound owner. `GI-T13` owner/assignee = `Codex-PR-Merge` — an agent
label sits in a human gate. DECISION: no action (the acceptance executor for GAME sprint gates is the merge
agent per fleet contract); flagged as vocabulary nuance. REACHABLE: none.

### F4. STG fence evidence holds; human gate is the only stop

EVIDENCE: fence purpose vs independent verification: catalog 23 images + renderer = 24/24 at `:d4af0992`
(manifest inspect today), publish run `44329726561` 22/22 dispatch-publish SUCCESS at tip, LAN deploy failure
is the documented `RESOURCE_FENCE_UPGRADE_REQUIRED` gate, not drift. DECISION: **no fence mutation**; the
remaining stop is the human root-idle upgrade run (password on voice-staging node). REACHABLE: owner `Codex-PR-Merge`.

### F5. Epoch coherence drift: A4

EVIDENCE: `docs/PLAN.md` keeps `A3–A7` Queued, but TT epoch `A4` already has 1 `in_progress` (`BE-019`,
assignee `astrie`), 8 review, 2 merging, 11 `stg_ready_for_deploy`. A1-only-active holds; GAME parallel
confirmed (45/81 done). DECISION: **report-only**; epoch queue decision belongs to the owner. REACHABLE: yes,
by owner after release-gate decision.

### F6. Branch-protection required-check lists under-declare the live CI contract

EVIDENCE: `.github/ci/branch-protection-checklist.md` Tier1 = 13 checks (`ci-gate`, `protobuf`, `compose-config`,
`flutter`, `web`, `golangci`, `backend-go`, `backend-go-integration-pr`, `backend-go-pkg`, `backend-auth`,
`developer-portal`, `admin`, `ci-skip-gate`), Tier2 = 9, Tier3 = 3. None of the Tier lists names jobs that
`gate-policy.md`/gate run requires: `current-diff-contract`, `ci-script-tests`, `full-validation-aggregation`,
`affected-check-plan`, `staging-images-dispatch-publish` (22-per-image matrix), `nats-hub-config-renderer-image`,
`backend-auth-image`. Publish run at tip enumerated 87 branch jobs. TT duplicate-check (`tt search` for
“branch protection”, “required check”, “checklist”) found no covering card.
DECISION: **one available card created**: `CI-BRANCH-PROTECTION-CHECKLIST-ALIGN-1`. REACHABLE: worker-owned.

### F7. Tip CI verdict bookkeeping (docs delta)

EVIDENCE: full run `44329501457` @tip FAILURE with 7 reds = documented debt family; `ci-script-tests` green at
tip; publish run `44329726561` SUCCESS with all validation jobs skipped; `ci-gate`/`current-diff-contract`/
`full-validation-aggregation` classes not re-run at tip. DECISION: dated receipt section added to
`docs/todo/ci.md`; existing OPEN rows at `@d4af0992` stay as history, not overwritten. REACHABLE: n/a (docs).

### F8. Audit-contract vocabulary drift (report-only)

EVIDENCE: contract paths `.github/BRANCH_PROTECTION.md` and `docs/CI.md` **do not exist**; authorities are
`.github/ci/branch-protection-checklist.md` + `docs/TESTING.md` + `docs/todo/ci.md`. `tt journal status` is
absent in the installed CLI (only batch/brief/claim/epoch/resource/session/task/search/graph/ready…).
Contract statuses `todo/doing/pr_open/stg_doing/stg_ready_for_accept` are not among live statuses
(`available/in_progress/waiting_for_human/review/merging/dev_*/stg_*/acceptance/done/blocked`).
DECISION: recorded as drift; no doc churn invented. REACHABLE: skill/doc owners.

### F9. `tt brief` bucket semantics

EVIDENCE: brief active 56 ≠ in_progress+wfh (52); blocked 99 ≠ 94; done 76 ≠ 77. DECISION: observation only,
not corrected (internal bucket logic is tracker-side). REACHABLE: n/a.

## Actions taken

1. TT: created **available** card `CI-BRANCH-PROTECTION-CHECKLIST-ALIGN-1` (F6; duplicate-checked first).
   No status transitions, no claims, no fence mutations, no comments on foreign cards, no deployments created.
   Tracker revision before this write: 23202.
2. `docs/audits/2026-11-19-tt-fleet-audit.md` — this document.
3. `docs/audits/README.md` — one index row.
4. `docs/todo/ci.md` — dated 2026-11-19 receipt section (tip actuals, publish-skipped caveat, checklist drift card link).
5. `docs/PLAN.md` — one dated audit note under the fleet WIP table (A4 drift flag; A1-active/GAME-parallel hold).
6. PR opened from branch `docs/tt-fleet-audit-20261119` (base `master`) — merge left to owner.

## Verdict

TT is **aligned for fleet management** on the fence/ownership axes (fence evidence independently corroborated,
wfh owners bound, no orphan `done` claims manufactured). Actionable drift found and handled in docs/TT:
CI required-check checklist under-declaration (→ one available card), tip CI verdict bookkeeping
(→ `docs/todo/ci.md` receipt), PLAN epoch-A4 WIP flag (→ dated note). Report-only: stale PR receipts on
cards, A4 epoch queue vs PLAN, agent-bound human gate (`GI-T13`), brief-bucket semantics, contract vocabulary
drift. Unverifiable: deployment linkage of `done` cards (no CLI enumeration).

## Deliberately not done

- No status transitions on any card; no claims/release of resource fences; no deployments or CI runs created.
- No comments on cards owned by other agents/sessions (evidence re-anchoring left to card owners; recorded here).
- No edits to `.github/ci/branch-protection-checklist.md` (required-check list rebuild belongs to the new card).
- No full epoch-card audit (16 epochs) — out of scope of a routine fleet inventory audit.

## Self-corrections during the audit

1. `tt journal status` (skill doc) is absent in the installed CLI — used per-status `task list --json` sums
   + `task search`; recorded as vocabulary drift (F8).
2. `.github/BRANCH_PROTECTION.md` / `docs/CI.md` (contract names) do not exist — authorities re-read from the
   real files above; no doc was invented at the phantom paths.
3. `tt task search --text` is not a valid verb (returned empty/usage error) — re-checked duplicates with the
   top-level `tt search <text>` before creating the card.
4. GHCR check: an early scripted loop reported `nats-hub-config-renderer` MANIFEST_UNKNOWN (quoting bug); the
   retried single `docker.exe manifest inspect --verbose` returned PRESENT digest `sha256:5489892bad61…` →
   final count **24/24**, not 23/24.

## Evidence pointers

- TT revision 23202 read snapshot; `task brief` totals; per-epoch `task list --json` histories (A1/A4/GAME).
- GHPR: `gh api repos/Poryadok/VoiceRoot/pulls?state=open` (0), PRs #1166–#1171; tip `3c2a24e3…`.
- CI runs: `44329501457` (full @tip, FAILURE), `44329726561` (publish @tip, SUCCESS/skipped-shape),
  `44426480170`, `44403401294` (LAN deploy gate FAILURE).
- GHCR receipts: `tt-audit/ghcr-result.txt` (23 digests) + `tt-audit/nats-renderer.json` (renderer manifest)
  under `C:\Users\Sergey\AppData\Local\Temp\opencode\tt-audit\`.
- Fences: STG claim `b73c1c98-1b13-43f9-a5db-9b046f71de6f` gen 14 rev 55; DEV free rev 302 gen 80.
- Authorities: `.github/ci/branch-protection-checklist.md`, `.github/ci/gate-policy.md`,
  `docs/TESTING.md` (tier table), `docs/todo/ci.md`, `scripts/ci/staging-image-catalog.json`.
- Created card: `CI-BRANCH-PROTECTION-CHECKLIST-ALIGN-1` (available, this audit's label in body).
