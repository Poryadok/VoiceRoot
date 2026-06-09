import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/messages_client.dart';

import 'support/gateway_test_client.dart';

/// HTTP contract tests for PLAN Phase 4 reactions (emoji + counters).
void main() {
  const config = GatewayConfig(baseUrl: 'http://api.test');
  const auth = 'Bearer access-token';

  group('VoiceMessagesClient.addReaction', () {
    test('POST /api/v1/messages/{id}/reactions with emoji body', () async {
      String? capturedPath;
      String? capturedBody;
      final mock = MockClient((req) async {
        expect(req.method, 'POST');
        capturedPath = req.url.path;
        capturedBody = req.body;
        return http.Response('{}', 204);
      });
      final client = VoiceMessagesClient(
        gateway: gatewayHttpForTest(mock, config: config),
      );

      final result = await client.addReaction(
        authorization: auth,
        messageId: 'msg-1',
        emoji: '👍',
      );

      expect(result, isA<MessagesApiOk<void>>());
      expect(capturedPath, '/api/v1/messages/msg-1/reactions');
      final body = jsonDecode(capturedBody!) as Map<String, dynamic>;
      expect(body['emoji'], '👍');
    });
  });

  group('VoiceMessagesClient.removeReaction', () {
    test('DELETE /api/v1/messages/{id}/reactions with emoji query', () async {
      String? capturedPath;
      final mock = MockClient((req) async {
        expect(req.method, 'DELETE');
        capturedPath = req.url.path;
        expect(req.url.queryParameters['emoji'], '👍');
        return http.Response('{}', 204);
      });
      final client = VoiceMessagesClient(
        gateway: gatewayHttpForTest(mock, config: config),
      );

      final result = await client.removeReaction(
        authorization: auth,
        messageId: 'msg-2',
        emoji: '👍',
      );

      expect(result, isA<MessagesApiOk<void>>());
      expect(capturedPath, '/api/v1/messages/msg-2/reactions');
    });
  });
}
