# LAN root ceremony — retirement authority-family drift root cause 2026-11-20

- **Agent:** OpenChamber session (model qwen3.8-flash-next-coder-iq1_m, working dir
  `D:\Git\Voice`, branch `master`), audit nick **LAN-Retirement-Authority-Drift-20261120**,
  write-actor label Codex-PR-Merge.
- **Purpose:** resolve why the user's LAN root ceremony
  `sudo bash /home/pmd/voice-nats-rollout-full-repair/rollout-root.sh --install --upgrade-idle-installed-cohort-v1`
  fails closed `controller.Blocked: operator_retirement_binding_invalid`, and record the
  reviewed direction. Read-only pass: no mutation locally or on LAN, no repair or adoption
  of any historical record.
- Facts are facts (exact quote / measured value / git anchor); the one labeled
  **Inference** names its basis.

## Scope and method

- Read-only trace of `scripts/staging/nats-rollout-preservation/*` at
  `origin/master` tip `fd97cec747ff5756eb9331685e5349d76a2a2948`; LAN node `voice-staging`
  over ssh as uid 1000 (`pmd`); TT comments c1–c10 on `OPS-STG-LAN-INSTALLED-COHORT-ROOT-V1`;
  era git facts via `git log -S gameintegration` and `git log -1` per file; GHCR tags-page
  re-scrape.
- Method: locate the only emitter of the error string, trace the failing validation chain on
  the `--install` route, diff LAN bundle members against repo `master`, compare the capture-era
  image family with the live `source_authority.py IMAGE_NAMES`.

## Ground truth (verified facts with anchors)

1. **Emitter (unique).** The string `operator_retirement_binding_invalid` occurs exactly once
   in the repo: `scripts/staging/nats-rollout-preservation/operator_retirement.py:41`
   `def fail(): raise Blocked('operator_retirement_binding_invalid')`. The `controller.Blocked`
   prefix comes from `from controller import Blocked,READ,file_sha` at `operator_retirement.py:15`.
2. **On the `--install --upgrade-idle-installed-cohort-v1` route the continuation document is
   never read.** Chain: `installer.py:575` route entry → cohort pins pass
   (`INSTALLED_COHORT_TARGET_PROJECTION d4c2c9f6…` equals the LAN bundle manifest projection)
   → `_upgrade_idle_transition` (`installer.py:310`) → full-history audit loop
   `installer.py:321-327` → `idle_state:1449` → `verify_terminal:263` → `verify_media_abort_terminal`
   → `:1401 templates = media_target_templates(…)` → fail condition
   `operator_retirement.py:1228`: `set(authority['images']) != set(source_authority.IMAGE_NAMES)`
   → `fail()` at `:41`.
3. **LAN continuation is foreign evidence, not the emitter.**
   `/home/pmd/voice-nats-rollout-full-repair/operator-retirement-continuation.json` (432 B,
   mtime Oct 11 03:08, sha256 `e43c95c16c75f66dd681b640057a9febb4b41b4d656881d81f2088a4306d24b3`)
   belongs to a different operation family `3340764a7d24` (`--continue-retirement`): its
   `failed_checkpoint_sha256` equals that family's pin `FAILED_RETIREMENT_SHA 5a0e2c2437f3…`,
   `original_plan_sha256 145bef3e…` equals `/home/pmd/voice-nats-rollout-idle/operator-retirement-plan.json`,
   `code_binding_sha256 c4e49ff3…` is Oct-11-era code. It validates nothing on this route.
4. **LAN bundle executes with the live master source.** Launcher `rollout-root.sh` 7185 B
   sha256 `c085c3e53deec4c1f945492bc60a744811642a63130e5437646089ce251ef0fe`; bundle 38328320 B
   sha256 `5146063251ff4efc1f650eba55ec93408242a529650942e35e3824a1e02f1ea5` equals `EXPECTED_SHA`
   inside the launcher. Members `installer.py cdb92825…`, `operator_retirement.py c6996f02…`,
   `source_authority.py c0897039…` are byte-identical to repo `master` → the ceremony validates
   against the current 24-name `IMAGE_NAMES`. LAN `root_cli.py e6f24fa6…` ≠ master `96d72572…`
   and `bridge.py bd37127f…` ≠ `82aa8feb…` (Nov-1 master lag), not causal for this fail.
5. **Era git facts.** `gameintegration` entered `source_authority.py IMAGE_NAMES` at exactly one
   commit: `1f6e775148e7fa7b446028644ca19b35e9ca05c0` «test: update GIS rollout catalog fixtures»
   2026-10-17 02:17:12 +0300; at `0abd98ac…` (2026-10-16 08:02) `IMAGE_NAMES` held 23 names.
   `operator_retirement.py` unchanged on master since `014bfa01702e` 2026-10-16 08:32:40 +0300
   → the `:1228` set-equality is live today.
6. **LAN media-abort family intact.** In root-only `/var/lib/voice-nats-preservation`,
   `rollout-0e2be62a3326` holds its whole media-target family with sizes equal to pins
   (`-reconciled-origin-runner.py` 30352, `-reconciled-blocked.json` 3352567,
   `-blocked.json` 3342669, `-compiled.json` 1076345); monitor `-source-authority.json`
   704693 B, mtime Oct 15 22:46. Capture era of that authority: Oct 15.
7. **No cohort receipt.** `installed/` has no `upgrade-idle-installed-cohort-v1.json`; last
   successful receipt is Oct 16 08:47 `upgrade-idle-combined-provisional-v1.json`. The failed
   Nov 20 13:42 attempt left capture `rollout-code-b37935f681254576b298c00c509e191c` (mode 0700).
