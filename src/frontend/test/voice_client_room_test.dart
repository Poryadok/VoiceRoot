import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/voice_client.dart';

import 'support/gateway_test_client.dart';

/// HTTP contract tests for PLAN Phase 5 space voice rooms.
void main() {
  const config = GatewayConfig(baseUrl: 'http://api.test');
  const auth = 'Bearer access-token';

  group('VoiceCallsClient.joinVoiceRoom', () {
    test('POST /api/v1/voice/rooms/{id}/join', () async {
      final mock = MockClient((req) async {
        expect(req.method, 'POST');
        expect(req.url.path, '/api/v1/voice/rooms/vr-lobby/join');
        return http.Response(
          jsonEncode({
            'voice_session': {
              'room_id': 'room-vr-lobby',
              'livekit_room_name': 'voice-room-vr-lobby',
              'voice_room_id': 'vr-lobby',
            },
          }),
          200,
        );
      });
      final client = VoiceCallsClient(
        gateway: gatewayHttpForTest(mock, config: config),
      );

      final result = await client.joinVoiceRoom(
        authorization: auth,
        voiceRoomId: 'vr-lobby',
        spaceId: 'space-1',
      );

      expect(result, isA<VoiceApiOk<VoiceRoomSession>>());
      final session = (result as VoiceApiOk<VoiceRoomSession>).data;
      expect(session.roomId, 'room-vr-lobby');
      expect(session.voiceRoomId, 'vr-lobby');
    });
  });

  group('VoiceCallsClient.leaveVoiceRoom', () {
    test('POST /api/v1/voice/rooms/{id}/leave', () async {
      final mock = MockClient((req) async {
        expect(req.method, 'POST');
        expect(req.url.path, '/api/v1/voice/rooms/vr-lobby/leave');
        return http.Response('', 204);
      });
      final client = VoiceCallsClient(
        gateway: gatewayHttpForTest(mock, config: config),
      );

      final result = await client.leaveVoiceRoom(
        authorization: auth,
        voiceRoomId: 'vr-lobby',
      );

      expect(result, isA<VoiceApiOk<void>>());
    });
  });

  group('VoiceCallsClient.getVoiceRoomStates', () {
    test('GET /api/v1/voice/rooms/{id}/states', () async {
      final mock = MockClient((req) async {
        expect(req.method, 'GET');
        expect(req.url.path, '/api/v1/voice/rooms/vr-lobby/states');
        return http.Response(
          jsonEncode({
            'participants': [
              {
                'profile_id': 'profile-a',
                'is_muted': false,
                'is_deafened': false,
                'is_video_on': false,
                'is_screen_sharing': false,
              },
            ],
          }),
          200,
        );
      });
      final client = VoiceCallsClient(
        gateway: gatewayHttpForTest(mock, config: config),
      );

      final result = await client.getVoiceRoomStates(
        authorization: auth,
        voiceRoomId: 'vr-lobby',
      );

      expect(result, isA<VoiceApiOk<List<VoiceRoomParticipantState>>>());
      final states =
          (result as VoiceApiOk<List<VoiceRoomParticipantState>>).data;
      expect(states, hasLength(1));
      expect(states.first.profileId, 'profile-a');
      expect(states.first.isMuted, isFalse);
    });
  });
}
