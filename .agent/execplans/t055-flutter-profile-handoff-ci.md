# ExecPlan: T-055 Flutter profile-handoff CI reachability

## Purpose

Make the existing opt-in T-055 Flutter live profile-switch/reconnect/inbox
contract reachable from a deliberately isolated A1 GitHub Actions job. The
job must exercise exactly that test against a generated Compose project, not a
developer's shared `voice` stack, and must be selected only by the documented
nightly/manual-full/master-push A1 paths. Extend the existing dedicated A1 E2E
path filter so a change below `src/backend/scripts/**` cannot silently bypass
this reachability proof.

## Context

- Product behavior: `docs/features/multi-profile.md` requires an instant
  global profile switch without reload, while text context switches and an
  existing voice binding remains intact.
- Client boundary: `docs/ARCHITECTURE_REQUIREMENTS.md` and the prior T-053
  plan require one global REST inbox reconciliation after the accepted realtime
  handoff and selected-chat-only history. The precise live assertion is
  `src/frontend/test/t055_profile_switch_reconnect_inbox_e2e_live_test.dart`.
- CI policy: `docs/TESTING.md` defines Flutter live opt-in with
  `VOICE_RUN_LIVE_INTEGRATION=true`, `VOICE_API_BASE_URL`, compose E2E tiers,
  and `make ci-script-tests` for CI path-filter contracts. `docs/PLAN.md`
  lists multi-profile as `core-live` with regression coverage still required.
- Existing patterns: `scripts/ci/compose-a1-multi-account-proof.sh` owns an
  isolated project, 17 adjacent ports, zero-byte explicit env-file, health
  waits, diagnostics, and exact-project cleanup. Its static test and
  `scripts/ci/compose-a1-ci-reachability_test.sh` are dependency-free Bash
  contracts. `.github/ci/e2e-features.yml` is parsed by the intentionally
  line-oriented `scripts/ci/e2e-manifest.sh`.
- Constraints from the task: no Docker/network live run; use `apply_patch` for
  edits; preserve the merged T-052/T-053/T-055 behavior; keep the two logical
  commits local and linear (RED first, then GREEN).

## Scope

- In: a manifest section containing exactly the T-055 Dart file; parser and
  static runner/reachability tests; a Make target; a separate isolated runner;
  one `a1-flutter-profile-handoff` CI job; and the
  `src/backend/scripts/**` A1 path-filter regression.
- Out: changes to T-055 Dart behavior, generic Compose E2E behavior, Docker
  images, live Docker/network execution, PR/`ci-gate` scheduling, and any
  product/API contract change.
- Documentation gap: the CI slice itself is directed by this task and follows
  the already-documented Flutter/Compose conventions; no product semantic is
  invented.

## Acceptance Criteria

1. Static contracts reject a missing/drifted manifest section and assert the
   exact sole T-055 Dart path.
2. The runner owns a generated non-`voice` Compose project, an isolated
   17-port range, zero-byte `--env-file`, and rejects every ambient Compose
   selector before Compose starts; it waits for health, emits diagnostics on a
   Flutter failure, and cleanup is opt-in, exact-project, and without volumes.
3. The runner passes compile-time `VOICE_RUN_LIVE_INTEGRATION=true` and
   `VOICE_API_BASE_URL` to exactly one `flutter test` invocation with
   concurrency one, using the manifest-selected path.
4. `a1-flutter-profile-handoff` needs `changes`, uses Ubuntu and a 60-minute
   timeout, checkout v5, cached `subosito/flutter-action@v2` with the workflow
   Flutter version, SQLite host prefetch, the exact Make target, and cleanup
   true. It runs only on schedule, full dispatch, and filtered master push;
   never on PR or as a `ci-gate` dependency.
5. `src/backend/scripts/**` is an explicit `a1_e2e` path and its assertion
   protects against future removal.

## Milestones

- [ ] RED: add static parser/runner/workflow/path-filter contracts and prove
  their expected pre-implementation failure; obtain an independent test review
  and commit the accepted RED surface.
- [ ] GREEN: add the manifest, target, runner, workflow job, and filter path;
  run bounded static/YAML/shim checks and obtain independent implementation
  review.
- [ ] Final: update this plan with evidence, inspect the two commits/diff, and
  leave the worktree clean without push.

