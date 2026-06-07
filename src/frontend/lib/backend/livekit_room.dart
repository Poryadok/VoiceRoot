import 'dart:async';

import 'package:livekit_client/livekit_client.dart' as livekit;

abstract interface class VoiceLiveKitRoom {
  /// Called when the browser blocks remote audio playback (web autoplay policy).
  void Function(bool needsUnlock)? onAudioPlaybackUnlockNeeded;

  Future<void> connect({
    required String url,
    required String token,
    required bool video,
  });

  Future<void> ensureAudioPlayback();
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
  livekit.EventsListener<livekit.RoomEvent>? _listener;
  bool _speakerMuted = false;

  @override
  void Function(bool needsUnlock)? onAudioPlaybackUnlockNeeded;

  static const livekit.ConnectOptions _connectOptions = livekit.ConnectOptions(
    timeouts: livekit.Timeouts(
      connection: Duration(seconds: 15),
      debounce: Duration(milliseconds: 20),
      publish: Duration(seconds: 15),
      subscribe: Duration(seconds: 15),
      peerConnection: Duration(seconds: 15),
      iceRestart: Duration(seconds: 15),
    ),
  );

  @override
  Future<void> connect({
    required String url,
    required String token,
    required bool video,
  }) async {
    // Publish mic/camera during join (same event chain as localParticipant
    // creation). A separate setMicrophoneEnabled after connect races on web.
    final fastConnect = livekit.FastConnectOptions(
      microphone: const livekit.TrackOption<bool, livekit.LocalAudioTrack>(
        enabled: true,
      ),
      camera: livekit.TrackOption<bool, livekit.LocalVideoTrack>(
        enabled: video,
      ),
    );

    await _room.prepareConnection(url, token);
    await _room
        .connect(
          url,
          token,
          connectOptions: _connectOptions,
          fastConnectOptions: fastConnect,
        )
        .timeout(_connectTimeout);

    final participant = await _waitForLocalParticipant();
    if (participant == null) {
      throw StateError('livekit_local_participant_missing');
    }

    _setupListener();
    await ensureAudioPlayback();
  }

  void _setupListener() {
    _listener?.dispose();
    _listener = _room.createListener()
      ..on<livekit.AudioPlaybackStatusChanged>((event) {
        if (!_room.canPlaybackAudio) {
          onAudioPlaybackUnlockNeeded?.call(true);
        }
      })
      ..on<livekit.TrackSubscribedEvent>((event) {
        if (_speakerMuted && event.track.kind == livekit.TrackType.AUDIO) {
          event.track.mediaStreamTrack.enabled = false;
        }
      });
  }

  @override
  Future<void> ensureAudioPlayback() async {
    await _room.startAudio();
    if (_room.canPlaybackAudio) {
      onAudioPlaybackUnlockNeeded?.call(false);
    }
  }

  Future<livekit.LocalParticipant?> _waitForLocalParticipant() async {
    const attempts = 50;
    for (var i = 0; i < attempts; i++) {
      final participant = _room.localParticipant;
      if (participant != null) return participant;
      await Future<void>.delayed(const Duration(milliseconds: 20));
    }
    return _room.localParticipant;
  }

  @override
  Future<void> setMuted(bool muted) async {
    await _room.localParticipant?.setMicrophoneEnabled(!muted);
  }

  @override
  Future<void> setSpeakerMuted(bool muted) async {
    _speakerMuted = muted;
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
  Future<void> disconnect() async {
    _listener?.dispose();
    _listener = null;
    _speakerMuted = false;
    try {
      await _room.disconnect();
    } catch (_) {}
  }
}
