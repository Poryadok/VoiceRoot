# 2026-11-22 CI debt family root causes — Codex-CI-Debt-Roots (OpenChamber session, model qwen3.8-flash-next-coder-iq1_m)

Verdict pass (docs-only, branch `docs/ci-debt-root-causes-20261122`): корнев причины deкомposition of the persistent red family
of full-validation CI — the family that blocks both LAN baseline capture legality (per `_ci`, audit
`2026-11-21-lan-capture-source-green-push-satisfiability.md`) and base-branch merges of code PRs (!1191, !1193).
This pass mutated nothing: no CI config, no test fixture, no product path; the class-A fixture-pin fix PR is a separate
in-flight artifact, verdict report-only here.

## 1. Scope and method

- Investigation read-only (gh api: runs/jobs census of workflow 263230665, job logs, pulls; GitHub compare api;
  repo source reads `src/backend/space/internal/store/account_ban.go`,
  `src/backend/space/internal/grpcsvc/lifecycle_deletion_auth_test.go`,
  `src/backend/space/internal/nativeinstaller/runner.go`,
  `src/backend/space/internal/store/native_catalog50_transition.go`,
  `src/backend/migrations/space_db`). Staleness caveat: all PR/run facts from gh API; local main checkout
  was stale (never mutated) and was not used as evidence.
- Runs censused: master push `44934578838` (head `eb341dd80325cfb640bfb9b519fcb0926dbff06c`, !1190 merge, 151 jobs, 10 red);
  night authoritative `workflow_dispatch profile=full` #6090 = run `44850194289` (head `7d7808a773364c06dfa3f4a696e66ef75b21c0e3`,
  133 jobs, 10 red — intake already recorded as TT comment c4 on `OPS-STG-LAN-CAPTURE-GREEN-PUSH-V1`);
  tonight authoritative dispatch #6105 = run `44971137142` (head tip `230166b84bc8d1d214e176638e146c52ebeb5069`,
  created 2026-11-22T01:22:10Z, **in_progress** — its verdict is NOT part of this pass, nightly session polls it).
- Determinism cross-check over master-push runs 42999756889 (2026-11-11T09:59Z), 43099468032, 43205490394,
  43389396732, 43463924560, 43626614882, 43641582322, 43695825108, 43768627883, 43787631870, 44153952853,
  44329501457, 44589215159; run 44786511718 (66 jobs, all-skipped) not a datapoint per verdict-reading rule.

## 2. Ground truth

