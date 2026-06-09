import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/gateway_config.dart';

import 'support/gateway_test_client.dart';

/// HTTP contract tests for PLAN Phase 4 groups (invite/kick/avatar).
/// VoiceChatsClient group methods are added with implementation; until then these
/// tests document expected Gateway routes via the same client surface.
void main() {
  const config = GatewayConfig(baseUrl: 'http://api.test');
  const auth = 'Bearer access-token';

  group('VoiceChatsClient.createGroup', () {
    test('POST /api/v1/chats with CHAT_TYPE_GROUP', () async {
      String? body;
      final mock = MockClient((req) async {
        expect(req.method, 'POST');
        expect(req.url.path, '/api/v1/chats');
        expect(req.headers['Authorization'], auth);
        body = req.body;
        return http.Response(
          jsonEncode({
            'chat': {
              'id': 'group-1',
              'type': 'CHAT_TYPE_GROUP',
              'name': 'Friday squad',
              'creator_profile_id': 'profile-a',
            },
          }),
          200,
        );
      });
      final client = VoiceChatsClient(gateway: gatewayHttpForTest(mock, config: config));
      final r = await client.createGroup(
        authorization: auth,
        name: 'Friday squad',
      );
      expect(r, isA<ChatsApiOk<VoiceChat>>());
      final chat = (r as ChatsApiOk<VoiceChat>).data;
      expect(chat.id, 'group-1');
      expect(chat.type, 'CHAT_TYPE_GROUP');
      expect(chat.name, 'Friday squad');
      final decoded = jsonDecode(body!) as Map<String, dynamic>;
      expect(decoded['type'], 'CHAT_TYPE_GROUP');
      expect(decoded['name'], 'Friday squad');
    });
  });

  group('VoiceChatsClient.addGroupMembers', () {
    test('POST /api/v1/chats/{chatId}/members', () async {
      String? body;
      final mock = MockClient((req) async {
        expect(req.method, 'POST');
        expect(req.url.path, '/api/v1/chats/group-1/members');
        body = req.body;
        return http.Response('', 204);
      });
      final client = VoiceChatsClient(gateway: gatewayHttpForTest(mock, config: config));
      final r = await client.addGroupMembers(
        authorization: auth,
        chatId: 'group-1',
        profileIds: const ['profile-b', 'profile-c'],
      );
      expect(r, isA<ChatsApiOk<void>>());
      final decoded = jsonDecode(body!) as Map<String, dynamic>;
      expect(decoded['profile_ids'], ['profile-b', 'profile-c']);
    });
  });

  group('VoiceChatsClient.removeGroupMember', () {
    test('DELETE /api/v1/chats/{chatId}/members/{profileId}', () async {
      String? path;
      final mock = MockClient((req) async {
        expect(req.method, 'DELETE');
        path = req.url.path;
        return http.Response('', 204);
      });
      final client = VoiceChatsClient(gateway: gatewayHttpForTest(mock, config: config));
      final r = await client.removeGroupMember(
        authorization: auth,
        chatId: 'group-1',
        profileId: 'profile-b',
      );
      expect(r, isA<ChatsApiOk<void>>());
      expect(path, '/api/v1/chats/group-1/members/profile-b');
    });
  });

  group('VoiceChatsClient.updateGroup', () {
    test('PATCH /api/v1/chats/{chatId} sets avatar_url', () async {
      String? body;
      final mock = MockClient((req) async {
        expect(req.method, 'PATCH');
        expect(req.url.path, '/api/v1/chats/group-1');
        body = req.body;
        return http.Response(
          jsonEncode({
            'chat': {
              'id': 'group-1',
              'type': 'CHAT_TYPE_GROUP',
              'avatar_url': 'https://cdn.voice.gg/groups/party.webp',
              'creator_profile_id': 'profile-a',
            },
          }),
          200,
        );
      });
      final client = VoiceChatsClient(gateway: gatewayHttpForTest(mock, config: config));
      final r = await client.updateGroup(
        authorization: auth,
        chatId: 'group-1',
        avatarUrl: 'https://cdn.voice.gg/groups/party.webp',
      );
      expect(r, isA<ChatsApiOk<VoiceChat>>());
      final chat = (r as ChatsApiOk<VoiceChat>).data;
      expect(chat.avatarUrl, 'https://cdn.voice.gg/groups/party.webp');
      final decoded = jsonDecode(body!) as Map<String, dynamic>;
      expect(decoded['avatar_url'], 'https://cdn.voice.gg/groups/party.webp');
    });
  });
}
