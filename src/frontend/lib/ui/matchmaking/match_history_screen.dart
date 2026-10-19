import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:intl/intl.dart';

import '../../backend/api_errors.dart';
import '../../backend/matchmaking_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/matchmaking_providers.dart';
import '../../state/social_providers.dart';
import '../../theme/voice_colors.dart';
import '../api_error_messages.dart';
import '../core/voice_skeleton.dart';
import '../core/voice_state_panel.dart';
import '../report/report_sheet.dart';
import '../social/profile_detail_sheet.dart';
import 'game_detail_screen.dart';
import 'game_catalog_screen.dart';
import 'match_rating_overlay.dart';

/// Paginated list of past and active match squads.
class MatchHistoryScreen extends ConsumerStatefulWidget {
  const MatchHistoryScreen({super.key});

  static const Key screenKey = Key('match_history_screen');
  static const Key listKey = Key('match_history_list');
  static const Key emptyKey = Key('match_history_empty');
  static const Key errorKey = Key('match_history_error');
  static const Key loadMoreKey = Key('match_history_load_more');
  static const Key findPlayersKey = Key('match_history_find_players');

  static Key matchTileKey(String matchId) => Key('match_history_tile_$matchId');
  static Key rateKey(String profileId) => Key('match_history_rate_$profileId');
  static Key reportKey(String profileId) =>
      Key('match_history_report_$profileId');
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
  int? _errorStatusCode;

  @override
  void initState() {
    super.initState();
    _loadInitial();
  }

  Future<void> _loadInitial() async {
    setState(() {
      _loading = true;
      _error = null;
      _errorStatusCode = null;
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

  bool get _hasMore => _nextCursor != null && _nextCursor!.isNotEmpty;

  Future<void> _fetchPage({String? cursor}) async {
    final auth = ref.read(authControllerProvider);
    final token = auth.session?.accessToken;
    if (token == null || token.isEmpty) {
      setState(() {
        _error = 'not authenticated';
        _errorStatusCode = null;
      });
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
          _errorStatusCode = null;
        });
      case MatchmakingApiFailure(:final statusCode):
        setState(() {
          _error = '';
          _errorStatusCode = statusCode;
        });
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

  Future<bool> _banParticipant(String profileId, String displayName) async {
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
    if (confirmed != true || !mounted) return false;
    return _submitParticipantBan(profileId);
  }

  Future<bool> _submitParticipantBan(String profileId) async {
    final l10n = AppLocalizations.of(context)!;
    final auth = ref.read(authControllerProvider);
    final token = auth.session?.accessToken;
    if (token == null) return false;
    final result = await ref
        .read(voiceMatchmakingClientProvider)
        .banFromMM(authorization: 'Bearer $token', targetProfileId: profileId);
    if (!mounted) return false;
    if (result is MatchmakingApiFailure) {
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text(l10n.matchRatingBanError)));
      return false;
    }
    return true;
  }

  void _openRating(MatchData match, String profileId, String displayName) {
    final raterId = ref.read(authControllerProvider).activeProfileId;
    if (raterId == null || raterId.isEmpty || raterId == profileId) return;

    showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      useSafeArea: true,
      builder: (sheetContext) => MatchRatingOverlay(
        match: match,
        raterProfileId: raterId,
        teammates: [
          RatedTeammate(profileId: profileId, displayName: displayName),
        ],
        onRate: (ratedProfileId, stars) async {
          final token = ref.read(authControllerProvider).session?.accessToken;
          if (token == null || token.isEmpty) return false;
          final result = await ref
              .read(voiceMatchmakingClientProvider)
              .rateMatch(
                authorization: 'Bearer $token',
                matchId: match.id,
                ratedProfileId: ratedProfileId,
                stars: stars,
              );
          if (!mounted) return false;
          if (result is MatchmakingApiFailure) {
            ScaffoldMessenger.of(context).showSnackBar(
              SnackBar(
                content: Text(
                  AppLocalizations.of(context)!.matchRatingSubmitError,
                ),
              ),
            );
            return false;
          }
          return true;
        },
        onBan: _submitParticipantBan,
        onSkipAll: () => Navigator.of(sheetContext).pop(),
        onDone: () => Navigator.of(sheetContext).pop(),
      ),
    );
  }