| Fact | Anchor |
|---|---|
| master push `44934578838` = `eb341dd80325…` (!1190 merge), 151 jobs, 10 red: `backend-go (role)` 136938452179, `backend-go-integration (space)` 136938452265, `backend-go-integration-pr (space)` 136938451893, `role-space-receipt-integration` 136938451613, `compose-e2e` 136938451708, `local-ci-parity` 136938451625, `space-native-installer-integration` 136938451544, `a1-attachment-restart-proof` 136938451510, `ci-gate` 136947207369, `full-validation-aggregation` 136949700380 | gh API run id |
| night authoritative #6090 = run 44850194289, `workflow_dispatch`, head `7d7808a7…`, 133 jobs = 10 red (`voice-history-audit` 136679539961, `compose-e2e` 136679540764, `space-native-installer-integration` 136679540913, `local-ci-parity` 136679540980, `backend-go (role)` 136679541184, `backend-go-integration-pr (space)` 136679541185, `a1-attachment-restart-proof` 136679541210, `role-space-receipt-integration` 136679541314, `backend-go-integration (space)` 136679541589, `full-validation-aggregation` 136694046773); `ci-gate` GREEN at this head; intake = TT c4 on `OPS-STG-LAN-CAPTURE-GREEN-PUSH-V1` | run id, job ids |
| tonight #6105 = run 44971137142 at tip `230166b84bc8…`, event `workflow_dispatch`, created 2026-11-22T01:22:10Z — in_progress, no verdict in this pass | run id |
| class A green→red boundary: green master push `547509ef9cee` run 43205490394 job 131140328215 — 0fail/2415pass, runner image release 20261101.186, git 2.56.0, go1.26.10, testcontainers-go v0.35.0 → red master push `ca73150643fa` run 43389396732 job 131765529570 — 17fail/2398pass, image 20261111.196, git 2.56.1; go/testcontainers-go identical both sides | job logs |
| compare `547509ef9cee...ca73150643fa`: status ahead, 5 commits, changed only `scripts/ci/f13_native_p3_successor_integration_test.py`, `scripts/ci/f13_role_conditional_successor_test.py`; Space/Role Go sources and migrations byte-identical at refs `547509ef9cee`, `ca73150643fa`, `3c2a24e3112d`, `eb341dd8` | compare API |
| migrations present at all four refs: `src/backend/migrations/space_db/000050_account_ban_coverage.{up,down}.sql`, `000051_native_catalog_rowjson.{up,down}.sql` | refs above |
| !1190 merged 2026-11-21T18:16:05Z, merge_commit_sha == run-44934578838 head (9 files, frontend/l10n/docs); !1191 `merged_at: null` (12 files) | gh pulls |
| leaf A: `complete account ban authority unavailable` `src/backend/space/internal/store/account_ban.go:29`; gate `:64-68` (`version != 50 && version != 51`); raise sites `:79`/`:92`; fixture hub `src/backend/space/internal/grpcsvc/lifecycle_deletion_auth_test.go:138` `db.Run(goto 34/44/45/50)`, first failing assertion `:163-164` `TestSpaceDeletionProofConsumedResponsePersistsInRealStore` on `TransitionNativeMembershipCatalog50` | code + logs |
| leaf B: `native50 transition failed` `src/backend/space/internal/nativeinstaller/runner.go:412` (legacy twin `legacy24_runner.go:305`) after `migrate(50)`; hub `src/backend/space/internal/store/native_catalog50_transition.go:50-143` pins MigrationSHA256 `:59`, installer-role ownership `:61-65`, schema version exactly 50 `:81`, frames drained `:98`, seal equality `:107-118`; red since earliest checked master run 42999756889 (2026-11-11T09:59Z, jobs 130454129570/130454129519) | code + logs |
| registry: these persistent red classes already registered in `docs/todo/ci.md` (Space/Role ban-catalog shard, native50 pin chain, aggregators/parity); this pass adds root causes, not a new class | docs |
| TT state: cards `CI-SPACE-BAN-AUTHORITY-FIXTURE-DRIFT` (claimed `Codex-CI-Debt-FixA`, in_progress), `CI-NATIVE50-INSTALLER-TRANSITION-CHAIN` (available); capture-card comment c4 (night intake) | TT journal |

## 3. Findings

### F1. Class A — Space/Role ban-authority reds: hosted runner-image roll-forward + unpinned DB fixture, NOT product drift

- Members (run 44934578838): `backend-go (role)`, `backend-go-integration (space)`,
  `backend-go-integration-pr (space)`, `role-space-receipt-integration`, `local-ci-parity` (card grouping;
  analyst put `compose-e2e` here instead of `local-ci-parity` — see Self-corrections).
- Proof chain: green→red discriminator changed only 2 CI-script files, Space/Role Go + migrations byte-identical
  at both refs; the flip coincides exactly with GitHub ubuntu26 image release 20261101.186→20261111.196
  (git 2.56.0→2.56.1); go/testcontainers-go identical. Postgres behavior via testcontainers floats on hosted
  environment; images roll forward only ⇒ persistent red since 2026-11-13.
- Decision (owner = CI/test-env): pin the Postgres testcontainer release used by the Space fixtures in minimal
  test/fixture env per `docs/TESTING.md` fixture policy, restoring the Nov-12 green behavior; fixture-pin site
  `auditMigrationFixture` not yet read (gap G1) — fix PR must locate it. No product touch: gate
  `account_ban.go:64-68` already tolerates M50/M51. Risk: low–medium, harness-only.
