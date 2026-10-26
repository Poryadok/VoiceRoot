import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/matchmaking_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/game_catalog_recent_provider.dart';
import '../../state/matchmaking_providers.dart';
import '../../theme/voice_colors.dart';
import '../../theme/voice_metrics.dart';
import '../api_error_messages.dart';
import '../core/voice_skeleton.dart';
import '../core/voice_state_panel.dart';
import 'add_game_screen.dart';
import 'game_detail_screen.dart';

/// Browse real recent games and the public, popularity-ordered catalog.
class GameCatalogScreen extends ConsumerStatefulWidget {
  const GameCatalogScreen({super.key, this.selectMode = false, this.spaceId});
  final bool selectMode;
  final String? spaceId;
  static const Key screenKey = Key('game_catalog_screen');
  static const Key searchFieldKey = Key('game_catalog_search');
  static const Key listKey = Key('game_catalog_list');
  static const Key addGameButtonKey = Key('game_catalog_add_game');
  static const Key browseAllKey = Key('game_catalog_browse_all');
  static const Key loadMoreKey = Key('game_catalog_load_more');
  @override
  ConsumerState<GameCatalogScreen> createState() => _GameCatalogScreenState();
}

class _GameCatalogScreenState extends ConsumerState<GameCatalogScreen> {
  late final TextEditingController _searchController;
  final _searchFocus = FocusNode();
  bool _expanded = false;
  bool _loadingMore = false;
  bool _loadedMore = false;
  bool _stale = false;
  String? _pageError;
  String? _nextCursor;
  final List<CatalogGame> _additional = [];
  int _generation = 0;

  @override
  void initState() {
    super.initState();
    _searchController = TextEditingController(
      text: ref.read(gameCatalogSearchQueryProvider),
    );
  }

  @override
  void dispose() {
    _searchController.dispose();
    _searchFocus.dispose();
    super.dispose();
  }

  void _changeQuery(String value) {
    setState(() {
      _generation++;
      _additional.clear();
      _loadedMore = false;
      _loadingMore = false;
      _pageError = null;
      _stale = false;
      _nextCursor = null;
    });
    ref.read(gameCatalogSearchQueryProvider.notifier).state = value;
  }

  void _retryCatalog() {
    _changeQuery(_searchController.text);
    ref.invalidate(gameCatalogProvider);
    ref.invalidate(gameCatalogSearchProvider);
  }

  Future<void> _loadMore(String cursor) async {
    if (_loadingMore) return;
    final generation = _generation;
    setState(() {
      _loadingMore = true;
      _pageError = null;
    });
    final token = ref.read(authControllerProvider).session?.accessToken;
    final result = token == null || token.isEmpty
        ? const MatchmakingApiFailure(message: 'not authenticated')
        : await ref
              .read(voiceMatchmakingClientProvider)
              .listGames(authorization: 'Bearer $token', cursor: cursor);
    if (!mounted || generation != _generation) return;
    setState(() {
      _loadingMore = false;
      switch (result) {
        case MatchmakingApiOk(:final data):
          _additional.addAll(data.games);
          _nextCursor = data.nextCursor;
          _loadedMore = true;
        case MatchmakingApiFailure(:final message, :final statusCode):
          _stale = statusCode == 412;
          _pageError = gameCatalogErrorMessage(
            AppLocalizations.of(context)!,
            Exception(message),
          );
      }
    });
  }

