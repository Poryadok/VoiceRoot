import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/shell_providers.dart';
import 'package:voice_frontend/ui/a11y/voice_shortcuts.dart';

import 'support/auth_test_overrides.dart';
import 'support/fake_voice_api_clients.dart';

void main() {
  testWidgets('Ctrl+K focuses global search', (tester) async {
    final container = await _pumpShortcuts(tester);
    addTearDown(container.dispose);

    await _sendShortcut(
      tester,
      const SingleActivator(LogicalKeyboardKey.keyK, control: true),
    );
    await tester.pump();

    expect(container.read(globalSearchFocusRequestProvider), greaterThan(0));
    expect(container.read(navigationSectionProvider), NavigationSection.chats);
  });

  testWidgets('Escape focuses composer', (tester) async {
    final container = await _pumpShortcuts(tester);
    addTearDown(container.dispose);

    await _sendShortcut(
      tester,
      const SingleActivator(LogicalKeyboardKey.escape),
    );
    await tester.pump();

    expect(container.read(composerFocusRequestProvider), greaterThan(0));
  });

  testWidgets('Alt+Down selects next unread chat', (tester) async {
    const seedChatList = [
      ChatListItem(
        chat: VoiceChat(
          id: 'chat-a',
          type: 'CHAT_TYPE_DM',
          creatorProfileId: 'peer-a',
        ),
        unreadCount: 2,
      ),
      ChatListItem(
        chat: VoiceChat(
          id: 'chat-b',
          type: 'CHAT_TYPE_DM',
          creatorProfileId: 'peer-b',
        ),
        unreadCount: 1,
      ),
    ];

    final container = await _pumpShortcuts(
      tester,
      seedChatList: seedChatList,
      prepareChatList: true,
    );
    addTearDown(container.dispose);

    container.read(selectedChatIdProvider.notifier).state = 'chat-a';

    await _sendShortcut(
      tester,
      const SingleActivator(LogicalKeyboardKey.arrowDown, alt: true),
    );
    await tester.pump();

    expect(container.read(selectedChatIdProvider), 'chat-b');
  });

  test('selectNextUnread advances to next unread chat', () {
    const seedChatList = [
      ChatListItem(
        chat: VoiceChat(
          id: 'chat-a',
          type: 'CHAT_TYPE_DM',
          creatorProfileId: 'peer-a',
        ),
        unreadCount: 2,
      ),
      ChatListItem(
        chat: VoiceChat(
          id: 'chat-b',
          type: 'CHAT_TYPE_DM',
          creatorProfileId: 'peer-b',
        ),
        unreadCount: 1,
      ),
    ];

    final container = ProviderContainer(
      overrides: [
        ...voiceAppTestOverrides(
          client: MockClient((_) async => throw UnimplementedError()),
        ),
        voiceChatsClientProvider.overrideWithValue(
          FakeVoiceChatsClient(pages: [ChatListData(items: seedChatList)]),
        ),
      ],
    );
    addTearDown(container.dispose);

    container.read(chatListControllerProvider.notifier).state = ChatListState(
      items: seedChatList,
    );
    container.read(selectedChatIdProvider.notifier).state = 'chat-a';
    container.read(unreadChatNavigationProvider.notifier).selectNextUnread();

    expect(container.read(selectedChatIdProvider), 'chat-b');
  });

  testWidgets('Enter opens context menu request for selected message', (
    tester,
  ) async {
    final container = await _pumpShortcuts(tester);
    addTearDown(container.dispose);

    container.read(selectedChatIdProvider.notifier).state = 'chat-a';
    container.read(chatMessageKeyboardProvider.notifier).state = 'msg-1';
    await tester.pump();

    await _sendShortcut(
      tester,
      const SingleActivator(LogicalKeyboardKey.enter),
    );
    await tester.pump();

    expect(
      container.read(chatMessageContextMenuRequestProvider('chat-a')),
      'msg-1',
    );
  });
}

Future<ProviderContainer> _pumpShortcuts(
  WidgetTester tester, {
  List<ChatListItem> seedChatList = const [
    ChatListItem(
      chat: VoiceChat(
        id: 'chat-a',
        type: 'CHAT_TYPE_DM',
        creatorProfileId: 'peer-a',
      ),
      unreadCount: 1,
    ),
  ],
  bool prepareChatList = false,
}) async {
  late ProviderContainer container;

  await tester.pumpWidget(
    UncontrolledProviderScope(
      container: container = ProviderContainer(
        overrides: [
          ...voiceAppTestOverrides(
            client: MockClient((_) async => throw UnimplementedError()),
          ),
          voiceChatsClientProvider.overrideWithValue(
            FakeVoiceChatsClient(pages: [ChatListData(items: seedChatList)]),
          ),
        ],
      ),
      child: MaterialApp(
        home: VoiceShortcuts(
          child: const SizedBox.expand(),
        ),
      ),
    ),
  );
  await tester.pump();

  if (prepareChatList) {
    container.read(chatListControllerProvider);
    await pumpEventQueue(times: 50);
    container.read(chatListControllerProvider.notifier).state = ChatListState(
      items: seedChatList,
    );
    await tester.pump();
  }

  return container;
}

Future<void> _ensureShortcutsFocused(WidgetTester tester) async {
  final node = tester.widget<Focus>(find.byType(Focus)).focusNode!;
  node.requestFocus();
  await tester.pump();
}

Future<void> _sendShortcut(WidgetTester tester, SingleActivator activator) async {
  await _ensureShortcutsFocused(tester);

  if (activator.control) {
    await tester.sendKeyDownEvent(LogicalKeyboardKey.controlLeft);
  }
  if (activator.alt) {
    await tester.sendKeyDownEvent(LogicalKeyboardKey.altLeft);
  }
  if (activator.shift) {
    await tester.sendKeyDownEvent(LogicalKeyboardKey.shiftLeft);
  }
  if (activator.meta) {
    await tester.sendKeyDownEvent(LogicalKeyboardKey.metaLeft);
  }

  await tester.sendKeyDownEvent(activator.trigger);
  await tester.sendKeyUpEvent(activator.trigger);

  if (activator.meta) {
    await tester.sendKeyUpEvent(LogicalKeyboardKey.metaLeft);
  }
  if (activator.shift) {
    await tester.sendKeyUpEvent(LogicalKeyboardKey.shiftLeft);
  }
  if (activator.alt) {
    await tester.sendKeyUpEvent(LogicalKeyboardKey.altLeft);
  }
  if (activator.control) {
    await tester.sendKeyUpEvent(LogicalKeyboardKey.controlLeft);
  }
}
