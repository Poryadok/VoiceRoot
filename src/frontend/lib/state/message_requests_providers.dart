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
    this.isAuthoritative = true,
  });

  final int pendingCount;
  final int unreadCount;

  /// False when the request list could not be loaded completely enough to
  /// justify navigation based on its count.
  final bool isAuthoritative;

  bool get isVisible => pendingCount > 0;
}

final messageRequestsSummaryProvider = FutureProvider<MessageRequestsSummary>((
  ref,
) async {
  final auth = ref.watch(authorizationHeaderProvider);
  if (auth == null) {
    return const MessageRequestsSummary(
      pendingCount: 0,
      unreadCount: 0,
      isAuthoritative: false,
    );
  }
  final result = await ref
      .watch(voiceChatsClientProvider)
      .listChats(authorization: auth, inbox: 'requests');
  return switch (result) {
    ChatsApiOk(:final data) => MessageRequestsSummary(
      pendingCount: data.items.length,
      unreadCount: data.items
          .where((item) => item.unreadCount > 0)
          .fold<int>(0, (sum, item) => sum + item.unreadCount),
      isAuthoritative: data.nextCursor == null,
    ),
    ChatsApiFailure() => const MessageRequestsSummary(
      pendingCount: 0,
      unreadCount: 0,
      isAuthoritative: false,
    ),
  };
});

final messageRequestsNavigationGenerationProvider = StateProvider<int>(
  (ref) => 0,
);

final previousChatFolderBeforeMessageRequestsProvider = StateProvider<String?>(
  (ref) => null,
);

bool isMessageRequestsFolderSelected(String? folderId) =>
    folderId == kVirtualMessageRequestsFolderId;

void selectChatFolder(WidgetRef ref, String? folderId) {
  final wasInRequests = ref.read(chatInboxProvider) == 'requests';
  if (isMessageRequestsFolderSelected(folderId)) {
    if (!wasInRequests) {
      final selectedFolder = ref.read(selectedChatFolderIdProvider);
      ref
          .read(previousChatFolderBeforeMessageRequestsProvider.notifier)
          .state = isMessageRequestsFolderSelected(selectedFolder)
          ? null
          : selectedFolder;
    }
    ref.read(chatInboxProvider.notifier).state = 'requests';
    ref.read(selectedChatFolderIdProvider.notifier).state =
        kVirtualMessageRequestsFolderId;
  } else {
    ref.read(chatInboxProvider.notifier).state = 'main';
    ref.read(selectedChatFolderIdProvider.notifier).state = folderId;
    ref.read(previousChatFolderBeforeMessageRequestsProvider.notifier).state =
        null;
  }
  ref.read(messageRequestsNavigationGenerationProvider.notifier).state++;
  ref.read(chatListControllerProvider.notifier).loadInitial();
}

Future<void> restorePreviousChatFolderAfterFinalRequest(
  ProviderContainer container, {
  required Future<MessageRequestsSummary> summaryFuture,
  required int expectedGeneration,
  required String expectedAuthorization,
  required String? expectedProfileId,
}) async {
  MessageRequestsSummary summary;
  try {
    summary = await summaryFuture;
  } on Object {
    return;
  }
  final session = container.read(authControllerProvider).session;
  if (!summary.isAuthoritative ||
      summary.pendingCount != 0 ||
      container.read(messageRequestsNavigationGenerationProvider) !=
          expectedGeneration ||
      container.read(chatInboxProvider) != 'requests' ||
      !isMessageRequestsFolderSelected(
        container.read(selectedChatFolderIdProvider),
      ) ||
      session?.activeProfileId != expectedProfileId ||
      session?.authorizationHeader != expectedAuthorization) {
    return;
  }

  final previousFolder = container.read(
    previousChatFolderBeforeMessageRequestsProvider,
  );
  container.read(chatInboxProvider.notifier).state = 'main';
  container.read(selectedChatFolderIdProvider.notifier).state = previousFolder;
  container
          .read(previousChatFolderBeforeMessageRequestsProvider.notifier)
          .state =
      null;
  container.read(messageRequestsNavigationGenerationProvider.notifier).state++;
  await container.read(chatListControllerProvider.notifier).reloadInitial();
}

void invalidateMessageRequestsData(WidgetRef ref) {
  ref.invalidate(messageRequestsSummaryProvider);
}
