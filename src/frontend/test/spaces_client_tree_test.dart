import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/gateway_http.dart';
import 'package:voice_frontend/backend/spaces_client.dart';
import 'package:voice_frontend/gen/voice/chat/v1/chat.pbenum.dart';

void main() {
  test('listSpaceTree parses categories and mixed nodes', () async {
    final client = VoiceSpacesClient(
      gateway: GatewayHttpClient(
        httpClient: MockClient((request) async {
          expect(request.url.path, '/api/v1/spaces/space-1/tree');
          return http.Response(
            jsonEncode({
              'categories': [
                {
                  'id': 'cat-1',
                  'space_id': 'space-1',
                  'name': 'General',
                  'sort_order': 0,
                },
              ],
              'nodes': [
                {
                  'id': 'node-text',
                  'space_id': 'space-1',
                  'category_id': 'cat-1',
                  'kind': 'text_chat',
                  'linked_chat': {'id': 'chat-1'},
                  'sort_order': 0,
                  'is_system': false,
                },
                {
                  'id': 'node-voice',
                  'space_id': 'space-1',
                  'kind': 'voice_room',
                  'voice_room_id': 'vr-1',
                  'sort_order': 1,
                  'is_system': false,
                },
              ],
              'voice_rooms': [
                {'id': 'vr-1', 'space_id': 'space-1', 'name': 'Lobby'},
              ],
            }),
            200,
          );
        }),
        config: const GatewayConfig(baseUrl: 'http://api.test'),
      ),
    );

    final result = await client.listSpaceTree(
      authorization: 'Bearer t',
      spaceId: 'space-1',
    );

    expect(result, isA<SpacesApiOk<SpaceTreeData>>());
    final tree = (result as SpacesApiOk<SpaceTreeData>).data;
    expect(tree.categories, hasLength(1));
    expect(tree.nodes, hasLength(2));
    expect(tree.nodes.first.linkedChatId, 'chat-1');
    expect(tree.nodes.last.displayName, 'Lobby');
  });

  test('createCategory posts to space categories route', () async {
    String? path;
    final client = VoiceSpacesClient(
      gateway: GatewayHttpClient(
        httpClient: MockClient((request) async {
          path = request.url.path;
          return http.Response(
            jsonEncode({
              'category': {
                'id': 'cat-new',
                'space_id': 'space-1',
                'name': 'General',
                'sort_order': 0,
              },
            }),
            200,
          );
        }),
        config: const GatewayConfig(baseUrl: 'http://api.test'),
      ),
    );

    final result = await client.createCategory(
      authorization: 'Bearer t',
      spaceId: 'space-1',
      name: 'General',
    );

    expect(path, '/api/v1/spaces/space-1/categories');
    expect(result, isA<SpacesApiOk<SpaceCategory>>());
    expect((result as SpacesApiOk<SpaceCategory>).data.name, 'General');
  });

  test('createVoiceRoom parses voice room response', () async {
    final client = VoiceSpacesClient(
      gateway: GatewayHttpClient(
        httpClient: MockClient((request) async {
          return http.Response(
            jsonEncode({
              'voice_room': {
                'id': 'vr-1',
                'space_id': 'space-1',
                'name': 'Lobby',
              },
            }),
            200,
          );
        }),
        config: const GatewayConfig(baseUrl: 'http://api.test'),
      ),
    );

    final result = await client.createVoiceRoom(
      authorization: 'Bearer t',
      spaceId: 'space-1',
      name: 'Lobby',
    );

    expect(result, isA<SpacesApiOk<VoiceRoomData>>());
    expect((result as SpacesApiOk<VoiceRoomData>).data.name, 'Lobby');
  });

  test('createSpaceChat maps proto display_name onto tree node', () async {
    final client = VoiceSpacesClient(
      gateway: GatewayHttpClient(
        httpClient: MockClient((request) async {
          expect(request.url.path, '/api/v1/spaces/space-1/chats');
          return http.Response(
            jsonEncode({
              'space_tree_node': {
                'id': 'node-new',
                'space_id': 'space-1',
                'kind': 'text_chat',
                'linked_chat': {'id': 'chat-99', 'type': 'CHAT_TYPE_CHANNEL'},
                'display_name': 'announcements',
              },
            }),
            200,
          );
        }),
        config: const GatewayConfig(baseUrl: 'http://api.test'),
      ),
    );

    final result = await client.createSpaceChat(
      authorization: 'Bearer t',
      spaceId: 'space-1',
      name: 'announcements',
      chatType: ChatType.CHAT_TYPE_CHANNEL,
    );

    expect(result, isA<SpacesApiOk<SpaceTreeNodeData>>());
    final node = (result as SpacesApiOk<SpaceTreeNodeData>).data;
    expect(node.displayName, 'announcements');
    expect(node.isChannelChat, isTrue);
  });

  test(
    'createSpaceChat assigns the returned node to a selected category',
    () async {
      final paths = <String>[];
      final client = VoiceSpacesClient(
        gateway: GatewayHttpClient(
          httpClient: MockClient((request) async {
            paths.add(request.url.path);
            if (request.url.path.endsWith('/chats')) {
              return http.Response(
                jsonEncode({
                  'space_tree_node': {
                    'id': 'node-new',
                    'space_id': 'space-1',
                    'kind': 'text_chat',
                    'linked_chat': {
                      'id': 'chat-99',
                      'type': 'CHAT_TYPE_CHANNEL',
                    },
                    'display_name': 'announcements',
                  },
                }),
                200,
              );
            }
            expect(request.url.path, '/api/v1/spaces/space-1/tree/nodes');
            final body = jsonDecode(request.body) as Map<String, dynamic>;
            expect(body['node_id'], 'node-new');
            expect(body['category_id'], 'cat-1');
            expect(body['kind'], 'text_chat');
            return http.Response(
              jsonEncode({
                'space_tree_node': {
                  'id': 'node-new',
                  'space_id': 'space-1',
                  'category_id': 'cat-1',
                  'kind': 'text_chat',
                  'linked_chat': {'id': 'chat-99', 'type': 'CHAT_TYPE_CHANNEL'},
                  'display_name': 'announcements',
                },
              }),
              200,
            );
          }),
          config: const GatewayConfig(baseUrl: 'http://api.test'),
        ),
      );

      final result = await client.createSpaceChat(
        authorization: 'Bearer t',
        spaceId: 'space-1',
        name: 'announcements',
        chatType: ChatType.CHAT_TYPE_CHANNEL,
        categoryId: 'cat-1',
      );

      expect(paths, [
        '/api/v1/spaces/space-1/chats',
        '/api/v1/spaces/space-1/tree/nodes',
      ]);
      expect(result, isA<SpacesApiOk<SpaceTreeNodeData>>());
      expect(
        (result as SpacesApiOk<SpaceTreeNodeData>).data.categoryId,
        'cat-1',
      );
    },
  );

  test(
    'category assignment failure marks a newly created chat as partial success',
    () async {
      final client = VoiceSpacesClient(
        gateway: GatewayHttpClient(
          httpClient: MockClient((request) async {
            if (request.url.path.endsWith('/chats')) {
              return http.Response(
                jsonEncode({
                  'space_tree_node': {
                    'id': 'node-new',
                    'space_id': 'space-1',
                    'kind': 'text_chat',
                    'linked_chat': {'id': 'chat-new'},
                    'display_name': 'announcements',
                  },
                }),
                200,
              );
            }
            return http.Response(
              jsonEncode({'message': 'tree permission denied'}),
              403,
            );
          }),
          config: const GatewayConfig(baseUrl: 'http://api.test'),
        ),
      );

      final result = await client.createSpaceChat(
        authorization: 'Bearer t',
        spaceId: 'space-1',
        name: 'announcements',
        categoryId: 'cat-1',
      );

      expect(result, isA<SpacesApiFailure>());
      final failure = result as SpacesApiFailure;
      expect(failure.statusCode, 403);
      expect(failure.partialSuccess, isTrue);
      expect(failure.message, 'tree permission denied');
    },
  );

  test('category assignment 5xx marks placement as uncertain', () async {
    final client = VoiceSpacesClient(
      gateway: GatewayHttpClient(
        httpClient: MockClient((request) async {
          if (request.url.path.endsWith('/chats')) {
            return http.Response(
              jsonEncode({
                'space_tree_node': {
                  'id': 'node-new',
                  'space_id': 'space-1',
                  'kind': 'text_chat',
                  'linked_chat': {'id': 'chat-new'},
                  'display_name': 'announcements',
                },
              }),
              200,
            );
          }
          return http.Response(
            jsonEncode({'message': 'upstream timeout'}),
            503,
          );
        }),
        config: const GatewayConfig(baseUrl: 'http://api.test'),
      ),
    );

    final result = await client.createSpaceChat(
      authorization: 'Bearer t',
      spaceId: 'space-1',
      name: 'announcements',
      categoryId: 'cat-1',
    );

    expect(result, isA<SpacesApiFailure>());
    final failure = result as SpacesApiFailure;
    expect(failure.partialSuccess, isTrue);
    expect(failure.outcomeUncertain, isTrue);
    expect(failure.statusCode, 503);
  });

  test(
    'category assignment network failure marks placement as uncertain',
    () async {
      final client = VoiceSpacesClient(
        gateway: GatewayHttpClient(
          httpClient: MockClient((request) async {
            if (request.url.path.endsWith('/chats')) {
              return http.Response(
                jsonEncode({
                  'space_tree_node': {
                    'id': 'node-new',
                    'space_id': 'space-1',
                    'kind': 'text_chat',
                    'linked_chat': {'id': 'chat-new'},
                    'display_name': 'announcements',
                  },
                }),
                200,
              );
            }
            throw const SocketException('offline');
          }),
          config: const GatewayConfig(baseUrl: 'http://api.test'),
        ),
      );

      final result = await client.createSpaceChat(
        authorization: 'Bearer t',
        spaceId: 'space-1',
        name: 'announcements',
        categoryId: 'cat-1',
      );

      expect(result, isA<SpacesApiFailure>());
      final failure = result as SpacesApiFailure;
      expect(failure.partialSuccess, isTrue);
      expect(failure.outcomeUncertain, isTrue);
      expect(failure.errorCode, 'network_error');
    },
  );

  test('createSpaceChat marks gateway 5xx as an uncertain outcome', () async {
    final client = VoiceSpacesClient(
      gateway: GatewayHttpClient(
        httpClient: MockClient(
          (_) async =>
              http.Response(jsonEncode({'message': 'internal error'}), 503),
        ),
        config: const GatewayConfig(baseUrl: 'http://api.test'),
      ),
    );

    final result = await client.createSpaceChat(
      authorization: 'Bearer t',
      spaceId: 'space-1',
      name: 'announcements',
    );

    expect(result, isA<SpacesApiFailure>());
    final failure = result as SpacesApiFailure;
    expect(failure.statusCode, 503);
    expect(failure.outcomeUncertain, isTrue);
  });

  test(
    'createSpaceChat marks a network exception as an uncertain outcome',
    () async {
      final client = VoiceSpacesClient(
        gateway: GatewayHttpClient(
          httpClient: MockClient(
            (_) async => throw const SocketException('offline'),
          ),
          config: const GatewayConfig(baseUrl: 'http://api.test'),
        ),
      );

      final result = await client.createSpaceChat(
        authorization: 'Bearer t',
        spaceId: 'space-1',
        name: 'announcements',
      );

      expect(result, isA<SpacesApiFailure>());
      final failure = result as SpacesApiFailure;
      expect(failure.errorCode, 'network_error');
      expect(failure.outcomeUncertain, isTrue);
    },
  );

  test(
    'reorderSpaceTree posts ordered node ids to the tree reorder route',
    () async {
      final client = VoiceSpacesClient(
        gateway: GatewayHttpClient(
          httpClient: MockClient((request) async {
            expect(request.method, 'POST');
            expect(request.url.path, '/api/v1/spaces/space-1/tree/reorder');
            expect(jsonDecode(request.body), {
              'space_id': 'space-1',
              'ordered_node_ids': ['node-2', 'node-1'],
            });
            return http.Response('', 204);
          }),
          config: const GatewayConfig(baseUrl: 'http://api.test'),
        ),
      );

      final result = await client.reorderSpaceTree(
        authorization: 'Bearer t',
        spaceId: 'space-1',
        orderedNodeIds: const ['node-2', 'node-1'],
      );

      expect(
        result,
        isA<SpacesApiOk<void>>(),
        reason: result is SpacesApiFailure
            ? '${result.statusCode}: ${result.message}'
            : null,
      );
    },
  );
}
