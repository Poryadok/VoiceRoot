# LAN root ceremony — reviewed `VERIFIED_GATED_TARGET_BASELINE` successor route mapped 2026-11-21

- **Agent:** OpenChamber session (model qwen3.8-flash-next-coder-iq1_m, working dir
  `D:\Git\Voice`, branch `master`), audit nick **LAN-Reviewed-Baseline-Successor-20261121**,
  write-actor label Codex-Baseline-Successor (TT card `OPS-STG-REVIEWED-BASELINE-SUCCESSOR-V1`).
- **Purpose:** the owner decision of 2026-11-21 ("yes, do the new baseline") replaced rerunning the
  frozen `--install --upgrade-idle-installed-cohort-v1` command (BANNED, root cause = authority-family
  drift 23 vs 24 per `docs/audits/2026-11-20-lan-retirement-authority-drift.md`). This audit maps the
  reviewed alternative — establishing a fresh `VERIFIED_GATED_TARGET_BASELINE` from current-master
  source — to concrete repo/GH/node mechanics, and states exactly what unblocks it and when.
  Read-only pass: zero mutation locally or on the LAN node; the assistant never runs `sudo` there.
- Facts are facts (exact quote / measured value / git or API anchor); one item is labeled **Inference**
  and names its basis.

## Scope and method

- Read-only trace of `scripts/staging/nats-rollout-preservation/*` and `.github/workflows/ci.yml` at
  `origin/master` tip `a4a43e841e8f2c0d06dfa3f4a696e66ef75b21c0e3` (the cited code blobs are
  byte-identical to tip `7d7808a773364c06dfa3f4a696e66ef75b21c0e3`: the intervening PRs
  `!1187`/`!1188` changed only `docs/PLAN.md`, `docs/audits/2026-11-21-tt-fleet-audit.md`,
  `docs/audits/README.md`, `docs/todo/ci.md`); live `gh api` reads of Actions runs/jobs/artifacts;
  node state as uid 1000 (`pmd`) only, per prior receipts (`c1`–`c15` on
  `OPS-STG-LAN-INSTALLED-COHORT-ROOT-V1`); TT comments c1–c2 on the successor card.
- Method: locate the ceremony entry (there is none in argv), identify the repo primitives the owner
  composes, decide whether the verify data model accepts a fresh 24-image authority as-is, then test
  whether any capture can bind today and name the single repo-side precondition that is missing.

## Ground truth (verified facts with anchors)

1. **No ceremony argv exists.** `installer.py:572` usage lists exactly
   `ROOT_PRIVATE_POLICY_JSON | --upgrade-idle-* | --upgrade-v*` — no baseline/capture route;
   `--upgrade-idle-installed-cohort-v1` dispatches at `installer.py:575`. `root-launch.template.sh:38-42`
   routes only `--install`/`--retire`/`--continue-retirement`. The reviewed baseline ceremony is an
   **owner node-run** composing repo primitives, not a repo command.
2. **Capture primitive (repo-side).** `source_authority.py:551` `capture_source(...)`
   (phases `_capture_source_at_master:475-548`): master-ref pin `_head:120-125`; CI binding `_ci:145-198`
   requires a `push`-event run of `.github/workflows/ci.yml` with `head_branch == 'master'` and
   `head_sha == source_sha` (`_ci:147-152`), exactly one completed-success `ci-gate` job and one
   `staging-stack-lock` job (`_ci:157-162`); source tarball of `source_sha` `:496-503`; the run's
   `staging-stack-lock` artifact fetched, exactly one non-expired member zip bound to that run/job
   (`:504-527`, retention 14 days per `ci.yml:1833` + `expires_at` checks `:511-515`); per-name GHCR
   digest resolution `:529-531`.
3. **The verify model already accepts a fresh 24-name capture.** `operator_retirement.py:1360` admits
   `reference == 'voice-media-target-baseline-reference-v1'` (prefix `operator-media-target-*`,
   `MEDIA_TARGET_PREFIX:901`; receipt check `:1374` on basis `MEDIA_TARGET_BASIS:904` =
   `VERIFIED_GATED_TARGET_BASELINE`). `media_target_templates:1217-1267` is data-driven: per-image digest
   shape `:1222-1236`, per-container rebinding of the 22 deployment templates to
   `authority['images'][service]` `:1249-1263` (foreign-container fallback `:1260-1261`), count/sets check
   `:1265`; `voice-p3-principal-jwks` (nginx, `deploy/staging/services.yaml:1591-1627`) and
   `voice-p3-principal-public` ConfigMap (`:1571-1573`) are outside the 22 and not authority-bound.
   The failing Oct-era clause `:1228` `set(authority['images']) != set(source_authority.IMAGE_NAMES)` is
   **dynamic**: live `IMAGE_NAMES` = 24 (`source_authority.py:31-33`; catalog canon
   `source_authority.py:373` = 23 catalog entries + `nats-hub-config-renderer`,
   `scripts/ci/staging-image-catalog.json`).
