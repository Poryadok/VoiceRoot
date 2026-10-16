import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/api_errors.dart';
import '../../backend/friends_client.dart';
import '../../backend/matchmaking_client.dart';
import '../../backend/users_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/chat_providers.dart';
import '../../state/call_providers.dart';
import '../../state/contact_profile_providers.dart';
import '../../state/dm_permission_provider.dart';
import '../../state/gateway_providers.dart';
import '../../state/shell_providers.dart';
import '../../state/space_providers.dart';
import '../../state/presence_providers.dart';
import '../../state/matchmaking_providers.dart';
import '../../state/social_providers.dart';
import '../../state/stories_providers.dart';
import '../../theme/voice_layout.dart';
import '../../theme/voice_metrics.dart';
import '../../routing/deep_link_urls.dart';
import '../api_error_messages.dart';
import '../core/chat_author_label.dart';
import '../core/voice_avatar.dart';
import '../core/voice_share_link.dart';
import '../core/voice_skeleton.dart';
import '../core/voice_state_panel.dart';
import '../report/report_sheet.dart';
import '../stories/highlights_section.dart';
import '../stories/story_ring_avatar.dart';
import '../../routing/stories_routes.dart';
import 'presence_indicator.dart';
import 'profile_avatar_viewer.dart';

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
  static const Key avatarKey = Key('profile_avatar');
  static const Key callKey = Key('profile_call');
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
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            _ProfileHeader(
              profile: profileAsync.when(
                loading: () => null,
                error: (error, stackTrace) => null,
                data: (profile) => profile,
              ),
              isGuest: isGuest,
              isSelf: isSelf,
              onBlock: (profile) => _confirmBlock(context, ref, profile),
              onReport: () => _openReport(context),
            ),
            Flexible(
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
                  final bannerUrl = profile.bannerUrl;
                  final hasBanner = bannerUrl != null && bannerUrl.isNotEmpty;
                  final avatar = Semantics(
                    key: avatarKey,
                    label: hasActiveStory ? null : l10n.profileOpenAvatar,
                    button: hasActiveStory ? null : true,
                    child: StoryRingAvatar(
                      displayName: profile.displayName,
                      imageUrl: profile.avatarUrl,
                      hasActiveStory: hasActiveStory,
                      size: hasBanner
                          ? context.voiceMetrics.layoutSize('avatarLg', fallback: 80)
                          : 56,
                      onTap: hasActiveStory
                          ? () {
                              final stories = profileStoriesAsync.valueOrNull;
                              if (stories == null || stories.isEmpty) {
                                return;
                              }
                              Navigator.of(context).pop();
                              StoriesRoutes.openViewer(
                                context,
                                storyIds: stories.map((s) => s.id).toList(),
                                profileId: profileId,
                              );
                            }
                          : () => Navigator.of(context).push<void>(
                              MaterialPageRoute(
                                builder: (_) => ProfileAvatarViewer(
                                  profileId: profileId,
                                ),
                              ),
                            ),
                    ),
                  );
                  final messageAction = isGuest
                      ? OutlinedButton(
                          key: ProfileDetailSheet.messageKey,
                          onPressed: null,
                          child: Text(l10n.profileMessage),
                        )
                      : dmPermission?.when<Widget?>(
                          loading: () => null,
                          error: (error, stackTrace) => SizedBox(
                            width: double.infinity,
                            child: VoiceStatePanel(
                              title: l10n.chatListLoadError,
                              icon: Icons.cloud_off_outlined,
                              actionLabel: l10n.commonRetry,
                              onAction: () => ref.invalidate(
                                dmPermissionProvider(dmPermissionRequest!),
                              ),
                            ),
                          ),
                          data: (allowed) => allowed
                              ? OutlinedButton(
                                  key: ProfileDetailSheet.messageKey,
                                  onPressed: () =>
                                      _openDm(context, ref, profileId),
                                  child: Text(l10n.profileMessage),
                                )
                              : null,
                        );
                  return SingleChildScrollView(
                    child: Column(
                      mainAxisSize: MainAxisSize.min,
                      crossAxisAlignment: CrossAxisAlignment.stretch,
                      children: [
                        if (bannerUrl != null && bannerUrl.isNotEmpty) ...[
                          _ProfileBannerHeader(bannerUrl: bannerUrl, avatar: avatar),
                          SizedBox(
                            height: context.voiceMetrics.spacing('12', fallback: 12),
                          ),
                        ],
                        Row(
                          children: [
                            if (!hasBanner) ...[
                              avatar,
                              const SizedBox(width: 16),
                            ],
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
                                      Flexible(
                                        child: Text(_presenceLabel(context, l10n, presence)),
                                      ),
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
                        if (!isSelf) ...[
                          const SizedBox(height: 20),
                          Wrap(
                            key: const Key('profile_communication_actions'),
                            spacing: context.voiceMetrics.spacing('12', fallback: 12),
                            runSpacing: context.voiceMetrics.spacing('8', fallback: 8),
                            children: [
                              ?messageAction,
                              _ProfileCallAction(
                                key: ValueKey((profileId, session, isProfileSwitching)),
                                profileId: profileId,
                              ),
                              if (isFriend)
                                _ProfileFavoriteAction(profileId: profileId),
                            ],
                          ),
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
                        if (!isGuest && !isSelf)
                          _ProfileCommonSpaces(profileId: profileId),
                        if (!isSelf) ...[
                          const SizedBox(height: 20),
                          _FriendActionButton(
                            profileId: profileId,
                            pendingOutgoing: pendingOutgoing,
                            pendingIncoming: pendingIncoming,
                            isFriend: isFriend,
                            isGuest: isGuest,
                          ),
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
                            onPressed: () => _openReport(context),
                            child: Text(l10n.reportAction),
                          ),
                        ],
                      ],
                    ),
                  );
                },
              ),
            ),
          ],
        ),
      ),
    );
  }

  void _openReport(BuildContext context) {
    Navigator.of(context).pop();
    ReportSheet.show(
      context,
      target: ReportUserTarget(profileId: profileId),
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
            : profileLastSeenLabel(context, presence.lastSeen!),
    };
  }

}

