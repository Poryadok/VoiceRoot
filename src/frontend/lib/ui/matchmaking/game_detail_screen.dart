import 'package:flutter/material.dart';

import '../../backend/matchmaking_client.dart';
import '../../l10n/app_localizations.dart';
import '../../theme/voice_colors.dart';
import 'player_profile_sheet.dart';
import 'queue_search_screen.dart';

/// Game page with modes, in-game roles and ranks from catalog config.
class GameDetailScreen extends StatelessWidget {
  const GameDetailScreen({super.key, required this.game});

  static const Key screenKey = Key('game_detail_screen');
  static const Key rolesSectionKey = Key('game_detail_roles');
  static const Key ranksSectionKey = Key('game_detail_ranks');

  final CatalogGame game;

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    final theme = Theme.of(context);

    return Scaffold(
      key: GameDetailScreen.screenKey,
      appBar: AppBar(title: Text(game.name)),
      body: ListView(
        padding: const EdgeInsets.all(16),
        children: [
          if (game.config.genre != null)
            Text(game.config.genre!, style: theme.textTheme.titleMedium),
          if (game.config.regions.isNotEmpty) ...[
            const SizedBox(height: 8),
            Text(
              l10n.gameCatalogRegions(game.config.regions.join(', ')),
              style: theme.textTheme.bodyMedium?.copyWith(color: voice.textSecondary),
            ),
          ],
          for (final mode in game.config.modes) ...[
            const SizedBox(height: 20),
            Text(mode.name, style: theme.textTheme.titleSmall),
            const SizedBox(height: 4),
            Text(
              l10n.gameCatalogModeSlots(mode.slots, mode.partySizeMin, mode.partySizeMax),
              style: theme.textTheme.bodySmall?.copyWith(color: voice.textSecondary),
            ),
            if (mode.roles.isNotEmpty) ...[
              const SizedBox(height: 12),
              Text(l10n.gameCatalogInGameRoles, style: theme.textTheme.labelLarge),
              const SizedBox(height: 6),
              Wrap(
                key: GameDetailScreen.rolesSectionKey,
                spacing: 8,
                runSpacing: 8,
                children: [
                  for (final role in mode.roles)
                    Chip(
                      label: Text(role.name),
                      avatar: role.required
                          ? Icon(Icons.star, size: 16, color: theme.colorScheme.primary)
                          : null,
                    ),
                ],
              ),
            ],
            if (mode.ranks.isNotEmpty) ...[
              const SizedBox(height: 12),
              Text(l10n.gameCatalogRankLadder, style: theme.textTheme.labelLarge),
              const SizedBox(height: 6),
              Column(
                key: GameDetailScreen.ranksSectionKey,
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  for (final rank in mode.ranks)
                    Padding(
                      padding: const EdgeInsets.only(bottom: 4),
                      child: Text('${rank.name} (${rank.value})'),
                    ),
                ],
              ),
            ],
            const SizedBox(height: 12),
            FilledButton(
              key: Key('game_detail_start_queue_${mode.name}'),
              onPressed: () {
                Navigator.of(context).push(
                  MaterialPageRoute<void>(
                    builder: (_) => QueueSearchScreen(game: game, mode: mode),
                  ),
                );
              },
              child: Text(l10n.queueSearchStart),
            ),
          ],
          const SizedBox(height: 24),
          OutlinedButton.icon(
            onPressed: () {
              showModalBottomSheet<void>(
                context: context,
                isScrollControlled: true,
                builder: (_) => PlayerProfileSheet(initialGame: game),
              );
            },
            icon: const Icon(Icons.person_outline),
            label: Text(l10n.playerProfileForGame),
          ),
        ],
      ),
    );
  }
}