4. **The Oct-15 LAN terminal is the monitor family**, not baseline: `MEDIA_MONITOR_REFERENCE:1099`,
   authority file `operator-media-target-monitor-source-authority.json` 704693 B mtime Oct-15
   (`docs/audits/2026-11-20-lan-retirement-authority-drift.md` findings 6–8). That 23-name record is
   what trips `:1228` today; a fresh 24-name authority written under the baseline prefix passes.
   The frozen sizes at `MEDIA_FAILED_APPLY:909-919` (30352 / 3352567 / 704136) are pins of the
   *failed-apply evidence* for `reconciled`/`gateway`/`monitor` references (`:1386-1397`) — the pure
   baseline branch `:1360` does not consult them.
5. **Cohort closure after the new baseline (no code change).** The node's
   `rollout-0e2be62a3326/checkpoint.json` must declare `state['operator_media_abort']['schema'] ==
   'voice-media-target-baseline-reference-v1'` (`:1360`) and content equal
   `media_abort_terminal(blocked, receipt)` (`:1379`); then `idle_state:1449` → `verify_terminal:263`
   → `verify_media_abort_terminal:1354-1446` reads the fresh `operator-media-target-{source-authority,
   compiled,normalized,prepared}.json` (`:1381-1384`), binds templates at `:1401` (passes `:1228` with
   the fresh authority), and the baseline ceremony releases the marker (`:1406-1410`: data minus
   `knownRolloutOperation`, phase active, bumped `resourceVersion`). The cohort pins then close
   unchanged: projection `d4c2c9f6…` (`installer.py:33`, check `:448`), launcher/bundle digests per
   `installed-cohort-transition.md:19-29` entry `:26` (launcher `c085c3e5…`/7185 B, bundle
   `51460632…`/38328320 B — verified byte-identical on the node), receipt
   `installed/upgrade-idle-installed-cohort-v1.json`. `knownBaselineOperation` present in the marker
   at audit time is a **rejection** (`:316-319` `bridge_upgrade_live_operation_present`), not a
   short-circuit; `idle_state` short-circuits only on `status in ('PASS','ROLLED_BACK')` (`:1449-1452`).
6. **No capture binds today (live gh verified 2026-11-21).** Tip CI runs `#6089` 44786511718
   (success, **0 artifacts**) and `#6090` 44850194289 (the nightly authoritative dispatch, **failure**,
   and `workflow_dispatch` — not push-event, fails `_ci:147-152` regardless); head `a4a43e84` likewise
   has no `staging-stack-lock` artifact. The `d4af0992`-era lock artifact 14364610605 (run 44153952853,
   `#6032`) exists but `capture_equivalent_master_source` (`_equivalent_master_source:452-472`) fails at
   the ci.yml blob equality clause `:469` for it: `ci.yml` was rewritten after `d4af0992`
   (`a213bec9…` at `d4af0992` vs `b5f30142…` at tip; rewrites at `1930e6fa…`, `365c1060…`, `370e58ab…` —
   the staging-publish PR chain), while `src/backend` tree, `backend-docker-context.txt` and
   `staging-image-catalog.json` blobs (`b5f30142…/be55bafe…/2d0ad49b…`) match tip. Artifact retention
   (14 days) self-expires that lock anyway.
7. **The single missing repo-side precondition.** `staging-stack-lock` job (`ci.yml:1790`) runs only when
   its gate holds: `github.event_name == 'push' && github.ref == 'refs/heads/master' &&
   needs.changes.outputs.code == 'true' && needs.changes.outputs.ci_only_change != 'true'`
   (`ci.yml:1801-1804`). `changes.code` filter = `.github/ci/path-filters.yml:529-538`
   (`**` minus `docs/**`, `README*`, `.cursor/**`, `.claude/**`, `.codegraph/**`, `*.md`,
   docs-link-check workflow, `.markdown-link-check.json`) ⇒ any merge touching `src/frontend/**`
   qualifies. When it fires, the job renders `stack.lock.yaml` (`ci.yml:1817-1832`,
   `VOICE_IMAGE_HEAD_TAG: ${{ github.sha }}` `:1822`) and the promote jobs
   (`staging-images-promote`/`staging-images-push`, `promote_from_sha` output `ci.yml:114`) publish/retag
   the 24 catalog images at the new head sha; then `capture_source` binds strictly at that sha, and
   later docs-only merges keep binding via `capture_equivalent_master_source` (ancestor at `:454-461` +
   byte-equal `src/backend` tree/`ci.yml`/docker-context/catalog at `:462-469`).

## Findings

1. **No new repo tooling is owed.** The verify/ceremony data model at tip already accepts the fresh
   24-name baseline capture (fact 3); the Oct-era blocker is a *stale captured record* (monitor-family
   23-name authority), not a frozen verifier that needs a successor patch. The prior card wording
   ("committed reconciled verifier structurally unsatisfiable") applies to the `reconciled` reference
   branch only; the baseline-reference branch is the documented reviewed route and is satisfiable.
