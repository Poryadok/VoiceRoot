import 'dart:async';
import 'dart:convert';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:livekit_client/livekit_client.dart' as livekit;
import 'package:voice_frontend/backend/auth_client.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/guest_credentials_storage.dart';
import 'package:voice_frontend/backend/livekit_room.dart';
import 'package:voice_frontend/backend/realtime_client.dart';
import 'package:voice_frontend/backend/voice_client.dart';
import 'package:voice_frontend/settings/voice_input_settings.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/call_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/state/profile_switch_coordinator.dart';

import 'support/gateway_test_client.dart';

/// T-053 Cycle 5 RED: multi-profile.md says an A-owned voice session survives
/// profile switches, while a different active profile must first explicitly
/// leave A before entering another voice room. This suite does not define how
/// post-revocation A controls are credentialed.
void main() {
  test(
    'A-owned active LiveKit session survives real coordinator A to B to C',
    () async {
      final harness = _VoiceBindingHarness();
      addTearDown(harness.dispose);

      await harness.connectActiveA();
      final before = harness.container.read(callControllerProvider);
      expect(before.voiceBindingProfileId, 'profile-a');
      expect(before.mediaTracksVersion, 7);
      expect(harness.liveKit.connectCalls, 1);

      expect(
        await harness.coordinator.switchTo('profile-b'),
        isA<ProfileSwitchApplied>(),
      );
      expect(
        await harness.coordinator.switchTo('profile-c'),
        isA<ProfileSwitchApplied>(),
      );

      final after = harness.container.read(callControllerProvider);
      expect(harness.auth.state.session, _session('profile-c'));
      expect(after.phase, CallPhase.active);
      expect(after.session?.roomId, 'room-a');
      expect(after.session?.livekitRoomName, 'livekit-a');
      expect(after.voiceBindingProfileId, 'profile-a');
      expect(after.mediaTracksVersion, 7);
      expect(harness.liveKit.connectCalls, 1);
      expect(harness.liveKit.disconnectCalls, 0);
      expect(
        harness.voicePaths.where(
          (path) => path.endsWith('/end') || path.contains('/leave'),
        ),
        isEmpty,
        reason: 'a passive profile handoff must not leave or end A voice',
      );
    },
  );

  final entries = <_VoiceEntry>[
    _VoiceEntry(
      'startCall',
      (controller) =>
          controller.startCall(chatId: 'chat-new', calleeProfileId: 'peer-new'),
    ),
    _VoiceEntry(
      'joinVoiceRoom',
      (controller) => controller.joinVoiceRoom(
        voiceRoomId: 'space-room-new',
        spaceId: 'space-new',
      ),
    ),
    _VoiceEntry(
      'startGroupVoice',
      (controller) => controller.startGroupVoice(groupChatId: 'group-new'),
    ),
    _VoiceEntry(
      'joinGroupVoice',
      (controller) => controller.joinGroupVoice(roomId: 'group-room-new'),
    ),
  ];

  for (final profileId in const ['profile-b', 'profile-c']) {
    for (final entry in entries) {
      test(
        '$profileId ${entry.name} conflicts with active A voice without Voice API or LiveKit start',
        () async {
          final harness = _VoiceBindingHarness();
          addTearDown(harness.dispose);
          harness.setActiveA();

          expect(
            await harness.coordinator.switchTo(profileId),
            isA<ProfileSwitchApplied>(),
          );
          await entry.invoke(harness.call);

          final state = harness.container.read(callControllerProvider);
          expect(state.phase, CallPhase.active);
          expect(state.session?.roomId, 'room-a');
          expect(state.voiceBindingProfileId, 'profile-a');
          expect(state.errorMessage, 'voice_session_conflict');
          expect(harness.voicePaths, isEmpty);
          expect(harness.liveKit.connectCalls, 0);
          expect(harness.liveKit.disconnectCalls, 0);
        },
      );
    }
  }

  group('profile-bound call signaling contract', () {
    test(
      'late A or B signals cannot replace or end active A after C switch',
      () async {
        final harness = _VoiceBindingHarness();
        addTearDown(harness.dispose);
        harness.setActiveA();
        await harness.coordinator.switchTo('profile-b');
        await harness.coordinator.switchTo('profile-c');
        final cBinding = _binding('profile-c', generation: 3);
        harness.container.read(realtimeHelloBindingProvider.notifier).state =
            cBinding;

        for (final stale in [
          ProfileBoundRealtimeFrame(
            frame: const RealtimeFrame(
              op: 'call_incoming',
              data: {'room_id': 'room-b'},
            ),
            binding: _binding('profile-a', generation: 1),
          ),
          ProfileBoundRealtimeFrame(
            frame: const RealtimeFrame(
              op: 'call_accepted',
              data: {'room_id': 'room-a'},
            ),
            binding: _binding('profile-b', generation: 2),
          ),
          ProfileBoundRealtimeFrame(
            frame: const RealtimeFrame(
              op: 'call_ended',
              data: {'room_id': 'room-a'},
            ),
            binding: _binding('profile-a', generation: 1),
          ),
        ]) {
          harness.boundSignals.add(stale);
        }
        await _drain();

        final state = harness.container.read(callControllerProvider);
        expect(harness.auth.state.session, _session('profile-c'));
        expect(state.phase, CallPhase.active);
        expect(state.session?.roomId, 'room-a');
        expect(state.voiceBindingProfileId, 'profile-a');
        expect(harness.liveKit.disconnectCalls, 0);
      },
    );

    test('idle C accepts exactly one C-bound incoming call', () async {
      final harness = _VoiceBindingHarness();
      addTearDown(harness.dispose);
      await harness.coordinator.switchTo('profile-b');
      await harness.coordinator.switchTo('profile-c');
      final cBinding = _binding('profile-c', generation: 3);
      harness.container.read(realtimeHelloBindingProvider.notifier).state =
          cBinding;
      var acceptedCIncoming = 0;
      final stateSub = harness.container.listen<CallState>(
        callControllerProvider,
        (_, next) {
          if (next.phase == CallPhase.incoming &&
              next.session?.roomId == 'room-c') {
            acceptedCIncoming++;
          }
        },
      );
      addTearDown(stateSub.close);
      harness.boundSignals
        ..add(
          ProfileBoundRealtimeFrame(
            frame: const RealtimeFrame(
              op: 'call_incoming',
              data: {'room_id': 'room-a-late'},
            ),
            binding: _binding('profile-a', generation: 1),
          ),
        )
        ..add(
          ProfileBoundRealtimeFrame(
            frame: const RealtimeFrame(
              op: 'call_incoming',
              data: {'room_id': 'room-b-late'},
            ),
            binding: _binding('profile-b', generation: 2),
          ),
        );
      await _drain();
      expect(
        harness.container.read(callControllerProvider).phase,
        CallPhase.idle,
      );
      final incoming = ProfileBoundRealtimeFrame(
        frame: const RealtimeFrame(
          op: 'call_incoming',
          data: {
            'room_id': 'room-c',
            'livekit_room_name': 'livekit-c',
            'chat_id': 'chat-c',
            'initiator_profile_id': 'peer-c',
            'callee_profile_id': 'profile-c',
          },
        ),
        binding: cBinding,
      );

      harness.boundSignals
        ..add(incoming)
        ..add(incoming);
      await _drain();

      final state = harness.container.read(callControllerProvider);
      expect(state.phase, CallPhase.incoming);
      expect(state.session?.roomId, 'room-c');
      expect(state.voiceBindingProfileId, 'profile-c');
      expect(acceptedCIncoming, 1);
    });
  });
}

