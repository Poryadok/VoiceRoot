# ExecPlan: assemble the Rudeus colony into develop

## Purpose

Collect the completed source results of Rudeus-1 through Rudeus-140 into one
history-preserving `develop` branch. This is the user's explicit 2026-10-18
instruction to finish current work and assemble its results. It does not claim
that incomplete feature slices become complete when merged.

## Context

- Tracker: project `voice`, task `SWARM-DEVELOP-20261018`, endpoint
  `http://192.168.0.109:30990`. Product tasks retain their author ownership.
- Canon: `AGENTS.md`, `.agent/AGENTS.md`, `.agent/PLANS.md`,
  `docs/CONTRIBUTING.md`, `docs/TESTING.md`, and each selected source task's
  accepted feature/service contract. The user's directive overrides the old
  A1-only admission restriction and explicitly selects `develop` as target.
- Sole physical develop writer and integration/CI owner: Rudeus-121. Main
  checkout remains on `master`; its pre-existing dirty files are untouched.
- Integration workspace: `D:/Git/Voice/.treehouse/Voice-451d38/39/Voice`, own
  lease `4c53ae84671c0eb1384cc4def6a0ec6f`, holder `rudeus-121-develop`.
- Initial local/cached remote develop:
  `4d6e26608934b3350be68b71402aae54662eec51`, an ancestor of cached master
  `698d4f064f032ea74abf3ede2017100b26c8ee69`. There were no develop-only commits.
- Reproducible local inventory under `tmp/develop-integration/`: `tasks.json`
  contains full TT source/check records, `task-index.json` the 670-task index at
  initial revision 1133, `refs.txt` local refs, and `candidates.json` commit-to-TT
  evidence associations. Portable copies/bundle must be preserved before return.

## Scope

Include already frozen colony sources and final successors from current authors,
with ancestor deduplication. Read-only independent review of unreviewed final
ACL/client heads is part of this integration. Exclude unrelated/user branches,
uncommitted source, deployment, acceptance appointment, and blindly closing old
PRs. Preserve actual full-feature/runtime/hosted-CI gaps in TT and the manifest.
No new agents or ordinary implementation tasks may be started.

## Milestones

- [x] Finish BE-102 review, record narrow source CLEAR at exact `0361d8597` (c9).
- [x] Atomically claim integration task and acquire exclusive worktree lease.
- [x] Inspect existing develop history and merge cached master with merge commit.
- [ ] Inventory final source heads, associate TT evidence, dedupe ancestors.
- [ ] Merge ready dependency groups and resolve conflicts against accepted canon.
- [ ] Independently review current final identity/ACL source heads before merge.
- [ ] Verify assembled generated contracts, migrations, and affected components.
- [ ] Regenerate graph with `graphify update . --no-cluster`, preserve bundle and
  portable manifest/evidence, reconcile remote develop and publish normally.

## Detailed Steps

1. Reconcile author final messages with TT immutable checkpoints and local Git
   objects/bundles. Do not take every local branch as colony work. Heads already
   ancestral to cached master/develop are accounted without redundant merges.
2. Integrate independent reviewed leaves first. Preserve every source commit;
   use `git merge --no-ff --no-stat HEAD -m ...`, never squash/rebase/amend/reset.
3. Assemble shared Role/Space/Chat groups in dependency order. Role Admin17
   precedes producer18 and entry-policy19. Role20 was released: admission DTOs
   and receipt codec are contract-only, with no schema/receiver activation.
   Space nickname26 and audit27/29,
   composite28, entry-policy30, composite audit-ID correction31 require their
   exact final author order. Do not invent or silently renumber migrations.
4. Preserve all additive protobuf fields/RPCs. Resolve source proto conflicts
   first, then regenerate canonical Go/Dart consumer trees as one assembled
   result. Do not choose one generated side over another added field.
5. Integrate BE-102 wire before BE-106 committed journal encoder adapter.
   BE-256 pending-intent safety and ordinary media identity mapping both edit
   `call_providers.dart`: retain their separate imports and distinct logic.
6. For a material conflict, record exact parents/hunks and consult the owning
   author. Original authors review integration-authored behavioral deltas.
7. Run bounded affected checks after coherent groups; broaden only for actual
   integration risk. Record exact SHA, executed tests, skips and failures. Run
   Realtime only on supported Linux; Windows compilation is not execution.
8. Remote transport recovery belongs to Rudeus-135. Its observed proxy TLS reset
   is not repaired by repeat probes. Until actual recovery, preserve local
   develop and verified bundle. On recovery read live develop ref, merge any
   genuine remote additions without rewriting, then ordinary push.

## Validation

- CRLF-aware `git diff --check` and no unmerged paths after each resolution.
- Ancestor checks cover every accepted manifest head at the assembled tip.
- Proto lint/format/breaking and generated Go/Dart parity for assembled additions.
- Relevant Go short/vet/lint plus real no-skip PostgreSQL tests for affected
  migration/transaction groups, using the recovered authorized local runner.
- Relevant Flutter tests/analyze for combined provider/media/client changes.
- Relevant Python/shell/workflow tests for NATS, CI and admin browser changes;
  source review is distinct from Linux JWT/TLS/bootstrap/Realtime runtime proof.
- Preserve hosted CI, media, deployment and appointed acceptance limitations;
  local merge commits prove only source integration.

## Progress

Cached master merged as `0bd6750ff`. Initial independent group merged without
conflicts: BE-120 `472556807`, BE-122 `eadbf70df`, BE-070 `b5fef7ab`, BE-051
`21f5959ba`, BE-050 `f8c713b06`, BE-141 `e78be9ddf`; resulting tip `f99f747d8`.
Author-provided exact final heads and remaining gates continue in TT/manifest.

## Decisions

- Preserve existing develop instead of replacing it: user requires history.
- One physical writer/queue; original task checks/ownership stay with authors.
- Pending source review and runtime prerequisites remain visible even if the
  underlying source checkpoint is accepted into the assembly.
- No remote mutation until successful exact-ref reconciliation; cached tracking
  refs are not live remote evidence.

## Risks And Follow-Ups

Current BE-106/BE-130/BE-131 authors are finishing corrections and checks;
integrate their final immutable successors rather than stale snapshots. BE-064
has no new implementation source and its clean preparation lease was returned.
BE-010 final `deff1b0` delivers a bounded policy slice, not the full admission
runtime; Role20 remains unimplemented and is not a missing commit to invent.
Shared generated contracts and Role/Space migrations need assembled verification.
BE-102's source CLEAR excludes actual committed journal emission and Linux
runtime gates. Several product tasks intentionally retain larger open scopes.
GitHub transport remains externally blocked by configured proxy upstream TLS
reset; no hosted CI/publication success is implied by local progress.
