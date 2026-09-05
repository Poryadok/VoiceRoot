import 'dart:async';
import 'dart:convert';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:livekit_client/livekit_client.dart' as livekit;
import 'package:voice_frontend/backend/livekit_room.dart';
import 'package:voice_frontend/backend/livekit_url.dart';
import 'package:voice_frontend/backend/realtime_client.dart';
import 'package:voice_frontend/backend/voice_client.dart';
import 'package:voice_frontend/backend/guest_credentials_storage.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/settings/voice_input_settings.dart';
import 'package:voice_frontend/state/call_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';

import 'support/gateway_test_client.dart';

class _FakeLiveKitRoom implements VoiceLiveKitRoom {
  int connectCalls = 0;
  String? lastUrl;
  bool connectThrows = false;
  bool endDisconnectCalled = false;
  bool throwOnSetSpeakerMuted = false;
  bool throwOnSetMuted = false;

  @override
  void Function(bool needsUnlock)? onAudioPlaybackUnlockNeeded;

  @override
  void Function()? onTracksChanged;

  @override
  Future<void> ensureAudioPlayback() async {}

  @override
  Future<void> connect({
    required String url,
    required String token,
    required bool video,
  }) async {
    connectCalls++;
    lastUrl = url;
    if (connectThrows) {
      throw Exception('livekit connect failed');
    }
  }

  @override
  Future<void> disconnect() async {
    endDisconnectCalled = true;
  }

  bool? lastSetMuted;

  @override
  Future<void> setMuted(bool muted) async {
    lastSetMuted = muted;
    if (throwOnSetMuted) throw Exception('mic failed');
  }

  @override
  Future<void> setSpeakerMuted(bool muted) async {
    if (throwOnSetSpeakerMuted) throw Exception('speaker failed');
  }

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
  }) => [];

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

VoiceCallSession _ringingSession({
  String roomId = 'room-1',
  String initiator = 'prof-caller',
  String callee = 'prof-test',
}) {
  return VoiceCallSession(
    roomId: roomId,
    livekitRoomName: 'voice-dm-$roomId',
    chatId: 'chat-1',
    initiatorProfileId: initiator,
    calleeProfileId: callee,
    mediaKind: VoiceCallMediaKind.audio,
    status: VoiceCallStatus.ringing,
  );
}

Map<String, dynamic> _sessionJson({
  required String roomId,
  required String initiator,
  required String callee,
  String status = 'CALL_STATUS_RINGING',
}) => {
  'room_id': roomId,
  'livekit_room_name': 'voice-dm-$roomId',
  'linked_chat': {'id': 'chat-1'},
  'initiator_profile_id': initiator,
  'callee_profile_id': callee,
  'media_kind': 'CALL_MEDIA_KIND_AUDIO',
  'status': status,
};

AuthController _authControllerForProfile(Ref ref, String profileId) {
  final controller = AuthController(
    authClient: ref.watch(voiceAuthClientProvider),
    storage: ref.watch(authSessionStorageProvider),
    guestCredentialsStorage: ref.watch(guestCredentialsStorageProvider),
  );
  controller.state = AuthState(
    session: AuthSession(
      accessToken: 'test-access',
      refreshToken: 'test-refresh',
      accountId: 'acc-test',
      activeProfileId: profileId,
      expiresInSeconds: 900,
    ),
  );
  return controller;
}

ProviderContainer _callTestContainer({
  required http.Client client,
  required StreamController<RealtimeFrame> realtime,
  StreamController<ProfileBoundRealtimeFrame>? boundRealtime,
  required _FakeLiveKitRoom fakeRoom,
  required String activeProfileId,
  GatewayConfig config = const GatewayConfig(
    baseUrl: 'http://api.test',
    livekitUrl: 'ws://127.0.0.1:7880',
  ),
}) {
  return ProviderContainer(
    overrides: [
      authSessionStorageProvider.overrideWithValue(
        InMemoryAuthSessionStorage(),
      ),
      guestCredentialsStorageProvider.overrideWithValue(
        InMemoryGuestCredentialsStorage(),
      ),
      httpClientProvider.overrideWithValue(client),
      gatewayConfigProvider.overrideWithValue(config),
      gatewayHttpClientProvider.overrideWithValue(
        gatewayHttpForTest(client, config: config),
      ),
      voiceCallsClientProvider.overrideWith(
        (ref) =>
            VoiceCallsClient(gateway: ref.watch(gatewayHttpClientProvider)),
      ),
      liveKitRoomFactoryProvider.overrideWithValue(() => fakeRoom),
      callSignalingStreamProvider.overrideWith((ref) => realtime.stream),
      if (boundRealtime != null)
        profileBoundRealtimeEventProvider.overrideWith(
          (ref) => boundRealtime.stream,
        ),
      authControllerProvider.overrideWith(
        (ref) => _authControllerForProfile(ref, activeProfileId),
      ),
      voiceInputSettingsProvider.overrideWith(
        () => _FixedVadVoiceInputSettings(),
      ),
    ],
  );
}

