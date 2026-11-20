# LAN root rollout ceremony — handoff receipt 2026-11-20 (pre-ceremony verification)

- **Agent:** OpenChamber session (model qwen3.8-flash-next-coder-iq1_m, working dir
  `D:\Git\Voice`, branch `master`), audit nick **LAN-Ceremony-Handoff-Receipt-20261120**.
- **Purpose:** prove, immediately before the user's `euid=0` step, that the STAGING LAN rollout
  ceremony artifacts on the node are byte-coherent with the current `origin/master` tip, so the
  root route can run without rebuilding or re-deriving anything. Product truth stays in
  `scripts/staging/nats-rollout-preservation/*` and `docs/STAGING_SERVER.md`.

## Head state (verified)

- `origin/master` tip `6315070bac926dece67ac71159475b79cd871a08` (PR #1177 merge, 2026-11-20T04:20:45Z).
- Bundle was built from head `aab06d19098ebc0f381f991751205fa44b771092`; the head delta
  `aab06d19..6315070b` is docs-only (`git diff --name-only`: `docs/STAGING_SERVER.md`,
  `docs/audits/README.md`, `docs/audits/2026-11-20-dev-ch-oom-reobservation.md`,
  plus this PR's docs) — no cohort or authority bytes changed, so no rebuild is owed.

## Head-sourced and pinned inputs (re-pinned at the tip via `git cat-file blob`)

| Member | tip `6315070b…` sha256 / bytes | pinned tree `9aa2e4d1…` sha256 / bytes | verdict |
| --- | --- | --- | --- |
| `installer.py` | `cdb928250af50455bf6c6fb808ac12bf697740892fc15f514bc7b2b80759df67` / 41343 | `9b164c2a13fcad5e84ff42f89264807daa8cc987d08149d19b8c97cc1ad56b5c` / 34639 | bundle uses the **running** head installer; tip == recorded head pin → bundle input unchanged |
| `root-launch.template.sh` | `c6b42955de894e5597d36b92a92f68ba1852cb834b6f0bddd7c770327a6809d9` / 3198 | identical | launcher template has no pin→head drift |
| `bundle.py` | `31862bbb2a1f4147fddfc15634c62eb4abdc2bcf0f0f8b7b6e5e412231c8fa5b` / 5543 | `15e48752862cd06f8befd1373d50aea0b6f6617d1cd181c9865954cab0cafac5` / 4334 | documented pin→head drift class, tip == prev head → unchanged |

## Local build artifacts (custody dir `…\Temp\opencode\cohort-build-20261120-installer-head`)

- `rollout-bundle.tar` — sha256 `5146063251ff4efc1f650eba55ec93408242a529650942e35e3824a1e02f1ea5`, 38328320 B, 87 tar members.
- `rollout-root.sh` — sha256 `c085c3e53deec4c1f945492bc60a744811642a63130e5437646089ce251ef0fe`, 7185 B.

## Node state (read-only probe from `pmd` on `voice-staging` = 192.168.0.109)

- `/home/pmd/voice-nats-rollout-full-repair/rollout-bundle.tar` sha256 equals the local digest exactly
  (38328320 B), `tar -tf` member count 87.
- `/home/pmd/voice-nats-rollout-full-repair/rollout-root.sh` sha256 equals the local digest exactly
  (7185 B), mode `0700`, owner `pmd` (uid 1000).
- Directory mode `0700` `pmd pmd` (custody preserved, no group/other bits anywhere).
- Launcher SOURCE guard preconditions satisfied for the bundle file: uid 1000, `mode & 0022 == 0`
  (file mode `0600`), nlink 1, exact recorded size and sha256.
- Eligibility facts re-confirmed the same day (read-only): `installed/code` = 81 members, binding
  `78f5532c…` (`INSTALLED_COHORT_BINDING` predecessor), inbox `01730`,
  `installed/tt-resource-fence.json` ABSENT → `tt_resource_journal_snapshot` early-returns,
  so the reviewed transition `--install --upgrade-idle-installed-cohort-v1` is the correct route.

## Actions taken

- LAN cluster: **read-only only** (sha256, `tar -tf` count, `ls -la`); zero mutation of
  `voice-staging` deployments; the TT-held STAGING fence claim
  `b73c1c98-1b13-43f9-a5db-9b046f71de6f` (generation 14, window `35981b03-e774-4d68-8e13-3f0059eb8448`,
  revision 55, deploy phase, operation 11) untouched.
- GitHub: zero CI dispatches in this record, zero pushes to product paths; one docs-only PR
  (this file + the `docs/audits/README.md` index row).
- TT: zero card state changes; no `claim`/`test`/`accept`/`recheck`; no resource acquire/release.

## Verdict

The ceremony is **handoff-ready as built**: head-sourced inputs and pinned authority bytes at the
current tip hash to the same values the bundle was built from, and the node copies hash to the
same values as the local artifacts with custody modes intact. No bundle rebuild owed. The root
step remains the user's:

```
/home/pmd/voice-nats-rollout-full-repair/rollout-root.sh --install --upgrade-idle-installed-cohort-v1
```

under `euid=0` on `voice-staging`; expected bridge
`NATS_ROLLOUT_BRIDGE=UPGRADED_IDLE_INSTALLED_COHORT_V1_HISTORY_PRESERVED` plus receipt
`installed/upgrade-idle-installed-cohort-v1.json`; the inbox must stay `01730` (guard never widened);
no `policy.json` argument on this route.

## Deliberately not done

- The `euid=0` run itself (the user performs it; afterwards the job must change so root access is not needed).
- STAGING rollout re-dispatch, per-card STG acceptance, and `resource release STAGING --safety safe --outcome PASS`
  — all behind a green rollout plus `deployment create`.
- `OPS-STG-OWNER-CUSTODY-V1` — intentionally deferred until the rollout lands.
- Any product-code, workflow, inventory, or gate-script byte; any change to the live fence cluster.
