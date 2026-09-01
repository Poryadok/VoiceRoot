import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../backend/chats_client.dart';
import 'auth_providers.dart';
import 'chat_providers.dart';

final quickAccessListProvider = FutureProvider<QuickAccessListData>((ref) async {
  final auth = ref.watch(authorizationHeaderProvider);
  if (auth == null) {
    throw StateError('not_authenticated');
  }
  final result = await ref.watch(voiceChatsClientProvider).listQuickAccess(
        authorization: auth,
      );
  return switch (result) {
    ChatsApiOk(:final data) => data,
    ChatsApiFailure(:final message) => throw Exception(message),
  };
});

final chatFoldersProvider = FutureProvider<FolderListData>((ref) async {
  final auth = ref.watch(authorizationHeaderProvider);
  if (auth == null) {
    throw StateError('not_authenticated');
  }
  final result = await ref.watch(voiceChatsClientProvider).listFolders(
        authorization: auth,
      );
  return switch (result) {
    ChatsApiOk(:final data) => data,
    ChatsApiFailure(:final message) => throw Exception(message),
  };
});

void invalidateChatNavigationData(WidgetRef ref) {
  ref.invalidate(quickAccessListProvider);
  ref.invalidate(chatFoldersProvider);
}