/// Formats an already viewer-filtered timestamp; [now] permits boundary tests.
String profileLastSeenLabel(BuildContext context, DateTime lastSeen, {DateTime? now}) {
  final l10n = AppLocalizations.of(context)!;
  final age = (now ?? DateTime.now()).difference(lastSeen);
  if (age < const Duration(minutes: 1)) return l10n.socialPresenceLastSeenJustNow;
  if (age < const Duration(hours: 1)) return l10n.socialPresenceLastSeenMinutes(age.inMinutes);
  if (age < const Duration(days: 1)) return l10n.socialPresenceLastSeenHours(age.inHours);
  if (age < const Duration(days: 7)) return l10n.socialPresenceLastSeenDays(age.inDays);
  // The canon leaves day 7 unspecified. Preserve the existing absolute date/time
  // fallback for >=7 days rather than define another product bucket here.
  final local = lastSeen.toLocal();
  final date = MaterialLocalizations.of(context).formatShortDate(local);
  final time = TimeOfDay.fromDateTime(local).format(context);
  return l10n.socialPresenceLastSeen('$date $time');
}

enum _ProfileMenuAction { block, report }

class _ProfileHeader extends StatelessWidget {
  const _ProfileHeader({
    required this.profile,
    required this.isGuest,
    required this.isSelf,
    required this.onBlock,
    required this.onReport,
  });

  final VoiceProfile? profile;
  final bool isGuest;
  final bool isSelf;
  final ValueChanged<VoiceProfile> onBlock;
  final VoidCallback onReport;

  @override
  Widget build(BuildContext context) {
    final narrow = VoiceLayout.isNarrow(MediaQuery.sizeOf(context).width);
    final l10n = AppLocalizations.of(context)!;
    final target = profile;
    final dismiss = FocusTraversalOrder(
      order: const NumericFocusOrder(0),
      child: narrow
          ? const BackButton(key: Key('profile_dismiss'))
          : const CloseButton(key: Key('profile_dismiss')),
    );
    return SizedBox(
      key: const Key('profile_header_controls'),
      height: context.voiceMetrics.layoutSize('headerHeight', fallback: 56),
      child: IconTheme.merge(
        data: IconThemeData(
          size: context.voiceMetrics.layoutSize('iconMd', fallback: 20),
        ),
        child: FocusTraversalGroup(
          policy: OrderedTraversalPolicy(),
          child: Row(
            children: [
              if (narrow) dismiss,
              Expanded(
                child: Semantics(
                  key: const Key('profile_header_title'),
                  header: true,
                  child: Text(
                    l10n.profileDetailTitle,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: Theme.of(context).textTheme.titleMedium,
                  ),
                ),
              ),
              if (target != null && !isSelf) ...[
                if (!isGuest && target.username.isNotEmpty)
                  FocusTraversalOrder(
                    order: const NumericFocusOrder(1),
                    child: VoiceShareLinkButton(
                      link: profileShareUrl(target.username),
                      tooltip: l10n.shareLinkAction,
                    ),
                  ),
                FocusTraversalOrder(
                  order: const NumericFocusOrder(2),
                  child: PopupMenuButton<_ProfileMenuAction>(
                    key: const Key('profile_more'),
                    tooltip: l10n.profileMoreActions,
                    icon: const Icon(Icons.more_vert),
                    onSelected: (action) {
                      switch (action) {
                        case _ProfileMenuAction.block:
                          onBlock(target);
                        case _ProfileMenuAction.report:
                          onReport();
                      }
                    },
                    itemBuilder: (context) => [
                      PopupMenuItem(
                        key: const Key('profile_more_block'),
                        value: _ProfileMenuAction.block,
                        child: Text(l10n.profileBlock),
                      ),
                      PopupMenuItem(
                        key: const Key('profile_more_report'),
                        value: _ProfileMenuAction.report,
                        child: Text(l10n.reportAction),
                      ),
                    ],
                  ),
                ),
              ],
              if (!narrow) dismiss,
            ],
          ),
        ),
      ),
    );
  }
}

