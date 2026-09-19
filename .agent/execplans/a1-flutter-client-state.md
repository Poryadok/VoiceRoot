# ExecPlan: A1 Flutter client-state regressions

## Purpose

Make the documented daily-messaging transitions dependable: switching profile is
immediate without an application reload, switching an open chat does not expose
a transient full-pane flicker/stray line, and empty/error/offline presentation
does not replace already available user data.

## Context

- Docs: `docs/PLAN.md` A1, `docs/todo/client.md` (multi-profile and Chat UX),
  `docs/features/multi-profile.md`, `docs/features/text-chat.md`,
  `docs/features/navigation.md`, and `docs/TESTING.md`.
- Existing related work: `profile_context_controller`, chat providers and room
  UI, plus the shared `VoiceListSkeleton`/`VoiceStatePanel` patterns.
- Constraints: this is a Flutter controller/view-boundary slice. Persistent
  per-chat drafts, Penpot, and A2+ behavior are excluded.

## Scope

- In: smallest profile/chat state invalidation and rendering corrections, with
  focused widget/regression tests for the documented failures and relevant
  empty/error/offline outcomes.
- Out: backend/API contracts, profile creation design, draft persistence, and
  unrelated residual loader surfaces.
- Documentation gap: the TODO reports the visual chat-switch symptom but does
  not prescribe a specific implementation; existing Flutter testable behavior
  and state ownership will determine the minimal correction.

## Milestones

- [x] Record documentation-backed plan before production edits.
- [ ] RED: add focused regression tests and demonstrate they fail on current
  production code for the intended state-transition defect.
- [ ] GREEN: make only the controller/view changes necessary for those tests.
- [ ] Refactor/verify: inspect the final diff, run affected Flutter checks,
  obtain exact-head CI gate, and merge with a merge commit.

## Detailed Steps

1. Trace the current profile context, selected chat, room state, and
   empty/error/offline rendering seams; retain local drafts outside the change.
2. Add widget tests first for each documented externally visible behavior and
   run their focused command to capture RED evidence.
3. Make the smallest profile-keyed invalidation/fencing and/or room rendering
   update that turns each test GREEN. Do not weaken accepted tests.
4. Run the same focused tests after each change; refactor only while they stay
   green. Add no behavior outside the declared scope.
5. Run `flutter analyze`, affected Flutter tests and `make flutter-ci` as
   required by `docs/TESTING.md`; push, inspect GitHub exact-head checks, then
   merge only when the required `ci-gate` is green.

## Validation

- [ ] Focused new/neighboring `flutter test` commands prove the RED then GREEN
  transitions and profile/chat/state regressions.
- [ ] `cd src/frontend; flutter analyze`
- [ ] `make flutter-ci`
- [ ] `git diff --check` and final scoped diff inspection.
- [ ] GitHub exact-head required checks, including `ci-gate`, are green before
  `gh pr merge --merge`.

## Progress

- [x] Required documentation and prior profile-handoff plans read.
- [x] Clean working branch created from `origin/master`.
- [ ] Test-only author is preparing the initial RED contract in a separate
  lease; production implementation has not started.

## Decisions

- Use existing state ownership and widget test conventions rather than create a
  new cross-feature abstraction: docs specify user-visible behavior, not a new
  client architecture.
- Treat the observed chat visual symptom as a regression test target; select
  the implementation only after a focused test reliably demonstrates it.

## Risks And Follow-Ups

- A test may reveal the TODO symptom needs a device/integration reproduction
  unavailable in widget tests. In that case preserve the precise testable state
  contract, record the remaining visual proof limitation, and do not invent UI
  behavior.
- Local drafts remain their own documented persistence outcome.
