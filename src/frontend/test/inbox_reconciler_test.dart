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
    test(
      'starts the three-scope snapshot after realtime reconnects',
      () async {
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
      },
    );

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
        expect(
          container
              .read(inboxReconcilerProvider)
              .profileSnapshots['prof-test']!
              .scopes[InboxScope.main]!
              .items
              .map((item) => item.chatId),
          ['main-page-1'],
        );
        expect(
          container
              .read(inboxReconcilerProvider)
              .profileSnapshots['prof-test']!
              .scopes[InboxScope.requests]!
              .items
              .map((item) => item.chatId),
          ['requests-page-1'],
        );
        expect(
          container
              .read(inboxReconcilerProvider)
              .profileSnapshots['prof-test']!
              .scopes[InboxScope.archive]!
              .items
              .map((item) => item.chatId),
          ['archive-page-1'],
        );

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
        }
        expect(
          messages.getCalls,
          isEmpty,
          reason: 'global inbox reconciliation must not load message history',
        );
      },
    );

    test(
      'failed later page keeps rows and retries the exact opaque cursor',
      () async {
        final chats = InboxReconcilerChatsFake();
        chats
          ..enqueue(
            InboxChatPageScript(
              inbox: 'main',
              cursor: null,
              result: ChatsApiOk(
                ChatListData(
                  items: [inboxChatItem('cached-main')],
                  nextCursor: 'opaque-main-2',
                ),
              ),
            ),
          )
          ..enqueue(
            const InboxChatPageScript(
              inbox: 'requests',
              cursor: null,
              result: ChatsApiOk(ChatListData(items: [])),
            ),
          )
          ..enqueue(
            const InboxChatPageScript(
              inbox: 'archive',
              cursor: null,
              result: ChatsApiOk(ChatListData(items: [])),
            ),
          )
          ..enqueue(
            const InboxChatPageScript(
              inbox: 'main',
              cursor: 'opaque-main-2',
              result: ChatsApiFailure(message: 'offline', statusCode: 503),
            ),
          )
          ..enqueue(
            InboxChatPageScript(
              inbox: 'main',
              cursor: 'opaque-main-2',
              result: ChatsApiOk(
                ChatListData(items: [inboxChatItem('after-retry')]),
              ),
            ),
          );
        final container = _container(
          chats: chats,
          messages: InboxReconcilerMessagesFake(),
        );
        addTearDown(container.dispose);

        final controller = container.read(inboxReconcilerProvider.notifier);
        await controller.reconcile();
        var scope = container
            .read(inboxReconcilerProvider)
            .profileSnapshots['prof-test']!
            .scopes[InboxScope.main]!;
        expect(scope.items.map((item) => item.chatId), ['cached-main']);
        expect(scope.failedCursor, 'opaque-main-2');
        expect(scope.nextCursor, 'opaque-main-2');
        expect(scope.errorMessage, 'offline');
        expect(scope.isComplete, isFalse);
        expect(
          container
              .read(inboxReconcilerProvider)
              .profileSnapshots['prof-test']!
              .scopes[InboxScope.requests]!
              .isComplete,
          isTrue,
        );

        await controller.retry(InboxScope.main);
        scope = container
            .read(inboxReconcilerProvider)
            .profileSnapshots['prof-test']!
            .scopes[InboxScope.main]!;
        expect(scope.items.map((item) => item.chatId), [
          'cached-main',
          'after-retry',
        ]);
        expect(scope.failedCursor, isNull);
        expect(scope.isComplete, isTrue);
        expect(
          chats.calls.map((call) => '${call.inbox}:${call.cursor}'),
          containsAllInOrder([
            'main:null',
            'main:opaque-main-2',
            'main:opaque-main-2',
          ]),
        );
      },
    );

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

        for (final call in staleCalls) {
          await chats.completeCall(
            call!,
            result: const ChatsApiFailure(
              message: 'stale failure',
              statusCode: 503,
            ),
          );
        }
        await stale;
        state = container.read(inboxReconcilerProvider);
        expect(
          state.profileSnapshots['prof-test']!.scopes.values
              .expand((scope) => scope.items)
              .every((item) => item.chatId.startsWith('fresh-')),
          isTrue,
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
        await reconciler.reconcile();

        authController.controller.state = const AuthState(
          session: AuthSession(
            accessToken: 'access-b',
            refreshToken: 'refresh-b',
            accountId: 'account-1',
            activeProfileId: 'profile-b',
            expiresInSeconds: 900,
          ),
        );
        for (final inbox in ['main', 'requests', 'archive']) {
          chats.enqueue(
            InboxChatPageScript(
              inbox: inbox,
              cursor: null,
              profileId: 'profile-b',
              result: ChatsApiOk(
                ChatListData(items: [inboxChatItem('same-chat', preview: 'B')]),
              ),
            ),
          );
        }
        await reconciler.reconcile();

        final snapshots = container
            .read(inboxReconcilerProvider)
            .profileSnapshots;
        expect(snapshots.keys, containsAll(<String>['profile-a', 'profile-b']));
        expect(
          snapshots['profile-a']!
              .scopes[InboxScope.main]!
              .items
              .single
              .lastMessagePreview,
          isNull,
        );
        expect(
          snapshots['profile-b']!
              .scopes[InboxScope.main]!
              .items
              .single
              .lastMessagePreview,
          'B',
        );
      },
    );

    test(
      'global reconcile is REST-only: no history fetch and no WS replay API',
      () async {
        final chats = InboxReconcilerChatsFake();
        for (final inbox in ['main', 'requests', 'archive']) {
          chats.enqueue(
            InboxChatPageScript(
              inbox: inbox,
              cursor: null,
              result: ChatsApiOk(ChatListData(items: [inboxChatItem(inbox)])),
            ),
          );
        }
        final messages = InboxReconcilerMessagesFake();
        final hub = _NoReplayRealtimeHub();
        final container = _container(
          chats: chats,
          messages: messages,
          realtimeHub: hub,
        );
        addTearDown(container.dispose);

        await container.read(inboxReconcilerProvider.notifier).reconcile();
        expect(messages.getCalls, isEmpty);
        expect(hub.replayRequests, isEmpty);
        expect(
          hub.resumeRequests,
          isEmpty,
          reason: 'resume is a live-session trigger, not inbox history',
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

    test(
      'profile boundary ignores stale selected-room reconnect work',
      () async {
        final authController = _AuthHarness();
        final messages = InboxReconcilerMessagesFake(
          profileByAuthorization: const {'Bearer access-a': 'profile-a'},
        )..enqueue(MessageListData(messages: []), manual: true);
        final container = _container(
          chats: InboxReconcilerChatsFake(),
          messages: messages,
          authController: authController.controller,
        );
        addTearDown(container.dispose);
        container.read(inboxReconcilerProvider);
        container.read(selectedChatIdProvider.notifier).state = 'old-chat';
        final subscription = container.listen<ChatRoomState>(
          chatRoomControllerProvider('old-chat'),
          (_, _) {},
          fireImmediately: true,
        );
        addTearDown(subscription.close);
        await pumpEventQueue();
        final oldCall = messages.getCalls.single;

        authController.controller.state = const AuthState(
          session: AuthSession(
            accessToken: 'access-b',
            refreshToken: 'refresh-b',
            accountId: 'account-1',
            activeProfileId: 'profile-b',
            expiresInSeconds: 900,
          ),
        );
        await pumpEventQueue();
        await messages.completeCall(
          oldCall,
          result: MessagesApiOk(
            MessageListData(messages: [inboxMessage('old-profile-message')]),
          ),
        );
        await pumpEventQueue();

        expect(container.read(selectedChatIdProvider), isNull);
        expect(messages.getCalls.single, same(oldCall));
        expect(
          container.read(chatRoomControllerProvider('old-chat')).messages,
          isEmpty,
          reason: 'old-profile async history must not commit after boundary',
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

  final replayRequests = <Object>[];
  final resumeRequests = <Object>[];

  @override
  Stream<RealtimeFrame> get events => const Stream.empty();

  @override
  Future<void> ensureConnected() async {}

  @override
  void ensureSubscribed(String chatId) {}

  @override
  Future<void> dispose() async {}
}

class _UnwiredRef implements Ref {
  @override
  dynamic noSuchMethod(Invocation invocation) => throw UnimplementedError();
}
