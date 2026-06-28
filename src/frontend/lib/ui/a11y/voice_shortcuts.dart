import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/messages_client.dart';
import '../../state/chat_providers.dart';
import '../../state/shared_media_providers.dart';
import '../../state/shell_providers.dart';

/// Desktop/web keyboard shortcuts (docs/features/accessibility.md).
class VoiceShortcuts extends ConsumerWidget {
  const VoiceShortcuts({super.key, required this.child});

  final Widget child;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return Shortcuts(
      shortcuts: const {
        SingleActivator(LogicalKeyboardKey.keyK, control: true): _FocusSearchIntent(),
        SingleActivator(LogicalKeyboardKey.comma, control: true): _OpenSettingsIntent(),
        SingleActivator(LogicalKeyboardKey.arrowDown, alt: true): _NextUnreadChatIntent(),
        SingleActivator(LogicalKeyboardKey.arrowUp, alt: true): _PrevUnreadChatIntent(),
        SingleActivator(LogicalKeyboardKey.escape): _FocusComposerIntent(),
        SingleActivator(LogicalKeyboardKey.arrowDown): _NextMessageIntent(),
        SingleActivator(LogicalKeyboardKey.arrowUp): _PrevMessageIntent(),
        SingleActivator(LogicalKeyboardKey.keyR): _ReplyMessageIntent(),
        SingleActivator(LogicalKeyboardKey.keyE): _ReactMessageIntent(),
        SingleActivator(LogicalKeyboardKey.enter): _OpenMessageMenuIntent(),
      },
      child: Actions(
        actions: {
          _FocusSearchIntent: CallbackAction<_FocusSearchIntent>(
            onInvoke: (_) {
              ref.read(navigationSectionProvider.notifier).state =
                  NavigationSection.chats;
              ref.read(globalSearchFocusRequestProvider.notifier).state++;
              return null;
            },
          ),
          _OpenSettingsIntent: CallbackAction<_OpenSettingsIntent>(
            onInvoke: (_) {
              ref.read(settingsSheetRequestProvider.notifier).state = true;
              return null;
            },
          ),
          _NextUnreadChatIntent: CallbackAction<_NextUnreadChatIntent>(
            onInvoke: (_) {
              ref.read(unreadChatNavigationProvider.notifier).selectNextUnread();
              return null;
            },
          ),
          _PrevUnreadChatIntent: CallbackAction<_PrevUnreadChatIntent>(
            onInvoke: (_) {
              ref.read(unreadChatNavigationProvider.notifier).selectPrevUnread();
              return null;
            },
          ),
          _FocusComposerIntent: CallbackAction<_FocusComposerIntent>(
            onInvoke: (_) {
              ref.read(composerFocusRequestProvider.notifier).state++;
              return null;
            },
          ),
          _NextMessageIntent: CallbackAction<_NextMessageIntent>(
            onInvoke: (_) {
              ref.read(chatMessageKeyboardProvider.notifier).selectNext();
              return null;
            },
          ),
          _PrevMessageIntent: CallbackAction<_PrevMessageIntent>(
            onInvoke: (_) {
              ref.read(chatMessageKeyboardProvider.notifier).selectPrevious();
              return null;
            },
          ),
          _ReplyMessageIntent: CallbackAction<_ReplyMessageIntent>(
            onInvoke: (_) {
              ref.read(chatMessageKeyboardProvider.notifier).replyToSelected();
              return null;
            },
          ),
          _ReactMessageIntent: CallbackAction<_ReactMessageIntent>(
            onInvoke: (_) {
              ref.read(chatMessageKeyboardProvider.notifier).reactToSelected();
              return null;
            },
          ),
          _OpenMessageMenuIntent: CallbackAction<_OpenMessageMenuIntent>(
            onInvoke: (_) {
              ref
                  .read(chatMessageKeyboardProvider.notifier)
                  .openContextMenuOnSelected();
              return null;
            },
          ),
        },
        child: Focus(autofocus: true, child: child),
      ),
    );
  }
}

class _FocusSearchIntent extends Intent {
  const _FocusSearchIntent();
}

class _OpenSettingsIntent extends Intent {
  const _OpenSettingsIntent();
}

class _NextUnreadChatIntent extends Intent {
  const _NextUnreadChatIntent();
}

