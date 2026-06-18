import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../l10n/app_localizations.dart';
import '../../state/matchmaking_providers.dart';
import '../../theme/voice_colors.dart';
import '../matchmaking/game_detail_screen.dart';

/// Game tag overlay for non-LFP stories (docs/features/stories.md §Game tag).
class StoryGameTagChip extends ConsumerWidget {
  const StoryGameTagChip({
    super.key,
    required this.gameTag,
  });

  static const Key chipKey = Key('story_game_tag_chip');

  final String gameTag;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    final catalogAsync = ref.watch(gameCatalogProvider);

    final label = catalogAsync.maybeWhen(
      data: (catalog) {
        for (final game in catalog.games) {
          if (game.id == gameTag) return game.name;
        }
        return gameTag;
      },
      orElse: () => gameTag,
    );

    return Material(
      key: chipKey,
      color: voice.elevated.withValues(alpha: 0.92),
      borderRadius: BorderRadius.circular(20),
      child: InkWell(
        borderRadius: BorderRadius.circular(20),
        onTap: () {
          final game = catalogAsync.valueOrNull?.games
              .where((g) => g.id == gameTag)
              .firstOrNull;
          if (game == null) return;
          Navigator.of(context).push(
            MaterialPageRoute(
              builder: (_) => GameDetailScreen(game: game),
            ),
          );
        },
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(Icons.sports_esports_outlined,
                  size: 16, color: voice.profileAccent),
              const SizedBox(width: 6),
              Text(
                label,
                style: Theme.of(context).textTheme.labelMedium?.copyWith(
                      color: voice.textPrimary,
                    ),
              ),
              const SizedBox(width: 4),
              Semantics(
                label: l10n.storyGameTagTapHint,
                child: Icon(Icons.chevron_right,
                    size: 16, color: voice.textSecondary),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
