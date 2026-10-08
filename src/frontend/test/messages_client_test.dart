import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/messages_client.dart';
import 'package:voice_frontend/backend/proto_mappers.dart';
import 'package:voice_frontend/gen/voice/messaging/v1/messaging.pb.dart'
    as messaging_pb;

import 'support/gateway_test_client.dart';

void main() {
  const config = GatewayConfig(baseUrl: 'http://api.test');
  const auth = 'Bearer access-token';

  group('VoiceMessagesClient.getMessages', () {
    test(
      'maps deleted dm peer state without adding a message or changing paging',
      () async {
        final mock = MockClient((req) async {
          expect(req.method, 'GET');
          expect(req.url.path, '/api/v1/messages');
          return http.Response(
            jsonEncode({
              'message_list': {
                'messages': [
                  {
                    'id': 'msg-1',
                    'chat': {'id': 'chat-1'},
                    'sender_profile_id': 'profile-peer',
                    'content': 'History survives',
                    'created_at': '2024-01-01T00:00:00Z',
                  },
                ],
                'next_cursor': 'cursor-older',
                'has_more': true,
              },
              'dm_peer_state': 'DM_PEER_STATE_DELETED',
            }),
            200,
          );
        });
        final client = VoiceMessagesClient(
          gateway: gatewayHttpForTest(mock, config: config),
        );

        final result = await client.getMessages(
          authorization: auth,
          chatId: 'chat-1',
        );

        expect(result, isA<MessagesApiOk<MessageListData>>());
        final data = (result as MessagesApiOk<MessageListData>).data;
        expect(
          data.dmPeerState,
          messaging_pb.DmPeerState.DM_PEER_STATE_DELETED,
        );
        expect(data.messages, hasLength(1));
        expect(data.messages.single.id, 'msg-1');
        expect(data.messages.single.messageKind, VoiceMessageKind.regular);
        expect(data.nextCursor, 'cursor-older');
        expect(data.hasMore, isTrue);
      },
    );

    test('omitted dm peer state is not interpreted as deleted', () async {
      final mock = MockClient((_) async {
        return http.Response(
          jsonEncode({
            'message_list': {
              'messages': [],
              'next_cursor': 'cursor-next',
              'has_more': true,
            },
          }),
          200,
        );
      });
      final client = VoiceMessagesClient(
        gateway: gatewayHttpForTest(mock, config: config),
      );

      final result = await client.getMessages(
        authorization: auth,
        chatId: 'chat-1',
      );

      final data = (result as MessagesApiOk<MessageListData>).data;
      expect(data.dmPeerState, isNull);
      expect(data.messages, isEmpty);
      expect(data.nextCursor, 'cursor-next');
      expect(data.hasMore, isTrue);
    });

    test('unspecified dm peer state is not interpreted as deleted', () async {
      final mock = MockClient((_) async {
        return http.Response(
          jsonEncode({
            'message_list': {'messages': [], 'has_more': false},
            'dm_peer_state': 'DM_PEER_STATE_UNSPECIFIED',
          }),
          200,
        );
      });
      final client = VoiceMessagesClient(
        gateway: gatewayHttpForTest(mock, config: config),
      );

      final result = await client.getMessages(
        authorization: auth,
        chatId: 'chat-1',
      );

      final data = (result as MessagesApiOk<MessageListData>).data;
      expect(
        data.dmPeerState,
        messaging_pb.DmPeerState.DM_PEER_STATE_UNSPECIFIED,
      );
      expect(
        data.dmPeerState,
        isNot(equals(messaging_pb.DmPeerState.DM_PEER_STATE_DELETED)),
      );
      expect(data.messages, isEmpty);
    });

    test('thread history does not consume dm peer state', () async {
      final mock = MockClient((req) async {
        expect(req.url.path, '/api/v1/messages/thread');
        return http.Response(
          jsonEncode({
            'message_list': {
              'messages': [
                {
                  'id': 'reply-1',
                  'chat': {'id': 'chat-1'},
                  'sender_profile_id': 'profile-peer',
                  'content': 'Reply',
                },
              ],
            },
          }),
          200,
        );
      });
      final client = VoiceMessagesClient(
        gateway: gatewayHttpForTest(mock, config: config),
      );

      final result = await client.getThreadMessages(
        authorization: auth,
        chatId: 'chat-1',
        threadParentId: 'msg-1',
      );

      final data = (result as MessagesApiOk<MessageListData>).data;
      expect(data.dmPeerState, isNull);
      expect(data.messages.single.id, 'reply-1');
    });

    test('pinned history does not consume dm peer state', () async {
      final mock = MockClient((req) async {
        expect(req.url.path, '/api/v1/chats/chat-1/pinned-messages');
        return http.Response(
          jsonEncode({
            'message_list': {
              'messages': [
                {
                  'id': 'pinned-1',
                  'chat': {'id': 'chat-1'},
                  'sender_profile_id': 'profile-peer',
                  'content': 'Pinned',
                },
              ],
            },
          }),
          200,
        );
      });
      final client = VoiceMessagesClient(
        gateway: gatewayHttpForTest(mock, config: config),
      );

      final result = await client.getPinnedMessages(
        authorization: auth,
        chatId: 'chat-1',
      );

      final data = (result as MessagesApiOk<MessageListData>).data;
      expect(data.dmPeerState, isNull);
      expect(data.messages.single.id, 'pinned-1');
    });

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
      final client = VoiceMessagesClient(
        gateway: gatewayHttpForTest(mock, config: config),
      );
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
      final client = VoiceMessagesClient(
        gateway: gatewayHttpForTest(mock, config: config),
      );
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
        final client = VoiceMessagesClient(
          gateway: gatewayHttpForTest(mock, config: config),
        );
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
      final client = VoiceMessagesClient(
        gateway: gatewayHttpForTest(mock, config: config),
      );
      final r = await client.sendMessage(
        authorization: auth,
        chatId: 'chat-1',
        content: 'Sent',
      );
      expect(r, isA<MessagesApiOk<VoiceMessage>>());
      expect((r as MessagesApiOk<VoiceMessage>).data.id, 'msg-new');
    });

    test(
      'scheduled send preserves the documented request identity and mode',
      () async {
        final mock = MockClient((req) async {
          expect(req.method, 'POST');
          expect(req.url.path, '/api/v1/messages/send');
          final body = jsonDecode(req.body) as Map<String, dynamic>;
          expect(body['client_message_id'], 'schedule-retry-id');
          expect(body['send_when_online'], isTrue);
          expect(body.containsKey('scheduled_at'), isFalse);
          expect(body['chat'], {'id': 'chat-1'});
          return http.Response(
            jsonEncode({
              'scheduled_message': {
                'id': 'schedule-1',
                'chat': {'id': 'chat-1'},
                'sender_profile_id': 'profile-a',
                'client_message_id': 'schedule-retry-id',
                'send_when_online': true,
                'status': 'SCHEDULED_MESSAGE_STATUS_PENDING',
                'payload': {'content': 'Later'},
              },
            }),
            200,
          );
        });
        final client = VoiceMessagesClient(
          gateway: gatewayHttpForTest(mock, config: config),
        );
        final result = await client.scheduleMessage(
          authorization: auth,
          chatId: 'chat-1',
          content: 'Later',
          clientMessageId: 'schedule-retry-id',
          sendWhenOnline: true,
        );
        expect(result, isA<MessagesApiOk<messaging_pb.ScheduledMessage>>());
        expect(
          (result as MessagesApiOk<messaging_pb.ScheduledMessage>).data.id,
          'schedule-1',
        );
      },
    );

    test('scheduled send rejects an immediate-message response arm', () async {
      final mock = MockClient((_) async {
        return http.Response(
          jsonEncode({
            'message': {
              'id': 'msg-wrong-arm',
              'chat': {'id': 'chat-1'},
              'sender_profile_id': 'profile-a',
              'content': 'Later',
            },
          }),
          200,
        );
      });
      final client = VoiceMessagesClient(
        gateway: gatewayHttpForTest(mock, config: config),
      );
      final result = await client.scheduleMessage(
        authorization: auth,
        chatId: 'chat-1',
        content: 'Later',
        clientMessageId: 'schedule-id',
        scheduledAt: DateTime.utc(2026, 10, 10),
      );
      expect(result, isA<MessagesApiFailure>());
    });

    test(
      'scheduled send preserves an unavailable response as a retryable failure',
      () async {
        final mock = MockClient((_) async {
          return http.Response(
            jsonEncode({'code': 'UNAVAILABLE', 'message': 'temporary failure'}),
            503,
          );
        });
        final client = VoiceMessagesClient(
          gateway: gatewayHttpForTest(mock, config: config),
        );

        final result = await client.scheduleMessage(
          authorization: auth,
          chatId: 'chat-1',
          content: 'Later',
          clientMessageId: 'same-retry-id',
          scheduledAt: DateTime.utc(2026, 10, 10),
        );

        expect(result, isA<MessagesApiFailure>());
        expect((result as MessagesApiFailure).statusCode, 503);
      },
    );

    test(
      'schedule list uses chat-scoped cursor request and snake-case response',
      () async {
        final mock = MockClient((req) async {
          expect(req.method, 'GET');
          expect(req.url.path, '/api/v1/messages/scheduled');
          expect(req.url.queryParameters, {
            'chat_id': 'chat-1',
            'cursor': 'opaque-cursor',
            'page_size': '20',
          });
          return http.Response(
            jsonEncode({
              'scheduled_messages': [
                {
                  'id': 'schedule-1',
                  'chat': {'id': 'chat-1'},
                  'client_message_id': 'client-1',
                  'send_when_online': true,
                  'payload': {'content': 'Later'},
                },
              ],
              'page': {'next_cursor': 'next', 'has_more': true},
            }),
            200,
          );
        });
        final client = VoiceMessagesClient(
          gateway: gatewayHttpForTest(mock, config: config),
        );
        final result = await client.listScheduledMessages(
          authorization: auth,
          chatId: 'chat-1',
          cursor: 'opaque-cursor',
          pageSize: 20,
        );
        expect(
          result,
          isA<MessagesApiOk<messaging_pb.ListScheduledMessagesResponse>>(),
        );
        final response =
            (result
                    as MessagesApiOk<
                      messaging_pb.ListScheduledMessagesResponse
                    >)
                .data;
        expect(response.scheduledMessages.single.id, 'schedule-1');
        expect(response.page.nextCursor, 'next');
      },
    );

    test('cancel and send-now use the documented lifecycle routes', () async {
      var calls = 0;
      final mock = MockClient((req) async {
        calls++;
        if (calls == 1) {
          expect(req.method, 'DELETE');
          expect(req.url.path, '/api/v1/messages/scheduled/schedule-1');
          return http.Response('', 204);
        }
        expect(req.method, 'POST');
        expect(req.url.path, '/api/v1/messages/scheduled/schedule-1/send-now');
        final body = jsonDecode(req.body) as Map<String, dynamic>;
        expect(body['scheduled_message_id'], 'schedule-1');
        return http.Response(
          jsonEncode({
            'message': {
              'id': 'msg-now',
              'chat': {'id': 'chat-1'},
              'sender_profile_id': 'profile-a',
              'content': 'Later',
            },
          }),
          200,
        );
      });
      final client = VoiceMessagesClient(
        gateway: gatewayHttpForTest(mock, config: config),
      );
      expect(
        await client.cancelScheduledMessage(
          authorization: auth,
          scheduledMessageId: 'schedule-1',
        ),
        isA<MessagesApiOk<void>>(),
      );
      final result = await client.sendScheduledMessageNow(
        authorization: auth,
        scheduledMessageId: 'schedule-1',
      );
      expect(result, isA<MessagesApiOk<VoiceMessage>>());
      expect((result as MessagesApiOk<VoiceMessage>).data.id, 'msg-now');
    });

    test(
      'schedule update sends an optional payload and selected delivery arm',
      () async {
        final mock = MockClient((req) async {
          expect(req.method, 'PATCH');
          expect(req.url.path, '/api/v1/messages/scheduled/schedule-1');
          final body = jsonDecode(req.body) as Map<String, dynamic>;
          expect(body['scheduled_message_id'], 'schedule-1');
          expect(body['send_when_online'], isTrue);
          expect(body.containsKey('scheduled_at'), isFalse);
          expect(body['payload'], {'content': 'Updated'});
          return http.Response(
            jsonEncode({
              'scheduled_message': {
                'id': 'schedule-1',
                'chat': {'id': 'chat-1'},
                'client_message_id': 'client-1',
                'send_when_online': true,
                'payload': {'content': 'Updated'},
              },
            }),
            200,
          );
        });
        final client = VoiceMessagesClient(
          gateway: gatewayHttpForTest(mock, config: config),
        );
        final result = await client.updateScheduledMessage(
          authorization: auth,
          scheduledMessageId: 'schedule-1',
          payload: messaging_pb.ScheduledMessagePayload(content: 'Updated'),
          sendWhenOnline: true,
        );
        expect(result, isA<MessagesApiOk<messaging_pb.ScheduledMessage>>());
        expect(
          (result as MessagesApiOk<messaging_pb.ScheduledMessage>)
              .data
              .payload
              .content,
          'Updated',
        );
      },
    );

    test('schedule request mapper sets only the selected delivery oneof', () {
      final scheduled = sendMessageRequestToProto(
        chatId: 'chat-1',
        content: 'Later',
        scheduledAt: DateTime.utc(2026, 10, 10),
      );
      expect(scheduled.hasScheduledAt(), isTrue);
      expect(scheduled.whichDeliverySchedule().name, 'scheduledAt');
      final online = sendMessageRequestToProto(
        chatId: 'chat-1',
        content: 'When available',
        sendWhenOnline: true,
      );
      expect(online.sendWhenOnline, isTrue);
      expect(online.whichDeliverySchedule().name, 'sendWhenOnline');
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
      final client = VoiceMessagesClient(
        gateway: gatewayHttpForTest(mock, config: config),
      );
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
      expect(
        msg.mentions.single.targetId,
        '22222222-2222-2222-2222-222222222222',
      );
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
      final client = VoiceMessagesClient(
        gateway: gatewayHttpForTest(mock, config: config),
      );
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
      final client = VoiceMessagesClient(
        gateway: gatewayHttpForTest(mock, config: config),
      );
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
      final client = VoiceMessagesClient(
        gateway: gatewayHttpForTest(mock, config: config),
      );
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
      final client = VoiceMessagesClient(
        gateway: gatewayHttpForTest(mock, config: config),
      );
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
      final client = VoiceMessagesClient(
        gateway: gatewayHttpForTest(mock, config: config),
      );
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
