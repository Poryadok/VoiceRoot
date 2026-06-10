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
      final client = VoiceMessagesClient(gateway: gatewayHttpForTest(mock, config: config));
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

    test('parses attachments_json into message attachments', () async {
      final mock = MockClient((req) async {
        return http.Response(
          jsonEncode({
            'message_list': {
              'messages': [
                {
                  'id': 'msg-media',
                  'chat': {'id': 'chat-1'},
                  'sender_profile_id': 'profile-b',
                  'content': '',
                  'attachments_json': jsonEncode([
                    {
                      'file_id': 'file-1',
                      'type': 'image',
                      'url': 'https://cdn.example/full.webp',
                      'preview_url': 'https://cdn.example/thumb.webp',
                      'name': 'cat.png',
                      'size_bytes': 2048,
                    },
                  ]),
                },
              ],
            },
          }),
          200,
        );
      });
      final client = VoiceMessagesClient(gateway: gatewayHttpForTest(mock, config: config));
      final r = await client.getMessages(authorization: auth, chatId: 'chat-1');
      final message =
          (r as MessagesApiOk<MessageListData>).data.messages.single;

      expect(message.attachments.single.fileId, 'file-1');
      expect(
        message.attachments.single.previewUrl,
        'https://cdn.example/thumb.webp',
      );
      expect(message.attachments.single.isImage, isTrue);
    });

    test(
      'GET /api/v1/messages with last_message_id for reconnect catch-up',
      () async {
        final mock = MockClient((req) async {
          expect(req.url.queryParameters['last_message_id'], 'msg-last');
          return http.Response(
            jsonEncode({
              'message_list': {'messages': [], 'has_more': false},
            }),
            200,
          );
        });
        final client = VoiceMessagesClient(gateway: gatewayHttpForTest(mock, config: config));
        final r = await client.getMessages(
          authorization: auth,
          chatId: 'chat-1',
          lastMessageId: 'msg-last',
        );
        expect(r, isA<MessagesApiOk<MessageListData>>());
        expect((r as MessagesApiOk<MessageListData>).data.messages, isEmpty);
        expect(r.data.hasMore, isFalse);
      },
    );
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
      final client = VoiceMessagesClient(gateway: gatewayHttpForTest(mock, config: config));
      final r = await client.sendMessage(
        authorization: auth,
        chatId: 'chat-1',
        content: 'Sent',
      );
      expect(r, isA<MessagesApiOk<VoiceMessage>>());
      expect((r as MessagesApiOk<VoiceMessage>).data.id, 'msg-new');
    });

    test('POST /api/v1/messages/send includes mentions_json', () async {
      final mock = MockClient((req) async {
        final body = jsonDecode(req.body) as Map<String, dynamic>;
        final mentions =
            jsonDecode(body['mentions_json'] as String) as List<dynamic>;
        expect(mentions.single, containsPair('type', 'user'));
        expect(
          mentions.single,
          containsPair('target_id', '22222222-2222-2222-2222-222222222222'),
        );
        return http.Response(
          jsonEncode({
            'message': {
              'id': 'msg-mention',
              'chat': {'id': 'chat-1'},
              'sender_profile_id': 'profile-a',
              'content': 'hey',
              'mentions_json': body['mentions_json'],
            },
          }),
          200,
        );
      });
      final client = VoiceMessagesClient(gateway: gatewayHttpForTest(mock, config: config));
      final r = await client.sendMessage(
        authorization: auth,
        chatId: 'chat-1',
        content: 'hey @user',
        mentions: const [
          MessageMention(
            type: 'user',
            targetId: '22222222-2222-2222-2222-222222222222',
          ),
        ],
      );
      expect(r, isA<MessagesApiOk<VoiceMessage>>());
      final msg = (r as MessagesApiOk<VoiceMessage>).data;
      expect(msg.mentions.single.targetId, '22222222-2222-2222-2222-222222222222');
    });

    test('POST /api/v1/messages/send includes attachments_json', () async {
      final mock = MockClient((req) async {
        final body = jsonDecode(req.body) as Map<String, dynamic>;
        final attachments =
            jsonDecode(body['attachments_json'] as String) as List<dynamic>;
        expect(attachments.single, containsPair('file_id', 'file-1'));
        expect(
          attachments.single,
          containsPair('preview_url', 'https://cdn.example/thumb.webp'),
        );
        return http.Response(
          jsonEncode({
            'message': {
              'id': 'msg-attachment',
              'chat': {'id': 'chat-1'},
              'sender_profile_id': 'profile-a',
              'content': '',
              'attachments_json': body['attachments_json'],
            },
          }),
          200,
        );
      });
      final client = VoiceMessagesClient(gateway: gatewayHttpForTest(mock, config: config));
      final r = await client.sendMessage(
        authorization: auth,
        chatId: 'chat-1',
        content: '',
        attachments: const [
          MessageAttachment(
            fileId: 'file-1',
            type: 'image',
            url: 'https://cdn.example/full.webp',
            previewUrl: 'https://cdn.example/thumb.webp',
          ),
        ],
      );

      expect(r, isA<MessagesApiOk<VoiceMessage>>());
      expect(
        (r as MessagesApiOk<VoiceMessage>).data.attachments.single.fileId,
        'file-1',
      );
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
      final client = VoiceMessagesClient(gateway: gatewayHttpForTest(mock, config: config));
      final r = await client.markRead(
        authorization: auth,
        chatId: 'chat-1',
        lastReadMessageId: 'msg-9',
      );
      expect(r, isA<MessagesApiOk<void>>());
    });
  });

  group('VoiceMessagesClient.editDelete', () {
    test('PATCH /api/v1/messages/{id}', () async {
      final mock = MockClient((req) async {
        expect(req.method, 'PATCH');
        expect(req.url.path, '/api/v1/messages/msg-1');
        final body = jsonDecode(req.body) as Map<String, dynamic>;
        expect(body['content'], 'edited');
        return http.Response(
          jsonEncode({
            'message': {
              'id': 'msg-1',
              'chat': {'id': 'chat-1'},
              'sender_profile_id': 'profile-a',
              'content': 'edited',
              'edited_at': '2024-01-03T00:00:00Z',
            },
          }),
          200,
        );
      });
      final client = VoiceMessagesClient(gateway: gatewayHttpForTest(mock, config: config));
      final r = await client.editMessage(
        authorization: auth,
        messageId: 'msg-1',
        content: 'edited',
      );
      expect(r, isA<MessagesApiOk<VoiceMessage>>());
      expect((r as MessagesApiOk<VoiceMessage>).data.editedAt, isNotNull);
    });

    test('DELETE /api/v1/messages/{id}?scope=me', () async {
      final mock = MockClient((req) async {
        expect(req.method, 'DELETE');
        expect(req.url.path, '/api/v1/messages/msg-1');
        expect(req.url.queryParameters['scope'], 'me');
        return http.Response('', 204);
      });
      final client = VoiceMessagesClient(gateway: gatewayHttpForTest(mock, config: config));
      final r = await client.deleteMessage(
        authorization: auth,
        messageId: 'msg-1',
        scope: 'me',
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
      final client = VoiceMessagesClient(gateway: gatewayHttpForTest(mock, config: config));
      final r = await client.getReadState(
        authorization: auth,
        chatId: 'chat-1',
      );
      expect(r, isA<MessagesApiOk<ReadStateData>>());
      final data = (r as MessagesApiOk<ReadStateData>).data;
      expect(data.lastReadMessageId, 'msg-9');
      expect(data.profileId, 'profile-b');
    });
  });

  group('VoiceMessagesClient.getMessages reactions_json', () {
    test('parses reactions_json from getMessages proto response', () async {
      final mock = MockClient((req) async {
        return utf8JsonResponse(
          jsonEncode({
            'message_list': {
              'messages': [
                {
                  'id': 'msg-react',
                  'chat': {'id': 'chat-1'},
                  'sender_profile_id': 'profile-b',
                  'content': 'hi',
                  'reactions_json': jsonEncode([
                    {'emoji': '👍', 'count': 2, 'reacted_by_me': true},
                  ]),
                  'created_at': '2024-01-02T00:00:00Z',
                },
              ],
            },
          }),
        );
      });
      final client = VoiceMessagesClient(
        gateway: gatewayHttpForTest(mock, config: config),
      );
      final r = await client.getMessages(authorization: auth, chatId: 'chat-1');
      expect(r, isA<MessagesApiOk<MessageListData>>());
      final msg = (r as MessagesApiOk<MessageListData>).data.messages.single;
      expect(msg.reactions, hasLength(1));
      expect(msg.reactions.single.count, 2);
    });
  });

  group('VoiceMessage reactions_json', () {
    test('parses aggregated emoji counters and reacted_by_me', () {
      final msg = VoiceMessage.fromJson({
        'id': 'msg-react',
        'chat': {'id': 'chat-1'},
        'sender_profile_id': 'profile-b',
        'content': 'hi',
        'reactions_json': jsonEncode([
          {'emoji': '👍', 'count': 3, 'reacted_by_me': true},
          {'emoji': '🔥', 'count': 1, 'reacted_by_me': false},
        ]),
      });

      expect(msg.reactions, hasLength(2));
      expect(msg.reactions.first.emoji, '👍');
      expect(msg.reactions.first.count, 3);
      expect(msg.reactions.first.reactedByMe, isTrue);
      expect(msg.reactions.last.emoji, '🔥');
      expect(msg.reactions.last.count, 1);
      expect(msg.reactions.last.reactedByMe, isFalse);
    });
  });
}
