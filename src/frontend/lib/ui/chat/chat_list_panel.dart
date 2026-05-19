import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../l10n/app_localizations.dart';
import '../../state/chat_providers.dart';
import '../../state/social_providers.dart';

/// Middle column: DM chat list from `GET /api/v1/chats`.
class ChatListPanel extends ConsumerWidget {
  const ChatListPanel({super.key});

  static const Key panelKey = Key('chat_list_panel');
  static const Key listKey = Key('chat_list_view');
  static Key tileKey(String chatId) => Key('chat_list_tile_$chatId');

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final chatsAsync = ref.watch(chatListProvider);
    final selectedId = ref.watch(selectedChatIdProvider);
    final peerMap = ref.watch(dmPeerProfileByChatIdProvider);

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
          child: chatsAsync.when(
            loading: () => const Center(child: CircularProgressIndicator()),
            error: (e, st) => Center(child: Text(l10n.chatListLoadError)),
            data: (data) {
              final items = data?.items ?? const [];
              if (items.isEmpty) {
                return Center(child: Text(l10n.chatListEmpty));
              }
              return ListView.builder(
                key: listKey,
                itemCount: items.length,
                itemBuilder: (context, index) {
                  final item = items[index];
                  final peerId = peerMap[item.chatId];
                  final titleAsync = peerId != null
                      ? ref.watch(profileProvider(peerId))
                      : null;
                  final title = titleAsync?.valueOrNull?.displayName ??
                      item.chat.name ??
                      l10n.chatListDmFallback(item.chatId.substring(0, 8));
                  final subtitle = item.lastMessagePreview ?? '';
                  final selected = item.chatId == selectedId;
                  return ListTile(
                    key: tileKey(item.chatId),
                    selected: selected,
                    title: Text(title, maxLines: 1, overflow: TextOverflow.ellipsis),
                    subtitle: subtitle.isEmpty
                        ? null
                        : Text(subtitle, maxLines: 1, overflow: TextOverflow.ellipsis),
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
