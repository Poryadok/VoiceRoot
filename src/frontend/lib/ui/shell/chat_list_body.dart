import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/api_errors.dart';
import '../../backend/chats_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/chat_providers.dart';
import '../../state/in_app_notifications.dart';
import '../../state/inbox_reconciler.dart';
import '../../state/presence_providers.dart';
import '../../state/shell_providers.dart';
import '../../state/social_providers.dart';
import '../../state/space_providers.dart';
import '../../state/subscription_providers.dart';
import '../../theme/voice_colors.dart';
import '../api_error_messages.dart';
import '../core/chat_author_label.dart';
import '../core/voice_avatar.dart';
import '../core/voice_badge.dart';
import '../core/voice_list_row.dart';
import '../core/voice_skeleton.dart';
import '../core/voice_state_panel.dart';
import '../social/presence_indicator.dart';
import '../space/create_space_sheet.dart';
import '../space/join_space_invite_sheet.dart';
import '../chat/create_group_sheet.dart';
import '../onboarding/onboarding_anchor_keys.dart';
import '../../state/chat_navigation_providers.dart';
import '../../state/folder_pin_providers.dart';
import 'quick_access_actions.dart';

/// Reusable chat list content for navigation column and legacy middle column.
class ChatListBody extends ConsumerStatefulWidget {
  const ChatListBody({
    super.key,
    this.showHeader = true,
    this.onChatSelected,
  });

  static const Key listKey = Key('chat_list_view');
  static Key tileKey(String chatId) => Key('chat_list_tile_$chatId');
  static Key presenceIndicatorKey(String chatId) =>
      Key('chat_list_presence_$chatId');
  static const Key loadMoreKey = Key('chat_list_load_more');
  static const Key unavailableKey = Key('chat_list_unavailable');
  static const Key createGroupKey = Key('chat_list_create_group');
  static const Key createSpaceKey = Key('chat_list_create_space');
  static const Key joinSpaceInviteKey = Key('chat_list_join_space_invite');
  static Key spaceTileKey(String spaceId) => Key('chat_list_space_$spaceId');
  static Key muteActionKey(String chatId) => Key('chat_list_mute_$chatId');
  static Key pinActionKey(String chatId) => Key('chat_list_pin_$chatId');
  static Key quickAccessActionKey(String chatId) =>
      Key('chat_list_quick_access_$chatId');
  static Key archiveActionKey(String chatId) =>
      Key('chat_list_archive_$chatId');

  final bool showHeader;
  final void Function(String chatId)? onChatSelected;

  @override
  ConsumerState<ChatListBody> createState() => _ChatListBodyState();
}

class _ChatListBodyState extends ConsumerState<ChatListBody> {
  late final ScrollController _scrollController;

  @override
  void initState() {
    super.initState();
    _scrollController = ScrollController()..addListener(_persistScrollOffset);
  }

  @override
  void dispose() {
    _scrollController.removeListener(_persistScrollOffset);
    _scrollController.dispose();
    super.dispose();
  }

  void _persistScrollOffset() {
    if (!_scrollController.hasClients) return;
    ref.read(chatListScrollOffsetProvider.notifier).state =
        _scrollController.offset;
  }

