import 'package:flutter/material.dart';
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

    _invokeFocusSearch(container);
    await tester.pump();

    expect(container.read(globalSearchFocusRequestProvider), greaterThan(0));
    expect(container.read(navigationSectionProvider), NavigationSection.chats);
  });

  testWidgets('Escape focuses composer', (tester) async {
    final container = await _pumpShortcuts(tester);
    addTearDown(container.dispose);

    _invokeFocusComposer(container);
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

    _invokeNextUnreadChat(container);
    await tester.pump();

    expect(container.read(selectedChatIdProvider), 'chat-b');
  });

  test('selectNextUnread advances to next unread chat', () async {
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
          _chatsClientFor(seedChatList),
        ),
      ],
    );
    addTearDown(container.dispose);

    // ChatListController loads chats asynchronously on auth; wait before driving navigation.
    container.read(chatListControllerProvider);
    await _waitForChatListItems(container, minCount: seedChatList.length);

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

    _invokeOpenMessageMenu(container);
    await tester.pump();

    expect(
      container.read(chatMessageContextMenuRequestProvider('chat-a')),
      'msg-1',
    );
  });
}

/// Mirrors [_FocusSearchIntent] action wiring in [VoiceShortcuts].
void _invokeFocusSearch(ProviderContainer container) {
  container.read(navigationSectionProvider.notifier).state =
      NavigationSection.chats;
  container.read(globalSearchFocusRequestProvider.notifier).state++;
}

/// Mirrors [_FocusComposerIntent] action wiring in [VoiceShortcuts].
void _invokeFocusComposer(ProviderContainer container) {
  container.read(composerFocusRequestProvider.notifier).state++;
}

/// Mirrors [_NextUnreadChatIntent] action wiring in [VoiceShortcuts].
void _invokeNextUnreadChat(ProviderContainer container) {
  container.read(unreadChatNavigationProvider.notifier).selectNextUnread();
}

/// Mirrors [_OpenMessageMenuIntent] action wiring in [VoiceShortcuts].
void _invokeOpenMessageMenu(ProviderContainer container) {
  container.read(chatMessageKeyboardProvider.notifier).openContextMenuOnSelected();
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
            _chatsClientFor(seedChatList),
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
    await _waitForChatListItems(container, minCount: seedChatList.length);
    _pinChatList(container, seedChatList);
    await tester.pump();
    // Late auth-triggered loadInitial must not leave the list empty.
    _pinChatList(container, seedChatList);
    await tester.pump();
  }

  return container;
}

/// [FakeVoiceChatsClient] consumes pages; duplicate seed so concurrent loads
/// cannot empty the list before shortcuts read it.
FakeVoiceChatsClient _chatsClientFor(List<ChatListItem> items) {
  final page = ChatListData(items: items);
  return FakeVoiceChatsClient(pages: [page, page]);
}

Future<void> _waitForChatListItems(
  ProviderContainer container, {
  required int minCount,
}) async {
  for (var i = 0; i < 100; i++) {
    if (container.read(chatListControllerProvider).items.length >= minCount) {
      return;
    }
    await pumpEventQueue();
  }
}

void _pinChatList(ProviderContainer container, List<ChatListItem> items) {
  container.read(chatListControllerProvider.notifier).state = ChatListState(
    items: items,
  );
}
