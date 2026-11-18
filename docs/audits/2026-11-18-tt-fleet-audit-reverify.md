# Fleet re-verification 2026-11-18 — read-only pass over head / GHCR gate / STG cluster / claim / CI

- **Agent:** OpenChamber session (model qwen3.8-flash-next-coder, working dir
  `D:\Git\Voice`, branch `master`), audit label **TT-Fleet-Reverify-20261118**.
- **Scope:** honest fresh re-verification of five stated fleet claims, nothing else:
  (1) `master` head vs product pin `62b6fca83b606c4f458f190d20a7e8e02610168c` and STG
  head `d4af0992db2a50fac8eb94758e1020ad315f8be9`; (2) GHCR gate at `:d4af0992…` over
  the 23 catalog images in `scripts/ci/staging-image-catalog.json` + `nats-hub-config-renderer`
  (24 targets); (3) live `voice-staging` kubectl state (expect 33 deployments / 51 containers,
  zero drift); (4) STAGING resource record (held claim `ccba8020…`, gen 12, rev 41, owner
  Codex-PR-Merge, pending dispatch text) and card `OPS-STAGING-INITIAL-RECONCILE-144` =
  `waiting_for_human` with A/B/C decision; (5) CI runs at head: `staging-images-push` /
  `staging-images-promote` conclusions. **No mutation of anything.**
- **Method:** `gh api` GETs only (`commits/master`, compare `62b6fca…master`, `actions/runs`,
  `actions/runs/<id>/jobs` pages 1–2); `docker manifest inspect` ×24 (full-SHA tags, same
  tag convention as `.github/workflows/ci.yml` `tags: …:${{ github.sha }}`);
  `kubectl get deployments -n voice-staging -o json` with `KUBECONFIG=C:\Users\Sergey\.kube\config`;
  `tt` CLI reads (`resource list`, `resource show STAGING`, `task show OPS-STAGING-INITIAL-RECONCILE-144`)
  at `--url http://192.168.0.109:30990 --project voice`. Local worktree not used as reference.

## Ground truth (verified this pass)

- `gh api repos/Poryadok/VoiceRoot/commits/master`: **head = `d4af0992db2a50fac8eb94758e1020ad315f8be9`**
  ("Merge pull request #1166 … UIV3 Chat Info"). **STG head claim == master head ✓.**
- `compare/62b6fca8…master`: `status=ahead`, **15 commits, 16 files, zero `src/backend`**
  (files: workflow `dev-candidate-publish.yml`, docs ×6, `scripts/dev/*` ×4,
  `src/frontend/lib/l10n/*` ×5, `src/frontend/lib/ui/chat/chat_info_panel.dart`).
  → `:d4af` backend image content == pinned product content (pin `62b6fca8` is an ancestor).
- **GHCR gate at `:d4af0992db2a50fac8eb94758e1020ad315f8be9`** (24 `docker manifest inspect`):
  **PRESENT = 2** (`ghcr.io/poryadok/voiceroot/web`, `…/nats-hub-config-renderer`, OCI index
  manifests, exit 0), **MISSING = 22** (manifest-unknown), **AUTHFAIL = 0**. Matches recorded
  2/24 (`stgate.json` era) — no drift in the gate since the prior pass. Receipt: `%TEMP%\ghcr-gate.txt`.
- **kubectl `voice-staging`: 33 deployments / 51 containers**; all 33 carry
  `Available=True` conditions, `readyReplicas>=replicas`, no `unavailableReplicas`;
  live container image map equals the STAGING resource `observed.images` (r41) on **23/23 app-scope
  keys, drift = 0 vs recorded observation**; 44/51 refs immutable `@sha256`, 7 infra tag-specs.
  Receipt: `%TEMP%\stg-deployments.json`.
- **TT STAGING resource** (`resource show STAGING`): `state=held`, **claim `ccba8020-0e7c-42f7-b042-9a8ce1dffb43`**,
  **generation 12**, **revision 41**, **owner Codex-PR-Merge**, `phase=deploy`,
  acquired 2026-11-17T19:54:26.9Z / checkpoint 19:54:29.5Z, **`stale=true`**;
  `next_action="gh workflow run Staging deploy; poll run to completion; kubectl read-back;
  deployment create; per-card STG recheck; release PASS"`;
  `pending="Staging deploy workflow dispatch (app-only) at image_tag d4af0992… with changed_services
  admin,auth,developer-portal,notification,realtime,user,web under this exact claim; STG deployment
  record STG-PKG-D4AF0992-20261117-V1 pending after read-back"` ✓. Card list includes
  OPS-STAGING-INITIAL-RECONCILE-144 (+94 more).
