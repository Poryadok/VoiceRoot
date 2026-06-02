import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/api_errors.dart';
import '../../backend/chats_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/chat_providers.dart';
import '../../state/presence_providers.dart';
import '../../state/social_providers.dart';
import '../api_error_messages.dart';
import '../social/presence_indicator.dart';

/// Middle column: DM chat list from `GET /api/v1/chats`.
class ChatListPanel extends ConsumerWidget {
  const ChatListPanel({super.key});

  static const Key panelKey = Key('chat_list_panel');
  static const Key listKey = Key('chat_list_view');
  static Key tileKey(String chatId) => Key('chat_list_tile_$chatId');
  static Key presenceIndicatorKey(String chatId) =>
      Key('chat_list_presence_$chatId');
  static const Key loadMoreKey = Key('chat_list_load_more');
  static const Key unavailableKey = Key('chat_list_unavailable');

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final chats = ref.watch(chatListControllerProvider);
    final selectedId = ref.watch(selectedChatIdProvider);
    final peerMap = ref.watch(dmPeerProfileByChatIdProvider);
    final activeProfileId = ref.watch(authControllerProvider).activeProfileId;

    return Column(
      key: panelKey,
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Padding(
          padding: const EdgeInsets.fromLTRB(12, 8, 12, 4),
          child: Text(
            l10n.chatListTitle,
            style: Theme.of(context).textTheme.titleMedium,
          ),
        ),
        Expanded(
          child: Builder(
            builder: (context) {
              final items = chats.items;
              if (chats.isLoading && items.isEmpty) {
                return const Center(child: CircularProgressIndicator());
              }
              if (chats.errorMessage != null && items.isEmpty) {
                final error = isBackendUnavailable(chats.errorStatusCode)
                    ? const BackendUnavailableException()
                    : Exception(chats.errorMessage);
                return Center(
                  child: Text(
                    key: ChatListPanel.unavailableKey,
                    chatListErrorMessage(l10n, error),
                    textAlign: TextAlign.center,
                  ),
                );
              }
              if (items.isEmpty) {
                return Center(child: Text(l10n.chatListEmpty));
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
                          : IconButton(
                              key: ChatListPanel.loadMoreKey,
                              tooltip: MaterialLocalizations.of(
                                context,
                              ).moreButtonTooltip,
                              icon: const Icon(Icons.expand_more),
                              onPressed: () => ref
                                  .read(chatListControllerProvider.notifier)
                                  .loadMore(),
                            ),
                    );
                  }
                  final item = items[index];
                  final peerId = _resolvePeerId(
                    item,
                    peerMap[item.chatId],
                    activeProfileId,
                  );
                  final titleAsync = peerId != null
                      ? ref.watch(profileProvider(peerId))
                      : null;
                  final title =
                      titleAsync?.valueOrNull?.displayName ??
                      item.chat.name ??
                      l10n.chatListDmFallback(_shortChatId(item.chatId));
                  final subtitle = item.lastMessagePreview ?? '';
                  final selected = item.chatId == selectedId;
                  final presence = peerId != null
                      ? ref.watch(presenceProvider(peerId))
                      : null;
                  return ListTile(
                    key: tileKey(item.chatId),
                    selected: selected,
                    leading: peerId != null
                        ? Stack(
                            clipBehavior: Clip.none,
                            children: [
                              CircleAvatar(
                                child: Text(
                                  title.isNotEmpty
                                      ? title[0].toUpperCase()
                                      : '?',
                                ),
                              ),
                              if (presence != null)
                                Positioned(
                                  right: -2,
                                  bottom: -2,
                                  child: PresenceIndicator(
                                    key: presenceIndicatorKey(item.chatId),
                                    presence: presence,
                                    size: 12,
                                  ),
                                ),
                            ],
                          )
                        : null,
                    title: Text(
                      title,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                    ),
                    subtitle: subtitle.isEmpty
                        ? null
                        : Text(
                            subtitle,
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                          ),
                    trailing: item.unreadCount > 0
                        ? CircleAvatar(
                            radius: 10,
                            child: Text(
                              '${item.unreadCount}',
                              style: const TextStyle(fontSize: 10),
                            ),
                          )
                        : null,
                    onTap: () =>
                        ref.read(chatActionsProvider).selectChat(item.chatId),
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

String? _resolvePeerId(
  ChatListItem item,
  String? knownPeerId,
  String? activeProfileId,
) {
  if (knownPeerId != null) return knownPeerId;
  if (!item.chat.isDm || activeProfileId == null) return null;
  final creator = item.chat.creatorProfileId;
  if (creator.isEmpty || creator == activeProfileId) return null;
  return creator;
}

String _shortChatId(String chatId) {
  return chatId.length <= 8 ? chatId : chatId.substring(0, 8);
}