  void _restoreScrollOffset() {
    final offset = ref.read(chatListScrollOffsetProvider);
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted || !_scrollController.hasClients) return;
      final maxExtent = _scrollController.position.maxScrollExtent;
      _scrollController.jumpTo(offset.clamp(0, maxExtent));
      ref.read(chatListScrollRestoreProvider.notifier).state = false;
    });
  }

  @override
  Widget build(BuildContext context) {
    ref.listen<bool>(chatListScrollRestoreProvider, (previous, next) {
      if (next) _restoreScrollOffset();
    });
    ref.watch(inAppNotificationControllerProvider);
    final l10n = AppLocalizations.of(context)!;
    final chats = ref.watch(chatListControllerProvider);
    final inbox = ref.watch(chatInboxProvider);
    final selectedId = ref.watch(selectedChatIdProvider);
    final peerMap = ref.watch(dmPeerProfileByChatIdProvider);
    final activeProfileId = ref.watch(authControllerProvider).activeProfileId;
    final shellNav = ref.read(shellNavigationProvider);
    final selectedFolderId = ref.watch(selectedChatFolderIdProvider);
    final foldersAsync = ref.watch(chatFoldersProvider);
    final customFolderReorder = foldersAsync.valueOrNull?.folders
            .any((f) => f.id == selectedFolderId && !f.isSystem) ??
        false;
    final reconciler = ref.watch(inboxReconcilerProvider);
    final reconcilerScope = selectedFolderId == null && activeProfileId != null
        ? reconciler.profileSnapshots[activeProfileId]?.scopes[
            inbox == 'requests' ? InboxScope.requests : InboxScope.main]
        : null;

    void selectChat(String chatId) {
      if (widget.onChatSelected != null) {
        widget.onChatSelected!(chatId);
      } else {
        shellNav.selectChatFromHome(chatId);
      }
    }

    return KeyedSubtree(
      key: OnboardingAnchorKeys.chatsNav,
      child: Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        if (widget.showHeader)
          Padding(
            padding: const EdgeInsets.fromLTRB(4, 4, 4, 4),
            child: Row(
              children: [
                Expanded(
                  child: Padding(
                    padding: const EdgeInsets.only(left: 8),
                    child: Text(
                      l10n.chatListTitle,
                      style: Theme.of(context).textTheme.titleMedium,
                    ),
                  ),
                ),
                IconButton(
                  key: ChatListBody.createSpaceKey,
                  icon: const Icon(Icons.hub_outlined),
                  tooltip: l10n.spaceCreateTooltip,
                  onPressed: () => CreateSpaceSheet.show(context),
                ),
                IconButton(
                  key: ChatListBody.joinSpaceInviteKey,
                  icon: const Icon(Icons.link),
                  tooltip: l10n.spaceInviteJoinTooltip,
                  onPressed: () => JoinSpaceInviteSheet.show(context),
                ),
                IconButton(
                  key: ChatListBody.createGroupKey,
                  icon: const Icon(Icons.group_add_outlined),
                  tooltip: l10n.chatCreateGroupTooltip,
                  onPressed: () => CreateGroupSheet.show(context),
                ),
              ],
            ),
          ),
        const _MySpacesStrip(),
        Expanded(
          child: Builder(
            builder: (context) {
              final items = reconcilerScope == null
                  ? activeProfileId != null && chats.profileId == activeProfileId
                      ? chats.items
                      : const <ChatListItem>[]
                  : reconcilerScope.isComplete
                      ? reconcilerScope.items
                      : chats.profileId == activeProfileId
                          ? mergeInboxRows(chats.items, reconcilerScope.items)
                          : reconcilerScope.items;
              final legacyMatchesProfile = activeProfileId != null &&
                  chats.profileId == activeProfileId;
              final isLoading = reconcilerScope?.isLoading ??
                  (legacyMatchesProfile
                      ? chats.isLoading
                      : activeProfileId != null);
              final errorMessage =
                  reconcilerScope?.errorMessage ??
                  (legacyMatchesProfile ? chats.errorMessage : null);
              final errorStatusCode = reconcilerScope?.errorStatusCode ??
                  (legacyMatchesProfile ? chats.errorStatusCode : null);
              final hasReconcilerError = reconcilerScope?.errorMessage != null;
              if (isLoading && items.isEmpty) {
                return const VoiceListSkeleton();
              }
              if (errorMessage != null && items.isEmpty) {
                final error = isBackendUnavailable(errorStatusCode)
                    ? const BackendUnavailableException()
                    : Exception(errorMessage);
                return KeyedSubtree(
                  key: ChatListBody.unavailableKey,
                  child: VoiceStatePanel(
                    title: l10n.chatListLoadError,
                    message: chatListErrorMessage(l10n, error),
                    icon: Icons.cloud_off_outlined,
                    actionLabel: l10n.commonRetry,
                    onAction: reconcilerScope != null
                        ? () => ref
                              .read(inboxReconcilerProvider.notifier)
                              .retry(inbox == 'requests'
                                  ? InboxScope.requests
                                  : InboxScope.main)
                        : () => ref
                              .read(chatListControllerProvider.notifier)
                              .loadInitial(),
                  ),
                );
              }
              if (items.isEmpty) {
                return VoiceStatePanel(
                  title: inbox == 'requests'
                      ? l10n.chatInboxRequests
                      : l10n.chatListEmpty,
                  message: inbox == 'requests'
                      ? l10n.chatMessageRequestsEmptyHint
                      : l10n.chatListEmptyHint,
                  icon: inbox == 'requests'
                      ? Icons.mark_email_unread_outlined
                      : Icons.forum_outlined,
                );
              }
              final hasFooter = reconcilerScope != null
                  ? isLoading ||
                      reconcilerScope.nextCursor != null ||
                      hasReconcilerError
                  : chats.hasMore || chats.isLoadingMore;
              final canReorder =
                  customFolderReorder &&
                  selectedFolderId != null &&
                  inbox == 'main' &&
                  !hasFooter;

              Widget buildRow(
                ChatListItem item, {
                int? reorderIndex,
              }) {
                final peerId = resolveDmPeerProfileId(
                  item: item,
                  knownPeerId: peerMap[item.chatId],
                  activeProfileId: activeProfileId,
                );
                final titleAsync = peerId != null
                    ? ref.watch(profileProvider(peerId))
                    : null;
                final profile = titleAsync?.valueOrNull;
                final title =
                    profile?.displayName ??
                    item.chat.name ??
                    l10n.chatListDmFallback(_shortChatId(item.chatId));
                final showPremium = peerId != null &&
                    ref.watch(profilePremiumBadgeProvider(peerId));
                final subtitle = item.lastMessagePreview ?? '';
                final selected = item.chatId == selectedId;
                final presence = peerId != null
                    ? ref.watch(presenceProvider(peerId))
                    : null;
                final pinned = item.isPinned ||
                    isChatPinnedInFolder(ref, selectedFolderId, item.chatId);
                final displayItem = pinned ? item.copyWith(isPinned: true) : item;
                return Column(
                  key: ChatListBody.tileKey(item.chatId),
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    VoiceListRow(
                      selected: selected,
                      title: title,
                      titleWidget: peerId != null && !item.chat.isGroup
                          ? ChatAuthorLabel(
                              displayName: title,
                              isPremium: showPremium,
                              verificationType:
                                  profile?.verificationType ?? 'none',
                              premiumBadgeSemanticLabel:
                                  l10n.premiumBadgeLabel,
                              verifiedBadgeSemanticLabel:
                                  profile?.verificationType == 'organization'
                                  ? l10n.verifiedBadgeOrganization
                                  : l10n.verifiedBadgePersonal,
                            )
                          : null,
                      subtitle: subtitle.isEmpty ? null : subtitle,
                      leading: item.chat.isGroup
                          ? VoiceAvatar(
                              imageUrl: item.chat.avatarUrl,
                              label: title,
                            )
                          : peerId != null
                          ? VoiceAvatarWithPresence(
                              avatar: VoiceAvatar(
                                imageUrl: profile?.avatarUrl,
                                label: title,
                              ),
                              presence: presence != null
                                  ? PresenceIndicator(
                                      key: ChatListBody.presenceIndicatorKey(
                                        item.chatId,
                                      ),
                                      presence: presence,
                                      semanticLabel: _presenceLabel(
                                        l10n,
                                        presence.status,
                                      ),
                                      size: 12,
                                    )
                                  : null,
                            )
                          : null,
                      trailing: _ChatListTrailing(
                        l10n: l10n,
                        inbox: inbox,
                        item: displayItem,
                        muted: ref.watch(chatMutedUntilProvider)[item.chatId] !=
                            null,
                        showDragHandle: reorderIndex != null,
                        dragIndex: reorderIndex,
                        onAccept: () async {
                          final session = ref.read(authControllerProvider).session;
                          if (session == null) return;
                          final error = await ref
                              .read(chatListControllerProvider.notifier)
                              .acceptRequest(item.chatId);
                          if (!context.mounted) return;
                          if (error == null) {
                            ref
                                .read(inboxReconcilerProvider.notifier)
                                .removeChat(
                                  InboxScope.requests,
                                  item.chatId,
                                  expectedProfileId: session.activeProfileId,
                                  expectedAuthorization:
                                      session.authorizationHeader,
                                );
                          }
                        },
                        onDecline: () async {
                          final session = ref.read(authControllerProvider).session;
                          if (session == null) return;
                          final error = await ref
                              .read(chatListControllerProvider.notifier)
                              .declineRequest(item.chatId);
                          if (!context.mounted) return;
                          if (error == null) {
                            ref
                                .read(inboxReconcilerProvider.notifier)
                                .removeChat(
                                  InboxScope.requests,
                                  item.chatId,
                                  expectedProfileId: session.activeProfileId,
                                  expectedAuthorization:
                                      session.authorizationHeader,
                                );
                          }
                        },
                      ),
                      onTap: () => selectChat(item.chatId),
                      onLongPress: inbox == 'requests'
                          ? null
                          : () => _showChatRowActions(
                                context,
                                ref,
                                l10n,
                                displayItem,
                                folderId: selectedFolderId,
                              ),
                    ),
                    if (item.isStranger && inbox == 'main')
                      Padding(
                        padding: const EdgeInsets.only(left: 56, bottom: 4),
                        child: Align(
                          alignment: Alignment.centerLeft,
                          child: _StrangerChip(
                            label: l10n.chatListStrangerBadge,
                          ),
                        ),
                      ),
                  ],
                );
              }

              if (canReorder) {
                return ReorderableListView.builder(
                  key: ChatListBody.listKey,
                  scrollController: _scrollController,
                  buildDefaultDragHandles: false,
                  itemCount: items.length,
                  onReorder: (oldIndex, newIndex) {
                    if (newIndex > oldIndex) newIndex -= 1;
                    final ids = items.map((item) => item.chatId).toList();
                    final moved = ids.removeAt(oldIndex);
                    ids.insert(newIndex, moved);
                    ref
                        .read(chatListControllerProvider.notifier)
                        .reorderFolderChats(ids);
                  },
                  itemBuilder: (context, index) {
                    return buildRow(items[index], reorderIndex: index);
                  },
                );
              }

              return ListView.builder(
                key: ChatListBody.listKey,
                controller: _scrollController,
                itemCount: items.length + (hasFooter ? 1 : 0),
                itemBuilder: (context, index) {
                  if (index == items.length) {
                    if (hasReconcilerError) {
                      final error = isBackendUnavailable(errorStatusCode)
                          ? const BackendUnavailableException()
                          : Exception(errorMessage);
                      return VoiceStatePanel(
                        title: l10n.chatListLoadError,
                        message: chatListErrorMessage(l10n, error),
                        icon: Icons.cloud_off_outlined,
                        actionLabel: l10n.commonRetry,
                        onAction: () => ref
                            .read(inboxReconcilerProvider.notifier)
                            .retry(inbox == 'requests'
                                ? InboxScope.requests
                                : InboxScope.main),
                      );
                    }
                    return Center(
                      child: reconcilerScope != null || chats.isLoadingMore
                          ? const Padding(
                              padding: EdgeInsets.all(8),
                              child: SizedBox(
                                width: 24,
                                height: 24,
                                child: CircularProgressIndicator(
                                  strokeWidth: 2,
                                ),
                              ),
                            )
                          : TextButton.icon(
                              key: ChatListBody.loadMoreKey,
                              icon: const Icon(Icons.expand_more),
                              label: Text(l10n.chatListLoadMore),
                              onPressed: () => ref
                                  .read(chatListControllerProvider.notifier)
                                  .loadMore(),
                            ),
                    );
                  }
                  return buildRow(items[index]);
                },
              );
            },
          ),
        ),
      ],
    ),
    );
  }
}

