# CI gate audit 2026-11-18 — nightly full-master dispatch on head d4af0992

- **Agent:** OpenChamber session (model qwen3.8-flash-next-coder-iq1_m, working dir
  `D:\Git\Voice`, branch `master`), audit nick **CI-Gate-Full-Audit-20261118**.
- **Scope:** honest audit of the full-master CI gate without touching product code:
  dispatch `.github/workflows/ci.yml` (workflow id 263230665) at master head with
  `profile=full`, poll to completion, classify reds against the documented debt
  family (`backend-go (role)`, Space integration jobs, `compose-e2e`,
  `voice-history-audit`, `local-ci-parity`) plus repo docs registries; evaluate
  `staging-images-push` outcome and the GHCR gate (23 catalog images from
  `scripts/ci/staging-image-catalog.json` + `nats-hub-config-renderer`) at head.
- **Method:** `gh api` (runs/jobs endpoints, per-job step conclusions, per-job
  logs for the two reds outside the family), `git ls-remote origin` head re-check,
  public GHCR tags-page scrape (`ghcr.io/Poryadok/voiceroot/<name>?tab=tags`,
  exact 40-char sha-ref presence; token lacks `read:packages` so registry API was
  unavailable), repo reads of `docs/todo/ci.md`, `.github/ci/branch-protection-checklist.md`,
  `scripts/staging/verify-staging-images.sh`, `scripts/ci/render-stack-lock.sh`.
  `fence7.json` was searched (repo tree, `search/code` for "fence7"/"fence", worktrees)
  and **does not exist in the repo**: evaluation used the enumerated family above
  + `docs/todo/ci.md` red-class registry + branch-protection checklist.

## Ground truth (verified 2026-11-18)

- Dispatch run: **CI #6034**, id `44205730031`, event `workflow_dispatch`,
  head `d4af0992db2a50fac8eb94758e1020ad315f8be9`, created 2026-11-18T00:11:13Z,
  completed 2026-11-18T01:18:26Z (~67 min), overall **conclusion=failure**.
  Link: https://github.com/Poryadok/VoiceRoot/actions/runs/44205730031
- Head re-verified after the pass: `git ls-remote origin refs/heads/master` →
  `d4af0992…` (unchanged since dispatch; audit ran on the true tip).
- `profile=full` confirmed semantically (API run object does not echo `inputs`):
  full-scope jobs ran (`compose-e2e`, `t31-critical / compose-https-claim-inbox-ack`,
  `full-validation-aggregation`, `local-ci-parity`, `flutter-windows`); selective-only
  jobs (`affected-check-plan`, `current-diff-contract`) skipped by design on dispatch.
- Jobs totals (re-derived twice from the jobs API): **132 jobs = 110 success +
  11 skipped + 11 failure**.
- REDS (11): `admin`, `voice-history-audit`, `role-space-receipt-integration`,
  `space-native-installer-integration`, `compose-e2e`, `backend-go (role)`,
  `a1-flutter-profile-handoff`, `local-ci-parity`, `backend-go-integration-pr (space)`,
  `backend-go-integration (space)`, `full-validation-aggregation`.
- SKIPPED (11): `affected-check-plan`, `current-diff-contract`, `ci-skip-gate`,
  `minio-server-image-publish`, `minio-mc-image-publish`, `staging-images-push`,
  `backend-go-integration-evidence`, `ci-gate`, `staging-stack-lock`,
  `staging-images-promote`, `deploy-staging`.
- GHCR gate at head (public tags-page scrape, exact sha-ref string per package):
  **PRESENT 2/24 — `web` and `nats-hub-config-renderer`**; ABSENT 22: all 19 Go
  catalog images, `auth`, `admin`, `developer-portal` (+ `chat`, `role`, …). Older
  head-tag lineage elsewhere sits at `ecfd63ca…` (~2026-11-13).
- `staging-images-push` was **skipped** on this dispatch run by design: its `if`
  requires `github.event_name == 'push'`; dispatch never publishes GHCR images.
  `staging-images-promote` / `staging-stack-lock` / `deploy-staging` skipped:
  require green aggregation; staging deploy correctly NOT authorized by this run.
