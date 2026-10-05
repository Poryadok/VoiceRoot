import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/friends_client.dart';
import '../../backend/matchmaking_client.dart';
import '../../backend/users_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/chat_providers.dart';
import '../../state/dm_permission_provider.dart';
import '../../state/matchmaking_providers.dart';
import '../../state/social_providers.dart';
import '../api_error_messages.dart';
import '../core/voice_state_panel.dart';

/// A participant's existing account and Matchmaking information.
class MatchmakingPlayerProfileSheet extends ConsumerStatefulWidget {
  const MatchmakingPlayerProfileSheet({
    super.key,
    required this.profileId,
    required this.gameId,
    required this.canBanFromCurrentMatch,
  });

  static const Key sheetKey = Key('matchmaking_player_profile_sheet');
  static const Key ratingKey = Key('matchmaking_player_profile_rating');
  static const Key ratingStarKey = Key(
    'matchmaking_player_profile_rating_star',
  );
  static const Key friendKey = Key('matchmaking_player_profile_friend');
  static const Key messageKey = Key('matchmaking_player_profile_message');
  static const Key messagePermissionRetryKey = Key(
    'matchmaking_player_profile_message_permission_retry',
  );
  static const Key banKey = Key('matchmaking_player_profile_ban');

  final String profileId;
  final String gameId;
  final bool canBanFromCurrentMatch;

  @override
  ConsumerState<MatchmakingPlayerProfileSheet> createState() =>
      _MatchmakingPlayerProfileSheetState();
}

