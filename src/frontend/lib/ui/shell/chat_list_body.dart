import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/api_errors.dart';
import '../../backend/chats_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/chat_providers.dart';
import '../../state/in_app_notifications.dart';
import '../../state/presence_providers.dart';
import '../../state/shell_providers.dart';
import '../../state/social_providers.dart';
import '../../state/space_providers.dart';
import '../../theme/voice_colors.dart';
import '../api_error_messages.dart';
import '../core/voice_avatar.dart';
import '../core/voice_badge.dart';
import '../core/voice_list_row.dart';
import '../core/voice_skeleton.dart';
import '../core/voice_state_panel.dart';
import '../social/presence_indicator.dart';
import '../space/create_space_sheet.dart';
import '../space/join_space_invite_sheet.dart';
import '../chat/create_group_sheet.dart';

/// Reusable chat list content for navigation column and legacy middle column.
class ChatListBody extends ConsumerWidget {
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

  final bool showHeader;
  final void Function(String chatId)? onChatSelected;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    ref.watch(inAppNotificationControllerProvider);
    final l10n = AppLocalizations.of(context)!;
    final chats = ref.watch(chatListControllerProvider);
    final inbox = ref.watch(chatInboxProvider);
    final selectedId = ref.watch(selectedChatIdProvider);
    final peerMap = ref.watch(dmPeerProfileByChatIdProvider);
    final activeProfileId = ref.watch(authControllerProvider).activeProfileId;
    final shellNav = ref.read(shellNavigationProvider);

    void selectChat(String chatId) {
      if (onChatSelected != null) {
        onChatSelected!(chatId);
      } else {
        shellNav.selectChatFromHome(chatId);
      }
    }

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        if (showHeader)
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
                  key: createSpaceKey,
                  icon: const Icon(Icons.hub_outlined),
                  tooltip: l10n.spaceCreateTooltip,
                  onPressed: () => CreateSpaceSheet.show(context),
                ),
                IconButton(
                  key: joinSpaceInviteKey,
                  icon: const Icon(Icons.link),
                  tooltip: l10n.spaceInviteJoinTooltip,
                  onPressed: () => JoinSpaceInviteSheet.show(context),
                ),
                IconButton(
                  key: createGroupKey,
                  icon: const Icon(Icons.group_add_outlined),
                  tooltip: l10n.chatCreateGroupTooltip,
                  onPressed: () => CreateGroupSheet.show(context),
                ),
              ],
            ),
          ),
        const _MySpacesStrip(),
        if (chats.errorMessage == null || chats.items.isNotEmpty)
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 4),
            child: SegmentedButton<String>(
              segments: [
                ButtonSegment(value: 'main', label: Text(l10n.chatInboxDm)),
                ButtonSegment(
                  value: 'requests',
                  label: Text(l10n.chatInboxRequests),
                ),
              ],
              selected: {inbox},
              onSelectionChanged: (next) => ref
                  .read(chatListControllerProvider.notifier)
                  .setInbox(next.single),
            ),
          ),
        Expanded(
          child: Builder(
            builder: (context) {
              final items = chats.items;
              if (chats.isLoading && items.isEmpty) {
                return const VoiceListSkeleton();
              }
              if (chats.errorMessage != null && items.isEmpty) {
                final error = isBackendUnavailable(chats.errorStatusCode)
                    ? const BackendUnavailableException()
                    : Exception(chats.errorMessage);
                return KeyedSubtree(
                  key: unavailableKey,
                  child: VoiceStatePanel(
                    title: l10n.chatListLoadError,
                    message: chatListErrorMessage(l10n, error),
                    icon: Icons.cloud_off_outlined,
                    actionLabel: l10n.commonRetry,
                    onAction: () => ref
                        .read(chatListControllerProvider.notifier)
                        .loadInitial(),
                  ),
                );
              }
              if (items.isEmpty) {
                return VoiceStatePanel(
                  title: l10n.chatListEmpty,
                  message: l10n.chatListEmptyHint,
                  icon: Icons.forum_outlined,
                );
              }
              final hasFooter = chats.hasMore || chats.isLoadingMore;
              return ListView.builder(
                key: listKey,
                itemCount: items.length + (hasFooter ? 1 : 0),
                itemBuilder: (context, index) {
                  if (index == items.length) {
                    return Center(
                      child: chats.isLoadingMore
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
                              key: loadMoreKey,
                              icon: const Icon(Icons.expand_more),
                              label: Text(l10n.chatListLoadMore),
                              onPressed: () => ref
                                  .read(chatListControllerProvider.notifier)
                                  .loadMore(),
                            ),
                    );
                  }
                  final item = items[index];
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
                  final subtitle = item.lastMessagePreview ?? '';
                  final selected = item.chatId == selectedId;
                  final presence = peerId != null
                      ? ref.watch(presenceProvider(peerId))
                      : null;
                  return Column(
                    key: tileKey(item.chatId),
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      VoiceListRow(
                        selected: selected,
                        title: title,
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
                                        key: presenceIndicatorKey(item.chatId),
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
                          item: item,
                          onAccept: () => ref
                              .read(chatListControllerProvider.notifier)
                              .acceptRequest(item.chatId),
                          onDecline: () => ref
                              .read(chatListControllerProvider.notifier)
                              .declineRequest(item.chatId),
                        ),
                        onTap: () => selectChat(item.chatId),
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
                },
              );
            },
          ),
        ),
      ],
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

class _ChatListTrailing extends StatelessWidget {
  const _ChatListTrailing({
    required this.l10n,
    required this.inbox,
    required this.item,
    required this.onAccept,
    required this.onDecline,
  });

  final AppLocalizations l10n;
  final String inbox;
  final ChatListItem item;
  final VoidCallback onAccept;
  final VoidCallback onDecline;

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
    if (item.unreadCount > 0) {
      return VoiceBadge(
        count: item.unreadCount,
        semanticLabel: l10n.chatListUnreadCount(item.unreadCount),
      );
    }
    return const SizedBox.shrink();
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
