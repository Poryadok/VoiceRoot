import 'dart:async';
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_client.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/guest_credentials_storage.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/state/inbox_reconciler.dart';
import 'package:voice_frontend/ui/chat/chat_archive_screen.dart';
import 'package:voice_frontend/ui/shell/chat_list_body.dart';

import 'support/gateway_test_client.dart';
import 'support/inbox_reconciler_fakes.dart';

void main() {
  group('T052 archive mutation reconciliation (RED)', () {
    testWidgets(
      'keeps a confirmed archive mutation when a stale archive snapshot completes',
      (tester) async {
        final chats = _ArchiveMutationChatsFake();
        _enqueueSnapshot(chats, archiveItems: const [], mainItems: ['chat-1']);
        _enqueueSnapshot(chats, archiveItems: const [], manualArchive: true);
        final auth = _AuthHarness();
        final container = _container(chats: chats, auth: auth);
        var containerDisposed = false;
        void disposeContainer() {
          if (containerDisposed) return;
          containerDisposed = true;
          container.dispose();
        }

        InboxChatCall? staleArchiveCall;
        Future<void>? staleReconcile;
        Object? bodyFailure;
        Object? cleanupFailure;
        StackTrace? cleanupStack;

        Future<void> runCleanup(FutureOr<void> Function() action) async {
          try {
            await action();
          } catch (error, stack) {
            cleanupFailure ??= error;
            cleanupStack ??= stack;
          }
        }

        try {
          final reconciler = container.read(inboxReconcilerProvider.notifier);
          await reconciler.reconcile();
          final initialArchive = container
              .read(inboxReconcilerProvider)
              .profileSnapshots['profile-a']![InboxScope.archive];
          expect(initialArchive.items, isEmpty);
          expect(initialArchive.isComplete, isTrue);

          staleReconcile = reconciler.reconcile();
          await tester.pump();
          staleArchiveCall = chats.findCall(inbox: 'archive', cursor: null);
          expect(staleArchiveCall, isNotNull);

          final main = container.read(chatListControllerProvider.notifier)
            ..state = ChatListState(
              profileId: 'profile-a',
              items: [inboxChatItem('chat-1')],
            );
          expect(await main.archiveChat('chat-1', archived: true), isNull);
          expect(chats.archiveCalls, [
            const _ArchiveCall(
              authorization: 'Bearer access-a',
              chatId: 'chat-1',
              archived: true,
            ),
          ]);

          final confirmedArchive = container
              .read(inboxReconcilerProvider)
              .profileSnapshots['profile-a']![InboxScope.archive];
          expect(
            confirmedArchive.items.map((item) => item.chatId),
            ['chat-1'],
            reason:
                'the confirmed ArchiveChat mutation must update the authoritative '
                'profile-scoped archive snapshot, not a legacy list',
          );
          expect(confirmedArchive.isComplete, isTrue);

          await tester.pumpWidget(_testApp(container));
          await tester.pump();
          expect(
            find.byKey(ChatArchiveScreen.tileKey('chat-1')),
            findsOneWidget,
          );

          await chats.completeCall(
            staleArchiveCall!,
            result: const ChatsApiOk(ChatListData(items: [])),
          );
          await staleReconcile;
          await tester.pump();

          final afterStaleArchive = container
              .read(inboxReconcilerProvider)
              .profileSnapshots['profile-a']![InboxScope.archive];
          expect(afterStaleArchive.items.map((item) => item.chatId), [
            'chat-1',
          ]);
          expect(afterStaleArchive.isComplete, isTrue);
          expect(
            find.byKey(ChatArchiveScreen.tileKey('chat-1')),
            findsOneWidget,
            reason:
                'a stale archive scope response for the same profile/session must not erase the confirmed archive mutation',
          );
          expect(chats.unmatchedCalls, isEmpty);
          expect(
            chats.calls.every(
              (call) =>
                  call.profileId == 'profile-a' &&
                  call.authorization == 'Bearer access-a',
            ),
            isTrue,
            reason:
                'archive reconciliation must stay scoped to the active session',
          );
        } catch (error) {
          bodyFailure = error;
          rethrow;
        } finally {
          await runCleanup(() => tester.pumpWidget(const SizedBox.shrink()));
          await runCleanup(() => tester.pump());
          await runCleanup(() async {
            for (final pendingArchiveCall
                in chats.calls
                    .where((call) => call.completer != null && !call.completed)
                    .toList()) {
              await chats.completeCall(
                pendingArchiveCall,
                result: const ChatsApiOk(ChatListData(items: [])),
              );
            }
          });
          final pendingReconcile = staleReconcile;
          if (pendingReconcile != null) {
            await runCleanup(() => pendingReconcile);
          }
          await runCleanup(disposeContainer);
          if (bodyFailure == null && cleanupFailure != null) {
            Error.throwWithStackTrace(cleanupFailure!, cleanupStack!);
          }
        }
      },
    );

    testWidgets(
      'archives a reconciler-owned visible main row when the legacy list is empty',
      (tester) async {
        await tester.binding.setSurfaceSize(const Size(800, 1200));
        var surfaceRestored = false;
        Future<void> restoreSurface() async {
          if (surfaceRestored) return;
          surfaceRestored = true;
          await tester.binding.setSurfaceSize(null);
        }

        final chats = _ArchiveMutationChatsFake();
        _enqueueSnapshot(chats, archiveItems: const [], mainItems: ['chat-1']);
        _enqueueSnapshot(chats, archiveItems: const [], manualArchive: true);
        final auth = _AuthHarness();
        final container = _container(chats: chats, auth: auth);
        var containerDisposed = false;
        void disposeContainer() {
          if (containerDisposed) return;
          containerDisposed = true;
          container.dispose();
        }

        InboxChatCall? staleArchiveCall;
        Future<void>? staleReconcile;
        Object? bodyFailure;
        Object? cleanupFailure;
        StackTrace? cleanupStack;

        Future<void> runCleanup(FutureOr<void> Function() action) async {
          try {
            await action();
          } catch (error, stack) {
            cleanupFailure ??= error;
            cleanupStack ??= stack;
          }
        }

        try {
          final reconciler = container.read(inboxReconcilerProvider.notifier);
          await reconciler.reconcile();
          final legacy = container.read(chatListControllerProvider.notifier)
            ..state = const ChatListState(profileId: 'profile-a');
          expect(legacy.state.items, isEmpty);

          await tester.pumpWidget(_mainTestApp(container));
          await tester.pump();
          expect(find.byKey(ChatListBody.tileKey('chat-1')), findsOneWidget);

          await tester.longPress(find.byKey(ChatListBody.tileKey('chat-1')));
          final archiveAction = find.byKey(
            ChatListBody.archiveActionKey('chat-1'),
          );
          await _pumpUntilVisible(tester, archiveAction);

          staleReconcile = reconciler.reconcile();
          await tester.pump();
          staleArchiveCall = chats.findCall(inbox: 'archive', cursor: null);
          expect(staleArchiveCall, isNotNull);

          await tester.ensureVisible(archiveAction);
          await tester.tap(archiveAction);
          await tester.pump();

          final afterArchive = container
              .read(inboxReconcilerProvider)
              .profileSnapshots['profile-a']!;
          expect(afterArchive[InboxScope.main].items, isEmpty);
          expect(
            afterArchive[InboxScope.archive].items.map((item) => item.chatId),
            ['chat-1'],
            reason:
                'the visible reconciler row, not legacy controller state, owns '
                'the ArchiveChat mutation payload',
          );

          await chats.completeCall(
            staleArchiveCall!,
            result: const ChatsApiOk(ChatListData(items: [])),
          );
          await staleReconcile;
          expect(
            container
                .read(inboxReconcilerProvider)
                .profileSnapshots['profile-a']![InboxScope.archive]
                .items
                .map((item) => item.chatId),
            ['chat-1'],
          );
        } catch (error) {
          bodyFailure = error;
          rethrow;
        } finally {
          await runCleanup(() => tester.pumpWidget(const SizedBox.shrink()));
          await runCleanup(() => tester.pump());
          await runCleanup(() async {
            for (final pendingArchiveCall
                in chats.calls
                    .where((call) => call.completer != null && !call.completed)
                    .toList()) {
              await chats.completeCall(
                pendingArchiveCall,
                result: const ChatsApiOk(ChatListData(items: [])),
              );
            }
          });
          final pendingReconcile = staleReconcile;
          if (pendingReconcile != null) {
            await runCleanup(() => pendingReconcile);
          }
          await runCleanup(disposeContainer);
          await runCleanup(restoreSurface);
          if (bodyFailure == null && cleanupFailure != null) {
            Error.throwWithStackTrace(cleanupFailure!, cleanupStack!);
          }
        }
      },
    );

    test(
      'retires archive protection after a current archive snapshot confirms it',
      () async {
        final chats = _ArchiveMutationChatsFake();
        _enqueueSnapshot(chats, archiveItems: const [], mainItems: ['chat-1']);
        final auth = _AuthHarness();
        final container = _container(chats: chats, auth: auth);
        addTearDown(container.dispose);
        final reconciler = container.read(inboxReconcilerProvider.notifier);
        await reconciler.reconcile();

        final main = container.read(chatListControllerProvider.notifier)
          ..state = ChatListState(
            profileId: 'profile-a',
            items: [inboxChatItem('chat-1')],
          );
        expect(await main.archiveChat('chat-1', archived: true), isNull);

        _enqueueSnapshot(chats, archiveItems: ['chat-1'], mainItems: const []);
        await reconciler.reconcile();
        expect(
          container
              .read(inboxReconcilerProvider)
              .profileSnapshots['profile-a']![InboxScope.archive]
              .items
              .map((item) => item.chatId),
          ['chat-1'],
        );

        _enqueueSnapshot(chats, archiveItems: const [], mainItems: const []);
        await reconciler.reconcile();
        expect(
          container
              .read(inboxReconcilerProvider)
              .profileSnapshots['profile-a']![InboxScope.archive]
              .items,
          isEmpty,
          reason:
              'a confirmed archive mutation must not protect rows after its '
              'current authoritative archive snapshot has succeeded',
        );
      },
    );

    test(
      'retires archive protection for an empty snapshot started after mutation',
      () async {
        final chats = _ArchiveMutationChatsFake();
        _enqueueSnapshot(chats, archiveItems: const [], mainItems: ['chat-1']);
        final auth = _AuthHarness();
        final container = _container(chats: chats, auth: auth);
        addTearDown(container.dispose);
        final reconciler = container.read(inboxReconcilerProvider.notifier);
        await reconciler.reconcile();

        final main = container.read(chatListControllerProvider.notifier)
          ..state = ChatListState(
            profileId: 'profile-a',
            items: [inboxChatItem('chat-1')],
          );
        expect(await main.archiveChat('chat-1', archived: true), isNull);

        _enqueueSnapshot(chats, archiveItems: const [], mainItems: const []);
        await reconciler.reconcile();
        expect(
          container
              .read(inboxReconcilerProvider)
              .profileSnapshots['profile-a']![InboxScope.archive]
              .items,
          isEmpty,
          reason:
              'only archive mutations newer than a request may protect its '
              'authoritative empty response',
        );
      },
    );

    testWidgets(
      'unarchive clears archive protection before a later empty snapshot',
      (tester) async {
        final chats = _ArchiveMutationChatsFake();
        _enqueueSnapshot(chats, archiveItems: const [], mainItems: ['chat-1']);
        final auth = _AuthHarness();
        final container = _container(chats: chats, auth: auth);
        addTearDown(container.dispose);
        final reconciler = container.read(inboxReconcilerProvider.notifier);
        await reconciler.reconcile();

        final main = container.read(chatListControllerProvider.notifier)
          ..state = ChatListState(
            profileId: 'profile-a',
            items: [inboxChatItem('chat-1')],
          );
        expect(await main.archiveChat('chat-1', archived: true), isNull);
        await tester.pumpWidget(_testApp(container));
        await tester.pumpAndSettle();
        expect(find.byKey(ChatArchiveScreen.tileKey('chat-1')), findsOneWidget);

        await tester.tap(find.text('Unarchive'));
        await tester.pumpAndSettle();
        expect(find.byKey(ChatArchiveScreen.tileKey('chat-1')), findsNothing);
        expect(
          chats.archiveCalls.last,
          const _ArchiveCall(
            authorization: 'Bearer access-a',
            chatId: 'chat-1',
            archived: false,
          ),
        );

        _enqueueSnapshot(chats, archiveItems: const [], mainItems: const []);
        await reconciler.reconcile();
        expect(
          container
              .read(inboxReconcilerProvider)
              .profileSnapshots['profile-a']![InboxScope.archive]
              .items,
          isEmpty,
          reason: 'unarchive must remove the archive mutation protection',
        );
      },
    );

    test(
      'unarchive moves the exact authoritative row from archive to main immediately',
      () async {
        final chats = _ArchiveMutationChatsFake();
        final authoritative = inboxChatItem(
          'chat-1',
          preview: 'authoritative archive preview',
          creatorProfileId: 'authoritative-peer',
        );
        for (final scope in InboxScope.values) {
          chats.enqueue(
            InboxChatPageScript(
              inbox: scope.name,
              cursor: null,
              profileId: 'profile-a',
              authorization: 'Bearer access-a',
              result: ChatsApiOk(
                ChatListData(
                  items: scope == InboxScope.archive
                      ? [authoritative]
                      : const [],
                ),
              ),
            ),
          );
        }
        // The real legacy main controller needs one initial response and a
        // second one only because current unarchive still reloads it. The
        // call-count assertion below makes that second response a regression,
        // not an unhandled-future artifact.
        for (var request = 0; request < 2; request++) {
          chats.enqueue(
            const InboxChatPageScript(
              inbox: 'main',
              cursor: null,
              profileId: 'profile-a',
              authorization: 'Bearer access-a',
              result: ChatsApiOk(ChatListData(items: [])),
            ),
          );
        }
        final auth = _AuthHarness();
        final container = _container(
          chats: chats,
          auth: auth,
          realArchiveController: true,
          realMainController: true,
        );
        addTearDown(container.dispose);

        final reconciler = container.read(inboxReconcilerProvider.notifier);
        await reconciler.reconcile();
        final archiveSubscription = container.listen<ChatListState>(
          chatArchiveListControllerProvider,
          (_, _) {},
          fireImmediately: true,
        );
        addTearDown(archiveSubscription.close);
        final archiveController =
            container.read(chatArchiveListControllerProvider.notifier)
              ..state = ChatListState(
                profileId: 'profile-a',
                items: [
                  inboxChatItem(
                    'chat-1',
                    preview: 'legacy archive preview',
                    creatorProfileId: 'legacy-peer',
                  ),
                ],
              );
        container.read(chatListControllerProvider);
        await pumpEventQueue();
        final listCallsBeforeUnarchive = chats.calls.length;

        expect(await archiveController.unarchiveChat('chat-1'), isNull);

        final snapshot = container
            .read(inboxReconcilerProvider)
            .profileSnapshots['profile-a']!;
        expect(snapshot[InboxScope.archive].items, isEmpty);
        expect(
          snapshot[InboxScope.main].items,
          [same(authoritative)],
          reason:
              'the authoritative main scope must receive the exact confirmed '
              'unarchived archive row, not the legacy controller copy, without '
              'waiting for reconnect',
        );
        expect(
          chats.calls,
          hasLength(listCallsBeforeUnarchive),
          reason:
              'a confirmed unarchive updates its two authoritative scopes; it '
              'must not launch a full or unrelated ListChats snapshot',
        );
        expect(
          chats.archiveCalls.last,
          const _ArchiveCall(
            authorization: 'Bearer access-a',
            chatId: 'chat-1',
            archived: false,
          ),
        );
      },
    );

    test(
      'unarchive keeps the authoritative main row after an older first page completes',
      () async {
        final chats = _ArchiveMutationChatsFake();
        final authoritative = inboxChatItem(
          'chat-1',
          preview: 'authoritative archive preview',
          creatorProfileId: 'authoritative-peer',
        );
        for (final scope in InboxScope.values) {
          chats.enqueue(
            InboxChatPageScript(
              inbox: scope.name,
              cursor: null,
              profileId: 'profile-a',
              authorization: 'Bearer access-a',
              result: ChatsApiOk(
                ChatListData(
                  items: scope == InboxScope.archive
                      ? [authoritative]
                      : const [],
                ),
              ),
            ),
          );
        }
        // Begin a second, deliberately older main first page before the
        // mutation. Archive keeps its current row so the unarchive action can
        // still obtain the authoritative object from the reconciler snapshot.
        for (final scope in InboxScope.values) {
          chats.enqueue(
            InboxChatPageScript(
              inbox: scope.name,
              cursor: null,
              profileId: 'profile-a',
              authorization: 'Bearer access-a',
              manual: scope == InboxScope.main,
              result: ChatsApiOk(
                ChatListData(
                  items: scope == InboxScope.archive
                      ? [authoritative]
                      : const [],
                ),
              ),
            ),
          );
        }
        final auth = _AuthHarness();
        final container = _container(
          chats: chats,
          auth: auth,
          realArchiveController: true,
        );
        addTearDown(container.dispose);

        final reconciler = container.read(inboxReconcilerProvider.notifier);
        await reconciler.reconcile();
        final archiveSubscription = container.listen<ChatListState>(
          chatArchiveListControllerProvider,
          (_, _) {},
          fireImmediately: true,
        );
        addTearDown(archiveSubscription.close);
        final archiveController =
            container.read(chatArchiveListControllerProvider.notifier)
              ..state = ChatListState(
                profileId: 'profile-a',
                items: [
                  inboxChatItem(
                    'chat-1',
                    preview: 'legacy archive preview',
                    creatorProfileId: 'legacy-peer',
                  ),
                ],
              );

        final staleReconcile = reconciler.reconcile();
        final staleMainCall = chats.findCall(
          inbox: 'main',
          cursor: null,
          profileId: 'profile-a',
          authorization: 'Bearer access-a',
        );
        expect(staleMainCall, isNotNull);
        await pumpEventQueue();
        final callsBeforeUnarchive = chats.calls.length;

        expect(await archiveController.unarchiveChat('chat-1'), isNull);
        var snapshot = container
            .read(inboxReconcilerProvider)
            .profileSnapshots['profile-a']!;
        expect(snapshot[InboxScope.archive].items, isEmpty);
        expect(snapshot[InboxScope.main].items, [same(authoritative)]);
        expect(chats.calls, hasLength(callsBeforeUnarchive));

        await chats.completeCall(
          staleMainCall!,
          result: const ChatsApiOk(ChatListData(items: [])),
        );
        await staleReconcile;

        snapshot = container
            .read(inboxReconcilerProvider)
            .profileSnapshots['profile-a']!;
        expect(snapshot[InboxScope.archive].items, isEmpty);
        expect(
          snapshot[InboxScope.main].items,
          [same(authoritative)],
          reason:
              'an older empty first main page must not erase the confirmed '
              'authoritative unarchive handoff',
        );
        expect(
          chats.calls,
          hasLength(callsBeforeUnarchive),
          reason:
              'unarchive must not trigger a reconnect or unrelated full '
              'ListChats request',
        );
        expect(chats.unmatchedCalls, isEmpty);
      },
    );

    testWidgets(
      'does not apply a successful stale A archive mutation to B snapshots or archive UI',
      (tester) async {
        final chats = _ArchiveMutationChatsFake(
          profileByAuthorization: const {
            'Bearer access-a': 'profile-a',
            'Bearer access-b': 'profile-b',
          },
        );
        final archiveCompletion = Completer<ChatsApiResult<void>>();
        chats.deferredArchive = archiveCompletion;
        final auth = _AuthHarness();
        final container = _container(chats: chats, auth: auth);
        addTearDown(container.dispose);
        addTearDown(() async {
          if (!archiveCompletion.isCompleted) {
            archiveCompletion.complete(const ChatsApiOk(null));
          }
        });

        final main = container.read(chatListControllerProvider.notifier)
          ..state = ChatListState(
            profileId: 'profile-a',
            items: [inboxChatItem('a-chat')],
          );
        final pendingArchive = main.archiveChat('a-chat', archived: true);
        await tester.pump();
        expect(chats.archiveCalls, [
          const _ArchiveCall(
            authorization: 'Bearer access-a',
            chatId: 'a-chat',
            archived: true,
          ),
        ]);

        expect(await auth.controller.switchActiveProfile('profile-b'), isNull);
        expect(
          container.read(authControllerProvider).session?.authorizationHeader,
          'Bearer access-b',
        );

        _enqueueSnapshot(
          chats,
          profileId: 'profile-b',
          authorization: 'Bearer access-b',
          mainItems: ['b-main'],
          archiveItems: ['b-archive'],
        );
        await container.read(inboxReconcilerProvider.notifier).reconcile();
        await tester.pumpWidget(_testApp(container));
        await tester.pump();

        _expectProfileBSnapshots(container);
        expect(find.byKey(ChatArchiveScreen.tileKey('a-chat')), findsNothing);
        expect(
          find.byKey(ChatArchiveScreen.tileKey('b-archive')),
          findsOneWidget,
        );

        archiveCompletion.complete(const ChatsApiOk(null));
        expect(await pendingArchive, kChatActionStaleContext);
        await tester.pump();

        _expectProfileBSnapshots(container);
        expect(find.byKey(ChatArchiveScreen.tileKey('a-chat')), findsNothing);
        expect(
          find.byKey(ChatArchiveScreen.tileKey('b-archive')),
          findsOneWidget,
        );
        expect(chats.unmatchedCalls, isEmpty);
        container.dispose();
      },
    );
  });
}

