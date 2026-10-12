import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/api_errors.dart';
import '../../backend/friends_client.dart';
import '../../backend/matchmaking_client.dart';
import '../../backend/users_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/chat_providers.dart';
import '../../state/dm_permission_provider.dart';
import '../../state/presence_providers.dart';
import '../../state/matchmaking_providers.dart';
import '../../state/social_providers.dart';
import '../../state/stories_providers.dart';
import '../../routing/deep_link_urls.dart';
import '../api_error_messages.dart';
import '../core/chat_author_label.dart';
import '../core/voice_share_link.dart';
import '../core/voice_skeleton.dart';
import '../core/voice_state_panel.dart';
import '../report/report_sheet.dart';
import '../stories/highlights_section.dart';
import '../stories/story_ring_avatar.dart';
import '../../routing/stories_routes.dart';
import 'presence_indicator.dart';

/// Resolves contact membership across every page, scoped to the current session.
/// A partial list or a session switch is never interpreted as "not a contact".
final profileContactMembershipProvider = FutureProvider.autoDispose
    .family<bool?, String>((ref, targetProfileId) async {
      var disposed = false;
      ref.onDispose(() => disposed = true);
      final session = ref.watch(
        authControllerProvider.select((state) => state.session),
      );
      if (session == null || ref.watch(profileSwitchInProgressProvider)) {
        return null;
      }

      final client = ref.watch(voiceFriendsClientProvider);
      final seenCursors = <String>{};
      String? cursor;
      while (true) {
        final result = await client.listContacts(
          authorization: session.authorizationHeader,
          cursor: cursor,
        );
        if (disposed || ref.read(profileSwitchInProgressProvider)) return null;

        final currentSession = ref.read(authControllerProvider).session;
        if (currentSession?.accountId != session.accountId ||
            currentSession?.activeProfileId != session.activeProfileId ||
            currentSession?.authorizationHeader !=
                session.authorizationHeader) {
          return null;
        }

        switch (result) {
          case FriendsApiOk(:final data):
            if (data.contacts.any(
              (contact) => contact.profileId == targetProfileId,
            )) {
              return false;
            }
            final nextCursor = data.nextCursor;
            if (nextCursor == null || nextCursor.isEmpty) return true;
            if (nextCursor == cursor || !seenCursors.add(nextCursor)) {
              throw StateError('profile_contacts_cursor_did_not_advance');
            }
            cursor = nextCursor;
          case FriendsApiFailure():
            throw StateError('profile_contacts_unavailable');
        }
      }
    });

String _mmEntryLabel(
  AsyncValue<GameListData> catalogAsync,
  PlayerGameEntry entry,
) {
  final games = catalogAsync.valueOrNull?.games ?? const [];
  var name = entry.gameId;
  for (final g in games) {
    if (g.id == entry.gameId) {
      name = g.name;
      break;
    }
  }
  final parts = <String>[name, entry.region];
  if (entry.role != null && entry.role!.isNotEmpty) parts.add(entry.role!);
  if (entry.rank != null && entry.rank!.isNotEmpty) parts.add(entry.rank!);
  return parts.join(' · ');
}

/// Bottom sheet with profile details, presence, and friend-request action.
class ProfileDetailSheet extends ConsumerWidget {
  const ProfileDetailSheet({super.key, required this.profileId});

  static const Key sheetKey = Key('profile_detail_sheet');
  static const Key onlineIndicatorKey = Key('profile_online_indicator');
  static const Key addFriendKey = Key('profile_add_friend');
  static const Key removeFriendKey = Key('profile_remove_friend');
  static const Key messageKey = Key('profile_message');
  static const Key blockKey = Key('profile_block');