- Structural notes verified by repo reads: `render-stack-lock.sh` never queries
  GHCR (stack lock is aspirational, not an existence proof); the real existence
  gate is `scripts/staging/verify-staging-images.sh` (catalog 23 + renderer at the
  exact tag), which would be red at head.

## Findings

1. **8/11 reds ∈ enumerated documented debt family** (no surprises):
   `backend-go (role)`, `backend-go-integration (space)`, `backend-go-integration-pr
   (space)`, `role-space-receipt-integration`, `space-native-installer-integration`,
   `compose-e2e`, `voice-history-audit`, `local-ci-parity`. `local-ci-parity` is
   tier3 per `docs/todo/ci.md` (full-scope failure is not a formal PR failure).
2. **`full-validation-aggregation` red = documented formal class** — the aggregator
   itself rejects the red required jobs; promote/deploy chain correctly blocked.
3. **Two reds OUTSIDE the enumerated family** (logs retrieved, classified):
   - `admin` (job 134471136476): failing step `npm ci, lint, test`; log tail shows
     live-browser infra flake — `chrome_startup state=port_missing … SIGKILL, stderr
     "DBus error"`, `AssertionError: Chrome did not publish a DevTools port`.
     Infra flakiness of the hosted live-e2e harness, not a product regression.
   - `a1-flutter-profile-handoff` (job 134471137762): failing step
     `T055/T106/T107 Flutter A1 live proof`; `A1 Flutter Compose startup failed with
     status 1`, `make: compose-a1-flutter-profile-handoff Error 1`. This is the
     **open A1 acceptance debt documented in `docs/todo/ci.md`** ("A1 remains open")
     — documented in repo docs, but absent from the enumerated fence7-style family.
4. **GHCR head gate is red**: only `web` + `nats-hub-config-renderer` carry the
   `d4af0992` tag; the full 23+1 head-tag set was never established at head. This
   dispatch neither published (push job skipped) nor was authorized to publish.
5. **`fence7.json` gap**: no such file (or any "fence" match) exists in the repo;
   family membership was evaluated against the enumerated list + docs registries,
   so `admin`/`a1` membership in a hypothetical fence7 list is unverifiable.

## Actions taken

- None against product code, clusters, or tracker. Read-only pass: `gh` GET calls,
  one `workflow_dispatch` POST (run #6034), artifact/log fetches, GHCR page fetches,
  local file reads. No commits existed from the audit itself until this file was
  authored (this pass authored only `docs/audits/2026-11-18-ci-full-gate-audit.md`
  + README index row).

## Verdict

**NOT cleanly ACCEPTED — "effectively consistent with debt" with two caveats:**
1. No new/undocumented product regressions: 8 reds in enumerated debt family + 1
   documented formal aggregator class; the 2 outside-family reds are a Chrome live-e2e
   infra flake (`admin`) and the documented-open A1 debt (`a1-flutter-profile-handoff`).
2. Strict fence7 verdict impossible: `fence7.json` not found; if A1 + admin-flake
   classes are meant to be inside the fence7 family, the gate reads ACCEPTED with
   the caveat above.
3. GHCR gate at head is RED (2/24) — expected structurally for dispatch, but it means
   the 23+1 exact-head-tag set has never been green at `d4af0992`.
4. Zero unauthorized staging actions (`deploy-staging` skipped, correct).

## Deliberately not done

- No product code, cluster, docs-registry, or tracker changes (report mandate; the
  only repo change is this audit file itself).
- No local history audits re-run (CI dispatch covered the full scope on CI runners).
- Did not flip/cancel anything on run #6034; did not retry `admin` flake (a rerun
  could flip it green but would not change the head gate).
- GHCR registry API unused (token lacks `read:packages`); public tags pages used as
  existence evidence with the caveat that tag lists are scraped HTML.

## Self-corrections

- An earlier in-session GHCR snapshot (full 24-package pass over lowercase
  `user/…` URLs) 404'd everything and produced false ABSENTs; the corrected scrape
  (`ghcr.io/Poryadok/voiceroot/<name>?tab=tags`, exact sha-ref substring) yields the
  2/24 numbers quoted here. The `web` head-tag PRESENT result was reproduced on both
  URL casings.
