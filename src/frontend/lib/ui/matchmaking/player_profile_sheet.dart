import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/matchmaking_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/matchmaking_providers.dart';
import '../../theme/voice_colors.dart';
import 'game_catalog_screen.dart';

/// Edit per-game matchmaking profile (region, role, rank).
class PlayerProfileSheet extends ConsumerStatefulWidget {
  const PlayerProfileSheet({super.key, this.initialGame});

  static const Key sheetKey = Key('player_profile_sheet');
  static const Key addGameButtonKey = Key('player_profile_add_game');
  static const Key saveButtonKey = Key('player_profile_save');
  static const Key regionChipKey = Key('player_profile_region_chip');

  final CatalogGame? initialGame;

  @override
  ConsumerState<PlayerProfileSheet> createState() => _PlayerProfileSheetState();
}

class _PlayerProfileSheetState extends ConsumerState<PlayerProfileSheet> {
  CatalogGame? _selectedGame;
  String? _region;
  String? _role;
  String? _rank;
  var _saving = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    _selectedGame = widget.initialGame;
    if (_selectedGame != null) {
      _applyGameDefaults(_selectedGame!);
    }
  }

  void _applyGameDefaults(CatalogGame game) {
    if (game.config.regions.isNotEmpty) {
      _region ??= game.config.regions.first;
    }
    final mode = game.config.modes.isNotEmpty ? game.config.modes.first : null;
    if (mode != null) {
      if (mode.roles.isNotEmpty) _role ??= mode.roles.first.name;
      if (mode.ranks.isNotEmpty) _rank ??= mode.ranks.first.name;
    }
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    final profileAsync = ref.watch(myPlayerProfileProvider);
    final catalogAsync = ref.watch(gameCatalogProvider);

    return SafeArea(
      child: Padding(
        key: PlayerProfileSheet.sheetKey,
        padding: const EdgeInsets.fromLTRB(24, 16, 24, 24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Text(
              l10n.playerProfileTitle,
              style: Theme.of(context).textTheme.titleLarge,
            ),
            const SizedBox(height: 12),
            profileAsync.when(
              loading: () => const LinearProgressIndicator(),
              error: (error, stackTrace) => Text(l10n.playerProfileLoadError),
              data: (profile) {
                if (profile.entries.isEmpty) {
                  return Text(
                    l10n.playerProfileEmpty,
                    style: TextStyle(color: voice.textSecondary),
                  );
                }
                return Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    for (final entry in profile.entries)
                      ListTile(
                        contentPadding: EdgeInsets.zero,
                        title: Text(_gameName(catalogAsync, entry.gameId)),
                        subtitle: Text(_entrySummary(entry)),
                        trailing: IconButton(
                          icon: const Icon(Icons.delete_outline),
                          onPressed: _saving
                              ? null
                              : () => _deleteEntry(entry.gameId),
                        ),
                      ),
                  ],
                );
              },
            ),
            const SizedBox(height: 16),
            OutlinedButton.icon(
              key: PlayerProfileSheet.addGameButtonKey,
              onPressed: _saving ? null : () => _pickGame(context),
              icon: const Icon(Icons.sports_esports_outlined),
              label: Text(l10n.playerProfileAddGame),
            ),
            if (_selectedGame != null) ...[
              const SizedBox(height: 16),
              Text(
                _selectedGame!.name,
                style: Theme.of(context).textTheme.titleSmall,
              ),
              const SizedBox(height: 8),
              Text(l10n.gameCatalogRegions(_selectedGame!.config.regions.join(', '))),
              const SizedBox(height: 8),
              Wrap(
                spacing: 8,
                children: [
                  for (final region in _selectedGame!.config.regions)
                    ChoiceChip(
                      key: region == _region ? PlayerProfileSheet.regionChipKey : null,
                      label: Text(region),
                      selected: _region == region,
                      onSelected: _saving
                          ? null
                          : (selected) {
                              if (selected) setState(() => _region = region);
                            },
                    ),
                ],
              ),
              ..._buildModePickers(context, l10n, _selectedGame!),
            ],
            if (_error != null) ...[
              const SizedBox(height: 12),
              Text(
                _error!,
                style: TextStyle(color: Theme.of(context).colorScheme.error),
              ),
            ],
            const SizedBox(height: 20),
            FilledButton(
              key: PlayerProfileSheet.saveButtonKey,
              onPressed: _saving || _selectedGame == null || _region == null
                  ? null
                  : _save,
              child: _saving
                  ? const SizedBox(
                      height: 18,
                      width: 18,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    )
                  : Text(l10n.playerProfileSave),
            ),
          ],
        ),
      ),
    );
  }

  List<Widget> _buildModePickers(
    BuildContext context,
    AppLocalizations l10n,
    CatalogGame game,
  ) {
    if (game.config.modes.isEmpty) return const [];
    final mode = game.config.modes.first;
    return [
      if (mode.roles.isNotEmpty) ...[
        const SizedBox(height: 12),
        Text(l10n.gameCatalogInGameRoles),
        Wrap(
          spacing: 8,
          children: [
            for (final role in mode.roles)
              ChoiceChip(
                label: Text(role.name),
                selected: _role == role.name,
                onSelected: _saving
                    ? null
                    : (selected) {
                        if (selected) setState(() => _role = role.name);
                      },
              ),
          ],
        ),
      ],
      if (mode.ranks.isNotEmpty) ...[
        const SizedBox(height: 12),
        Text(l10n.gameCatalogRankLadder),
        Wrap(
          spacing: 8,
          children: [
            for (final rank in mode.ranks)
              ChoiceChip(
                label: Text(rank.name),
                selected: _rank == rank.name,
                onSelected: _saving
                    ? null
                    : (selected) {
                        if (selected) setState(() => _rank = rank.name);
                      },
              ),
          ],
        ),
      ],
    ];
  }

  String _gameName(AsyncValue<GameListData> catalogAsync, String gameId) {
    final games = catalogAsync.valueOrNull?.games ?? const [];
    for (final g in games) {
      if (g.id == gameId) return g.name;
    }
    return gameId;
  }

  String _entrySummary(PlayerGameEntry entry) {
    final parts = <String>[entry.region];
    if (entry.role != null && entry.role!.isNotEmpty) parts.add(entry.role!);
    if (entry.rank != null && entry.rank!.isNotEmpty) parts.add(entry.rank!);
    return parts.join(' · ');
  }

  Future<void> _pickGame(BuildContext context) async {
    final game = await Navigator.of(context).push<CatalogGame>(
      MaterialPageRoute(builder: (_) => const GameCatalogScreen(selectMode: true)),
    );
    if (game == null || !mounted) return;
    setState(() {
      _selectedGame = game;
      _region = null;
      _role = null;
      _rank = null;
      _applyGameDefaults(game);
    });
  }

  Future<void> _save() async {
    final game = _selectedGame;
    final region = _region;
    if (game == null || region == null) return;
    setState(() {
      _saving = true;
      _error = null;
    });
    try {
      await ref.read(playerProfileActionsProvider).upsertEntry(
        gameId: game.id,
        region: region,
        role: _role,
        rank: _rank,
      );
      if (!mounted) return;
      Navigator.of(context).pop();
    } catch (e) {
      setState(() => _error = '$e');
    } finally {
      if (mounted) setState(() => _saving = false);
    }
  }

  Future<void> _deleteEntry(String gameId) async {
    setState(() => _saving = true);
    try {
      await ref.read(playerProfileActionsProvider).deleteEntry(gameId);
    } finally {
      if (mounted) setState(() => _saving = false);
    }
  }
}
