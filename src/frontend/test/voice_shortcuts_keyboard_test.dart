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
    );
    addTearDown(container.dispose);

    container.read(chatListControllerProvider.notifier).state = ChatListState(
      items: seedChatList,
    );
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

    container.read(chatListControllerProvider);
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
  container.read(chatListControllerProvider);
  await pumpEventQueue(times: 50);
  return container;
}

Future<void> _sendShortcut(WidgetTester tester, SingleActivator activator) async {
  final keys = <LogicalKeyboardKey>[];
  if (activator.control) keys.add(LogicalKeyboardKey.controlLeft);
  if (activator.alt) keys.add(LogicalKeyboardKey.altLeft);
  if (activator.shift) keys.add(LogicalKeyboardKey.shiftLeft);
  if (activator.meta) keys.add(LogicalKeyboardKey.metaLeft);
  keys.add(activator.trigger);

  for (final key in keys) {
    await tester.sendKeyDownEvent(key);
  }
  for (final key in keys.reversed) {
    await tester.sendKeyUpEvent(key);
  }
}
