# ExecPlan: Flutter global inbox reconnect reconciler

## Purpose

After a WebSocket reconnect, the Flutter client must rebuild the active profile's
authoritative chat-list state from Chat REST without hiding useful cached rows.
It fetches every page of `main`, `requests`, and `archive`, exposes each first
page immediately, and retries a failed later page from the same opaque cursor.
The global reconciler calls Chat `ListChats` only. The existing selected
`ChatRoomController` remains the sole owner of per-chat Messaging history. The
client must never treat Realtime `resume` as an inbox or message-history replay.

This first milestone intentionally stops at the RED review gate: only this plan,
test fixtures, and failing provider/widget tests are added. Production providers
and widgets are not changed until the tests are reviewed and accepted.

## Context

Documentation sources:

- `docs/PLAN.md` A1 and "Неподвижные архитектурные границы": A1 requires a
  reconnect proof across the three inboxes, while global state is reconciled by
  Chat `ListChats` and selected-chat history by Messaging cursor APIs.
- `docs/features/text-chat.md` "Статусы доставки" and "Технические решения": a
  reconnect requires a global paginated `main` / `requests` / `archive`
  snapshot; only the selected chat loads messages.
- `docs/features/navigation.md` "Message requests" and "Archived chats":
  requests and archive are distinct per-profile views.
- `docs/features/multi-profile.md` "Переключение": text state follows the active
  profile and must not leak between profiles.
- `docs/features/privacy.md`: policy and identity are profile-scoped; the
  reconciler must use the authenticated active profile rather than account-wide
  state.
- `docs/microservices/chat-service.md` "Reconnect: global inbox snapshot": all
  scopes paginate to completion, first pages may render early, unfinished scopes
  retain cached rows, and failed pages are retried.
- `docs/microservices/messaging-service.md` `GetMessages`: message catch-up is
  cursor-based and scoped to a `chat_id`.
- `docs/microservices/realtime-service.md` "Reconnect checklist (client)":
  Realtime has no durable inbox/history journal and does not replay old-session
  events through `resume`.
- `docs/ARCHITECTURE_REQUIREMENTS.md` "Reconnect / delivery": Chat REST owns
  global reconciliation and Messaging REST owns per-chat history.
- `docs/TESTING.md` "Порядок разработки (TDD)" and Flutter checks.
- `tmp/fleet/plans/A1-daily-messaging.md` T-052 dependency and scope.

Current code:

- `src/frontend/lib/state/chat_providers.dart` has separate, view-driven
  `ChatListController`, `ChatArchiveListController`, and per-chat
  `ChatRoomController`. Only `ChatRoomController` listens for reconnect and it
  fetches a delta for every mounted room provider.
- `ChatListController.loadInitial` loads only the currently selected inbox and
  does not guard overlapping requests by profile/generation.
- `ChatArchiveListController` is loaded only by the archive screen.
- `src/frontend/lib/state/profile_context_controller.dart` reloads only the
  current chat list after profile reconnect and leaves subscription cleanup to
  the later atomic-switch task T-053.
- `src/frontend/lib/ui/shell/chat_list_body.dart` preserves rows after a
  load-more failure but does not surface that failure alongside the rows.
- Existing `VoiceChatsClient.listChats` and `VoiceMessagesClient.getMessages`
  already expose the needed opaque cursors; no wire-contract change is needed.

The requested behavior matches the documentation. No product decision is
missing for this slice. Exact visual placement is deliberately not specified;
tests use existing localized error/retry language and stable widget keys.

## Scope

In scope:

- A single reconnect generation for one active `profile_id`.
- Independent paginated snapshots for `main`, `requests`, and `archive`.
- Progressive first-page commit and append-only commit of later pages.
- Exact-cursor retry after a later-page failure without clearing committed rows.
- Stale-generation rejection for overlapping reconnects and profile changes.
- Preservation of selected-chat-only Messaging history refresh through the
  existing `ChatRoomController`, with a negative assertion that the global
  reconciler makes zero `GetMessages` calls.