- Card: `CI-SPACE-BAN-AUTHORITY-FIXTURE-DRIFT`; fix PR branch `codex/ci-space-ban-authority-pin-20261122` in flight
  (no green verdict claimed for it in this pass).

### F2. Class B — native50 installer transition chain: candidate product provisioning gap, tripping branch NOT proven

- Chain members: `space-native-installer-integration`, `compose-e2e` (own compose receipts stop at `migrated50`
  without `trusted50`; `Makefile:123` `compose-migrate-all` Error 1; `Makefile:168` `compose-db-init-1` log
  `native50 transition failed`), `a1-attachment-restart-proof` (chain-adjacent, but analyst log census classifies
  this job as transient (c) — see F4; card mentions it, closure not implied).
- Installer reaches hub preconditions in `native_catalog50_transition.go`; exactly which branch trips
  (`:59`/`:81`/seal `:107-118`) unproven (gap G2). Red since 2026-11-11 — predates class A and predates !1190/!1191.
- Decision: prove the tripping branch in compose-db-init first (analyst gap), only then reconcile M50/M51
  provisioning; separate PR from class A, never combined; blind fix risk high (transition load-bearing).
  No lawful fix authorized in registries before that confirmation.
- Card: `CI-NATIVE50-INSTALLER-TRANSITION-CHAIN` (available).

### F3. Aggregators — `ci-gate`, `full-validation-aggregation` have no own defect

- `ci-gate`: re-derives leaf results (`compose-e2e result=failure`, abort at
  `.github/ci/verify-required-jobs.sh:33`); GREEN at #6090 head, red at 44934578838 head — consistent with
  propagator behavior, not a defect. Wiring `.github/workflows/ci.yml:4473-4476`/`:4557-4558`, policy
  `.github/ci/gate-policy.md`. `full-validation-aggregation`: `integration source inventory incomplete
  (unresolved=14)` + `jobs did not succeed: backend-go, backend-go-integration-pr`.
- Decision: no PR — gates flip green automatically when class A/B leaves flip; weakening the gate would
  violate gate-policy. Gap G3: the `unresolved=14` inventory leg is unattributed
  (`scripts/ci/full_validation_gate.py`, `scripts/ci/go_integration_matrix.py` unread).

### F4. Transient hosted-flake retry class — monitor only, no retry policy invented

- `a1-attachment-restart-proof`: red at 44934578838 but GREEN at Nov-18/Nov-19 heads
  (44329501457, 44589215159 per analyst log census); mirror pair `a1-flutter-profile-handoff` /
  `t31-critical / compose-https-claim-inbox-ack` swaps liveness between runs; Nov-13–16 hosted family
  (`ci-script-tests`, `backend-auth`, `admin`, `local-docker-isolation`) self-healed by Nov-18.
- Decision: retry-class only (no code fix, no registry change).

### F5. Attribution — the persistent reds are NOT caused by our merges

- !1190 merged 2026-11-21T18:16:05Z (9 files, frontend/l10n/docs), !1191 unmerged (12 files).
  Space/Role red present from run 43389396732 (Nov-13, six days before the merge); installer/parity red
  from 42999756889 (2026-11-11T09:59Z). Run 44934578838 is only the first master push after the merge —
  so the base-policy BLOCKED state of !1191/!1193 is debt-caused, not merge-caused.

### F6. Honest gaps — no root cause reached (report-only)

- G1 class A: pin site `auditMigrationFixture` unread; the concrete floating PG image/tag delta not pinned from
  job logs (mechanism inferred from image-release diff + fail counts 0→17); class split (c) vs (d) evidenced, not proof.
