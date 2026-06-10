import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/gateway_http.dart';
import 'package:voice_frontend/backend/spaces_client.dart';

void main() {
  test('listSpaceTree parses categories and mixed nodes', () async {
    final client = VoiceSpacesClient(
      gateway: GatewayHttpClient(
        httpClient: MockClient((request) async {
          expect(request.url.path, '/api/v1/spaces/space-1/tree');
          return http.Response(
            jsonEncode({
              'categories': [
                {'id': 'cat-1', 'space_id': 'space-1', 'name': 'General', 'sort_order': 0},
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
}
