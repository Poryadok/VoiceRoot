import 'package:flutter_riverpod/flutter_riverpod.dart';
const kQuickAccessLimit = 15;

/// Session-local pin overlay keyed by folder id (ListChats has no is_pinned yet).
final folderPinnedChatIdsProvider =
    StateProvider<Map<String, Set<String>>>((ref) => {});

bool isChatPinnedInFolder(
  WidgetRef ref,
  String? folderId,
  String chatId,
) {
  if (folderId == null) return false;
  return ref.watch(folderPinnedChatIdsProvider)[folderId]?.contains(chatId) ??
      false;
}

void markChatPinnedInFolder(
  Ref ref,
  String folderId,
  String chatId,
  bool pinned,
) {
  ref.read(folderPinnedChatIdsProvider.notifier).update((map) {
    final next = Map<String, Set<String>>.from(map);
    final set = Set<String>.from(next[folderId] ?? {});
    if (pinned) {
      set.add(chatId);
    } else {
      set.remove(chatId);
    }
    next[folderId] = set;
    return next;
  });
}