- G2 class B: tripping branch in `native_catalog50_transition.go` unproven; leaf (product provisioning vs test-env) unresolved.
- G3 `unresolved=14` inventory leg and go-integration-matrix rejection mechanism unattributed.
- G4 `voice-history-audit`: red in night intake #6090, not independently root-caused in this pass
  (night intake c4 classifies it inside documented debt/flake family).
- G5 transient family: classified (c) by self-heal/non-repro pattern only, no per-job cause.
- G6 `account_ban.go` beyond `:131` unread; moderation.go anchors are context, not failure sites.

## 4. Actions taken (receipts)

- 2 TT cards created, epoch A1, descriptions carry EVIDENCE + DECISION + PRIORITY HIGH: `CI-SPACE-BAN-AUTHORITY-FIXTURE-DRIFT`
  (claimed by `Codex-CI-Debt-FixA`, in_progress), `CI-NATIVE50-INSTALLER-TRANSITION-CHAIN` (available). 97 `CI-*` cards
  checked for duplicates — zero coverage of these two families.
- TT evidence comment c4 on `OPS-STG-LAN-CAPTURE-GREEN-PUSH-V1`: authoritative night verdict #6090 intake
  (133 jobs, 10 named reds, all inside documented debt/flake family, zero new classes).
- Docs 4 files: this audit + `docs/audits/README.md` index row + `docs/todo/ci.md` dated receipt +
  `docs/PLAN.md` A4 row and journal row; branch `docs/ci-debt-root-causes-20261122`; docs-only PR to
  `master` (merge after checks settle; `owned-enrollment`-only red tolerated per !1192 precedent). Merge receipt is
  posted into debt-card and capture-card TT history by the merging leg, not claimed here.
- Report-only: class-A fixture-pin fix PR in flight (`Codex-CI-Debt-FixA`) — no CI green verdict claimed for it in this pass.

## 5. Verdict

- The full-validation debt family is decomposed into: one actionable CI/test-env family (A — fixture/toolchain
  pin, low–medium risk, PR in flight), one product-side chain (B — tripping-branch proof owed before any fix),
  two leaf propagators (C — auto-green with leaves, untouched), one retry class (D — hosted flakes, monitor only).
- LAN capture legality chain: merged satisfiability audit holds — zero green full-graph master pushes in the
  24-image era (scan 2026-10-17..2026-11-21: 42 push runs, 0 green; full-graph rows only from 2026-11-07);
  tonight's dispatch #6105 verdict pending at this writing. The human gate is debt-gated, not human-gated.
  Capture target = the next all-green full-graph master push after A/B leaves flip.
- STG `:d4af0992` HOLD persists until LAN Stage C closes; !1191/!1193 merges stay base-policy BLOCKED until leaves flip.

## 6. Deliberately not done

- Product paths untouched (`account_ban.go` gate, installer hub, migrations, `runner.go`) — only after M51-era
  contract confirmation (class B decision requires the tripping branch first).
- No class-B PR opened this pass (proof owed); no blind fix of the load-bearing transition.
- Aggregators and retry class: no changes per gate-policy; retry policy not invented.
- Class-A fix PR CI verdicts: in flight — receipts land in debt-card TT history when verdicts settle;
  nothing here fabricates CI evidence.
- LAN node and STG fences: untouched (STAGING fence `b73c1c98…` gen 14 rev 57 HOLD as-is).

## Self-corrections

- Grouping of two audited-run jobs (`compose-e2e`, `local-ci-parity`) differs between the analyst report and the
  two TT cards: analyst put `compose-e2e` under A (ban-authority leaf observed in its log) and
  `local-ci-parity` under B; the cards put `local-ci-parity` under A and `compose-e2e` under B (own evidence:
  compose receipts stop at `migrated50` without `trusted50`). Both jobs are leaf propagators — the corresponding
  fix PR must record, per job verdict, which leaf flipped. Card grouping was not rewritten this pass.
- An earlier migrations-404 artifact in the investigation was a wrong-path artifact, corrected to real names
  `000050_account_ban_coverage`, `000051_native_catalog_rowjson`.
