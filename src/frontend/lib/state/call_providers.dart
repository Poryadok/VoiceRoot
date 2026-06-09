import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../backend/livekit_room.dart';
import '../backend/livekit_url.dart';
import '../backend/realtime_client.dart';
import '../backend/voice_client.dart';
import 'auth_providers.dart';
import 'chat_providers.dart';
import 'gateway_providers.dart';

/// Call signaling must use the raw hub stream — [realtimeEventProvider] keeps only
/// the latest frame and can drop `call_incoming` between heartbeats.
final callSignalingStreamProvider = Provider<Stream<RealtimeFrame>>((ref) {
  return ref.watch(realtimeHubProvider).events;
});

final voiceCallsClientProvider = Provider<VoiceCallsClient>((ref) {
  return VoiceCallsClient(gateway: ref.watch(gatewayHttpClientProvider));
});

final liveKitRoomFactoryProvider = Provider<VoiceLiveKitRoom Function()>((ref) {
  return () => LiveKitVoiceRoom();
});

/// Bumped when group voice sessions start/end so [groupActiveCallProvider] refreshes.
final groupActiveCallRefreshTickProvider = StateProvider<int>((ref) => 0);

final groupActiveCallProvider = FutureProvider.family<VoiceCallSession?, String>((
  ref,
  chatId,
) async {
  ref.watch(groupActiveCallRefreshTickProvider);
  final auth = ref.watch(authorizationHeaderProvider);
  if (auth == null) return null;
  final result = await ref
      .read(voiceCallsClientProvider)
      .getActiveGroupCallForChat(authorization: auth, groupChatId: chatId);
  return switch (result) {
    VoiceApiOk(:final data) => data,
    VoiceApiFailure() => null,
  };
});

enum CallPhase { idle, outgoing, incoming, connecting, active, ended, failed }

class CallState {
  const CallState({
    this.phase = CallPhase.idle,
    this.session,
    this.outgoingChatId,
    this.outgoingCalleeProfileId,
    this.isMuted = false,
    this.isSpeakerMuted = false,
    this.isVideoEnabled = false,
    this.needsAudioPlaybackUnlock = false,
    this.errorMessage,
  });

  final CallPhase phase;
  final VoiceCallSession? session;
  final String? outgoingChatId;
  final String? outgoingCalleeProfileId;
  final bool isMuted;
  final bool isSpeakerMuted;
  final bool isVideoEnabled;
  final bool needsAudioPlaybackUnlock;
  final String? errorMessage;

  bool get hasCall => session != null && phase != CallPhase.idle;
  bool get isIncoming => phase == CallPhase.incoming;
  bool get isOutgoing => phase == CallPhase.outgoing;
  bool get isActive =>
      phase == CallPhase.active || phase == CallPhase.connecting;

  CallState copyWith({
    CallPhase? phase,
    VoiceCallSession? session,
    bool clearSession = false,
    String? outgoingChatId,
    String? outgoingCalleeProfileId,
    bool clearOutgoingTarget = false,
    bool? isMuted,
    bool? isSpeakerMuted,
    bool? isVideoEnabled,
    bool? needsAudioPlaybackUnlock,
    String? errorMessage,
    bool clearError = false,
  }) {
    return CallState(
      phase: phase ?? this.phase,
      session: clearSession ? null : (session ?? this.session),
      outgoingChatId: clearOutgoingTarget
          ? null
          : (outgoingChatId ?? this.outgoingChatId),
      outgoingCalleeProfileId: clearOutgoingTarget
          ? null
          : (outgoingCalleeProfileId ?? this.outgoingCalleeProfileId),
      isMuted: isMuted ?? this.isMuted,
      isSpeakerMuted: isSpeakerMuted ?? this.isSpeakerMuted,
      isVideoEnabled: isVideoEnabled ?? this.isVideoEnabled,
      needsAudioPlaybackUnlock:
          needsAudioPlaybackUnlock ?? this.needsAudioPlaybackUnlock,
      errorMessage: clearError ? null : (errorMessage ?? this.errorMessage),
    );
  }
}

