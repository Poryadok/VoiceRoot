import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/ordinary_voice_identity.dart';
import 'package:voice_frontend/backend/voice_client.dart';

const profile = '4bc6e0c4-f2d1-4b16-9cb5-96dc4cb93851';
const membership = '6e7b1a09-2d73-4b0a-b7c1-ae4203556178';
const nextMembership = 'a2c980c6-d703-40fb-9b20-dd51d14aa680';
const room = '4b4be70e-25c1-45a9-a853-e454a0f53266';

void main() {
  test(
    'local and remote public association never rewrites SDK membership key',
    () {
      final first = OrdinaryVoiceMediaIdentity.parse(
        'ov:$profile:$membership',
      )!;
      final refreshed = OrdinaryVoiceMediaIdentity.parse(first.sdkIdentity)!;
      final rejoined = OrdinaryVoiceMediaIdentity.parse(
        'ov:$profile:$nextMembership',
      )!;
      expect(first.profileId, profile);
      expect(refreshed.sdkIdentity, first.sdkIdentity);
      expect(refreshed.membershipId, membership);
      expect(rejoined.profileId, first.profileId);
      expect(rejoined.membershipId, nextMembership);
      expect(rejoined.sdkIdentity, isNot(first.sdkIdentity));
    },
  );

  test('foreign and malformed identities have no guessed ordinary profile', () {
    for (final identity in [
      profile,
      'profile:$profile:media:$membership',
      'ms:$profile:$membership',
      'game:$profile:$membership',
      'ov:$profile:$membership:extra',
      'ov:$profile',
      'ov::$membership',
      'ov:$profile:',
      'ov:${profile.toUpperCase()}:$membership',
      'ov:$profile:${membership.toUpperCase()}',
      'ov:00000000-0000-0000-0000-000000000000:$membership',
      'ov:$profile:00000000-0000-0000-0000-000000000000',
      ' ov:$profile:$membership',
      'ov:$profile:$membership\n',
      '',
    ]) {
      expect(
        OrdinaryVoiceMediaIdentity.parse(identity),
        isNull,
        reason: identity,
      );
    }
  });

  test(
    'only matched ordinary API kind and room name select ordinary decoder',
    () {
      expect(usesOrdinaryVoicePresentation(_session()), isTrue);
      expect(
        usesOrdinaryVoicePresentation(
          _session(
            kind: VoiceSessionKind.groupVoice,
            name: 'voice-group-$room',
          ),
        ),
        isTrue,
      );
      for (final session in [
        _session(name: 'voice-game-session-$room'),
        _session(name: 'voice-group-$room'),
        _session(kind: VoiceSessionKind.groupVoice),
        _session(kind: VoiceSessionKind.voiceRoom),
        _session(kind: VoiceSessionKind.unknown),
        _session(name: 'voice-dm-other-room'),
        _session(
          id: room.toUpperCase(),
          name: 'voice-dm-${room.toUpperCase()}',
        ),
        _session(spaceId: room),
        _session(voiceRoomId: room),
        _session(matchId: room),
        _session(mediaEpoch: membership),
      ]) {
        expect(usesOrdinaryVoicePresentation(session), isFalse);
      }
    },
  );
}

VoiceCallSession _session({
  String id = room,
  String name = 'voice-dm-$room',
  VoiceSessionKind kind = VoiceSessionKind.dm,
  String? spaceId,
  String? voiceRoomId,
  String? matchId,
  String? mediaEpoch,
}) => VoiceCallSession(
  roomId: id,
  livekitRoomName: name,
  chatId: 'chat',
  initiatorProfileId: profile,
  calleeProfileId: 'peer',
  mediaKind: VoiceCallMediaKind.audio,
  status: VoiceCallStatus.active,
  sessionKind: kind,
  spaceId: spaceId,
  voiceRoomId: voiceRoomId,
  matchId: matchId,
  mediaEpoch: mediaEpoch,
);