  final String profileId;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final profileAsync = ref.watch(profileProvider(profileId));
    final mmProfileAsync = ref.watch(playerProfileProvider(profileId));
    final catalogAsync = ref.watch(gameCatalogProvider);
    final firstGameId = mmProfileAsync.valueOrNull?.entries.firstOrNull?.gameId;
    final mmRatingAsync = firstGameId == null
        ? const AsyncValue<PlayerRatingData?>.data(null)
        : ref.watch(
            playerRatingProvider((profileId: profileId, gameId: firstGameId)),
          );
    final presence = ref.watch(presenceProvider(profileId));
    final requestsAsync = ref.watch(friendRequestsProvider);
    final auth = ref.watch(authControllerProvider);
    final isProfileSwitching = ref.watch(profileSwitchInProgressProvider);
    final activeId = auth.activeProfileId;
    final isGuest = auth.isGuest;
    final isSelf = activeId == profileId;
    final session = auth.session;
    final contactMembershipAsync =
        !isGuest && !isSelf && activeId != null && session != null
        ? ref.watch(profileContactMembershipProvider(profileId))
        : null;
    final dmPermissionRequest = session == null || activeId == null
        ? null
        : (
            authorization: session.authorizationHeader,
            accountId: session.accountId,
            viewerProfileId: activeId,
            targetProfileId: profileId,
          );
    final dmPermission = !isGuest && !isSelf && dmPermissionRequest != null
        ? ref.watch(dmPermissionProvider(dmPermissionRequest))
        : null;

    final outgoing = requestsAsync.valueOrNull?.outgoing ?? const [];
    final incoming = requestsAsync.valueOrNull?.incoming ?? const [];
    final pendingOutgoing = outgoing.any(
      (request) => request.profileId == profileId && !request.isDeclined,
    );
    final pendingIncoming = incoming.contains(profileId);
    final isFriend = ref.watch(isFriendProvider(profileId));
    final showNotInContactsWarning =
        !isGuest &&
        !auth.isRestoring &&
        !isProfileSwitching &&
        session != null &&
        activeId != null &&
        !isSelf &&
        contactMembershipAsync != null &&
        contactMembershipAsync.valueOrNull == true &&
        !contactMembershipAsync.isLoading &&
        !contactMembershipAsync.isRefreshing &&
        !contactMembershipAsync.isReloading &&
        !contactMembershipAsync.hasError;
    final activeAuthors = ref.watch(activeStoryAuthorIdsProvider);
    final hasActiveStory = activeAuthors.contains(profileId);
    final profileStoriesAsync = ref.watch(profileStoriesProvider(profileId));

