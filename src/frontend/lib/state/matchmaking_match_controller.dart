import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../backend/matchmaking_client.dart';
import '../backend/realtime_client.dart';
import '../backend/auth_session.dart';
import '../ui/matchmaking/match_found_overlay.dart';
import 'auth_providers.dart';
import 'chat_providers.dart';
import 'matchmaking_providers.dart';
import 'matchmaking_rating_controller.dart';
import 'matchmaking_search_controller.dart';

class PendingMatchState {
  const PendingMatchState({
    this.match,
    this.deadlineClock,
    this.isResponding = false,
  });

  final MatchData? match;
  final MatchDeadlineClock? deadlineClock;
  final bool isResponding;
}

class MatchmakingMatchController extends Notifier<PendingMatchState> {
  ProviderSubscription<AsyncValue<RealtimeFrame>>? _sub;
  ProviderSubscription<AuthState>? _authSub;
  final Map<String, Future<MatchData?>> _inFlightLoads = {};
  int _contextGeneration = 0;
  int _authGeneration = 0;
  int _operationGeneration = 0;
  String? _latestLoadMatchId;
  AuthSession? _lastAuthSession;
  Future<RespondToMatchData?>? _responseFlight;
  String? _responseFlightMatchId;
  bool? _responseFlightAction;
  int? _responseFlightAuthGeneration;
  bool _disposed = false;

