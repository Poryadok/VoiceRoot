import 'package:flutter/material.dart';

import '../../backend/matchmaking_client.dart';
import '../../l10n/app_localizations.dart';
import '../../theme/voice_colors.dart';
import 'player_profile_sheet.dart';
import 'queue_search_screen.dart';

/// Game page with modes, in-game roles and ranks from catalog config.
class GameDetailScreen extends StatefulWidget {
  const GameDetailScreen({super.key, required this.game, this.spaceId});

  static const Key screenKey = Key('game_detail_screen');
  static const Key rolesSectionKey = Key('game_detail_roles');
  static const Key ranksSectionKey = Key('game_detail_ranks');
  static const String regionPickerKeyName = 'game_detail_region_picker';
  static const Key regionPickerKey = Key(regionPickerKeyName);

  static String rolePickerKey(String mode) => 'game_detail_role_picker_$mode';
  static String ownRankPickerKey(String mode) =>
      'game_detail_own_rank_picker_$mode';
  static String soughtRankMinKey(String mode) =>
      'game_detail_sought_rank_min_$mode';
  static String soughtRankMaxKey(String mode) =>
      'game_detail_sought_rank_max_$mode';
  static String startButtonKey(String mode) =>
      'game_detail_start_queue_$mode';

  final CatalogGame game;
  final String? spaceId;

  @override
  State<GameDetailScreen> createState() => _GameDetailScreenState();
}

class _GameDetailScreenState extends State<GameDetailScreen> {
  String? _region;
  final Map<String, String?> roleByMode = {};
  final Map<String, String?> rankByMode = {};
  final Map<String, String?> minByMode = {};
  final Map<String, String?> maxByMode = {};

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    final theme = Theme.of(context);
    final game = widget.game;
    final spaceId = widget.spaceId;

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
              style: theme.textTheme.bodyMedium?.copyWith(
                color: voice.textSecondary,
              ),
            ),
            const SizedBox(height: 12),
            _dropdown(
              key: GameDetailScreen.regionPickerKeyName,
              label: l10n.queueSearchRegion,
              value: _region,
              items: game.config.regions,
              onChanged: (value) => setState(
                () => _region = (value == null || value.isEmpty) ? null : value,
              ),
            ),
          ],
          for (final mode in game.config.modes) ...[
            const SizedBox(height: 20),
            Text(mode.name, style: theme.textTheme.titleSmall),
            const SizedBox(height: 4),
            Text(
              l10n.gameCatalogModeSlots(
                mode.slots,
                mode.partySizeMin,
                mode.partySizeMax,
              ),
              style: theme.textTheme.bodySmall?.copyWith(
                color: voice.textSecondary,
              ),
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
            if (mode.roles.isNotEmpty)
              _dropdown(
                key: GameDetailScreen.rolePickerKey(mode.name),
                label: l10n.queueSearchRole,
                value: roleByMode[mode.name],
                items: [
                  if (!mode.rolesRequired) '',
                  for (final role in mode.roles) role.name,
                ],
                emptyItemLabel: mode.rolesRequired
                    ? null
                    : l10n.queueSearchNoRolePreference,
                onChanged: (value) => setState(
                  () => roleByMode[mode.name] =
                      (value == null || value.isEmpty) ? null : value,
                ),
              ),
            if (mode.rankRequired && mode.ranks.isNotEmpty) ...[
              _dropdown(
                key: GameDetailScreen.ownRankPickerKey(mode.name),
                label: l10n.queueSearchRank,
                value: rankByMode[mode.name],
                items: [for (final rank in mode.ranks) rank.name],
                onChanged: (value) => setState(
                  () => rankByMode[mode.name] =
                      (value == null || value.isEmpty) ? null : value,
                ),
              ),
              _dropdown(
                key: GameDetailScreen.soughtRankMinKey(mode.name),
                label: l10n.queueSearchSoughtRankMin,
                value: minByMode[mode.name],
                items: [for (final rank in mode.ranks) rank.name],
                onChanged: (value) => setState(
                  () => minByMode[mode.name] =
                      (value == null || value.isEmpty) ? null : value,
                ),
              ),
              _dropdown(
                key: GameDetailScreen.soughtRankMaxKey(mode.name),
                label: l10n.queueSearchSoughtRankMax,
                value: maxByMode[mode.name],
                items: [for (final rank in mode.ranks) rank.name],
                onChanged: (value) => setState(
                  () => maxByMode[mode.name] =
                      (value == null || value.isEmpty) ? null : value,
                ),
              ),
            ],
            FilledButton(
              key: Key(GameDetailScreen.startButtonKey(mode.name)),
              onPressed: () {
                Navigator.of(context).push(
                  MaterialPageRoute<void>(
                    builder: (_) => QueueSearchScreen(
                      game: game,
                      mode: mode,
                      spaceId: spaceId,
                    ),
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

  DropdownButtonFormField<String> _dropdown({
    required String key,
    required String label,
    required String? value,
    required List<String> items,
    required ValueChanged<String?> onChanged,
    String? emptyItemLabel,
  }) {
    return DropdownButtonFormField<String>(
      key: Key(key),
      initialValue: value,
      decoration: InputDecoration(labelText: label),
      items: [
        for (final item in items)
          DropdownMenuItem(
            value: item,
            child: Text(
              item.isEmpty ? (emptyItemLabel ?? item) : item,
            ),
          ),
      ],
      onChanged: onChanged,
    );
  }
}