    return SafeArea(
      child: Padding(
        key: sheetKey,
        padding: const EdgeInsets.fromLTRB(24, 16, 24, 24),
        child: profileAsync.when(
          loading: () => const VoiceListSkeleton(rowCount: 3),
          error: (e, st) {
            if (e is ProfileUnavailableException) {
              return VoiceStatePanel(
                title: l10n.socialProfileUnavailable,
                icon: Icons.person_off_outlined,
              );
            }
            return VoiceStatePanel(
              title: l10n.socialProfileLoadError,
              message: socialProfileErrorMessage(l10n, e),
              icon: Icons.person_off_outlined,
              actionLabel: l10n.commonRetry,
              onAction: () => ref.invalidate(profileProvider(profileId)),
            );
          },
          data: (profile) {
            if (profile == null) {
              return VoiceStatePanel(
                title: l10n.socialProfileUnavailable,
                icon: Icons.person_off_outlined,
              );
            }
            return SingleChildScrollView(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  Row(
                    children: [
                      StoryRingAvatar(
                        displayName: profile.displayName,
                        imageUrl: profile.avatarUrl,
                        hasActiveStory: hasActiveStory,
                        size: 56,
                        onTap: hasActiveStory
                            ? () {
                                final stories = profileStoriesAsync.valueOrNull;
                                if (stories == null || stories.isEmpty) return;
                                Navigator.of(context).pop();
                                StoriesRoutes.openViewer(
                                  context,
                                  storyIds: stories.map((s) => s.id).toList(),
                                  profileId: profileId,
                                );
                              }
                            : null,
                      ),
                      const SizedBox(width: 16),
                      Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            ChatAuthorLabel(
                              displayName: profile.displayName,
                              verificationType: profile.verificationType,
                              style: Theme.of(context).textTheme.titleLarge,
                              verifiedBadgeSemanticLabel:
                                  switch (profile.verificationType) {
                                    'personal' => l10n.verifiedBadgePersonal,
                                    'organization' =>
                                      l10n.verifiedBadgeOrganization,
                                    _ => null,
                                  },
                            ),
                            Text(profile.handle),
                            const SizedBox(height: 4),
                            Row(
                              children: [
                                PresenceIndicator(
                                  key: onlineIndicatorKey,
                                  presence: presence,
                                ),
                                const SizedBox(width: 8),
                                Text(_presenceLabel(context, l10n, presence)),
                              ],
                            ),
                            if (presence?.customStatus case final status?
                                when status.isNotEmpty) ...[
                              const SizedBox(height: 4),
                              Text(
                                status,
                                maxLines: 2,
                                overflow: TextOverflow.ellipsis,
                                style: Theme.of(context).textTheme.bodySmall,
                              ),
                            ],
                            mmRatingAsync.when(
                              loading: () => const SizedBox.shrink(),
                              error: (error, stackTrace) =>
                                  const SizedBox.shrink(),
                              data: (rating) {
                                if (rating == null) {
                                  return const SizedBox.shrink();
                                }
                                return Padding(
                                  padding: const EdgeInsets.only(top: 4),
                                  child: Text(
                                    l10n.profileMmRating(
                                      rating.ratingValue.toStringAsFixed(1),
                                    ),
                                    style: Theme.of(
                                      context,
                                    ).textTheme.bodySmall,
                                  ),
                                );
                              },
                            ),
                          ],
                        ),
                      ),
                      if (!isGuest && profile.username.isNotEmpty && !isSelf)
                        VoiceShareLinkButton(
                          link: profileShareUrl(profile.username),
                          tooltip: l10n.shareLinkAction,
                        ),
                    ],
                  ),
                  if (showNotInContactsWarning) ...[
                    const SizedBox(height: 16),
                    Container(
                      key: const Key('profile_not_in_contacts_warning'),
                      padding: const EdgeInsets.all(12),
                      decoration: BoxDecoration(
                        color: Theme.of(
                          context,
                        ).colorScheme.surfaceContainerHighest,
                        borderRadius: BorderRadius.circular(12),
                      ),
                      child: Row(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Icon(
                            Icons.info_outline,
                            color: Theme.of(context).colorScheme.onSurface,
                            size: 20,
                          ),
                          const SizedBox(width: 8),
                          Expanded(
                            child: Text(l10n.profileNotInContactsWarning),
                          ),
                        ],
                      ),
                    ),
                  ],
                  if (profile.bio != null && profile.bio!.isNotEmpty) ...[
                    const SizedBox(height: 16),
                    Text(profile.bio!),
                  ],
                  HighlightsSection(profileId: profileId),
                  mmProfileAsync.when(
                    loading: () => const SizedBox.shrink(),
                    error: (error, stackTrace) => const SizedBox.shrink(),
                    data: (mmProfile) {
                      if (mmProfile.entries.isEmpty) {
                        return const SizedBox.shrink();
                      }
                      return Column(
                        crossAxisAlignment: CrossAxisAlignment.stretch,
                        children: [
                          const SizedBox(height: 16),
                          Text(
                            l10n.playerProfileSection,
                            style: Theme.of(context).textTheme.titleSmall,
                          ),
                          const SizedBox(height: 8),
                          for (final entry in mmProfile.entries)
                            Text(
                              _mmEntryLabel(catalogAsync, entry),
                              style: Theme.of(context).textTheme.bodyMedium,
                            ),
                        ],
                      );
                    },
                  ),
                  if (!isSelf) ...[
                    const SizedBox(height: 20),
                    if (isGuest)
                      OutlinedButton(
                        key: ProfileDetailSheet.messageKey,
                        onPressed: null,
                        child: Text(l10n.profileMessage),
                      )
                    else if (dmPermission != null)
                      dmPermission.when(
                        loading: () => const SizedBox.shrink(),
                        error: (error, stackTrace) => VoiceStatePanel(
                          title: l10n.chatListLoadError,
                          icon: Icons.cloud_off_outlined,
                          actionLabel: l10n.commonRetry,
                          onAction: () => ref.invalidate(
                            dmPermissionProvider(dmPermissionRequest!),
                          ),
                        ),
                        data: (allowed) => allowed
                            ? OutlinedButton(
                                key: ProfileDetailSheet.messageKey,
                                onPressed: () =>
                                    _openDm(context, ref, profileId),
                                child: Text(l10n.profileMessage),
                              )
                            : const SizedBox.shrink(),
                      ),
                    const SizedBox(height: 8),
                    _FriendActionButton(
                      profileId: profileId,
                      pendingOutgoing: pendingOutgoing,
                      pendingIncoming: pendingIncoming,
                      isFriend: isFriend,
                      isGuest: isGuest,
                    ),
                    if (isFriend) ...[
                      const SizedBox(height: 8),
                      _ProfileFavoriteAction(profileId: profileId),
                    ],
                    const SizedBox(height: 8),
                    TextButton(
                      key: ProfileDetailSheet.blockKey,
                      onPressed: () => _confirmBlock(context, ref, profile),
                      style: TextButton.styleFrom(
                        foregroundColor: Theme.of(context).colorScheme.error,
                      ),
                      child: Text(l10n.profileBlock),
                    ),
                    const SizedBox(height: 8),
                    TextButton(
                      key: const Key('profile_report'),
                      onPressed: () {
                        Navigator.of(context).pop();
                        ReportSheet.show(
                          context,
                          target: ReportUserTarget(profileId: profileId),
                        );
                      },
                      child: Text(l10n.reportAction),
                    ),
                  ],
                ],
              ),
            );
          },
        ),
      ),
    );
  }

  Future<void> _confirmBlock(
    BuildContext context,
    WidgetRef ref,
    VoiceProfile profile,
  ) async {
    final l10n = AppLocalizations.of(context)!;
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) {
        final dialogL10n = AppLocalizations.of(ctx)!;
        return AlertDialog(
          title: Text(dialogL10n.profileBlockConfirmTitle),
          content: Text(dialogL10n.profileBlockConfirmMessage),
          actions: [
            TextButton(
              onPressed: () => Navigator.of(ctx).pop(false),
              child: Text(dialogL10n.commonCancel),
            ),
            FilledButton(
              onPressed: () => Navigator.of(ctx).pop(true),
              child: Text(dialogL10n.profileBlock),
            ),
          ],
        );
      },
    );
    if (confirmed != true || !context.mounted) return;
    final err = await ref
        .read(socialActionsProvider)
        .blockAccount(profile.accountId, blockedProfileId: profile.id);
    if (!context.mounted) return;
    if (err != null) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(socialActionErrorMessage(l10n, err))),
      );
      return;
    }
    Navigator.of(context).pop();
  }

  Future<void> _openDm(
    BuildContext context,
    WidgetRef ref,
    String profileId,
  ) async {
    final l10n = AppLocalizations.of(context)!;
    final err = await ref
        .read(chatActionsProvider)
        .openDmWithProfile(profileId);
    if (!context.mounted) return;
    if (err != null) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(chatActionErrorMessage(l10n, err))),
      );
      return;
    }
    Navigator.of(context).pop();
  }

  String _presenceLabel(
    BuildContext context,
    AppLocalizations l10n,
    VoicePresence? presence,
  ) {
    if (presence == null) return l10n.socialPresenceUnknown;
    return switch (presence.status) {
      'online' => l10n.socialPresenceOnline,
      'idle' => l10n.socialPresenceIdle,
      'dnd' => l10n.socialPresenceDnd,
      _ =>
        presence.lastSeen == null
            ? l10n.socialPresenceOffline
            : l10n.socialPresenceLastSeen(
                _formatLastSeen(context, presence.lastSeen!),
              ),
    };
  }

  String _formatLastSeen(BuildContext context, DateTime lastSeen) {
    final local = lastSeen.toLocal();
    final material = MaterialLocalizations.of(context);
    final date = material.formatShortDate(local);
    final time = TimeOfDay.fromDateTime(local).format(context);
    return '$date $time';
  }
}

