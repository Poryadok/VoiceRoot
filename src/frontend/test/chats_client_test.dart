import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/gateway_config.dart';

import 'support/gateway_test_client.dart';

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
      final client = VoiceChatsClient(gateway: gatewayHttpForTest(mock, config: config));
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

    test('GET /api/v1/chats supports requests inbox filter', () async {
      final mock = MockClient((req) async {
        expect(req.url.queryParameters['inbox'], 'requests');
        return http.Response(
          jsonEncode({
            'chat_list': {'items': []},
          }),
          200,
        );
      });
      final client = VoiceChatsClient(gateway: gatewayHttpForTest(mock, config: config));
      final r = await client.listChats(authorization: auth, inbox: 'requests');
      expect(r, isA<ChatsApiOk<ChatListData>>());
    });

    test('GET /api/v1/chats supports archive inbox filter', () async {
      final mock = MockClient((req) async {
        expect(req.url.queryParameters['inbox'], 'archive');
        return http.Response(
          jsonEncode({
            'chat_list': {'items': []},
          }),
          200,
        );
      });
      final client = VoiceChatsClient(gateway: gatewayHttpForTest(mock, config: config));
      final r = await client.listChats(authorization: auth, inbox: 'archive');
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
      final client = VoiceChatsClient(gateway: gatewayHttpForTest(mock, config: config));
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
        gateway: gatewayHttpForTest(
          MockClient((req) async {
            paths.add(req.url.path);
            return http.Response('', 204);
          }),
          config: config,
        ),
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

  group('VoiceChatsClient.archiveAndMute', () {
    test('POST archive and mute routes', () async {
      final paths = <String>[];
      final bodies = <String>[];
      final client = VoiceChatsClient(
        gateway: gatewayHttpForTest(
          MockClient((req) async {
            paths.add(req.url.path);
            bodies.add(req.body);
            return http.Response('', 204);
          }),
          config: config,
        ),
      );

      expect(
        await client.archiveChat(
          authorization: auth,
          chatId: 'chat-1',
          archived: true,
        ),
        isA<ChatsApiOk<void>>(),
      );
      expect(
        await client.muteChat(
          authorization: auth,
          chatId: 'chat-1',
          mutedUntil: DateTime.utc(2030, 1, 2, 3, 4, 5),
        ),
        isA<ChatsApiOk<void>>(),
      );
      expect(
        await client.muteChat(authorization: auth, chatId: 'chat-1'),
        isA<ChatsApiOk<void>>(),
      );
      expect(paths, [
        '/api/v1/chats/chat-1/archive',
        '/api/v1/chats/chat-1/mute',
        '/api/v1/chats/chat-1/mute',
      ]);
      expect(jsonDecode(bodies[0])['archived'], isTrue);
      expect(jsonDecode(bodies[1])['muted_until'], '2030-01-02T03:04:05.000Z');
      expect(jsonDecode(bodies[2]), isEmpty);
    });
  });

  group('VoiceChatsClient.quickAccess', () {
    test('GET/POST/DELETE/PUT quick-access routes', () async {
      final paths = <String>[];
      final methods = <String>[];
      final client = VoiceChatsClient(
        gateway: gatewayHttpForTest(
          MockClient((req) async {
            paths.add(req.url.path);
            methods.add(req.method);
            if (req.method == 'GET') {
              return http.Response(
                jsonEncode({
                  'items': [
                    {
                      'chat_id': 'chat-qa',
                      'sort_order': 1,
                      'chat': {
                        'id': 'chat-qa',
                        'type': 'CHAT_TYPE_DM',
                        'creator_profile_id': 'profile-a',
                        'name': 'QA Chat',
                      },
                    },
                  ],
                }),
                200,
              );
            }
            return http.Response('', 204);
          }),
          config: config,
        ),
      );

      final list = await client.listQuickAccess(authorization: auth);
      expect(list, isA<ChatsApiOk<QuickAccessListData>>());
      expect((list as ChatsApiOk<QuickAccessListData>).data.items, hasLength(1));
      expect(
        await client.addQuickAccess(authorization: auth, chatId: 'chat-qa'),
        isA<ChatsApiOk<void>>(),
      );
      expect(
        await client.removeQuickAccess(authorization: auth, chatId: 'chat-qa'),
        isA<ChatsApiOk<void>>(),
      );
      expect(
        await client.reorderQuickAccess(
          authorization: auth,
          chatIds: ['c2', 'c1'],
        ),
        isA<ChatsApiOk<void>>(),
      );
      expect(methods, ['GET', 'POST', 'DELETE', 'PUT']);
      expect(paths, [
        '/api/v1/chats/quick-access',
        '/api/v1/chats/quick-access',
        '/api/v1/chats/quick-access/chat-qa',
        '/api/v1/chats/quick-access/order',
      ]);
    });
  });

  group('VoiceChatsClient.folders', () {
    test('GET /api/v1/chats/folders parses folder_list', () async {
      final client = VoiceChatsClient(
        gateway: gatewayHttpForTest(
          MockClient((req) async {
            expect(req.url.path, '/api/v1/chats/folders');
            return http.Response(
              jsonEncode({
                'folder_list': {
                  'folders': [
                    {
                      'id': 'folder-all',
                      'name': 'All',
                      'folder_type': 'system',
                      'sort_order': 0,
                    },
                  ],
                },
              }),
              200,
            );
          }),
          config: config,
        ),
      );
      final result = await client.listFolders(authorization: auth);
      expect(result, isA<ChatsApiOk<FolderListData>>());
      final data = (result as ChatsApiOk<FolderListData>).data;
      expect(data.folders.single.id, 'folder-all');
      expect(data.folders.single.isSystem, isTrue);
    });

    test('GET /api/v1/chats passes folder_id filter', () async {
      final client = VoiceChatsClient(
        gateway: gatewayHttpForTest(
          MockClient((req) async {
            expect(req.url.queryParameters['folder_id'], 'folder-dm');
            return http.Response(
              jsonEncode({'chat_list': {'items': []}}),
              200,
            );
          }),
          config: config,
        ),
      );
      final result = await client.listChats(
        authorization: auth,
        folderId: 'folder-dm',
      );
      expect(result, isA<ChatsApiOk<ChatListData>>());
    });

    test('pin/unpin/reorder folder chat routes', () async {
      final paths = <String>[];
      final methods = <String>[];
      final client = VoiceChatsClient(
        gateway: gatewayHttpForTest(
          MockClient((req) async {
            paths.add(req.url.path);
            methods.add(req.method);
            return http.Response('', 204);
          }),
          config: config,
        ),
      );

      expect(
        await client.pinChatInFolder(
          authorization: auth,
          folderId: 'f1',
          chatId: 'c1',
        ),
        isA<ChatsApiOk<void>>(),
      );
      expect(
        await client.unpinChatInFolder(
          authorization: auth,
          folderId: 'f1',
          chatId: 'c1',
        ),
        isA<ChatsApiOk<void>>(),
      );
      expect(
        await client.reorderFolderChats(
          authorization: auth,
          folderId: 'f1',
          chatIds: ['c2', 'c1'],
        ),
        isA<ChatsApiOk<void>>(),
      );
      expect(methods, ['POST', 'DELETE', 'PUT']);
      expect(paths, [
        '/api/v1/chats/folders/f1/chats/c1/pin',
        '/api/v1/chats/folders/f1/chats/c1/pin',
        '/api/v1/chats/folders/f1/chats/order',
      ]);
    });
  });
}
