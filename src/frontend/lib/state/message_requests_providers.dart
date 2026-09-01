import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../backend/chats_client.dart';
import 'auth_providers.dart';
import 'chat_providers.dart';
import 'shell_providers.dart';

/// Virtual folder id for DM message requests (navigation.md § Message requests).
const String kVirtualMessageRequestsFolderId = '__message_requests__';

class MessageRequestsSummary {
  const MessageRequestsSummary({
    required this.pendingCount,
    required this.unreadCount,
  });

  final int pendingCount;
  final int unreadCount;

  bool get isVisible => pendingCount > 0;
}

final messageRequestsSummaryProvider =
    FutureProvider<MessageRequestsSummary>((ref) async {
  final auth = ref.watch(authorizationHeaderProvider);
  if (auth == null) {
    return const MessageRequestsSummary(pendingCount: 0, unreadCount: 0);
  }
  final result = await ref.watch(voiceChatsClientProvider).listChats(
        authorization: auth,
        inbox: 'requests',
      );
  return switch (result) {
    ChatsApiOk(:final data) => MessageRequestsSummary(
        pendingCount: data.items.length,
        unreadCount: data.items
            .where((item) => item.unreadCount > 0)
            .fold<int>(0, (sum, item) => sum + item.unreadCount),
      ),
    ChatsApiFailure() => const MessageRequestsSummary(
        pendingCount: 0,
        unreadCount: 0,
      ),
  };
});

bool isMessageRequestsFolderSelected(String? folderId) =>
    folderId == kVirtualMessageRequestsFolderId;

void selectChatFolder(WidgetRef ref, String? folderId) {
  if (isMessageRequestsFolderSelected(folderId)) {
    ref.read(chatInboxProvider.notifier).state = 'requests';
    ref.read(selectedChatFolderIdProvider.notifier).state =
        kVirtualMessageRequestsFolderId;
  } else {
    ref.read(chatInboxProvider.notifier).state = 'main';
    ref.read(selectedChatFolderIdProvider.notifier).state = folderId;
  }
  ref.read(chatListControllerProvider.notifier).loadInitial();
}

void invalidateMessageRequestsData(WidgetRef ref) {
  ref.invalidate(messageRequestsSummaryProvider);
}