class _ProfileFavoriteAction extends ConsumerStatefulWidget {
  const _ProfileFavoriteAction({required this.profileId});

  final String profileId;

  @override
  ConsumerState<_ProfileFavoriteAction> createState() =>
      _ProfileFavoriteActionState();
}

class _ProfileFavoriteActionState
    extends ConsumerState<_ProfileFavoriteAction> {
  static const _buttonKey = Key('profile_favorite_toggle');

  final FocusNode _focusNode = FocusNode(debugLabel: 'profile-favorite-action');
  bool _busy = false;
  int _contextGeneration = 0;
  String? _authorization;
  String? _activeProfileId;

  @override
  void didUpdateWidget(covariant _ProfileFavoriteAction oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.profileId != widget.profileId) {
      _contextGeneration++;
      _busy = false;
    }
  }

  @override
  void dispose() {
    _focusNode.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final auth = ref.watch(authControllerProvider);
    final authorization = auth.session?.authorizationHeader;
    if (_authorization != authorization ||
        _activeProfileId != auth.activeProfileId) {
      _authorization = authorization;
      _activeProfileId = auth.activeProfileId;
      _contextGeneration++;
      _busy = false;
    }

    final favorites = ref.watch(favoritesListProvider);
    if (favorites.isLoading) {
      return const Center(
        child: SizedBox(
          width: 24,
          height: 24,
          child: CircularProgressIndicator(),
        ),
      );
    }
    if (favorites.hasError) {
      return VoiceStatePanel(
        title: socialListErrorMessage(l10n, favorites.error!),
        icon: Icons.cloud_off_outlined,
        actionLabel: l10n.commonRetry,
        onAction: () => ref.invalidate(favoritesListProvider),
      );
    }

    final isFavorite = favorites.requireValue.favorites.contains(
      widget.profileId,
    );
    final label = isFavorite
        ? l10n.socialRemoveFavorite
        : l10n.socialAddFavorite;
    return Tooltip(
      message: label,
      child: OutlinedButton.icon(
        key: _buttonKey,
        focusNode: _focusNode,
        onPressed: _busy ? null : () => _toggle(isFavorite),
        icon: Icon(isFavorite ? Icons.star : Icons.star_border),
        label: Text(label),
      ),
    );
  }

  Future<void> _toggle(bool isFavorite) async {
    final l10n = AppLocalizations.of(context)!;
    final generation = _contextGeneration;
    final authorization = ref.read(authorizationHeaderProvider);
    final activeProfileId = ref.read(authControllerProvider).activeProfileId;
    final messenger = ScaffoldMessenger.of(context);
    setState(() => _busy = true);

    String? error;
    try {
      error = await ref
          .read(socialActionsProvider)
          .setFavorite(widget.profileId, !isFavorite);
    } catch (_) {
      error = 'unknown';
    }
    if (!mounted || generation != _contextGeneration) return;
    final currentAuth = ref.read(authControllerProvider);
    if (authorization != ref.read(authorizationHeaderProvider) ||
        activeProfileId != currentAuth.activeProfileId) {
      return;
    }

    setState(() => _busy = false);
    if (error != null) {
      messenger.showSnackBar(
        SnackBar(content: Text(socialActionErrorMessage(l10n, error))),
      );
    }
  }
}

