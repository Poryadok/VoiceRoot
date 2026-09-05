import 'dart:convert';

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
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/state/inbox_reconciler.dart';

import 'support/gateway_test_client.dart';
import 'support/inbox_reconciler_fakes.dart';

void main() {
  group('T052 profile-switch reconnect ordering (RED)', () {
    test(
      'does not accept a B snapshot from a bare link-status transition',
      () async {
        final auth = _SwitchingAuthHarness();
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
              profileId: 'profile-b',
              authorization: 'Bearer access-b',
              result: const ChatsApiOk(ChatListData(items: [])),
            ),
          );
        }
        final container = _container(chats: chats, auth: auth);
        addTearDown(container.dispose);

        // Cycle4's real RealtimeHub transport test supplies the matching B
        // hello. This lifecycle unit protects the negative half: a mutable
        // link status cannot impersonate that accepted generation signal.
        container.read(inboxReconcilerProvider);
        container.read(realtimeLinkStatusProvider.notifier).state =
            RealtimeLinkStatus.reconnecting;

        expect(await auth.controller.switchActiveProfile('profile-b'), isNull);
        await pumpEventQueue();
        expect(_profileBCalls(chats), isEmpty);

        container.read(realtimeLinkStatusProvider.notifier).state =
            RealtimeLinkStatus.connected;
        await pumpEventQueue();

        expect(
          _profileBCalls(chats),
          isEmpty,
          reason:
              'a bare reconnecting-to-connected state transition cannot '
              "replace Cycle3's accepted B hello binding",
        );
        expect(auth.switchRequestCount, 1);
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