class _MatchmakingPlayerProfileSheetState
    extends ConsumerState<MatchmakingPlayerProfileSheet> {
  String? _initialAuthorization;
  String? _initialViewerId;
  bool _closingForContextChange = false;
  Route<dynamic>? _ownedSheetRoute;
  DialogRoute<bool>? _ownedBanConfirmationRoute;

  @override
  void initState() {
    super.initState();
    final auth = ref.read(authControllerProvider);
    _initialAuthorization = auth.session?.authorizationHeader;
    _initialViewerId = auth.activeProfileId;
  }

  @override
  Widget build(BuildContext context) {
    _ownedSheetRoute ??= ModalRoute.of(context);
    final auth = ref.watch(authControllerProvider);
    final authorization = auth.session?.authorizationHeader;
    final viewerId = auth.activeProfileId;
    final args = (profileId: widget.profileId, gameId: widget.gameId);
    ref.listen<AuthState>(authControllerProvider, (previous, next) {
      final previousAuthorization = previous?.session?.authorizationHeader;
      final nextAuthorization = next.session?.authorizationHeader;
      if (previous?.activeProfileId == next.activeProfileId &&
          previousAuthorization == nextAuthorization) {
        return;
      }
      _closingForContextChange = true;
      ref.invalidate(playerRatingProvider(args));
      if (context.mounted) _closeOwnedRoutesForContextChange();
    });

    final contextChanged =
        authorization != _initialAuthorization || viewerId != _initialViewerId;
    if (_closingForContextChange || contextChanged) {
      return const SizedBox.shrink();
    }

    final l10n = AppLocalizations.of(context)!;
    final profile = ref.watch(profileProvider(widget.profileId));
    final rating = ref.watch(playerRatingProvider(args));

    return SafeArea(
      child: Padding(
        padding: EdgeInsets.only(
          left: 20,
          right: 20,
          bottom: 20 + MediaQuery.viewInsetsOf(context).bottom,
        ),
        child: ConstrainedBox(
          constraints: BoxConstraints(
            maxHeight: MediaQuery.sizeOf(context).height * .86,
          ),
          child: SingleChildScrollView(
            child: Column(
              key: MatchmakingPlayerProfileSheet.sheetKey,
              crossAxisAlignment: CrossAxisAlignment.stretch,
              mainAxisSize: MainAxisSize.min,
              children: [
                profile.when(
                  skipLoadingOnRefresh: false,
                  loading: () => const Padding(
                    padding: EdgeInsets.all(32),
                    child: Center(child: CircularProgressIndicator()),
                  ),
                  error: (error, stackTrace) => _loadError(l10n),
                  data: (value) {
                    if (value == null) return _loadError(l10n);
                    return _buildProfile(
                      context,
                      ref,
                      l10n,
                      auth,
                      value,
                      rating,
                    );
                  },
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  void _closeOwnedRoutesForContextChange() {
    final dialogRoute = _ownedBanConfirmationRoute;
    _ownedBanConfirmationRoute = null;
    if (dialogRoute?.isActive ?? false) {
      dialogRoute!.navigator?.removeRoute(dialogRoute);
    }

    final sheetRoute = _ownedSheetRoute;
    if (sheetRoute?.isActive ?? false) {
      sheetRoute!.navigator?.removeRoute(sheetRoute);
    }
  }

  Widget _loadError(AppLocalizations l10n) => VoiceStatePanel(
    title: l10n.socialProfileLoadError,
    icon: Icons.cloud_off_outlined,
    actionLabel: l10n.commonRetry,
    onAction: () {
      ref.invalidate(profileProvider(widget.profileId));
      ref.invalidate(
        playerRatingProvider((
          profileId: widget.profileId,
          gameId: widget.gameId,
        )),
      );
    },
  );

  Widget _buildProfile(
    BuildContext context,
    WidgetRef ref,
    AppLocalizations l10n,
    AuthState auth,
    VoiceProfile profile,
    AsyncValue<PlayerRatingData?> rating,
  ) {
    final isSelf = auth.activeProfileId == widget.profileId;
    final canContact = auth.session != null && !auth.isGuest && !isSelf;
    final canBan =
        auth.session != null && widget.canBanFromCurrentMatch && !isSelf;
    final friends = ref.watch(friendsListProvider);
    final requests = ref.watch(friendRequestsProvider);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Row(
          children: [
            CircleAvatar(
              backgroundImage: profile.avatarUrl == null
                  ? null
                  : NetworkImage(profile.avatarUrl!),
              child: profile.avatarUrl == null
                  ? Text(
                      profile.displayName.isEmpty
                          ? (profile.username.isEmpty
                                ? '?'
                                : profile.username
                                      .substring(0, 1)
                                      .toUpperCase())
                          : profile.displayName.substring(0, 1).toUpperCase(),
                    )
                  : null,
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    profile.displayName.isEmpty
                        ? profile.handle
                        : profile.displayName,
                    style: Theme.of(context).textTheme.titleLarge,
                  ),
                  Text(profile.handle),
                ],
              ),
            ),
          ],
        ),
        if (profile.bio != null && profile.bio!.isNotEmpty) ...[
          const SizedBox(height: 12),
          Text(profile.bio!),
        ],
        const SizedBox(height: 16),
        rating.when(
          skipLoadingOnRefresh: false,
          loading: () => const Center(child: CircularProgressIndicator()),
          error: (error, stackTrace) => const SizedBox.shrink(),
          data: (value) {
            if (value == null ||
                value.profileId != widget.profileId ||
                value.gameId != widget.gameId) {
              return const SizedBox.shrink();
            }
            final ratingLabel = l10n.profileMmRating(
              value.ratingValue.toStringAsFixed(1),
            );
            final visibleRatingLabel = ratingLabel.endsWith(' ★')
                ? ratingLabel.substring(0, ratingLabel.length - 2)
                : ratingLabel;
            return Semantics(
              key: MatchmakingPlayerProfileSheet.ratingKey,
              label: ratingLabel,
              child: ExcludeSemantics(
                child: Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Text(visibleRatingLabel),
                    const SizedBox(width: 4),
                    Icon(
                      Icons.star,
                      key: MatchmakingPlayerProfileSheet.ratingStarKey,
                      size: 16,
                    ),
                  ],
                ),
              ),
            );
          },
        ),
        const SizedBox(height: 16),
        if (canContact) ...[
          _friendAction(context, l10n, friends, requests),
          const SizedBox(height: 8),
          _messagePermissionAction(context, ref, l10n, auth),
        ],
        if (canBan) ...[
          const SizedBox(height: 8),
          OutlinedButton.icon(
            key: MatchmakingPlayerProfileSheet.banKey,
            onPressed: () => _confirmBan(context, ref, l10n, profile),
            icon: const Icon(Icons.block_outlined),
            label: Text(l10n.matchRatingBanAction),
          ),
        ],
      ],
    );
  }

  Widget _messagePermissionAction(
    BuildContext context,
    WidgetRef ref,
    AppLocalizations l10n,
    AuthState auth,
  ) {
    final session = auth.session;
    final viewerProfileId = auth.activeProfileId;
    if (session == null || viewerProfileId == null || viewerProfileId.isEmpty) {
      return const SizedBox.shrink();
    }
    final request = (
      authorization: session.authorizationHeader,
      accountId: session.accountId,
      viewerProfileId: viewerProfileId,
      targetProfileId: widget.profileId,
    );
    final permission = ref.watch(dmPermissionProvider(request));
    return permission.when(
      loading: () => const SizedBox(
        height: 48,
        child: Center(
          child: SizedBox.square(
            dimension: 20,
            child: CircularProgressIndicator(strokeWidth: 2),
          ),
        ),
      ),
      error: (_, _) => Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(l10n.chatListLoadError),
          TextButton.icon(
            key: MatchmakingPlayerProfileSheet.messagePermissionRetryKey,
            onPressed: () => ref.invalidate(dmPermissionProvider(request)),
            icon: const Icon(Icons.refresh),
            label: Text(l10n.commonRetry),
          ),
        ],
      ),
      data: (allowed) => allowed
          ? OutlinedButton.icon(
              key: MatchmakingPlayerProfileSheet.messageKey,
              onPressed: () => _message(context, ref, l10n),
              icon: const Icon(Icons.chat_bubble_outline),
              label: Text(l10n.profileMessage),
            )
          : const SizedBox.shrink(),
    );
  }

  Widget _friendAction(
    BuildContext context,
    AppLocalizations l10n,
    AsyncValue<FriendsListData> friends,
    AsyncValue<FriendRequestsData> requests,
  ) {
    if (friends.isLoading || requests.isLoading) {
      return const Center(child: CircularProgressIndicator());
    }
    if (friends.hasError || requests.hasError) {
      return Text(
        socialListErrorMessage(l10n, friends.error ?? requests.error!),
      );
    }
    if (friends.requireValue.friends.contains(widget.profileId)) {
      return const SizedBox.shrink();
    }
    final pending = requests.requireValue.outgoing.any(
      (request) => request.profileId == widget.profileId && !request.isDeclined,
    );
    if (pending) return Text(l10n.socialRequestPending);
    return FilledButton.icon(
      key: MatchmakingPlayerProfileSheet.friendKey,
      onPressed: () => _sendFriendRequest(context, ref, l10n),
      icon: const Icon(Icons.person_add_alt_1),
      label: Text(l10n.socialAddFriend),
    );
  }

  Future<void> _sendFriendRequest(
    BuildContext context,
    WidgetRef ref,
    AppLocalizations l10n,
  ) async {
    final error = await ref
        .read(socialActionsProvider)
        .sendFriendInvitation(widget.profileId);
    if (!context.mounted) return;
    if (error != null) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(socialActionErrorMessage(l10n, error))),
      );
    }
  }

  Future<void> _message(
    BuildContext context,
    WidgetRef ref,
    AppLocalizations l10n,
  ) async {
    final error = await ref
        .read(chatActionsProvider)
        .openDmWithProfile(widget.profileId);
    if (!context.mounted) return;
    if (error != null) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(chatActionErrorMessage(l10n, error))),
      );
      return;
    }
    Navigator.of(context).pop();
  }

  Future<void> _confirmBan(
    BuildContext context,
    WidgetRef ref,
    AppLocalizations l10n,
    VoiceProfile profile,
  ) async {
    final navigator = Navigator.of(context);
    final dialogRoute = DialogRoute<bool>(
      context: context,
      themes: InheritedTheme.capture(from: context, to: navigator.context),
      barrierColor:
          DialogTheme.of(context).barrierColor ??
          Theme.of(context).dialogTheme.barrierColor ??
          Colors.black54,
      barrierDismissible: true,
      barrierLabel: MaterialLocalizations.of(context).modalBarrierDismissLabel,
      useSafeArea: true,
      builder: (dialogContext) => AlertDialog(
        title: Text(l10n.matchRatingBanTitle),
        content: Text(
          l10n.matchRatingBanMessage(
            profile.displayName.isEmpty ? profile.handle : profile.displayName,
          ),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(dialogContext).pop(false),
            child: Text(l10n.matchRatingBanCancel),
          ),
          FilledButton(
            onPressed: () => Navigator.of(dialogContext).pop(true),
            child: Text(l10n.matchRatingBanConfirm),
          ),
        ],
      ),
    );
    _ownedBanConfirmationRoute = dialogRoute;
    final confirmed = await navigator.push(dialogRoute);
    if (identical(_ownedBanConfirmationRoute, dialogRoute)) {
      _ownedBanConfirmationRoute = null;
    }
    if (confirmed != true || !context.mounted) return;
    final auth = ref.read(authControllerProvider);
    final session = auth.session;
    if (session == null || auth.activeProfileId != _initialViewerId) return;
    final result = await ref
        .read(voiceMatchmakingClientProvider)
        .banFromMM(
          authorization: session.authorizationHeader,
          targetProfileId: widget.profileId,
        );
    if (!context.mounted) return;
    final current = ref.read(authControllerProvider);
    if (current.activeProfileId != _initialViewerId ||
        current.session?.authorizationHeader != _initialAuthorization) {
      return;
    }
    if (result case MatchmakingApiFailure()) {
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text(l10n.matchRatingBanError)));
    }
  }
}
