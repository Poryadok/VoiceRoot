import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/stories_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/chat_providers.dart';
import '../../state/matchmaking_providers.dart';
import '../../theme/voice_colors.dart';
import '../matchmaking/game_catalog_screen.dart';
import '../matchmaking/queue_search_screen.dart';

/// Looking-for-party story card with join/write actions.
class LfpStoryCard extends StatelessWidget {
  const LfpStoryCard({
    super.key,
    required this.story,
    this.onJoin,
    this.onWrite,
  });

  static const Key cardKey = Key('lfp_story_card');
  static const Key joinKey = Key('lfp_story_join');
  static const Key writeKey = Key('lfp_story_write');

  final StoryData story;
  final VoidCallback? onJoin;
  final VoidCallback? onWrite;

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    final criteria = _parseCriteria(story.lfpCriteriaJson);

    return Card(
      key: LfpStoryCard.cardKey,
      color: voice.elevated,
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          mainAxisSize: MainAxisSize.min,
          children: [
            Row(
              children: [
                Icon(Icons.groups_outlined, color: voice.profileAccent),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(
                    l10n.storyLfpTitle,
                    style: Theme.of(context).textTheme.titleMedium?.copyWith(
                          color: voice.textPrimary,
                        ),
                  ),
                ),
              ],
            ),
            if (story.gameTag != null && story.gameTag!.isNotEmpty) ...[
              const SizedBox(height: 8),
              _InfoRow(
                label: l10n.storyLfpGame,
                value: story.gameTag!,
              ),
            ],
            if (criteria.isNotEmpty) ...[
              const SizedBox(height: 8),
              ...criteria.entries.map(
                (e) => _InfoRow(label: e.key, value: '${e.value}'),
              ),
            ],
            if (story.textContent != null && story.textContent!.isNotEmpty) ...[
              const SizedBox(height: 12),
              Text(
                story.textContent!,
                style: TextStyle(color: voice.textSecondary),
              ),
            ],
            const SizedBox(height: 16),
            Row(
              children: [
                Expanded(
                  child: FilledButton(
                    key: LfpStoryCard.joinKey,
                    onPressed: onJoin ?? () => LfpStoryActions.join(context, story),
                    child: Text(l10n.storyLfpJoin),
                  ),
                ),
                const SizedBox(width: 8),
                Expanded(
                  child: OutlinedButton(
                    key: LfpStoryCard.writeKey,
                    onPressed: onWrite ?? () => LfpStoryActions.write(context, story),
                    child: Text(l10n.storyLfpWrite),
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }

  Map<String, Object?> _parseCriteria(String? raw) {
    if (raw == null || raw.isEmpty) return const {};
    try {
      final decoded = jsonDecode(raw);
      if (decoded is Map<String, dynamic>) return decoded;
    } catch (_) {}
    return const {};
  }
}

/// Navigation stubs for LFP story actions (matchmaking + DM).
abstract final class LfpStoryActions {
  static Future<void> join(BuildContext context, StoryData story) async {
    ProviderContainer? container;
    try {
      container = ProviderScope.containerOf(context, listen: false);
    } catch (_) {
      return;
    }
    final ref = _RefReader(container);
    final gameId = story.gameTag;
    if (gameId == null || gameId.isEmpty) {
      await Navigator.of(context).push(
        MaterialPageRoute<void>(
          builder: (_) => const GameCatalogScreen(selectMode: true),
        ),
      );
      return;
    }

    final catalog = await ref.read(gameCatalogProvider.future);
    final game = catalog.games.where((g) => g.id == gameId).firstOrNull;
    final modes = game?.config.modes ?? const [];
    if (game == null || modes.isEmpty) {
      if (!context.mounted) return;
      await Navigator.of(context).push(
        MaterialPageRoute<void>(
          builder: (_) => const GameCatalogScreen(selectMode: true),
        ),
      );
      return;
    }

    if (!context.mounted) return;
    await Navigator.of(context).push(
      MaterialPageRoute<void>(
        builder: (_) => QueueSearchScreen(
          game: game,
          mode: modes.first,
        ),
      ),
    );
  }

  static Future<void> write(BuildContext context, StoryData story) async {
    ProviderContainer? container;
    try {
      container = ProviderScope.containerOf(context, listen: false);
    } catch (_) {
      return;
    }
    final ref = _RefReader(container);
    final authorId = story.authorProfileId;
    if (authorId.isEmpty) return;
    final err = await ref
        .read(chatActionsProvider)
        .openDmWithProfile(authorId);
    if (!context.mounted) return;
    if (err != null) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(err)),
      );
    }
  }
}

/// Minimal ref-like reader for static navigation helpers.
class _RefReader {
  _RefReader(this._container);

  final ProviderContainer _container;

  T read<T>(ProviderListenable<T> provider) => _container.read(provider);
}

class _InfoRow extends StatelessWidget {
  const _InfoRow({required this.label, required this.value});

  final String label;
  final String value;

  @override
  Widget build(BuildContext context) {
    final voice = VoiceColors.of(context);
    return Padding(
      padding: const EdgeInsets.only(bottom: 4),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            '$label: ',
            style: TextStyle(
              color: voice.textSecondary,
              fontWeight: FontWeight.w600,
            ),
          ),
          Expanded(
            child: Text(value, style: TextStyle(color: voice.textPrimary)),
          ),
        ],
      ),
    );
  }
}
