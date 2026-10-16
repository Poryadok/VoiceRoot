import 'dart:collection';

import 'package:flutter_test/flutter_test.dart';
import 'package:livekit_client/livekit_client.dart' as livekit;
import 'package:voice_frontend/backend/livekit_room.dart';

void main() {
  group('LiveKit screen-share identity lookup', () {
    const profileId = '4bc6e0c4-f2d1-4b16-9cb5-96dc4cb93851';
    const otherProfileId = 'e09e459d-f55a-41ce-a629-e9f891a9573b';
    const generationOne = '6e7b1a09-2d73-4b0a-b7c1-ae4203556178';
    const generationTwo = 'a2c980c6-d703-40fb-9b20-dd51d14aa680';

    test(
      'matches signed Space identities by canonical profile across generations',
      () {
        final firstTrack = _RemoteVideoTrack();
        final secondTrack = _RemoteVideoTrack();
        final otherTrack = _RemoteVideoTrack();
        final first = _participant(
          'profile:$profileId:media:$generationOne',
          firstTrack,
          attributes: {'profile_id': otherProfileId},
        );
        final second = _participant(
          'profile:$profileId:media:$generationTwo',
          secondTrack,
        );
        final other = _participant(
          'profile:$otherProfileId:media:$generationOne',
          otherTrack,
          attributes: {'profile_id': profileId},
        );
        final room = LiveKitVoiceRoom(
          room: _FakeRoom({
            first.identity: first,
            second.identity: second,
            other.identity: other,
          }),
        );

        expect(room.remoteScreenShareTracks(participantIdentity: profileId), [
          firstTrack,
          secondTrack,
        ]);
      },
    );

    test('keeps legacy canonical profile identities working', () {
      final track = _RemoteVideoTrack();
      final participant = _participant(profileId, track);
      final room = LiveKitVoiceRoom(
        room: _FakeRoom({participant.identity: participant}),
      );

      expect(room.remoteScreenShareTracks(participantIdentity: profileId), [
        track,
      ]);
    });

    test(
      'voice organizer keeps the Space commander audible by profile ID',
      () async {
        final commanderTrack = _RemoteAudioTrack();
        final otherTrack = _RemoteAudioTrack();
        final commander = _FakeRemoteParticipant(
          'profile:$profileId:media:$generationOne',
          const [],
          [_FakeRemoteAudioPublication(commanderTrack)],
        );
        final other = _FakeRemoteParticipant(
          'profile:$otherProfileId:media:$generationOne',
          const [],
          [_FakeRemoteAudioPublication(otherTrack)],
        );
        final room = LiveKitVoiceRoom(
          room: _FakeRoom({
            commander.identity: commander,
            other.identity: other,
          }),
        );

        await room.setCommanderDucking(
          enabled: true,
          commanderIdentity: profileId,
        );

        expect(commanderTrack.enableCalls, 1);
        expect(commanderTrack.disableCalls, 0);
        expect(otherTrack.enableCalls, 0);
        expect(otherTrack.disableCalls, 1);
      },
    );

    test('does not map malformed Space identities to a profile', () {
      final malformedIdentities = [
        'profile:${profileId.toUpperCase()}:media:$generationOne',
        'profile:$profileId:media:$generationOne:extra',
        'profile:$profileId:media:not-a-uuid',
        'profile:00000000-0000-0000-0000-000000000000:media:$generationOne',
      ];

      for (final identity in malformedIdentities) {
        final track = _RemoteVideoTrack();
        final participant = _participant(
          identity,
          track,
          attributes: {'profile_id': profileId},
        );
        final room = LiveKitVoiceRoom(
          room: _FakeRoom({participant.identity: participant}),
        );

        expect(
          room.remoteScreenShareTracks(participantIdentity: profileId),
          isEmpty,
          reason: 'malformed identity must not be attributed to $profileId',
        );
      }
    });
  });
}

_FakeRemoteParticipant _participant(
  String identity,
  livekit.RemoteVideoTrack track, {
  Map<String, String> attributes = const {},
}) => _FakeRemoteParticipant(
  identity,
  [_FakeRemoteTrackPublication(track)],
  const [],
  attributes: attributes,
);

class _FakeRoom extends Fake implements livekit.Room {
  _FakeRoom(Map<String, livekit.RemoteParticipant> remoteParticipants)
    : _remoteParticipants = remoteParticipants;

  final Map<String, livekit.RemoteParticipant> _remoteParticipants;

  @override
  UnmodifiableMapView<String, livekit.RemoteParticipant>
  get remoteParticipants => UnmodifiableMapView(_remoteParticipants);
}

class _FakeRemoteParticipant extends Fake implements livekit.RemoteParticipant {
  _FakeRemoteParticipant(
    this.identity,
    this.videoTrackPublications,
    this.audioTrackPublications, {
    Map<String, String> attributes = const {},
  }) : _attributes = attributes;

  @override
  final String identity;

  @override
  final List<livekit.RemoteTrackPublication<livekit.RemoteVideoTrack>>
  videoTrackPublications;

  @override
  final List<livekit.RemoteTrackPublication<livekit.RemoteAudioTrack>>
  audioTrackPublications;
  final Map<String, String> _attributes;

  @override
  UnmodifiableMapView<String, String> get attributes =>
      UnmodifiableMapView(_attributes);
}

class _FakeRemoteTrackPublication extends Fake
    implements livekit.RemoteTrackPublication<livekit.RemoteVideoTrack> {
  _FakeRemoteTrackPublication(this.track);

  @override
  livekit.TrackSource get source => livekit.TrackSource.screenShareVideo;

  @override
  final livekit.RemoteVideoTrack track;
}

class _RemoteVideoTrack extends Fake implements livekit.RemoteVideoTrack {}

class _FakeRemoteAudioPublication extends Fake
    implements livekit.RemoteTrackPublication<livekit.RemoteAudioTrack> {
  _FakeRemoteAudioPublication(this.track);

  @override
  livekit.TrackSource get source => livekit.TrackSource.microphone;

  @override
  final livekit.RemoteAudioTrack track;
}

class _RemoteAudioTrack extends Fake implements livekit.RemoteAudioTrack {
  int enableCalls = 0;
  int disableCalls = 0;

  @override
  Future<void> enable() async => enableCalls++;

  @override
  Future<void> disable() async => disableCalls++;
}
