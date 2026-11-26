# TT fleet inventory audit v2 — 2026-11-26

## Verdict

TT outage — fourth consecutive day; fresh receipts this pass: `task list --status
available` (fresh `--instance` namespace, `--timeout=90s`) → `connection_error: context
deadline exceeded`, and `--timeout=90s projects list` → `connect tcp ... 192.168.0.109:30990:
Connection refused (os error 112: ConnectEx)` — census = `0` successful per-status calls,
all TT axes BLOCKED with receipts, zero tracker writes, zero new cards (v1 outage payload
stands quoted in the v1 doc, dedup rule). PR reality: exactly one open PR #1208
(`CONFLICTING`/`DIRTY`, docs-audit branch of the 2026-11-24 pass vs advanced master),
merged chain #1215–#1220, tip `7fcc22cc9df6`. CI job-name drift v1→v2 tip: NET ZERO —
window `f5db0517..7fcc22cc9` = 3 docs files `+51/−1`, ZERO files under `.github/`,
`scripts/ci/`, `src/`; blobs identical (`verify-required-jobs.sh 7ff776e682947181ba150a3360d712ade2e7c02e`,
`ci.yml 3afca89e445d4b6ff007835d1f7259d454114660`, checklist `60901ebf39e9639e67abb15fc3e81f75d6ed8fe9`,
gate-policy `f6e14bf5a4fd1f1dadea1f8b6c81d57572e62842`); registry paths `docs/CI.md`,
`.github/BRANCH_PROTECTION.md` still absent (report-only, F9); live branch protection
`contexts=["ci-gate"] strict:false` vs checklist Tier-1 four names (F7 class, unchanged).
Tip master-push CI run #6174 (`45740179154`) **completed**: `66 jobs = 2 success /
0 failure / 64 skipped` — zero reds at the tip; full-profile authority stays run #6170
(`45728902140`, head `0ca77e1e`): `133 = 118/11/4`, zero new debt classes. STG gate digest
cross-check at the retained pin `:d4af0992db2a50fac8eb94758e1020ad315f8be9`: `24/24` PRESENT,
`24/24` byte-equal vs the digest map recorded in `docs/audits/2026-11-22-tt-fleet-inventory-audit.md`
(consistent with the 11-19 audit and with v1's live receipts); vs the 2026-11-23 audit table at the
same pin `8 OK / 16 DIFF` — that 11-23 table is an outlier vs 11-19/11-22/live agreement
(self-correction candidate for the recorded-map column, report-only). Presence at the docs
tip tag `:7fcc22cc9d` = `0/24` (expected-absent, tag-retention class, monitor row already
registered in `docs/todo/ci.md`); negative probe rejected (`manifest unknown`). Gate HOLD
stays operational (stale STAGING fence + LAN fail-closed chain), not a digest mismatch.

## Evidence

Worktree `tmp/tt-fleet-inventory-audit-20261126-v2`, branch
`codex/tt-fleet-inventory-audit-20261126-v2`, tip commit `7fcc22cc9df6dcb3d5f3114db5dfe8e64871cff8`
= PR #1220 merge. All receipts are raw outputs of this pass unless marked carried (from landed
records, with file paths).

### TT census — 0 success receipts

```
$ tt --json --instance=voice-audit-20261126-v2 --timeout=90s task list --status available
tt.exe: HTTP request failed: connection_error: context deadline exceeded (exit code 1)

$ tt --timeout=90s projects list
tt.exe: HTTP request failed: connection_error: failed to connect: connect
  tcp: target 192.168.0.109:30990: connect: Connection refused
  (os error 112: ConnectEx: Unable to connect to remote host) (exit code 1)