class _ProfileBannerHeader extends StatelessWidget {
  const _ProfileBannerHeader({required this.bannerUrl, required this.avatar});

  final String bannerUrl;
  final Widget avatar;

  @override
  Widget build(BuildContext context) {
    final metrics = context.voiceMetrics;
    final narrow = VoiceLayout.isNarrow(MediaQuery.sizeOf(context).width);
    final avatarSize = metrics.layoutSize('avatarLg', fallback: 80);
    final overlap = metrics.spacing('32', fallback: 32);
    final bannerHeight =
        avatarSize + (narrow ? overlap : metrics.spacing('16', fallback: 16));
    final surface = Theme.of(context).colorScheme.surface;
    return SizedBox(
      height: bannerHeight + overlap,
      child: Stack(
        clipBehavior: Clip.none,
        children: [
          Positioned(
            left: 0,
            right: 0,
            top: 0,
            height: bannerHeight,
            child: IgnorePointer(
              child: ExcludeSemantics(
                child: ClipRRect(
                  key: const Key('profile_banner'),
                  borderRadius: BorderRadius.vertical(
                    top: Radius.circular(
                      narrow ? 0 : metrics.corner('lg', fallback: 8),
                    ),
                  ),
                  child: ColoredBox(
                    color: Theme.of(context).colorScheme.surfaceContainerHighest,
                    child: Image.network(
                      bannerUrl,
                      key: ValueKey(bannerUrl),
                      fit: BoxFit.cover,
                      width: double.infinity,
                      height: bannerHeight,
                      errorBuilder: (context, error, stackTrace) =>
                          const SizedBox.expand(
                            key: Key('profile_banner_unavailable'),
                          ),
                    ),
                  ),
                ),
              ),
            ),
          ),
          Positioned(
            left: metrics.spacing('20', fallback: 20),
            top: bannerHeight + overlap - avatarSize,
            child: DecoratedBox(
              decoration: BoxDecoration(
                shape: BoxShape.circle,
                border: Border.all(
                  color: surface,
                  width: metrics.spacing('4', fallback: 4),
                  strokeAlign: BorderSide.strokeAlignOutside,
                ),
              ),
              child: avatar,
            ),
          ),
        ],
      ),
    );
  }
}

class _ProfileCommonSpaces extends ConsumerWidget {
  const _ProfileCommonSpaces({required this.profileId});

  final String profileId;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final session = ref.watch(authControllerProvider).session;
    final switching = ref.watch(profileSwitchInProgressProvider);
    if (switching) return const SizedBox.shrink();
    return ref
        .watch(contactCommonSpacesProvider(profileId))
        .when(
          skipLoadingOnRefresh: false,
          loading: () => const Padding(
            padding: EdgeInsets.only(top: 16),
            child: Center(child: CircularProgressIndicator()),
          ),
          error: (_, _) => VoiceStatePanel(
            key: const Key('profile_common_spaces_error'),
            title: l10n.profileCommonSpacesLoadError,
            icon: Icons.cloud_off_outlined,
            actionLabel: l10n.commonRetry,
            onAction: () =>
                ref.invalidate(contactCommonSpacesProvider(profileId)),
          ),
          data: (spaces) => spaces.isEmpty
              ? const SizedBox.shrink()
              : Column(
                  key: const Key('profile_common_spaces'),
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    const SizedBox(height: 16),
                    Text(
                      l10n.profileCommonSpaces,
                      style: Theme.of(context).textTheme.titleSmall,
                    ),
                    for (final space in spaces)
                      ListTile(
                        key: ValueKey('profile_common_space_${space.id}'),
                        contentPadding: EdgeInsets.zero,
                        leading: VoiceAvatar(
                          imageUrl: space.iconUrl,
                          label: space.name,
                        ),
                        title: Text(space.name),
                        trailing: const Icon(Icons.chevron_right),
                        onTap: () {
                          final current = ref
                              .read(authControllerProvider)
                              .session;
                          if (session == null ||
                              current?.accountId != session.accountId ||
                              current?.activeProfileId !=
                                  session.activeProfileId ||
                              current?.authorizationHeader !=
                                  session.authorizationHeader ||
                              ref.read(profileSwitchInProgressProvider)) {
                            return;
                          }
                          if (ref.read(selectedSpaceIdProvider) != space.id) {
                            ref
                                .read(shellNavigationProvider)
                                .selectSpace(space.id);
                          }
                          Navigator.of(context).pop();
                        },
                      ),
                  ],
                ),
        );
  }
}

