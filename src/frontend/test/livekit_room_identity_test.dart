import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/livekit_room.dart';

void main() {
  const profile = 'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa';
  const otherProfile = 'bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb';
  const oldEpoch = 'cccccccc-cccc-4ccc-8ccc-cccccccccccc';
  const newEpoch = 'dddddddd-dddd-4ddd-8ddd-dddddddddddd';

  group('MatchSquad LiveKit presentation identity', () {
    test('resolves one exact current-room remote identity', () {
      expect(
        resolveMatchSquadRemoteIdentity(profile, [
          'ms:$profile:$newEpoch',
          'ms:$otherProfile:$oldEpoch',
        ]),
        'ms:$profile:$newEpoch',
      );
    });

    test('does not select missing, malformed, or ordinary identities', () {
      expect(resolveMatchSquadRemoteIdentity(profile, const []), isNull);
      expect(resolveMatchSquadRemoteIdentity(profile, [profile]), isNull);
      expect(
        resolveMatchSquadRemoteIdentity(profile, ['ms:$profile:bad-epoch']),
        isNull,
      );
      expect(
        resolveMatchSquadRemoteIdentity(profile.toUpperCase(), [
          'ms:$profile:$newEpoch',
        ]),
        isNull,
      );
      expect(
        resolveMatchSquadRemoteIdentity(profile, [
          'ms:$profile:00000000-0000-0000-0000-000000000000',
        ]),
        isNull,
      );
    });

    test('ambiguous epochs and duplicate participants remain unresolved', () {
      expect(
        resolveMatchSquadRemoteIdentity(profile, [
          'ms:$profile:$oldEpoch',
          'ms:$profile:$newEpoch',
        ]),
        isNull,
      );
      expect(
        resolveMatchSquadRemoteIdentity(profile, [
          'ms:$profile:$newEpoch',
          'ms:$profile:$newEpoch',
        ]),
        isNull,
      );
    });

    test('local identity is bound to the local profile and current epoch', () {
      expect(
        matchesMatchSquadLocalIdentity(
          profile,
          newEpoch,
          'ms:$profile:$newEpoch',
        ),
        isTrue,
      );
      expect(
        matchesMatchSquadLocalIdentity(
          profile,
          oldEpoch,
          'ms:$profile:$newEpoch',
        ),
        isFalse,
      );
      expect(
        matchesMatchSquadLocalIdentity(
          profile,
          newEpoch,
          'ms:$otherProfile:$newEpoch',
        ),
        isFalse,
      );
      expect(
        matchesMatchSquadLocalIdentity(profile, null, 'ms:$profile:$newEpoch'),
        isFalse,
      );
    });
  });
}