8. **Cluster intact after the failed ceremony.** Zero-mutation re-proof: 258 configmap/manifest
   entries, marker uid `cbafb90c…`, drift-0 data, generation `r20260930a4`, 33 deployments
   all `avail=1`.
9. **GHCR current truth (re-scraped tags pages):** the head-tag gate at
   `:d4af0992db2a50fac8eb94758e1020ad315f8be9` is **24/24** immutable refs (Path B CI-owned
   `staging-images-dispatch-publish`, publish run 44329726561, PRs #1170/#1171); `d4af0992`
   present on admin/gameintegration/user/web/chat/notification tags pages. The 2/24 quotes in
   `2026-11-18-*` audits are historical era statements, not current truth.

**Inference (named basis):** the captured Oct-15 media-target authority for `0e2be62a3326`
carries the 23-name image family. The record itself is unreadable to `pmd` (root-only tree).
Basis: the record self-validates at capture time only against the code of that time, the
Oct-16 idle ceremonies were green, and the Nov-20 attempt fails at exactly the `:1228` set
equality — the only input that changed between those eras is the live `IMAGE_NAMES`.
Optional upgrade to a direct fact: the user's euid-0 read of the staged read-only inspector
`sudo python3 /tmp/insp2026.py 2>&1 | tee /root/insp-20261121.log` on the node
(inspector staged at `/tmp/insp2026.py`, 13022 B, sha256 `df845ebf…` verified; mutation-free).

## Findings

1. **Root cause: authority-family era drift.** The captured media-target authority for media-abort
   operation `0e2be62a3326` (Oct 15 era, 23 image names) vs live `source_authority.IMAGE_NAMES`
   at master tip (24 names; `gameintegration` added 2026-10-17). `set(23) != set(24)` at
   `operator_retirement.py:1228` → deterministic `Blocked('operator_retirement_binding_invalid')`
   at `:41`. Not an infra flake, not a custody/secret problem.
2. **Consequence: deterministic fail-closed for any idle route today.** The first ceremony
   attempted after 2026-10-17 must fail — the Nov-20 attempt is that first one. The same drift
   blocks `--install`, `--retire` and `--continue-retirement` idle routes alike.
3. **Foreign-evidence correction.** The Oct-11 continuation on the node belongs to operation
   `3340764a7d24`, foreign to this route and era (see Self-corrections).
4. **Docs-era guard.** Old audits quoting GHCR 2/24 at head must not be read as current
   regression; current gate is 24/24 (finding 9).

## Actions taken

- TT: comment `c11` on `OPS-STG-LAN-INSTALLED-COHORT-ROOT-V1` (root cause, foreign-continuation
  correction, do-not-rerun warning, reviewed direction); new card
  `OPS-STG-MEDIA-AUTHORITY-FAMILY-DRIFT-23-24-V1` (status `available`, epoch `A1`) created per
  the audits consumer contract with `evidence = docs/audits/2026-11-20-lan-retirement-authority-drift.md`,
  related to the LAN ceremony card; `task pr` on the drift card with this docs PR.
- docs-only PR: this file + one index row in `docs/audits/README.md` + pointer paragraph in the
  NATS rollout section of `docs/DEPLOYMENT.md`.
- No code PR owed: the fail is deterministic and already documented policy
  (`installer.py:311` "historical evidence, never repaired or adopted"; README `:91-92`
  frozen runner must not rerun; README `:120-124` never reuse expired authority).

## Verdict

The LAN installed-cohort root ceremony is fail-closed with an understood, evidence-anchored
root cause (authority-family era drift). No route documented in this repository can make the
Oct-15-era media-target authority pass against the current 24-name live source; the STG rollout
for package `:d4af0992` stays behind this decision, and the STAGING fence claim
`b73c1c98-1b13-43f9-a5db-9b046f71de6f` remains held. Unblock requires an explicit product
decision: a new reviewed baseline authority from a current-master capture
(README `:94-96` `VERIFIED_GATED_TARGET_BASELINE`), never adoption or reuse of expired
authority (README `:120-124`).

## Deliberately not done

- **No rerun of the failing command.** A rerun cannot pass and only creates another fail-closed
  0700 capture (Nov-20 capture `rollout-code-b37935f6…` already exists; not adoptable).
- No repair, rotation or adoption of historical records: `0e2be62a3326` media-target family,
  the Oct-11 continuation, `/home/pmd/voice-nats-rollout-idle/*`, the Nov-20 capture — forbidden
  by documented policy (`installer.py:311`, README `:91-92`).
- Did not run the inspector myself (pmd cannot read the root-only authority); the user's
  euid-0 probe would upgrade one labeled inference to a fact, without changing the decision.
- Did not start a new `VERIFIED_GATED_TARGET_BASELINE` ceremony — explicit product/user decision
  required; it changes reviewed baseline authority for the LAN preservation custody.

## Self-corrections

- Pre-ceremony planning (and the prior session statement) treated the stale Oct-11
  continuation's mismatched pins as the prime suspect of the ceremony failure. Corrected:
  the `--install` route never reads the continuation; the emitter is the media-target authority
  set equality at `operator_retirement.py:1228`. The foreign-continuation observation stays true
  but is not causal.
- Earlier docs quoted GHCR 2/24 at head (`2026-11-18-tt-fleet-audit-reverify.md`,
  `2026-11-18-ci-full-gate-audit.md`): era truth; the current 24/24 gate at `:d4af0992`
  is re-verified in this pass (finding 9).
