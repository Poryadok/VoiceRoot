import 'dart:async';

import 'package:livekit_client/livekit_client.dart' as livekit;

import 'ordinary_voice_identity.dart';

abstract interface class OrdinaryVoicePresentationRoom
    implements VoiceLiveKitRoom {
  set ordinaryVoicePresentation(bool enabled);
}

abstract interface class MatchSquadVoiceLiveKitRoom
    implements VoiceLiveKitRoom {
  String? resolveRemoteParticipantIdentity(String profileId);
  bool matchesLocalParticipantIdentity(String profileId, String? mediaEpoch);
}

/// Returns the one current-room MatchSquad identity belonging to [profileId].
/// Multiple epochs or malformed identities are intentionally unresolved.
String? resolveMatchSquadRemoteIdentity(
  String profileId,
  Iterable<String> participantIdentities,
) {
  if (!_isCanonicalNonzeroUuid(profileId)) return null;
  final matches = participantIdentities
      .where((identity) => _matchSquadIdentityProfile(identity) == profileId)
      .toList(growable: false);
  return matches.length == 1 ? matches.single : null;
}

String? _matchSquadIdentityProfile(String identity) {
  final parts = identity.split(':');
  if (parts.length != 3 ||
      parts[0] != 'ms' ||
      !_isCanonicalNonzeroUuid(parts[1]) ||
      !_isCanonicalNonzeroUuid(parts[2])) {
    return null;
  }
  return parts[1];
}

String? _matchSquadIdentity(String profileId, String? mediaEpoch) {
  if (!_isCanonicalNonzeroUuid(profileId) ||
      mediaEpoch == null ||
      !_isCanonicalNonzeroUuid(mediaEpoch)) {
    return null;
  }
  return 'ms:$profileId:$mediaEpoch';
}

bool _isCanonicalNonzeroUuid(String value) =>
    RegExp(r'^[0-9a-f]{8}-(?:[0-9a-f]{4}-){3}[0-9a-f]{12}$').hasMatch(value) &&
    value != '00000000-0000-0000-0000-000000000000';

bool matchesMatchSquadLocalIdentity(
  String profileId,
  String? mediaEpoch,
  String? sdkIdentity,
) {
  final expected = _matchSquadIdentity(profileId, mediaEpoch);
  return expected != null && sdkIdentity == expected;
}

final RegExp _spaceMediaParticipantIdentity = RegExp(
  r'^profile:([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}):media:([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})$',
);

String? _profileIdFromLiveKitIdentity(
  String identity, {
  bool ordinary = false,
}) {
  if (_isCanonicalNonzeroUuid(identity)) return identity;
  if (ordinary) {
    final parsed = OrdinaryVoiceMediaIdentity.parse(identity);
    if (parsed != null) return parsed.profileId;
  }

  final match = _spaceMediaParticipantIdentity.firstMatch(identity);
  if (match == null) return null;
  final profileId = match.group(1)!;
  final generationId = match.group(2)!;
  if (!_isCanonicalNonzeroUuid(profileId) ||
      !_isCanonicalNonzeroUuid(generationId)) {
    return null;
  }
  return profileId;
}

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

  /// Client-side commander ducking (voice-chat.md): lower non-commander remote audio.
  Future<void> setCommanderDucking({
    required bool enabled,
    String? commanderIdentity,
    double duckedVolume,
  });
  Future<void> setVideoEnabled(bool enabled);
  Future<void> startScreenShare({
    double maxFrameRate,
    bool captureSystemAudio,
    String? sourceId,
  });
  Future<void> pauseScreenShare(bool paused);
  Future<void> stopScreenShare();
  bool get isScreenSharing;
  bool get isScreenSharePaused;
  List<livekit.RemoteVideoTrack> remoteScreenShareTracks({
    String? participantIdentity,
  });
  livekit.LocalVideoTrack? localCameraTrack();
  livekit.LocalVideoTrack? localScreenShareTrack();
  livekit.RemoteVideoTrack? remoteCameraTrack();
  Future<void> disconnect();
}

class VoiceCallParticipant {
  const VoiceCallParticipant({
    required this.identity,
    required this.name,
    required this.isSpeaking,
    this.profileId,
    this.avatarUrl,
  });
  final String identity;
  final String name;
  final bool isSpeaking;
  final String? profileId;
  final String? avatarUrl;
}