Future<void> _acceptBoundCall({
  required ProviderContainer container,
  required StreamController<RealtimeFrame> rawRealtime,
  required StreamController<ProfileBoundRealtimeFrame> boundRealtime,
  required VoiceCallSession session,
  required _FakeLiveKitRoom fakeRoom,
}) async {
  final activeSession = container.read(authControllerProvider).session!;
  final binding = RealtimeHelloBinding(
    generation: 1,
    bindingGeneration: 1,
    profileId: activeSession.activeProfileId,
    authorization: activeSession.authorizationHeader,
  );
  container.read(realtimeHelloBindingProvider.notifier).state = binding;
  boundRealtime.add(
    ProfileBoundRealtimeFrame(
      frame: RealtimeFrame(
        op: 'call_accepted',
        data: {'room_id': session.roomId},
      ),
      binding: binding,
    ),
  );
  await drainMicrotasks();
  expect(container.read(callControllerProvider).phase, CallPhase.active);
  expect(fakeRoom.connectCalls, 1);

  rawRealtime.add(
    RealtimeFrame(op: 'call_accepted', data: {'room_id': session.roomId}),
  );
  await drainMicrotasks();
  expect(container.read(callControllerProvider).phase, CallPhase.active);
  expect(fakeRoom.connectCalls, 1);
}

Future<void> drainMicrotasks({int rounds = 30}) async {
  for (var i = 0; i < rounds; i++) {
    await Future<void>.delayed(Duration.zero);
  }
}

