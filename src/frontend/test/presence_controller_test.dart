import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/realtime_client.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/state/presence_providers.dart';

import 'support/auth_test_overrides.dart';

void main() {
  test('applies presence_update from Realtime WS to watched profile', () async {
    final container = ProviderContainer(
      overrides: [
        authSessionStorageProvider.overrideWithValue(
          InMemoryAuthSessionStorage(),
        ),
        authControllerProvider.overrideWith(authenticatedAuthController),
        gatewayConfigProvider.overrideWithValue(
          const GatewayConfig(baseUrl: 'http://api.test'),
        ),
        httpClientProvider.overrideWithValue(
          MockClient((_) async => http.Response('{}', 404)),
        ),
        realtimeLinkStatusProvider.overrideWith(
          (ref) => RealtimeLinkStatus.connected,
        ),
        realtimeEventProvider.overrideWith(
          (ref) => Stream.value(
            RealtimeFrame(
              op: 'presence_update',
              data: {
                'profile_id': 'peer-1',
                'status': 'dnd',
                'last_seen': '2026-06-02T18:30:00Z',
              },
            ),
          ),
        ),
      ],
    );
    addTearDown(container.dispose);

    container.read(presenceProvider('peer-1'));
    await Future<void>.delayed(Duration.zero);

    expect(container.read(presenceProvider('peer-1'))?.status, 'dnd');
    expect(
      container.read(presenceProvider('peer-1'))?.lastSeen,
      DateTime.utc(2026, 6, 2, 18, 30),
    );
  });

  test(
    'refreshes watched profiles via bulk REST when WS is disconnected',
    () async {
      var bulkCalls = 0;
      final container = ProviderContainer(
        overrides: [
          authSessionStorageProvider.overrideWithValue(
            InMemoryAuthSessionStorage(),
          ),
          authControllerProvider.overrideWith(authenticatedAuthController),
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(baseUrl: 'http://api.test'),
          ),
          httpClientProvider.overrideWithValue(
            MockClient((req) async {
              if (req.method == 'POST' &&
                  req.url.path == '/api/v1/users/presence/bulk') {
                bulkCalls++;
                return http.Response(
                  '{"byProfileId":{"peer-1":{"profileId":"peer-1","status":"online"}}}',
                  200,
                  headers: {'content-type': 'application/json'},
                );
              }
              return http.Response('{}', 404);
            }),
          ),
          realtimeLinkStatusProvider.overrideWith(
            (ref) => RealtimeLinkStatus.disconnected,
          ),
          realtimeEventProvider.overrideWith((ref) => const Stream.empty()),
        ],
      );
      addTearDown(container.dispose);

      container.read(presenceProvider('peer-1'));
      await Future<void>.delayed(const Duration(milliseconds: 50));

      expect(bulkCalls, greaterThanOrEqualTo(1));
      expect(container.read(presenceProvider('peer-1'))?.isOnline, isTrue);
    },
  );
}