class _PrevUnreadChatIntent extends Intent {
  const _PrevUnreadChatIntent();
}

class _FocusComposerIntent extends Intent {
  const _FocusComposerIntent();
}

class _NextMessageIntent extends Intent {
  const _NextMessageIntent();
}

class _PrevMessageIntent extends Intent {
  const _PrevMessageIntent();
}

class _ReplyMessageIntent extends Intent {
  const _ReplyMessageIntent();
}

class _ReactMessageIntent extends Intent {
  const _ReactMessageIntent();
}

class _OpenMessageMenuIntent extends Intent {
  const _OpenMessageMenuIntent();
}

/// Signals [VoiceApp] to open settings (Ctrl+,).
final settingsSheetRequestProvider = StateProvider<bool>((ref) => false);

/// Cycles unread chats (Alt+↑/↓).
final unreadChatNavigationProvider =
    NotifierProvider<UnreadChatNavigation, void>(UnreadChatNavigation.new);

class UnreadChatNavigation extends Notifier<void> {
  @override
  void build() {}

  void selectNextUnread() => _selectUnread(step: 1);

  void selectPrevUnread() => _selectUnread(step: -1);

  void _selectUnread({required int step}) {
    final items = ref.read(chatListControllerProvider).items;
    if (items.isEmpty) return;
    final unread = items.where((item) => item.unreadCount > 0).toList();
    if (unread.isEmpty) return;

    final selectedId = ref.read(selectedChatIdProvider);
    var index = unread.indexWhere((item) => item.chatId == selectedId);
    if (index < 0) {
      index = step > 0 ? 0 : unread.length - 1;
    } else {
      index = (index + step) % unread.length;
      if (index < 0) index += unread.length;
    }
    ref.read(shellNavigationProvider).selectChatFromHome(unread[index].chatId);
  }
}

/// Keyboard-selected message in the active chat (↑/↓, R, E).
final chatMessageKeyboardProvider =
    NotifierProvider<ChatMessageKeyboard, String?>(ChatMessageKeyboard.new);

class ChatMessageKeyboard extends Notifier<String?> {
  @override
  String? build() => null;

  void selectNext() => _move(1);

  void selectPrevious() => _move(-1);

  void _move(int delta) {
    final chatId = ref.read(selectedChatIdProvider);
    if (chatId == null) return;
    final messages = ref.read(chatRoomControllerProvider(chatId)).messages;
    if (messages.isEmpty) return;
    final currentIndex = state == null
        ? -1
        : messages.indexWhere((message) => message.id == state);
    var nextIndex = currentIndex + delta;
    if (nextIndex < 0) nextIndex = 0;
    if (nextIndex >= messages.length) nextIndex = messages.length - 1;
    state = messages[nextIndex].id;
    ref.read(pendingChatMessageScrollProvider(chatId).notifier).state =
        messages[nextIndex].id;
  }

  void replyToSelected() {
    final chatId = ref.read(selectedChatIdProvider);
    final messageId = state;
    if (chatId == null || messageId == null) return;
    final messages = ref.read(chatRoomControllerProvider(chatId)).messages;
    VoiceMessage? message;
    for (final item in messages) {
      if (item.id == messageId) {
        message = item;
        break;
      }
    }
    if (message == null) return;
    ref.read(chatReplyTargetProvider(chatId).notifier).state = message;
    ref.read(composerFocusRequestProvider.notifier).state++;
  }

  void reactToSelected() {
    final chatId = ref.read(selectedChatIdProvider);
    final messageId = state;
    if (chatId == null || messageId == null) return;
    ref.read(chatMessageReactionRequestProvider(chatId).notifier).state =
        messageId;
  }

  void openContextMenuOnSelected() {
    final chatId = ref.read(selectedChatIdProvider);
    final messageId = state;
    if (chatId == null || messageId == null) return;
    ref.read(chatMessageContextMenuRequestProvider(chatId).notifier).state =
        messageId;
  }
}

/// Triggers reaction picker for keyboard-selected message.
final chatMessageReactionRequestProvider =
    StateProvider.family<String?, String>((ref, chatId) => null);

/// Triggers context menu for keyboard-selected message (Enter).
final chatMessageContextMenuRequestProvider =
    StateProvider.family<String?, String>((ref, chatId) => null);
