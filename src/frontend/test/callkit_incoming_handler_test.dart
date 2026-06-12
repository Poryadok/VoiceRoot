import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/voice_client.dart';
import 'package:voice_frontend/state/callkit_incoming_handler.dart';

void main() {
  group('shouldIgnoreDuplicateIncoming', () {
    test('ignores duplicate room while incoming', () {
      expect(
        shouldIgnoreDuplicateIncoming(
          roomId: 'room-1',
          currentRoomId: 'room-1',
          isIncomingPhase: true,
        ),
        isTrue,
      );
    });

    test('allows new room while incoming', () {
      expect(
        shouldIgnoreDuplicateIncoming(
          roomId: 'room-2',
          currentRoomId: 'room-1',
          isIncomingPhase: true,
        ),
        isFalse,
      );
    });
  });

  group('sessionFromVoIPPayload', () {
    test('builds ringing DM session', () {
      final session = sessionFromVoIPPayload({
        'room_id': 'room-1',
        'chat_id': 'chat-1',
        'initiator_profile_id': 'init',
        'callee_profile_id': 'callee',
        'media_kind': 'video',
        'livekit_room_name': 'lk-1',
        'expires_at': '2026-06-12T12:00:00Z',
      });
      expect(session, isNotNull);
      expect(session!.roomId, 'room-1');
      expect(session.mediaKind, VoiceCallMediaKind.video);
      expect(session.status, VoiceCallStatus.ringing);
      expect(session.sessionKind, VoiceSessionKind.dm);
    });

    test('returns null without room_id', () {
      expect(parseIncomingCallPayload({'chat_id': 'x'}), isNull);
    });
  });
}