abstract interface class VoiceCallPresentationRoom implements VoiceLiveKitRoom {
  List<VoiceCallParticipant> get callParticipants;
  void Function()? onParticipantsChanged;
}

class LiveKitVoiceRoom
    implements
        MatchSquadVoiceLiveKitRoom,
        OrdinaryVoicePresentationRoom,
        VoiceCallPresentationRoom {
  LiveKitVoiceRoom({livekit.Room? room, this.ordinaryVoicePresentation = false})
    : _room = room ?? livekit.Room();

  /// Presentation dispatch only; never an authorization or SDK resource key.
  bool ordinaryVoicePresentation;

  static const Duration _connectTimeout = Duration(seconds: 20);
  static const Duration _mediaEnableTimeout = Duration(seconds: 12);

  final livekit.Room _room;
  livekit.EventsListener<livekit.RoomEvent>? _listener;
  bool _speakerMuted = false;
  bool _screenSharePaused = false;
  livekit.LocalTrackPublication<livekit.LocalTrack>? _screenSharePublication;

  livekit.Room get room => _room;
  @override
  void Function()? onParticipantsChanged;

  /// SDK names and opaque identities are presentation only, never API keys.
  @override
  List<VoiceCallParticipant> get callParticipants {
    final participants = <livekit.Participant>[
      if (_room.localParticipant != null) _room.localParticipant!,
      ..._room.remoteParticipants.values,
    ];
    final speakers = _room.activeSpeakers.map((p) => p.identity).toSet();
    final ordered = [
      ...participants.where((p) => speakers.contains(p.identity)),
      ...participants.where((p) => !speakers.contains(p.identity)),
    ];
    return [
      for (final p in ordered)
        VoiceCallParticipant(
          identity: p.identity,
          name: p.name,
          isSpeaking: speakers.contains(p.identity),
          profileId:
              _profileIdFromLiveKitIdentity(
                p.identity,
                ordinary: ordinaryVoicePresentation,
              ) ??
              _matchSquadIdentityProfile(p.identity),
        ),
    ];
  }

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
      ..on<livekit.ActiveSpeakersChangedEvent>((event) {
        onParticipantsChanged?.call();
      })
      ..on<livekit.ParticipantConnectedEvent>((event) {
        _notifyTracksChanged();
      })
      ..on<livekit.ParticipantDisconnectedEvent>((event) {
        _notifyTracksChanged();
      })
      ..on<livekit.AudioPlaybackStatusChanged>((event) {
        if (!_room.canPlaybackAudio) {
          onAudioPlaybackUnlockNeeded?.call(true);
        }
      })
      ..on<livekit.TrackSubscribedEvent>((event) {
        if (event.track.kind == livekit.TrackType.AUDIO) {
          if (_speakerMuted) {
            unawaited(_setRemoteAudioTrackAudible(event.track, audible: false));
          }
          unawaited(ensureAudioPlayback());
        }
        if (event.track.kind == livekit.TrackType.VIDEO &&
            (event.publication.source == livekit.TrackSource.camera ||
                event.publication.source ==
                    livekit.TrackSource.screenShareVideo)) {
          _notifyTracksChanged();
        }
      })
      ..on<livekit.TrackUnsubscribedEvent>((event) {
        if (event.track.kind == livekit.TrackType.VIDEO &&
            (event.publication.source == livekit.TrackSource.camera ||
                event.publication.source ==
                    livekit.TrackSource.screenShareVideo)) {
          _notifyTracksChanged();
        }
      })
      ..on<livekit.LocalTrackPublishedEvent>((event) {
        if (event.publication.source == livekit.TrackSource.camera ||
            event.publication.source == livekit.TrackSource.screenShareVideo) {
          _notifyTracksChanged();
        }
      })
      ..on<livekit.LocalTrackUnpublishedEvent>((event) {
        if (event.publication.source == livekit.TrackSource.camera ||
            event.publication.source == livekit.TrackSource.screenShareVideo) {
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
    await _applyRemoteSpeakerMuted(muted);
    if (!muted) {
      await ensureAudioPlayback();
    }
  }

  @override
  Future<void> setCommanderDucking({
    required bool enabled,
    String? commanderIdentity,
    double duckedVolume = 0.2,
  }) async {
    if (_speakerMuted) return;
    // livekit_client 2.8 has no RemoteAudioTrack.setVolume; approximate ducking
    // by muting non-commander remotes while broadcast is active (spec: client ducking).
    final muteOthers = enabled && duckedVolume < 1.0;
    final commanderProfileId =
        commanderIdentity != null && _isCanonicalNonzeroUuid(commanderIdentity)
        ? commanderIdentity
        : null;
    for (final participant in _room.remoteParticipants.values) {
      final isCommander =
          commanderProfileId != null &&
          _profileIdFromLiveKitIdentity(
                participant.identity,
                ordinary: ordinaryVoicePresentation,
              ) ==
              commanderProfileId;
      final audible = !(muteOthers && !isCommander);
      for (final publication in participant.audioTrackPublications) {
        final track = publication.track;
        if (track == null) continue;
        await _setRemoteAudioTrackAudible(track, audible: audible);
      }
    }
  }

  @override
  String? resolveRemoteParticipantIdentity(String profileId) =>
      resolveMatchSquadRemoteIdentity(
        profileId,
        _room.remoteParticipants.values.map(
          (participant) => participant.identity,
        ),
      );

  @override
  bool matchesLocalParticipantIdentity(String profileId, String? mediaEpoch) {
    return matchesMatchSquadLocalIdentity(
      profileId,
      mediaEpoch,
      _room.localParticipant?.identity,
    );
  }

  Future<void> _applyRemoteSpeakerMuted(bool muted) async {
    for (final participant in _room.remoteParticipants.values) {
      for (final publication in participant.audioTrackPublications) {
        final track = publication.track;
        if (track == null) continue;
        await _setRemoteAudioTrackAudible(track, audible: !muted);
      }
    }
  }

  Future<void> _setRemoteAudioTrackAudible(
    livekit.Track track, {
    required bool audible,
  }) async {
    try {
      if (audible) {
        await track.enable();
        if (track is livekit.RemoteAudioTrack) {
          await track.start();
        }
      } else {
        await track.disable();
      }
    } catch (_) {
      // Web audio elements can fail transiently during subscribe/unmute.
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
  livekit.LocalVideoTrack? localScreenShareTrack() {
    final participant = _room.localParticipant;
    if (participant == null) return null;
    for (final publication in participant.videoTrackPublications) {
      if (publication.source == livekit.TrackSource.screenShareVideo &&
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
    String? sourceId,
  }) async {
    final participant = _room.localParticipant;
    if (participant == null) {
      throw StateError('livekit_local_participant_missing');
    }
    _screenSharePublication = await participant.setScreenShareEnabled(
      true,
      captureScreenAudio: captureSystemAudio,
      screenShareCaptureOptions: livekit.ScreenShareCaptureOptions(
        sourceId: sourceId,
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
    final exactOrdinary =
        ordinaryVoicePresentation && participantIdentity != null
        ? OrdinaryVoiceMediaIdentity.parse(participantIdentity)
        : null;
    if (participantIdentity != null &&
        !_isCanonicalNonzeroUuid(participantIdentity) &&
        exactOrdinary == null) {
      return const [];
    }
    final candidates = _room.remoteParticipants.values
        .where((remote) {
          if (participantIdentity == null) return true;
          if (exactOrdinary != null) {
            return remote.identity == exactOrdinary.sdkIdentity;
          }
          return _profileIdFromLiveKitIdentity(
                remote.identity,
                ordinary: ordinaryVoicePresentation,
              ) ==
              participantIdentity;
        })
        .toList(growable: false);
    // The API profile does not choose a membership generation. Keep ambiguous
    // SDK resources distinct until the old exact participant has been removed.
    if (participantIdentity != null &&
        exactOrdinary == null &&
        candidates.length > 1 &&
        candidates.any(
          (remote) =>
              ordinaryVoicePresentation &&
              OrdinaryVoiceMediaIdentity.parse(remote.identity) != null,
        )) {
      return const [];
    }
    final tracks = <livekit.RemoteVideoTrack>[];
    for (final remote in candidates) {
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
    onParticipantsChanged = null;
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
