# CI gate audit 2026-11-19 — nightly full-master gate at head 3c2a24e3

- **Agent:** OpenChamber session (model qwen3.8-flash-next-coder-iq1_m, working dir
  `D:\Git\Voice`, branch `master`), audit nick **CI-Gate-Full-Audit-20261119**.
- **Scope:** honest audit of the full-master CI gate at master head without touching
  product code: `workflow_dispatch` of `.github/workflows/ci.yml` (workflow id 263230665)
  with `profile=auto` (run #6051), classification of red jobs against the documented
  debt classes in `docs/todo/ci.md`, staging-images push/publish status, and the GHCR
  gate (catalog 23 images from `scripts/ci/staging-image-catalog.json` +
  `nats-hub-config-renderer`) at the exact head tag.
- **Method:** `gh api` (`repositories/1168297071/actions/...` runs/jobs endpoints,
  per-job logs for the red outside the documented classes; one dispatch POST with
  `inputs[profile]=auto`), `git ls-remote origin` head re-check, GHCR existence via
  24 `docker manifest inspect --verbose` refs at lowercase
  `ghcr.io/poryadok/voiceroot/<name>:<head-sha>` (Docker 27.4, anonymous), repo reads
  of `docs/todo/ci.md`, `scripts/staging/verify-staging-images.sh`,
  `scripts/ci/staging-image-catalog.json`.

## Ground truth (verified in the audit window, UTC receipts)

- Audit head: `git ls-remote origin refs/heads/master` → `3c2a24e3112daecdc9e54b25b46d309ea35610c2`
  (merge of PR #1171), unchanged throughout the audit window (dispatch #6051, polls,
  GHCR pass).
- Own dispatch run: **CI #6051** id `44426480170`, event `workflow_dispatch`,
  created 2026-11-19T00:07:48Z — **VACUOUS** at this head: 66 jobs = 1 success
  (`changes`) + 65 skipped + 0 failure, conclusion success. `profile=auto` selected
  nothing to run; this run is non-authoritative by construction (no full-gate proof).
- Authoritative run at head: **push CI #6049** id `44329501457` (event `push`),
  created 2026-11-18T14:54:40Z: **133 jobs = 112 success / 10 skipped / 11 failure**,
  conclusion failure. Link: https://github.com/Poryadok/VoiceRoot/actions/runs/44329501457
- REDS (11, job ids): `local-ci-parity` 134889964550; `role-space-receipt-integration`
  134889964608; `backend-go (role)` 134889964682; `space-native-installer-integration`
  134889965058; `backend-go-integration-pr (space)` 134889965376; `compose-e2e`
  134889965411; `backend-go-integration (space)` 134889965753; **`backend-go-integration
  (gameintegration)` 134889965907**; `t31-critical / compose-https-claim-inbox-ack`
  134889969780; `ci-gate` 134911346264; `full-validation-aggregation` 134938204454.
- SKIPPED (10) in #6049 include `voice-history-audit`, `ci-skip-gate`,
  `staging-images-push`, `staging-images-promote`, `deploy-staging`; greens include
  `admin`, `backend-auth`, `a1-flutter-profile-handoff` (flakes flipped green on this push).
- Same-head dispatch **CI #6050** id `44329726561` (2026-11-18T14:55:52Z, 87 jobs):
  23 `staging-images-dispatch-publish` SUCCESS + `changes`, conclusion success — but it
  did **not** establish the `3c2a24e3` GHCR tag set (GHCR gate below); flagged as an
  unresolved publish-profile semantics gap, not counted as gate green.
- GHCR gate at head (24 refs = 23 catalog + renderer, `docker manifest inspect --verbose`):
  **PRESENT 2/24** — `web` `sha256:bfa2a5996a5192e26f7748a6d7c095586de52d7ecf1c14b5b02798ccfe12ddbf`,
  `nats-hub-config-renderer` `sha256:b51d9813732ef1127db388e1bfcc772582d43511ff464275fc62264bed6ccde8`;
  ABSENT 22 (all Go catalog images, `auth`, `admin`, `developer-portal`) → gate RED at head.
- In #6051 both `staging-images-push` and `staging-images-dispatch-publish` were skipped:
  the own dispatch published nothing (authorized ✓).
- Head moved after the audit window (PR #1172, #1173, branch-protection rebuild; tip
  `e5f0dba7` at commit time of this file): this audit's verdict is scoped to the
  `3c2a24e3` window; the later-head runs are out of its scope and belong to the next
  nightly pass.

## Findings

1. **10/11 reds of #6049 ∈ documented debt/propagator classes** in `docs/todo/ci.md`:
   Space/Role Go ban-catalog classes (`backend-go (role)`, `backend-go-integration (space)`,
   `backend-go-integration-pr (space)`, `role-space-receipt-integration`,
   `space-native-installer-integration`), `compose-e2e`, tier3 `local-ci-parity`,
   `t31-critical / compose-https-claim-inbox-ack` receiver-DB class, and the
   `ci-gate` / `full-validation-aggregation` propagators.
2. **1 NEW unregistered class:** `backend-go-integration (gameintegration)` (job
   134889965907) — no `gameintegration` authority-source class exists in the `docs/todo/ci.md`
   registry (class-section grep shows only Space/Role ban-authority classes).
   Root-cause receipt from the per-job log: failing step `go test -timeout 20m ./...`;
   `--- FAIL: TestGISAuthoritySourceActualMigratorProtectedFactory (90.46s)`;
   assert site `src/backend/pkg/integrationtest/authority_migrator.go:75` ←
   `src/backend/gameintegration/authority_source_server_test.go:28`
   (`Error: Should be true`, after `start and wait for pinned source migrator`;
   `http: TLS handshake error from 127.0.0.1:40272: EOF`); sibling gameintegration
   packages green (`httpapi` 94s, `registry` 322s). Root cause **unresolved** — local in-test
   TLS handshake EOF vs real regression: registered as a class with an owner decision to
   reproduce locally at a fresh head before touching product code.
3. **GHCR head-tag gate is RED (2/24):** the 23+1 exact-head-tag set has never been
   established at `3c2a24e3`; neither the own dispatch (skipped) nor #6050 (green publish
   jobs) changed that — semantics gap recorded honestly.
