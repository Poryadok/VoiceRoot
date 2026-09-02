import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/users_client.dart';
import '../../l10n/app_localizations.dart';
import '../api_error_messages.dart';
import '../../state/presence_providers.dart';
import '../../state/social_providers.dart';
import '../../state/stories_providers.dart';
import '../core/voice_skeleton.dart';
import '../core/voice_state_panel.dart';
import '../onboarding/onboarding_anchor_keys.dart';
import '../matchmaking/game_catalog_screen.dart';
import '../matchmaking/match_history_screen.dart';
import '../stories/story_ring_avatar.dart';
import '../../routing/stories_routes.dart';
import 'presence_indicator.dart';
import 'profile_detail_sheet.dart';

/// Friends, search, and friend-requests UI (app stack social column).
class SocialPanel extends ConsumerStatefulWidget {
  const SocialPanel({super.key, this.initialTabIndex = 0});

  static const Key panelKey = Key('social_panel');
  static const Key tabSearchKey = Key('social_tab_search');
  static const Key tabFriendsKey = Key('social_tab_friends');
  static const Key tabContactsKey = Key('social_tab_contacts');
  static const Key tabFavoritesKey = Key('social_tab_favorites');
  static const Key tabRequestsKey = Key('social_tab_requests');
  static const Key searchFieldKey = Key('social_search_field');
  static const Key searchSubmitKey = Key('social_search_submit');
  static const Key friendsListKey = Key('social_friends_list');
  static const Key contactsListKey = Key('social_contacts_list');
  static const Key favoritesListKey = Key('social_favorites_list');
  static const Key friendsUnavailableKey = Key('social_friends_unavailable');
  static const Key contactsUnavailableKey = Key('social_contacts_unavailable');
  static const Key favoritesUnavailableKey = Key('social_favorites_unavailable');
  static const Key requestsUnavailableKey = Key('social_requests_unavailable');
  static const Key searchUnavailableKey = Key('social_search_unavailable');
  static const Key searchLoadingKey = Key('social_search_loading');

  static Key requestAcceptKey(String profileId) =>
      Key('social_request_accept_$profileId');

  static Key requestDeclineKey(String profileId) =>
      Key('social_request_decline_$profileId');

  static Key profileTileKey(String profileId) =>
      Key('social_profile_tile_$profileId');

  static Key favoriteToggleKey(String profileId) =>
      Key('social_favorite_toggle_$profileId');

  final int initialTabIndex;

  @override
  ConsumerState<SocialPanel> createState() => _SocialPanelState();
}

class _SocialPanelState extends ConsumerState<SocialPanel>
    with SingleTickerProviderStateMixin {
  late final TabController _tabs;
  final _searchController = TextEditingController();

  @override
  void initState() {
    super.initState();
    _tabs = TabController(
      length: 5,
      vsync: this,
      initialIndex: widget.initialTabIndex,
    );
  }

  @override
  void dispose() {
    _tabs.dispose();
    _searchController.dispose();
    super.dispose();
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
    ref.watch(friendsPresenceSyncProvider);
    ref.watch(storyFeedProvider);
    final l10n = AppLocalizations.of(context)!;

    return Stack(
      key: SocialPanel.panelKey,
      children: [
        Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Align(
              alignment: Alignment.centerLeft,
              child: Wrap(
                spacing: 4,
                children: [
                  TextButton.icon(
                    key: OnboardingAnchorKeys.matchmaking,
                    onPressed: () {
                      Navigator.of(context).push(
                        MaterialPageRoute<void>(
                          builder: (_) => const GameCatalogScreen(),
                        ),
                      );
                    },
                    icon: const Icon(Icons.sports_esports_outlined),
                    label: Text(l10n.gameCatalogEntry),
                  ),
                  TextButton.icon(
                    key: const Key('social_match_history_entry'),
                    onPressed: () {
                      Navigator.of(context).push(
                        MaterialPageRoute<void>(
                          builder: (_) => const MatchHistoryScreen(),
                        ),
                      );
                    },
                    icon: const Icon(Icons.history),
                    label: Text(l10n.matchHistoryEntry),
                  ),
                ],
              ),
            ),
            TabBar(
              controller: _tabs,
              isScrollable: true,
              tabs: [
                Tab(key: SocialPanel.tabSearchKey, text: l10n.socialTabSearch),
                Tab(key: SocialPanel.tabFriendsKey, text: l10n.socialTabFriends),
                Tab(
                  key: SocialPanel.tabContactsKey,
                  text: l10n.socialTabContacts,
                ),
                Tab(
                  key: SocialPanel.tabFavoritesKey,
                  text: l10n.socialTabFavorites,
                ),
                Tab(
                  key: SocialPanel.tabRequestsKey,
                  text: l10n.socialTabRequests,
                ),
              ],
            ),
            Expanded(
              child: TabBarView(
                controller: _tabs,
                children: [
                  _SearchTab(
                    controller: _searchController,
                    onOpenProfile: _openProfile,
                  ),
                  _FriendsTab(onOpenProfile: _openProfile),
                  _ContactsTab(onOpenProfile: _openProfile),
                  _FavoritesTab(onOpenProfile: _openProfile),
                  _RequestsTab(onOpenProfile: _openProfile),
                ],
              ),
            ),
          ],
        ),
        Positioned(
          right: 16,
          bottom: 16,
          child: FloatingActionButton.extended(
            key: const Key('social_story_create_fab'),
            onPressed: () => StoriesRoutes.openCreate(context),
            icon: const Icon(Icons.add),
            label: Text(l10n.socialStoryCreate),
          ),
        ),
      ],
    );
  }
}

