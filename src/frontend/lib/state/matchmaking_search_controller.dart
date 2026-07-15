import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../backend/realtime_client.dart';
import 'chat_providers.dart';
import 'matchmaking_providers.dart';

enum SearchRecoveryReason { timeout, declined }

class MatchmakingSearchState {
  const MatchmakingSearchState({
    this.nudgeVisible = false,
    this.recoveryReason,
  });

  final bool nudgeVisible;
  final SearchRecoveryReason? recoveryReason;

  bool get timedOut => recoveryReason == SearchRecoveryReason.timeout;

  MatchmakingSearchState copyWith({
    bool? nudgeVisible,
    SearchRecoveryReason? recoveryReason,
    bool clearRecoveryReason = false,
  }) {
    return MatchmakingSearchState(
      nudgeVisible: nudgeVisible ?? this.nudgeVisible,
      recoveryReason:
          clearRecoveryReason ? null : (recoveryReason ?? this.recoveryReason),
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

  void clearRecovery() {
    state = state.copyWith(clearRecoveryReason: true);
  }

  void clearTimedOut() => clearRecovery();

  void showDeclinedRecovery() {
    state = state.copyWith(
      nudgeVisible: false,
      recoveryReason: SearchRecoveryReason.declined,
    );
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
      state = state.copyWith(
        nudgeVisible: true,
        clearRecoveryReason: true,
      );
      return;
    }
    if (type == 'search_timeout') {
      ref.read(activeSearchSessionProvider.notifier).state = null;
      state = state.copyWith(
        nudgeVisible: false,
        recoveryReason: SearchRecoveryReason.timeout,
      );
    }
  }
}

final matchmakingSearchControllerProvider =
    NotifierProvider<MatchmakingSearchController, MatchmakingSearchState>(
  MatchmakingSearchController.new,
);
