import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:intl/intl.dart';

import '../../backend/matchmaking_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/matchmaking_providers.dart';
import '../../state/social_providers.dart';
import '../../theme/voice_colors.dart';
import '../core/voice_state_panel.dart';
import '../social/profile_detail_sheet.dart';
import 'game_detail_screen.dart';

/// Paginated list of past and active match squads.
class MatchHistoryScreen extends ConsumerStatefulWidget {
  const MatchHistoryScreen({super.key});

  static const Key screenKey = Key('match_history_screen');
  static const Key listKey = Key('match_history_list');
  static const Key emptyKey = Key('match_history_empty');
  static const Key errorKey = Key('match_history_error');
  static const Key loadMoreKey = Key('match_history_load_more');

  static Key matchTileKey(String matchId) => Key('match_history_tile_$matchId');
  static Key addFriendKey(String profileId) =>
      Key('match_history_add_friend_$profileId');
  static Key banKey(String profileId) => Key('match_history_ban_$profileId');

  @override
  ConsumerState<MatchHistoryScreen> createState() => _MatchHistoryScreenState();
}

class _MatchHistoryScreenState extends ConsumerState<MatchHistoryScreen> {
  final List<MatchData> _matches = [];
  String? _nextCursor;
  bool _loading = true;
  bool _loadingMore = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    _loadInitial();
  }

  Future<void> _loadInitial() async {
    setState(() {
      _loading = true;
      _error = null;
      _matches.clear();
      _nextCursor = null;
    });
    await _fetchPage();
    if (mounted) setState(() => _loading = false);
  }

  Future<void> _loadMore() async {
    if (_loadingMore || _nextCursor == null || _nextCursor!.isEmpty) return;
    setState(() => _loadingMore = true);
    await _fetchPage(cursor: _nextCursor);
    if (mounted) setState(() => _loadingMore = false);
  }

  Future<void> _fetchPage({String? cursor}) async {
    final auth = ref.read(authControllerProvider);
    final token = auth.session?.accessToken;
    if (token == null || token.isEmpty) {
      setState(() => _error = 'not authenticated');
      return;
    }
    final client = ref.read(voiceMatchmakingClientProvider);
    final result = await client.listMatchHistory(
      authorization: 'Bearer $token',
      cursor: cursor,
      pageSize: 20,
    );
    switch (result) {
      case MatchmakingApiOk(:final data):
        setState(() {
          _matches.addAll(data.matches);
          _nextCursor = data.nextCursor;
          _error = null;
        });
      case MatchmakingApiFailure(:final message):
        setState(() => _error = message);
    }
  }

  String _gameName(MatchData match, GameListData? catalog) {
    if (match.gameName != null && match.gameName!.isNotEmpty) {
      return match.gameName!;
    }
    for (final game in catalog?.games ?? const <CatalogGame>[]) {
      if (game.id == match.gameId) return game.name;
    }
    return match.gameId;
  }

  String _statusLabel(AppLocalizations l10n, String status) {
    return switch (status) {
      'active' => l10n.matchHistoryStatusActive,
      'completed' => l10n.matchHistoryStatusCompleted,
      _ => status,
    };
  }

  Future<void> _banParticipant(String profileId, String displayName) async {
    final l10n = AppLocalizations.of(context)!;
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(l10n.matchRatingBanTitle),
        content: Text(l10n.matchRatingBanMessage(displayName)),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(false),
            child: Text(l10n.matchRatingBanCancel),
          ),
          FilledButton(
            onPressed: () => Navigator.of(ctx).pop(true),
            child: Text(l10n.matchRatingBanConfirm),
          ),
        ],
      ),
    );
    if (confirmed != true || !mounted) return;

    final auth = ref.read(authControllerProvider);
    final token = auth.session?.accessToken;
    if (token == null) return;
    final result = await ref.read(voiceMatchmakingClientProvider).banFromMM(
          authorization: 'Bearer $token',
          targetProfileId: profileId,
        );
    if (!mounted) return;
    if (result is MatchmakingApiFailure) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(l10n.matchRatingBanError)),
      );
    }
  }

  void _openGame(CatalogGame game) {
    Navigator.of(context).push(
      MaterialPageRoute<void>(builder: (_) => GameDetailScreen(game: game)),
    );
  }

  void _openProfile(String profileId) {
    final container = ProviderScope.containerOf(context);
    showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      builder: (sheetContext) => UncontrolledProviderScope(
        container: container,
        child: ProfileDetailSheet(profileId: profileId),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final catalogAsync = ref.watch(gameCatalogProvider);
    final activeId = ref.watch(authControllerProvider).activeProfileId;

    return Scaffold(
      key: MatchHistoryScreen.screenKey,
      appBar: AppBar(title: Text(l10n.matchHistoryTitle)),
      body: _loading
          ? const Center(child: CircularProgressIndicator())
          : _error != null && _matches.isEmpty
              ? VoiceStatePanel(
                  key: MatchHistoryScreen.errorKey,
                  title: l10n.matchHistoryLoadError,
                  message: _error,
                  icon: Icons.error_outline,
                  actionLabel: l10n.commonRetry,
                  onAction: _loadInitial,
                )
              : _matches.isEmpty
                  ? VoiceStatePanel(
                      key: MatchHistoryScreen.emptyKey,
                      title: l10n.matchHistoryEmpty,
                      icon: Icons.sports_esports_outlined,
                    )
                  : ListView.builder(
                      key: MatchHistoryScreen.listKey,
                      padding: const EdgeInsets.symmetric(vertical: 8),
                      itemCount: _matches.length + (_nextCursor != null ? 1 : 0),
                      itemBuilder: (context, index) {
                        if (index >= _matches.length) {
                          return Padding(
                            padding: const EdgeInsets.all(16),
                            child: Center(
                              child: _loadingMore
                                  ? const CircularProgressIndicator()
                                  : TextButton(
                                      key: MatchHistoryScreen.loadMoreKey,
                                      onPressed: _loadMore,
                                      child: Text(l10n.matchHistoryLoadMore),
                                    ),
                            ),
                          );
                        }
                        final match = _matches[index];
                        return _MatchHistoryTile(
                          key: MatchHistoryScreen.matchTileKey(match.id),
                          match: match,
                          gameName: _gameName(match, catalogAsync.valueOrNull),
                          statusLabel: _statusLabel(l10n, match.status),
                          activeProfileId: activeId,
                          onOpenGame: () {
                            final catalog = catalogAsync.valueOrNull;
                            CatalogGame? game;
                            for (final g in catalog?.games ?? const <CatalogGame>[]) {
                              if (g.id == match.gameId) {
                                game = g;
                                break;
                              }
                            }
                            if (game != null) {
                              _openGame(game);
                            }
                          },
                          onAddFriend: (profileId) async {
                            final err = await ref
                                .read(socialActionsProvider)
                                .sendFriendInvitation(profileId);
                            if (!context.mounted) return;
                            if (err != null) {
                              ScaffoldMessenger.of(context).showSnackBar(
                                SnackBar(content: Text(l10n.socialActionError(err))),
                              );
                            }
                          },
                          onBan: _banParticipant,
                          onOpenProfile: _openProfile,
                        );
                      },
                    ),
    );
  }
}