class CallController extends StateNotifier<CallState> {
  CallController(this._ref) : super(const CallState()) {
    _eventsSub = _ref.read(callSignalingStreamProvider).listen(_onRealtimeFrame);
    _linkSub = _ref.listen<RealtimeLinkStatus>(
      realtimeLinkStatusProvider,
      (prev, next) {
        if (next == RealtimeLinkStatus.connected) {
          unawaited(_syncActiveCallIfIdle());
        }
      },
      fireImmediately: true,
    );
  }

  final Ref _ref;
  StreamSubscription<RealtimeFrame>? _eventsSub;
  ProviderSubscription<RealtimeLinkStatus>? _linkSub;
  VoiceLiveKitRoom? _room;
  bool _startCallInFlight = false;
  bool _groupVoiceInFlight = false;
  int _connectGeneration = 0;

  void _refreshGroupActiveCalls() {
    _ref.read(groupActiveCallRefreshTickProvider.notifier).state++;
  }

  Future<void> startCall({
    required String chatId,
    required String calleeProfileId,
    VoiceCallMediaKind mediaKind = VoiceCallMediaKind.audio,
  }) async {
    final auth = _ref.read(authorizationHeaderProvider);
    if (auth == null) return;
    if (_startCallInFlight) return;
    final current = state.session;
    if (state.phase == CallPhase.outgoing &&
        state.outgoingChatId == chatId &&
        state.outgoingCalleeProfileId == calleeProfileId &&
        (current == null ||
            (current.calleeProfileId == calleeProfileId &&
                current.chatId == chatId))) {
      return;
    }
    _startCallInFlight = true;
    state = state.copyWith(
      phase: CallPhase.outgoing,
      outgoingChatId: chatId,
      outgoingCalleeProfileId: calleeProfileId,
      clearError: true,
    );
    try {
      final result = await _ref
          .read(voiceCallsClientProvider)
          .startCall(
            authorization: auth,
            chatId: chatId,
            calleeProfileId: calleeProfileId,
            mediaKind: mediaKind,
          );
      if (!mounted) return;
      switch (result) {
        case VoiceApiOk(:final data):
          _applySession(data, CallPhase.outgoing);
        case VoiceApiFailure(:final message, :final statusCode):
          if (statusCode == 412 && await _tryRecoverActiveCall(auth)) {
            return;
          }
          state = state.copyWith(
            phase: CallPhase.failed,
            errorMessage: message,
            clearOutgoingTarget: true,
          );
      }
    } finally {
      _startCallInFlight = false;
    }
  }

  void dismissFailure() {
    if (state.phase == CallPhase.failed) {
      state = const CallState();
    }
  }

  Future<void> startGroupVoice({
    required String groupChatId,
    VoiceCallMediaKind mediaKind = VoiceCallMediaKind.audio,
  }) async {
    final auth = _ref.read(authorizationHeaderProvider);
    if (auth == null) return;
    if (_groupVoiceInFlight) return;
    _groupVoiceInFlight = true;
    state = state.copyWith(
      phase: CallPhase.connecting,
      outgoingChatId: groupChatId,
      clearError: true,
    );
    try {
      final result = await _ref
          .read(voiceCallsClientProvider)
          .startGroupVoice(
            authorization: auth,
            groupChatId: groupChatId,
            mediaKind: mediaKind,
          );
      if (!mounted) return;
      switch (result) {
        case VoiceApiOk(:final data):
          state = state.copyWith(session: data, clearOutgoingTarget: true);
          _refreshGroupActiveCalls();
          await _connectLiveKit(data);
        case VoiceApiFailure(:final message, :final statusCode):
          if (statusCode == 412 && await _tryRecoverActiveCall(auth)) {
            return;
          }
          state = state.copyWith(
            phase: CallPhase.failed,
            errorMessage: message,
            clearOutgoingTarget: true,
          );
      }
    } finally {
      _groupVoiceInFlight = false;
    }
  }