void main() {
  test('resolveLivekitConnectUrl prefers client fallback for docker host', () {
    expect(
      resolveLivekitConnectUrl(
        apiUrl: 'ws://livekit:7880',
        clientFallback: 'ws://127.0.0.1:7880',
      ),
      'ws://127.0.0.1:7880',
    );
    expect(
      resolveLivekitConnectUrl(
        apiUrl: 'wss://livekit.example.com',
        clientFallback: 'ws://127.0.0.1:7880',
      ),
      'wss://livekit.example.com',
    );
  });

  test('startCall deduplicates in-flight outgoing to same peer', () async {
    final startCompleter = Completer<http.Response>();
    var startPosts = 0;
    final client = MockClient((req) async {
      if (req.method == 'POST' && req.url.path == '/api/v1/voice/calls') {
        startPosts++;
        return startCompleter.future;
      }
      return http.Response('{}', 404);
    });
    final realtime = StreamController<RealtimeFrame>.broadcast();
    final fakeRoom = _FakeLiveKitRoom();
    final container = _callTestContainer(
      client: client,
      realtime: realtime,
      fakeRoom: fakeRoom,
      activeProfileId: 'prof-test',
    );
    addTearDown(container.dispose);
    addTearDown(realtime.close);

    final notifier = container.read(callControllerProvider.notifier);
    final first = notifier.startCall(
      chatId: 'chat-1',
      calleeProfileId: 'peer-b',
    );
    await drainMicrotasks();
    await notifier.startCall(chatId: 'chat-1', calleeProfileId: 'peer-b');
    expect(startPosts, 1);

    startCompleter.complete(
      http.Response(
        jsonEncode({
          'call_session': _sessionJson(
            roomId: 'room-1',
            initiator: 'prof-test',
            callee: 'peer-b',
          ),
        }),
        200,
      ),
    );
    await first;
    expect(container.read(callControllerProvider).phase, CallPhase.outgoing);
  });

  test('call_accepted on WS does not reconnect callee', () async {
    final client = MockClient((req) async => http.Response('{}', 404));
    final realtime = StreamController<RealtimeFrame>.broadcast();
    final fakeRoom = _FakeLiveKitRoom();
    final session = _ringingSession();
    final container = _callTestContainer(
      client: client,
      realtime: realtime,
      fakeRoom: fakeRoom,
      activeProfileId: 'prof-test',
    );
    addTearDown(container.dispose);
    addTearDown(realtime.close);

    container.read(callControllerProvider.notifier).state = CallState(
      phase: CallPhase.incoming,
      session: session,
    );
    realtime.add(
      RealtimeFrame(op: 'call_accepted', data: {'room_id': session.roomId}),
    );
    await drainMicrotasks();
    expect(fakeRoom.connectCalls, 0);
    expect(container.read(callControllerProvider).phase, CallPhase.incoming);
    expect(
      container.read(callControllerProvider).session?.roomId,
      session.roomId,
    );
  });

  test('call_accepted on WS connects initiator to LiveKit', () async {
    final client = MockClient((req) async {
      if (req.method == 'GET' && req.url.path.endsWith('/token')) {
        return http.Response(
          jsonEncode({'jwt': 'jwt', 'livekit_url': 'ws://livekit:7880'}),
          200,
        );
      }
      return http.Response('{}', 404);
    });
    final realtime = StreamController<RealtimeFrame>.broadcast();
    final boundRealtime =
        StreamController<ProfileBoundRealtimeFrame>.broadcast();
    final fakeRoom = _FakeLiveKitRoom();
    final session = _ringingSession();
    final container = _callTestContainer(
      client: client,
      realtime: realtime,
      boundRealtime: boundRealtime,
      fakeRoom: fakeRoom,
      activeProfileId: 'prof-caller',
    );
    addTearDown(container.dispose);
    addTearDown(realtime.close);
    addTearDown(boundRealtime.close);

    container.read(callControllerProvider.notifier).state = CallState(
      phase: CallPhase.outgoing,
      session: session,
    );
    await _acceptBoundCall(
      container: container,
      rawRealtime: realtime,
      boundRealtime: boundRealtime,
      session: session,
      fakeRoom: fakeRoom,
    );
    expect(fakeRoom.connectCalls, 1);
    expect(fakeRoom.lastUrl, 'ws://127.0.0.1:7880');
    expect(container.read(callControllerProvider).phase, CallPhase.active);
  });

  test('syncs ringing incoming call when realtime link connects', () async {
    final session = _ringingSession(
      initiator: 'prof-caller',
      callee: 'prof-test',
    );
    final client = MockClient((req) async {
      if (req.method == 'GET' && req.url.path.endsWith('/calls/active')) {
        return http.Response(
          jsonEncode({
            'call_session': _sessionJson(
              roomId: session.roomId,
              initiator: session.initiatorProfileId,
              callee: session.calleeProfileId,
            ),
          }),
          200,
        );
      }
      return http.Response('{}', 404);
    });
    final realtime = StreamController<RealtimeFrame>.broadcast();
    final fakeRoom = _FakeLiveKitRoom();
    final container = _callTestContainer(
      client: client,
      realtime: realtime,
      fakeRoom: fakeRoom,
      activeProfileId: 'prof-test',
    );
    addTearDown(container.dispose);
    addTearDown(realtime.close);

    container.read(callControllerProvider);
    container.read(realtimeLinkStatusProvider.notifier).state =
        RealtimeLinkStatus.connected;
    await drainMicrotasks();

    final call = container.read(callControllerProvider);
    expect(call.phase, CallPhase.incoming);
    expect(call.session?.roomId, session.roomId);
  });

  test('LiveKit connect failure ends call and surfaces error', () async {
    var endCalls = 0;
    final session = _ringingSession();
    final client = MockClient((req) async {
      if (req.method == 'POST' && req.url.path.endsWith('/accept')) {
        return http.Response(
          jsonEncode({
            'call_session': _sessionJson(
              roomId: session.roomId,
              initiator: session.initiatorProfileId,
              callee: session.calleeProfileId,
              status: 'CALL_STATUS_ACTIVE',
            ),
          }),
          200,
        );
      }
      if (req.method == 'GET' && req.url.path.endsWith('/token')) {
        return http.Response(
          jsonEncode({'jwt': 'jwt', 'livekit_url': 'ws://127.0.0.1:7880'}),
          200,
        );
      }
      if (req.method == 'POST' && req.url.path.endsWith('/end')) {
        endCalls++;
        return http.Response('{}', 200);
      }
      return http.Response('{}', 404);
    });
    final realtime = StreamController<RealtimeFrame>.broadcast();
    final fakeRoom = _FakeLiveKitRoom()..connectThrows = true;
    final container = _callTestContainer(
      client: client,
      realtime: realtime,
      fakeRoom: fakeRoom,
      activeProfileId: 'prof-test',
    );
    addTearDown(container.dispose);
    addTearDown(realtime.close);

    container.read(callControllerProvider.notifier).state = CallState(
      phase: CallPhase.incoming,
      session: session,
    );
    await container.read(callControllerProvider.notifier).acceptCall();

    expect(endCalls, 1);
    expect(fakeRoom.endDisconnectCalled, isTrue);
    final call = container.read(callControllerProvider);
    expect(call.phase, CallPhase.failed);
    expect(call.errorMessage, 'livekit_connect_failed');
  });

  test('setSpeakerMuted reverts state when LiveKit throws', () async {
    final session = _ringingSession();
    final client = MockClient((req) async {
      if (req.method == 'GET' && req.url.path.endsWith('/token')) {
        return http.Response(
          jsonEncode({'jwt': 'jwt', 'livekit_url': 'ws://127.0.0.1:7880'}),
          200,
        );
      }
      return http.Response('{}', 404);
    });
    final realtime = StreamController<RealtimeFrame>.broadcast();
    final boundRealtime =
        StreamController<ProfileBoundRealtimeFrame>.broadcast();
    final fakeRoom = _FakeLiveKitRoom()..throwOnSetSpeakerMuted = true;
    final container = _callTestContainer(
      client: client,
      realtime: realtime,
      boundRealtime: boundRealtime,
      fakeRoom: fakeRoom,
      activeProfileId: 'prof-caller',
    );
    addTearDown(container.dispose);
    addTearDown(realtime.close);
    addTearDown(boundRealtime.close);

    final notifier = container.read(callControllerProvider.notifier);
    notifier.state = CallState(phase: CallPhase.outgoing, session: session);
    await _acceptBoundCall(
      container: container,
      rawRealtime: realtime,
      boundRealtime: boundRealtime,
      session: session,
      fakeRoom: fakeRoom,
    );
    expect(container.read(callControllerProvider).phase, CallPhase.active);

    notifier.state = container
        .read(callControllerProvider)
        .copyWith(isSpeakerMuted: true);
    await notifier.setSpeakerMuted(false);

    expect(container.read(callControllerProvider).isSpeakerMuted, isTrue);
  });

  test(
    'startGroupVoice connects LiveKit without outgoing overlay phase',
    () async {
      final client = MockClient((req) async {
        if (req.method == 'POST' && req.url.path == '/api/v1/voice/calls') {
          return http.Response(
            jsonEncode({
              'call_session': {
                'room_id': 'room-group-1',
                'livekit_room_name': 'voice-group-room-1',
                'room_type_enum': 'VOICE_SESSION_KIND_GROUP_VOICE',
                'linked_chat': {'id': 'group-1'},
                'initiator_profile_id': 'prof-test',
                'media_kind': 'CALL_MEDIA_KIND_AUDIO',
                'status': 'CALL_STATUS_ACTIVE',
              },
            }),
            200,
          );
        }
        if (req.method == 'GET' && req.url.path.endsWith('/token')) {
          return http.Response(
            jsonEncode({'jwt': 'jwt', 'livekit_url': 'ws://127.0.0.1:7880'}),
            200,
          );
        }
        return http.Response('{}', 404);
      });
      final realtime = StreamController<RealtimeFrame>.broadcast();
      final fakeRoom = _FakeLiveKitRoom();
      final container = _callTestContainer(
        client: client,
        realtime: realtime,
        fakeRoom: fakeRoom,
        activeProfileId: 'prof-test',
      );
      addTearDown(container.dispose);
      addTearDown(realtime.close);

      await container
          .read(callControllerProvider.notifier)
          .startGroupVoice(groupChatId: 'group-1');

      expect(fakeRoom.connectCalls, 1);
      final call = container.read(callControllerProvider);
      expect(call.phase, CallPhase.active);
      expect(call.session?.isGroupVoice, isTrue);
      expect(call.isOutgoing, isFalse);
    },
  );

  test(
    'active call keeps voiceBindingProfileId after profile switch',
    () async {
      final realtime = StreamController<RealtimeFrame>.broadcast();
      final container = _callTestContainer(
        client: MockClient((_) async => http.Response('{}', 404)),
        realtime: realtime,
        fakeRoom: _FakeLiveKitRoom(),
        activeProfileId: 'profile-a',
      );
      addTearDown(container.dispose);
      addTearDown(realtime.close);

      final session = VoiceCallSession(
        roomId: 'room-1',
        livekitRoomName: 'lk',
        chatId: 'chat-1',
        initiatorProfileId: 'profile-a',
        calleeProfileId: 'profile-b',
        mediaKind: VoiceCallMediaKind.audio,
        status: VoiceCallStatus.active,
      );

      container.read(callControllerProvider.notifier).state = CallState(
        phase: CallPhase.active,
        session: session,
        voiceBindingProfileId: 'profile-a',
      );

      final auth = container.read(authControllerProvider.notifier);
      final prev = auth.state.session!;
      auth.state = auth.state.copyWith(
        session: AuthSession(
          accessToken: prev.accessToken,
          refreshToken: prev.refreshToken,
          accountId: prev.accountId,
          activeProfileId: 'profile-b',
          expiresInSeconds: prev.expiresInSeconds,
          accountType: prev.accountType,
        ),
      );

      final call = container.read(callControllerProvider);
      expect(call.phase, CallPhase.active);
      expect(call.voiceBindingProfileId, 'profile-a');
      expect(call.session?.roomId, 'room-1');
    },
  );

  test('PTT hold unmutes LiveKit without flipping user mute', () async {
    final session = _ringingSession();
    final client = MockClient((req) async {
      if (req.method == 'GET' && req.url.path.endsWith('/token')) {
        return http.Response(
          jsonEncode({'jwt': 'jwt', 'livekit_url': 'ws://127.0.0.1:7880'}),
          200,
        );
      }
      if (req.url.path.contains('voice-state') ||
          req.url.path.contains('voice_state')) {
        return http.Response('{}', 200);
      }
      return http.Response('{}', 404);
    });
    final realtime = StreamController<RealtimeFrame>.broadcast();
    final boundRealtime =
        StreamController<ProfileBoundRealtimeFrame>.broadcast();
    final fakeRoom = _FakeLiveKitRoom();
    final container = ProviderContainer(
      overrides: [
        authSessionStorageProvider.overrideWithValue(
          InMemoryAuthSessionStorage(),
        ),
        guestCredentialsStorageProvider.overrideWithValue(
          InMemoryGuestCredentialsStorage(),
        ),
        httpClientProvider.overrideWithValue(client),
        gatewayConfigProvider.overrideWithValue(
          const GatewayConfig(
            baseUrl: 'http://api.test',
            livekitUrl: 'ws://127.0.0.1:7880',
          ),
        ),
        gatewayHttpClientProvider.overrideWithValue(
          gatewayHttpForTest(
            client,
            config: const GatewayConfig(
              baseUrl: 'http://api.test',
              livekitUrl: 'ws://127.0.0.1:7880',
            ),
          ),
        ),
        voiceCallsClientProvider.overrideWith(
          (ref) =>
              VoiceCallsClient(gateway: ref.watch(gatewayHttpClientProvider)),
        ),
        liveKitRoomFactoryProvider.overrideWithValue(() => fakeRoom),
        callSignalingStreamProvider.overrideWith((ref) => realtime.stream),
        profileBoundRealtimeEventProvider.overrideWith(
          (ref) => boundRealtime.stream,
        ),
        authControllerProvider.overrideWith(
          (ref) => _authControllerForProfile(ref, 'prof-caller'),
        ),
        voiceInputSettingsProvider.overrideWith(
          () => _FixedPttVoiceInputSettings(),
        ),
      ],
    );
    addTearDown(container.dispose);
    addTearDown(realtime.close);
    addTearDown(boundRealtime.close);

    final notifier = container.read(callControllerProvider.notifier);
    notifier.state = CallState(phase: CallPhase.outgoing, session: session);
    await _acceptBoundCall(
      container: container,
      rawRealtime: realtime,
      boundRealtime: boundRealtime,
      session: session,
      fakeRoom: fakeRoom,
    );
    expect(container.read(callControllerProvider).phase, CallPhase.active);

    await notifier.setPttHeld(false);
    expect(container.read(callControllerProvider).isMuted, isFalse);
    expect(container.read(callControllerProvider).isPttHeld, isFalse);
    expect(fakeRoom.lastSetMuted, isTrue);

    await notifier.setPttHeld(true);
    expect(container.read(callControllerProvider).isMuted, isFalse);
    expect(container.read(callControllerProvider).isPttHeld, isTrue);
    expect(fakeRoom.lastSetMuted, isFalse);

    await notifier.setMuted(true);
    await notifier.setPttHeld(true);
    expect(fakeRoom.lastSetMuted, isTrue);
  });
}

class _FixedVadVoiceInputSettings extends VoiceInputSettingsNotifier {
  @override
  VoiceInputSettings build() => const VoiceInputSettings();
}

class _FixedPttVoiceInputSettings extends VoiceInputSettingsNotifier {
  @override
  VoiceInputSettings build() =>
      const VoiceInputSettings(mode: VoiceInputMode.ptt);
}
