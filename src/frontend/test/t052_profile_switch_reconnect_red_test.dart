import 'dart:async';
import 'dart:convert';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_client.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/friends_client.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/guest_credentials_storage.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/state/inbox_reconciler.dart';
import 'package:voice_frontend/state/profile_context_controller.dart';
import 'package:voice_frontend/state/social_providers.dart';
import 'package:voice_frontend/state/subscription_providers.dart';

import 'support/gateway_test_client.dart';
import 'support/inbox_reconciler_fakes.dart';

void main() {
  group('T052 profile-switch reconnect ordering (RED)', () {
    test(
      'does not start B global ListChats until its awaited WS reconnect succeeds',
      () async {
        final auth = _SwitchingAuthHarness();
        final chats = InboxReconcilerChatsFake(
          profileByAuthorization: const {
            'Bearer access-a': 'profile-a',
            'Bearer access-b': 'profile-b',
          },
        );
        // The second three scripts make a duplicate snapshot observable on the
        // legacy Auth-triggered path. The required handoff accepts only one.
        for (var snapshot = 0; snapshot < 2; snapshot++) {
          for (final inbox in ['main', 'requests', 'archive']) {
            chats.enqueue(
              InboxChatPageScript(
                inbox: inbox,
                cursor: null,
                profileId: 'profile-b',
                authorization: 'Bearer access-b',
                manual: true,
                result: ChatsApiOk(
                  ChatListData(items: [inboxChatItem('b-$snapshot-$inbox')]),
                ),
              ),
            );
          }
        }
        final hub = _DeferredProfileSwitchRealtimeHub();
        final container = _container(chats: chats, auth: auth, hub: hub);
        addTearDown(container.dispose);
        addTearDown(() async {
          for (final call in _profileBCalls(chats)) {
            if (!call.completed) {
              await chats.completeCall(
                call,
                result: const ChatsApiOk(ChatListData(items: [])),
              );
            }
          }
        });

        // Both listeners must be installed before the A -> B boundary.
        container.read(profileContextCoordinatorProvider);
        container.read(inboxReconcilerProvider);
        container.read(realtimeLinkStatusProvider.notifier).state =
            RealtimeLinkStatus.reconnecting;

        expect(await auth.controller.switchActiveProfile('profile-b'), isNull);
        await pumpEventQueue();
        final callsBeforeReconnectSuccess = _profileBCalls(chats).length;

        hub.emitReconnectHello(container);
        await pumpEventQueue();
        final callsAfterReconnectSuccess = _profileBCalls(chats);

        expect(
          [callsBeforeReconnectSuccess, callsAfterReconnectSuccess.length],
          [0, 3],
          reason:
              'T052 must accept exactly one B main/requests/archive snapshot '
              'only after the awaited WS reconnect succeeds',
        );
        expect(callsAfterReconnectSuccess.map((call) => call.inbox).toSet(), {
          'main',
          'requests',
          'archive',
        });
        expect(
          callsAfterReconnectSuccess.every(
            (call) =>
                call.authorization == 'Bearer access-b' &&
                call.profileId == 'profile-b' &&
                call.cursor == null,
          ),
          isTrue,
        );
        expect(auth.switchRequestCount, 1);

        for (final call in callsAfterReconnectSuccess) {
          await chats.completeCall(
            call,
            result: ChatsApiOk(
              ChatListData(items: [inboxChatItem('accepted-${call.inbox}')]),
            ),
          );
        }
        await pumpEventQueue();
        final snapshot = container
            .read(inboxReconcilerProvider)
            .profileSnapshots['profile-b']!;
        for (final scope in InboxScope.values) {
          expect(snapshot[scope].items.map((item) => item.chatId), [
            'accepted-${scope.name}',
          ]);
        }
      },
    );
  });
}

List<InboxChatCall> _profileBCalls(InboxReconcilerChatsFake chats) => chats
    .calls
    .where(
      (call) =>
          call.profileId == 'profile-b' &&
          call.authorization == 'Bearer access-b',
    )
    .toList(growable: false);

ProviderContainer _container({
  required InboxReconcilerChatsFake chats,
  required _SwitchingAuthHarness auth,
  required RealtimeHub hub,
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
      voiceMessagesClientProvider.overrideWithValue(
        InboxReconcilerMessagesFake(),
      ),
      chatListControllerProvider.overrideWith(_NoAutoChatListController.new),
      realtimeAutoConnectProvider.overrideWithValue(false),
      realtimeHubProvider.overrideWithValue(hub),
      activeProfileProvider.overrideWith((ref) async => null),
      profileProvider('profile-b').overrideWith((ref) async => null),
      friendsListProvider.overrideWith(
        (ref) async => const FriendsListData(friends: []),
      ),
      subscriptionProvider.overrideWith((ref) async => null),
      myProfilesProvider.overrideWith((ref) async => const []),
    ],
  );
}

class _NoAutoChatListController extends ChatListController {
  _NoAutoChatListController(super.ref);

  @override
  Future<void> loadInitial() async {}
}

class _SwitchingAuthHarness {
  _SwitchingAuthHarness() {
    final mock = MockClient((request) async {
      if (request.url.path != '/api/v1/auth/switch-profile') {
        return http.Response('not found', 404);
      }
      switchRequestCount++;
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
  var switchRequestCount = 0;
}

class _DeferredProfileSwitchRealtimeHub extends RealtimeHub {
  _DeferredProfileSwitchRealtimeHub() : super(_UnwiredRef());

  final Completer<void> _reconnect = Completer<void>();

  @override
  Future<void> reconnectWithNewSession() => _reconnect.future;

  void emitReconnectHello(ProviderContainer container) {
    container.read(realtimeLinkStatusProvider.notifier).state =
        RealtimeLinkStatus.connected;
    if (!_reconnect.isCompleted) _reconnect.complete();
  }

  @override
  Future<void> dispose() async {}
}

class _UnwiredRef implements Ref {
  @override
  dynamic noSuchMethod(Invocation invocation) => throw UnimplementedError();
}
