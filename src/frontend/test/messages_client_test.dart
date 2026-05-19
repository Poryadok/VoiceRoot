import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/messages_client.dart';

void main() {
  const config = GatewayConfig(baseUrl: 'http://api.test');
  const auth = 'Bearer access-token';

  group('VoiceMessagesClient.getMessages', () {
    test('GET /api/v1/messages with chat_id and after_message_id', () async {
      final mock = MockClient((req) async {
        expect(req.method, 'GET');
        expect(req.url.path, '/api/v1/messages');
        expect(req.url.queryParameters['chat_id'], 'chat-1');
        expect(req.url.queryParameters['after_message_id'], 'msg-1');
        return http.Response(
          jsonEncode({
            'message_list': {
              'messages': [
                {
                  'id': 'msg-2',
                  'chat': {'id': 'chat-1'},
                  'sender_profile_id': 'profile-b',
                  'content': 'Hi',
                  'created_at': '2024-01-02T00:00:00Z',
                },
              ],
              'has_more': false,
            },
          }),
          200,
        );
      });
      final client = VoiceMessagesClient(httpClient: mock, config: config);
      final r = await client.getMessages(
        authorization: auth,
        chatId: 'chat-1',
        afterMessageId: 'msg-1',
      );
      expect(r, isA<MessagesApiOk<MessageListData>>());
      final data = (r as MessagesApiOk<MessageListData>).data;
      expect(data.messages.single.id, 'msg-2');
      expect(data.messages.single.content, 'Hi');
    });

    test('GET /api/v1/messages with last_message_id for reconnect catch-up', () async {
      final mock = MockClient((req) async {
        expect(req.url.queryParameters['last_message_id'], 'msg-last');
        return http.Response(
          jsonEncode({
            'message_list': {
              'messages': [],
              'has_more': false,
            },
          }),
          200,
        );
      });
      final client = VoiceMessagesClient(httpClient: mock, config: config);
      await client.getMessages(
        authorization: auth,
        chatId: 'chat-1',
        lastMessageId: 'msg-last',
      );
    });
  });

  group('VoiceMessagesClient.sendMessage', () {
    test('POST /api/v1/messages/send', () async {
      final mock = MockClient((req) async {
        expect(req.method, 'POST');
        expect(req.url.path, '/api/v1/messages/send');
        return http.Response(
          jsonEncode({
            'message': {
              'id': 'msg-new',
              'chat': {'id': 'chat-1'},
              'sender_profile_id': 'profile-a',
              'content': 'Sent',
              'created_at': '2024-01-03T00:00:00Z',
            },
          }),
          200,
        );
      });
      final client = VoiceMessagesClient(httpClient: mock, config: config);
      final r = await client.sendMessage(
        authorization: auth,
        chatId: 'chat-1',
        content: 'Sent',
      );
      expect(r, isA<MessagesApiOk<VoiceMessage>>());
      expect((r as MessagesApiOk<VoiceMessage>).data.id, 'msg-new');
    });
  });

  group('VoiceMessagesClient.markRead', () {
    test('POST /api/v1/messages/read', () async {
      final mock = MockClient((req) async {
        expect(req.method, 'POST');
        expect(req.url.path, '/api/v1/messages/read');
        final body = jsonDecode(req.body) as Map<String, dynamic>;
        expect(body['chat'], {'id': 'chat-1'});
        expect(body['last_read_message_id'], 'msg-9');
        return http.Response('{}', 200);
      });
      final client = VoiceMessagesClient(httpClient: mock, config: config);
      final r = await client.markRead(
        authorization: auth,
        chatId: 'chat-1',
        lastReadMessageId: 'msg-9',
      );
      expect(r, isA<MessagesApiOk<void>>());
    });
  });

  group('VoiceMessagesClient.getReadState', () {
    test('GET /api/v1/messages/read-state', () async {
      final mock = MockClient((req) async {
        expect(req.method, 'GET');
        expect(req.url.path, '/api/v1/messages/read-state');
        expect(req.url.queryParameters['chat_id'], 'chat-1');
        return http.Response(
          jsonEncode({
            'read_state': {
              'chat': {'id': 'chat-1'},
              'profile_id': 'profile-b',
              'last_read_message_id': 'msg-9',
            },
          }),
          200,
        );
      });
      final client = VoiceMessagesClient(httpClient: mock, config: config);
      final r = await client.getReadState(authorization: auth, chatId: 'chat-1');
      expect(r, isA<MessagesApiOk<ReadStateData>>());
      final data = (r as MessagesApiOk<ReadStateData>).data;
      expect(data.lastReadMessageId, 'msg-9');
      expect(data.profileId, 'profile-b');
    });
  });
}
