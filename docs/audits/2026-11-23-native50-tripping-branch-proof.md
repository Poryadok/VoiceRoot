# Native50 transition tripping branch — class-B proof pass (2026-11-23)

Agent: Codex-CI-Debt-FixB (fleet worker) + Codex-Main-Verify (main-session read-back verification), OpenChamber session, model qwen3.8-flash-next-coder-iq1_m.

## 1. Scope and method

Prove the tripping branch of the class-B leaf `native50 transition failed`
(`src/backend/space/internal/nativeinstaller/runner.go:412`,
`TransitionNativeMembershipCatalog50`) to the exact failing check, decide whether
any class-B-only code change is canon-prescribed, and correct the class-A/class-B
mechanism attribution carried by `docs/audits/2026-11-22-ci-debt-family-root-causes.md`.

Method: source read of the whole transition chain (all line numbers cited were
re-read in the docs worktree at base `2285c041c4f01328a5ba53f84de21fcc8c5019f6`
and verified by the main session), the canonical t31 replay recorded in `docs/todo/ci.md`
(replay of all 14 `assertNoInstallableBlockersForNative50` predicates at a fresh
compose-db-init position), CI receipts (class-B job 136938451708), and the
owner decision (b) recorded 2026-11-22 on card `CI-SPACE-BAN-AUTHORITY-FIXTURE-DRIFT` c5.

## 2. Ground truth

- Tripping branch PROVEN: `src/backend/space/internal/store/native_catalog50_transition.go:86-88`
  — the FIRST `checkMemberAuthorityCatalog(ctx, tx)` guard, executed BEFORE the CAS
  commit (`s.Commit` at `:140`). Chain:
  `:86` → `member_authority_catalog.go:183-185` (`checkNativeReadinessCatalogVersion`
  call at `:184`) → `native_readiness_catalog.go:47` (`checkAccountBanCatalog`)
  → `account_ban_catalog.go:84-86` structure-digest pin compare
  (pins `accountBanStructure16SHA=972518e6…` at `:63`,
  `accountBanStructure18SHA=f5cff76a…` at `:64`, version-map expectation at `:76`).
  The failing digest compare is the class-A hub itself — class B has no separate hub.
- Elimination of the other candidate guards:
  - `migrated50` was emitted in every failing trail ⇒ guards at `:59`/`:61-65`/`:81`
    (MigrationSHA256 constant, installer-role ownership, schema-version-50) are green;
    `:59` is a receipt-constant assignment, not a check.
  - `:107-118` (frames drained, seal equality) run after the projection block, past the
    first failure point.
  - `:133` recheck re-runs the identical `checkAccountBanCatalog` with unchanged inputs
    after commit — cannot be the FIRST failure.
  - The two go-test jobs that fail alongside assert installer behavior conditioned on
    this very failure (collateral, not independent causes).
- Exhaustive proof by the t31 replay record (`docs/todo/ci.md`): replaying all 14
  read-only predicates of the 50-transition on a fresh `space_db` at the compose-db-init
  position, every guard is green and "The ONLY failing check is `checkAccountBanCatalog`".
- Corrected drift mechanism (supersedes the runner-bump attribution): `compose-db-init`
  builds the Postgres service from the FLOATING base tag `postgres:18-alpine`
  (logged in class-B job 136938451708); the Postgres float moves attver-sensitive
  catalog text bytes between the pin era (18.7) and the boundary era (18.16):
  observed attver 96 (db-init) vs 102 (testcontainer) at the same head/M50 seed
  `230166B84BC8`, digest-relevant byte deltas `29275528` vs `29275584` at offsets 294/854.
  Both boundary jobs of the 2026-11-22 audit log the SAME runner image
  `20261101.186` ⇒ runner-image bump attribution is INVALID.
- Docs-canon verdict on the class-B reconciliation item ("transition pinned to M50"
  vs M51): BY DESIGN, not debt. Chain order is `transition50 → trusted50 → transition51 → 51`,
  and `src/backend/migrations/space_db/000051.up.*` prereq itself requires a genuine
  trusted old-50 transaction; the docs' `Native51 trusted50 and fresh51 attver position`
  gate already covers M51. No docs or migration reorder is owed.
