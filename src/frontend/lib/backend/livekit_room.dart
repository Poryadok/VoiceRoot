import 'dart:async';

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

  static const Duration _connectTimeout = Duration(seconds: 20);
  static const Duration _mediaEnableTimeout = Duration(seconds: 12);

  final livekit.Room _room;

  @override
  Future<void> connect({
    required String url,
    required String token,
    required bool video,
  }) async {
    await _room
        .connect(
          url,
          token,
          connectOptions: const livekit.ConnectOptions(
            timeouts: livekit.Timeouts(
              connection: Duration(seconds: 15),
              debounce: Duration(milliseconds: 20),
              publish: Duration(seconds: 15),
              subscribe: Duration(seconds: 15),
              peerConnection: Duration(seconds: 15),
              iceRestart: Duration(seconds: 15),
            ),
          ),
        )
        .timeout(_connectTimeout);
    await _enableLocalMedia(video: video);
  }

  Future<void> _enableLocalMedia({required bool video}) async {
    final participant = _room.localParticipant;
    if (participant == null) return;

    await participant
        .setMicrophoneEnabled(true)
        .timeout(_mediaEnableTimeout);
    if (!video) return;

    try {
      await participant
          .setCameraEnabled(true)
          .timeout(_mediaEnableTimeout);
    } on TimeoutException {
      await participant.setCameraEnabled(false);
    }
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
    await _room.localParticipant
        ?.setCameraEnabled(enabled)
        .timeout(_mediaEnableTimeout);
  }

  @override
  Future<void> disconnect() => _room.disconnect();
}
