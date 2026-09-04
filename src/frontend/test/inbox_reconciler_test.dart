import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_client.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/guest_credentials_storage.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/messages_client.dart';
import 'package:voice_frontend/backend/realtime_client.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/inbox_reconciler.dart';
import 'package:voice_frontend/state/gateway_providers.dart';

import 'support/auth_test_overrides.dart';
import 'support/gateway_test_client.dart';
import 'support/inbox_reconciler_fakes.dart';

void main() {
  group('InboxReconcilerController', () {
    test('starts the three-scope snapshot after realtime reconnects', () async {
      final chats = InboxReconcilerChatsFake();
      for (final inbox in ['main', 'requests', 'archive']) {
        for (var run = 0; run < 2; run++) {
          chats.enqueue(
            InboxChatPageScript(
              inbox: inbox,
              cursor: null,
              result: const ChatsApiOk(ChatListData(items: [])),
            ),
          );
        }
      }
      final container = _container(
        chats: chats,
        messages: InboxReconcilerMessagesFake(),
      );
      addTearDown(container.dispose);
      container.read(inboxReconcilerProvider);
      await pumpEventQueue();
      final callsBeforeReconnect = chats.calls.length;

      container.read(realtimeLinkStatusProvider.notifier).state =
          RealtimeLinkStatus.reconnecting;
      container.read(realtimeLinkStatusProvider.notifier).state =
          RealtimeLinkStatus.connected;
      await pumpEventQueue();

      expect(chats.calls, hasLength(callsBeforeReconnect + 3));
      expect(
        chats.calls
            .skip(callsBeforeReconnect)
            .map((call) => call.inbox)
            .toSet(),
        {'main', 'requests', 'archive'},
      );
    });

    test(
      'reconciles all inboxes to completion and publishes first pages early',
      () async {
        final chats = _allScopesWithTwoPages();
        final messages = InboxReconcilerMessagesFake();
        final container = _container(chats: chats, messages: messages);
        addTearDown(container.dispose);

        final controller = container.read(inboxReconcilerProvider.notifier);
        final done = controller.reconcile();
        await pumpEventQueue();

        expect(chats.calls.map((call) => call.inbox).toSet(), {
          'main',
          'requests',
          'archive',
        });
        final progressive = container
            .read(inboxReconcilerProvider)
            .profileSnapshots['prof-test']!;
        for (final scope in InboxScope.values) {
          final scopeState = progressive.scopes[scope]!;
          expect(scopeState.items.map((item) => item.chatId), [
            '${scope.name}-page-1',
          ]);
          expect(scopeState.nextCursor, '${scope.name}-cursor-2');
          expect(scopeState.failedCursor, isNull);
          expect(scopeState.errorMessage, isNull);
          expect(scopeState.isLoading, isTrue);
          expect(scopeState.isComplete, isFalse);
        }

        expect(
          chats.calls
              .where((call) => call.cursor != null)
              .map((call) => '${call.inbox}:${call.cursor}')
              .toSet(),
          {
            'main:main-cursor-2',
            'requests:requests-cursor-2',
            'archive:archive-cursor-2',
          },
        );
        for (final inbox in ['main', 'requests', 'archive']) {
          final scopeCalls = chats.calls.where((call) => call.inbox == inbox);
          expect(scopeCalls.map((call) => call.cursor), [
            null,
            '$inbox-cursor-2',
          ]);
          expect(
            scopeCalls.every(
              (call) => call.authorization == 'Bearer test-access',
            ),
            isTrue,
          );
          expect(
            scopeCalls.every((call) => call.profileId == 'prof-test'),
            isTrue,
          );
          expect(scopeCalls.every((call) => call.pageSize == null), isTrue);
          expect(scopeCalls.every((call) => call.folderId == null), isTrue);
          expect(scopeCalls.every((call) => call.inbox == inbox), isTrue);
        }

        for (final inbox in ['main', 'requests', 'archive']) {
          final call = chats.findCall(
            inbox: inbox,
            cursor: '${inbox}-cursor-2',
          );
          expect(call, isNotNull);
          await chats.completeCall(
            call!,
            result: ChatsApiOk(
              ChatListData(items: [inboxChatItem('$inbox-page-2')]),
            ),
          );
        }
        await done;

        final state = container.read(inboxReconcilerProvider);
        for (final scope in InboxScope.values) {
          final scopeState =
              state.profileSnapshots['prof-test']!.scopes[scope]!;
          expect(scopeState.isComplete, isTrue);
          expect(scopeState.isLoading, isFalse);
          expect(scopeState.nextCursor, isNull);
          expect(scopeState.failedCursor, isNull);
          expect(scopeState.items.map((item) => item.chatId), [
            '${scope.name}-page-1',
            '${scope.name}-page-2',
          ]);
        }
        expect(chats.calls, hasLength(6));
        expect(chats.unmatchedCalls, isEmpty);
        expect(chats.pendingScripts, 0);
        expect(
          messages.getCalls,
          isEmpty,
          reason: 'global inbox reconciliation must not load message history',
        );
      },
    );

    test(
      'each scope keeps cached rows and retries only its exact failed cursor',
      () async {
        for (final failedScope in InboxScope.values) {
          final failedInbox = failedScope.name;
          final failedCursor = 'opaque-$failedInbox-2';
          final chats = InboxReconcilerChatsFake();
          for (final inbox in ['main', 'requests', 'archive']) {
            chats.enqueue(
              InboxChatPageScript(
                inbox: inbox,
                cursor: null,
                result: ChatsApiOk(
                  ChatListData(
                    items: [inboxChatItem('cached-$inbox')],
                    nextCursor: inbox == failedInbox ? failedCursor : null,
                  ),
                ),
              ),
            );
          }
          chats
            ..enqueue(
              InboxChatPageScript(
                inbox: failedInbox,
                cursor: failedCursor,
                result: const ChatsApiFailure(
                  message: 'later page unavailable',
                  statusCode: 503,
                ),
              ),
            )
            ..enqueue(
              InboxChatPageScript(
                inbox: failedInbox,
                cursor: failedCursor,
                result: ChatsApiOk(
                  ChatListData(
                    items: [inboxChatItem('after-retry-$failedInbox')],
                  ),
                ),
              ),
            );
          final container = _container(
            chats: chats,
            messages: InboxReconcilerMessagesFake(),
          );
          final controller = container.read(inboxReconcilerProvider.notifier);

          await controller.reconcile();
          var scope = container
              .read(inboxReconcilerProvider)
              .profileSnapshots['prof-test']!
              .scopes[failedScope]!;
          expect(scope.items.map((item) => item.chatId), [
            'cached-$failedInbox',
          ]);
          expect(scope.failedCursor, failedCursor);
          expect(scope.nextCursor, failedCursor);
          expect(scope.errorMessage, 'later page unavailable');
          expect(scope.isLoading, isFalse);
          expect(scope.isComplete, isFalse);
          final callsBeforeRetry = chats.calls.length;

          await controller.retry(failedScope);

          scope = container
              .read(inboxReconcilerProvider)
              .profileSnapshots['prof-test']!
              .scopes[failedScope]!;
          expect(scope.items.map((item) => item.chatId), [
            'cached-$failedInbox',
            'after-retry-$failedInbox',
          ]);
          expect(scope.failedCursor, isNull);
          expect(scope.nextCursor, isNull);
          expect(scope.errorMessage, isNull);
          expect(scope.isLoading, isFalse);
          expect(scope.isComplete, isTrue);
          expect(chats.calls, hasLength(callsBeforeRetry + 1));
          final retryCall = chats.calls.last;
          expect(retryCall.inbox, failedInbox);
          expect(retryCall.cursor, failedCursor);
          expect(retryCall.authorization, 'Bearer test-access');
          expect(retryCall.profileId, 'prof-test');
          expect(retryCall.pageSize, isNull);
          expect(retryCall.folderId, isNull);
          expect(chats.unmatchedCalls, isEmpty);
          expect(chats.pendingScripts, 0);
          container.dispose();
        }
      },
    );

    test(
      'merges later pages by chat id and keeps newest row metadata',
      () async {
        final chats = InboxReconcilerChatsFake();
        for (final inbox in ['main', 'requests', 'archive']) {
          chats
            ..enqueue(
              InboxChatPageScript(
                inbox: inbox,
                cursor: null,
                result: ChatsApiOk(
                  ChatListData(
                    items: [
                      inboxChatItem(
                        'duplicate-$inbox',
                        preview: 'old',
                        unreadCount: 1,
                      ),
                    ],
                    nextCursor: '$inbox-next',
                  ),
                ),
              ),
            )
            ..enqueue(
              InboxChatPageScript(
                inbox: inbox,
                cursor: '$inbox-next',
                result: ChatsApiOk(
                  ChatListData(
                    items: [
                      inboxChatItem(
                        'duplicate-$inbox',
                        preview: 'new',
                        unreadCount: 7,
                      ),
                      inboxChatItem('new-$inbox'),
                    ],
                  ),
                ),
              ),
            );
        }
        final container = _container(
          chats: chats,
          messages: InboxReconcilerMessagesFake(),
        );
        addTearDown(container.dispose);

        await container.read(inboxReconcilerProvider.notifier).reconcile();
        final state = container.read(inboxReconcilerProvider);
        for (final scope in InboxScope.values) {
          final rows =
              state.profileSnapshots['prof-test']!.scopes[scope]!.items;
          expect(
            rows.where((item) => item.chatId == 'duplicate-${scope.name}'),
            hasLength(1),
          );
          final duplicate = rows.singleWhere(
            (item) => item.chatId == 'duplicate-${scope.name}',
          );
          expect(duplicate.lastMessagePreview, 'new');
          expect(duplicate.unreadCount, 7);
          expect(
            rows.map((item) => item.chatId),
            contains('new-${scope.name}'),
          );
        }
      },
    );

    test('isolates a later-page failure to its own inbox scope', () async {
      for (final failedInbox in ['main', 'requests', 'archive']) {
        final chats = InboxReconcilerChatsFake();
        for (final inbox in ['main', 'requests', 'archive']) {
          chats.enqueue(
            InboxChatPageScript(
              inbox: inbox,
              cursor: null,
              result: ChatsApiOk(
                ChatListData(
                  items: [inboxChatItem('cached-$inbox')],
                  nextCursor: '$inbox-later',
                ),
              ),
            ),
          );
          chats.enqueue(
            InboxChatPageScript(
              inbox: inbox,
              cursor: '$inbox-later',
              result: inbox == failedInbox
                  ? const ChatsApiFailure(
                      message: 'later page unavailable',
                      statusCode: 503,
                    )
                  : ChatsApiOk(
                      ChatListData(items: [inboxChatItem('fresh-$inbox')]),
                    ),
            ),
          );
        }
        final container = _container(
          chats: chats,
          messages: InboxReconcilerMessagesFake(),
        );
        addTearDown(container.dispose);
        await container.read(inboxReconcilerProvider.notifier).reconcile();

        final state = container.read(inboxReconcilerProvider);
        for (final scope in InboxScope.values) {
          final current = state.profileSnapshots['prof-test']!.scopes[scope]!;
          expect(current.items, isNotEmpty);
          if (scope.name == failedInbox) {
            expect(current.items.single.chatId, 'cached-$failedInbox');
            expect(current.failedCursor, '$failedInbox-later');
            expect(current.errorMessage, 'later page unavailable');
            expect(current.isComplete, isFalse);
          } else {
            expect(
              current.items.map((item) => item.chatId),
              contains('fresh-${scope.name}'),
            );
            expect(current.failedCursor, isNull);
            expect(current.errorMessage, isNull);
            expect(current.isComplete, isTrue);
          }
        }
      }
    });

    test(
      'drops every stale generation result, including errors and cursors',
      () async {
        final chats = InboxReconcilerChatsFake();
        for (final inbox in ['main', 'requests', 'archive']) {
          chats.enqueue(
            InboxChatPageScript(
              inbox: inbox,
              cursor: null,
              manual: true,
              result: ChatsApiOk(
                ChatListData(
                  items: [inboxChatItem('stale-$inbox')],
                  nextCursor: 'stale-cursor',
                ),
              ),
            ),
          );
        }
        final container = _container(
          chats: chats,
          messages: InboxReconcilerMessagesFake(),
        );
        addTearDown(container.dispose);
        final controller = container.read(inboxReconcilerProvider.notifier);
        final stale = controller.reconcile();
        await pumpEventQueue();
        final staleCalls = [
          for (final inbox in ['main', 'requests', 'archive'])
            chats.findCall(inbox: inbox, cursor: null),
        ];
        expect(staleCalls, everyElement(isNotNull));

        for (final inbox in ['main', 'requests', 'archive']) {
          chats.enqueue(
            InboxChatPageScript(
              inbox: inbox,
              cursor: null,
              result: ChatsApiOk(
                ChatListData(items: [inboxChatItem('fresh-$inbox')]),
              ),
            ),
          );
        }
        await controller.reconcile();
        var state = container.read(inboxReconcilerProvider);
        for (final scope in InboxScope.values) {
          final current = state.profileSnapshots['prof-test']!.scopes[scope]!;
          expect(current.items.map((item) => item.chatId), [
            'fresh-${scope.name}',
          ]);
          expect(current.isLoading, isFalse);
          expect(current.errorMessage, isNull);
          expect(current.nextCursor, isNull);
          expect(current.failedCursor, isNull);
        }

        for (var index = 0; index < staleCalls.length; index++) {
          final call = staleCalls[index]!;
          await chats.completeCall(
            call,
            result: index == 1
                ? const ChatsApiFailure(
                    message: 'stale failure',
                    statusCode: 503,
                  )
                : ChatsApiOk(
                    ChatListData(
                      items: [inboxChatItem('stale-${call.inbox}')],
                      nextCursor: 'stale-${call.inbox}-cursor',
                    ),
                  ),
          );
        }
        await stale;
        state = container.read(inboxReconcilerProvider);
        for (final scope in InboxScope.values) {
          final current = state.profileSnapshots['prof-test']!.scopes[scope]!;
          expect(current.items.map((item) => item.chatId), [
            'fresh-${scope.name}',
          ]);
          expect(current.nextCursor, isNull);
          expect(current.failedCursor, isNull);
          expect(current.errorMessage, isNull);
          expect(current.isLoading, isFalse);
          expect(current.isComplete, isTrue);
        }
        final peers = container.read(dmPeerProfileByChatIdProvider);
        expect(
          peers.keys.any((chatId) => chatId.startsWith('stale-')),
          isFalse,
        );
        expect(
          peers.values.any((peerId) => peerId.startsWith('peer-stale-')),
          isFalse,
        );
      },
    );

    test(
      'keeps snapshots keyed by profile, even when chat IDs overlap',
      () async {
        final authController = _AuthHarness();
        final chats = InboxReconcilerChatsFake(
          profileByAuthorization: const {
            'Bearer access-a': 'profile-a',
            'Bearer access-b': 'profile-b',
          },
        );
        for (final inbox in ['main', 'requests', 'archive']) {
          chats.enqueue(
            InboxChatPageScript(
              inbox: inbox,
              cursor: null,
              profileId: 'profile-a',
              authorization: 'Bearer access-a',
              manual: true,
              result: ChatsApiOk(
                ChatListData(items: [inboxChatItem('same-chat')]),
              ),
            ),
          );
        }
        final container = _container(
          chats: chats,
          messages: InboxReconcilerMessagesFake(),
          authController: authController.controller,
        );
        addTearDown(container.dispose);
        final reconciler = container.read(inboxReconcilerProvider.notifier);
        final profileAReconnect = reconciler.reconcile();
        await pumpEventQueue();

        authController.controller.state = const AuthState(
          session: AuthSession(
            accessToken: 'access-b',
            refreshToken: 'refresh-b',
            accountId: 'account-1',
            activeProfileId: 'profile-b',
            expiresInSeconds: 900,
          ),
        );
        container.read(selectedChatIdProvider.notifier).state =
            'profile-b-selection';
        for (final inbox in ['main', 'requests', 'archive']) {
          chats.enqueue(
            InboxChatPageScript(
              inbox: inbox,
              cursor: null,
              profileId: 'profile-b',
              authorization: 'Bearer access-b',
              result: ChatsApiOk(
                ChatListData(
                  items: [
                    inboxChatItem(
                      'same-chat',
                      preview: 'B',
                      creatorProfileId: 'peer-b',
                      unreadCount: 4,
                    ),
                  ],
                ),
              ),
            ),
          );
        }
        await reconciler.reconcile();

        for (final inbox in ['main', 'requests', 'archive']) {
          final staleCall = chats.findCall(
            inbox: inbox,
            cursor: null,
            profileId: 'profile-a',
            authorization: 'Bearer access-a',
          );
          expect(staleCall, isNotNull);
          await chats.completeCall(
            staleCall!,
            result: ChatsApiOk(
              ChatListData(
                items: [
                  inboxChatItem(
                    'same-chat',
                    preview: 'stale-A',
                    creatorProfileId: 'peer-a',
                    unreadCount: 99,
                  ),
                ],
                nextCursor: 'stale-A-cursor',
              ),
            ),
          );
        }
        await profileAReconnect;

        final snapshots = container
            .read(inboxReconcilerProvider)
            .profileSnapshots;
        expect(snapshots.keys, containsAll(<String>['profile-a', 'profile-b']));
        expect(
          snapshots['profile-b']!
              .scopes[InboxScope.main]!
              .items
              .single
              .lastMessagePreview,
          'B',
        );
        expect(
          snapshots['profile-b']!.scopes.values
              .expand((scope) => scope.items)
              .every((item) => item.chat.creatorProfileId == 'peer-b'),
          isTrue,
        );
        expect(
          chats.calls
              .where((call) => call.profileId == 'profile-b')
              .every((call) => call.authorization == 'Bearer access-b'),
          isTrue,
        );
        expect(
          snapshots['profile-a']!.scopes.values.expand((scope) => scope.items),
          isEmpty,
          reason: 'stale profile A rows must not commit after profile boundary',
        );
        final peers = container.read(dmPeerProfileByChatIdProvider);
        expect(peers['same-chat'], 'peer-b');
        expect(peers.values, isNot(contains('peer-a')));
        expect(
          container.read(selectedChatIdProvider),
          'profile-b-selection',
          reason: 'late profile A inbox work must not mutate B selection',
        );
        expect(chats.unmatchedCalls, isEmpty);
        expect(chats.pendingScripts, 0);
      },
    );

    test(
      'global reconcile is REST-only: no history fetch and no WS replay API',
      () async {
        final chats = InboxReconcilerChatsFake();
        for (var run = 0; run < 2; run++) {
          for (final inbox in ['main', 'requests', 'archive']) {
            chats.enqueue(
              InboxChatPageScript(
                inbox: inbox,
                cursor: null,
                result: ChatsApiOk(
                  ChatListData(items: [inboxChatItem('$inbox-$run')]),
                ),
              ),
            );
          }
        }
        final messages = InboxReconcilerMessagesFake();
        final hub = _NoReplayRealtimeHub();
        final container = _container(
          chats: chats,
          messages: messages,
          realtimeHub: hub,
        );
        addTearDown(container.dispose);

        container.read(inboxReconcilerProvider);
        await pumpEventQueue();
        final beforeReconnect = chats.calls.length;
        container.read(realtimeLinkStatusProvider.notifier).state =
            RealtimeLinkStatus.reconnecting;
        container.read(realtimeLinkStatusProvider.notifier).state =
            RealtimeLinkStatus.connected;
        await pumpEventQueue();
        expect(chats.calls.length, beforeReconnect + 3);
        final reconnectCalls = chats.calls.skip(beforeReconnect).toList();
        expect(reconnectCalls.map((call) => call.inbox).toSet(), {
          'main',
          'requests',
          'archive',
        });
        for (final call in reconnectCalls) {
          expect(call.authorization, 'Bearer test-access');
          expect(call.profileId, 'prof-test');
          expect(call.cursor, isNull);
          expect(call.pageSize, isNull);
          expect(call.folderId, isNull);
        }
        expect(chats.unmatchedCalls, isEmpty);
        expect(messages.getCalls, isEmpty);
        expect(
          hub.interactions,
          isEmpty,
          reason: 'global reconciliation is triggered by link state only',
        );
      },
    );

    test(
      'selected-room reconnect ignores mounted unselected rooms and uses its cursor',
      () async {
        final messages = InboxReconcilerMessagesFake()
          ..enqueue(MessageListData(messages: [inboxMessage('last-message')]))
          ..enqueue(MessageListData(messages: [inboxMessage('new-message')]));
        final container = _container(
          chats: InboxReconcilerChatsFake(),
          messages: messages,
        );
        addTearDown(container.dispose);
        container.read(selectedChatIdProvider.notifier).state = 'selected-chat';
        final subscription = container.listen<ChatRoomState>(
          chatRoomControllerProvider('selected-chat'),
          (_, _) {},
          fireImmediately: true,
        );
        addTearDown(subscription.close);
        final unselectedSubscription = container.listen<ChatRoomState>(
          chatRoomControllerProvider('unselected-chat'),
          (_, _) {},
          fireImmediately: true,
        );
        addTearDown(unselectedSubscription.close);
        await pumpEventQueue();

        expect(
          messages.getCalls.map((call) => call.chatId),
          everyElement('selected-chat'),
          reason: 'a mounted but unselected room must not load history',
        );

        container.read(realtimeLinkStatusProvider.notifier).state =
            RealtimeLinkStatus.reconnecting;
        container.read(realtimeLinkStatusProvider.notifier).state =
            RealtimeLinkStatus.connected;
        await pumpEventQueue();

        expect(messages.getCalls, hasLength(2));
        expect(messages.getCalls.last.chatId, 'selected-chat');
        expect(messages.getCalls.last.lastMessageId, 'last-message');
        expect(
          messages.getCalls.map((call) => call.chatId),
          everyElement('selected-chat'),
        );
      },
    );
  });
}