- Companion digests fresh-printed in failing jobs (`source_sha256=59c34244…`,
  `executable_sha256=9090f7c1…`, `migrator_sha256=352d0e23…`): receipt-level only.
  `src/backend/space/internal/nativeinstaller/source_manifest.go:13` pins
  `InventorySHA256=ceddc7e7…`; `352d0e23…` exists nowhere in the code, so those
  fresh values are not failing comparisons.

## 3. Findings

1. Class B (`space-native-installer-integration`, `compose-e2e`,
   `a1-attachment-restart-proof` leaves under `native50 transition failed`) shares the
   class-A hub (`account_ban_catalog.go` digest pin). A class-B-only code change is NOT
   canon-prescribed: the remedy canon for the hub is owner decision (b) — attver-aware
   normalization of strictly historical fields + re-record pins from the db-init
   (source-bound) position + fail-closed drift test — landing in the class-A lane
   (`docs/todo/ci.md` «Korrekcia», card c5).
2. Class-B jobs auto-green when the class-A hub flips; F2/F3/F4/F5 audit items stay
   open until FRESH CI re-run evidence shows the class-B jobs green — deliberately
   unclaimed until then.
3. The M50/M51 reconciliation is by design (Section 2), no action owed.
4. Honest gaps: CI logs carry no structure digests; "CI boot ≈ local boot" equivalence
   remains canon-assumed, not directly measured; class-A fix branch
   `codex/ci-space-ban-attver-normalization-20261123` is still in flight.

## 4. Actions taken

- Class-B card `CI-NATIVE50-INSTALLER-TRANSITION-CHAIN`: proof comment c2
  (`Codex-CI-Debt-FixB`), checkpoint e2; main-session verification comment c3
  (`Codex-Main-Verify`) + checkpoint e3 — read-back: worker worktree
  `tmp/ci-native50-transition-proof-20261122` clean at base `85bbba48`, no origin
  branch, no PR — the honest empty-diff outcome (canon prescribes no class-B-only change);
  code anchors cited in the proof were re-read and confirmed by the main session.
- LAN card `OPS-STG-LAN-CAPTURE-GREEN-PUSH-V1`: checkpoint records class-B proof
  delivered + verified; capture target still zero until class-A merges, then the first
  all-green full-graph master push.
- Class-B proof worktree removed, local branch deleted (pool cleanup).
- This docs-only PR: audit file added, `docs/audits/README.md` index row,
  `docs/todo/ci.md` class-B closure + mechanism-correction bullet.
- 0 new TT cards: every finding is carried by the existing class-A/class-B cards.

## 5. Verdict

Class-B debt is now PROVEN to be pure class-A propagation: its leaf shares the
class-A hub, its closure follows from the class-A fix CI evidence, and the
runner-bump mechanism claim from the 2026-11-22 audit is corrected to the floating
`postgres:18-alpine` base tag. The chain to the LAN capture legality remains:
class-A fix PR (`codex/ci-space-ban-attver-normalization-20261123`) merge → first
all-green full-graph master push = the only legal Stage B capture source.

## Self-corrections

- `docs/audits/2026-11-22-ci-debt-family-root-causes.md` class-A "hosted runner image
  bump 20261101.186→20261111.196 rolled the Postgres fixture" attribution is INVALID:
  both boundary jobs log the same runner image `20261101.186` (already superseded in
  ci.md by the 2026-11-22 «Korrekcia» correction; restated here for the class-B side).
- The class-B card reconciliation item ("transition pinned to M50") is answered as
  by-design, not debt — recorded in ci.md so it is never reopened as a docs fix.

## 6. Deliberately not done

- No class-B code change and no class-B PR: canon prescribes none; the honest
  outcome is an empty diff. The abandoned old pin-route branch
  `codex/ci-space-ban-authority-pin-20261122` stays deleted (pin route disproven,
  matrix c2); the active class-A branch is
  `codex/ci-space-ban-attver-normalization-20261123`.
- No re-pinning or edits of `account_ban_catalog.go` here — class-A lane's exclusive scope.
- F2/F3/F4/F5 not closed — need fresh CI evidence post class-A merge.
- LAN Stage B–C untouched: still debt-gated, zero legal capture source; owner ceremony
  pack at the session temp dir stays BLOCKED.
