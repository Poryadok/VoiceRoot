import 'dart:async';

import 'package:livekit_client/livekit_client.dart' as livekit;

abstract interface class VoiceLiveKitRoom {
  /// Called when the browser blocks remote audio playback (web autoplay policy).
  void Function(bool needsUnlock)? onAudioPlaybackUnlockNeeded;

  /// Called when local/remote camera tracks are published or subscribed.
  void Function()? onTracksChanged;

  Future<void> connect({
    required String url,
    required String token,
    required bool video,
  });

  Future<void> ensureAudioPlayback();
  Future<void> setMuted(bool muted);
  Future<void> setSpeakerMuted(bool muted);
  Future<void> setVideoEnabled(bool enabled);
  Future<void> startScreenShare({
    double maxFrameRate,
    bool captureSystemAudio,
  });
  Future<void> pauseScreenShare(bool paused);
  Future<void> stopScreenShare();
  bool get isScreenSharing;
  bool get isScreenSharePaused;
  List<livekit.RemoteVideoTrack> remoteScreenShareTracks({
    String? participantIdentity,
  });
  livekit.LocalVideoTrack? localCameraTrack();
  livekit.RemoteVideoTrack? remoteCameraTrack();
  Future<void> disconnect();
}

class LiveKitVoiceRoom implements VoiceLiveKitRoom {
  LiveKitVoiceRoom({livekit.Room? room}) : _room = room ?? livekit.Room();

  static const Duration _connectTimeout = Duration(seconds: 20);
  static const Duration _mediaEnableTimeout = Duration(seconds: 12);

  final livekit.Room _room;
  livekit.EventsListener<livekit.RoomEvent>? _listener;
  bool _speakerMuted = false;
  bool _screenSharePaused = false;
  livekit.LocalTrackPublication<livekit.LocalTrack>? _screenSharePublication;

  livekit.Room get room => _room;

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
  void Function()? onTracksChanged;

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

  void _notifyTracksChanged() => onTracksChanged?.call();

  void _setupListener() {
    _listener?.dispose();
    _listener = _room.createListener()
      ..on<livekit.AudioPlaybackStatusChanged>((event) {
        if (!_room.canPlaybackAudio) {
          onAudioPlaybackUnlockNeeded?.call(true);
        }
      })
      ..on<livekit.TrackSubscribedEvent>((event) {
        if (event.track.kind == livekit.TrackType.AUDIO) {
          if (_speakerMuted) {
            event.track.mediaStreamTrack.enabled = false;
          }
          unawaited(ensureAudioPlayback());
        }
        if (event.track.kind == livekit.TrackType.VIDEO &&
            event.publication.source == livekit.TrackSource.camera) {
          _notifyTracksChanged();
        }
      })
      ..on<livekit.TrackUnsubscribedEvent>((event) {
        if (event.track.kind == livekit.TrackType.VIDEO &&
            event.publication.source == livekit.TrackSource.camera) {
          _notifyTracksChanged();
        }
      })
      ..on<livekit.LocalTrackPublishedEvent>((event) {
        if (event.publication.source == livekit.TrackSource.camera) {
          _notifyTracksChanged();
        }
      })
      ..on<livekit.LocalTrackUnpublishedEvent>((event) {
        if (event.publication.source == livekit.TrackSource.camera) {
          _notifyTracksChanged();
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
    _notifyTracksChanged();
  }

  @override
  livekit.LocalVideoTrack? localCameraTrack() {
    final participant = _room.localParticipant;
    if (participant == null) return null;
    for (final publication in participant.videoTrackPublications) {
      if (publication.source == livekit.TrackSource.camera &&
          publication.track is livekit.LocalVideoTrack) {
        return publication.track as livekit.LocalVideoTrack;
      }
    }
    return null;
  }

  @override
  livekit.RemoteVideoTrack? remoteCameraTrack() {
    for (final remote in _room.remoteParticipants.values) {
      for (final publication in remote.videoTrackPublications) {
        if (publication.source == livekit.TrackSource.camera &&
            publication.track is livekit.RemoteVideoTrack) {
          return publication.track as livekit.RemoteVideoTrack;
        }
      }
    }
    return null;
  }

  @override
  bool get isScreenSharing => _screenSharePublication != null;

  @override
  bool get isScreenSharePaused => _screenSharePaused;

  @override
  Future<void> startScreenShare({
    double maxFrameRate = 15,
    bool captureSystemAudio = false,
  }) async {
    final participant = _room.localParticipant;
    if (participant == null) {
      throw StateError('livekit_local_participant_missing');
    }
    _screenSharePublication = await participant.setScreenShareEnabled(
      true,
      captureScreenAudio: captureSystemAudio,
      screenShareCaptureOptions: livekit.ScreenShareCaptureOptions(
        maxFrameRate: maxFrameRate,
        params: livekit.VideoParametersPresets.screenShareH1080FPS15,
      ),
    );
    _screenSharePaused = false;
  }

  @override
  Future<void> pauseScreenShare(bool paused) async {
    final track = _screenSharePublication?.track;
    if (track is livekit.LocalVideoTrack) {
      if (paused) {
        await track.disable();
      } else {
        await track.enable();
      }
      _screenSharePaused = paused;
    }
  }

  @override
  Future<void> stopScreenShare() async {
    final participant = _room.localParticipant;
    if (participant != null) {
      await participant.setScreenShareEnabled(false);
    }
    _screenSharePublication = null;
    _screenSharePaused = false;
  }

  @override
  List<livekit.RemoteVideoTrack> remoteScreenShareTracks({
    String? participantIdentity,
  }) {
    final tracks = <livekit.RemoteVideoTrack>[];
    for (final remote in _room.remoteParticipants.values) {
      if (participantIdentity != null &&
          remote.identity != participantIdentity) {
        continue;
      }
      for (final publication in remote.videoTrackPublications) {
        if (publication.source == livekit.TrackSource.screenShareVideo &&
            publication.track is livekit.RemoteVideoTrack) {
          tracks.add(publication.track as livekit.RemoteVideoTrack);
        }
      }
    }
    return tracks;
  }

  @override
  Future<void> disconnect() async {
    _listener?.dispose();
    _listener = null;
    _speakerMuted = false;
    _screenSharePublication = null;
    _screenSharePaused = false;
    try {
      await _room.disconnect();
    } catch (_) {}
  }
}