- Honest loading/offline/error/retry state while cached rows remain visible.
- Clearing or isolating T-052-owned selection/subscription inputs at a profile
  boundary so an old-profile chat cannot trigger history work. Full atomic
  switch sequencing, persistence, voice retention, and membership proof remain
  T-053.

Out of scope:

- Any server, protobuf, Gateway, Realtime, or database change.
- Global WebSocket history/event replay.
- Fetching message history for every inbox row.
- T-053's complete profile-switch orchestration.
- Visual redesign or new product copy.

## Acceptance Criteria

1. A reconnect triggers `ListChats` for all three scopes and follows each
   `next_cursor` until empty.
2. Each scope publishes its first page before its final page completes.
3. A failed later page leaves already committed rows and its failed cursor in
   state; retry calls that exact scope/cursor and continues from there.
4. If reconnect generation B starts while generation A is in flight, results
   from A cannot overwrite B.
5. Snapshot state is keyed by active `profile_id`; rows/errors/cursors from one
   profile are not observable as another profile's state.
6. The global reconciler makes zero `GetMessages` calls. Only the selected,
   mounted `ChatRoomController` requests `GetMessages`, using that chat's
   last-message cursor; no unselected inbox row starts message history.
7. Realtime reconnect/resume is only the trigger/live path: no test fake or
   production API models global WS history replay.
8. Offline or failed reconciliation is visible with a retry action while
   previously committed rows remain rendered.
9. On a profile boundary, stale selection and T-052-owned history/subscription
   work are cleared or ignored before old async results can commit. The stale
   generation cannot commit rows, cursors, errors, cache writes, or loading
   flags. T-053 will own the broader atomic switch.

## Test Strategy And Fixtures

- Add provider tests under `src/frontend/test/inbox_reconciler_test.dart`.
- Add widget-state tests under
  `src/frontend/test/inbox_reconciler_widget_test.dart`.
- Add a reusable scripted Chat/Messaging fake under
  `src/frontend/test/support/inbox_reconciler_fakes.dart`. It records profile,
  inbox, chat and cursor; pages may resolve immediately, fail, or be completed
  manually to deterministically test concurrency.
- Use `ProviderContainer`, the repository's authenticated auth override, and
  `realtimeAutoConnectProvider=false`; never open a real socket or make network
  requests.
- Provider assertions cover criteria 1–7 and 9, including two profiles that
  contain the same `chat_id`. Widget assertions distinguish initial failure
  from partial-page/offline failure and cover progressive render plus criterion
  8 using existing localized
  `chatListLoadError` / `commonRetry` semantics.
- Keep existing `chat_state_test.dart`, `chat_offline_ui_test.dart`,
  `chat_archive_screen_test.dart`, and `message_requests_providers_test.dart`
  unchanged as neighboring regression evidence.

## Red-Green-Refactor Milestones

- [x] Read documentation, current providers, clients, and existing tests.
- [x] Record this self-contained plan before production edits.
- [x] RED 1: tests require all-scope, all-page reconciliation with progressive
  first-page state and exact-cursor retry.
- [x] RED 2: tests require generation/profile isolation and stale-result discard.
- [x] RED 3: tests preserve selected-`ChatRoomController` Messaging catch-up and
  prove the global reconciler makes zero Messaging/global WS replay calls.
- [x] RED 4: widget tests require cached rows plus visible offline/error/retry.
- [x] RED 5: tests require old-profile selection/history work to be ignored;
  leave complete atomic switch behavior to T-053.
- [ ] Test review gate: a fresh reviewer checks documentation coverage, assertion
  quality, fixture realism, expected failures, and absence of production edits.
- [ ] GREEN 1: add a profile-keyed reconnect snapshot state/controller and make
  RED 1 pass with the smallest implementation.
- [ ] GREEN 2: add generation checks at every async commit and make RED 2 pass.
- [ ] GREEN 3: keep selected-chat cursor catch-up outside the global reconciler
  and make RED 3/5 pass without changing ChatRoom ownership.
- [ ] GREEN 4: bind existing list/archive UI to honest partial/error state and
  make RED 4 pass without introducing new copy.
- [ ] Refactor only after every focused group is green; then run Flutter CI.

