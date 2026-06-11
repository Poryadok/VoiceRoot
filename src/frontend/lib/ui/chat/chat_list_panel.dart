import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../shell/chat_list_body.dart';

/// Legacy middle-column wrapper; list content lives in [ChatListBody].
class ChatListPanel extends ConsumerWidget {
  const ChatListPanel({super.key});

  static const Key panelKey = Key('chat_list_panel');

  static Key tileKey(String chatId) => ChatListBody.tileKey(chatId);
  static Key presenceIndicatorKey(String chatId) =>
      ChatListBody.presenceIndicatorKey(chatId);
  static const Key loadMoreKey = ChatListBody.loadMoreKey;
  static const Key unavailableKey = ChatListBody.unavailableKey;
  static const Key createGroupKey = ChatListBody.createGroupKey;
  static const Key createSpaceKey = ChatListBody.createSpaceKey;
  static const Key joinSpaceInviteKey = ChatListBody.joinSpaceInviteKey;
  static Key spaceTileKey(String spaceId) => ChatListBody.spaceTileKey(spaceId);
  static const Key listKey = ChatListBody.listKey;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return const ChatListBody(
      key: panelKey,
      showHeader: true,
    );
  }
}