- **Card OPS-STAGING-INITIAL-RECONCILE-144**: `status=waiting_for_human`, checkpoint
  2026-11-17T22:04:48.6Z, `next_action` = A/B/C: **(A) manual GHCR publication (6 pinned-content
  images + promote-retag 16 from live digests) — autonomous default recorded, awaiting user veto**;
  (B) debt-fix re-green CI push/promote; (C) accept live content, close without rollout. Assignee/owner
  Рудеус-144; events e12/e13 carry the honest evidence (2/24 gate, 22 MANIFEST_UNKNOWN, chainHits=[],
  16-file landing, dash-form claim vs slash-form fence = unused for mutation). Resource wait
  `6d687b5a-…` (actor Рудеус-144) remains `ready=false`.
- **CI at head** (`actions/runs` + jobs, 132 jobs/run): two CI runs at `d4af0992` —
  push **#44153952853** (2026-11-17T19:00:42Z) and manual **dispatch #44154583011** (19:04:14Z),
  both `completed/failure`. **`staging-images-push` = skipped in BOTH runs; `staging-images-promote`
  = skipped in BOTH runs** (debt-red prerequisites). `nats-hub-config-renderer-image` = **success**
  both runs. Red jobs (union): `backend-go (role)`, `backend-go-integration (space)`,
  `backend-go-integration-pr (space)`, `backend-go-integration-evidence`→skipped,
  `space-native-installer-integration`, `role-space-receipt-integration`, `compose-e2e`,
  `local-ci-parity`, `voice-history-audit`, `a1-flutter-profile-handoff`, `admin`,
  `t31-critical / compose-https-claim-inbox-ack`, `full-validation-aggregation`, `ci-gate`.
  **No Staging deploy run at head** (pending dispatch has not fired; latest 50 runs window).

## Findings (deltas vs stated claims)

**Zero deltas.** All five claims re-verified against fresh evidence and match exactly:
head==STG target ✓, pin ancestry + 16 files/no-src/backend ✓, gate 2/24 ✓, cluster 33/51 drift-0 ✓,
claim ccba8020/gen12/rev41/owner/pending ✓, card `waiting_for_human` + A/B/C ✓,
push/promote skipped ✓.

Caveats (report-only):
1. STAGING claim is **`stale=true`**: held idle since 2026-11-17T19:54Z; the pending dispatch was
   never executed (no Staging deploy run at head, gate still 2/24).
2. There are **two** CI runs at head (push + a workflow_dispatch at 19:04Z), both red — conclusions
   identical for the two staging jobs in both runs, so the claim holds for either run.
3. Run enumeration covered the latest 50 runs (3 at head found: CI×2 + Docs link check success);
   older at-head runs were not exhaustively enumerated.
4. `web` is PRESENT at `:d4af` although `staging-images-push` skipped — publish of web at head came
   from a non-skipped publish job outside that job; recorded as observed, not explained further.
5. GHCR gate evidence is `docker manifest inspect` only; raw registry-API 404 receipts from the prior
   pass (`stgate.json`) were not re-produced here.

## Actions taken

**None that mutate.** This pass is read-only. Receipt counts (reads): 10 `gh api` GETs (commits,
compare ×2, runs, jobs ×4 pages, resource/branch probes), 24 `docker manifest inspect`, 1 `kubectl get`,
3 `tt` read commands (`resource list`, `resource show`, `task show`). Failed shell attempts were
PowerShell/jq quoting retries only (no 4xx from network, no TT write attempt ever made).
TT graph revision observed ~23154 (read), no writes.

## Verdict

Fleet records are **consistent with reality** as of this pass: master head = STG rollout target
`d4af0992`; product content at `:d4af` equals pin `62b6fca` for backend (docs/l10n/UI diff only);
GHCR staging gate genuinely 2/24; live cluster healthy (33/51, zero drift vs recorded map);
STAGING claim `ccba8020` held-but-stale with its pending app-only dispatch at `:d4af` never fired;
card 144 honestly `waiting_for_human` on the A/B/C decision (autonomous default A recorded, awaiting
user veto). Blocking state is genuine and unchanged — nothing drifted, nothing was fixed or fabricated.

## Deliberately not done

- No GHCR publication, no Staging deploy dispatch, no claim release/re-acquire, no deployment record,
  no status/ownership transitions on any card (per instruction "Do NOT mutate anything").
- No debt-red fixes, no CI re-run dispatch.
- No raw GHCR API receipts beyond docker CLI.
- Local worktree (stale vs origin) untouched.

## Evidence pointers

- Temp receipts: `%TEMP%\stg-deployments.json` (33 items), `%TEMP%\tt-staging.json` (resource show),
  `%TEMP%\gh-runs.json`, `%TEMP%\jobs-dispatch{,2}.json`, `%TEMP%\jobs-push{,2}.json` (132 jobs each),
  `%TEMP%\ghcr-gate.txt` (24 lines PRESENT/MISSING), inspector `inspect_stg3..6.js`.
- TT anchors: STAGING r41/g12 claim `ccba8020-0e7c-42f7-b042-9a8ce1dffb43`; card 144 events e12–e14;
  CI runs #44153952853 (push), #44154583011 (dispatch).
