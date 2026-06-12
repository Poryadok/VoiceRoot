import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/users_client.dart';
import '../../l10n/app_localizations.dart';
import '../api_error_messages.dart';
import '../../state/presence_providers.dart';
import '../../state/social_providers.dart';
import '../core/voice_state_panel.dart';
import '../matchmaking/game_catalog_screen.dart';
import '../matchmaking/match_history_screen.dart';
import 'presence_indicator.dart';
import 'profile_detail_sheet.dart';

/// Friends, search, and friend-requests UI (Phase 1 social column).
class SocialPanel extends ConsumerStatefulWidget {
  const SocialPanel({super.key, this.initialTabIndex = 0});

  static const Key panelKey = Key('social_panel');
  static const Key tabSearchKey = Key('social_tab_search');
  static const Key tabFriendsKey = Key('social_tab_friends');
  static const Key tabRequestsKey = Key('social_tab_requests');
  static const Key searchFieldKey = Key('social_search_field');
  static const Key searchSubmitKey = Key('social_search_submit');
  static const Key friendsListKey = Key('social_friends_list');
  static const Key friendsUnavailableKey = Key('social_friends_unavailable');
  static const Key requestsUnavailableKey = Key('social_requests_unavailable');
  static const Key searchUnavailableKey = Key('social_search_unavailable');
  static const Key searchLoadingKey = Key('social_search_loading');

  static Key requestAcceptKey(String profileId) =>
      Key('social_request_accept_$profileId');

  static Key requestDeclineKey(String profileId) =>
      Key('social_request_decline_$profileId');

  static Key profileTileKey(String profileId) =>
      Key('social_profile_tile_$profileId');

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
      length: 3,
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
    final l10n = AppLocalizations.of(context)!;

    return Column(
      key: SocialPanel.panelKey,
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Align(
          alignment: Alignment.centerLeft,
          child: Wrap(
            spacing: 4,
            children: [
              TextButton.icon(
                key: const Key('social_game_catalog_entry'),
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
          tabs: [
            Tab(key: SocialPanel.tabSearchKey, text: l10n.socialTabSearch),
            Tab(key: SocialPanel.tabFriendsKey, text: l10n.socialTabFriends),
            Tab(key: SocialPanel.tabRequestsKey, text: l10n.socialTabRequests),
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
              _RequestsTab(onOpenProfile: _openProfile),
            ],
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
                icon: search.isLoading
                    ? const SizedBox(
                        width: 20,
                        height: 20,
                        child: CircularProgressIndicator(strokeWidth: 2),
                      )
                    : const Icon(Icons.search),
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
        child: Center(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              const CircularProgressIndicator(),
              const SizedBox(height: 16),
              Text(
                l10n.socialSearchLoading,
                style: Theme.of(context).textTheme.bodyMedium,
              ),
            ],
          ),
        ),
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

    return friendsAsync.when(
      loading: () => const Center(child: CircularProgressIndicator()),
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
            return _ProfileIdTile(
              profileId: ids[index],
              onTap: () => onOpenProfile(ids[index]),
            );
          },
        );
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
      loading: () => const Center(child: CircularProgressIndicator()),
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
                (id) => _ProfileIdTile(
                  profileId: id,
                  subtitle: l10n.socialRequestPending,
                  onTap: () => onOpenProfile(id),
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
  });

  final String profileId;
  final VoidCallback onTap;
  final String? subtitle;

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
        );
      },
    );
  }
}

class _ProfileListTile extends StatelessWidget {
  const _ProfileListTile({
    super.key,
    required this.profile,
    required this.onTap,
    this.presence,
    this.subtitle,
  });

  final VoiceProfile profile;
  final VoicePresence? presence;
  final VoidCallback onTap;
  final String? subtitle;

  @override
  Widget build(BuildContext context) {
    final presence = this.presence;
    return ListTile(
      leading: Stack(
        clipBehavior: Clip.none,
        children: [
          CircleAvatar(
            child: Text(
              profile.displayName.isNotEmpty
                  ? profile.displayName[0].toUpperCase()
                  : '?',
            ),
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