void _expectProfileBSnapshots(ProviderContainer container) {
  final snapshots = container.read(inboxReconcilerProvider).profileSnapshots;
  final profileB = snapshots['profile-b']!;
  expect(profileB[InboxScope.main].items.map((item) => item.chatId), [
    'b-main',
  ]);
  expect(profileB[InboxScope.archive].items.map((item) => item.chatId), [
    'b-archive',
  ]);
  expect(profileB[InboxScope.main].isComplete, isTrue);
  expect(profileB[InboxScope.archive].isComplete, isTrue);
}

Widget _testApp(ProviderContainer container) {
  return UncontrolledProviderScope(
    container: container,
    child: MaterialApp(
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      home: const ChatArchiveScreen(),
    ),
  );
}

Widget _mainTestApp(ProviderContainer container) {
  return UncontrolledProviderScope(
    container: container,
    child: MaterialApp(
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      home: const Scaffold(body: ChatListBody(showHeader: false)),
    ),
  );
}

Future<void> _pumpUntilVisible(WidgetTester tester, Finder finder) async {
  const frame = Duration(milliseconds: 16);
  for (var frameCount = 0; frameCount < 24; frameCount++) {
    await tester.pump(frame);
    if (finder.evaluate().isEmpty) continue;
    final rect = tester.getRect(finder);
    final viewport = tester.binding.renderViews.single.size;
    if (rect.top >= 0 && rect.bottom <= viewport.height) return;
  }
  throw TestFailure('archive action sheet did not become visible');
}

