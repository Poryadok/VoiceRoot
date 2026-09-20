# ExecPlan: A5 session-restore technical shell

## Purpose

Make the Web/Windows bootstrap fail visibly and recoverably if local session
restoration itself throws, rather than leaving the user on an indefinite loading
screen. Preserve the existing persisted-refresh flow and the authenticated shell
only after restoration completes.

## Context

- Docs: `docs/PLAN.md` A5 requires cold-start session restoration and honest
  loading/error/permission states. `docs/features/auth-and-contacts.md` defines
  device-local persisted sessions. `docs/ARCHITECTURE_REQUIREMENTS.md` keeps
  reconnect inbox reconciliation REST-authoritative and per-chat history scoped.
  `docs/features/accessibility.md` defines the existing global shortcuts and
  Tab/Shift+Tab reachability.
- Code: `bootstrap/voice_app_bootstrap.dart` awaits `AuthController.restore()`
  before mounting `VoiceApp`; a thrown storage or bootstrap dependency error
  currently leaves `_restoreComplete` false forever. `AuthController.restore()`
  already covers normal refresh failures. `InboxReconcilerController` already
  fences profile-bound reconnect snapshots and must not be duplicated.
- Constraints: no backend changes, no global WS catch-up, no host runtime, no
  A6 media or tray/PTT scope, and no duplicate of merged draft PR #409.

## Scope

- In: bootstrap restore failure state, retry action, and focused widget tests.
- Out: changing successful refresh semantics, reconnect protocol, navigation
  shortcuts, private-Space ACLs, and platform integrations.
- Documentation gap: no new shortcut binding is specified for archive/profile/
  Space; existing controls remain reachable through documented Tab order.

## Milestones

- [x] Add a red widget test for a thrown session-storage restore and retry.
- [x] Add the smallest bootstrap state transition and accessible retry UI.
- [ ] Run focused Flutter tests, formatter, analyzer, graph update, hosted CI,
  then merge the exact-head PR through ci-gate.

## Detailed Steps

1. Add a test-only `AuthSessionStorage` whose first `read` throws and whose
   retry succeeds; assert loading is replaced by an error panel and `Try again`
   reaches the auth screen.
2. Run the focused test to establish the red failure.
3. Track restore-in-flight and caught error in `VoiceAppBootstrap`; expose
   `VoiceStatePanel` with the existing localized unavailable/retry labels, and
   retry the same restore operation without mounting the shell prematurely.
4. Run the focused tests after the green change, format, analyze the affected
   Flutter package, inspect the diff, update Graphify, then commit/push/PR.

## Validation

- [x] `flutter test test/session_restore_bootstrap_test.dart`
- [x] `flutter test test/auth_controller_test.dart test/voice_shortcuts_keyboard_test.dart`
- [x] `dart format --output=none lib/bootstrap/voice_app_bootstrap.dart test/session_restore_bootstrap_test.dart`
- [x] `flutter analyze` (four pre-existing infos outside this change)
- [ ] Hosted exact-head CI and ci-gate pass before merge.

## Progress

- [x] Documentation and existing reconnect/session behavior inspected.
- [x] Red test written and observed (pre-change it remained on the restoring UI).
- [x] Bootstrap implementation complete.
- [x] Local verification complete.
- [ ] Hosted exact-head CI and ci-gate merge.

## Decisions

- Catch only bootstrap-level exceptions: normal authentication refresh failures
  remain governed by `AuthController` so stored sessions continue to support
  offline recovery as already implemented.
- Reuse the existing generic unavailable/retry localization to avoid an
  unreviewed product copy decision.

## Risks And Follow-Ups

- Hosted CI is required for final merge; no local Windows runtime/Compose test
  will be started for this client-only change.