2. **The repo-side unblock is already in flight:** three UIV3 Flutter client PRs
   (`UIV3-PANEL-CHAT-GROUPSETTINGS`, `UIV3-PANEL-SPACE-MEMBERS`, `UIV3-PANEL-STORIES-HIGHLIGHTEDIT`)
   touch `src/frontend/**` — merging any one of them advances master with `code == 'true'` and fires the
   `staging-stack-lock` job + promote at the new tip, producing the fresh push-event CI pack
   (`ci-gate` + `staging-stack-lock` job, single fresh artifact) that `capture_source` binds.
   *Inference (basis: facts 1–7 + path-filters/gate quotes):* these merges satisfy the precondition;
   actual stack-lock job success will be recorded as receipt on the cards when it lands.
3. **GHCR head tag stays `:d4af0992` until a qualifying push**; the 24/24 immutable gate at
   `:d4af0992` (publish run 44329726561) is the STG rollout gate and is unaffected — the new tip adds a
   second tag set for capture purposes only (promotion of unchanged digests, `promote_from_sha`).

## Actions taken

- Read-only research pass at tip (this audit); TT actualization: LAN card comment c15 (full map) +
  checkpoint; successor card `OPS-STG-REVIEWED-BASELINE-SUCCESSOR-V1` claimed by
  `Codex-Baseline-Successor` with c1 citation fix (the cited `docs/design/nats-rollout-verification.md`
  does **not** exist at tip; the successor contract is `scripts/staging/nats-rollout-preservation/README.md`
  sections at `:94-106` / `:132-143`) and c2 verdict; this audit doc + index row + `docs/PLAN.md` WIP
  refresh via fresh-worktree docs PR (base `a4a43e84`).
- Staged owner script pack (read-only artifacts, no execution): `insp2026.py` already node-resident at
  `/tmp/insp2026.py` (13022 B, sha256 `df845ebf570bf942b25148cfafd38ab114ccfb5a26adbe7fa6e327864abd8fba`);
  the owner command plan is recorded in this doc below.

## Owner euid-0 ceremony plan (user-run; assistant never runs sudo on the node)

Stage A — probes (unchanged from before): `sudo python3 /tmp/insp2026.py 2>&1 | tee /root/insp-20261121.log`;
root-only listings of `/var/lib/voice-nats-preservation/rollout-0e2be62a3326/` and `installed/`.

Stage B — after the first qualifying code-merge master push CI completes GREEN with a `staging-stack-lock`
job + artifact (receipt posted on both cards with the run id): bind the fresh authority with the repo
primitive `source_authority.capture_source(token, run_id, source_sha, destination)` (`:551`) against that
run, then run the baseline ceremony per `scripts/staging/nats-rollout-preservation/README.md:94-106`
(material: `deploy/staging/media-principal.md`, canonical rendered manifests, node-resident
`operator-media-0e2-tools` family under `/var/lib/voice-nats-preservation`) so it writes the fresh
`operator-media-target-*` set, refrences `rollout-0e2be62a3326/checkpoint.json` to the
`voice-media-target-baseline-reference-v1` terminal (equal to `media_abort_terminal(blocked, receipt)`,
`:1379`), and releases the marker (`:1406-1410` semantics).

Stage C — closure: `sudo bash /home/pmd/voice-nats-rollout-full-repair/rollout-root.sh --install
--upgrade-idle-installed-cohort-v1` (`root-launch.template.sh:39`; bundle/launcher digests
`51460632…`/`c085c3e5…` byte-verified unchanged on the node): the audit walk now passes on the baseline
branch (facts 3–5), expected marker line `NATS_ROLLOUT_BRIDGE=UPGRADED_IDLE_INSTALLED_COHORT_V1_HISTORY_PRESERVED`
and receipt `installed/upgrade-idle-installed-cohort-v1.json`; then the STG rollout gate for package
`:d4af0992` continues under fence claim `b73c1c98-1b13-43f9-a5db-9b046f71de6f`.

## Verdict

- **ACCEPTED route:** the reviewed `VERIFIED_GATED_TARGET_BASELINE` requires **no new repo tooling**; it
  requires (repo) one code-touching master merge — already in flight — and (node) the owner's euid-0
  stages B–C above. The frozen cohort command stays BANNED; the `rollout-code-b37935f6…` capture stays
  unadopted (`README.md:91-92`/`:120-124`, `installer.py:311` historical evidence never repaired/adopted).
- STG rollout `:d4af0992` and the 84-card acceptance stay HOLD until Stage C closes; fence claim
  `b73c1c98…` (gen 14) stays held with honest pending.

## Deliberately not done

- No sudo / no node mutation / no cluster read at root scope by the assistant; no rerun of the frozen
  cohort command; no adoption or repair of any historical record (monitor-era authority,
  `rollout-code-b37935f6` capture); no product code changes; no merge of the UIV3 PRs in this PR (they
  merge only via their own reviewed PR CI).

## Self-corrections

- The successor-card description cited `docs/design/nats-rollout-verification.md:132-143` — that path
  does not exist at tip (`git show` fatal, `git ls-tree docs/design` empty for nats); corrected in card
  comment c1 to the README sections. The description text itself stays; c1 governs.
