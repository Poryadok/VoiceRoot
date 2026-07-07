import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/matchmaking_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/matchmaking_providers.dart';
import '../core/voice_state_panel.dart';
import 'game_detail_screen.dart';

/// Browse matchmaking game catalog (docs/features/game-catalog.md).
class GameCatalogScreen extends ConsumerStatefulWidget {
  const GameCatalogScreen({super.key, this.selectMode = false});

  /// When true, tapping a game returns it to the previous route.
  final bool selectMode;

  static const Key screenKey = Key('game_catalog_screen');
  static const Key searchFieldKey = Key('game_catalog_search');
  static const Key listKey = Key('game_catalog_list');

  @override
  ConsumerState<GameCatalogScreen> createState() => _GameCatalogScreenState();
}

class _GameCatalogScreenState extends ConsumerState<GameCatalogScreen> {
  final _searchController = TextEditingController();

  @override
  void dispose() {
    _searchController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final catalogAsync = ref.watch(gameCatalogSearchProvider);

    return Scaffold(
      key: GameCatalogScreen.screenKey,
      appBar: AppBar(
        title: Text(l10n.gameCatalogTitle),
      ),
      body: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Padding(
            padding: const EdgeInsets.fromLTRB(16, 12, 16, 8),
            child: TextField(
              key: GameCatalogScreen.searchFieldKey,
              controller: _searchController,
              decoration: InputDecoration(
                hintText: l10n.gameCatalogSearchHint,
                prefixIcon: const Icon(Icons.search),
                border: const OutlineInputBorder(),
              ),
              onChanged: (value) {
                ref.read(gameCatalogSearchQueryProvider.notifier).state = value;
              },
            ),
          ),
          Expanded(
            child: catalogAsync.when(
              data: (data) => _GameList(
                key: GameCatalogScreen.listKey,
                games: data.games,
                onTap: (game) {
                  if (widget.selectMode) {
                    Navigator.of(context).pop(game);
                    return;
                  }
                  Navigator.of(context).push(
                    MaterialPageRoute<void>(
                      builder: (_) => GameDetailScreen(game: game),
                    ),
                  );
                },
              ),
              loading: () => const Center(child: CircularProgressIndicator()),
              error: (error, _) => VoiceStatePanel(
                icon: Icons.videogame_asset_off_outlined,
                title: l10n.gameCatalogLoadError,
                message: error.toString(),
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class _GameList extends StatelessWidget {
  const _GameList({
    super.key,
    required this.games,
    required this.onTap,
  });

  final List<CatalogGame> games;
  final void Function(CatalogGame game) onTap;

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    if (games.isEmpty) {
      return VoiceStatePanel(
        icon: Icons.search_off,
        title: l10n.gameCatalogEmpty,
      );
    }
    return ListView.separated(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
      itemCount: games.length,
      separatorBuilder: (context, index) => const SizedBox(height: 8),
      itemBuilder: (context, index) {
        final game = games[index];
        return Card(
          key: Key('game_catalog_card_${game.id}'),
          child: ListTile(
            leading: const Icon(Icons.sports_esports_outlined),
            title: Text(game.name),
            subtitle: Text(
              [
                if (game.config.genre != null) game.config.genre!,
                if (game.config.modes.isNotEmpty) game.config.modes.first.name,
              ].join(' · '),
            ),
            trailing: const Icon(Icons.chevron_right),
            onTap: () => onTap(game),
          ),
        );
      },
    );
  }
}