class _StrangerChip extends StatelessWidget {
  const _StrangerChip({required this.label});

  final String label;

  @override
  Widget build(BuildContext context) {
    final voice = VoiceColors.of(context);
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
      decoration: BoxDecoration(
        border: Border.all(color: voice.borderDefault),
        borderRadius: BorderRadius.circular(4),
      ),
      child: Text(label, style: Theme.of(context).textTheme.labelSmall),
    );
  }
}

Future<void> _showChatRowActions(
  BuildContext context,
  WidgetRef ref,
  AppLocalizations l10n,
  ChatListItem item, {
  String? folderId,
}) async {
  final muted = ref.read(chatMutedUntilProvider)[item.chatId] != null;
  final qaAsync = ref.read(quickAccessListProvider);
  final inQuickAccess = qaAsync.valueOrNull?.items
          .any((entry) => entry.chatId == item.chatId) ??
      false;
  final action = await showModalBottomSheet<String>(
    context: context,
    builder: (ctx) {
      return SafeArea(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            if (folderId != null)
              ListTile(
                key: ChatListBody.pinActionKey(item.chatId),
                leading: Icon(
                  item.isPinned ? Icons.push_pin : Icons.push_pin_outlined,
                ),
                title: Text(item.isPinned ? l10n.chatListUnpin : l10n.chatListPin),
                onTap: () => Navigator.pop(ctx, item.isPinned ? 'unpin' : 'pin'),
              ),
            if (!inQuickAccess)
              ListTile(
                key: ChatListBody.quickAccessActionKey(item.chatId),
                leading: const Icon(Icons.star_outline),
                title: Text(l10n.chatListAddQuickAccess),
                onTap: () => Navigator.pop(ctx, 'quick_access'),
              ),
            ListTile(
              key: ChatListBody.muteActionKey(item.chatId),
              leading: Icon(muted ? Icons.notifications_active : Icons.notifications_off),
              title: Text(muted ? l10n.chatListUnmute : l10n.chatListMute),
              onTap: () => Navigator.pop(ctx, muted ? 'unmute' : 'mute'),
            ),
            if (item.chat.isDm)
              ListTile(
                key: ChatListBody.archiveActionKey(item.chatId),
                leading: const Icon(Icons.archive_outlined),
                title: Text(l10n.chatListArchive),
                onTap: () => Navigator.pop(ctx, 'archive'),
              ),
          ],
        ),
      );
    },
  );
  if (action == null || !context.mounted) return;
  final controller = ref.read(chatListControllerProvider.notifier);
  switch (action) {
    case 'pin':
      if (folderId != null) {
        await controller.setChatPinnedInFolder(
          folderId: folderId,
          chatId: item.chatId,
          pinned: true,
        );
      }
    case 'unpin':
      if (folderId != null) {
        await controller.setChatPinnedInFolder(
          folderId: folderId,
          chatId: item.chatId,
          pinned: false,
        );
      }
    case 'quick_access':
      await addChatToQuickAccess(context, ref, chatId: item.chatId);
    case 'mute':
      final until = DateTime.utc(9999, 12, 31);
      final err = await controller.muteChat(item.chatId, mutedUntil: until);
      if (err == null) {
        ref.read(chatMutedUntilProvider.notifier).update((m) {
          final next = Map<String, DateTime>.from(m);
          next[item.chatId] = until;
          return next;
        });
      }
    case 'unmute':
      final err = await controller.muteChat(item.chatId);
      if (err == null) {
        ref.read(chatMutedUntilProvider.notifier).update((m) {
          final next = Map<String, DateTime>.from(m);
          next.remove(item.chatId);
          return next;
        });
      }
    case 'archive':
      await controller.archiveChat(item.chatId, archived: true);
  }
}

