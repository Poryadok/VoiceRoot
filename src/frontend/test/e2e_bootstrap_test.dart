import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/e2e_client.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/e2e/e2e_bootstrap.dart';
import 'package:voice_frontend/e2e/e2e_crypto_adapter.dart';

import 'support/gateway_test_client.dart';

void main() {
  test('ensurePreKeysUploaded uploads bundle', () async {
    var uploadCalls = 0;
    final gateway = gatewayHttpForTest(
      MockClient((request) async {
        if (request.method == 'POST' && request.url.path.endsWith('/messages/prekeys')) {
          uploadCalls++;
          return http.Response('{}', 200);
        }
        return http.Response('{}', 404);
      }),
      config: const GatewayConfig(baseUrl: 'http://api.test'),
    );
    final adapter = E2eCryptoAdapter.inMemoryForTest();
    final client = VoiceE2eClient(gateway: gateway, adapter: adapter);
    final bootstrap = E2eBootstrapService(e2eClient: client);

    await bootstrap.ensurePreKeysUploaded(
      authorization: 'Bearer ${testVoiceAccessToken()}',
    );

    expect(uploadCalls, 1);
  });
}

String testVoiceAccessToken() {
  // Minimal JWT-shaped token with profile_id claim for bootstrap pre-key path.
  const header = 'eyJhbGciOiJub25lIn0';
  const payload =
      'eyJwcm9maWxlX2lkIjoicHJvZi10ZXN0IiwidXNlcl9pZCI6ImFjYy10ZXN0In0';
  return '$header.$payload.';
}
