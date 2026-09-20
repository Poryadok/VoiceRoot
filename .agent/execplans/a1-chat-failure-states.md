# ExecPlan: A1 Flutter chat failure-state safety

## Purpose

Keep the chat room honest when a history request fails: a user sees a localized,
actionable error instead of a raw backend string, while the existing loading,
offline-cache, retry, and profile-history ownership behaviour remains intact.

## Context

- Docs: `docs/PLAN.md` A1 DoD requires empty/error/offline states not to mask
  data loss; `docs/features/text-chat.md` defines inline recoverable chat
  errors; `docs/features/i18n.md` requires client localization by API error
  code; `docs/TESTING.md` requires TDD for changed behavior.
- Code: `src/frontend/lib/ui/chat/chat_room_panel.dart` renders a retry state
  for initial history failure; `src/frontend/lib/ui/api_error_messages.dart`
  currently lets unknown room failures flow through as raw text.
- Existing protections intentionally retained: `ChatRoomController` keeps
  cached/offline history and profile ownership fencing; `ChatListBody` keeps
  partial inbox rows and scope-local retry. No global WebSocket history replay
  is introduced.

## Scope

- In: safe chat-room raw-error mapping, a localized permission-denied state,
  focused unit/widget regression coverage, and truthful TODO progress.
- Out: social phone contacts, Auth recovery, attachment UI, generic residual
  loaders, Realtime/Gateway contracts, and any WebSocket catch-up.
- Documentation gap: no new interaction is invented. The existing retry action
  remains the recovery action; a stable localized permission denial is the
  client-side rendering of the documented error-code localization contract.

## Milestones

- [x] Documentation and current state seams traced; clean lease and branch made.
- [x] RED: unit tests demonstrate that raw upstream text is unsafe and that a
  permission denial has a localized room state.
- [x] GREEN: map those values without weakening recognized privacy mapping or
  backend-unavailable handling.
- [x] Refactor and focused Flutter verification complete; TODO reflects only
  residual non-chat surfaces.

## Detailed Steps

1. Add unit assertions for `chatRoomErrorMessage` before production edits:
   unknown text must not appear; `permission_denied` must render a stable
   localized denial; known privacy text and unavailable status stay unchanged.
2. Run the focused test and record the expected RED failure.
3. Add the smallest `api_error_messages.dart` mapping plus EN/RU ARB key(s),
   regenerate localizations if required, and rerun the same test GREEN.
4. Run neighboring chat offline/widget tests and analyzer. Update the client
   TODO only to make raw-error residual coverage actionable, without claiming
   unrelated loader completion.
5. Inspect diff, commit/push, open one PR, wait for exact-head `ci-gate`, merge
   with a merge commit, verify `master`, then return this exact Treehouse lease.

## Validation

- [ ] `cd src/frontend; flutter test --no-pub test/api_error_messages_test.dart`
- [ ] `cd src/frontend; flutter test --no-pub test/chat_offline_ui_test.dart test/chat_room_panel_test.dart`
- [ ] `cd src/frontend; flutter analyze`
- [ ] `git diff --check`, exact-head hosted `ci-gate`, and post-merge master check.

## Progress

- [x] Plan written before production edits.
- [x] RED test authored and observed failing: absent localized keys prevented
  compilation of the new assertions.
- [x] GREEN implementation and regression checks: focused mapping test and
  neighboring offline/room widget tests pass; `dart analyze` of changed paths
  is clean. Full `flutter analyze` exceeds the Windows command window after
  repeatedly resolving native assets, so hosted exact-head CI remains required.
- [ ] Hosted CI and merge lifecycle.

## Decisions

- Unknown server text is never displayed in the chat room: `docs/features/i18n.md`
  makes error-code localization the contract, and raw text could disclose an
  internal failure or be inconsistent across locales.
- Preserve specific recognized privacy denials because they are already mapped
  to safe localized user guidance; map `permission_denied` to a generic room
  denial instead of guessing a missing fine-grained ACL reason.

## Risks And Follow-Ups

- Fine-grained room permission copy requires an explicit backend error-code
  contract; this slice will not infer one.
- `docs/todo/client.md` residual loaders remain separate unless a changed chat
  surface proves they overlap this error mapping.