class _SearchTab extends ConsumerWidget {
  const _SearchTab({required this.controller, required this.onOpenProfile});

  final TextEditingController controller;
  final void Function(String profileId) onOpenProfile;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final search = ref.watch(searchProfilesControllerProvider);

    return Column(
      children: [
        Padding(
          padding: const EdgeInsets.all(8),
          child: Row(
            children: [
              Expanded(
                child: TextField(
                  key: SocialPanel.searchFieldKey,
                  controller: controller,
                  decoration: InputDecoration(
                    hintText: l10n.socialSearchHint,
                    isDense: true,
                  ),
                  onSubmitted: (q) => ref
                      .read(searchProfilesControllerProvider.notifier)
                      .search(q),
                ),
              ),
              const SizedBox(width: 8),
              IconButton(
                key: SocialPanel.searchSubmitKey,
                icon: const Icon(Icons.search),
                onPressed: search.isLoading
                    ? null
                    : () => ref
                          .read(searchProfilesControllerProvider.notifier)
                          .search(controller.text),
              ),
            ],
          ),
        ),
        Expanded(
          child: _SearchResults(state: search, onOpenProfile: onOpenProfile),
        ),
      ],
    );
  }
}

class _SearchResults extends ConsumerWidget {
  const _SearchResults({required this.state, required this.onOpenProfile});

  final SearchProfilesState state;
  final void Function(String profileId) onOpenProfile;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    if (state.isLoading) {
      return KeyedSubtree(
        key: SocialPanel.searchLoadingKey,
        child: const VoiceListSkeleton(rowCount: 4),
      );
    }
    if (state.errorMessage != null) {
      final query = state.lastQuery ?? '';
      return KeyedSubtree(
        key: SocialPanel.searchUnavailableKey,
        child: VoiceStatePanel(
          title: socialActionErrorMessage(
            l10n,
            state.errorMessage!,
            statusCode: state.errorStatusCode,
          ),
          icon: Icons.cloud_off_outlined,
          actionLabel: query.isEmpty ? null : l10n.commonRetry,
          onAction: query.isEmpty
              ? null
              : () => ref
                    .read(searchProfilesControllerProvider.notifier)
                    .search(query),
        ),
      );
    }

    final query = state.lastQuery ?? '';
    if (state.results.isEmpty) {
      if (query.isEmpty) {
        return VoiceStatePanel(
          title: l10n.socialSearchStart,
          message: l10n.socialSearchStartHint,
          icon: Icons.search,
        );
      }
      return VoiceStatePanel(
        title: l10n.socialSearchEmpty,
        message: l10n.socialSearchEmptyHint,
        icon: Icons.person_search_outlined,
      );
    }

    return ListView.builder(
      itemCount: state.results.length,
      itemBuilder: (context, index) {
        final profile = state.results[index];
        return _ProfileListTile(
          key: SocialPanel.profileTileKey(profile.id),
          profile: profile,
          onTap: () => onOpenProfile(profile.id),
        );
      },
    );
  }
}