  void _openGame(CatalogGame game) {
    if (widget.selectMode) {
      Navigator.of(context).pop(game);
      return;
    }
    Navigator.of(context).push(
      MaterialPageRoute<void>(
        builder: (_) => GameDetailScreen(game: game, spaceId: widget.spaceId),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final colors = VoiceColors.of(context);
    final query = ref.watch(gameCatalogSearchQueryProvider);
    final searching = query.trim().isNotEmpty;
    final catalog = ref.watch(gameCatalogSearchProvider);
    final recent = searching ? null : ref.watch(gameCatalogRecentProvider);
    return Scaffold(
      key: GameCatalogScreen.screenKey,
      backgroundColor: colors.canvas,
      appBar: AppBar(
        title: Text(l10n.gameCatalogTitle),
        toolbarHeight: 56,
        titleTextStyle: Theme.of(context).textTheme.titleMedium?.copyWith(
          fontSize: 16,
          fontWeight: FontWeight.w600,
        ),
        backgroundColor: colors.canvas,
        shape: Border(bottom: BorderSide(color: colors.borderDefault)),
        actions: [
          if (!widget.selectMode)
            IconButton(
              key: GameCatalogScreen.addGameButtonKey,
              tooltip: l10n.gameCatalogAddGame,
              icon: const Icon(Icons.add),
              onPressed: () => Navigator.of(context).push(
                MaterialPageRoute<void>(builder: (_) => const AddGameScreen()),
              ),
            ),
        ],
      ),
      body: SafeArea(
        top: false,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Padding(
              padding: const EdgeInsets.all(16),
              child: TextField(
                key: GameCatalogScreen.searchFieldKey,
                controller: _searchController,
                focusNode: _searchFocus,
                textInputAction: TextInputAction.search,
                decoration: InputDecoration(
                  hintText: l10n.gameCatalogSearchHint,
                  hintStyle: Theme.of(context).textTheme.bodyMedium,
                  prefixIcon: const Icon(Icons.search, size: 20),
                  prefixIconConstraints: const BoxConstraints(
                    minWidth: 44,
                    minHeight: 44,
                  ),
                  suffixIconConstraints: const BoxConstraints(
                    minWidth: 44,
                    minHeight: 44,
                  ),
                  suffixIcon: query.isEmpty
                      ? null
                      : IconButton(
                          tooltip: l10n.gameCatalogClearSearch,
                          icon: const Icon(Icons.close, size: 20),
                          onPressed: () {
                            _searchController.clear();
                            _changeQuery('');
                            _searchFocus.requestFocus();
                          },
                        ),
                  isDense: true,
                  contentPadding: const EdgeInsets.symmetric(
                    horizontal: 16,
                    vertical: 8,
                  ),
                  enabledBorder: OutlineInputBorder(
                    borderRadius: BorderRadius.circular(999),
                    borderSide: BorderSide(color: colors.borderDefault),
                  ),
                  focusedBorder: OutlineInputBorder(
                    borderRadius: BorderRadius.circular(999),
                    borderSide: BorderSide(color: colors.focusRing, width: 2),
                  ),
                ),
                onChanged: _changeQuery,
              ),
            ),
            Expanded(
              child: catalog.when(
                loading: () => const VoiceListSkeleton(),
                error: (error, _) => VoiceStatePanel(
                  icon: Icons.videogame_asset_off_outlined,
                  title: l10n.gameCatalogLoadError,
                  message: gameCatalogErrorMessage(l10n, error),
                  actionLabel: l10n.commonRetry,
                  onAction: _retryCatalog,
                ),
                data: (data) {
                  final ids = <String>{};
                  final games = [
                    ...data.games,
                    ..._additional,
                  ].where((game) => ids.add(game.id)).toList();
                  final cursor = _loadedMore ? _nextCursor : data.nextCursor;
                  return ListView(
                    key: GameCatalogScreen.listKey,
                    padding: const EdgeInsets.fromLTRB(16, 0, 16, 24),
                    children: [
                      if (recent != null) ...[
                        if (recent.isLoading) const LinearProgressIndicator(),
                        if (recent.hasError)
                          Row(
                            children: [
                              Expanded(
                                child: Text(l10n.gameCatalogRecentError),
                              ),
                              TextButton(
                                onPressed: () =>
                                    ref.invalidate(gameCatalogRecentProvider),
                                child: Text(l10n.commonRetry),
                              ),
                            ],
                          ),
                        if (recent.valueOrNull?.isNotEmpty ?? false) ...[
                          _SectionHeading(l10n.gameCatalogRecent),
                          const SizedBox(height: 12),
                          _GameGrid(
                            games: recent.valueOrNull!,
                            section: 'recent',
                            onTap: _openGame,
                          ),
                          const SizedBox(height: 24),
                        ],
                      ],
                      LayoutBuilder(
                        builder: (context, constraints) {
                          final actions = Wrap(
                            spacing: 8,
                            runSpacing: 8,
                            children: [
                              if (!searching && !_expanded)
                                OutlinedButton(
                                  key: GameCatalogScreen.browseAllKey,
                                  onPressed: () =>
                                      setState(() => _expanded = true),
                                  child: Text(l10n.gameCatalogBrowseAll),
                                ),
                            ],
                          );
                          final heading = _SectionHeading(
                            searching
                                ? l10n.gameCatalogSearchResults
                                : _expanded
                                ? l10n.gameCatalogAll
                                : l10n.gameCatalogPopular,
                          );
                          if (constraints.maxWidth < 600) {
                            return Column(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                heading,
                                const SizedBox(height: 12),
                                actions,
                              ],
                            );
                          }
                          return Row(
                            children: [
                              Expanded(child: heading),
                              actions,
                            ],
                          );
                        },
                      ),
                      const SizedBox(height: 12),
                      if (games.isEmpty)
                        VoiceStatePanel(
                          icon: Icons.search_off,
                          title: l10n.gameCatalogEmpty,
                        )
                      else
                        _GameGrid(
                          games: searching || _expanded
                              ? games
                              : games.take(3).toList(),
                          section: 'catalog',
                          onTap: _openGame,
                        ),
                      if (!searching &&
                          _expanded &&
                          cursor != null &&
                          cursor.isNotEmpty) ...[
                        const SizedBox(height: 16),
                        if (_pageError != null)
                          Semantics(
                            liveRegion: true,
                            child: Text(
                              _stale ? l10n.gameCatalogChanged : _pageError!,
                            ),
                          ),
                        Align(
                          alignment: Alignment.centerLeft,
                          child: OutlinedButton(
                            key: GameCatalogScreen.loadMoreKey,
                            onPressed: _loadingMore
                                ? null
                                : _stale
                                ? _retryCatalog
                                : () => _loadMore(cursor),
                            child: _loadingMore
                                ? const SizedBox(
                                    width: 20,
                                    height: 20,
                                    child: CircularProgressIndicator(
                                      strokeWidth: 2,
                                    ),
                                  )
                                : Text(
                                    _pageError != null
                                        ? l10n.commonRetry
                                        : l10n.gameCatalogLoadMore,
                                  ),
                          ),
                        ),
                      ],
                    ],
                  );
                },
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _SectionHeading extends StatelessWidget {
  const _SectionHeading(this.label);
  final String label;
  @override
  Widget build(BuildContext context) => Semantics(
    header: true,
    child: Text(
      label,
      style: Theme.of(context).textTheme.titleMedium?.copyWith(
        fontSize: 16,
        fontWeight: FontWeight.w600,
      ),
    ),
  );
}

class _GameGrid extends StatelessWidget {
  const _GameGrid({
    required this.games,
    required this.section,
    required this.onTap,
  });
  final List<CatalogGame> games;
  final String section;
  final ValueChanged<CatalogGame> onTap;
  @override
  Widget build(BuildContext context) {
    final colors = VoiceColors.of(context);
    final radius = BorderRadius.circular(
      context.voiceMetrics.corner('md', fallback: 6),
    );
    return LayoutBuilder(
      builder: (context, constraints) {
        final columns = constraints.maxWidth < 300
            ? 1
            : constraints.maxWidth < 600
            ? 2
            : 3;
        final width = (constraints.maxWidth - (columns - 1) * 12) / columns;
        final height = constraints.maxWidth < 600 ? 112.0 : 132.0;
        return Wrap(
          spacing: 12,
          runSpacing: 12,
          children: [
            for (final game in games)
              SizedBox(
                width: width,
                child: Material(
                  key: Key(
                    section == 'catalog'
                        ? 'game_catalog_card_${game.id}'
                        : 'game_catalog_recent_${game.id}',
                  ),
                  color: colors.surface,
                  shape: RoundedRectangleBorder(
                    borderRadius: radius,
                    side: BorderSide(color: colors.borderDefault),
                  ),
                  clipBehavior: Clip.antiAlias,
                  child: InkWell(
                    onTap: () => onTap(game),
                    child: Semantics(
                      button: true,
                      child: ConstrainedBox(
                        constraints: BoxConstraints(minHeight: height),
                        child: Stack(
                          children: [
                            if (game.iconUrl?.isNotEmpty ?? false)
                              Positioned.fill(
                                child: ExcludeSemantics(
                                  child: Image.network(
                                    game.iconUrl!,
                                    fit: BoxFit.cover,
                                    errorBuilder: (_, _, _) =>
                                        const SizedBox.shrink(),
                                  ),
                                ),
                              ),
                            Container(
                              width: double.infinity,
                              constraints: BoxConstraints(minHeight: height),
                              padding: const EdgeInsets.all(16),
                              decoration: BoxDecoration(
                                gradient: LinearGradient(
                                  begin: Alignment.topCenter,
                                  end: Alignment.bottomCenter,
                                  colors: [
                                    game.iconUrl?.isNotEmpty == true
                                        ? colors.surface.withValues(alpha: 0.45)
                                        : Color.alphaBlend(
                                            colors.profileAccent.withValues(
                                              alpha: 0.22,
                                            ),
                                            colors.surface,
                                          ),
                                    colors.surface,
                                  ],
                                ),
                              ),
                              child: Column(
                                crossAxisAlignment: CrossAxisAlignment.start,
                                children: [
                                  ExcludeSemantics(
                                    child: Icon(
                                      Icons.sports_esports_outlined,
                                      color: colors.textSecondary,
                                      size: 20,
                                    ),
                                  ),
                                  SizedBox(
                                    height: constraints.maxWidth < 600 ? 0 : 20,
                                  ),
                                  Text(
                                    game.name,
                                    style: Theme.of(context)
                                        .textTheme
                                        .titleSmall
                                        ?.copyWith(
                                          fontSize: 15,
                                          fontWeight: FontWeight.w600,
                                        ),
                                  ),
                                  if (game.config.genre?.isNotEmpty == true ||
                                      game.config.modes.isNotEmpty) ...[
                                    const SizedBox(height: 4),
                                    Text(
                                      [
                                        if (game.config.genre?.isNotEmpty ==
                                            true)
                                          game.config.genre!,
                                        if (game.config.modes.isNotEmpty)
                                          game.config.modes.first.name,
                                      ].join(' · '),
                                      style: Theme.of(context)
                                          .textTheme
                                          .bodySmall
                                          ?.copyWith(
                                            color: colors.textSecondary,
                                          ),
                                    ),
                                  ],
                                ],
                              ),
                            ),
                          ],
                        ),
                      ),
                    ),
                  ),
                ),
              ),
          ],
        );
      },
    );
  }
}