class _ProfileCallAction extends ConsumerStatefulWidget {
  const _ProfileCallAction({super.key, required this.profileId});

  final String profileId;

  @override
  ConsumerState<_ProfileCallAction> createState() => _ProfileCallActionState();
}

class _ProfileCallActionState extends ConsumerState<_ProfileCallAction> {
  bool _busy = false;

  @override
  Widget build(BuildContext context) {
    final auth = ref.watch(authControllerProvider);
    final enabled =
        !_busy &&
        !auth.isGuest &&
        !auth.isRestoring &&
        auth.session != null &&
        !ref.watch(profileSwitchInProgressProvider) &&
        ref.watch(gatewayConfigProvider).canPlaceVoiceCalls;
    return OutlinedButton.icon(
      key: ProfileDetailSheet.callKey,
      onPressed: enabled ? _startCall : null,
      icon: const Icon(Icons.call_outlined),
      label: Text(AppLocalizations.of(context)!.callStartAudio),
    );
  }

  Future<void> _startCall() async {
    final session = ref.read(authControllerProvider).session;
    if (session == null || _busy) return;
    final l10n = AppLocalizations.of(context)!;
    setState(() => _busy = true);
    String? error;
    try {
      error = await ref
          .read(chatActionsProvider)
          .openDmWithProfile(widget.profileId);
    } catch (_) {
      error = 'unknown';
    }
    if (!mounted) return;
    final current = ref.read(authControllerProvider).session;
    if (current?.accountId != session.accountId ||
        current?.activeProfileId != session.activeProfileId ||
        current?.authorizationHeader != session.authorizationHeader ||
        ref.read(profileSwitchInProgressProvider)) {
      return;
    }
    setState(() => _busy = false);
    if (error != null) {
      if (error != kChatActionStaleContext) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(chatActionErrorMessage(l10n, error))),
        );
      }
      return;
    }
    final chatId = ref.read(selectedChatIdProvider);
    if (chatId == null ||
        ref.read(dmPeerProfileByChatIdProvider)[chatId] != widget.profileId) {
      return;
    }
    // The existing controller owns privacy errors, conflicts and call overlays.
    unawaited(
      ref
          .read(callControllerProvider.notifier)
          .startCall(chatId: chatId, calleeProfileId: widget.profileId),
    );
    Navigator.of(context).pop();
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
    if (_busy || widget.isGuest || !widget.isFriend) return;
    final controller = ref.read(authControllerProvider.notifier);
    final generation = controller.sessionInstallGeneration;
    final session = ref.read(authControllerProvider).session;
    if (session == null || ref.read(profileSwitchInProgressProvider)) return;
    final profileId = widget.profileId;
    bool isCurrent() {
      if (!mounted || widget.profileId != profileId || !widget.isFriend) {
        return false;
      }
      final current = ref.read(authControllerProvider).session;
      return !widget.isGuest &&
          identical(ref.read(authControllerProvider.notifier), controller) &&
          controller.sessionInstallGeneration == generation &&
          !ref.read(profileSwitchInProgressProvider) &&
          current?.accountId == session.accountId &&
          current?.activeProfileId == session.activeProfileId &&
          current?.authorizationHeader == session.authorizationHeader;
    }

    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      final confirmed = await showDialog<bool>(
        context: context,
        builder: (dialogContext) {
          final l10n = AppLocalizations.of(dialogContext)!;
          return AlertDialog(
            key: const Key('profile_remove_friend_dialog'),
            title: Text(l10n.profileRemoveFriendConfirmTitle),
            content: Text(l10n.profileRemoveFriendConfirmMessage),
            actions: [
              TextButton(
                key: const Key('profile_remove_friend_cancel'),
                autofocus: true,
                onPressed: () => Navigator.of(dialogContext).pop(false),
                child: Text(l10n.commonCancel),
              ),
              FilledButton(
                key: const Key('profile_remove_friend_confirm'),
                onPressed: () => Navigator.of(dialogContext).pop(true),
                child: Text(l10n.socialRemoveFriend),
              ),
            ],
          );
        },
      );
      if (confirmed != true || !mounted || !isCurrent()) return;
      String? error;
      try {
        error = await ref.read(socialActionsProvider).removeFriend(profileId);
      } catch (_) {
        error = 'unknown';
      }
      if (!mounted || !isCurrent()) return;
      setState(() => _error = error);
      if (error == null) Navigator.of(context).pop();
    } finally {
      if (mounted) setState(() => _busy = false);
    }
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