## Detailed Steps

1. Add an exact T-055 manifest/parser contract, a fake-tool runner contract,
   and reachability assertions including the new backend scripts path. Run the
   contracts before their production counterparts exist and record the RED
   failure. No workflow, manifest, Makefile, or runner implementation changes
   occur in this cycle.
2. Ask a fresh read-only reviewer to check the RED assertions for coverage,
   false positives, and accidental production edits. Address critical findings
   locally, then commit only plan/tests as the RED commit.
3. Add the smallest manifest/parser vocabulary extension and Make target, then
   the isolated runner patterned after the existing A1 runner. Use only exact
   paths and arguments asserted by the RED contract.
4. Add the separate job and A1 path filter. Run the focused static contracts
   after each coherent change; make no Docker/network call.
5. Run YAML parse and `make -n` checks plus bounded Bash contracts. Treat a
   Git Bash hang as environment evidence only, preserving the no-live-run
   constraint.
6. Ask a new read-only reviewer to verify the completed implementation against
   this plan, docs, accepted RED contracts, and the final diff. Resolve any
   critical finding locally and rerun affected checks.

## Validation

- [ ] `bash scripts/ci/e2e-manifest_test.sh` validates parser, manifest,
  workflow, and dedicated path-filter contracts without Docker/network.
- [ ] `bash scripts/ci/compose-a1-flutter-profile-handoff_test.sh` exercises
  the runner through fake `docker`, `curl`, and `flutter` shims only.
- [ ] `make -n compose-a1-flutter-profile-handoff` proves the exact isolated
  target.
- [ ] A local YAML parse verifies `.github/workflows/ci.yml`,
  `.github/ci/e2e-features.yml`, and `.github/ci/path-filters.yml` are valid.
- [ ] `git diff --check`, targeted status/diff review, and fresh review show
  no unrelated changes.

## Progress

- [x] Required project, TDD, git, CI/testing, feature, and existing-runner
  sources read; task matches the existing product and CI conventions.
- [x] RED contract authoring. The absent runner/job/parser/filter vocabulary is
  also confirmed by direct static search; the focused scripts cannot start in
  this host because `C:\\Windows\\System32\\bash.exe` has no WSL `/bin/bash`
  and spawned Git Bash children hang before shell execution. No Docker/network
  command was attempted.
- [x] RED review found and corrected one P1 (exact-project cleanup assertion)
  plus P2 coverage for delayed health readiness, exact trigger structure, and
  the `ci-script-tests` target boundary. Fresh re-review additionally required
  cleanup's identity to equal every Compose invocation; the RED contract now
  asserts its shared project/env-file/directory/file/profile tuple. A final
  review also required proof that the runner obtains the test path through the
  manifest parser rather than a hard-coded filename; the runner contract now
  requires that parser/section call explicitly. The subsequent review found
  the workflow reachability test itself could be dead code, so its RED contract
  now requires both new shell tests in the bounded `ci-script-tests` recipe.
  Final acceptance also requires a shimmed alternate parser result to become
  the sole Flutter test path, preventing a parser call plus hard-coded runner;
  the contract counts one Dart positional argument rather than merely requiring
  the alternate path among potentially several tests. Final review found the
  parser shim was not executable; it is now explicitly included in its local
  fixture `chmod` set.
- [ ] GREEN implementation and bounded verification.
- [ ] GREEN review, final checklist, and clean worktree.

## Decisions

- Keep T-055 outside generic smoke/full lists: it owns a distinct isolated
  project and must not contend with the developer stack, following the existing
  A1 proof pattern.
- Use static shell shims instead of a live run: the task explicitly prohibits
  Docker/network while the shims can prove command shape, environment ownership,
  diagnostics, and cleanup safety.
- The current writer owns every edit because the task explicitly designates one
  consolidated writer. Fresh agents are read-only review delegates, satisfying
  the independent-review requirement without shared-write conflicts.

## Risks And Follow-Ups

- Static contracts cannot prove a GitHub-hosted runner can build the full app
  stack or perform a real websocket handoff; CI will provide that evidence once
  the user chooses to push.
- Host Git Bash may hang around Bash process cleanup. Record that as local
  infrastructure behavior rather than broadening scope or launching Docker.