## Intended Implementation Shape (after RED approval)

1. Introduce a dedicated reconciler in `state/chat_providers.dart` or a narrowly
   named `state/inbox_reconciler.dart`, with immutable state keyed by
   `profile_id` and inbox scope.
2. Listen to the `reconnecting -> connected` transition and capture the active
   profile, authorization, and a monotonically increasing generation. Do not
   capture or enumerate chat history work in this controller.
3. Run the three scope loops independently. Commit page 1 immediately; merge
   later pages by `chat_id`; only finalize removal/replacement semantics when a
   scope reaches an empty cursor. Preserve `failedCursor` on error.
4. Before every state mutation or cache write, compare generation and active
   profile. Discard stale rows, cursors, errors, writes, and loading-state
   completions without touching the current profile's state.
5. Leave Messaging catch-up in the existing selected `ChatRoomController`.
   The global controller must not enumerate inbox rows for history, call
   `GetMessages`, or add a Realtime replay API.
6. Expose partial loading/error/retry to `ChatListBody` and
   `ChatArchiveScreen`, preserving rendered rows and existing localized copy.
7. At profile change, invalidate T-052-owned selected-history work and ignore
   old generation completions. Coordinate the wider session/WS/subscription
   transaction in T-053 rather than expanding this change.

## Validation

- [ ] `flutter test test/inbox_reconciler_test.dart` — expected RED before
  production implementation; record exact diagnostics/assertions.
- [ ] `flutter test test/inbox_reconciler_widget_test.dart` — expected RED before
  production implementation; record exact diagnostics/assertions.
- [ ] `flutter test test/chat_state_test.dart test/chat_offline_ui_test.dart test/chat_archive_screen_test.dart test/message_requests_providers_test.dart`
  remains green to isolate the missing reconciler.
- [ ] `dart format --output=none --set-exit-if-changed test/inbox_reconciler_test.dart test/inbox_reconciler_widget_test.dart test/support/inbox_reconciler_fakes.dart`
- [ ] `flutter analyze` if the expected missing production contract does not make
  the signal redundant; otherwise record the expected RED diagnostics.
- [ ] `git diff --check`.

## Progress

- [x] Worktree lease and detached base `37fa468d` verified.
- [x] Created `feature/flutter-inbox-reconciler` without a local collision.
- [x] RED tests and test-only fakes written.
- [x] RED failures captured: provider suite cannot compile because the planned
  `lib/state/inbox_reconciler.dart`, `InboxScope`, and
  `inboxReconcilerProvider` do not exist; widget suite reaches assertions and
  fails because partial/offline state renders the cached row without the
  documented error/retry affordance.
- [ ] Independent test review has no critical findings.
- [ ] Commit, push, and draft PR attempted.

## Decisions

- Three inbox loops are independent: this is the smallest way to permit
  progressive rendering and scope-local retry without blocking healthy scopes.
- The failed cursor is durable state until that page succeeds: Chat cursors are
  opaque and restarting from page 1 would violate the explicit retry contract.
- Profile plus generation gates every async commit: authorization tokens alone
  do not prevent an old response from landing after a profile switch.
- Selected-chat history is a single REST concern. Realtime is not given an
  event-log abstraction because the architecture explicitly forbids it.
- Widget expectations reuse existing copy rather than inventing product text.

## Risks And Follow-Ups

- Existing per-chat `ChatRoomController` instances listen independently to
  reconnect. T-052 preserves selected-chat-only ownership and guards stale
  profile work; T-053 may need broader lifecycle cleanup for mounted providers
  without breaking normal live-event delta fetches.
- `RealtimeHub.reconnectWithNewSession` currently retains its subscription set.
  T-052 prevents stale selected-chat work from leaking, but T-053 must make the
  full profile switch (session, socket, subscriptions, selection) atomic.
- Request-summary counts currently fetch only one requests page; reconciling the
  visible inbox must not silently claim a complete summary until pagination is
  complete.
- A RED commit intentionally fails the new focused tests. It must remain a draft
  PR and must not be merged until the later GREEN milestone.