ProviderContainer _container({
  required _ArchiveMutationChatsFake chats,
  required _AuthHarness auth,
  bool realArchiveController = false,
  bool realMainController = false,
}) {
  return ProviderContainer(
    overrides: [
      authSessionStorageProvider.overrideWithValue(
        InMemoryAuthSessionStorage(),
      ),
      authControllerProvider.overrideWith((ref) => auth.controller),
      gatewayConfigProvider.overrideWithValue(
        const GatewayConfig(baseUrl: 'http://api.test'),
      ),
      httpClientProvider.overrideWithValue(
        MockClient((_) async => http.Response('{}', 404)),
      ),
      voiceChatsClientProvider.overrideWithValue(chats),
      realtimeAutoConnectProvider.overrideWithValue(false),
      realtimeLinkStatusProvider.overrideWith(
        (ref) => RealtimeLinkStatus.connected,
      ),
      if (!realMainController)
        chatListControllerProvider.overrideWith(_NoAutoChatListController.new),
      if (!realArchiveController)
        chatArchiveListControllerProvider.overrideWith(
          _NoAutoArchiveListController.new,
        ),
    ],
  );
}

class _NoAutoChatListController extends ChatListController {
  _NoAutoChatListController(super.ref);

  @override
  Future<void> loadInitial() async {}
}

