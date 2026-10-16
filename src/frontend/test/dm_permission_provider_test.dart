import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/gateway_http.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/dm_permission_provider.dart';

import 'support/auth_test_overrides.dart';

class _PermissionChatsClient extends VoiceChatsClient {
  _PermissionChatsClient()
    : super(
        gateway: GatewayHttpClient(
          httpClient: MockClient((_) async => http.Response('{}', 200)),
          config: const GatewayConfig(baseUrl: 'http://unused.test'),
        ),
      );

  final requests = <({String authorization, String target})>[];
  bool allowed = false;

  @override
  Future<ChatsApiResult<bool>> canCreateDm({
    required String authorization,
    required String otherProfileId,
  }) async {
    requests.add((authorization: authorization, target: otherProfileId));
    return ChatsApiOk<bool>(allowed);
  }
}

void main() {
  test('permission lookup is bound to the current viewer and target', () async {
    final chats = _PermissionChatsClient();
    final container = ProviderContainer(
      overrides: [
        ...voiceAppTestOverrides(
          client: MockClient((_) async => http.Response('{}', 200)),
        ),
        voiceChatsClientProvider.overrideWithValue(chats),
      ],
    );
    addTearDown(container.dispose);
    final request = (
      authorization: 'Bearer test-access',
      accountId: 'acc-test',
      viewerProfileId: 'prof-test',
      targetProfileId: 'profile-2',
    );

    final allowed = await container.read(dmPermissionProvider(request).future);
    expect(allowed, isFalse);
    expect(chats.requests, [
      (authorization: 'Bearer test-access', target: 'profile-2'),
    ]);

    chats.allowed = true;
    final otherTarget = (
      authorization: request.authorization,
      accountId: request.accountId,
      viewerProfileId: request.viewerProfileId,
      targetProfileId: 'profile-3',
    );
    expect(
      await container.read(dmPermissionProvider(otherTarget).future),
      isTrue,
    );
    expect(chats.requests.last.target, 'profile-3');
  });

  test(
    'stale viewer key fails closed without an authorization request',
    () async {
      final chats = _PermissionChatsClient()..allowed = true;
      final container = ProviderContainer(
        overrides: [
          ...voiceAppTestOverrides(
            client: MockClient((_) async => http.Response('{}', 200)),
          ),
          voiceChatsClientProvider.overrideWithValue(chats),
        ],
      );
      addTearDown(container.dispose);

      await expectLater(
        container.read(
          dmPermissionProvider((
            authorization: 'Bearer test-access',
            accountId: 'acc-test',
            viewerProfileId: 'old-profile',
            targetProfileId: 'profile-2',
          )).future,
        ),
        throwsA(isA<DmPermissionLoadException>()),
      );
      expect(chats.requests, isEmpty);
    },
  );
}
