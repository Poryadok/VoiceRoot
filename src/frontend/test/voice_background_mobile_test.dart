import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:livekit_client/livekit_client.dart' as livekit;
import 'package:voice_frontend/backend/livekit_room.dart';
import 'package:voice_frontend/backend/voice_client.dart';
import 'package:voice_frontend/backend/realtime_client.dart';
import 'package:voice_frontend/state/call_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/voice_background_mobile.dart';

import 'support/auth_test_overrides.dart';

class _FakeRealtimeHub extends RealtimeHub {
  _FakeRealtimeHub(super.ref);

  @override
  Stream<RealtimeFrame> get events => const Stream.empty();

  @override
  Future<void> ensureConnected() async {}

  @override
  void ensureSubscribed(String chatId) {}
}

void main() {
  test('voice background session activates when call becomes active', () async {
    final recording = RecordingVoiceBackgroundSession();
    voiceBackgroundSessionTestOverride = recording;
    addTearDown(() => voiceBackgroundSessionTestOverride = null);

    final container = ProviderContainer(
      overrides: [
        ...voiceAppTestOverrides(
          client: MockClient((_) async => http.Response('{}', 200)),
        ),
        voiceBackgroundSessionProvider.overrideWithValue(recording),
        liveKitRoomFactoryProvider.overrideWithValue(() => _FakeLiveKitRoom()),
        realtimeHubProvider.overrideWith(_FakeRealtimeHub.new),
      ],
    );
    addTearDown(container.dispose);

    final controller = container.read(callControllerProvider.notifier);
    controller.state = CallState(
      phase: CallPhase.connecting,
      session: const VoiceCallSession(
        roomId: 'room-1',
        livekitRoomName: 'lk-room',
        chatId: 'chat-1',
        initiatorProfileId: 'me',
        calleeProfileId: 'peer',
        mediaKind: VoiceCallMediaKind.audio,
        status: VoiceCallStatus.ringing,
      ),
    );
    await pumpEventQueue();

    expect(recording.isActive, isTrue);

    controller.state = const CallState(phase: CallPhase.idle);
    await pumpEventQueue();

    expect(recording.isActive, isFalse);
    expect(recording.callCount, greaterThanOrEqualTo(2));
  });
}

class _FakeLiveKitRoom implements VoiceLiveKitRoom {
  @override
  void Function(bool needsUnlock)? onAudioPlaybackUnlockNeeded;

  @override
  void Function()? onTracksChanged;

  @override
  Future<void> connect({
    required String url,
    required String token,
    required bool video,
  }) async {}

  @override
  Future<void> disconnect() async {}

  @override
  Future<void> ensureAudioPlayback() async {}

  @override
  Future<void> setMuted(bool muted) async {}

  @override
  Future<void> setSpeakerMuted(bool muted) async {}

  @override
  Future<void> setVideoEnabled(bool enabled) async {}

  @override
  bool get isScreenSharing => false;

  @override
  bool get isScreenSharePaused => false;

  @override
  Future<void> pauseScreenShare(bool paused) async {}

  @override
  List<livekit.RemoteVideoTrack> remoteScreenShareTracks({
    String? participantIdentity,
  }) => [];

  @override
  livekit.LocalVideoTrack? localCameraTrack() => null;

  @override
  livekit.RemoteVideoTrack? remoteCameraTrack() => null;

  @override
  Future<void> startScreenShare({
    double maxFrameRate = 15,
    bool captureSystemAudio = false,
  }) async {}

  @override
  Future<void> stopScreenShare() async {}
}
