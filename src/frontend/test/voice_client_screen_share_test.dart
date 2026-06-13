import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/voice_client.dart';

import 'support/gateway_test_client.dart';

void main() {
  const config = GatewayConfig(baseUrl: 'http://api.test');
  const auth = 'Bearer access-token';

  group('VoiceCallsClient screen share', () {
    test('POST start returns stream id', () async {
      final mock = MockClient((req) async {
        expect(req.method, 'POST');
        expect(req.url.path, '/api/v1/voice/calls/room-1/screen-share/start');
        return http.Response(
          jsonEncode({
            'screen_share_session': {'stream_id': 'stream-abc'},
          }),
          200,
        );
      });
      final client = VoiceCallsClient(
        gateway: gatewayHttpForTest(mock, config: config),
      );

      final result = await client.startScreenShare(
        authorization: auth,
        roomId: 'room-1',
      );

      expect(result, isA<VoiceApiOk<String>>());
      expect((result as VoiceApiOk<String>).data, 'stream-abc');
    });

    test('POST stop with stream id', () async {
      final mock = MockClient((req) async {
        expect(req.method, 'POST');
        expect(req.url.path, '/api/v1/voice/calls/room-1/screen-share/stop');
        final body = jsonDecode(req.body as String) as Map<String, dynamic>;
        expect(body['stream_id'], 'stream-abc');
        return http.Response('', 204);
      });
      final client = VoiceCallsClient(
        gateway: gatewayHttpForTest(mock, config: config),
      );

      final result = await client.stopScreenShare(
        authorization: auth,
        roomId: 'room-1',
        streamId: 'stream-abc',
      );

      expect(result, isA<VoiceApiOk<void>>());
    });
  });
}