class _FriendActionButton extends ConsumerStatefulWidget {
  const _FriendActionButton({
    required this.profileId,
    required this.pendingOutgoing,
    required this.pendingIncoming,
    required this.isFriend,
    required this.isGuest,
  });

  final String profileId;
  final bool pendingOutgoing;
  final bool pendingIncoming;
  final bool isFriend;
  final bool isGuest;

  @override
  ConsumerState<_FriendActionButton> createState() =>
      _FriendActionButtonState();
}

class _FriendActionButtonState extends ConsumerState<_FriendActionButton> {
  var _busy = false;
  String? _error;

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    if (widget.pendingIncoming) {
      return Row(
        children: [
          Expanded(
            child: FilledButton(
              onPressed: _busy ? null : () => _accept(l10n),
              child: Text(l10n.socialAcceptRequest),
            ),
          ),
          const SizedBox(width: 8),
          Expanded(
            child: OutlinedButton(
              onPressed: _busy ? null : () => _decline(l10n),
              child: Text(l10n.socialDeclineRequest),
            ),
          ),
        ],
      );
    }
    if (widget.pendingOutgoing) {
      return Text(l10n.socialRequestPending);
    }
    if (widget.isFriend) {
      return Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          OutlinedButton(
            key: ProfileDetailSheet.removeFriendKey,
            onPressed: _busy ? null : _removeFriend,
            child: Text(l10n.socialRemoveFriend),
          ),
          if (_error != null) ...[
            const SizedBox(height: 8),
            Text(
              socialActionErrorMessage(l10n, _error!),
              style: TextStyle(color: Theme.of(context).colorScheme.error),
            ),
          ],
        ],
      );
    }
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        FilledButton(
          key: ProfileDetailSheet.addFriendKey,
          onPressed: widget.isGuest || _busy ? null : _sendRequest,
          child: Text(l10n.socialAddFriend),
        ),
        if (_error != null) ...[
          const SizedBox(height: 8),
          Text(
            socialActionErrorMessage(l10n, _error!),
            style: TextStyle(color: Theme.of(context).colorScheme.error),
          ),
        ],
      ],
    );
  }

  Future<void> _sendRequest() async {
    setState(() {
      _busy = true;
      _error = null;
    });
    final err = await ref
        .read(socialActionsProvider)
        .sendFriendInvitation(widget.profileId);
    if (!mounted) return;
    setState(() {
      _busy = false;
      _error = err;
    });
    if (err == null) Navigator.of(context).pop();
  }

  Future<void> _removeFriend() async {
    setState(() {
      _busy = true;
      _error = null;
    });
    final err = await ref
        .read(socialActionsProvider)
        .removeFriend(widget.profileId);
    if (!mounted) return;
    setState(() {
      _busy = false;
      _error = err;
    });
    if (err == null) Navigator.of(context).pop();
  }

  Future<void> _accept(AppLocalizations l10n) async {
    setState(() {
      _busy = true;
      _error = null;
    });
    final err = await ref
        .read(socialActionsProvider)
        .acceptFriendInvitation(widget.profileId);
    if (!mounted) return;
    setState(() => _busy = false);
    if (err != null) {
      setState(() => _error = err);
    } else {
      Navigator.of(context).pop();
    }
  }

  Future<void> _decline(AppLocalizations l10n) async {
    setState(() {
      _busy = true;
      _error = null;
    });
    final err = await ref
        .read(socialActionsProvider)
        .declineFriendInvitation(widget.profileId);
    if (!mounted) return;
    setState(() => _busy = false);
    if (err != null) {
      setState(() => _error = err);
    } else {
      Navigator.of(context).pop();
    }
  }
}
