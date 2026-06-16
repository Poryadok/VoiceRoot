import 'dart:async';
import 'dart:convert';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:livekit_client/livekit_client.dart' as livekit;
import 'package:voice_frontend/backend/livekit_room.dart';
import 'package:voice_frontend/backend/livekit_url.dart';
import 'package:voice_frontend/backend/realtime_client.dart';
import 'package:voice_frontend/backend/voice_client.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/state/auth_providers.dart';
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

  @override
  Future<void> setMuted(bool muted) async {
    if (throwOnSetMuted) throw Exception('mic failed');
  }

  @override
  Future<void> setSpeakerMuted(bool muted) async {
    if (throwOnSetSpeakerMuted) throw Exception('speaker failed');
  }

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
  required _FakeLiveKitRoom fakeRoom,
  required String activeProfileId,
  GatewayConfig config = const GatewayConfig(
    baseUrl: 'http://api.test',
    livekitUrl: 'ws://127.0.0.1:7880',
  ),
}) {
  return ProviderContainer(
    overrides: [
      authSessionStorageProvider.overrideWithValue(InMemoryAuthSessionStorage()),
      httpClientProvider.overrideWithValue(client),
      gatewayConfigProvider.overrideWithValue(config),
      gatewayHttpClientProvider.overrideWithValue(
        gatewayHttpForTest(client, config: config),
      ),
      voiceCallsClientProvider.overrideWith(
        (ref) => VoiceCallsClient(
          gateway: ref.watch(gatewayHttpClientProvider),
        ),
      ),
      liveKitRoomFactoryProvider.overrideWithValue(() => fakeRoom),
      callSignalingStreamProvider.overrideWith((ref) => realtime.stream),
      authControllerProvider.overrideWith(
        (ref) => _authControllerForProfile(ref, activeProfileId),
      ),
    ],
  );
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
    expect(container.read(callControllerProvider).session?.roomId, session.roomId);
  });

  test('call_accepted on WS connects initiator to LiveKit', () async {
    final client = MockClient((req) async {
      if (req.method == 'GET' && req.url.path.endsWith('/token')) {
        return http.Response(
          jsonEncode({
            'jwt': 'jwt',
            'livekit_url': 'ws://livekit:7880',
          }),
          200,
        );
      }
      return http.Response('{}', 404);
    });
    final realtime = StreamController<RealtimeFrame>.broadcast();
    final fakeRoom = _FakeLiveKitRoom();
    final session = _ringingSession();
    final container = _callTestContainer(
      client: client,
      realtime: realtime,
      fakeRoom: fakeRoom,
      activeProfileId: 'prof-caller',
    );
    addTearDown(container.dispose);
    addTearDown(realtime.close);

    container.read(callControllerProvider.notifier).state = CallState(
      phase: CallPhase.outgoing,
      session: session,
    );
    realtime.add(
      RealtimeFrame(op: 'call_accepted', data: {'room_id': session.roomId}),
    );
    await drainMicrotasks();
    expect(fakeRoom.connectCalls, 1);
    expect(fakeRoom.lastUrl, 'ws://127.0.0.1:7880');
    expect(container.read(callControllerProvider).phase, CallPhase.active);
  });

  test('syncs ringing incoming call when realtime link connects', () async {
    final session = _ringingSession(initiator: 'prof-caller', callee: 'prof-test');
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
    final fakeRoom = _FakeLiveKitRoom()..throwOnSetSpeakerMuted = true;
    final container = _callTestContainer(
      client: client,
      realtime: realtime,
      fakeRoom: fakeRoom,
      activeProfileId: 'prof-caller',
    );
    addTearDown(container.dispose);
    addTearDown(realtime.close);

    final notifier = container.read(callControllerProvider.notifier);
    notifier.state = CallState(
      phase: CallPhase.outgoing,
      session: session,
    );
    realtime.add(
      RealtimeFrame(op: 'call_accepted', data: {'room_id': session.roomId}),
    );
    await drainMicrotasks();
    expect(container.read(callControllerProvider).phase, CallPhase.active);

    notifier.state = container.read(callControllerProvider).copyWith(
      isSpeakerMuted: true,
    );
    await notifier.setSpeakerMuted(false);

    expect(container.read(callControllerProvider).isSpeakerMuted, isTrue);
  });

  test('startGroupVoice connects LiveKit without outgoing overlay phase', () async {
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
  });
}
