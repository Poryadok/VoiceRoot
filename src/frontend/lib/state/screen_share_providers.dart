import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../backend/realtime_client.dart';
import 'auth_providers.dart';
import 'call_providers.dart';

class ActiveScreenShare {
  const ActiveScreenShare({
    required this.roomId,
    required this.profileId,
    required this.streamId,
  });

  final String roomId;
  final String profileId;
  final String streamId;
}

class ScreenShareUiState {
  const ScreenShareUiState({
    this.streams = const [],
    this.selectedProfileId,
    this.localStreamId,
    this.isSharing = false,
    this.isPaused = false,
    this.errorMessage,
  });

  final List<ActiveScreenShare> streams;
  final String? selectedProfileId;
  final String? localStreamId;
  final bool isSharing;
  final bool isPaused;
  final String? errorMessage;

  ActiveScreenShare? get selectedStream {
    if (selectedProfileId == null) return null;
    for (final stream in streams) {
      if (stream.profileId == selectedProfileId) return stream;
    }
    return streams.isNotEmpty ? streams.first : null;
  }

  ScreenShareUiState copyWith({
    List<ActiveScreenShare>? streams,
    String? selectedProfileId,
    bool clearSelected = false,
    String? localStreamId,
    bool clearLocalStreamId = false,
    bool? isSharing,
    bool? isPaused,
    String? errorMessage,
    bool clearError = false,
  }) {
    return ScreenShareUiState(
      streams: streams ?? this.streams,
      selectedProfileId:
          clearSelected ? null : (selectedProfileId ?? this.selectedProfileId),
      localStreamId: clearLocalStreamId
          ? null
          : (localStreamId ?? this.localStreamId),
      isSharing: isSharing ?? this.isSharing,
      isPaused: isPaused ?? this.isPaused,
      errorMessage: clearError ? null : (errorMessage ?? this.errorMessage),
    );
  }
}

class ScreenShareController extends StateNotifier<ScreenShareUiState> {
  ScreenShareController(this._ref) : super(const ScreenShareUiState()) {
    _sub = _ref.read(callSignalingStreamProvider).listen(_onFrame);
  }

  final Ref _ref;
  StreamSubscription<RealtimeFrame>? _sub;

  @override
  void dispose() {
    _sub?.cancel();
    super.dispose();
  }

  void _onFrame(RealtimeFrame frame) {
    final data = frame.data;
    if (data == null) return;
    switch (frame.op) {
      case 'screen_share_started':
        final roomId = data['room_id'] as String? ?? '';
        final profileId = data['profile_id'] as String? ?? '';
        final streamId = data['stream_id'] as String? ?? '';
        if (roomId.isEmpty || profileId.isEmpty) return;
        final next = [
          ...state.streams.where((s) => s.profileId != profileId),
          ActiveScreenShare(
            roomId: roomId,
            profileId: profileId,
            streamId: streamId,
          ),
        ];
        final selfId = _ref.read(authControllerProvider).activeProfileId;
        final isRemote = selfId != null && profileId != selfId;
        String? selected = state.selectedProfileId;
        if (selected == null) {
          selected = profileId;
        } else if (isRemote &&
            (selected == selfId || !state.streams.any((s) => s.profileId == selected))) {
          selected = profileId;
        }
        state = state.copyWith(
          streams: next,
          selectedProfileId: selected,
        );
      case 'screen_share_stopped':
        final profileId = data['profile_id'] as String? ?? '';
        if (profileId.isEmpty) return;
        final next = state.streams
            .where((s) => s.profileId != profileId)
            .toList(growable: false);
        state = state.copyWith(
          streams: next,
          clearSelected: state.selectedProfileId == profileId,
          selectedProfileId: next.isEmpty
              ? null
              : (state.selectedProfileId == profileId
                    ? next.first.profileId
                    : state.selectedProfileId),
        );
    }
  }

  void selectStream(String profileId) {
    state = state.copyWith(selectedProfileId: profileId);
  }

  void setLocalSharing({
    required bool isSharing,
    String? streamId,
    bool isPaused = false,
  }) {
    state = state.copyWith(
      isSharing: isSharing,
      localStreamId: streamId,
      clearLocalStreamId: !isSharing,
      isPaused: isPaused,
      clearError: true,
    );
  }

  void setError(String message) {
    state = state.copyWith(errorMessage: message);
  }

  void clearForRoomEnd() {
    state = const ScreenShareUiState();
  }
}

final screenShareControllerProvider =
    StateNotifierProvider<ScreenShareController, ScreenShareUiState>((ref) {
      return ScreenShareController(ref);
    });