  Future<void> joinGroupVoice({required String roomId}) async {
    final auth = _ref.read(authorizationHeaderProvider);
    if (auth == null) return;
    if (_groupVoiceInFlight) return;
    _groupVoiceInFlight = true;
    state = state.copyWith(phase: CallPhase.connecting, clearError: true);
    try {
      final result = await _ref
          .read(voiceCallsClientProvider)
          .joinCall(authorization: auth, roomId: roomId);
      if (!mounted) return;
      switch (result) {
        case VoiceApiOk(:final data):
          state = state.copyWith(session: data, clearOutgoingTarget: true);
          _refreshGroupActiveCalls();
          await _connectLiveKit(data);
        case VoiceApiFailure(:final message, :final statusCode):
          if (statusCode == 412 && await _tryRecoverActiveCall(auth)) {
            return;
          }
          state = state.copyWith(
            phase: CallPhase.failed,
            errorMessage: message,
            clearOutgoingTarget: true,
          );
      }
    } finally {
      _groupVoiceInFlight = false;
    }
  }

  Future<void> acceptCall() async {
    final auth = _ref.read(authorizationHeaderProvider);
    final current = state.session;
    if (auth == null || current == null) return;
    state = state.copyWith(phase: CallPhase.connecting, clearError: true);
    final accepted = await _ref
        .read(voiceCallsClientProvider)
        .acceptCall(authorization: auth, roomId: current.roomId);
    if (!mounted) return;
    switch (accepted) {
      case VoiceApiOk(:final data):
        state = state.copyWith(session: data);
        await _connectLiveKit(data);
      case VoiceApiFailure(:final message):
        state = state.copyWith(phase: CallPhase.failed, errorMessage: message);
    }
  }

  Future<void> declineCall() async {
    final auth = _ref.read(authorizationHeaderProvider);
    final current = state.session;
    if (auth == null || current == null) {
      state = const CallState();
      return;
    }
    await _ref
        .read(voiceCallsClientProvider)
        .declineCall(authorization: auth, roomId: current.roomId);
    state = const CallState();
  }

  Future<void> hangUp() async {
    final auth = _ref.read(authorizationHeaderProvider);
    final current = state.session;
    await _room?.disconnect();
    _room = null;
    if (auth != null && current != null) {
      await _ref
          .read(voiceCallsClientProvider)
          .endCall(authorization: auth, roomId: current.roomId);
    }
    if (mounted) {
      state = const CallState();
      _refreshGroupActiveCalls();
    }
  }

  Future<void> unlockAudioPlayback() async {
    await _room?.ensureAudioPlayback();
  }

  Future<void> setMuted(bool muted) async {
    final auth = _ref.read(authorizationHeaderProvider);
    final current = state.session;
    state = state.copyWith(isMuted: muted);
    await _room?.ensureAudioPlayback();
    await _room?.setMuted(muted);
    if (auth != null && current != null) {
      await _ref
          .read(voiceCallsClientProvider)
          .updateVoiceState(
            authorization: auth,
            roomId: current.roomId,
            isMuted: muted,
          );
    }
  }

  Future<void> setSpeakerMuted(bool muted) async {
    state = state.copyWith(isSpeakerMuted: muted);
    await _room?.ensureAudioPlayback();
    await _room?.setSpeakerMuted(muted);
  }

  Future<void> setVideoEnabled(bool enabled) async {
    final auth = _ref.read(authorizationHeaderProvider);
    final current = state.session;
    state = state.copyWith(isVideoEnabled: enabled);
    await _room?.setVideoEnabled(enabled);
    if (auth != null && current != null) {
      await _ref
          .read(voiceCallsClientProvider)
          .updateVoiceState(
            authorization: auth,
            roomId: current.roomId,
            isVideoOn: enabled,
          );
    }
  }