  @override
  PendingMatchState build() {
    _disposed = false;
    _sub?.close();
    _authSub?.close();
    _lastAuthSession = ref.read(authControllerProvider).session;
    _sub = ref.listen<AsyncValue<RealtimeFrame>>(
      realtimeEventProvider,
      (_, next) => next.whenData(_onFrame),
    );
    _authSub = ref.listen<AuthState>(authControllerProvider, (previous, next) {
      final prior = previous?.session ?? _lastAuthSession;
      _lastAuthSession = next.session;
      if (prior == next.session) return;
      _authGeneration++;
      _contextGeneration++;
      _operationGeneration++;
      _latestLoadMatchId = null;
      _inFlightLoads.clear();
      _responseFlight = null;
      _responseFlightMatchId = null;
      _responseFlightAction = null;
      _responseFlightAuthGeneration = null;
      state = const PendingMatchState();
    });
    ref.onDispose(() {
      _disposed = true;
      _sub?.close();
      _authSub?.close();
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

  _MatchBinding? _captureBinding() {
    if (_disposed) return null;
    final session = ref.read(authControllerProvider).session;
    if (session == null ||
        session.accessToken.isEmpty ||
        session.accountId.isEmpty ||
        session.activeProfileId.isEmpty) {
      return null;
    }
    return _MatchBinding(session: session, authGeneration: _authGeneration);
  }

  bool _isBindingCurrent(_MatchBinding binding) {
    if (_disposed || binding.authGeneration != _authGeneration) return false;
    final current = ref.read(authControllerProvider).session;
    return current != null &&
        current.accessToken == binding.session.accessToken &&
        current.refreshToken == binding.session.refreshToken &&
        current.accountId == binding.session.accountId &&
        current.activeProfileId == binding.session.activeProfileId;
  }

  Future<MatchData?> _loadAndShow(
    String matchId, {
    _MatchBinding? binding,
  }) async {
    final requestBinding = binding ?? _captureBinding();
    if (requestBinding == null || !_isBindingCurrent(requestBinding)) {
      return null;
    }
    if (_latestLoadMatchId != matchId) {
      _contextGeneration++;
      _latestLoadMatchId = matchId;
      _inFlightLoads.clear();
      if (state.match?.id != matchId) state = const PendingMatchState();
    }
    final generation = _contextGeneration;
    final loadKey = '${requestBinding.authGeneration}:$generation:$matchId';
    final inFlight = _inFlightLoads[loadKey];
    if (inFlight != null) return inFlight;
    final client = ref.read(voiceMatchmakingClientProvider);
    final elapsed = Stopwatch()..start();
    late final Future<MatchData?> request;
    request = () async {
      try {
        final result = await client.getMatch(
          authorization: requestBinding.session.authorizationHeader,
          matchId: matchId,
        );
        final requestElapsed = elapsed.elapsed;
        if (!_isBindingCurrent(requestBinding) ||
            generation != _contextGeneration ||
            _latestLoadMatchId != matchId) {
          return null;
        }
        if (result is! MatchmakingApiOk<MatchData> ||
            result.data.id != matchId ||
            !result.data.profileIds.contains(
              requestBinding.session.activeProfileId,
            )) {
          return null;
        }
        _applyLoadedMatch(result.data, requestElapsed: requestElapsed);
        return result.data;
      } catch (_) {
        return null;
      } finally {
        elapsed.stop();
      }
    }();
    _inFlightLoads[loadKey] = request;
    try {
      return await request;
    } finally {
      if (identical(_inFlightLoads[loadKey], request)) {
        _inFlightLoads.remove(loadKey);
      }
    }
  }

  Future<bool> refreshMatch(String matchId) async {
    if (state.match?.id != matchId) return false;
    final binding = _captureBinding();
    if (binding == null) return false;
    final loaded = await _loadAndShow(matchId, binding: binding);
    if (!_isBindingCurrent(binding)) return false;
    if (loaded == null) return state.match?.id == matchId;
    return state.match?.id == matchId &&
        state.match?.status == 'pending_accept';
  }

  void _applyLoadedMatch(
    MatchData match, {
    Duration requestElapsed = Duration.zero,
  }) {
    final currentId = state.match?.id;
    final clock = _clockFor(match, requestElapsed: requestElapsed);
    if (match.status == 'pending_accept') {
      state = PendingMatchState(
        match: match,
        deadlineClock: clock,
        isResponding: state.isResponding,
      );
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
          deadlineClock: clock,
          isResponding: state.isResponding,
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

  MatchDeadlineClock? _clockFor(
    MatchData match, {
    Duration requestElapsed = Duration.zero,
  }) {
    final serverNow = match.serverNow;
    final deadline = match.acceptanceDeadlineAt;
    final currentClock = state.match?.id == match.id
        ? state.deadlineClock
        : null;
    if (serverNow == null || deadline == null) return currentClock;
    if (currentClock == null) {
      return MatchDeadlineClock(
        serverNow: serverNow,
        deadline: deadline,
        requestElapsed: requestElapsed,
      );
    }
    currentClock.updateFromServer(
      serverNow: serverNow,
      deadline: deadline,
      requestElapsed: requestElapsed,
    );
    return currentClock;
  }

  void clear() {
    _contextGeneration++;
    _operationGeneration++;
    _latestLoadMatchId = null;
    _inFlightLoads.clear();
    state = const PendingMatchState();
  }

  Future<RespondToMatchData?> respond(bool accept) {
    final match = state.match;
    final binding = _captureBinding();
    if (match == null || binding == null) return Future.value(null);
    final activeFlight = _responseFlight;
    if (activeFlight != null) {
      if (_responseFlightAction == accept &&
          _responseFlightMatchId == match.id &&
          _responseFlightAuthGeneration == binding.authGeneration) {
        return activeFlight;
      }
      return Future.value(null);
    }
    final matchId = match.id;
    final contextGeneration = _contextGeneration;
    final operationGeneration = ++_operationGeneration;
    final client = ref.read(voiceMatchmakingClientProvider);
    late final Future<RespondToMatchData?> flight;
    flight =
        _respondBound(
          client: client,
          matchId: matchId,
          accept: accept,
          binding: binding,
          contextGeneration: contextGeneration,
          operationGeneration: operationGeneration,
        ).whenComplete(() {
          final ownsCurrentFlight =
              identical(_responseFlight, flight) &&
              _responseFlightMatchId == matchId &&
              _responseFlightAuthGeneration == binding.authGeneration &&
              _operationGeneration == operationGeneration;
          if (identical(_responseFlight, flight)) {
            _responseFlight = null;
            _responseFlightMatchId = null;
            _responseFlightAction = null;
            _responseFlightAuthGeneration = null;
          }
          if (ownsCurrentFlight &&
              !_disposed &&
              state.match?.id == matchId &&
              state.isResponding) {
            state = PendingMatchState(
              match: state.match,
              deadlineClock: state.deadlineClock,
            );
          }
        });
    _responseFlight = flight;
    _responseFlightMatchId = matchId;
    _responseFlightAction = accept;
    _responseFlightAuthGeneration = binding.authGeneration;
    state = PendingMatchState(
      match: match,
      deadlineClock: state.deadlineClock,
      isResponding: true,
    );
    return flight;
  }

  Future<RespondToMatchData?> _respondBound({
    required VoiceMatchmakingClient client,
    required String matchId,
    required bool accept,
    required _MatchBinding binding,
    required int contextGeneration,
    required int operationGeneration,
  }) async {
    bool current() =>
        _isBindingCurrent(binding) &&
        contextGeneration == _contextGeneration &&
        operationGeneration == _operationGeneration &&
        state.match?.id == matchId;
    final preflight = await _loadAndShow(matchId, binding: binding);
    if (!current() ||
        preflight?.id != matchId ||
        preflight?.status != 'pending_accept' ||
        state.match?.status != 'pending_accept') {
      return null;
    }
    final result = await client.respondToMatch(
      authorization: binding.session.authorizationHeader,
      matchId: matchId,
      accept: accept,
    );
    if (!current() ||
        result is! MatchmakingApiOk<RespondToMatchData> ||
        result.data.match.id != matchId) {
      return null;
    }
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
        isResponding: false,
      );
    }
    return data;
  }
}

class _MatchBinding {
  const _MatchBinding({required this.session, required this.authGeneration});

  final AuthSession session;
  final int authGeneration;
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
    final matchState = ref.watch(matchmakingMatchControllerProvider);
    final pending = matchState.match;
    if (pending == null) return const SizedBox.shrink();
    return MatchFoundOverlay(
      match: pending,
      deadlineClock: ref
          .watch(matchmakingMatchControllerProvider)
          .deadlineClock,
      isResponding: matchState.isResponding,
      onRefresh: () => ref
          .read(matchmakingMatchControllerProvider.notifier)
          .refreshMatch(pending.id),
      onRespond: (accept) =>
          ref.read(matchmakingMatchControllerProvider.notifier).respond(accept),
    );
  }
}
