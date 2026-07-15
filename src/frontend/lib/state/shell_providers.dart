import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'chat_providers.dart';
import 'space_providers.dart';

enum ShellSidePanel { none, members, emoji, chatInfo }

enum NavigationSection { chats, social }

final shellSidePanelProvider = StateProvider<ShellSidePanel>(
  (ref) => ShellSidePanel.none,
);

final navigationSectionProvider = StateProvider<NavigationSection>(
  (ref) => NavigationSection.chats,
);

/// Emoji picked in side panel; consumed by [ChatRoomPanel] composer.
final pendingComposerEmojiProvider = StateProvider<String?>((ref) => null);

/// Bumped to request global search field focus (Ctrl+K).
final globalSearchFocusRequestProvider = StateProvider<int>((ref) => 0);

/// Bumped to request composer focus (Escape).
final composerFocusRequestProvider = StateProvider<int>((ref) => 0);

/// Last scroll offset of the home/space chat list (mobile back restores this).
final chatListScrollOffsetProvider = StateProvider<double>((ref) => 0);

/// When true, [ChatListBody] restores [chatListScrollOffsetProvider] on next frame.
final chatListScrollRestoreProvider = StateProvider<bool>((ref) => false);

/// Coordinates shell column visibility and selection consistency.
class ShellNavigation {
  ShellNavigation(this._ref);

  final Ref _ref;

  void exitSpace() {
    _ref.read(selectedSpaceIdProvider.notifier).state = null;
    _ref.read(shellSidePanelProvider.notifier).state = ShellSidePanel.none;
  }

  void selectSpace(String spaceId) {
    final current = _ref.read(selectedSpaceIdProvider);
    if (current == spaceId) {
      exitSpace();
      return;
    }
    _ref.read(selectedSpaceIdProvider.notifier).state = spaceId;
    _ref.read(shellSidePanelProvider.notifier).state = ShellSidePanel.none;
    _clearChatIfNotInSpace(spaceId);
  }

  void selectChatFromHome(String chatId) {
    exitSpace();
    _ref.read(shellSidePanelProvider.notifier).state = ShellSidePanel.none;
    _ref.read(chatActionsProvider).selectChat(chatId);
  }

  void selectChatInSpace(String chatId) {
    _ref.read(selectedChatIdProvider.notifier).state = chatId;
    _ref.read(realtimeHubProvider).ensureSubscribed(chatId);
    _ref.read(chatActionsProvider).rememberDmPeerForChat(chatId);
    _ref.read(shellSidePanelProvider.notifier).state = ShellSidePanel.none;
  }

  /// Mobile back: close open chat and restore chat-list scroll position.
  void backToChatList() {
    _ref.read(shellSidePanelProvider.notifier).state = ShellSidePanel.none;
    _ref.read(chatListScrollRestoreProvider.notifier).state = true;
    _ref.read(selectedChatIdProvider.notifier).state = null;
  }

  void selectStripChat(String chatId, {required bool inSpace}) {
    if (inSpace) {
      selectChatInSpace(chatId);
    } else {
      selectChatFromHome(chatId);
    }
  }

  void toggleSidePanel(ShellSidePanel panel) {
    final current = _ref.read(shellSidePanelProvider);
    _ref.read(shellSidePanelProvider.notifier).state =
        current == panel ? ShellSidePanel.none : panel;
  }

  void closeSidePanel() {
    _ref.read(shellSidePanelProvider.notifier).state = ShellSidePanel.none;
  }

  void setNavigationSection(NavigationSection section) {
    _ref.read(navigationSectionProvider.notifier).state = section;
  }

  void _clearChatIfNotInSpace(String spaceId) {
    final chatId = _ref.read(selectedChatIdProvider);
    if (chatId == null) return;
    final items = _ref.read(chatListControllerProvider).items;
    for (final item in items) {
      if (item.chatId != chatId) continue;
      if (item.chat.spaceId == spaceId) return;
      _ref.read(selectedChatIdProvider.notifier).state = null;
      return;
    }
    _ref.read(selectedChatIdProvider.notifier).state = null;
  }
}

final shellNavigationProvider = Provider<ShellNavigation>(
  (ref) => ShellNavigation(ref),
);
