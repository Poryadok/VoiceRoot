# TT Fleet Inventory Audit 2026-11-26 — Routine inventory pass; TT backend unreachable (fresh receipts, all TT axes BLOCKED); CI job-name drift NET ZERO (step-level `ci.yml` nsi edits landed via !1216/!1217); STG digest gate HOLD `:d4af0992` (fence candidate re-verified `24/24` byte-exact vs recorded authority)

- Date: `2026-11-26`
- TT CLI: `C:\Users\Sergey\AppData\Local\TTenrec\bin\tt.exe --url http://192.168.0.109:30990 --project voice`
- Scope: routine fleet inventory audit authored in a fresh `origin/master` worktree `tmp/tt-fleet-inventory-audit-20261126`, branch `codex/tt-fleet-inventory-audit-20261126`, base tip `ab309265a65d93a4ae60d08b91c03f0f3ff24c96` (PR #1217 merge landed 2026-11-25). Docs-only pass: zero product/CI code, zero cluster/fence/candidate/GHCR mutations, zero retries/re-runs.
- Previous: [`2026-11-25-tt-fleet-inventory-audit-v2.md`](./2026-11-25-tt-fleet-inventory-audit-v2.md) (PR #1214), [`2026-11-25-tt-fleet-inventory-audit.md`](./2026-11-25-tt-fleet-inventory-audit.md) (PR #1212), [`2026-11-23-openchamber-fleet-inventory-audit.md`](./2026-11-23-openchamber-fleet-inventory-audit.md), [`2026-11-22-tt-fleet-inventory-audit.md`](./2026-11-22-tt-fleet-inventory-audit.md). The 2026-11-24 inventory-audit file authored on branch `codex/tt-fleet-inventory-audit-20261124` (open PR #1208) is still pending on master and stays unlinked per README canon.

## Verdict — 2026-11-26 inventory pass

- **TT backend UNREACHABLE (fresh, third consecutive day):** every TT graph route (`task show`, `projects list`, per-status `task list --status`, `resource list/show`) fails with `connection_error` at `192.168.0.109:30990` — `connect ETCPREFILED` / `context deadline exceeded` even with `--timeout=90s`/`--timeout=120s` (no backoff), and raw LAN contact (`/api/health`, `/`, `/api/v1/health`) returns `http_code=000` (no TCP answer at all). Nothing on the local host listens on any `3099x` port. **Consequence:** the required per-status census (`available`, `todo`, `doing`, `pr_open`, `dev_test`, `dev_ready`, `dev_verify`, `stg_ready_for_deploy`, `stg_doing`, `stg_ready_for_accept`, `waiting_for_human`, `done`, `archived`, `blocked`) is **BLOCKED at all 14 statuses with 0 successful calls**; the TT-side audit axes (done↔deployment records, `pr_open`↔PR mapping, `waiting_for_human` owner-reasons, resource-fence claims/reconcile placeholders, epoch A1/A4/GAME coherence, `journal status` — client verb still absent in the installed CLI surface) are recorded BLOCKED with receipts. **Zero tracker writes possible; zero cards created.**
- **CI contract drift (required-check-name lists): NET ZERO at tip.** Live `.github/workflows/ci.yml` top-level job keys: `66` at tip `ab309265` and `66` at the checklist-rebuild anchor `bc9e4909e092f1be63a45c4d81e20b3bf5b9a902` — `only_tip=[]`, `only_anchor=[]` (zero adds/removals/renames). `verify-required-jobs.sh` blob `7ff776e682947181ba150a3360d712ade2e7c02e` **unchanged** at both anchors; its `34` conditional required job names are all ⊆ the live 66-job graph. The checklist (rebuilt at `bc9e4909`, PR #1199) still enumerates the live graph (its tier tables cover the 66 jobs + non-job tokens). `.github/**` deltas `bc9e4909..HEAD` are **step-level only** inside one existing job + the checklist rebuild itself: `ci.yml` `+70` lines = `space-native-installer-integration` job `timeout-minutes: 30 → 60` (`ci.yml:3209`) and new proof steps `Run Space legacy24 populated installer proofs` (`ci.yml:3510`), `Run Space legacy25 populated installer proof` (`:3522`), `Run Space native51 failed-capture retry pair proof` (`:3571`), `Enable Space catalog-call accounting` (`:3579`) + required-inventory JSONs; landed via PRs **!1216** (merge `c666d365…`, then `291b118a…`) and **!1217** (merge `ab309265…`). No job id renamed/added/removed → **no required-check-name drift**, and `docs/CI.md`/`.github/BRANCH_PROTECTION.md` naming drift stays report-only per the F9 precedent (those files do not exist; live authorities = `.github/ci/branch-protection-checklist.md` + `.github/ci/gate-policy.md`).
- **Branch protection (fresh API read, report-only):** `GET /branches/master/protection` → `required_status_checks.strict=false`, `contexts=["ci-gate"]` (app_id `15368`) — checklist Tier-1 table requires `4` names (`ci-gate`, `full-validation-aggregation`, `owned-enrollment`, `local-ci-parity`). Enforcement-vs-checklist drift carried (owner-side checklist decision, never audit-side).
- **PR reality (fresh):** exactly one open PR — **#1208** (`codex/tt-fleet-inventory-audit-20261124`, head `05eb6318`, base `6f652d06`, 3 files `+78`): `state OPEN`, `mergeable CONFLICTING`, `mergeStateStatus DIRTY` — the README/PLAN anchor-clash class already registered by the 11-25 passes (its audit content superseded by the landed 11-25 v1/v2 + this pass). Landing/refresh/closure stays that PR's owner decision; report-only, no card.
- **PR checks at #1217 head (fresh `gh pr checks 1217`):** `ci-gate SUCCESS`, `space-native-installer-integration SUCCESS` (the new nsi steps green), `backend-auth-image SUCCESS` (Maven-429 registry entry retires on green per its own policy — no re-run performed by me), sole FAILURE `owned-enrollment` = registered infra class (same class shape as PR #1208). Tip master push `ab309265` has a fresh **CI dispatch in progress** at audit time; the authoritative `133`-job full-graph verdict for the tip lands at the next inventory pass (rule: full graph, not the ~66 path-filtered subset).
- **STG rollout gate — HOLD `:d4af0992db2a50fac8eb94758e1020ad315f8be9` (GHCR digest cross-check fresh).** Catalog `scripts/ci/staging-image-catalog.json` = `23` staging images + `nats-hub-config-renderer` = `24` refs; catalog/registry maps carry **no digest fields by design** (GHCR catalog payload verified), so digest authority = GHCR tag manifests (Docker CLI `27.4.0` fresh) cross-checked byte-for-byte against the digest maps recorded in landed audits (`docs/audits/2026-11-23-*`, `2026-11-22-*`, `2026-11-25-v2`). Fresh presence sweep (24-ref target): **`:d4af0992` = `24/24`, every digest byte-equal vs the recorded candidate authority** — `chat b4ade2edf43d…`, `notification bf40d51e1377…`, `realtime f4ea05d42dec…`, `user e1e7b9fbdafd…`, `auth 5a12dc03230f…`, `web 3256a8171381…`, `developer-portal d64e732ec9b0…`, `admin 44fa715b09ad…`, `nats-hub-config-renderer 5489892bad61…` (full 24 table recorded 2026-11-23). Recent product pins are tag-sparse (GHCR retention/republish class, report-only): tip `:ab309265` `2/24` (`web 03225559c514…`, renderer `1fdc35bce902…`), `:291b118a` `2/24`, `:b92eb0c3`/`:c666d365` `0/24`, `:bfefc167` `23/24` (`chat c9ce68213233…` present, `auth` absent — `auth` latest pin `:291b118a`), `:eb341dd8` `23/24` (carried), `:e4b1a567`/`:026f10b8`/`:6f65b3b8`/`:33f70227` `0/24`. Negative sanity probe `chat@sha256:0000…0000` → `not resolves` (no tag aliasing). `staging-images-dispatch-publish` did not fire at the tip (docs-only delta, publish inputs empty) — **zero GHCR mutations by this pass**. Verdict: **gate authority verified intact, gate stays HOLD `:d4af0992`**; tag absence at recent pins is inventory observation (monitor-only), never a defect claim, never a re-anchor.
- **Fleet state (carried honestly while TT is down; from landed 11-22/11-23/11-25 records, not fabricated):** `done = 77` / eventless `76` (cleanup card `OPS-DONE-CARDS-EVENTLESS-RECEIPTS-20261123` open); `waiting_for_human = 3` = carried trio with recorded reasons (`OPS-GATEWAY-INTEGRATION-T13`→`GI-267`, `OPS-STAGING-INITIAL-RECONCILE-144`→fence `b73c1c98`, `OPS-STG-LAN-INSTALLED-COHORT-ROOT-V1`→`d5279735`); fences STAGING HELD `gen 14 / rev 57` claim `b73c1c98-1b13-43f9-a5db-9b046f71de6f` `stale:true` (2026-11-21 LAN fail-close `operator_retirement_binding_invalid`, zero cluster mutation), DEV `free` (last `6b500f31…` released, outcome PASS, `DEV-CHFIX-62B6FCA-20261121-V14`); epoch coherence carried (v10→v16 chain complete, v17 unopened, A4 title reconciled 11-21, GAME `T13→GI-267`) — **fresh registry-vs-epoch reconciliation needs a healthy TT graph = BLOCKED axis**.
- **Docs drift:** `docs/audits/README.md` index vs landed audit files: only the known pending 2026-11-24 file on the open PR #1208 branch (report-only class); this pass adds one index row; no dangling rows otherwise.
- **Actionable gaps: ZERO new cards.** Every finding is carried/registered (PR #1208 conflict + `owned-enrollment` red = registered classes; `backend-auth-image` 429 = registered, retired on green; auth-pin gap + tip tag sparsity = GHCR retention class (new registry row in `docs/todo/ci.md`, monitor-only); branch-protection-vs-checklist = F9 report-only; stale STAGING fence = existing owner cards; done-pool cleanup card exists). During the outage tracker writes are impossible, so the honest action is the verbatim outage-card payload below (**not created**).

## EVIDENCE — fresh receipts, outage receipts, and carried cross-checks

### TT outage receipts (fresh, 2026-11-26)

```text
$ tt.exe --url http://192.168.0.109:30990 --project voice task show OPS-STG-REVIEWED-BASELINE-SUCCESSOR-V1 --brief
{"error":{"code":"connection_error","message":"transport: context deadline exceeded"}}   # --timeout default 15s

$ tt.exe --url http://192.168.0.109:30990 --project voice --timeout=90s projects list
{"error":{"code":"connection_error","message":"transport: connect ETCPREFILED 192.168.0.109:30990: The connection was refused"}}

$ tt.exe --url http://192.168.0.109:30990 --project voice --timeout=120s projects list
... transport: Error: connect ETCPREFILED connectex: A connection attempt failed ... [192.168.0.109:30990]

$ tt.exe --url http://192.168.0.109:30990 --project voice --timeout=60 projects list
error: invalid timeout        # client-surface: --timeout needs a unit suffix (`60s`), carried report-only CLI drift

$ curl.exe -sS -o nul -w 'http_code=%{http_code} time=%{time_total}s' --max-time 12 http://192.168.0.109:30990/api/health
http_code=000 time=12.015968s
$ curl.exe ... http://192.168.0.109:30990/            -> http_code=000 time=12.015251s
$ curl.exe ... http://192.168.0.109:30990/api/v1/health -> http_code=000 time=12.016726s

$ netstat -ano | Select-String LISTENING | Select-String ':3099'   # whole-list scan -> no match
```

- Per-status enumeration: **BLOCKED** for all statuses (`available`, `todo`, `doing`, `pr_open`, `dev_test`, `dev_ready`, `dev_verify`, `stg_ready_for_deploy`, `stg_doing`, `stg_ready_for_accept`, `waiting_for_human`, `done`, `archived`, `blocked`), `0` successful `task list --status` calls; unfiltered `task list` is not an audit axis (errors on contract per `task.md`).
- Owed-but-uncreatable card `OPS-TT-BACKEND-UNREACHABLE-20261126` (`available`, P1, infra-tracker-outage): title/description = TT backend at `http://192.168.0.109:30990` (project `voice`) unreachable from this host since 2026-11-24 across three days (outage, not the 11-24 `internal_error` class); evidence = the receipts above verbatim; consumers = every TT-side audit axis; created only at first healthy contact (dedup by comment on the 11-25 card if the older card exists there).

### CI drift receipts (fresh at tip `ab309265`)

- `git ls-tree -r HEAD`/anchor job parse: `66` top-level job ids at both ends, `only_tip=[]`, `only_anchor=[]` (zero renames; step-level-only change).
- `git diff --name-only bc9e4909..HEAD -- .github docs/TESTING.md scripts/ci` = `17` files (`+540 / −42`): `.github/ci/branch-protection-checklist.md`, `.github/workflows/ci.yml`, `docs/TESTING.md`, `scripts/ci/{go_integration_exceptions.json, go_integration_receipts_test.py, go_integration_source_audit.py, local_ci_matrix_inventory.py, local_ci_routing.py, space_catalog51_calls_required.json, space_legacy24_proofs.sh, space_legacy24_proofs_required.json, space_legacy25_proof.sh, space_legacy25_required.json, space_native51_catalog_calls_proof.sh, space_native51_failed_capture_required.json, space_native51_retry_proof.sh, space_retry50_required.json}` — no job-id churn; new `scripts/ci/*.sh` are proof scripts consumed by the `space-native-installer-integration` steps, not new checks.
- `verify-required-jobs.sh` blob identical at both anchors (`7ff776e682947181ba150a3360d712ade2e7c02e`); `34` conditional job names (`require_job compose-e2e`, `current-diff-contract`, `check_if` set incl. flutter group + `backend-go-integration-pr`, `backend-auth`, `backend-auth-image`, `developer-portal`, `admin`) all ⊆ live graph.
- `gh api repos/Poryadok/VoiceRoot/branches/master/protection`: `strict=false`, `contexts=["ci-gate"]`.
- `gh pr checks 1217` states (above); `gh pr view 1208` CONFLICTING/DIRTY states (above); Actions full-jobs endpoint for run at `291b118a` returned transient `HTTP 502` twice — limitation recorded (PR-body check states + landed merge suffice for gate outcome).

### GHCR digest receipts (fresh, read-only)

- `docker.exe --version` = `27.4.0`; `docker manifest inspect --verbose` per package@pin → `Descriptor.digest`; GHCR catalog payload has no digest fields (presence only) — cross-check = presence sweep + byte-equality vs recorded audit maps.
- Pin sweep recorded fresh (values above in Verdict): `:d4af0992` `24/24` byte-exact vs 11-23 recorded table; `:bfefc167` `23/24`; `:eb341dd8` `23/24` (carried equality vs 11-22 map); tip/`291b118a` `2/24`; `b92eb0c3/c666d365/e4b1a567/026f10b8/6f65b3b8/33f70227` `0/24` (tag eviction).
- Content-level note (honest): cross-pin **layer-list** comparison not attempted — PowerShell 5.1 host lacks a usable base64 decoder (`[System.Text.Base64]` unavailable) and I did not fabricate equivalent "SAME" claims; digest equality (content-addressed) against recorded authorities is the sound cross-check used.
- Negative probe: `ghcr.io/poryadok/voiceroot/chat@sha256:0000…0000` → `not resolves`; sanity `admin@291b118a` digest unchanged (`25e8bb69…` era).

### Docs/PR/index receipts

- `docs/audits/README.md` index updated by this pass (one row); PLAN WIP gets one dated row; `docs/todo/ci.md` gets the GHCR tag-retention monitor row + the Maven-429 no-recurrence note.
- PR #1208 `gh pr view`: OPEN / CONFLICTING / DIRTY / base `6f652d06` / head `05eb6318` / 3 files `+78`.

## DECISION

- **Zero cards created, zero TT writes** (outage receipts above): all findings carried/registered or report-only; the single honest tracker action at contact = create `OPS-TT-BACKEND-UNREACHABLE-20261126` (P1) with the verbatim receipt payload above, deduped by comment against any surviving 11-25 outage card.
- **Docs-only:** audit file + one README index row + one PLAN WIP row + one `docs/todo/ci.md` registry row (GHCR tag-retention class monitor + Maven-429 green-retire note). No `.github/**`, product code, spec, config, or test change; no fence/candidate/cluster/GHCR mutation; no re-runs or retries (registered classes stay as recorded); no epoch touched (v17 unopened); no PLAN checklist/feature-status change.
- Gate authority untouched: fence `:d4af0992` stays HELD, candidate digest authority verified intact; the STG-LAN fence ceremony decision stays with `OPS-STG-LAN-INSTALLED-COHORT-ROOT-V1` + `OPS-STAGING-INITIAL-RECONCILE-144` (owner-side).
- Report-only owner decisions: PR #1208 refresh/land/close; branch-protection Tier-1 reconcile (`ci-gate`-only vs 4-name checklist); GHCR `auth`/tip-pin tag retention policy; stale STAGING fence re-anchor; tip full-graph CI verdict deferred to next pass while the tip dispatch is in flight.

## REACHABLE

1. **TT backend:** at the first LAN contact — re-run per-status census + fences + epochs + `pr_open`↔PR mapping fresh, create the outage card with the verbatim payload (or comment-dedup it), and reconcile `waiting_for_human` owners / epoch A1/A4/GAME coherence against the fresh registry.
2. **PR #1208:** owner-side refresh on the new base or closure (content superseded by landed 11-25/11-26 docs); CONFLICTING = registered anchor-clash class.
3. **STG rollout gate:** HOLD `:d4af0992` unchanged, authority re-verified `24/24` byte-exact this pass; retirement/re-anchor is owner-side on the two STG owner cards, never audit-side.
4. **GHCR retention:** watch whether `auth` retags at the next deploy push (latest `auth` pin observed `:291b118a`); monitor-only row lives in `docs/todo/ci.md`.
5. **Branch protection:** reconcile Tier-1 vs actual protection (owner-side checklist decision, F9 precedent), never enforced audit-side.
6. **Tip CI verdict:** authoritative `133`-job full-graph verdict for push `ab309265` completes at the next inventory pass once the in-flight dispatch finishes; per canon a full verdict reads the full profile graph.

## Self-corrections

- Earlier sessions' shorthand claimed `ci.yml` "byte-identical since `bc9e4909`"; this pass's fresh diff shows step-level additions landed via !1216/!1217 (`+70` lines inside `space-native-installer-integration`): job-name contract stays identical, but the file body does not — recorded honestly here (job-level drift remains zero, step-level delta documented).
- The `+214/−42` figure circulating in prior summaries does not match the fresh `bc9e4909..HEAD` diff (`+540/−42` over `17` files incl. `scripts/ci/**`); the fresh numbers above supersede it.

## Appendix — registry deltas landed this pass

- `docs/todo/ci.md`: GHCR `auth`-absence/tag-retention monitor row at `:eb341dd8`/`:bfefc167`/recent pins (fresh reconfirm) + Maven-429 entry green at #1217 checks (retire-on-green note).
- `docs/PLAN.md`: WIP row for 2026-11-26.
- `docs/audits/README.md`: index row for this audit.
