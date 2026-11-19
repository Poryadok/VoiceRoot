# Full duty audit 2026-11-19 — fence ceremony, CI cadence, LAN node, publication gate

- **Agent:** OpenChamber session (model qwen3.8-flash-next-coder-iq1_m, working dir `D:\Git\Voice`,
  branch `master`), audit nick **Full-Duty-Audit-20261119**, merge-agent label Codex-PR-Merge.
- **Scope:** the 12-hour cadence pass over the STAGING rollout ceremony: TT fence card state,
  TT deployment/task states, GitHub reality (runs/PRs/GHCR), LAN `voice-staging` node bundle
  (read-only as `pmd`), repo docs registries (`docs/todo/ci.md`, `docs/audits`). Cross-pass
  consumption of the parallel audit PR #1172.
- **Method:** `gh api` (runs/jobs/PRs; per-job rollups), `git ls-remote origin refs/heads/master`,
  `tt` CLI read-only (`resource list`, `task list --compact`, `task show`), GHCR public tags-page
  scrape (`ghcr.io/Poryadok/voiceroot/<name>?tab=tags`, exact 40-hex sha-ref substring),
  `ssh pmd@voice-staging` listing of `/var/lib/voice-nats-preservation/installed` (scp probe script),
  deploy-job log of run 44403401294 (#461). GH timestamps quoted as-is; LAN/TT server clocks run
  offset vs GH UTC at this hour — no cross-clock absolute-time comparisons were made, only
  same-clock ordering.

## Ground truth (verified 2026-11-19)

- `origin/master` at pass start: **3c2a24e3112daecdc9e54b25b46d309ea35610c2**. PR #1172
  (`docs/tt-fleet-audit-20261119`, head b106c6b6, CI #6053 SUCCESS) was merged **by this pass** with a
  merge commit → tip **5e48c61854668853904d97ce205d5bbd5aea1082** (re-verified by `git ls-remote`).
- Product pin `62b6fca83b606c4f458f190d20a7e8e02610168c`; package pin **d4af0992db2a50fac8eb94758e1020ad315f8be9**
  unchanged by any master movement during the pass (only docs merges landed).
- Publication (GHCR gate at `:d4af0992`): **24/24 PRESENT** (23 catalog images + `nats-hub-config-renderer`).
  Evidence chain: publish CI run **44329726561 (CI #6050**, created 2026-11-18T14:55:52Z) conclusion=success,
  22/22 `staging-images-dispatch-publish` jobs; immutable digest map `pubcheck1.json` (24 refs);
  this pass' tags-page scrape 23/24 with one fetch-miss at `realtime`, then explicit retry probe
  TAGFOUND on pages 1–3 (three consecutive fetches carrying the `d4af0992` string).
- CI cadence:
  - scheduled CI **run 44497648425 (CI #6052**, event=schedule, created 2026-11-19T09:34:23Z, head
    3c2a24e3): conclusion=failure, 86 jobs = 22 success + 5 failure + 59 skipped; the 5 reds
    (`voice-history-audit`, `local-ci-parity`, `a1-flutter-profile-handoff`,
    `backend-go-integration (space)`, `compose-e2e`) are all documented debt classes per
    `docs/todo/ci.md` (A1 open debt; tier3 parity; space/go debt family). No new red class.
  - no-op dispatch 44426480170 (CI #6051, 2026-11-19T00:07:48Z, branch master): 1 success + 65 skipped
    — not a full-scope run (recorded so the cadence claim is not overstated).
  - this pass dispatched **full-scope CI #6055** (2026-11-19T17:12:59Z, head **5e48c618** tip,
    `profile=full`, `publish_tag` empty so the publishable publish job stays inert): in flight at
    authoring time; verdict of that run is left to the next pass (no fabricated conclusion).
  - push-run CI #6054 at tip 5e48c618 SUCCESS (2 success + 64 skipped, docs-only delta).
- Fence `resource list` (fresh, TT server): **STAGING held, revision 55, generation 14, claim
  b73c1c98-1b13-43f9-a5db-9b046f71de6f, owner Codex-PR-Merge, phase deploy, operation 11**; pending
  text verbatim: "STG rollout blocked honestly at LAN runner bundle gate (run 44403401294 #461,
  RESOURCE_FENCE_UPGRADE_REQUIRED) … Human root idle upgrade on voice-staging node is the active
  external operation; no cluster mutation; fence claim and candidate preserved; STG data untouched."
  observed=known at commit d4af0992… with 23-entry images map; task package size **85**.
  DEV card: free, revision 302, generation 80 (untouched).
- TT task states (fresh): `stg_ready_for_deploy` total **86**, `stg_deploying` total **0**,
  `waiting_for_human` = GI-T13 (SDK identity gate, owner Рудеус-13-class human label) and
  **OPS-STAGING-INITIAL-RECONCILE-144** (owner Рудеус-144). New available card from the parallel
  audit: **CI-BRANCH-PROTECTION-CHECKLIST-ALIGN-1** (branch-protection required-check tiers vs live
  CI job contract), consumed as cross-reference, not duplicated.
- Package membership diff resolved with evidence: live 86 `stg_ready_for_deploy` vs fence package 85
  = live extras **BE-089** and **DEV-RELEASE-POLICY-REBIND-62B6FCA** (both `dev_deployment`-bound,
  outside this STG package, still `stg_ready_for_deploy`) minus package extra
  **OPS-STAGING-INITIAL-RECONCILE-144** (`waiting_for_human`, human-owned — acceptance is human duty).
- LAN node (`ssh voice-staging` as `pmd`, root dir `/var/lib/voice-nats-preservation`, listing via
  scp probe): installed dir holds the reviewed idle-upgrade receipts family (`upgrade-idle-*.json`)
  but **no `upgrade-idle-installed-cohort-v1.json`**; no launcher/rc file visible to `pmd`; journal
  and inbox entries require sudo. → the reviewed idle transition
  `rollout-root.sh --install --upgrade-idle-installed-cohort-v1` (predecessor = pinned installed
  cohort binding 78f5532c…, source tree 9aa2e4d17beec62c5a853437e48b65239538a856, verified green
  against observed binary pins in the prior pass) has **not been executed** at this hour.
- STG deployment ceremony: deployment `STG-PKG-D4AF0992-20261119-V1` exists on the TT server (recorded
  under the held claim, commit d4af0992… pinned); staging-deploy workflow 263689731 run inventory
  (this pass, per_page scan to 2026-11-18T10:01Z) contains **only #461 = run 44403401294**
  (created 2026-11-18T21:37:32Z, conclusion=failure) and no later run. Run #461 deploy-job log
  (`stgjob-135152125808.log`): root cause `NATS_ROLLOUT=RESOURCE_FENCE_UPGRADE_REQUIRED` at the
  capture-manifest gate, ~5 s after job start, before any journal/TT write — zero cluster mutation.

## Findings

1. **LAN fence-upgrade blocker still active and honestly recorded** in fence pending (operation
   11): node lacks the reviewed `upgrade-idle-installed-cohort-v1` transition (no receipt), which
   needs euid=0 and `pmd` sudo requires a password → human LAN root operation is the active
   external owner. Durable unit per the audits consumer contract: **OPS-STG-LAN-INSTALLED-COHORT-ROOT-V1**
   created by this pass, claimed and moved to `waiting_for_human` with reason (euid=0 requirement,
   password-only sudo — the agent cannot provide the human action). Human-gated cards GI-T13/OPS-144
   unchanged and not mine.
2. **CI cadence**: only the scheduled run #6052 covered the tip at that hour (22 jobs, debt-family
   reds only); a true full-scope master run was missing since 2026-11-18 (#6034), so this pass
   dispatched CI #6055 full at the merged tip; its conclusion belongs to the next pass. No new
   red class anywhere; `admin` (hosted Chrome live-e2e) did not re-fire today — existing card
   **ADMIN-E2E-FLAKE** (epoch A1, status `review`) already owns that infra-flake class.
3. **Package membership diff 86-vs-85** resolved (see Ground truth) — report-only; BE-089/
   DEV-RELEASE-POLICY-REBIND-62B6FCA stay outside the STG package and need their own follow-up
   cycle; OPS-144 acceptance stays human-gated. No card (resolution cited, no gap).
4. **Ceremony-state observation**: deployment recorded at phase `started` while no rollout CI run
   exists — the deployment-start record intentionally precedes the rollout dispatch, which is
   gated on the LAN root op; honest, no cluster mutation happened between the two records.
5. **GHCR `realtime` tags-page fetch flake** in this pass' first scrape (23/24) — transient
   scrape-side miss; retry probe reproduced the tag 3× (see Self-corrections). No product impact.
6. **Docs registry gap**: no repo doc describes the TT `stg_deploying` status transition
   semantics (grep over `docs/**` finds no reference). Consumer-contract card
   **DOC-STG-DEPLOYING-STATUS-SEMANTICS** created (`chore`, `available`): document the exact
   semantics from real receipts after the acceptance cycle completes; never fabricate behavior.
7. **Parallel audit PR #1172 consumed**: its new card CI-BRANCH-PROTECTION-CHECKLIST-ALIGN-1 is
   available for whoever picks up branch-protection alignment; not claimed by this pass.

## Actions taken

- Merged PR #1172 (parallel session's green docs audit PR, CI #6053 success) with a merge commit,
  admin merge powers, precedent chain #1165–#1171; master tip 3c2a24e3 → 5e48c618.
- Dispatched full-scope CI #6055 at tip 5e48c618 (`profile=full`, empty publish inputs — the
  publication stays pinned at `:d4af0992`, no re-tag).
- Authored this audit file + `docs/audits/README.md` index row (delivered as this docs PR).
- Created TT cards per the audits consumer contract: **OPS-STG-LAN-INSTALLED-COHORT-ROOT-V1**
  (claim by Codex-PR-Merge then `task status waiting_for_human` with reason — euid=0 requirement,
  password-only sudo) and **DOC-STG-DEPLOYING-STATUS-SEMANTICS** (`available` chore, unclaimed).
- Read-only: fence `resource list`, `task list`/`show`, deployment get, runs/jobs API pages,
  GHCR scrape + retry probe, LAN ssh listing probe, deploy-job log read of #461.
- Scratch worktrees from earlier publication fixes (`wt-pub-ci`, `wt-pub-repo`,
  `wt-pub-token`, `wt-pub-docs1`) were removed in the prior cleanup step; nothing else to clean
  (Docker Desktop holds no `voice`/`voiceroot` local tags).

## Verdict

After this pass: publication at `:d4af0992` is green (24/24 + publish run success, digests
immutable); CI debt reds remain classified against `docs/todo/ci.md` with a fresh full-scope run
in flight at the docs-merged tip; fence STAGING is held at rev 55 / operation 11 with the LAN
root-step blocker recorded verbatim in pending; the STG deployment is recorded (`started`) ahead
of the rollout dispatch; LAN node bundle unchanged (cohort upgrade pending, human-owned); TT
package/acceptance cycle is untouched and stays gated behind a green rollout. Zero unauthorized
cluster or package mutations.

## Deliberately not done

- Did not run the LAN root upgrade (human duty, passwordless-sudo absent); did not touch the node.
- Did not dispatch the rollout again before the node upgrade (same manifest-gate red expected).
- Did not release the STAGING fence claim (operation 11 incomplete) — release comes with
  `--safety safe --outcome PASS` only after a completed rollout + acceptance cycle.
- Did not create per-card STG acceptance records (rollout not green), did not touch BE-089 /
  DEV-RELEASE-POLICY-REBIND-62B6FCA (follow-up cycle), OPS-STAGING-INITIAL-RECONCILE-144
  (human-gated), CI-BRANCH-PROTECTION-CHECKLIST-ALIGN-1 (unclaimed, other duty).
- CI #6055 conclusion intentionally uncited (in flight; next pass records it).
- GHCR registry API still unavailable (token lacks `read:packages`); tags-page scrape carries the
  documented scraping caveat (page-1 pagination was explicitly probed to pages 1–3 for `realtime`).

## Self-corrections

- An earlier in-session statement claimed a rollout run "#464" was dispatched. The staging-deploy
  workflow inventory in this pass contains **no run after #461**: that dispatch never landed; the
  only rollout-attempt record is the failed #461 (id 44403401294), and the fence pending cites
  exactly that. Corrected to evidence.
- This pass' first GHCR scrape reported `realtime` ABSENT at `:d4af0992`; the retry probe found the
  tag string on all three tags pages; final gate is 24/24. The digest map was never in question.
