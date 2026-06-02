import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/voice_client.dart';

void main() {
  const config = GatewayConfig(baseUrl: 'http://api.test');
  const auth = 'Bearer access-token';

  test('startCall posts Phase 2 voice call payload', () async {
    String? capturedBody;
    final mock = MockClient((req) async {
      expect(req.method, 'POST');
      expect(req.url.path, '/api/v1/voice/calls');
      expect(req.headers['Authorization'], auth);
      capturedBody = req.body;
      return http.Response(
        jsonEncode({
          'call_session': {
            'room_id': 'room-1',
            'livekit_room_name': 'voice-dm-room-1',
            'linked_chat': {'id': 'chat-1'},
            'initiator_profile_id': 'profile-a',
            'callee_profile_id': 'profile-b',
            'media_kind': 'CALL_MEDIA_KIND_VIDEO',
            'status': 'CALL_STATUS_RINGING',
          },
        }),
        200,
      );
    });
    final client = VoiceCallsClient(httpClient: mock, config: config);

    final result = await client.startCall(
      authorization: auth,
      chatId: 'chat-1',
      calleeProfileId: 'profile-b',
      mediaKind: VoiceCallMediaKind.video,
    );

    expect(result, isA<VoiceApiOk<VoiceCallSession>>());
    final body = jsonDecode(capturedBody!) as Map<String, dynamic>;
    expect(body['linked_chat'], {'id': 'chat-1'});
    expect(body['callee_profile_id'], 'profile-b');
    expect(body['media_kind'], 'video');
    final session = (result as VoiceApiOk<VoiceCallSession>).data;
    expect(session.roomId, 'room-1');
    expect(session.mediaKind, VoiceCallMediaKind.video);
    expect(session.status, VoiceCallStatus.ringing);
  });

  test('getJoinToken parses LiveKit JWT response', () async {
    final mock = MockClient((req) async {
      expect(req.method, 'GET');
      expect(req.url.path, '/api/v1/voice/calls/room-1/token');
      return http.Response(
        jsonEncode({
          'jwt': 'livekit-jwt',
          'expires_at': '2026-01-01T00:00:00Z',
        }),
        200,
      );
    });
    final client = VoiceCallsClient(httpClient: mock, config: config);

    final result = await client.getJoinToken(
      authorization: auth,
      roomId: 'room-1',
    );

    expect(result, isA<VoiceApiOk<VoiceJoinToken>>());
    expect((result as VoiceApiOk<VoiceJoinToken>).data.jwt, 'livekit-jwt');
  });
}