class _FriendsTab extends ConsumerWidget {
  const _FriendsTab({required this.onOpenProfile});

  final void Function(String profileId) onOpenProfile;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final friendsAsync = ref.watch(friendsListProvider);
    final actions = ref.read(socialActionsProvider);

    return friendsAsync.when(
      loading: () => const VoiceListSkeleton(),
      error: (e, st) => Center(
        child: VoiceStatePanel(
          key: SocialPanel.friendsUnavailableKey,
          title: socialListErrorMessage(l10n, e),
          icon: Icons.cloud_off_outlined,
          actionLabel: l10n.commonRetry,
          onAction: () => ref.invalidate(friendsListProvider),
        ),
      ),
      data: (data) {
        final ids = data.friends;
        if (ids.isEmpty) {
          return VoiceStatePanel(
            title: l10n.socialFriendsEmpty,
            icon: Icons.people_outline,
          );
        }
        return ListView.builder(
          key: SocialPanel.friendsListKey,
          itemCount: ids.length,
          itemBuilder: (context, index) {
            final profileId = ids[index];
            return _ProfileIdTile(
              profileId: profileId,
              onTap: () => onOpenProfile(profileId),
              trailing: _FavoriteToggleButton(
                profileId: profileId,
                onToggle: (favorite) =>
                    actions.setFavorite(profileId, favorite),
              ),
            );
          },
        );
      },
    );
  }
}

class _ContactsTab extends ConsumerWidget {
  const _ContactsTab({required this.onOpenProfile});

  final void Function(String profileId) onOpenProfile;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final contactsAsync = ref.watch(contactsListProvider);
    final actions = ref.read(socialActionsProvider);

    return contactsAsync.when(
      loading: () => const VoiceListSkeleton(),
      error: (e, st) => Center(
        child: VoiceStatePanel(
          key: SocialPanel.contactsUnavailableKey,
          title: socialListErrorMessage(l10n, e),
          icon: Icons.cloud_off_outlined,
          actionLabel: l10n.commonRetry,
          onAction: () => ref.invalidate(contactsListProvider),
        ),
      ),
      data: (data) {
        if (data.contacts.isEmpty) {
          return VoiceStatePanel(
            title: l10n.socialContactsEmpty,
            message: l10n.socialContactsEmptyHint,
            icon: Icons.contacts_outlined,
          );
        }
        return ListView.builder(
          key: SocialPanel.contactsListKey,
          itemCount: data.contacts.length,
          itemBuilder: (context, index) {
            final contact = data.contacts[index];
            return _ProfileIdTile(
              profileId: contact.profileId,
              subtitle: contact.source.isEmpty ? null : contact.source,
              onTap: () => onOpenProfile(contact.profileId),
              trailing: _FavoriteToggleButton(
                profileId: contact.profileId,
                initialFavorite: contact.isFavorite,
                onToggle: (favorite) =>
                    actions.setFavorite(contact.profileId, favorite),
              ),
            );
          },
        );
      },
    );
  }
}

class _FavoritesTab extends ConsumerWidget {
  const _FavoritesTab({required this.onOpenProfile});

  final void Function(String profileId) onOpenProfile;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final favoritesAsync = ref.watch(favoritesListProvider);
    final actions = ref.read(socialActionsProvider);

    return favoritesAsync.when(
      loading: () => const VoiceListSkeleton(),
      error: (e, st) => Center(
        child: VoiceStatePanel(
          key: SocialPanel.favoritesUnavailableKey,
          title: socialListErrorMessage(l10n, e),
          icon: Icons.cloud_off_outlined,
          actionLabel: l10n.commonRetry,
          onAction: () => ref.invalidate(favoritesListProvider),
        ),
      ),
      data: (data) {
        final ids = data.favorites;
        if (ids.isEmpty) {
          return VoiceStatePanel(
            title: l10n.socialFavoritesEmpty,
            message: l10n.socialFavoritesEmptyHint,
            icon: Icons.star_outline,
          );
        }
        return ListView.builder(
          key: SocialPanel.favoritesListKey,
          itemCount: ids.length,
          itemBuilder: (context, index) {
            final profileId = ids[index];
            return _ProfileIdTile(
              profileId: profileId,
              onTap: () => onOpenProfile(profileId),
              trailing: _FavoriteToggleButton(
                profileId: profileId,
                initialFavorite: true,
                onToggle: (favorite) =>
                    actions.setFavorite(profileId, favorite),
              ),
            );
          },
        );
      },
    );
  }
}

