import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../backend/livekit_room.dart';
import '../backend/livekit_url.dart';
import '../backend/realtime_client.dart';
import '../backend/voice_client.dart';
import 'auth_providers.dart';
import 'chat_providers.dart';
import 'gateway_providers.dart';

final voiceCallsClientProvider = Provider<VoiceCallsClient>((ref) {
  return VoiceCallsClient(gateway: ref.watch(gatewayHttpClientProvider));
});

final liveKitRoomFactoryProvider = Provider<VoiceLiveKitRoom Function()>((ref) {
  return () => LiveKitVoiceRoom();
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
    this.errorMessage,
  });

  final CallPhase phase;
  final VoiceCallSession? session;
  final String? outgoingChatId;
  final String? outgoingCalleeProfileId;
  final bool isMuted;
  final bool isSpeakerMuted;
  final bool isVideoEnabled;
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
      errorMessage: clearError ? null : (errorMessage ?? this.errorMessage),
    );
  }
}

class CallController extends StateNotifier<CallState> {
  CallController(this._ref) : super(const CallState()) {
    _events = _ref.listen<AsyncValue<RealtimeFrame>>(realtimeEventProvider, (
      _,
      next,
    ) {
      next.whenData(_onRealtimeFrame);
    });
  }

  final Ref _ref;
  ProviderSubscription<AsyncValue<RealtimeFrame>>? _events;
  VoiceLiveKitRoom? _room;
  bool _startCallInFlight = false;
  bool _connectLiveKitInFlight = false;

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
    if (mounted) state = const CallState();
  }

  Future<void> setMuted(bool muted) async {
    final auth = _ref.read(authorizationHeaderProvider);
    final current = state.session;
    state = state.copyWith(isMuted: muted);
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
    if (_connectLiveKitInFlight || state.phase == CallPhase.active) {
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
    _connectLiveKitInFlight = true;
    try {
      final token = await _ref
          .read(voiceCallsClientProvider)
          .getJoinToken(authorization: auth, roomId: session.roomId);
      if (!mounted) return;
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
            );
            return;
          }
          final room = _ref.read(liveKitRoomFactoryProvider)();
          _room = room;
          try {
            await room.connect(
              url: livekitUrl,
              token: data.jwt,
              video: session.mediaKind == VoiceCallMediaKind.video,
            );
            if (mounted) {
              state = state.copyWith(
                phase: CallPhase.active,
                isVideoEnabled: session.mediaKind == VoiceCallMediaKind.video,
                clearOutgoingTarget: true,
              );
            }
          } catch (_) {
            await _failLiveKitConnect(
              auth: auth,
              session: session,
              errorMessage: 'livekit_connect_failed',
            );
          }
        case VoiceApiFailure(:final message):
          state = state.copyWith(phase: CallPhase.failed, errorMessage: message);
      }
    } finally {
      _connectLiveKitInFlight = false;
    }
  }

  Future<void> _failLiveKitConnect({
    required String auth,
    required VoiceCallSession session,
    required String errorMessage,
  }) async {
    await _room?.disconnect();
    _room = null;
    await _ref
        .read(voiceCallsClientProvider)
        .endCall(authorization: auth, roomId: session.roomId);
    if (mounted) {
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
          state = CallState(
            phase: CallPhase.incoming,
            session: session,
            isVideoEnabled: session.mediaKind == VoiceCallMediaKind.video,
          );
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
        if (current != null && frame.data?['room_id'] == current.roomId) {
          unawaited(_room?.disconnect());
          _room = null;
          state = const CallState();
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
      expiresAt: DateTime.tryParse(data['expires_at'] as String? ?? ''),
    );
  }

  @override
  void dispose() {
    _events?.close();
    unawaited(_room?.disconnect());
    super.dispose();
  }
}

final callControllerProvider = StateNotifierProvider<CallController, CallState>(
  (ref) {
    return CallController(ref);
  },
);
