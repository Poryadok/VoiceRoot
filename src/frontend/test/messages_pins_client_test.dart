import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/messages_client.dart';

import 'support/gateway_test_client.dart';

void main() {
  const config = GatewayConfig(baseUrl: 'http://api.test');
  const auth = 'Bearer access-token';

  group('VoiceMessagesClient.pinMessage', () {
    test('POST /api/v1/messages/{id}/pin with chat body', () async {
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

      final result = await client.pinMessage(
        authorization: auth,
        messageId: 'msg-1',
        chatId: 'chat-1',
      );

      expect(result, isA<MessagesApiOk<void>>());
      expect(capturedPath, '/api/v1/messages/msg-1/pin');
      final body = jsonDecode(capturedBody!) as Map<String, dynamic>;
      final chat = body['chat'] as Map<String, dynamic>;
      expect(chat['id'], 'chat-1');
    });
  });

  group('VoiceMessagesClient.unpinMessage', () {
    test('DELETE /api/v1/messages/{id}/pin with chat_id query', () async {
      String? capturedPath;
      final mock = MockClient((req) async {
        expect(req.method, 'DELETE');
        capturedPath = req.url.path;
        expect(req.url.queryParameters['chat_id'], 'chat-2');
        return http.Response('{}', 204);
      });
      final client = VoiceMessagesClient(
        gateway: gatewayHttpForTest(mock, config: config),
      );

      final result = await client.unpinMessage(
        authorization: auth,
        messageId: 'msg-2',
        chatId: 'chat-2',
      );

      expect(result, isA<MessagesApiOk<void>>());
      expect(capturedPath, '/api/v1/messages/msg-2/pin');
    });
  });

  group('VoiceMessagesClient.getPinnedMessages', () {
    test('GET /api/v1/chats/{id}/pinned-messages', () async {
      String? capturedPath;
      final mock = MockClient((req) async {
        expect(req.method, 'GET');
        capturedPath = req.url.path;
        return http.Response(
          jsonEncode({
            'message_list': {
              'messages': [
                {
                  'id': 'm1',
                  'chat': {'id': 'chat-3'},
                  'sender_profile_id': 'p1',
                  'content': 'pinned',
                  'is_pinned': true,
                  'reactions_json': '[]',
                  'mentions_json': '[]',
                  'attachments_json': '[]',
                  'type': 'regular',
                },
              ],
            },
          }),
          200,
          headers: {'content-type': 'application/json'},
        );
      });
      final client = VoiceMessagesClient(
        gateway: gatewayHttpForTest(mock, config: config),
      );

      final result = await client.getPinnedMessages(
        authorization: auth,
        chatId: 'chat-3',
      );

      expect(result, isA<MessagesApiOk<MessageListData>>());
      expect(capturedPath, '/api/v1/chats/chat-3/pinned-messages');
      final data = (result as MessagesApiOk<MessageListData>).data;
      expect(data.messages, hasLength(1));
      expect(data.messages.first.isPinned, isTrue);
    });
  });
}