```

Per-status census (workflow-2 statuses `available..acceptance`, `done`,
`waiting_for_human`, blocked, archived): `0` successful `task list` calls — every
enumeration BLOCKED by the outage above. Fenced/epoch/journal reads and all write verbs
not attempted (same transport). The installed CLI `1.5.17` (recorded v1) exposes no
`journal` verb; journal lines stay carried from v1. Outage-card payload
`OPS-TT-BACKEND-UNREACHABLE-20261126` (P1, infra) stays owed verbatim in the v1 doc
(`docs/audits/2026-11-26-tt-fleet-inventory-audit.md`); no duplicate card created here
(dedup rule); these two fresh receipts are the comment material for that card on first
healthy contact.

Carried fleet state (from landed records, quoted, not fresh): `done = 77` / eventless
`76` (`docs/audits/2026-11-25-tt-fleet-inventory-audit-v2.md` via v1); `waiting_for_human`
trio `GI-T13` → human OAuth consent decision, `OPS-STAGING-INITIAL-RECONCILE-144` → human,
`OPS-STG-LAN-INSTALLED-COHORT-ROOT-V1` → human — no TT-owner orphans in landed records;
DEV fence `b473c1655295…` free (gen 12, carried); STAGING HELD-claim `b73c1c982a81e900a6038e96a0319c5e632a7f6c`
gen 14 / rev 57 — stale, LAN fail-closed 2026-11-21 reason `operator_retirement_binding_invalid`
(carried, unchanged); epochs A1 latest `:284671f7` gen 32/rev 118, A4 latest `:1d04d3a2` gen 72/rev 261
(title reconciled 11-21), GAME latest `:a1563b67` gen 11/rev 32, v17 unopened (carried).

### PR reality — fresh receipts

```
$ gh pr list --state open --json number,title,headRefName,isDraft --limit 20
[{"headRefName":"codex/tt-fleet-inventory-audit-20261124","isDraft":false,
  "number":1208,"title":"docs: TT fleet inventory audit 2026-11-24 (zero-product inventory pass)"}]

$ gh pr view 1208 --json number,title,state,mergeable,headRefName,baseRefName,headRefOid,baseRefOid,updatedAt
{"state":"OPEN","mergeable":"CONFLICTING",
 "headRefName":"codex/tt-fleet-inventory-audit-20261124","headRefOid":"05eb6318fdb96e3cc14243dd8f727c93afe46b9a",
 "baseRefName":"master","baseRefOid":"6f652d0626e2e563b766a4992389651de9cdb6fb",
 "updatedAt":"2026-11-24T00:05:58Z","number":1208}
