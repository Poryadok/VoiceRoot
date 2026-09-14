import 'package:flutter_riverpod/flutter_riverpod.dart';

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
      selectedProfileId: clearSelected
          ? null
          : (selectedProfileId ?? this.selectedProfileId),
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
  ScreenShareController(Ref _) : super(const ScreenShareUiState());

  void selectStream(String profileId) {
    state = state.copyWith(selectedProfileId: profileId);
  }

  /// Replaces event-derived screen state with the current Voice snapshot, then
  /// admits only tracks currently present in LiveKit. REST decides who is still
  /// sharing; LiveKit decides whether media is presently renderable.
  void reconcileSnapshot({
    required String roomId,
    required Set<String> sharingProfileIds,
    required String? localProfileId,
    required bool hasLocalTrack,
    required bool Function(String profileId) hasRemoteTrack,
  }) {
    final previousByProfile = {
      for (final stream in state.streams) stream.profileId: stream,
    };
    final next = <ActiveScreenShare>[];
    for (final profileId in sharingProfileIds) {
      final isLocal = profileId == localProfileId;
      if (isLocal ? !hasLocalTrack : !hasRemoteTrack(profileId)) continue;
      final previous = previousByProfile[profileId];
      next.add(
        ActiveScreenShare(
          roomId: roomId,
          profileId: profileId,
          // GetVoiceStates does not expose the stream id. Retain a live event
          // id when available; profile id remains a stable UI key otherwise.
          streamId: previous?.streamId ?? profileId,
        ),
      );
    }
    final selected =
        next.any((stream) => stream.profileId == state.selectedProfileId)
        ? state.selectedProfileId
        : (next.isEmpty ? null : next.first.profileId);
    final localStillSharing =
        localProfileId != null &&
        sharingProfileIds.contains(localProfileId) &&
        hasLocalTrack;
    state = state.copyWith(
      streams: next,
      selectedProfileId: selected,
      clearSelected: selected == null,
      isSharing: localStillSharing,
      clearLocalStreamId: !localStillSharing,
      isPaused: localStillSharing ? state.isPaused : false,
    );
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