AuthSession _session(String profileId) => AuthSession(
  accessToken: 'access-$profileId',
  refreshToken: 'refresh-$profileId',
  accountId: 'account-1',
  activeProfileId: profileId,
  expiresInSeconds: 900,
);

VoiceCallSession _aSession() => VoiceCallSession(
  roomId: 'room-a',
  livekitRoomName: 'livekit-a',
  chatId: 'chat-a',
  initiatorProfileId: 'profile-a',
  calleeProfileId: 'peer-a',
  mediaKind: VoiceCallMediaKind.audio,
  status: VoiceCallStatus.active,
);

RealtimeHelloBinding _binding(String profileId, {required int generation}) =>
    RealtimeHelloBinding(
      generation: generation,
      bindingGeneration: generation,
      profileId: profileId,
      authorization: 'Bearer access-$profileId',
    );

class _VoiceEntry {
  const _VoiceEntry(this.name, this.invoke);

  final String name;
  final Future<void> Function(CallController controller) invoke;
}

class _VoiceBindingHarness {
  _VoiceBindingHarness() {
    final client = MockClient(_respond);
    auth = AuthController(
      authClient: VoiceAuthClient(gateway: gatewayHttpForTest(client)),
      storage: storage,
      guestCredentialsStorage: InMemoryGuestCredentialsStorage(),
    )..state = AuthState(session: _session('profile-a'));
    container = ProviderContainer(
      overrides: [
        authControllerProvider.overrideWith((_) => auth),
        authSessionStorageProvider.overrideWithValue(storage),
        guestCredentialsStorageProvider.overrideWithValue(
          InMemoryGuestCredentialsStorage(),
        ),
        gatewayConfigProvider.overrideWithValue(
          const GatewayConfig(
            baseUrl: 'http://api.test',
            livekitUrl: 'ws://127.0.0.1:7880',
          ),
        ),
        httpClientProvider.overrideWithValue(client),
        gatewayHttpClientProvider.overrideWithValue(gatewayHttpForTest(client)),
        voiceCallsClientProvider.overrideWith(
          (ref) =>
              VoiceCallsClient(gateway: ref.watch(gatewayHttpClientProvider)),
        ),
        liveKitRoomFactoryProvider.overrideWithValue(() => liveKit),
        callSignalingStreamProvider.overrideWith((_) => signaling.stream),
        profileBoundRealtimeEventProvider.overrideWith(
          (_) => boundSignals.stream,
        ),
        profileSwitchRealtimeBoundaryProvider.overrideWithValue(realtime),
        voiceInputSettingsProvider.overrideWith(() => _FixedVoiceInput()),
      ],
    );
    call = container.read(callControllerProvider.notifier);
    coordinator = container.read(profileSwitchCoordinatorProvider);
  }

