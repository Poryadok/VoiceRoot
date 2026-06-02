import 'package:livekit_client/livekit_client.dart' as livekit;

abstract interface class VoiceLiveKitRoom {
  Future<void> connect({
    required String url,
    required String token,
    required bool video,
  });

  Future<void> setMuted(bool muted);
  Future<void> setSpeakerMuted(bool muted);
  Future<void> setVideoEnabled(bool enabled);
  Future<void> disconnect();
}

class LiveKitVoiceRoom implements VoiceLiveKitRoom {
  LiveKitVoiceRoom({livekit.Room? room}) : _room = room ?? livekit.Room();

  final livekit.Room _room;

  @override
  Future<void> connect({
    required String url,
    required String token,
    required bool video,
  }) async {
    await _room.connect(url, token);
    await _room.localParticipant?.setMicrophoneEnabled(true);
    await _room.localParticipant?.setCameraEnabled(video);
  }

  @override
  Future<void> setMuted(bool muted) async {
    await _room.localParticipant?.setMicrophoneEnabled(!muted);
  }

  @override
  Future<void> setSpeakerMuted(bool muted) async {
    for (final participant in _room.remoteParticipants.values) {
      for (final publication in participant.audioTrackPublications) {
        publication.track?.mediaStreamTrack.enabled = !muted;
      }
    }
  }

  @override
  Future<void> setVideoEnabled(bool enabled) async {
    await _room.localParticipant?.setCameraEnabled(enabled);
  }

  @override
  Future<void> disconnect() => _room.disconnect();
}
