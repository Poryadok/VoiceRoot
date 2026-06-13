import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../backend/messages_client.dart';
import 'auth_providers.dart';
import 'chat_providers.dart';

final sharedMediaListProvider = AsyncNotifierProvider.family<
    SharedMediaListController, SharedMediaListData, (String, SharedMediaTabKind)>(
  SharedMediaListController.new,
);

class SharedMediaListController
    extends FamilyAsyncNotifier<SharedMediaListData, (String, SharedMediaTabKind)> {
  @override
  Future<SharedMediaListData> build((String, SharedMediaTabKind) arg) async {
    final (chatId, kind) = arg;
    return _fetch(chatId, kind, cursor: null);
  }

  Future<void> loadMore() async {
    final (chatId, kind) = arg;
    final current = state.valueOrNull;
    if (current == null || !current.hasMore) return;
    final cursor = current.nextCursor;
    if (cursor == null || cursor.isEmpty) return;

    final next = await _fetch(chatId, kind, cursor: cursor);
    state = AsyncData(
      SharedMediaListData(
        items: [...current.items, ...next.items],
        nextCursor: next.nextCursor,
        hasMore: next.hasMore,
      ),
    );
  }

  Future<void> refresh() async {
    final (chatId, kind) = arg;
    state = const AsyncLoading();
    state = await AsyncValue.guard(() => _fetch(chatId, kind, cursor: null));
  }

  Future<SharedMediaListData> _fetch(
    String chatId,
    SharedMediaTabKind kind, {
    required String? cursor,
  }) async {
    final auth = ref.read(authorizationHeaderProvider);
    if (auth == null || auth.isEmpty) {
      throw StateError('not authenticated');
    }
    final client = ref.read(voiceMessagesClientProvider);
    final result = await client.listSharedMedia(
      authorization: auth,
      chatId: chatId,
      kind: kind,
      cursor: cursor,
    );
    return switch (result) {
      MessagesApiOk(:final data) => data,
      MessagesApiFailure(:final message) => throw Exception(message),
    };
  }
}

/// Request [ChatRoomPanel] to scroll to a message (consumed after handling).
final pendingChatMessageScrollProvider =
    StateProvider.family<String?, String>((ref, chatId) => null);