  final storage = InMemoryAuthSessionStorage();
  final signaling = StreamController<RealtimeFrame>.broadcast();
  final boundSignals = StreamController<ProfileBoundRealtimeFrame>.broadcast();
  final liveKit = _RecordingLiveKitRoom();
  final realtime = _PassiveProfileSwitchRealtimeBoundary();
  final requestPaths = <String>[];
  late final AuthController auth;
  late final ProviderContainer container;
  late final CallController call;
  late final ProfileSwitchCoordinator coordinator;

  Iterable<String> get voicePaths =>
      requestPaths.where((path) => path.startsWith('/api/v1/voice/'));

  void setActiveA() {
    call.state = CallState(
      phase: CallPhase.active,
      session: _aSession(),
      voiceBindingProfileId: 'profile-a',
      mediaTracksVersion: 7,
    );
  }

  Future<void> connectActiveA() async {
    call.state = CallState(
      phase: CallPhase.outgoing,
      session: _aSession(),
      voiceBindingProfileId: 'profile-a',
      mediaTracksVersion: 7,
    );
    signaling.add(
      const RealtimeFrame(op: 'call_accepted', data: {'room_id': 'room-a'}),
    );
    await _drain();
    expect(container.read(callControllerProvider).phase, CallPhase.active);
  }

  Future<http.Response> _respond(http.Request request) async {
    requestPaths.add(request.url.path);
    if (request.url.path == '/api/v1/auth/switch-profile') {
      final body = jsonDecode(request.body) as Map<String, dynamic>;
      final profileId = body['profile_id'] as String?;
      if (profileId == 'profile-b' || profileId == 'profile-c') {
        return utf8JsonResponse(jsonEncode(_session(profileId!).toJson()));
      }
      return http.Response('unknown profile', 400);
    }
    if (request.url.path == '/api/v1/voice/calls/room-a/token') {
      return http.Response(
        jsonEncode({'jwt': 'livekit-a', 'livekit_url': 'ws://127.0.0.1:7880'}),
        200,
      );
    }
    return http.Response('{}', 404);
  }

  Future<void> dispose() async {
    container.dispose();
    await signaling.close();
    await boundSignals.close();
  }
}

class _PassiveProfileSwitchRealtimeBoundary
    implements ProfileSwitchRealtimeBoundary {
  @override
  Set<String> get activeSubscriptions => const {};

  @override
  Future<void> retireAndReconnect(ProfileSwitchHandoff handoff) async {}
}

class _FixedVoiceInput extends VoiceInputSettingsNotifier {
  @override
  VoiceInputSettings build() => const VoiceInputSettings();
}

class _RecordingLiveKitRoom implements VoiceLiveKitRoom {
  var connectCalls = 0;
  var disconnectCalls = 0;

  @override
  void Function(bool needsUnlock)? onAudioPlaybackUnlockNeeded;

  @override
  void Function()? onTracksChanged;

  @override
  Future<void> connect({
    required String url,
    required String token,
    required bool video,
  }) async {
    connectCalls++;
  }

  @override
  Future<void> disconnect() async {
    disconnectCalls++;
  }

  @override
  Future<void> ensureAudioPlayback() async {}

  @override
  Future<void> setMuted(bool muted) async {}

  @override
  Future<void> setSpeakerMuted(bool muted) async {}

  @override
  Future<void> setCommanderDucking({
    required bool enabled,
    String? commanderIdentity,
    double duckedVolume = 0.2,
  }) async {}

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
  }) => const [];

  @override
  livekit.LocalVideoTrack? localCameraTrack() => null;

  @override
  livekit.LocalVideoTrack? localScreenShareTrack() => null;

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

Future<void> _drain() async {
  for (var i = 0; i < 8; i++) {
    await Future<void>.delayed(Duration.zero);
  }
}
