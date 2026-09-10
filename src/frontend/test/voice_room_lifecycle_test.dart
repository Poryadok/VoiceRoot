import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/proto_mappers.dart';
import 'package:voice_frontend/backend/voice_client.dart';
import 'package:voice_frontend/gen/voice/calls/v1/calls.pb.dart' as calls_pb;

import 'support/gateway_test_client.dart';

const _auth = 'Bearer access-token';
const _spaceId = '11111111-1111-4111-8111-111111111111';
const _voiceRoomId = '22222222-2222-4222-8222-222222222222';
const _roomId = '33333333-3333-4333-8333-333333333333';
const _operationId = '44444444-4444-4444-8444-444444444444';

Map<String, dynamic> _roomResponse() => {
  'voice_session': {
    'room_id': _roomId,
    'livekit_room_name': 'voice-room-$_roomId',
    'voice_room_id': _voiceRoomId,
  },
};

void _expectCanonicalRequest(http.Request request, String action) {
  expect(request.method, 'POST');
  expect(
    request.url.path,
    '/api/v1/spaces/$_spaceId/voice-rooms/$_voiceRoomId/$action',
  );
  expect(request.url.query, isEmpty);
  expect(request.headers['Authorization'], _auth);
  expect(jsonDecode(request.body), {'operation_id': _operationId});
}

