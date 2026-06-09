import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/voice_client.dart';

import 'support/gateway_test_client.dart';

/// HTTP contract tests for PLAN Phase 4 group voice (до 32, join active call).
void main() {
  const config = GatewayConfig(baseUrl: 'http://api.test');
  const auth = 'Bearer access-token';

  group('VoiceCallsClient.startGroupVoice', () {
    test('POST /api/v1/voice/calls with GROUP_VOICE payload', () async {
      String? capturedBody;
      final mock = MockClient((req) async {
        expect(req.method, 'POST');
        expect(req.url.path, '/api/v1/voice/calls');
        capturedBody = req.body;
        return http.Response(
          jsonEncode({
            'call_session': {
              'room_id': 'room-group-1',
              'livekit_room_name': 'voice-group-room-1',
              'room_type_enum': 'VOICE_SESSION_KIND_GROUP_VOICE',
              'linked_chat': {
                'id': 'group-1',
                'type': 'CHAT_TYPE_GROUP',
              },
              'initiator_profile_id': 'profile-a',
              'media_kind': 'CALL_MEDIA_KIND_AUDIO',
              'status': 'CALL_STATUS_ACTIVE',
            },
          }),
          200,
        );
      });
      final client = VoiceCallsClient(
        gateway: gatewayHttpForTest(mock, config: config),
      );

      final result = await client.startGroupVoice(
        authorization: auth,
        groupChatId: 'group-1',
      );

      expect(result, isA<VoiceApiOk<VoiceCallSession>>());
      final body = jsonDecode(capturedBody!) as Map<String, dynamic>;
      expect(body['room_type_enum'], 'VOICE_SESSION_KIND_GROUP_VOICE');
      expect(body['linked_chat'], {
        'id': 'group-1',
        'type': 'CHAT_TYPE_GROUP',
      });
      expect(body.containsKey('callee_profile_id'), isFalse);

      final session = (result as VoiceApiOk<VoiceCallSession>).data;
      expect(session.roomId, 'room-group-1');
      expect(session.status, VoiceCallStatus.active);
    });
  });

  group('VoiceCallsClient.getActiveGroupCallForChat', () {
    test('GET /api/v1/voice/calls/active?chat_id=…', () async {
      Uri? capturedUri;
      final mock = MockClient((req) async {
        expect(req.method, 'GET');
        capturedUri = req.url;
        return http.Response(
          jsonEncode({
            'call_session': {
              'room_id': 'room-group-1',
              'room_type_enum': 'VOICE_SESSION_KIND_GROUP_VOICE',
              'linked_chat': {'id': 'group-1'},
              'status': 'CALL_STATUS_ACTIVE',
            },
          }),
          200,
        );
      });
      final client = VoiceCallsClient(
        gateway: gatewayHttpForTest(mock, config: config),
      );

      final result = await client.getActiveGroupCallForChat(
        authorization: auth,
        groupChatId: 'group-1',
      );

      expect(capturedUri?.queryParameters['chat_id'], 'group-1');
      expect(result, isA<VoiceApiOk<VoiceCallSession?>>());
      final session = (result as VoiceApiOk<VoiceCallSession?>).data;
      expect(session?.roomId, 'room-group-1');
      expect(session?.isGroupVoice, isTrue);
    });
  });

  group('VoiceCallsClient.joinCall', () {
    test('POST /api/v1/voice/calls/{roomId}/join', () async {
      final mock = MockClient((req) async {
        expect(req.method, 'POST');
        expect(req.url.path, '/api/v1/voice/calls/room-group-1/join');
        return http.Response(
          jsonEncode({
            'call_session': {
              'room_id': 'room-group-1',
              'livekit_room_name': 'voice-group-room-1',
              'room_type_enum': 'VOICE_SESSION_KIND_GROUP_VOICE',
              'linked_chat': {'id': 'group-1'},
              'initiator_profile_id': 'profile-a',
              'media_kind': 'CALL_MEDIA_KIND_AUDIO',
              'status': 'CALL_STATUS_ACTIVE',
            },
          }),
          200,
        );
      });
      final client = VoiceCallsClient(
        gateway: gatewayHttpForTest(mock, config: config),
      );

      final result = await client.joinCall(
        authorization: auth,
        roomId: 'room-group-1',
      );

      expect(result, isA<VoiceApiOk<VoiceCallSession>>());
      expect((result as VoiceApiOk<VoiceCallSession>).data.status,
          VoiceCallStatus.active);
    });
  });
}
