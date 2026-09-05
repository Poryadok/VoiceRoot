# ExecPlan: T-053 atomic profile switch — independent RED phase

## Purpose

Prepare executable Flutter RED contracts for an atomic profile switch: exactly
one persisted/committed replacement session, one WebSocket reconnect request,
one inbox-reconciliation request, stale async completions fenced by profile
generation, and no avatar side effects when switching a freshly created profile
fails. An active voice session stays bound to the identity that started it.

## Context

- Docs: `docs/features/multi-profile.md` (instant global switch; text context
  changes; an active voice session remains with its original profile),
  `docs/ARCHITECTURE_REQUIREMENTS.md` (one global REST inbox reconciliation
  after reconnect; selected-chat-only history),
  `docs/microservices/realtime-service.md` (no WS replay; subscriptions),
  `docs/PLAN.md` A1 (profile/chat switching regressions and global snapshot),
  `docs/TESTING.md`.
- Current client: `state/auth_providers.dart` persists and publishes a switched
  session directly; three profile UI entry points invoke it independently;
  `profile_context_controller.dart` reconnects then loads chats, while
  `ChatListController` also loads on every access-token change.
- T-049 owns the canonical fail-closed Auth-to-User service contract. T-052
  owns profile-keyed, paginated `main`/`requests`/`archive` reconciliation and
  selected-chat-only history. Neither implementation is copied here.

## Scope

- In: tests and test-only fixtures/interfaces for the T-053 orchestration
  contract; RED changes to existing create-profile and voice tests; this plan.
- Out: production orchestration, Auth/User RPC implementation, direct JDBC,
  snapshot pagination, Inbox reconciler implementation, production WS protocol
  changes, or a voice-service contract change.
- Documentation gap: the existing voice client retains `voiceBindingProfileId`,
  but all voice-control REST calls read the currently active JWT. The eventual
  implementation must prove how a switched-away original voice identity can
  still explicitly end/control its session.

## Milestones

- [x] Document contract and ownership boundaries.
- [x] Add focused independent RED tests/fixtures.
- [x] Static-test evidence captured; Flutter runtime evidence is environment-blocked.
- [x] Test-only review with no production edits (second review accepted).
- [ ] Commit/push/draft PR.

## Detailed Steps

1. Add a test-only atomic-switch coordinator contract/fake with a canonical
   Auth result and a narrow `reconcile(profileId, generation)` expectation.
2. Specify one successful commit, failed-switch preservation, and A→B→C stale
   completion fencing. The fake must make duplicated session writes, reconnects,
   or reconciliation calls observable.
3. Add a RED source-level contract for all four production surfaces
   (`ProfileSwitcher`, avatar menu, avatar switcher, create-profile sheet).
   requiring a shared `profileSwitchCoordinatorProvider`; no UI production code
   changes in this RED phase.
4. Extend the create-profile widget tests: if switch fails after successful
   creation, no avatar presign/upload/PATCH is called and the sheet remains
   open with an error.
5. Extend voice tests so changing active text profile preserves the original
   voice binding; record the control-credential gap as RED/follow-up rather
   than inventing a backend policy.
6. Run focused RED tests, then existing neighboring tests and `flutter analyze`.
   Do not add production code to turn RED green in this task.

## Validation

- [ ] `cd src/frontend; flutter test test/atomic_profile_switch_orchestrator_test.dart`
  — one deliberate RED (missing production adapter); reference-fixture cases
  execute controlled Auth/snapshot race assertions.
- [ ] `cd src/frontend; flutter test test/create_profile_sheet_test.dart test/profile_switcher_test.dart test/call_providers_test.dart`
  — existing baseline plus RED regression expectations.
- [ ] `cd src/frontend; flutter analyze`
- [ ] Inspect `git diff --check` and final diff; confirm only plan/tests/fixtures.

## Progress

- [x] Branch created from `origin/master`: `feature/flutter-atomic-profile-switch`.
- [x] Tests and fixtures added.
- [x] Static analysis: new/changed and neighbouring test files have no issues.
- [x] Flutter runtime attempt recorded: dependency/native asset setup does not
  complete in this worktree (`sqlite3mc.x64.windows.dll` absent).
- [x] Independent test review complete.
- [ ] Commit, push, and draft PR complete.

## Decisions

- T-053 calls a narrow T-052 reconciliation seam exactly once; the T-052
  pagination/retry algorithm remains outside this branch.
- T-049 is represented by a fake canonical Auth result, including failure;
  this branch does not preserve or test obsolete Auth JDBC implementation
  details.
- A created profile is not deleted when its subsequent switch fails: product
  docs prescribe backend archival on explicit delete only, not rollback of a
  successful creation.

## Risks And Follow-Ups

- An Auth server switch is irreversible from the client once the response is
  received; local atomicity means no stale local state or side effects, not a
  fabricated server rollback.
- T-052 must expose a stable enough seam for the one-call assertion before the
  production integration phase.
- The explicit voice-control authorization mechanism may require a follow-up
  with Voice/Auth owners if no existing original-profile capability exists.
- The production coordinator must re-check its generation after every awaited
  persistence and Realtime operation; the RED fixture covers late Auth and
  inbox completion, while a late side-effect continuation needs integration
  coverage when the real provider exists.