class _ChatListTrailing extends StatelessWidget {
  const _ChatListTrailing({
    required this.l10n,
    required this.inbox,
    required this.item,
    required this.muted,
    required this.onAccept,
    required this.onDecline,
    this.showDragHandle = false,
    this.dragIndex,
  });

  final AppLocalizations l10n;
  final String inbox;
  final ChatListItem item;
  final bool muted;
  final VoidCallback onAccept;
  final VoidCallback onDecline;
  final bool showDragHandle;
  final int? dragIndex;

  @override
  Widget build(BuildContext context) {
    if (inbox == 'requests') {
      return Wrap(
        spacing: 4,
        children: [
          TextButton(onPressed: onAccept, child: Text(l10n.socialAcceptRequest)),
          TextButton(
            onPressed: onDecline,
            child: Text(l10n.socialDeclineRequest),
          ),
        ],
      );
    }
    final voice = VoiceColors.of(context);
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        if (showDragHandle && dragIndex != null)
          ReorderableDragStartListener(
            index: dragIndex!,
            child: Padding(
              padding: const EdgeInsets.only(right: 4),
              child: Icon(
                Icons.drag_handle,
                size: 18,
                color: voice.textSecondary,
              ),
            ),
          ),
        if (item.isPinned)
          Padding(
            padding: const EdgeInsets.only(right: 4),
            child: Icon(
              Icons.push_pin,
              size: 16,
              color: voice.textSecondary,
              semanticLabel: l10n.chatListPin,
            ),
          ),
        if (muted)
          Padding(
            padding: const EdgeInsets.only(right: 4),
            child: Icon(
              Icons.notifications_off_outlined,
              size: 16,
              color: voice.textSecondary,
              semanticLabel: l10n.chatListMute,
            ),
          ),
        if (item.unreadCount > 0)
          VoiceBadge(
            count: item.unreadCount,
            semanticLabel: l10n.chatListUnreadCount(item.unreadCount),
          ),
      ],
    );
  }
}