```

Exactly one open PR; #1208 is the 2026-11-24 audit pass's own docs PR — its head branch
(`tmp/tt-fleet-inventory-audit-20261124` @ `05eb6318`) now conflicts with master advanced
through #1215–#1220; landing/closing is an owner decision (report-only, registered class).

```
$ git log --oneline -14 refs/remotes/origin/master
7fcc22cc9df6 Merge pull request #1220 from Poryadok/docs/ci-gate-verdict-20261126
29c6dcea237f docs: CI gate 2026-11-26 audit skip-list enumeration + head-anchor correction (#1218, 11 skipped publication jobs)
67bd2bb1684e Merge pull request #1219 from Poryadok/docs/ci-gate-verdict-20261126
cda34835854f docs: CI gate 2026-11-26 audit anchor correction to PR #1218
d0e3fee7e0d9 docs: CI gate 2026-11-26 index row + audit at head 0ca77e1e - run 6170 133 jobs 118/11/4, ...
718814fb865f docs: authoritative CI gate verdict 2026-11-26 at head 0ca77e1e ...
0ca77e1e3170 Merge pull request #1218 from Poryadok/codex/tt-fleet-inventory-audit-20261126
f5db0517cf66 docs: TT fleet inventory audit 2026-11-26 (TT outage receipts, CI drift zero job-level, STG gate HOLD :d4af0992 verified 24/24)
ab309265a65d Merge pull request #1217 from Poryadok/codex/space-legacy24-gate-reorder-20261125
291b118a7454 ci: Space native-installer legacy24/25 steps moved before the account-ban capture guard
bfefc1679445 Merge pull request #1216 from Poryadok/codex/ci-integration-inventory-gaps-20261125
b92eb0c3516b Merge origin/master into codex/ci-integration-inventory-gaps-20261125
c666d365b409 ci: close Space native-installer inventory gaps with executed proofs (unresolved 14->3)
e4b1a567d93b Merge pull request #1215 from Poryadok/codex/tt-fleet-inventory-audit-20261125-v2
```

`done ↔ deployments`: no fresh done receipts (census BLOCKED); carried counts show no
unexplained undeployed done in landed records. `ops-verification ↔ PR checks`: covered by
the CI run receipts below.

### CI contract — drift windows and blob identities (live, this pass)

```
$ git diff --stat f5db0517cf66..HEAD          # v1 audit head -> v2 tip
 docs/audits/2026-11-26-ci-gate.md | 45 +++
 docs/audits/README.md             |  1 +
 docs/todo/ci.md                   |  6 +++++-
 3 files changed, 51 insertions(+), 1 deletion(-)

$ git diff --name-only f5db0517cf66..HEAD -- .github/ scripts/ci/ src/ backend/
(empty — 0 files)

$ git diff --stat bc9e4909..f5db0517cf66
 96 files changed, 11814 insertions(+), 1022 deletions(-)
 (mostly UIV3 frontend batches merged to master 11-19..11-26; NOT part of the CI contract)

$ git diff --stat bc9e4909..f5db0517cf66 -- .github/
 .github/ci/branch-protection-checklist.md | 117 ++++++++++
 .github/workflows/ci.yml                  |  70 +++++++++-
 2 files changed, 162 insertions(+), 25 deletions(-)

$ git ls-files --stage .github/workflows/ci.yml .github/ci/verify-required-jobs.sh \
    .github/ci/branch-protection-checklist.md .github/ci/gate-policy.md
100644 3afca89e445d4b6ff007835d1f7259d454114660 .github/workflows/ci.yml
100644 7ff776e682947181ba150a3360d712ade2e7c02e .github/ci/verify-required-jobs.sh
100644 60901ebf39e9639e67abb15fc3e81f75d6ed8fe9 .github/ci/branch-protection-checklist.md
100644 f6e14bf5a4fd1f1dadea1f8b6c81d57572e62842 .github/ci/gate-policy.md
```

- `verify-required-jobs.sh` blob `7ff776e682947181ba150a3360d712ade2e7c02e` is byte-identical
  to the blob recorded (and name-verified, `34` conditional names ⊆ graph, `66↔66` live jobs) by
  the v1 pass at its tip — so the required-job name set at this tip is identical to v1's verified
  set. v1→v2 tip touched no CI file at all ⇒ job-name drift NET ZERO (receipt above).
- Registry-path drift: `docs/CI.md` MISSING, `.github/BRANCH_PROTECTION.md` MISSING (`Test-Path`
  → `False` in this worktree) — no phantom-file drift; the live authority remains the checklist
  itself, whose header pins its own re-derivation base `bc9e4909e092f1be63a45c4d81e20b3bf5b9a902`
  (= PR #1173 merge, 2026-11-19: `git log --oneline -1 bc9e4909`).
- Checklist Tier-1 (live file at tip, table rows): `ci-gate`, `current-diff-contract`,
  `affected-check-plan`, `changes` — with the explicit note that `ci-skip-gate` and
  `markdown-link-check` must NOT be required. Live branch protection
  (`gh api repos/Poryadok/VoiceRoot/branches/master/protection` →
  `{"contexts":["ci-gate"],"strict":false,…}`) requires only one of the four — F7-class
  strictness mismatch, unchanged in the window, owner decision, report-only (token has no `repo:admin`).
- `docs/TESTING.md` tier table: names unchanged since v1 (inside the 0-file window for CI scope;
  the v1→v2 docs window did not touch it — `docs/TESTING.md` last changed pre-v1).

CI runs (live, this pass):

- Tip master-push run: `gh api repos/Poryadok/VoiceRoot/actions/runs/45740179154` →
  run #6174 `push`, `master`, head `7fcc22cc9df6…`, status completed; jobs tally (both
  pages, `total_count 66`): `66 = 2 success / 0 failure / 64 skipped` — zero reds at the tip
  push; the two green are the gate legs (`ci-gate`, `ci-skip-gate` family), 64 path-filter
  skips expected for a docs-only delta.
- PR #1220 rollup (`gh pr checks 1220 --json name,state,bucket`, 66 checks):
  SUCCESS `ci-gate`, `ci-skip-gate`, `changes`, `current-diff-contract`,
  `affected-check-plan`, `markdown-link-check`, `seeded-enrollment`; FAILURE
  `owned-enrollment` (enrollment workflow run `45740137381`); `56` SKIPPED.
  `owned-enrollment` is a registered infra class in `docs/todo/ci.md` and is not
  gate-required (live protection requires only `ci-gate`); merged chain precedent
  #1218/#1219/#1220 all landed.
- Full-profile authority (carried, landed today in `docs/audits/2026-11-26-ci-gate.md`):
  dispatch run #6170 id `45728902140` at head `0ca77e1e`: `133 jobs = 118 success /
  11 skipped / 4 failure`, all 4 reds inside registered debt families, `0` new classes;
  tip deferral pattern for docs-only pins unchanged (F6-class).

### STG rollout gate — digest cross-check at retained pin (fresh sweep)

Catalog `scripts/ci/staging-image-catalog.json` = `23` images + gate ref
`nats-hub-config-renderer` = `24` refs; slug format `ghcr.io/poryadok/voiceroot/<name>:<commit-pin>`;
catalog carries no digest fields by design → digest authority = GHCR tag manifests, read
publicly (`docker manifest inspect`, Docker 27.4.0, no login, read-only; zero GHCR mutations).

Presence + digest sweep at the retained product pin `:d4af0992db2a50fac8eb94758e1020ad315f8be9`
(24 refs, raw manifests saved to this session's temp `ghcr_sweep_v2_raw.txt`; the durable
record is the 12-hex-prefix comparison below):

```
analytics bb95be18509f | bot 74fb1cf7cd33 | chat b4ade2edf43d | file b9791359860a
gameintegration 81e9c8965016 | gateway a22fdee9883a | matchmaking e95364cd18e5 | messaging c69762eaacb5
moderation 2cfcab6f9d85 | notification bf40d51e1377 | realtime f4ea05d42dec | role 38fd1eaadda1
search 000726cf5e92 | social 52ce0bc4fe89 | space d8a800838e07 | story 47ecf11fbf9f
subscription 61bd0ac13bcf | user e1e7b9fbdafd | voice 03b0d8b73e74 | auth 5a12dc03230f
web 3256a8171381 | developer-portal d64e732ec9b0 | admin 44fa715b09ad | nats-hub-config-renderer 5489892bad61
```

- vs the map recorded at `[d4af0992db2a:]` in `docs/audits/2026-11-22-tt-fleet-inventory-audit.md`
  (line 49): **`24/24` byte-equal** (every prefix above matches that table; the
  2026-11-19 audit independently records the same prefixes for analytics/bot/gateway/
  gameintegration/moderation/voice/auth/web — cross-agreement across three dates).
- vs the table in `docs/audits/2026-11-23-openchamber-fleet-inventory-audit.md` (its
  `:d4af0992` column): `8 OK / 16 DIFF` (`chat fa69b966→b4ade2ed`, `analytics 06b99ee3→bb95be18`,
  etc.). Since live tags agree with 11-19 and 11-22 and with v1's fresh receipts today
  (`chat b4ade2edf43d:` quoted in v1 + `docs/audits/2026-11-26-ci-gate.md`), the 11-23 table is
  an outlier for the `:d4af0992` column — recorded-map inconsistency, report-only self-correction
  candidate, no tag mutation by this pass and no re-anchor.
- Presence at the current docs tip tag `:7fcc22cc9d`: `0/24` — expected-absent for a docs-only
  pin (GHCR tag-retention/republish class, monitor-only, row registered in `docs/todo/ci.md`
  today via #1218/#1219; v1 recorded the recent-pin sparsity table).
- Negative sanity probe `voice-chat@sha256:0000…0000` → `manifest unknown` (rejected; no aliasing,
  no false positives in the sweep).
- Fence/operational chain (carried, unchanged): HELD-claim `b73c1c982a81…` gen14/rev57 stale +
  LAN fail-closed 2026-11-21 `operator_retirement_binding_invalid` — the HOLD blocker is
  operational (owner decision), NOT digest/registry mismatch. Gate verdict: digest axis INTACT
  at `:d4af0992`; HOLD stands.

## Decision

- Census honest (0 receipts, all BLOCKED); fleet state carried with file paths; no invented numbers.
- Drift verdict: job-name contract NET ZERO v1→v2 tip (0 CI files in window + identical blobs +
  v1's name verification carried); registry phantom paths report-only; F7 strictness mismatch
  unchanged; no new CI debt classes this pass (tip push run `2/0/64` zero reds; full-profile
  authority #6170 stays), so **no new `docs/todo/ci.md` rows** — the monitor rows landed today by
  #1218/#1219 already carry the tag-retention and enrollment classes.
- Gate verdict: digest authority at `:d4af0992` verified `24/24` vs the agreeing authority
  (11-22 map = live = 11-19 = v1); the 11-23 outlier table noted as a self-correction candidate
  (report-only, no mutation); HOLD remains operational/owner-side.
- Actionable (owner decisions, no cards — TT down; all classes already registered):
  PR #1208 conflict landing; F7 branch-protection strictness flip; GHCR tag retention;
  STAGING fence re-anchor; tip full-profile CI deferral pattern; human trio (`GI-T13` OAuth consent,
  `OPS-STAGING-INITIAL-RECONCILE-144`, `OPS-STG-LAN-INSTALLED-COHORT-ROOT-V1`); epoch v17 unopened.
- Cards created: `0`. Outage payload quoted in v1, dedup honored.
- This doc lands as a docs-only PR: audit file + one README index row + one PLAN WIP row
  (the 2026-11-26 ci.md monitor rows already landed via #1218/#1219; nothing new to register).

## Reachable

- Owner: #1208 landing/closing decision; branch-protection Tier-1 flip (needs `repo:admin`);
  GHCR retention restore; STAGING fence re-anchor; v17 epoch opening; tip full-profile re-run
  on next product pin. Infra: TT backend bring-up (card payload owed since v1).
- Human waiting points (not this audit): OAuth consent decision, initial-reconcile
  `OPS-STAGING-INITIAL-RECONCILE-144`, LAN cohort root.

## Self-corrections recorded this pass

- The v1 README row's "delta `bc9e4909..` (17 files +540/−42)" figure refers to the v1 PR's own
  changeset scope, not a full-tree drift: full tree `bc9e4909..f5db0517` = 96 files `+11814/−1022`
  (UIV3 frontend batches between the checklist anchor-base `bc9e4909` and v1 head, no CI contract
  files), `.github/**`-scoped = 2 files `+162/−25`, docs+scripts scope = 43 files. CI job-name
  drift conclusion remains NET ZERO (proven independently by the 0-file v1→v2 window + blob identity).
  Report-only documentation correction; no card.
- Digest authority map for `:d4af0992`: the 2026-11-23 audit table disagrees with live
  `+` 11-22/11-19/v1 agreement for 16 of 24 images; the 2026-11-22 audit table is the agreeing
  authority. Recorded-map column correction for the 11-23 doc (report-only; no tag mutation
  observed between 11-22 records and today — live equals those records byte-for-byte).
- `tt --timeout` requires a unit suffix (`90s`); bare numeric values fail client-side
  (`invalid timeout`) — receipt note, carried from v1's `--timeout=90s/120s` usage.
- `gh api` pagination endpoints return wrapper objects (`.workflow_runs` for runs, `.jobs`
  for jobs) — tooling note that shaped this pass's tallies; `gh pr checks --json` fields are
  `name`/`state`/`bucket` (no `pass` field).
- `origin/` literal directory in the main checkout makes `git log origin/master` ambiguous —
  all receipts here use `refs/remotes/origin/master` (carried rule, re-confirmed).

## Appendix — raw receipts of this pass

- TT: two `connection_error` receipts above (`context deadline exceeded`, `ConnectEx`); zero census/status reads; zero writes attempted.
- PR: `gh pr list` + `gh pr view 1208` JSON above; `git log -14 refs/remotes/origin/master` above.
- Windows: `git diff --stat` receipts above (`f5db0517..HEAD` 3 docs files; `-- .github/ scripts/ci/ src/ backend/` 0 files; `bc9e4909..f5db0517` 96 files; `.github/` scope 2 files `+162/−25`).
- Blobs: `git ls-files --stage` above (`7ff776e6…` unchanged vs recorded).
- Protection: `gh api …/branches/master/protection` → `{"contexts":["ci-gate"],"strict":false}` (unchanged since v1's fresh read).
- Runs: #6174 `45740179154` completed `66 = 2/0/64`; PR #1220 rollup `7 SUCCESS / 1 FAILURE owned-enrollment / 56 SKIPPED` (66 checks); authority #6170 carried.
- GHCR: 24-ref sweep `:d4af0992` all PRESENT, 24/24 byte-equal vs 11-22 table, `8/16` vs 11-23 outlier; tip `:7fcc22cc9d` `0/24`; probe `manifest unknown`; raw manifests in session temp `ghcr_sweep_v2_raw.txt` (temp not committed; this doc is the durable record).
- Catalog: `scripts/ci/staging-image-catalog.json` 23 images, no digest fields; gate ref renderer = 24 refs swept.