import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/gateway_http.dart';
import 'package:voice_frontend/backend/gateway_request_id.dart';

import 'support/gateway_test_client.dart';

void main() {
  test('newGatewayRequestId returns 32 lowercase hex chars', () {
    expect(newGatewayRequestId(), matches(RegExp(r'^[0-9a-f]{32}$')));
  });

  test('GatewayHttpClient sends X-Request-Id per request', () async {
    const config = GatewayConfig(baseUrl: 'http://api.test');
    String? capturedRequestId;
    final mock = MockClient((req) async {
      capturedRequestId = req.headers['X-Request-Id'];
      return http.Response('', 204);
    });
    final client = gatewayHttpForTest(mock, config: config);
    final result = await client.deleteEmpty(
      uri: Uri.parse('http://api.test/api/v1/chats/chat-1'),
      authorization: 'Bearer token',
    );
    expect(result, isA<GatewayHttpOk<void>>());
    expect(capturedRequestId, isNotNull);
    expect(capturedRequestId, matches(RegExp(r'^[0-9a-f]{32}$')));
  });
}