class _FavoriteToggleButton extends ConsumerStatefulWidget {
  const _FavoriteToggleButton({
    required this.profileId,
    required this.onToggle,
    this.initialFavorite,
  });

  final String profileId;
  final bool? initialFavorite;
  final Future<String?> Function(bool favorite) onToggle;

  @override
  ConsumerState<_FavoriteToggleButton> createState() =>
      _FavoriteToggleButtonState();
}

class _FavoriteToggleButtonState extends ConsumerState<_FavoriteToggleButton> {
  bool? _overrideFavorite;

  bool get _isFavorite {
    if (_overrideFavorite != null) return _overrideFavorite!;
    if (widget.initialFavorite != null) return widget.initialFavorite!;
    return ref.watch(isFavoriteProvider(widget.profileId));
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    return IconButton(
      key: SocialPanel.favoriteToggleKey(widget.profileId),
      tooltip: _isFavorite
          ? l10n.socialRemoveFavorite
          : l10n.socialAddFavorite,
      icon: Icon(_isFavorite ? Icons.star : Icons.star_border),
      onPressed: () async {
        final next = !_isFavorite;
        setState(() => _overrideFavorite = next);
        final err = await widget.onToggle(next);
        if (!mounted) return;
        if (err != null) {
          setState(() => _overrideFavorite = !next);
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(content: Text(err)),
          );
        } else {
          ref.invalidate(favoritesListProvider);
          ref.invalidate(contactsListProvider);
        }
      },
    );
  }
}

class _RequestsTab extends ConsumerWidget {
  const _RequestsTab({required this.onOpenProfile});

  final void Function(String profileId) onOpenProfile;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final requestsAsync = ref.watch(friendRequestsProvider);
    final actions = ref.read(socialActionsProvider);

    return requestsAsync.when(
      loading: () => const VoiceListSkeleton(),
      error: (e, st) => Center(
        child: VoiceStatePanel(
          key: SocialPanel.requestsUnavailableKey,
          title: socialRequestsErrorMessage(l10n, e),
          icon: Icons.cloud_off_outlined,
          actionLabel: l10n.commonRetry,
          onAction: () => ref.invalidate(friendRequestsProvider),
        ),
      ),
      data: (data) {
        final incoming = data.incoming;
        final outgoing = data.outgoing;
        if (incoming.isEmpty && outgoing.isEmpty) {
          return VoiceStatePanel(
            title: l10n.socialRequestsEmpty,
            icon: Icons.inbox_outlined,
          );
        }
        return ListView(
          children: [
            if (incoming.isNotEmpty) ...[
              Padding(
                padding: const EdgeInsets.fromLTRB(12, 8, 12, 4),
                child: Text(
                  l10n.socialIncomingRequests,
                  style: Theme.of(context).textTheme.titleSmall,
                ),
              ),
              ...incoming.map(
                (id) => _IncomingRequestTile(
                  profileId: id,
                  onOpenProfile: onOpenProfile,
                  onAccept: () => actions.acceptFriendInvitation(id),
                  onDecline: () => actions.declineFriendInvitation(id),
                ),
              ),
            ],
            if (outgoing.isNotEmpty) ...[
              Padding(
                padding: const EdgeInsets.fromLTRB(12, 16, 12, 4),
                child: Text(
                  l10n.socialOutgoingRequests,
                  style: Theme.of(context).textTheme.titleSmall,
                ),
              ),
              ...outgoing.map(
                (request) => _ProfileIdTile(
                  profileId: request.profileId,
                  subtitle: request.isDeclined
                      ? l10n.socialRequestDeclined
                      : l10n.socialRequestPending,
                  onTap: () => onOpenProfile(request.profileId),
                ),
              ),
            ],
          ],
        );
      },
    );
  }
}

class _IncomingRequestTile extends ConsumerWidget {
  const _IncomingRequestTile({
    required this.profileId,
    required this.onOpenProfile,
    required this.onAccept,
    required this.onDecline,
  });

