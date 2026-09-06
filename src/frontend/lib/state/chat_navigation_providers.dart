import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../backend/chats_client.dart';
import 'auth_providers.dart';
import 'chat_providers.dart';

final quickAccessListProvider = FutureProvider<QuickAccessListData>((
  ref,
) async {
  final auth = ref.watch(authorizationHeaderProvider);
  if (auth == null) {
    throw StateError('not_authenticated');
  }
  final result = await ref
      .watch(voiceChatsClientProvider)
      .listQuickAccess(authorization: auth);
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
  final result = await ref
      .watch(voiceChatsClientProvider)
      .listFolders(authorization: auth);
  return switch (result) {
    ChatsApiOk(:final data) => data,
    ChatsApiFailure(:final message) => throw Exception(message),
  };
});

class FolderActions {
  FolderActions(this._ref);

  final Ref _ref;

  Future<String?> createFolder(String name) async {
    final auth = _ref.read(authorizationHeaderProvider);
    if (auth == null) return 'not_authenticated';
    final result = await _ref
        .read(voiceChatsClientProvider)
        .createFolder(authorization: auth, name: name);
    return switch (result) {
      ChatsApiOk() => _invalidate(),
      ChatsApiFailure(:final message) => message,
    };
  }

  Future<String?> updateFolder({
    required String folderId,
    String? name,
    int? sortOrder,
  }) async {
    final auth = _ref.read(authorizationHeaderProvider);
    if (auth == null) return 'not_authenticated';
    final result = await _ref
        .read(voiceChatsClientProvider)
        .updateFolder(
          authorization: auth,
          folderId: folderId,
          name: name,
          sortOrder: sortOrder,
        );
    return switch (result) {
      ChatsApiOk() => _invalidate(),
      ChatsApiFailure(:final message) => message,
    };
  }

  Future<String?> deleteFolder(String folderId) async {
    final auth = _ref.read(authorizationHeaderProvider);
    if (auth == null) return 'not_authenticated';
    final result = await _ref
        .read(voiceChatsClientProvider)
        .deleteFolder(authorization: auth, folderId: folderId);
    return switch (result) {
      ChatsApiOk() => _invalidate(),
      ChatsApiFailure(:final message) => message,
    };
  }

  Future<String?> addChatToFolder({
    required String folderId,
    required String chatId,
  }) async {
    final auth = _ref.read(authorizationHeaderProvider);
    if (auth == null) return 'not_authenticated';
    final result = await _ref
        .read(voiceChatsClientProvider)
        .addChatToFolder(
          authorization: auth,
          folderId: folderId,
          chatId: chatId,
        );
    return switch (result) {
      ChatsApiOk() => _afterMembershipChange(),
      ChatsApiFailure(:final message) => message,
    };
  }

  Future<String?> removeChatFromFolder({
    required String folderId,
    required String chatId,
  }) async {
    final auth = _ref.read(authorizationHeaderProvider);
    if (auth == null) return 'not_authenticated';
    final result = await _ref
        .read(voiceChatsClientProvider)
        .removeChatFromFolder(
          authorization: auth,
          folderId: folderId,
          chatId: chatId,
        );
    return switch (result) {
      ChatsApiOk() => _afterMembershipChange(),
      ChatsApiFailure(:final message) => message,
    };
  }

  Future<String?> _afterMembershipChange() async {
    _invalidate();
    await _ref.read(chatListControllerProvider.notifier).loadInitial();
    return null;
  }

  String? _invalidate() {
    _ref.invalidate(quickAccessListProvider);
    _ref.invalidate(chatFoldersProvider);
    return null;
  }
}

final folderActionsProvider = Provider<FolderActions>((ref) {
  return FolderActions(ref);
});

class QuickAccessActions {
  QuickAccessActions(this._ref);

  final Ref _ref;

  Future<String?> reorder(List<String> chatIds) async {
    final auth = _ref.read(authorizationHeaderProvider);
    if (auth == null) return 'not_authenticated';
    final result = await _ref
        .read(voiceChatsClientProvider)
        .reorderQuickAccess(authorization: auth, chatIds: chatIds);
    return switch (result) {
      ChatsApiOk() => _invalidate(),
      ChatsApiFailure(:final message) => message,
    };
  }

  String? _invalidate() {
    _ref.invalidate(quickAccessListProvider);
    return null;
  }
}

final quickAccessActionsProvider = Provider<QuickAccessActions>((ref) {
  return QuickAccessActions(ref);
});

void invalidateChatNavigationData(WidgetRef ref) {
  ref.invalidate(quickAccessListProvider);
  ref.invalidate(chatFoldersProvider);
}
