import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../backend/matchmaking_client.dart';
import '../ui/matchmaking/match_rating_overlay.dart';
import 'auth_providers.dart';
import 'matchmaking_providers.dart';

class PendingRatingState {
  const PendingRatingState({this.match});

  final MatchData? match;
}

class MatchmakingRatingController extends Notifier<PendingRatingState> {
  @override
  PendingRatingState build() => const PendingRatingState();

  void showRatingForMatch(MatchData match) {
    state = PendingRatingState(match: match);
  }

  void clear() => state = const PendingRatingState();
}

final matchmakingRatingControllerProvider =
    NotifierProvider<MatchmakingRatingController, PendingRatingState>(
  MatchmakingRatingController.new,
);

/// Global host for post-match rating UI.
class MatchRatingOverlayHost extends ConsumerWidget {
  const MatchRatingOverlayHost({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final pending = ref.watch(matchmakingRatingControllerProvider).match;
    if (pending == null) return const SizedBox.shrink();

    final auth = ref.watch(authControllerProvider);
    final raterId = auth.activeProfileId;
    if (raterId == null || raterId.isEmpty) return const SizedBox.shrink();

    final teammates = pending.profileIds
        .where((id) => id != raterId)
        .map(
          (id) => RatedTeammate(
            profileId: id,
            displayName: id,
          ),
        )
        .toList(growable: false);

    return Positioned.fill(
      child: MatchRatingOverlay(
        match: pending,
        raterProfileId: raterId,
        teammates: teammates,
        onRate: (profileId, stars) => _rate(
          ref,
          pending.id,
          profileId,
          stars,
        ),
        onBan: (profileId) => _ban(ref, profileId),
        onSkipAll: () =>
            ref.read(matchmakingRatingControllerProvider.notifier).clear(),
        onDone: () =>
            ref.read(matchmakingRatingControllerProvider.notifier).clear(),
      ),
    );
  }

  Future<void> _rate(
    WidgetRef ref,
    String matchId,
    String profileId,
    int stars,
  ) async {
    final token = ref.read(authControllerProvider).session?.accessToken;
    if (token == null || token.isEmpty) return;
    final client = ref.read(voiceMatchmakingClientProvider);
    await client.rateMatch(
      authorization: 'Bearer $token',
      matchId: matchId,
      ratedProfileId: profileId,
      stars: stars,
    );
  }

  Future<void> _ban(WidgetRef ref, String profileId) async {
    final token = ref.read(authControllerProvider).session?.accessToken;
    if (token == null || token.isEmpty) return;
    final client = ref.read(voiceMatchmakingClientProvider);
    await client.banFromMM(
      authorization: 'Bearer $token',
      targetProfileId: profileId,
      reason: 'low_match_rating',
    );
  }
}
