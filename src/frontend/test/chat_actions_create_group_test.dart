import 'dart:async';
import 'dart:convert';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';

import 'support/auth_test_overrides.dart';

void main() {
  group('createGroupWithMembers session ownership', () {
    test(
      'profile switch during create prevents invite and selection',
      () async {
        final createResponse = Completer<http.Response>();
        final createStarted = Completer<void>();
        var inviteRequests = 0;
        final container = _container(
          MockClient((request) async {
            if (request.method == 'POST' &&
                request.url.path == '/api/v1/chats') {
              createStarted.complete();
              return createResponse.future;
            }
            if (request.url.path.endsWith('/members')) inviteRequests++;
            return http.Response('{}', 404);
          }),
        );
        addTearDown(container.dispose);

        final pending = container
            .read(chatActionsProvider)
            .createGroupWithMembers(
              name: 'Weekend plans',
              memberProfileIds: const ['dm-peer', 'friend-a'],
            );
        await createStarted.future;
        _switchProfile(container, 'profile-b', 'token-b');
        createResponse.complete(_createdGroupResponse());

        expect(await pending, kChatActionStaleContext);
        expect(inviteRequests, 0);
        expect(container.read(selectedChatIdProvider), isNull);
      },
    );

    test(
      'profile switch during invite prevents stale chat selection',
      () async {
        final inviteResponse = Completer<http.Response>();
        final inviteStarted = Completer<void>();
        final container = _container(
          MockClient((request) async {
            if (request.method == 'POST' &&
                request.url.path == '/api/v1/chats') {
              return _createdGroupResponse();
            }
            if (request.method == 'POST' &&
                request.url.path == '/api/v1/chats/group-from-dm/members') {
              inviteStarted.complete();
              return inviteResponse.future;
            }
            return http.Response('{}', 404);
          }),
        );
        addTearDown(container.dispose);

        final pending = container
            .read(chatActionsProvider)
            .createGroupWithMembers(
              name: 'Weekend plans',
              memberProfileIds: const ['dm-peer', 'friend-a'],
            );
        await inviteStarted.future;
        _switchProfile(container, 'profile-b', 'token-b');
        inviteResponse.complete(http.Response('', 204));

        expect(await pending, kChatActionStaleContext);
        expect(container.read(selectedChatIdProvider), isNull);
      },
    );
  });
}

ProviderContainer _container(http.Client client) {
  return ProviderContainer(
    overrides: [
      authSessionStorageProvider.overrideWithValue(
        InMemoryAuthSessionStorage(),
      ),
      authControllerProvider.overrideWith(authenticatedAuthController),
      gatewayConfigProvider.overrideWithValue(
        const GatewayConfig(baseUrl: 'http://api.test'),
      ),
      httpClientProvider.overrideWithValue(client),
      realtimeHubProvider.overrideWith((ref) => _NoopRealtimeHub()),
    ],
  );
}

void _switchProfile(
  ProviderContainer container,
  String profileId,
  String token,
) {
  container.read(authControllerProvider.notifier).state = AuthState(
    session: AuthSession(
      accessToken: token,
      refreshToken: 'refresh-$token',
      accountId: 'acc-test',
      activeProfileId: profileId,
      expiresInSeconds: 900,
    ),
  );
}

http.Response _createdGroupResponse() => http.Response(
  jsonEncode({
    'chat': {
      'id': 'group-from-dm',
      'type': 'CHAT_TYPE_GROUP',
      'name': 'Weekend plans',
      'creator_profile_id': 'profile-me',
    },
  }),
  200,
);

class _NoopRealtimeHub extends RealtimeHub {
  _NoopRealtimeHub() : super(_UnwiredRef());

  @override
  Future<void> ensureConnected() async {}

  @override
  void ensureSubscribed(String chatId) {}
}

class _UnwiredRef implements Ref {
  @override
  dynamic noSuchMethod(Invocation invocation) => throw UnimplementedError();
}