  void _openFindPlayers() {
    Navigator.of(
      context,
    ).push(MaterialPageRoute<void>(builder: (_) => const GameCatalogScreen()));
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
      bottomNavigationBar: SafeArea(
        top: false,
        child: Padding(
          padding: const EdgeInsets.all(16),
          child: SizedBox(
            width: double.infinity,
            child: FilledButton.icon(
              key: MatchHistoryScreen.findPlayersKey,
              onPressed: _openFindPlayers,
              icon: const Icon(Icons.search),
              label: Text(l10n.matchHistoryFindPlayers),
            ),
          ),
        ),
      ),
      body: _loading
          ? const VoiceListSkeleton()
          : _error != null && _matches.isEmpty
          ? VoiceStatePanel(
              key: MatchHistoryScreen.errorKey,
              title: l10n.matchHistoryLoadError,
              message: _errorStatusCode == null
                  ? _error
                  : isBackendUnavailable(_errorStatusCode)
                  ? l10n.backendUnavailable
                  : null,
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
              itemCount: _matches.length + (_hasMore ? 1 : 0),
              itemBuilder: (context, index) {
                if (index >= _matches.length) {
                  return Padding(
                    padding: const EdgeInsets.all(16),
                    child: Center(
                      child: _loadingMore
                          ? const CircularProgressIndicator()
                          : _error != null
                          ? Column(
                              mainAxisSize: MainAxisSize.min,
                              children: [
                                Text(
                                  _errorStatusCode != null &&
                                          isBackendUnavailable(_errorStatusCode)
                                      ? l10n.backendUnavailable
                                      : l10n.matchHistoryLoadError,
                                  textAlign: TextAlign.center,
                                ),
                                TextButton(
                                  key: MatchHistoryScreen.loadMoreKey,
                                  onPressed: _loadMore,
                                  child: Text(l10n.commonRetry),
                                ),
                              ],
                            )
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
                  isCompleted: match.status == 'completed',
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
                  onRate: (profileId, displayName) =>
                      _openRating(match, profileId, displayName),
                  onReport: (profileId) => ReportSheet.show(
                    context,
                    target: ReportUserTarget(profileId: profileId),
                  ),
                  onAddFriend: (profileId) async {
                    final err = await ref
                        .read(socialActionsProvider)
                        .sendFriendInvitation(profileId);
                    if (!context.mounted) return;
                    if (err != null) {
                      ScaffoldMessenger.of(context).showSnackBar(
                        SnackBar(
                          content: Text(socialActionErrorMessage(l10n, err)),
                        ),
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
    required this.isCompleted,
    required this.onOpenGame,
    required this.onRate,
    required this.onReport,
    required this.onAddFriend,
    required this.onBan,
    required this.onOpenProfile,
  });

  final MatchData match;
  final String gameName;
  final String statusLabel;
  final String? activeProfileId;
  final bool isCompleted;
  final VoidCallback onOpenGame;
  final void Function(String profileId, String displayName) onRate;
  final Future<void> Function(String profileId) onReport;
  final Future<void> Function(String profileId) onAddFriend;
  final Future<bool> Function(String profileId, String displayName) onBan;
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
              isCompleted: isCompleted,
              onRate: (name) => onRate(profileId, name),
              onReport: () => onReport(profileId),
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
    required this.isCompleted,
    required this.onRate,
    required this.onReport,
    required this.onAddFriend,
    required this.onBan,
    required this.onOpenProfile,
  });

  final String profileId;
  final bool isSelf;
  final bool isCompleted;
  final void Function(String displayName) onRate;
  final Future<void> Function() onReport;
  final VoidCallback onAddFriend;
  final Future<bool> Function(String displayName) onBan;
  final VoidCallback onOpenProfile;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final profileAsync = ref.watch(profileProvider(profileId));
    final isFriend = ref.watch(isFriendProvider(profileId));

    final name = profileAsync.valueOrNull?.displayName ?? profileId;
    final profile = profileAsync.valueOrNull;
    return Column(
      children: [
        profileAsync.when(
          loading: () => const ListTile(
            dense: true,
            leading: SizedBox(
              width: 24,
              height: 24,
              child: CircularProgressIndicator(strokeWidth: 2),
            ),
            title: Text('…'),
          ),
          error: (_, _) => ListTile(
            dense: true,
            title: Text(profileId),
            onTap: onOpenProfile,
          ),
          data: (_) => ListTile(
            dense: true,
            title: Text(name),
            subtitle: profile?.username != null
                ? Text('@${profile!.username}')
                : null,
            onTap: onOpenProfile,
          ),
        ),
        if (!isSelf)
          Padding(
            padding: const EdgeInsets.fromLTRB(16, 0, 16, 8),
            child: Align(
              alignment: Alignment.centerLeft,
              child: Wrap(
                spacing: 4,
                runSpacing: 4,
                children: [
                  if (isCompleted) ...[
                    TextButton(
                      key: MatchHistoryScreen.rateKey(profileId),
                      onPressed: () => onRate(name),
                      child: Text(l10n.matchHistoryRatePlayer),
                    ),
                    TextButton(
                      key: MatchHistoryScreen.reportKey(profileId),
                      onPressed: onReport,
                      child: Text(l10n.reportAction),
                    ),
                    if (!isFriend)
                      TextButton(
                        key: MatchHistoryScreen.addFriendKey(profileId),
                        onPressed: onAddFriend,
                        child: Text(l10n.matchHistoryAddFriend),
                      ),
                  ],
                  IconButton(
                    key: MatchHistoryScreen.banKey(profileId),
                    tooltip: l10n.matchRatingBanAction,
                    icon: Icon(
                      Icons.block,
                      color: VoiceColors.of(context).textSecondary,
                    ),
                    onPressed: () => onBan(name),
                  ),
                ],
              ),
            ),
          ),
      ],
    );
  }
}