  Future<void> _connectLiveKit(VoiceCallSession session) async {
    if (state.phase == CallPhase.active &&
        state.session?.roomId == session.roomId) {
      return;
    }
    final auth = _ref.read(authorizationHeaderProvider);
    if (auth == null) {
      state = state.copyWith(
        phase: CallPhase.failed,
        errorMessage: 'not_authenticated',
      );
      return;
    }
    final connectRoomId = session.roomId;
    final connectGeneration = ++_connectGeneration;
    await _room?.disconnect();
    _room = null;
    final token = await _ref
        .read(voiceCallsClientProvider)
        .getJoinToken(authorization: auth, roomId: connectRoomId);
    if (!mounted || !_isConnectCurrent(connectGeneration, connectRoomId)) {
      return;
    }
    switch (token) {
      case VoiceApiOk(:final data):
        final livekitUrl = resolveLivekitConnectUrl(
          apiUrl: data.livekitUrl,
          clientFallback:
              _ref.read(gatewayConfigProvider).effectiveLivekitFallback,
        );
        if (livekitUrl.isEmpty) {
          await _failLiveKitConnect(
            auth: auth,
            session: session,
            errorMessage: 'livekit_url_missing',
            generation: connectGeneration,
          );
          return;
        }
        final room = _ref.read(liveKitRoomFactoryProvider)();
        room.onAudioPlaybackUnlockNeeded = (needsUnlock) {
          if (!mounted) return;
          state = state.copyWith(needsAudioPlaybackUnlock: needsUnlock);
        };
        _room = room;
        try {
          await room.connect(
            url: livekitUrl,
            token: data.jwt,
            video: session.mediaKind == VoiceCallMediaKind.video,
          );
          if (!mounted || !_isConnectCurrent(connectGeneration, connectRoomId)) {
            return;
          }
          state = state.copyWith(
            phase: CallPhase.active,
            isVideoEnabled: session.mediaKind == VoiceCallMediaKind.video,
            clearOutgoingTarget: true,
          );
        } on Object {
          if (!mounted || !_isConnectCurrent(connectGeneration, connectRoomId)) {
            return;
          }
          await _failLiveKitConnect(
            auth: auth,
            session: session,
            errorMessage: 'livekit_connect_failed',
            generation: connectGeneration,
          );
        }
      case VoiceApiFailure(:final message):
        if (!mounted || !_isConnectCurrent(connectGeneration, connectRoomId)) {
          return;
        }
        state = state.copyWith(phase: CallPhase.failed, errorMessage: message);
    }
  }

  bool _isConnectCurrent(int generation, String roomId) {
    return _connectGeneration == generation &&
        state.session?.roomId == roomId;
  }

  Future<void> _failLiveKitConnect({
    required String auth,
    required VoiceCallSession session,
    required String errorMessage,
    required int generation,
  }) async {
    if (!_isConnectCurrent(generation, session.roomId)) return;
    try {
      await _room?.disconnect();
    } on Object catch (_) {
      // Room may be half-connected; ignore disconnect errors.
    }
    _room = null;
    await _ref
        .read(voiceCallsClientProvider)
        .endCall(authorization: auth, roomId: session.roomId);
    if (mounted && _isConnectCurrent(generation, session.roomId)) {
      state = state.copyWith(
        phase: CallPhase.failed,
        errorMessage: errorMessage,
        clearOutgoingTarget: true,
      );
    }
  }

