import 'package:flutter_riverpod/flutter_riverpod.dart';

/// Max opened chats tracked in the mobile active strip ([navigation.md] § Active strip).
const kMobileOpenedChatStripLimit = 100;

/// Visible avatar cap before horizontal scroll (~8 per [screen-controls.md] §1.6).
const kMobileOpenedChatStripVisibleCap = 8;

/// Result of [MobileOpenedChatStripController.openChat].
class MobileOpenedChatStripOpenResult {
  const MobileOpenedChatStripOpenResult({this.evictedChatId});

  /// Oldest chat removed when LRU evicts at the 100-chat cap.
  final String? evictedChatId;
}

/// Bumped when LRU evicts oldest chat — UI shows limit feedback snackbar.
final mobileStripEvictionNoticeProvider = StateProvider<int>((ref) => 0);

/// LRU of chat ids opened on mobile this session (most recent first).
final mobileOpenedChatStripProvider =
    StateNotifierProvider<MobileOpenedChatStripController, List<String>>(
  (ref) => MobileOpenedChatStripController(ref),
);

class MobileOpenedChatStripController extends StateNotifier<List<String>> {
  MobileOpenedChatStripController(this._ref) : super(const []);

  final Ref _ref;

  /// Marks [chatId] as opened; moves it to the front; evicts oldest when over limit.
  MobileOpenedChatStripOpenResult openChat(String chatId) {
    if (chatId.isEmpty) return const MobileOpenedChatStripOpenResult();
    final without = state.where((id) => id != chatId).toList();
    final next = [chatId, ...without];
    if (next.length <= kMobileOpenedChatStripLimit) {
      state = next;
      return const MobileOpenedChatStripOpenResult();
    }
    final evicted = next.last;
    state = next.sublist(0, kMobileOpenedChatStripLimit);
    _ref.read(mobileStripEvictionNoticeProvider.notifier).state++;
    return MobileOpenedChatStripOpenResult(evictedChatId: evicted);
  }

  /// Removes [chatId] from the strip (mobile back per [navigation.md] § Active strip).
  void removeChat(String chatId) {
    if (chatId.isEmpty || !state.contains(chatId)) return;
    state = state.where((id) => id != chatId).toList();
  }

  void clear() => state = const [];
}