class _NoAutoArchiveListController extends ChatArchiveListController {
  _NoAutoArchiveListController(super.ref);

  @override
  Future<void> loadInitial() async {}
}

void _enqueueSnapshot(
  _ArchiveMutationChatsFake chats, {
  required List<String> archiveItems,
  List<String> mainItems = const [],
  bool manualArchive = false,
  String profileId = 'profile-a',
  String authorization = 'Bearer access-a',
}) {
  for (final scope in InboxScope.values) {
    final ids = switch (scope) {
      InboxScope.main => mainItems,
      InboxScope.archive => archiveItems,
      InboxScope.requests => const <String>[],
    };
    chats.enqueue(
      InboxChatPageScript(
        inbox: scope.name,
        cursor: null,
        profileId: profileId,
        authorization: authorization,
        manual: scope == InboxScope.archive && manualArchive,
        result: ChatsApiOk(
          ChatListData(items: [for (final id in ids) inboxChatItem(id)]),
        ),
      ),
    );
  }
}

class _ArchiveMutationChatsFake extends InboxReconcilerChatsFake {
  _ArchiveMutationChatsFake({
    super.profileByAuthorization = const {'Bearer access-a': 'profile-a'},
  });

  final List<_ArchiveCall> archiveCalls = [];
  Completer<ChatsApiResult<void>>? deferredArchive;

