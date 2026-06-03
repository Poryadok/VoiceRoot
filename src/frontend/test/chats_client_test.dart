import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/gateway_config.dart';

void main() {
  const config = GatewayConfig(baseUrl: 'http://api.test');
  const auth = 'Bearer access-token';

  group('VoiceChatsClient.listChats', () {
    test('GET /api/v1/chats parses chat_list items', () async {
      final mock = MockClient((req) async {
        expect(req.method, 'GET');
        expect(req.url.path, '/api/v1/chats');
        expect(req.headers['Authorization'], auth);
        return http.Response(
          jsonEncode({
            'chat_list': {
              'items': [
                {
                  'chat': {
                    'id': 'chat-1',
                    'type': 'CHAT_TYPE_DM',
                    'creator_profile_id': 'profile-a',
                  },
                  'last_message_preview': 'Hello',
                  'unread_count': '2',
                  'inbox': 'requests',
                  'is_stranger': true,
                },
              ],
              'next_cursor': 'cursor-2',
            },
          }),
          200,
        );
      });
      final client = VoiceChatsClient(httpClient: mock, config: config);
      final r = await client.listChats(authorization: auth);
      expect(r, isA<ChatsApiOk<ChatListData>>());
      final data = (r as ChatsApiOk<ChatListData>).data;
      expect(data.items, hasLength(1));
      expect(data.items.first.chatId, 'chat-1');
      expect(data.items.first.lastMessagePreview, 'Hello');
      expect(data.items.first.unreadCount, 2);
      expect(data.items.first.inbox, 'requests');
      expect(data.items.first.isStranger, isTrue);
      expect(data.nextCursor, 'cursor-2');
    });

    test('GET /api/v1/chats supports inbox filter', () async {
      final mock = MockClient((req) async {
        expect(req.url.queryParameters['inbox'], 'requests');
        return http.Response(
          jsonEncode({
            'chat_list': {'items': []},
          }),
          200,
        );
      });
      final client = VoiceChatsClient(httpClient: mock, config: config);
      final r = await client.listChats(authorization: auth, inbox: 'requests');
      expect(r, isA<ChatsApiOk<ChatListData>>());
    });
  });

  group('VoiceChatsClient.createDm', () {
    test('POST /api/v1/chats/dm', () async {
      String? body;
      final mock = MockClient((req) async {
        expect(req.method, 'POST');
        expect(req.url.path, '/api/v1/chats/dm');
        body = req.body;
        return http.Response(
          jsonEncode({
            'chat': {
              'id': 'chat-dm',
              'type': 'CHAT_TYPE_DM',
              'creator_profile_id': 'profile-a',
            },
          }),
          200,
        );
      });
      final client = VoiceChatsClient(httpClient: mock, config: config);
      final r = await client.createDm(
        authorization: auth,
        otherProfileId: 'profile-b',
      );
      expect(r, isA<ChatsApiOk<VoiceChat>>());
      expect((r as ChatsApiOk<VoiceChat>).data.id, 'chat-dm');
      expect(jsonDecode(body!)['other_profile_id'], 'profile-b');
    });
  });

  group('VoiceChatsClient.dmRequests', () {
    test('POST accept and decline request routes', () async {
      final paths = <String>[];
      final client = VoiceChatsClient(
        httpClient: MockClient((req) async {
          paths.add(req.url.path);
          return http.Response('', 204);
        }),
        config: config,
      );

      expect(
        await client.acceptDmRequest(authorization: auth, chatId: 'chat-1'),
        isA<ChatsApiOk<void>>(),
      );
      expect(
        await client.declineDmRequest(authorization: auth, chatId: 'chat-1'),
        isA<ChatsApiOk<void>>(),
      );
      expect(paths, [
        '/api/v1/chats/chat-1/accept-request',
        '/api/v1/chats/chat-1/decline-request',
      ]);
    });
  });
}