  void _onRealtimeFrame(RealtimeFrame frame) {
    switch (frame.op) {
      case 'call_incoming':
        final session = _sessionFromFrame(frame, VoiceCallStatus.ringing);
        if (session != null) {
          final current = state.session;
          final sameRoomInProgress =
              current?.roomId == session.roomId &&
              (state.phase == CallPhase.outgoing ||
                  state.phase == CallPhase.connecting ||
                  state.phase == CallPhase.active);
          if (!sameRoomInProgress) {
            state = CallState(
              phase: CallPhase.incoming,
              session: session,
              isVideoEnabled: session.mediaKind == VoiceCallMediaKind.video,
            );
          }
        }
      case 'call_accepted':
        final current = state.session;
        final activeProfileId = _ref.read(authControllerProvider).activeProfileId;
        if (current != null &&
            frame.data?['room_id'] == current.roomId &&
            activeProfileId == current.initiatorProfileId) {
          state = state.copyWith(phase: CallPhase.connecting);
          unawaited(_connectLiveKit(current));
        }
      case 'call_declined' || 'call_missed' || 'call_ended':
        final current = state.session;
        final endedRoomId = frame.data?['room_id'] as String?;
        if (current != null && endedRoomId == current.roomId) {
          unawaited(_room?.disconnect());
          _room = null;
          state = const CallState();
        }
        if (endedRoomId != null && endedRoomId.isNotEmpty) {
          _refreshGroupActiveCalls();
        }
    }
  }

  void _applySession(VoiceCallSession session, CallPhase phase) {
    state = CallState(
      phase: phase,
      session: session,
      isVideoEnabled: session.mediaKind == VoiceCallMediaKind.video,
    );
  }

  Future<void> _syncActiveCallIfIdle() async {
    if (state.phase != CallPhase.idle) return;
    final auth = _ref.read(authorizationHeaderProvider);
    if (auth == null) return;
    await _tryRecoverActiveCall(auth);
  }

  Future<bool> _tryRecoverActiveCall(String auth) async {
    final activeProfileId = _ref.read(authControllerProvider).activeProfileId;
    if (activeProfileId == null) return false;

    final result = await _ref
        .read(voiceCallsClientProvider)
        .getActiveCall(authorization: auth);
    if (!mounted) return false;
    switch (result) {
      case VoiceApiOk(:final data):
        final session = data;
        if (session == null) return false;
        switch (session.status) {
          case VoiceCallStatus.ringing:
            if (session.isGroupVoice) return false;
            if (session.initiatorProfileId == activeProfileId) {
              _applySession(session, CallPhase.outgoing);
              return true;
            }
            if (session.calleeProfileId == activeProfileId) {
              _applySession(session, CallPhase.incoming);
              return true;
            }
            return false;
          case VoiceCallStatus.active:
            _applySession(session, CallPhase.connecting);
            await _connectLiveKit(session);
            return mounted;
          default:
            return false;
        }
      case VoiceApiFailure():
        return false;
    }
  }

  VoiceCallSession? _sessionFromFrame(
    RealtimeFrame frame,
    VoiceCallStatus status,
  ) {
    final data = frame.data;
    if (data == null) return null;
    final roomId = data['room_id'] as String? ?? '';
    if (roomId.isEmpty) return null;
    final roomType = '${data['room_type']}'.toLowerCase();
    final roomTypeEnum = '${data['room_type_enum']}'.toUpperCase();
    final sessionKind = roomTypeEnum.contains('GROUP_VOICE') ||
            roomType == 'group_voice'
        ? VoiceSessionKind.groupVoice
        : roomTypeEnum.contains('VOICE_ROOM') || roomType == 'voice_room'
        ? VoiceSessionKind.voiceRoom
        : VoiceSessionKind.dm;
    return VoiceCallSession(
      roomId: roomId,
      livekitRoomName: data['livekit_room_name'] as String? ?? '',
      chatId: data['chat_id'] as String? ?? '',
      initiatorProfileId: data['initiator_profile_id'] as String? ?? '',
      calleeProfileId: data['callee_profile_id'] as String? ?? '',
      mediaKind: '${data['media_kind']}'.toLowerCase() == 'video'
          ? VoiceCallMediaKind.video
          : VoiceCallMediaKind.audio,
      status: status,
      sessionKind: sessionKind,
      expiresAt: DateTime.tryParse(data['expires_at'] as String? ?? ''),
    );
  }

  @override
  void dispose() {
    _eventsSub?.cancel();
    _linkSub?.close();
    unawaited(_room?.disconnect());
    super.dispose();
  }
}

final callControllerProvider = StateNotifierProvider<CallController, CallState>(
  (ref) {
    return CallController(ref);
  },
);