class _MySpacesStrip extends ConsumerWidget {
  const _MySpacesStrip();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final spacesAsync = ref.watch(mySpacesProvider);
    final shellNav = ref.read(shellNavigationProvider);

    return spacesAsync.when(
      loading: () => const SizedBox.shrink(),
      error: (_, _) => const SizedBox.shrink(),
      data: (data) {
        if (data.spaces.isEmpty) return const SizedBox.shrink();
        return Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Padding(
              padding: const EdgeInsets.fromLTRB(16, 4, 16, 4),
              child: Text(
                l10n.spaceListTitle,
                style: Theme.of(context).textTheme.labelMedium,
              ),
            ),
            ...data.spaces.map(
              (space) => VoiceListRow(
                key: ChatListBody.spaceTileKey(space.id),
                title: space.name,
                subtitle: l10n.spaceOpenAction,
                leading: VoiceAvatar(
                  imageUrl: space.iconUrl,
                  label: space.name,
                ),
                onTap: () => shellNav.selectSpace(space.id),
              ),
            ),
            const Divider(height: 1),
          ],
        );
      },
    );
  }
}

String _shortChatId(String chatId) {
  return chatId.length <= 8 ? chatId : chatId.substring(0, 8);
}

String _presenceLabel(AppLocalizations l10n, String status) {
  return switch (status) {
    'online' => l10n.socialPresenceOnline,
    'idle' => l10n.socialPresenceIdle,
    'dnd' => l10n.socialPresenceDnd,
    _ => l10n.socialPresenceOffline,
  };
}
