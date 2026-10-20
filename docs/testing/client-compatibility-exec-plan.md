# ExecPlan: A4 client compatibility package

## Purpose

Make the A4/G1 qualification package enumerate every declared Windows OS/arch
and Web OS/arch/browser/version tuple against both previous/current clients and
backends. Missing tuple smoke, version enforcement, update handoff or updated
client rollback observations must prevent offline evidence closure.

## Context

Sources are PLAN A4/A5/G1, features/platforms.md, features/updates.md and
TESTING.md. Gateway owns version enforcement. Web uses CDN/API compatibility;
desktop owns the update prompt/manifest handoff. Existing
scripts/release/capability_manifest.py supplies strict JSON and digest helpers.
There is no supported-client inventory in the current release package.

## Scope

Add a versioned, operator-supplied compatibility declaration, deterministic case
compiler and offline evidence verifier beside the capability gate. The A4/G1
release qualification consumer supplies immutable artifact hashes and exact
observed tuples. Do not manufacture supported versions, execute installers,
deploy, approve support or attest external observations.

## Milestones

- [x] Confirm canonical scope and existing package tooling.
- [x] Add discriminating tests for complete cross-product and fail-closed results.
- [x] Implement compiler/verifier and CLI; document runner responsibilities.
- [ ] Run focused checks; freeze exact source for independent review and121 intake.

## Detailed Steps

1. Require both Windows and Web nonempty exact tuples and distinct previous/current
   release identities. Bind source and artifact content independently of labels.
2. Compile smoke for all four client/backend combinations per tuple, Windows
   version/forced-update/update-URL cases, and current client backend rollback.
3. Require every exact generated case in runner reports; reject missing/extra,
   duplicate, skipped, stale or nonportable evidence. Sampling policy applies
   only to extended regression, never mandatory smoke.
4. Wire focused unit tests into ci-script-tests. Keep runtime and qualification
   outside the offline verifier's claim.

## Validation

Run Python unittest discovery for compatibility and existing capability tests,
CLI plan/rejection checks and git diff --check. Tests are synthetic offline
contract fixtures and provide no actual client runtime evidence.

## Progress

- [x] Initial import-only failure was scaffolding, not behavioral RED.
- [x] Behavioral RED: 3 assertion failures admitted wrong Windows OS, nonexact
  browser version and rolling OS version; rollback case lacked client-update
  sequence (one error). Seven test methods executed.
- [x] GREEN: compatibility7PASS0skip and existing capability11PASS0skip on
  Python3.12, including full22case fixture, stale evidence and CLI error handling.
- [x] git diff --check exited0. No product runtime or hosted CI executed.
- [ ] Freeze source, independent review and sole121 accepted delivery.

## Decisions

No default supported OS/browser versions: canon requires observed support smoke
and supplies no version baseline. The operator must explicitly declare exact
tuples and actual previous/current immutable artifacts. Web gets API smoke and
rollback cases, not a fabricated desktop update flow (updates.md Web section).

## Risks And Follow-Ups

Actual tuple execution, desktop install/update/trust, browser cache transition,
server rollback, protected provenance, current CI, stands and appointed acceptance
remain open. An offline result checks evidence completeness, never its truth.
