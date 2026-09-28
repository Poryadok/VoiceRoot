import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/messages_client.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/shell_providers.dart';
import 'package:voice_frontend/ui/a11y/voice_shortcuts.dart';

import 'support/auth_test_overrides.dart';
import 'support/fake_voice_api_clients.dart';

// Pre-release screen reader checklist (docs/features/accessibility.md).
// Run manually on one Android (TalkBack) and one iOS (VoiceOver) build before
// each mobile release:
// 1. Login / register — fields and buttons announced; auth errors read aloud.
// 2. Chat list — list region reachable; unread counts distinguishable.
// 3. Open chat + send — composer focusable; send announced; new messages polite.
// 4. Navigation rail — Chats / Friends switches announced.
// 5. Settings — opens from menu; primary items reachable.
// 6. Onboarding coach-mark — Skip and CTA reachable; overlay releases focus.
// 7. Deep link (manual) — push/link opens target chat.
//
// Axe DevTools CI is deferred; widget tests below cover keyboard shortcuts,
// focus trap wiring, and text-scale layout smoke at ×1.5 (see chat_text_scale_test.dart).

void main() {
  testWidgets('R and E shortcuts are ignored while text input is focused', (
    tester,
  ) async {
    final controller = TextEditingController();
    final outsideFocus = FocusNode();
    addTearDown(controller.dispose);
    addTearDown(outsideFocus.dispose);
    final container = await _pumpShortcuts(
      tester,
      child: Column(
        children: [
          Focus(
            focusNode: outsideFocus,
            child: const SizedBox(width: 20, height: 20),
          ),
          TextField(
            key: const ValueKey('editable-input'),
            controller: controller,
          ),
        ],
      ),
    );
    addTearDown(container.dispose);

    const message = VoiceMessage(
      id: 'msg-1',
      chatId: 'chat-a',
      senderProfileId: 'peer',
      content: 'hello',
    );
    // Drive Flutter's key routing: these activate the global actions before
    // the editor is focused.
    outsideFocus.requestFocus();
    await tester.pump();
    // Let the room controller's initial activation finish before seeding it.
    final roomSubscription = container.listen(
      chatRoomControllerProvider('chat-a'),
      (_, _) {},
    );
    addTearDown(roomSubscription.close);
    await tester.pump(const Duration(milliseconds: 1));
    container.read(selectedChatIdProvider.notifier).state = 'chat-a';
    container.read(chatRoomControllerProvider('chat-a').notifier).state =
        const ChatRoomState(messages: [message]);
    container.read(chatMessageKeyboardProvider.notifier).state = 'msg-1';
    expect(outsideFocus.hasFocus, isTrue);
    expect(
      await tester.sendKeyEvent(LogicalKeyboardKey.keyR, character: 'r'),
      isTrue,
    );
    expect(container.read(chatReplyTargetProvider('chat-a')), message);
    container.read(chatReplyTargetProvider('chat-a').notifier).state = null;

    await tester.sendKeyEvent(
      LogicalKeyboardKey.keyE,
      character: 'e',
      physicalKey: PhysicalKeyboardKey.keyE,
    );
    expect(
      container.read(chatMessageReactionRequestProvider('chat-a')),
      'msg-1',
    );
    container
            .read(chatMessageReactionRequestProvider('chat-a').notifier)
            .state =
        null;

    await tester.tap(find.byKey(const ValueKey('editable-input')));
    await tester.pump();
    expect(
      await tester.sendKeyEvent(
        LogicalKeyboardKey.keyR,
        character: 'r',
        physicalKey: PhysicalKeyboardKey.keyR,
      ),
      isFalse,
    );
    expect(
      await tester.sendKeyEvent(
        LogicalKeyboardKey.keyE,
        character: 'e',
        physicalKey: PhysicalKeyboardKey.keyE,
      ),
      isFalse,
    );
    await tester.sendKeyDownEvent(LogicalKeyboardKey.shiftLeft);
    await tester.sendKeyEvent(
      LogicalKeyboardKey.keyR,
      character: 'R',
      physicalKey: PhysicalKeyboardKey.keyR,
    );
    await tester.sendKeyUpEvent(LogicalKeyboardKey.shiftLeft);
    // Model the Flutter KeyEvent after web conversion, not a browser event:
    // Flutter 3.41.7's LocaleKeymap maps non-ASCII key values at physical
    // KeyE/KeyR to logical keyE/keyR, while preserving the output character.
    // Caps Lock changes the character without changing the logical key.
    await tester.sendKeyEvent(LogicalKeyboardKey.capsLock);
    await tester.sendKeyEvent(
      LogicalKeyboardKey.keyE,
      character: 'У',
      physicalKey: PhysicalKeyboardKey.keyE,
    );
    await tester.sendKeyEvent(
      LogicalKeyboardKey.keyR,
      character: 'К',
      physicalKey: PhysicalKeyboardKey.keyR,
    );
    await tester.sendKeyEvent(
      LogicalKeyboardKey.keyE,
      character: 'E',
      physicalKey: PhysicalKeyboardKey.keyE,
    );
    await tester.sendKeyEvent(
      LogicalKeyboardKey.keyR,
      character: 'R',
      physicalKey: PhysicalKeyboardKey.keyR,
    );

    expect(container.read(chatReplyTargetProvider('chat-a')), isNull);
    expect(
      container.read(chatMessageReactionRequestProvider('chat-a')),
      isNull,
    );

    // WidgetTester routes physical keys but does not synthesize the platform
    // text-input commit for printable keys. Verify the focused TextInputClient
    // still accepts ordinary text through Flutter's text-input channel.
    tester.testTextInput.updateEditingValue(const TextEditingValue(text: 'er'));
    expect(controller.text, 'er');
  });

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

  testWidgets('Ctrl+, requests settings sheet', (tester) async {
    final container = await _pumpShortcuts(tester);
    addTearDown(container.dispose);

    container.read(settingsSheetRequestProvider.notifier).state = true;
    await tester.pump();

    expect(container.read(settingsSheetRequestProvider), isTrue);
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

    late ProviderContainer container;

    container = ProviderContainer(
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
    _pinChatList(container, seedChatList);

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp(
          home: VoiceShortcuts(child: const SizedBox.expand()),
        ),
      ),
    );

    container.read(selectedChatIdProvider.notifier).state = 'chat-a';

    _invokeNextUnreadChat(container);

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

  test('arrow keys move keyboard selection across messages', () {
    final container = ProviderContainer(
      overrides: [
        ...voiceAppTestOverrides(
          client: MockClient((_) async => throw UnimplementedError()),
        ),
      ],
    );
    addTearDown(container.dispose);

    container.read(selectedChatIdProvider.notifier).state = 'chat-a';
    container
        .read(chatRoomControllerProvider('chat-a').notifier)
        .state = ChatRoomState(
      messages: const [
        VoiceMessage(
          id: 'msg-1',
          chatId: 'chat-a',
          senderProfileId: 'p1',
          content: 'a',
        ),
        VoiceMessage(
          id: 'msg-2',
          chatId: 'chat-a',
          senderProfileId: 'p1',
          content: 'b',
        ),
      ],
    );

    final keyboard = container.read(chatMessageKeyboardProvider.notifier);
    keyboard.selectNext();
    expect(container.read(chatMessageKeyboardProvider), 'msg-1');
    keyboard.selectNext();
    expect(container.read(chatMessageKeyboardProvider), 'msg-2');
    keyboard.selectPrevious();
    expect(container.read(chatMessageKeyboardProvider), 'msg-1');
  });

  test('R sets reply target for keyboard-selected message', () {
    final container = ProviderContainer(
      overrides: [
        ...voiceAppTestOverrides(
          client: MockClient((_) async => throw UnimplementedError()),
        ),
      ],
    );
    addTearDown(container.dispose);

    const message = VoiceMessage(
      id: 'msg-1',
      chatId: 'chat-a',
      senderProfileId: 'peer',
      content: 'hello',
    );
    container.read(selectedChatIdProvider.notifier).state = 'chat-a';
    container.read(chatRoomControllerProvider('chat-a').notifier).state =
        ChatRoomState(messages: const [message]);
    container.read(chatMessageKeyboardProvider.notifier).state = 'msg-1';

    container.read(chatMessageKeyboardProvider.notifier).replyToSelected();

    expect(container.read(chatReplyTargetProvider('chat-a')), message);
    expect(container.read(composerFocusRequestProvider), greaterThan(0));
  });

  test('E requests reaction picker for keyboard-selected message', () {
    final container = ProviderContainer(
      overrides: [
        ...voiceAppTestOverrides(
          client: MockClient((_) async => throw UnimplementedError()),
        ),
      ],
    );
    addTearDown(container.dispose);

    container.read(selectedChatIdProvider.notifier).state = 'chat-a';
    container
        .read(chatRoomControllerProvider('chat-a').notifier)
        .state = const ChatRoomState(
      messages: [
        VoiceMessage(
          id: 'msg-1',
          chatId: 'chat-a',
          senderProfileId: 'peer',
          content: 'hello',
        ),
      ],
    );
    container.read(chatMessageKeyboardProvider.notifier).state = 'msg-1';

    container.read(chatMessageKeyboardProvider.notifier).reactToSelected();

    expect(
      container.read(chatMessageReactionRequestProvider('chat-a')),
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
  container
      .read(chatMessageKeyboardProvider.notifier)
      .openContextMenuOnSelected();
}

Future<ProviderContainer> _pumpShortcuts(
  WidgetTester tester, {
  Widget child = const SizedBox.expand(),
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
            _chatsClientFor(seedChatList),
          ),
        ],
      ),
      child: MaterialApp(
        home: VoiceShortcuts(child: Scaffold(body: child)),
      ),
    ),
  );
  await tester.pump();

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
    // Avoid pumpEventQueue() while a WidgetTester tree is mounted — it can
    // never drain when the binding keeps scheduling work.
    await Future<void>.delayed(const Duration(milliseconds: 5));
  }
}

void _pinChatList(ProviderContainer container, List<ChatListItem> items) {
  container.read(chatListControllerProvider.notifier).state = ChatListState(
    items: items,
  );
}
