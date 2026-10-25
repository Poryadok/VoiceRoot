import 'dart:convert';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'support/gateway_test_client.dart';

void main() {
  test(
    'caller permissions and distinct mutations use authenticated chat routes',
    () async {
      final requests = <http.Request>[];
      final client = VoiceChatsClient(
        gateway: gatewayHttpForTest(
          MockClient((r) async {
            requests.add(r);
            expect(r.headers['Authorization'], 'Bearer actor');
            if (r.method == 'GET') {
              return http.Response(
                jsonEncode({
                  'can_post_as_chat': false,
                  'can_add_members': true,
                  'can_manage_permissions': false,
                  'member_additions_enabled': true,
                  'explicit_post_as_chat': false,
                }),
                200,
              );
            }
            return http.Response('', 204);
          }),
          config: const GatewayConfig(baseUrl: 'http://api.test'),
        ),
      );
      final result = await client.getChatPermissions(
        authorization: 'Bearer actor',
        chatId: 'chat-1',
      );
      expect(result, isA<ChatsApiOk<ChatPermissions>>());
      final p = (result as ChatsApiOk<ChatPermissions>).data;
      expect(p.canPostAsChat, isFalse);
      expect(p.canAddMembers, isTrue);
      expect(p.canManagePermissions, isFalse);
      expect(requests.single.url.path, '/api/v1/chats/chat-1/permissions');
      await client.setMemberPostAsChat(
        authorization: 'Bearer actor',
        chatId: 'chat-1',
        profileId: 'member-1',
        allowed: true,
      );
      expect(requests.last.method, 'PUT');
      expect(
        requests.last.url.path,
        '/api/v1/chats/chat-1/members/member-1/post-as-chat',
      );
      expect(jsonDecode(requests.last.body), {'allowed': true});
    },
  );
}
