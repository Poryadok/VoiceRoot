import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/gateway_client.dart';
import 'package:voice_frontend/backend/gateway_config.dart';

void main() {
  group('VoiceGatewayClient.fetchHealth', () {
    test('missing base URL', () async {
      final c = VoiceGatewayClient(
        httpClient: MockClient((_) async => http.Response('unused', 500)),
        config: const GatewayConfig(baseUrl: ''),
      );
      final r = await c.fetchHealth();
      expect(r, isA<GatewayHealthFailure>());
      expect((r as GatewayHealthFailure).message, 'missing base URL');
    });

    test('200 OK', () async {
      final mock = MockClient((req) async {
        expect(req.url.path, '/health');
        return http.Response('ok', 200);
      });
      final client = VoiceGatewayClient(
        httpClient: mock,
        config: const GatewayConfig(baseUrl: 'http://api.test'),
      );
      final r = await client.fetchHealth();
      expect(r, isA<GatewayHealthOk>());
    });

    test('non-200', () async {
      final mock = MockClient((_) async => http.Response('err', 503));
      final client = VoiceGatewayClient(
        httpClient: mock,
        config: const GatewayConfig(baseUrl: 'http://api.test'),
      );
      final r = await client.fetchHealth();
      expect(r, isA<GatewayHealthFailure>());
      expect((r as GatewayHealthFailure).message, 'HTTP 503');
    });

    test('transport error', () async {
      final mock = MockClient((_) async {
        throw Exception('network');
      });
      final client = VoiceGatewayClient(
        httpClient: mock,
        config: const GatewayConfig(baseUrl: 'http://api.test'),
      );
      final r = await client.fetchHealth();
      expect(r, isA<GatewayHealthFailure>());
    });
  });

  group('fetchVersionBody', () {
    test('returns body on 200', () async {
      final mock = MockClient((req) async {
        expect(req.url.path, '/api/v1/version');
        return http.Response('{"ok":true}', 200);
      });
      final client = VoiceGatewayClient(
        httpClient: mock,
        config: const GatewayConfig(baseUrl: 'http://api.test'),
      );
      expect(await client.fetchVersionBody(), '{"ok":true}');
    });
  });
}
