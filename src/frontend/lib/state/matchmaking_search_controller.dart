import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../backend/realtime_client.dart';
import 'chat_providers.dart';
import 'matchmaking_providers.dart';

class MatchmakingSearchState {
  const MatchmakingSearchState({
    this.nudgeVisible = false,
    this.timedOut = false,
  });

  final bool nudgeVisible;
  final bool timedOut;

  MatchmakingSearchState copyWith({bool? nudgeVisible, bool? timedOut}) {
    return MatchmakingSearchState(
      nudgeVisible: nudgeVisible ?? this.nudgeVisible,
      timedOut: timedOut ?? this.timedOut,
    );
  }
}

class MatchmakingSearchController extends Notifier<MatchmakingSearchState> {
  ProviderSubscription<AsyncValue<RealtimeFrame>>? _sub;

  @override
  MatchmakingSearchState build() {
    _sub?.close();
    _sub = ref.listen<AsyncValue<RealtimeFrame>>(
      realtimeEventProvider,
      (_, next) => next.whenData(_onFrame),
    );
    ref.onDispose(() => _sub?.close());
    return const MatchmakingSearchState();
  }

  void onPushNotificationData(Map<String, dynamic>? data) {
    if (data == null) return;
    _handleSearchEvent(data['type'] as String?, data);
  }

  void dismissNudge() {
    state = state.copyWith(nudgeVisible: false);
  }

  void clearTimedOut() {
    state = state.copyWith(timedOut: false);
  }

  void _onFrame(RealtimeFrame frame) {
    if (frame.op == 'search_nudge' || frame.op == 'search_timeout') {
      _handleSearchEvent(frame.op, frame.data);
      return;
    }
    if (frame.op == 'notification') {
      final type = frame.data?['type'] as String?;
      if (type == 'search_nudge' || type == 'search_timeout') {
        onPushNotificationData(frame.data);
      }
    }
  }

  void _handleSearchEvent(String? type, Map<String, dynamic>? data) {
    if (type == 'search_nudge') {
      state = state.copyWith(nudgeVisible: true, timedOut: false);
      return;
    }
    if (type == 'search_timeout') {
      ref.read(activeSearchSessionProvider.notifier).state = null;
      state = state.copyWith(nudgeVisible: false, timedOut: true);
    }
  }
}

final matchmakingSearchControllerProvider =
    NotifierProvider<MatchmakingSearchController, MatchmakingSearchState>(
  MatchmakingSearchController.new,
);
