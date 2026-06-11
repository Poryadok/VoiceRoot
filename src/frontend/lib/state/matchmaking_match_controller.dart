import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../backend/matchmaking_client.dart';
import '../backend/realtime_client.dart';
import '../ui/matchmaking/match_found_overlay.dart';
import 'auth_providers.dart';
import 'chat_providers.dart';
import 'matchmaking_providers.dart';
import 'matchmaking_rating_controller.dart';

class PendingMatchState {
  const PendingMatchState({this.match});

  final MatchData? match;
}

class MatchmakingMatchController extends Notifier<PendingMatchState> {
  ProviderSubscription<AsyncValue<RealtimeFrame>>? _sub;

  @override
  PendingMatchState build() {
    _sub?.close();
    _sub = ref.listen<AsyncValue<RealtimeFrame>>(
      realtimeEventProvider,
      (_, next) => next.whenData(_onFrame),
    );
    ref.onDispose(() => _sub?.close());
    return const PendingMatchState();
  }

  void onPushNotificationData(Map<String, dynamic>? data) {
    if (data == null) return;
    if (data['type'] != 'match_found') return;
    final matchId = data['match_id'] as String?;
    if (matchId == null || matchId.isEmpty) return;
    unawaited(_loadAndShow(matchId));
  }

  void _onFrame(RealtimeFrame frame) {
    if (frame.op == 'match_found') {
      final data = frame.data;
      final matchId = data?['match_id'] as String?;
      if (matchId != null && matchId.isNotEmpty) {
        unawaited(_loadAndShow(matchId));
      }
      return;
    }
    if (frame.op == 'match_completed') {
      final data = frame.data;
      final matchId = data?['match_id'] as String?;
      if (matchId != null && matchId.isNotEmpty) {
        unawaited(_loadCompletedForRating(matchId));
      }
      return;
    }
    if (frame.op == 'notification' && dataIsMatchFound(frame.data)) {
      onPushNotificationData(frame.data);
    }
  }

  bool dataIsMatchFound(Map<String, dynamic>? data) =>
      data != null && data['type'] == 'match_found';

  Future<void> _loadCompletedForRating(String matchId) async {
    final token = ref.read(authControllerProvider).session?.accessToken;
    if (token == null || token.isEmpty) return;
    final client = ref.read(voiceMatchmakingClientProvider);
    final result = await client.getMatch(
      authorization: 'Bearer $token',
      matchId: matchId,
    );
    if (result is! MatchmakingApiOk<MatchData>) return;
    if (result.data.status != 'completed') return;
    ref.read(matchmakingRatingControllerProvider.notifier).showRatingForMatch(
          result.data,
        );
  }

  Future<void> _loadAndShow(String matchId) async {
    final token = ref.read(authControllerProvider).session?.accessToken;
    if (token == null || token.isEmpty) return;
    final client = ref.read(voiceMatchmakingClientProvider);
    final result = await client.getMatch(
      authorization: 'Bearer $token',
      matchId: matchId,
    );
    if (result is! MatchmakingApiOk<MatchData>) return;
    if (result.data.status != 'pending_accept') return;
    state = PendingMatchState(match: result.data);
  }

  void clear() => state = const PendingMatchState();

  Future<RespondToMatchData?> respond(bool accept) async {
    final match = state.match;
    if (match == null) return null;
    final token = ref.read(authControllerProvider).session?.accessToken;
    if (token == null || token.isEmpty) return null;
    final client = ref.read(voiceMatchmakingClientProvider);
    final result = await client.respondToMatch(
      authorization: 'Bearer $token',
      matchId: match.id,
      accept: accept,
    );
    if (result is! MatchmakingApiOk<RespondToMatchData>) return null;
    final data = result.data;
    if (!accept) {
      clear();
      ref.read(activeSearchSessionProvider.notifier).state = data.searchSession;
      return data;
    }
    if (data.match.status == 'active' &&
        data.match.chatId != null &&
        data.match.voiceRoomId != null) {
      clear();
      ref.read(activeSearchSessionProvider.notifier).state = null;
      ref.read(activeSquadMatchProvider.notifier).state = data.match;
    } else {
      state = PendingMatchState(match: data.match);
    }
    return data;
  }
}

final matchmakingMatchControllerProvider =
    NotifierProvider<MatchmakingMatchController, PendingMatchState>(
  MatchmakingMatchController.new,
);

/// Global overlay host for pending match accept/decline.
class MatchmakingMatchOverlayHost extends ConsumerWidget {
  const MatchmakingMatchOverlayHost({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final pending = ref.watch(matchmakingMatchControllerProvider).match;
    if (pending == null) return const SizedBox.shrink();
    return MatchFoundOverlay(
      match: pending,
      onRespond: (accept) =>
          ref.read(matchmakingMatchControllerProvider.notifier).respond(accept),
    );
  }
}