class _MatchHistoryTile extends StatelessWidget {
  const _MatchHistoryTile({
    super.key,
    required this.match,
    required this.gameName,
    required this.statusLabel,
    required this.activeProfileId,
    required this.onOpenGame,
    required this.onAddFriend,
    required this.onBan,
    required this.onOpenProfile,
  });

  final MatchData match;
  final String gameName;
  final String statusLabel;
  final String? activeProfileId;
  final VoidCallback onOpenGame;
  final Future<void> Function(String profileId) onAddFriend;
  final Future<void> Function(String profileId, String displayName) onBan;
  final void Function(String profileId) onOpenProfile;

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final date = match.createdAt != null
        ? DateFormat.yMMMd().add_Hm().format(match.createdAt!.toLocal())
        : '';

    return Card(
      margin: const EdgeInsets.symmetric(horizontal: 12, vertical: 4),
      child: ExpansionTile(
        title: Text(gameName),
        subtitle: Text('${match.mode} · ${match.region} · $statusLabel'),
        trailing: date.isNotEmpty
            ? Text(date, style: Theme.of(context).textTheme.bodySmall)
            : null,
        children: [
          ListTile(
            leading: const Icon(Icons.sports_esports_outlined),
            title: Text(gameName),
            subtitle: Text(l10n.gameCatalogTitle),
            onTap: onOpenGame,
          ),
          Padding(
            padding: const EdgeInsets.fromLTRB(16, 0, 16, 8),
            child: Align(
              alignment: Alignment.centerLeft,
              child: Text(
                l10n.matchHistoryParticipants,
                style: Theme.of(context).textTheme.titleSmall,
              ),
            ),
          ),
          for (final profileId in match.profileIds)
            _ParticipantRow(
              profileId: profileId,
              isSelf: profileId == activeProfileId,
              onAddFriend: () => onAddFriend(profileId),
              onBan: (name) => onBan(profileId, name),
              onOpenProfile: () => onOpenProfile(profileId),
            ),
          const SizedBox(height: 8),
        ],
      ),
    );
  }
}

class _ParticipantRow extends ConsumerWidget {
  const _ParticipantRow({
    required this.profileId,
    required this.isSelf,
    required this.onAddFriend,
    required this.onBan,
    required this.onOpenProfile,
  });

  final String profileId;
  final bool isSelf;
  final VoidCallback onAddFriend;
  final Future<void> Function(String displayName) onBan;
  final VoidCallback onOpenProfile;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final profileAsync = ref.watch(profileProvider(profileId));

    return profileAsync.when(
      loading: () => const ListTile(
        dense: true,
        leading: SizedBox(
          width: 24,
          height: 24,
          child: CircularProgressIndicator(strokeWidth: 2),
        ),
        title: Text('…'),
      ),
      error: (_, _st) => ListTile(
        dense: true,
        title: Text(profileId),
        onTap: onOpenProfile,
      ),
      data: (profile) {
        final name = profile?.displayName ?? profileId;
        return ListTile(
          dense: true,
          title: Text(name),
          subtitle: profile?.username != null ? Text('@${profile!.username}') : null,
          onTap: onOpenProfile,
          trailing: isSelf
              ? null
              : Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    TextButton(
                      key: MatchHistoryScreen.addFriendKey(profileId),
                      onPressed: onAddFriend,
                      child: Text(l10n.matchHistoryAddFriend),
                    ),
                    IconButton(
                      key: MatchHistoryScreen.banKey(profileId),
                      tooltip: l10n.matchRatingBanAction,
                      icon: Icon(Icons.block, color: VoiceColors.of(context).textSecondary),
                      onPressed: () => onBan(name),
                    ),
                  ],
                ),
        );
      },
    );
  }
}