void main() {
  group('Space room session identity', () {
    test('mapper preserves voice room ID without inventing Space ID', () {
      final session = voiceCallSessionFromProto(
        calls_pb.CallSession(
          roomId: _roomId,
          livekitRoomName: 'voice-room-$_roomId',
          voiceRoomId: _voiceRoomId,
          roomType: 'voice_room',
        ),
      );

      expect(session.sessionKind, VoiceSessionKind.voiceRoom);
      expect(session.roomId, _roomId);
      expect(session.voiceRoomId, _voiceRoomId);
      expect(session.spaceId, isNull);
    });

    for (final kind in ['call', 'group_voice', 'voice_room']) {
      test('$kind with absent room binding never infers it from roomId', () {
        final session = voiceCallSessionFromProto(
          calls_pb.CallSession(roomId: _roomId, roomType: kind),
        );

        expect(session.roomId, _roomId);
        expect(session.voiceRoomId, isNull);
        expect(session.spaceId, isNull);
      });
    }

    test(
      'active-call protobuf JSON decode retains incomplete room binding',
      () async {
        var requests = 0;
        final mock = MockClient((request) async {
          requests++;
          expect(request.method, 'GET');
          expect(request.url.path, '/api/v1/voice/calls/active');
          expect(request.headers['Authorization'], _auth);
          return utf8JsonResponse(
            jsonEncode({
              'call_session': {
                'room_id': _roomId,
                'livekit_room_name': 'voice-room-$_roomId',
                'voice_room_id': _voiceRoomId,
                'room_type_enum': 'VOICE_SESSION_KIND_VOICE_ROOM',
                'status': 'CALL_STATUS_ACTIVE',
              },
            }),
          );
        });
        final client = VoiceCallsClient(gateway: gatewayHttpForTest(mock));

        final result = await client.getActiveCall(authorization: _auth);

        expect(requests, 1);
        expect(result, isA<VoiceApiOk<VoiceCallSession?>>());
        final session = (result as VoiceApiOk<VoiceCallSession?>).data!;
        expect(session.sessionKind, VoiceSessionKind.voiceRoom);
        expect(session.status, VoiceCallStatus.active);
        expect(session.roomId, _roomId);
        expect(session.voiceRoomId, _voiceRoomId);
        expect(session.spaceId, isNull);
      },
    );
  });

  group('canonical Space room lifecycle transport', () {
    test('default capability rejects join and leave with zero HTTP', () async {
      var requests = 0;
      final client = VoiceCallsClient(
        gateway: gatewayHttpForTest(
          MockClient((request) async {
            requests++;
            return http.Response('', 204);
          }),
        ),
      );

      final join = await client.joinSpaceVoiceRoom(
        authorization: _auth,
        spaceId: _spaceId,
        voiceRoomId: _voiceRoomId,
        operationId: _operationId,
      );
      final leave = await client.leaveSpaceVoiceRoom(
        authorization: _auth,
        spaceId: _spaceId,
        voiceRoomId: _voiceRoomId,
        operationId: _operationId,
      );

      for (final result in [join, leave]) {
        expect(result, isA<VoiceApiFailure>());
        expect(
          (result as VoiceApiFailure).errorCode,
          'voice_room_lifecycle_unavailable',
        );
      }
      expect(requests, 0);
    });

    test('enabled join binds result to exact path room and Space', () async {
      var requests = 0;
      final client = VoiceCallsClient(
        canonicalRoomLifecycleEnabled: true,
        gateway: gatewayHttpForTest(
          MockClient((request) async {
            requests++;
            _expectCanonicalRequest(request, 'join');
            return utf8JsonResponse(jsonEncode(_roomResponse()));
          }),
        ),
      );

      final result = await client.joinSpaceVoiceRoom(
        authorization: _auth,
        spaceId: _spaceId,
        voiceRoomId: _voiceRoomId,
        operationId: _operationId,
      );

      expect(requests, 1);
      expect(result, isA<VoiceApiOk<VoiceRoomSession>>());
      final session = (result as VoiceApiOk<VoiceRoomSession>).data;
      expect(session.roomId, _roomId);
      expect(session.livekitRoomName, 'voice-room-$_roomId');
      expect(session.voiceRoomId, _voiceRoomId);
      expect(session.spaceId, _spaceId);
    });

    for (final invalid in [
      {'voice_room_id': '55555555-5555-4555-8555-555555555555'},
      {'voice_room_id': ''},
      {'room_id': ''},
    ]) {
      test('join rejects invalid response binding $invalid', () async {
        var requests = 0;
        final response = _roomResponse();
        (response['voice_session'] as Map<String, dynamic>).addAll(invalid);
        final client = VoiceCallsClient(
          canonicalRoomLifecycleEnabled: true,
          gateway: gatewayHttpForTest(
            MockClient((request) async {
              requests++;
              _expectCanonicalRequest(request, 'join');
              return utf8JsonResponse(jsonEncode(response));
            }),
          ),
        );

        final result = await client.joinSpaceVoiceRoom(
          authorization: _auth,
          spaceId: _spaceId,
          voiceRoomId: _voiceRoomId,
          operationId: _operationId,
        );

        expect(result, isA<VoiceApiFailure>());
        expect(requests, 1);
      });
    }

    test('enabled leave posts only operation ID and accepts 204', () async {
      var requests = 0;
      final client = VoiceCallsClient(
        canonicalRoomLifecycleEnabled: true,
        gateway: gatewayHttpForTest(
          MockClient((request) async {
            requests++;
            _expectCanonicalRequest(request, 'leave');
            return http.Response('', 204);
          }),
        ),
      );

      final result = await client.leaveSpaceVoiceRoom(
        authorization: _auth,
        spaceId: _spaceId,
        voiceRoomId: _voiceRoomId,
        operationId: _operationId,
      );

      expect(result, isA<VoiceApiOk<void>>());
      expect(requests, 1);
    });

    for (final error in {
      403: 'permission_denied',
      404: 'not_found',
      409: 'failed_precondition',
      503: 'unavailable',
    }.entries) {
      for (final action in ['join', 'leave']) {
        test(
          '$action preserves ${error.key}/${error.value} without fallback',
          () async {
            var requests = 0;
            final client = VoiceCallsClient(
              canonicalRoomLifecycleEnabled: true,
              gateway: gatewayHttpForTest(
                MockClient((request) async {
                  requests++;
                  _expectCanonicalRequest(request, action);
                  return utf8JsonResponse(
                    jsonEncode({'error_code': error.value}),
                    status: error.key,
                  );
                }),
              ),
            );

            final result = action == 'join'
                ? await client.joinSpaceVoiceRoom(
                    authorization: _auth,
                    spaceId: _spaceId,
                    voiceRoomId: _voiceRoomId,
                    operationId: _operationId,
                  )
                : await client.leaveSpaceVoiceRoom(
                    authorization: _auth,
                    spaceId: _spaceId,
                    voiceRoomId: _voiceRoomId,
                    operationId: _operationId,
                  );

            expect(result, isA<VoiceApiFailure>());
            final failure = result as VoiceApiFailure;
            expect(failure.errorCode, error.value);
            expect(failure.statusCode, error.key);
            expect(requests, 1);
          },
        );
      }
    }
  });
}
