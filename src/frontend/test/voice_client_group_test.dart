import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/voice_client.dart';

import 'support/gateway_test_client.dart';

/// HTTP contract tests for text-chat.md group voice (до 32, join active call).
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
              'linked_chat': {'id': 'group-1', 'type': 'CHAT_TYPE_GROUP'},
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
      expect(body['linked_chat'], {'id': 'group-1', 'type': 'CHAT_TYPE_GROUP'});
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
      expect(
        (result as VoiceApiOk<VoiceCallSession>).data.status,
        VoiceCallStatus.active,
      );
    });
  });

  group('VoiceCallsClient MatchSquad membership', () {
    test(
      'joins, gets epoch-bound token, and leaves through match routes',
      () async {
        final bodies = <String>[];
        var request = 0;
        final mock = MockClient((req) async {
          expect(req.method, 'POST');
          expect(req.headers['Authorization'], auth);
          bodies.add(req.body);
          switch (request++) {
            case 0:
              expect(
                req.url.path,
                '/api/v1/matchmaking/matches/match-1/voice/join',
              );
              return http.Response(
                jsonEncode({
                  'call_session': {
                    'room_id': 'room-1',
                    'livekit_room_name': 'squad-room-1',
                    'room_type_enum': 'VOICE_SESSION_KIND_GROUP_VOICE',
                    'status': 'CALL_STATUS_ACTIVE',
                  },
                  'media_epoch': 'epoch-1',
                  'membership_state': 'MATCH_SQUAD_MEMBERSHIP_STATE_JOINED',
                }),
                200,
              );
            case 1:
              expect(
                req.url.path,
                '/api/v1/matchmaking/matches/match-1/voice/token',
              );
              return http.Response(
                jsonEncode({
                  'token': {'jwt': 'opaque', 'livekit_url': 'wss://media.test'},
                  'media_epoch': 'epoch-1',
                }),
                200,
              );
            default:
              expect(
                req.url.path,
                '/api/v1/matchmaking/matches/match-1/voice/leave',
              );
              return http.Response(
                jsonEncode({
                  'call_session': {
                    'room_id': 'room-1',
                    'livekit_room_name': 'squad-room-1',
                    'room_type_enum': 'VOICE_SESSION_KIND_GROUP_VOICE',
                    'status': 'CALL_STATUS_ACTIVE',
                  },
                  'media_epoch': 'epoch-1',
                  'membership_state': 'MATCH_SQUAD_MEMBERSHIP_STATE_LEFT',
                }),
                200,
              );
          }
        });
        final client = VoiceCallsClient(
          gateway: gatewayHttpForTest(mock, config: config),
        );

        final joined = await client.joinMatchSquadRoom(
          authorization: auth,
          matchId: 'match-1',
          roomId: 'room-1',
          operationId: 'join-op',
        );
        expect(joined, isA<VoiceApiOk<MatchSquadJoinResult>>());
        final join = (joined as VoiceApiOk<MatchSquadJoinResult>).data;
        expect(join.session.matchId, 'match-1');
        expect(join.session.mediaEpoch, 'epoch-1');
        expect(join.mediaEpoch, 'epoch-1');

        final token = await client.getMatchSquadJoinToken(
          authorization: auth,
          matchId: 'match-1',
          roomId: 'room-1',
          mediaEpoch: join.mediaEpoch,
        );
        expect(token, isA<VoiceApiOk<VoiceJoinToken>>());
        expect((token as VoiceApiOk<VoiceJoinToken>).data.jwt, 'opaque');

        final left = await client.leaveMatchSquadRoom(
          authorization: auth,
          matchId: 'match-1',
          roomId: 'room-1',
          mediaEpoch: join.mediaEpoch,
          operationId: 'leave-op',
        );
        expect(left, isA<VoiceApiOk<void>>());
        expect(request, 3);
        expect(jsonDecode(bodies[0]), {
          'protocol_version': 1,
          'operation_id': 'join-op',
          'match_id': 'match-1',
          'room_id': 'room-1',
        });
        expect(jsonDecode(bodies[1]), {
          'protocol_version': 1,
          'match_id': 'match-1',
          'room_id': 'room-1',
          'media_epoch': 'epoch-1',
        });
        expect(jsonDecode(bodies[2]), {
          'protocol_version': 1,
          'operation_id': 'leave-op',
          'match_id': 'match-1',
          'room_id': 'room-1',
          'expected_media_epoch': 'epoch-1',
        });
      },
    );

    test('rejects a token response for a different media epoch', () async {
      final mock = MockClient(
        (_) async => http.Response(
          jsonEncode({
            'token': {'jwt': 'opaque'},
            'media_epoch': 'other-epoch',
          }),
          200,
        ),
      );
      final client = VoiceCallsClient(
        gateway: gatewayHttpForTest(mock, config: config),
      );
      final result = await client.getMatchSquadJoinToken(
        authorization: auth,
        matchId: 'match-1',
        roomId: 'room-1',
        mediaEpoch: 'epoch-1',
      );
      expect(result, isA<VoiceApiFailure>());
    });

    test('does not confirm squad leave while provider reports leaving', () async {
      final mock = MockClient(
        (_) async => http.Response(
          jsonEncode({
            'call_session': {
              'room_id': 'room-1',
              'livekit_room_name': 'squad-room-1',
              'room_type_enum': 'VOICE_SESSION_KIND_GROUP_VOICE',
              'status': 'CALL_STATUS_ACTIVE',
            },
            'media_epoch': 'epoch-1',
            'membership_state': 'MATCH_SQUAD_MEMBERSHIP_STATE_LEAVING',
          }),
          200,
        ),
      );
      final client = VoiceCallsClient(
        gateway: gatewayHttpForTest(mock, config: config),
      );
      final result = await client.leaveMatchSquadRoom(
        authorization: auth,
        matchId: 'match-1',
        roomId: 'room-1',
        mediaEpoch: 'epoch-1',
        operationId: 'leave-op',
      );
      expect(result, isA<VoiceApiFailure>());
    });
  });
}
