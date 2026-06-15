import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/e2e_client.dart';
import 'package:voice_frontend/backend/gateway_http.dart';
import 'package:voice_frontend/e2e/e2e_crypto_adapter.dart';
import 'package:voice_frontend/e2e/e2e_store_factory.dart';

import 'support/gateway_test_client.dart';

/// Batch E2E-A red test: network pre-key fetch without in-process bilateral sessions.
void main() {
  group('VoiceE2eClient.encryptForChat', () {
    test('uses HTTP pre-key bundle and does not create bilateral in-memory peer store', () async {
      const localProfileId = 'local-net-1';
      const peerProfileId = 'peer-net-2';
      final authorization = _testAuthorization(localProfileId);

      final peerStore = await createInitializedSignalStore();
      final peerBundleWire = await serializePreKeyBundle(peerStore);

      var preKeyFetchCount = 0;
      final gateway = gatewayHttpForTest(
        MockClient((req) async {
          if (req.url.path == '/api/v1/messages/prekeys' &&
              req.url.queryParameters['profile_id'] == peerProfileId) {
            preKeyFetchCount++;
            return http.Response(
              jsonEncode({'bundle': peerBundleWire}),
              200,
              headers: {'content-type': 'application/json'},
            );
          }
          return http.Response('not found', 404);
        }),
      );

      final adapter = E2eCryptoAdapter.inMemoryForTest();
      final client = VoiceE2eClient(gateway: gateway, adapter: adapter);

      final ciphertext = await client.encryptForChat(
        authorization: authorization,
        chatId: 'dm-chat-net',
        peerProfileId: peerProfileId,
        plaintext: 'network-path-secret',
      );

      expect(preKeyFetchCount, 1);
      expect(ciphertext, isNotEmpty);
      expect(
        adapter.sessionManager.peekStore(peerProfileId),
        isNull,
        reason: 'prod path must not keep bilateral in-memory peer store',
      );
    });
  });
}

String _testAuthorization(String profileId) {
  final payload = base64Url.encode(
    utf8.encode(jsonEncode({'profile_id': profileId})),
  );
  return 'Bearer header.$payload.sig';
}
