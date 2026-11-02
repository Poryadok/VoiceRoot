import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../backend/matchmaking_client.dart';
import '../l10n/app_localizations.dart';
import '../ui/matchmaking/match_rating_overlay.dart';
import '../ui/matchmaking/story_suggestion_overlay.dart';
import '../ui/stories/story_create_screen.dart';
import 'auth_providers.dart';
import 'matchmaking_providers.dart';
import 'social_providers.dart';

class PendingRatingState {
  const PendingRatingState({this.match, this.suggestStory = false});

  final MatchData? match;
  final bool suggestStory;
}

class MatchmakingRatingController extends Notifier<PendingRatingState> {
  @override
  PendingRatingState build() => const PendingRatingState();

  void showRatingForMatch(MatchData match) {
    state = PendingRatingState(match: match);
  }

  void clear() => state = const PendingRatingState();

  void finishRating() {
    final match = state.match;
    if (match == null) return;
    state = PendingRatingState(match: match, suggestStory: true);
  }
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

    if (ref.watch(matchmakingRatingControllerProvider).suggestStory) {
      final ids = pending.profileIds.where((id) => id != raterId).toList();
      final handles = [
        for (final id in ids)
          ref.watch(profileProvider(id)).valueOrNull?.handle ?? id,
      ];
      var gameName = pending.gameName ?? pending.gameId;
      final games =
          ref.watch(gameCatalogProvider).valueOrNull?.games ??
          const <CatalogGame>[];
      for (final game in games) {
        if (game.id == pending.gameId) gameName = game.name;
      }
      return Positioned.fill(
        child: StorySuggestionOverlay(
          gameName: gameName,
          teammateHandles: handles,
          onDismiss: () =>
              ref.read(matchmakingRatingControllerProvider.notifier).clear(),
          onEdit: () {
            ref.read(matchmakingRatingControllerProvider.notifier).clear();
            Navigator.of(context).push(
              MaterialPageRoute<void>(
                builder: (_) => StoryCreateScreen(
                  initialText: 'Match completed · $gameName',
                  initialGameTag: pending.gameId,
                  initialGameTagLabel: gameName,
                  initialMentionProfileIds: ids,
                ),
              ),
            );
          },
        ),
      );
    }

    final teammates = pending.profileIds
        .where((id) => id != raterId)
        .map((id) {
          final name = ref.watch(profileProvider(id)).valueOrNull?.displayName;
          return RatedTeammate(
            profileId: id,
            displayName: name == null || name.trim().isEmpty ? id : name,
          );
        })
        .toList(growable: false);

    return Positioned.fill(
      child: MatchRatingOverlay(
        match: pending,
        raterProfileId: raterId,
        teammates: teammates,
        onRate: (profileId, stars) =>
            _rate(context, ref, pending.id, profileId, stars),
        onBan: (profileId) => _ban(context, ref, profileId),
        onSkipAll: () => ref
            .read(matchmakingRatingControllerProvider.notifier)
            .finishRating(),
        onDone: () => ref
            .read(matchmakingRatingControllerProvider.notifier)
            .finishRating(),
      ),
    );
  }

  Future<bool> _rate(
    BuildContext context,
    WidgetRef ref,
    String matchId,
    String profileId,
    int stars,
  ) async {
    final token = ref.read(authControllerProvider).session?.accessToken;
    if (token == null || token.isEmpty) return false;
    final client = ref.read(voiceMatchmakingClientProvider);
    final result = await client.rateMatch(
      authorization: 'Bearer $token',
      matchId: matchId,
      ratedProfileId: profileId,
      stars: stars,
    );
    if (result is MatchmakingApiFailure) {
      if (context.mounted) {
        final l10n = AppLocalizations.of(context)!;
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text(l10n.matchRatingSubmitError)));
      }
      return false;
    }
    return true;
  }

  Future<bool> _ban(
    BuildContext context,
    WidgetRef ref,
    String profileId,
  ) async {
    final token = ref.read(authControllerProvider).session?.accessToken;
    if (token == null || token.isEmpty) return false;
    final client = ref.read(voiceMatchmakingClientProvider);
    final result = await client.banFromMM(
      authorization: 'Bearer $token',
      targetProfileId: profileId,
      reason: 'low_match_rating',
    );
    if (result is MatchmakingApiFailure) {
      if (context.mounted) {
        final l10n = AppLocalizations.of(context)!;
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text(l10n.matchRatingBanError)));
      }
      return false;
    }
    return true;
  }
}