InboxReconcilerChatsFake _allScopesWithTwoPages() {
  final fake = InboxReconcilerChatsFake();
  for (final inbox in ['main', 'requests', 'archive']) {
    fake.enqueue(
      InboxChatPageScript(
        inbox: inbox,
        cursor: null,
        result: ChatsApiOk(
          ChatListData(
            items: [inboxChatItem('$inbox-page-1')],
            nextCursor: '$inbox-cursor-2',
          ),
        ),
      ),
    );
    fake.enqueue(
      InboxChatPageScript(
        inbox: inbox,
        cursor: '$inbox-cursor-2',
        manual: true,
        result: ChatsApiOk(
          ChatListData(items: [inboxChatItem('$inbox-page-2')]),
        ),
      ),
    );
  }
  return fake;
}

ProviderContainer _container({
  required InboxReconcilerChatsFake chats,
  required InboxReconcilerMessagesFake messages,
  AuthController? authController,
  RealtimeHub? realtimeHub,
}) {
  return ProviderContainer(
    overrides: [
      authSessionStorageProvider.overrideWithValue(
        InMemoryAuthSessionStorage(),
      ),
      authControllerProvider.overrideWith(
        (ref) => authController ?? authenticatedAuthController(ref),
      ),
      gatewayConfigProvider.overrideWithValue(
        const GatewayConfig(baseUrl: 'http://api.test'),
      ),
      httpClientProvider.overrideWithValue(
        MockClient((_) async => http.Response('{}', 404)),
      ),
      voiceChatsClientProvider.overrideWithValue(chats),
      voiceMessagesClientProvider.overrideWithValue(messages),
      realtimeAutoConnectProvider.overrideWithValue(false),
      if (realtimeHub != null)
        realtimeHubProvider.overrideWithValue(realtimeHub),
    ],
  );
}

class _AuthHarness {
  late final AuthController controller =
      AuthController(
          authClient: VoiceAuthClient(
            gateway: gatewayHttpForTest(
              MockClient((_) async => http.Response('{}', 500)),
            ),
          ),
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

class _NoReplayRealtimeHub extends RealtimeHub {
  _NoReplayRealtimeHub() : super(_UnwiredRef());

  final interactions = <String>[];

  @override
  Stream<RealtimeFrame> get events => const Stream.empty();

  @override
  Future<void> ensureConnected() async {
    interactions.add('ensureConnected');
  }

  @override
  void ensureSubscribed(String chatId) {
    interactions.add('ensureSubscribed:$chatId');
  }

  @override
  Future<void> reconnectWithNewSession() async {
    interactions.add('reconnectWithNewSession');
  }

  @override
  Future<void> dispose() async {}
}

class _UnwiredRef implements Ref {
  @override
  dynamic noSuchMethod(Invocation invocation) => throw UnimplementedError();
}