  @override
  Future<ChatsApiResult<void>> archiveChat({
    required String authorization,
    required String chatId,
    required bool archived,
  }) {
    archiveCalls.add(
      _ArchiveCall(
        authorization: authorization,
        chatId: chatId,
        archived: archived,
      ),
    );
    final pending = deferredArchive;
    return pending?.future ?? Future.value(const ChatsApiOk(null));
  }
}

class _ArchiveCall {
  const _ArchiveCall({
    required this.authorization,
    required this.chatId,
    required this.archived,
  });

  final String authorization;
  final String chatId;
  final bool archived;

  @override
  bool operator ==(Object other) {
    return other is _ArchiveCall &&
        other.authorization == authorization &&
        other.chatId == chatId &&
        other.archived == archived;
  }

  @override
  int get hashCode => Object.hash(authorization, chatId, archived);
}

class _AuthHarness {
  _AuthHarness() {
    final mock = MockClient((request) async {
      if (request.url.path != '/api/v1/auth/switch-profile') {
        return http.Response('not found', 404);
      }
      expect(request.method, 'POST');
      expect(request.headers['authorization'], 'Bearer access-a');
      expect(jsonDecode(request.body), containsPair('profile_id', 'profile-b'));
      return utf8JsonResponse(
        jsonEncode({
          'access_token': 'access-b',
          'refresh_token': 'refresh-b',
          'account_id': 'account-1',
          'profile_id': 'profile-b',
          'expires_in_seconds': 900,
        }),
      );
    });
    controller =
        AuthController(
            authClient: VoiceAuthClient(gateway: gatewayHttpForTest(mock)),
            storage: InMemoryAuthSessionStorage(),
            guestCredentialsStorage: InMemoryGuestCredentialsStorage(),
          )
          ..state = const AuthState(
            session: AuthSession(
              accessToken: 'access-a',
              refreshToken: 'refresh-a',
              accountId: 'account-1',
              activeProfileId: 'profile-a',
              expiresInSeconds: 900,
            ),
          );
  }

  late final AuthController controller;
}
