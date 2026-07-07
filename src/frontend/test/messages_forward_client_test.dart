import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/messages_client.dart';

import 'support/gateway_test_client.dart';

/// HTTP contract tests for text-chat.md forward with attribution.
void main() {
  const config = GatewayConfig(baseUrl: 'http://api.test');
  const auth = 'Bearer access-token';

  group('VoiceMessagesClient.forwardMessage', () {
    test('POST /api/v1/messages/forward with attribution fields', () async {
      String? capturedBody;
      final mock = MockClient((req) async {
        expect(req.method, 'POST');
        expect(req.url.path, '/api/v1/messages/forward');
        capturedBody = req.body;
        return http.Response(
          jsonEncode({
            'message': {
              'id': 'msg-fwd-1',
              'chat': {'id': 'chat-target'},
              'sender_profile_id': 'profile-a',
              'content': 'original text',
              'type': 'forward',
              'message_kind': 'MESSAGE_KIND_FORWARD',
              'forward_from_id': 'msg-src-1',
              'forward_from_sender': 'Alice',
            },
          }),
          200,
        );
      });
      final client = VoiceMessagesClient(
        gateway: gatewayHttpForTest(mock, config: config),
      );

      final result = await client.forwardMessage(
        authorization: auth,
        sourceMessageId: 'msg-src-1',
        targetChatId: 'chat-target',
      );

      expect(result, isA<MessagesApiOk<VoiceMessage>>());
      final body = jsonDecode(capturedBody!) as Map<String, dynamic>;
      expect(body['source_message_id'], 'msg-src-1');
      expect(body['target_chat'], {'id': 'chat-target'});

      final msg = (result as MessagesApiOk<VoiceMessage>).data;
      expect(msg.messageKind, VoiceMessageKind.forward);
      expect(msg.forwardFromId, 'msg-src-1');
      expect(msg.forwardFromSender, 'Alice');
      expect(msg.content, 'original text');
    });

    test('forwards optional commentary in request body', () async {
      String? capturedBody;
      final mock = MockClient((req) async {
        capturedBody = req.body;
        return http.Response(
          jsonEncode({
            'message': {
              'id': 'msg-fwd-2',
              'chat': {'id': 'chat-target'},
              'sender_profile_id': 'profile-a',
              'content': 'quoted',
              'message_kind': 'MESSAGE_KIND_FORWARD',
              'forward_from_id': 'msg-src-2',
              'forward_from_sender': 'Bob',
            },
          }),
          200,
        );
      });
      final client = VoiceMessagesClient(
        gateway: gatewayHttpForTest(mock, config: config),
      );

      await client.forwardMessage(
        authorization: auth,
        sourceMessageId: 'msg-src-2',
        targetChatId: 'chat-target',
        commentary: 'see this',
      );

      final body = jsonDecode(capturedBody!) as Map<String, dynamic>;
      expect(body['commentary'], 'see this');
    });
  });
}