4. **Own `profile=auto` dispatch is vacuous:** an ACCEPTED verdict from it alone would be
   false evidence; the full-master verdict at head is taken only from push #6049.

## Actions taken

- 1 `workflow_dispatch` POST (run #6051, `ref=master`, `profile=auto`); GET `gh api` runs/jobs
  + per-job log fetch for job 134889965907; 24 `docker manifest inspect` calls; repo reads.
- Docs-only: authored `docs/audits/2026-11-19-ci-gate-audit.md` + README index row +
  registered the new class in `docs/todo/ci.md`; 1 TT card created
  (`CI-GAMEINTEGRATION-AUTHORITY-TLS-20261119`, status `available`, evidence = this file).
  No retries, no cancels, no product-code or cluster actions. This docs-only commit lands
  on the post-audit master tip `e5f0dba7` (master had advanced after the audit head).

## Verdict

- **NOT ACCEPTED — one new unregistered red class** (`backend-go-integration (gameintegration)`):
  audited with a log receipt, registered in `docs/todo/ci.md`, TT card opened. The other
  10 reds stay within documented debt — no further debt surprises at this head.
- The own auto-dispatch verdict is ACCEPTED-as-vacuous only: full-gate proof at head
  exists solely via authoritative push #6049.
- GHCR gate at head RED (2/24) — structural, expected for this head, not introduced by this audit.
- Zero unauthorized staging actions.

## Deliberately not done

- No product-code, cluster, or CI-config changes; no retry of any flake job; no cancel/rerun
  on #6049; no local history-audit re-runs (CI runners covered the scope).
- No second full-profile dispatch at this head: the authoritative push run #6049 at the
  identical head already provides the full-scope verdict.
- The docs-only commit pushed by this audit triggers a new CI push run at a newer head —
  not re-audited here; the next nightly covers it.
- GHCR registry API not used; existence evidence is `docker manifest inspect` per ref.

## Self-corrections

- An earlier in-session GHCR existence attempt (tags-page scrape with wrong URL casing)
  404'd all 24 packages and produced false ABSENTs; the corrected evidence is per-ref
  `docker manifest inspect` at lowercase `ghcr.io/poryadok/voiceroot/…` → 2/24 as quoted.
- `jq`/PowerShell helpers over saved gh payload pages failed on quoting and produced no
  numbers (not wrong numbers); the #6049 totals (133 = 112/10/11) and the 11 RED job ids
  were re-derived from the raw payload pages with a node parser before being recorded.
