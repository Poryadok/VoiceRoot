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
import 'matchmaking_search_controller.dart';

class PendingMatchState {
  const PendingMatchState({this.match, this.deadlineClock});

  final MatchData? match;
  final MatchDeadlineClock? deadlineClock;
}

class MatchmakingMatchController extends Notifier<PendingMatchState> {
  ProviderSubscription<AsyncValue<RealtimeFrame>>? _sub;
  final Map<String, Future<void>> _inFlightLoads = {};
  int _loadGeneration = 0;
  String? _latestLoadMatchId;
  bool _disposed = false;

  @override
  PendingMatchState build() {
    _sub?.close();
    _sub = ref.listen<AsyncValue<RealtimeFrame>>(
      realtimeEventProvider,
      (_, next) => next.whenData(_onFrame),
    );
    ref.onDispose(() {
      _disposed = true;
      _sub?.close();
    });
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
    ref
        .read(matchmakingRatingControllerProvider.notifier)
        .showRatingForMatch(result.data);
  }

  Future<void> _loadAndShow(String matchId) async {
    final inFlight = _inFlightLoads[matchId];
    if (inFlight != null) return inFlight;
    final token = ref.read(authControllerProvider).session?.accessToken;
    if (token == null || token.isEmpty) return;
    final authSession = ref.read(authControllerProvider).session;
    final generation = ++_loadGeneration;
    _latestLoadMatchId = matchId;
    final client = ref.read(voiceMatchmakingClientProvider);
    late final Future<void> request;
    request = () async {
      final result = await client.getMatch(
        authorization: 'Bearer $token',
        matchId: matchId,
      );
      if (_disposed ||
          generation != _loadGeneration ||
          _latestLoadMatchId != matchId ||
          !identical(ref.read(authControllerProvider).session, authSession)) {
        return;
      }
      if (result is! MatchmakingApiOk<MatchData>) return;
      _applyLoadedMatch(result.data);
    }();
    _inFlightLoads[matchId] = request;
    try {
      await request;
    } finally {
      if (identical(_inFlightLoads[matchId], request)) {
        _inFlightLoads.remove(matchId);
      }
    }
  }

  Future<bool> refreshMatch(String matchId) async {
    if (state.match?.id != matchId) return false;
    await _loadAndShow(matchId);
    return state.match?.id == matchId;
  }

  void _applyLoadedMatch(MatchData match) {
    final currentId = state.match?.id;
    if (match.status == 'pending_accept') {
      state = PendingMatchState(match: match, deadlineClock: _clockFor(match));
      return;
    }
    if (currentId != null && currentId != match.id) return;
    if (match.status == 'active' && match.ownProposalResponse == 'accepted') {
      if (match.chatId != null && match.voiceRoomId != null) {
        clear();
        ref.read(activeSearchSessionProvider.notifier).state = null;
        ref.read(activeSquadMatchProvider.notifier).state = match;
      } else {
        // Keep the accepted caller in the loaded pending-activation state; do
        // not manufacture a successful squad navigation without both receipts.
        state = PendingMatchState(
          match: match,
          deadlineClock: _clockFor(match),
        );
      }
      return;
    }
    if (currentId == match.id) clear();
    final searchSession = match.ownSearchSession;
    if (searchSession != null) {
      ref.read(activeSearchSessionProvider.notifier).state = searchSession;
    }
    if (match.ownProposalResponse == 'declined') {
      ref
          .read(matchmakingSearchControllerProvider.notifier)
          .showDeclinedRecovery();
    }
  }

  MatchDeadlineClock? _clockFor(MatchData match) {
    final serverNow = match.serverNow;
    final deadline = match.acceptanceDeadlineAt;
    if (serverNow == null || deadline == null) return null;
    return MatchDeadlineClock(serverNow: serverNow, deadline: deadline);
  }

  void clear() {
    _loadGeneration++;
    _latestLoadMatchId = null;
    state = const PendingMatchState();
  }

  Future<RespondToMatchData?> respond(bool accept) async {
    final match = state.match;
    if (match == null) return null;
    final authSession = ref.read(authControllerProvider).session;
    final token = authSession?.accessToken;
    if (token == null || token.isEmpty) return null;
    final matchId = match.id;
    await _loadAndShow(matchId);
    if (_disposed ||
        !identical(ref.read(authControllerProvider).session, authSession) ||
        state.match?.id != matchId ||
        state.match?.status != 'pending_accept') {
      return null;
    }
    final generation = ++_loadGeneration;
    final client = ref.read(voiceMatchmakingClientProvider);
    final result = await client.respondToMatch(
      authorization: 'Bearer $token',
      matchId: matchId,
      accept: accept,
    );
    if (_disposed ||
        generation != _loadGeneration ||
        state.match?.id != matchId ||
        !identical(ref.read(authControllerProvider).session, authSession)) {
      return null;
    }
    if (result is! MatchmakingApiOk<RespondToMatchData>) return null;
    final data = result.data;
    if (!accept) {
      clear();
      ref.read(activeSearchSessionProvider.notifier).state = data.searchSession;
      ref
          .read(matchmakingSearchControllerProvider.notifier)
          .showDeclinedRecovery();
      return data;
    }
    if (data.match.status == 'active' &&
        data.match.chatId != null &&
        data.match.voiceRoomId != null) {
      clear();
      ref.read(activeSearchSessionProvider.notifier).state = null;
      ref.read(activeSquadMatchProvider.notifier).state = data.match;
    } else {
      state = PendingMatchState(
        match: data.match,
        deadlineClock: _clockFor(data.match),
      );
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
      deadlineClock: ref
          .watch(matchmakingMatchControllerProvider)
          .deadlineClock,
      onRefresh: () => ref
          .read(matchmakingMatchControllerProvider.notifier)
          .refreshMatch(pending.id),
      onRespond: (accept) =>
          ref.read(matchmakingMatchControllerProvider.notifier).respond(accept),
    );
  }
}
