import 'dart:collection';
import 'package:flutter_test/flutter_test.dart';
import 'package:livekit_client/livekit_client.dart' as livekit;
import 'package:voice_frontend/backend/livekit_room.dart';
import 'package:voice_frontend/backend/proto_mappers.dart';
import 'package:voice_frontend/gen/voice/calls/v1/calls.pb.dart';

void main() {
  test(
    'roster promotes speaker while preserving opaque membership generations',
    () {
      const id = '4bc6e0c4-f2d1-4b16-9cb5-96dc4cb93851';
      const generation = '6e7b1a09-2d73-4b0a-b7c1-ae4203556178';
      final first = _Participant('ov:$id:$generation', 'Alex');
      final speaker = _Participant('profile:$id:media:$generation', 'Maria');
      final malformed = _Participant('unknown:$id', 'Unknown');
      final sdk = _Room([first, speaker, malformed], [speaker]);
      final room = LiveKitVoiceRoom(room: sdk, ordinaryVoicePresentation: true);
      final roster = room.callParticipants;
      expect(roster.map((p) => p.identity), [
        speaker.identity,
        first.identity,
        malformed.identity,
      ]);
      expect(roster.map((p) => p.isSpeaking), [true, false, false]);
      expect(roster.map((p) => p.profileId), [id, id, null]);
      sdk.speakers = [first];
      expect(room.callParticipants.first.identity, first.identity);
      sdk.participants.remove(first.identity);
      sdk.speakers = [];
      expect(room.callParticipants.map((p) => p.identity), [
        speaker.identity,
        malformed.identity,
      ]);
    },
  );

  test(
    'call timestamp survives Gateway mapping; absent timestamp remains unknown',
    () {
      final started = DateTime.utc(2026, 10, 25, 20);
      final data = CallSession(
        roomId: 'room',
        startedAt: dateTimeToProtoTimestamp(started),
      );
      expect(voiceCallSessionFromProto(data).startedAt, started);
      expect(
        voiceCallSessionFromProto(CallSession(roomId: 'room')).startedAt,
        isNull,
      );
    },
  );
}

class _Participant extends Fake implements livekit.RemoteParticipant {
  _Participant(this.identity, this.name);
  @override
  final String identity;
  @override
  final String name;
}

class _Room extends Fake implements livekit.Room {
  _Room(List<livekit.RemoteParticipant> values, this.speakers)
    : participants = {for (final value in values) value.identity: value};
  final Map<String, livekit.RemoteParticipant> participants;
  List<livekit.Participant> speakers;
  @override
  livekit.LocalParticipant? get localParticipant => null;
  @override
  UnmodifiableMapView<String, livekit.RemoteParticipant>
  get remoteParticipants => UnmodifiableMapView(participants);
  @override
  UnmodifiableListView<livekit.Participant> get activeSpeakers =>
      UnmodifiableListView(speakers);
}