  final String profileId;
  final void Function(String profileId) onOpenProfile;
  final Future<String?> Function() onAccept;
  final Future<String?> Function() onDecline;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final profileAsync = ref.watch(profileProvider(profileId));

    return profileAsync.when(
      loading: () => ListTile(title: Text(l10n.commonLoading)),
      error: (e, st) => ListTile(title: Text(profileId)),
      data: (profile) => ListTile(
        leading: CircleAvatar(
          child: Text(
            (profile?.displayName.isNotEmpty ?? false)
                ? profile!.displayName[0].toUpperCase()
                : '?',
          ),
        ),
        title: Text(profile?.displayName ?? profileId),
        subtitle: profile != null ? Text(profile.handle) : null,
        onTap: () => onOpenProfile(profileId),
        trailing: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            IconButton(
              key: SocialPanel.requestAcceptKey(profileId),
              icon: const Icon(Icons.check),
              onPressed: () async {
                await onAccept();
                ref.invalidate(friendRequestsProvider);
              },
            ),
            IconButton(
              key: SocialPanel.requestDeclineKey(profileId),
              icon: const Icon(Icons.close),
              onPressed: () async {
                await onDecline();
                ref.invalidate(friendRequestsProvider);
              },
            ),
          ],
        ),
      ),
    );
  }
}

class _ProfileIdTile extends ConsumerWidget {
  const _ProfileIdTile({
    required this.profileId,
    required this.onTap,
    this.subtitle,
    this.trailing,
  });

  final String profileId;
  final VoidCallback onTap;
  final String? subtitle;
  final Widget? trailing;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final profileAsync = ref.watch(profileProvider(profileId));
    final presence = ref.watch(presenceProvider(profileId));

    return profileAsync.when(
      loading: () => ListTile(title: Text(l10n.commonLoading)),
      error: (e, st) => ListTile(title: Text(profileId), onTap: onTap),
      data: (profile) {
        if (profile == null) {
          return ListTile(title: Text(profileId), onTap: onTap);
        }
        return _ProfileListTile(
          profile: profile,
          presence: presence,
          subtitle: subtitle,
          onTap: onTap,
          trailing: trailing,
        );
      },
    );
  }
}

class _ProfileListTile extends ConsumerWidget {
  const _ProfileListTile({
    super.key,
    required this.profile,
    required this.onTap,
    this.presence,
    this.subtitle,
    this.trailing,
  });

  final VoiceProfile profile;
  final VoicePresence? presence;
  final VoidCallback onTap;
  final String? subtitle;
  final Widget? trailing;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final presence = this.presence;
    final activeAuthors = ref.watch(activeStoryAuthorIdsProvider);
    final hasActiveStory = activeAuthors.contains(profile.id);
    final profileStoriesAsync = ref.watch(profileStoriesProvider(profile.id));

    return ListTile(
      leading: Stack(
        clipBehavior: Clip.none,
        children: [
          StoryRingAvatar(
            displayName: profile.displayName,
            imageUrl: profile.avatarUrl,
            hasActiveStory: hasActiveStory,
            size: 40,
            onTap: hasActiveStory
                ? () {
                    final stories = profileStoriesAsync.valueOrNull;
                    if (stories == null || stories.isEmpty) return;
                    StoriesRoutes.openViewer(
                      context,
                      storyIds: stories.map((s) => s.id).toList(),
                      profileId: profile.id,
                    );
                  }
                : null,
          ),
          if (presence != null)
            Positioned(
              right: -2,
              bottom: -2,
              child: PresenceIndicator(
                presence: presence,
                semanticLabel: _presenceLabel(
                  AppLocalizations.of(context)!,
                  presence.status,
                ),
                size: 12,
              ),
            ),
        ],
      ),
      title: Text(profile.displayName),
      subtitle: Text(subtitle ?? profile.handle),
      onTap: onTap,
      trailing: trailing,
    );
  }
}

String _presenceLabel(AppLocalizations l10n, String status) {
  return switch (status) {
    'online' => l10n.socialPresenceOnline,
    'idle' => l10n.socialPresenceIdle,
    'dnd' => l10n.socialPresenceDnd,
    _ => l10n.socialPresenceOffline,
  };
}
