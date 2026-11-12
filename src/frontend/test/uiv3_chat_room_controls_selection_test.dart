import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/messages_client.dart';
import 'package:voice_frontend/backend/realtime_client.dart';
import 'package:voice_frontend/backend/space_permissions.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/state/presence_providers.dart';
import 'package:voice_frontend/state/space_providers.dart';
import 'package:voice_frontend/state/subscription_providers.dart';
import 'package:voice_frontend/ui/a11y/voice_shortcuts.dart';
import 'package:voice_frontend/ui/chat/chat_room_panel.dart';
import 'package:voice_frontend/ui/chat/chat_message_list.dart';
import 'package:voice_frontend/ui/chat/forward_message_sheet.dart';
import 'package:voice_frontend/ui/core/premium_badge.dart';

import 'support/auth_test_overrides.dart';
import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

// UIV3 screen-controls.md §3.3 #16, §3.4 #10, §3.5 #1-4, §3.5a, §3.7 #1-3.

ProviderContainer buildRoomContainer({
  required final ChatListController Function(Ref) list,
  required final ChatRoomController Function(Ref) room,
  final List<Override> extra = const [],
}) => ProviderContainer(
  overrides: [
    ...voiceThemeTestOverrides(),
    authSessionStorageProvider.overrideWithValue(InMemoryAuthSessionStorage()),
    authControllerProvider.overrideWith(authenticatedAuthController),
    gatewayConfigProvider.overrideWithValue(
      const GatewayConfig(baseUrl: 'http://api.test'),
    ),
    httpClientProvider.overrideWithValue(
      MockClient((_) async => http.Response('{}', 404)),
    ),
    realtimeHubProvider.overrideWith((ref) => _NoopRealtimeHub(ref)),
    presenceProvider.overrideWith((ref, profileId) => null),
    selectedChatIdProvider.overrideWith((ref) => 'chat-abc'),
    chatListControllerProvider.overrideWith(list),
    chatRoomControllerProvider('chat-abc').overrideWith(room),
    ...extra,
  ],
);

Widget buildApp(ProviderContainer container) => UncontrolledProviderScope(
  container: container,
  child: MaterialApp(
    theme: voiceTestTheme(),
    locale: const Locale('en'),
    localizationsDelegates: AppLocalizations.localizationsDelegates,
    supportedLocales: AppLocalizations.supportedLocales,
    home: const Scaffold(body: ChatRoomPanel(chatId: 'chat-abc')),
  ),
);

AppLocalizations l10nOf(WidgetTester tester) =>
    AppLocalizations.of(tester.element(find.byType(ChatRoomPanel)))!;

// ChatRoomState has no value equality, so assigning a fresh copyWith() copy
// always notifies and forces the panel to re-evaluate selection-only changes.
void forceRoomRebuild(ProviderContainer container) {
  final notifier = container.read(chatRoomControllerProvider('chat-abc').notifier);
  notifier.state = container.read(chatRoomControllerProvider('chat-abc'))
      .copyWith();
}

Future<void> selectMessage(
  WidgetTester tester,
  ProviderContainer container,
  String messageId,
) async {
  final menu = container.read(
    chatMessageContextMenuRequestProvider('chat-abc').notifier,
  );
  menu.state = messageId;
  await tester.pumpAndSettle();
  final tile = find.ancestor(
    of: find.text(l10nOf(tester).chatMessageSelect),
    matching: find.byType(ListTile),
  );
  expect(tile, findsOneWidget);
  await tester.ensureVisible(tile);
  await tester.tap(tile);
  await tester.pumpAndSettle();
  menu.state = null;
  await tester.pumpAndSettle();
  forceRoomRebuild(container);
  await tester.pump();
}

void main() {
  // The FW-05 batch-forward sheet awaits the reduced-motion preference once
  // per process, so the suite must stub SharedPreferences (like the FW-05 and
  // accessibility baselines do).
  setUp(() => SharedPreferences.setMockInitialValues({}));
  testWidgets(
    '§3.4 #10 Select toggle counts into the §3.5 bar and Cancel clears it',
    (tester) async {
      await tester.binding.setSurfaceSize(const Size(1280, 800));
      addTearDown(() => tester.binding.setSurfaceSize(null));
      final container = buildRoomContainer(
        list: (ref) => _GroupChatListController(ref),
        room: (ref) => _TwoMessageRoomController(ref, 'chat-abc'),
      );
      addTearDown(container.dispose);
      await tester.pumpWidget(buildApp(container));
      await tester.pumpAndSettle();
      expect(find.byKey(ChatRoomPanel.multiSelectBarKey), findsNothing);

      await selectMessage(tester, container, 'chat-abc-mine');
      final l10n = l10nOf(tester);
      expect(find.byKey(ChatRoomPanel.multiSelectBarKey), findsOneWidget);
      expect(find.text(l10n.chatMessagesSelected(1)), findsOneWidget);

      await selectMessage(tester, container, 'chat-abc-peer');
      expect(find.text(l10n.chatMessagesSelected(2)), findsOneWidget);

      // Re-tapping Select on the first message toggles it out.
      await selectMessage(tester, container, 'chat-abc-mine');
      expect(find.text(l10n.chatMessagesSelected(1)), findsOneWidget);

      await tester.tap(find.byKey(ChatRoomPanel.multiSelectCancelKey));
      await tester.pumpAndSettle();
      expect(find.byKey(ChatRoomPanel.multiSelectBarKey), findsNothing);
    },
  );

  testWidgets(
    '§3.5 #2 Forward selected opens the FW-05 batch sheet and keeps selection on cancel',
    (tester) async {
      await tester.binding.setSurfaceSize(const Size(390, 844));
      addTearDown(() => tester.binding.setSurfaceSize(null));
      final container = buildRoomContainer(
        list: (ref) => _GroupChatListController(ref),
        room: (ref) => _TwoMessageRoomController(ref, 'chat-abc'),
      );
      addTearDown(container.dispose);
      await tester.pumpWidget(buildApp(container));
      await tester.pumpAndSettle();

      await selectMessage(tester, container, 'chat-abc-mine');
      await selectMessage(tester, container, 'chat-abc-peer');
      final l10n = l10nOf(tester);
      expect(find.text(l10n.chatMessagesSelected(2)), findsOneWidget);

      await tester.tap(find.byKey(ChatRoomPanel.multiSelectForwardKey));
      await tester.pumpAndSettle();
      expect(find.byKey(ForwardMessageSheet.sheetKey), findsOneWidget);
      await tester.tap(find.byKey(ForwardMessageSheet.cancelButtonKey));
      await tester.pumpAndSettle();
      expect(find.byKey(ForwardMessageSheet.sheetKey), findsNothing);
      // Cancelling the sheet never clears the selection set.
      expect(find.byKey(ChatRoomPanel.multiSelectBarKey), findsOneWidget);
      expect(find.text(l10n.chatMessagesSelected(2)), findsOneWidget);
    },
  );

  testWidgets(
    '§3.5 #3 + §3.5a batch delete with MANAGE_MESSAGES deletes foreign space rows for everyone',
    (tester) async {
      await tester.binding.setSurfaceSize(const Size(1280, 800));
      addTearDown(() => tester.binding.setSurfaceSize(null));
      late _TwoMessageRoomController roomController;
      final permissionQuery = (
        spaceId: 'space-1',
        permission: SpacePermissions.textChatManageMessages,
        chatId: 'chat-abc',
        voiceRoomId: null,
      );
      final container = buildRoomContainer(
        list: (ref) => _GroupChatListController(ref),
        room: (ref) => roomController = _TwoMessageRoomController(ref, 'chat-abc'),
        extra: [
          spacePermissionProvider(permissionQuery).overrideWith((ref) async => true),
        ],
      );
      addTearDown(container.dispose);
      await tester.pumpWidget(buildApp(container));
      await tester.pumpAndSettle();

      await selectMessage(tester, container, 'chat-abc-mine');
      await selectMessage(tester, container, 'chat-abc-peer');
      final l10n = l10nOf(tester);
      await tester.tap(find.byKey(ChatRoomPanel.multiSelectDeleteKey));
      await tester.pumpAndSettle();
      expect(find.byKey(ChatRoomPanel.batchDeleteSheetKey), findsOneWidget);

      final everyoneTile = find.ancestor(
        of: find.text(l10n.chatMessageDeleteForEveryone),
        matching: find.byType(ListTile),
      );
      expect(everyoneTile, findsOneWidget);
      await tester.ensureVisible(everyoneTile);
      await tester.tap(everyoneTile);
      await tester.pump();
      expect(
        roomController.deleteMessageCalls,
        [('chat-abc-mine', false), ('chat-abc-peer', false)],
      );
      expect(
        find.descendant(
          of: find.byType(SnackBar),
          matching: find.text(l10n.chatBatchDeleteResult(2, 2)),
        ),
        findsOneWidget,
      );
      await tester.pumpAndSettle();
      forceRoomRebuild(container);
      await tester.pump();
      expect(find.byKey(ChatRoomPanel.multiSelectBarKey), findsNothing);
    },
  );

  testWidgets(
    '§3.5a batch delete without MANAGE_MESSAGES skips the foreign space row',
    (tester) async {
      await tester.binding.setSurfaceSize(const Size(1280, 800));
      addTearDown(() => tester.binding.setSurfaceSize(null));
      late _TwoMessageRoomController roomController;
      final permissionQuery = (
        spaceId: 'space-1',
        permission: SpacePermissions.textChatManageMessages,
        chatId: 'chat-abc',
        voiceRoomId: null,
      );
      final container = buildRoomContainer(
        list: (ref) => _GroupChatListController(ref),
        room: (ref) => roomController = _TwoMessageRoomController(ref, 'chat-abc'),
        extra: [
          spacePermissionProvider(permissionQuery).overrideWith((ref) async => false),
        ],
      );
      addTearDown(container.dispose);
      await tester.pumpWidget(buildApp(container));
      await tester.pumpAndSettle();

      await selectMessage(tester, container, 'chat-abc-mine');
      await selectMessage(tester, container, 'chat-abc-peer');
      final l10n = l10nOf(tester);
      await tester.tap(find.byKey(ChatRoomPanel.multiSelectDeleteKey));
      await tester.pumpAndSettle();
      expect(find.byKey(ChatRoomPanel.batchDeleteSheetKey), findsOneWidget);

      final everyoneTile = find.ancestor(
        of: find.text(l10n.chatMessageDeleteForEveryone),
        matching: find.byType(ListTile),
      );
      expect(everyoneTile, findsOneWidget);
      await tester.ensureVisible(everyoneTile);
      await tester.tap(everyoneTile);
      await tester.pump();
      // The peer row needs MANAGE_MESSAGES, so only the own message deletes.
      expect(roomController.deleteMessageCalls, [('chat-abc-mine', false)]);
      expect(
        find.descendant(
          of: find.byType(SnackBar),
          matching: find.text(l10n.chatBatchDeleteResult(1, 2)),
        ),
        findsOneWidget,
      );
      await tester.pumpAndSettle();
    },
  );

  testWidgets(
    '§3.5a foreign-only selection without MANAGE_MESSAGES explains the denial and keeps selection',
    (tester) async {
      await tester.binding.setSurfaceSize(const Size(1280, 800));
      addTearDown(() => tester.binding.setSurfaceSize(null));
      late _TwoMessageRoomController roomController;
      final permissionQuery = (
        spaceId: 'space-1',
        permission: SpacePermissions.textChatManageMessages,
        chatId: 'chat-abc',
        voiceRoomId: null,
      );
      final container = buildRoomContainer(
        list: (ref) => _GroupChatListController(ref),
        room: (ref) => roomController = _TwoMessageRoomController(ref, 'chat-abc'),
        extra: [
          spacePermissionProvider(permissionQuery).overrideWith((ref) async => false),
        ],
      );
      addTearDown(container.dispose);
      await tester.pumpWidget(buildApp(container));
      await tester.pumpAndSettle();

      await selectMessage(tester, container, 'chat-abc-peer');
      final l10n = l10nOf(tester);
      await tester.tap(find.byKey(ChatRoomPanel.multiSelectDeleteKey));
      await tester.pumpAndSettle();
      expect(roomController.deleteMessageCalls, isEmpty);
      expect(find.byKey(ChatRoomPanel.batchDeleteSheetKey), findsNothing);
      expect(
        find.descendant(
          of: find.byType(SnackBar),
          matching: find.text(l10n.spaceModerationUnavailable),
        ),
        findsOneWidget,
      );
      // The selection stays visible so the user can act differently.
      expect(find.byKey(ChatRoomPanel.multiSelectBarKey), findsOneWidget);
      expect(find.text(l10n.chatMessagesSelected(1)), findsOneWidget);
    },
  );

  testWidgets(
    '§3.5a DM batch offers for-me only and hides the peer row for me',
    (tester) async {
      await tester.binding.setSurfaceSize(const Size(390, 844));
      addTearDown(() => tester.binding.setSurfaceSize(null));
      late _TwoMessageRoomController roomController;
      final container = buildRoomContainer(
        list: (ref) => _DmChatListController(ref),
        room: (ref) => roomController = _TwoMessageRoomController(ref, 'chat-abc'),
      );
      addTearDown(container.dispose);
      await tester.pumpWidget(buildApp(container));
      await tester.pumpAndSettle();

      await selectMessage(tester, container, 'chat-abc-peer');
      final l10n = l10nOf(tester);
      await tester.tap(find.byKey(ChatRoomPanel.multiSelectDeleteKey));
      await tester.pumpAndSettle();
      expect(find.byKey(ChatRoomPanel.batchDeleteSheetKey), findsOneWidget);
      expect(
        find.text(l10n.chatMessageDeleteForEveryone),
        findsNothing, // Peer DM rows can only be hidden for me.
      );
      final forMeTile = find.ancestor(
        of: find.text(l10n.chatMessageDeleteForMe),
        matching: find.byType(ListTile),
      );
      expect(forMeTile, findsOneWidget);
      await tester.ensureVisible(forMeTile);
      await tester.tap(forMeTile);
      await tester.pump();
      expect(roomController.deleteMessageCalls, [('chat-abc-peer', true)]);
      expect(
        find.descendant(
          of: find.byType(SnackBar),
          matching: find.text(l10n.chatBatchDeleteResult(1, 1)),
        ),
        findsOneWidget,
      );
    },
  );

  testWidgets(
    '§3.3 #16 sender premium badge renders in the group author header',
    (tester) async {
      await tester.binding.setSurfaceSize(const Size(390, 844));
      addTearDown(() => tester.binding.setSurfaceSize(null));
      final container = buildRoomContainer(
        list: (ref) => _GroupChatListController(ref),
        room: (ref) => _TwoMessageRoomController(ref, 'chat-abc'),
        extra: [
          profilePremiumBadgeProvider('peer-1').overrideWith((ref) => true),
        ],
      );
      addTearDown(container.dispose);
      await tester.pumpWidget(buildApp(container));
      await tester.pumpAndSettle();
      expect(find.byKey(PremiumBadge.badgeKey), findsOneWidget);
    },
  );

  testWidgets(
    '§3.3 #16 no badge when the sender profile is not premium',
    (tester) async {
      await tester.binding.setSurfaceSize(const Size(1280, 800));
      addTearDown(() => tester.binding.setSurfaceSize(null));
      final container = buildRoomContainer(
        list: (ref) => _GroupChatListController(ref),
        room: (ref) => _TwoMessageRoomController(ref, 'chat-abc'),
        extra: [
          profilePremiumBadgeProvider('peer-1').overrideWith((ref) => false),
        ],
      );
      addTearDown(container.dispose);
      await tester.pumpWidget(buildApp(container));
      await tester.pumpAndSettle();
      expect(find.byKey(PremiumBadge.badgeKey), findsNothing);
    },
  );

  testWidgets(
    '§3.7 #1 scroll-to-bottom FAB appears after scrolling away and jumps to the latest row',
    (tester) async {
      await tester.binding.setSurfaceSize(const Size(1280, 800));
      addTearDown(() => tester.binding.setSurfaceSize(null));
      final container = buildRoomContainer(
        list: (ref) => _DmChatListController(ref, unreadCount: 24),
        room: (ref) => _ManyMessagesRoomController(ref, 'chat-abc'),
      );
      addTearDown(container.dispose);
      await tester.pumpWidget(buildApp(container));
      await tester.pumpAndSettle();
      // Opening the room starts near the newest messages: no FAB.
      expect(find.byKey(ChatRoomPanel.scrollToBottomFabKey), findsNothing);
      expect(find.byType(ChatUnreadSeparator), findsOneWidget);

      await tester.drag(
        find.byKey(ChatRoomPanel.messagesKey),
        const Offset(0, -700),
      );
      await tester.pumpAndSettle();
      forceRoomRebuild(container);
      await tester.pump();
      expect(find.byKey(ChatRoomPanel.scrollToBottomFabKey), findsOneWidget);

      await tester.tap(find.byKey(ChatRoomPanel.scrollToBottomFabKey));
      await tester.pump();
      await tester.pumpAndSettle();
      forceRoomRebuild(container);
      await tester.pump();
      expect(find.byKey(ChatRoomPanel.scrollToBottomFabKey), findsNothing);
      // The top unread separator scrolled off: the room is at the bottom now.
      expect(find.byType(ChatUnreadSeparator), findsNothing);
    },
  );

  testWidgets(
    '§3.7 #2/#3 tapping the unread separator jumps to the first unread message',
    (tester) async {
      await tester.binding.setSurfaceSize(const Size(390, 844));
      addTearDown(() => tester.binding.setSurfaceSize(null));
      final container = buildRoomContainer(
        list: (ref) => _DmChatListController(ref, unreadCount: 4),
        room: (ref) => _ManyMessagesRoomController(ref, 'chat-abc'),
      );
      addTearDown(container.dispose);
      await tester.pumpWidget(buildApp(container));
      await tester.pumpAndSettle();

      // The pan leaves the separator visible in the lower half of the list,
      // with the first unread row already in view.
      await tester.drag(
        find.byKey(ChatRoomPanel.messagesKey),
        const Offset(0, -2400),
      );
      await tester.pumpAndSettle();
      final region = tester.getRect(find.byKey(ChatRoomPanel.messagesKey));
      final separatorCenter = tester.getCenter(
        find.byKey(ChatRoomPanel.unreadSeparatorTapKey),
      );
      await tester.drag(
        find.byKey(ChatRoomPanel.unreadSeparatorTapKey),
        Offset(0, region.bottom - separatorCenter.dy - 30),
      );
      await tester.pumpAndSettle();
      final separator = find.byType(ChatUnreadSeparator);
      expect(separator, findsOneWidget);
      final before = tester.getRect(separator);
      expect(
        before.top,
        greaterThan(region.top + (region.bottom - region.top) * 0.35),
      );

      await tester.tap(find.byKey(ChatRoomPanel.unreadSeparatorTapKey));
      await tester.pump();
      await tester.pumpAndSettle();
      expect(separator, findsOneWidget);
      final after = tester.getRect(separator);
      // §3.7 #2/#3: tapping the separator guarantees the first unread row is
      // in view (ensureVisible jump). The row already is in view here, so the
      // jump leaves the room put: the separator must stay where it was and
      // fully visible inside the list region.
      expect(before.top - after.top, lessThan(50));
      expect(after.top, greaterThan(region.top));
      expect(after.bottom, lessThanOrEqualTo(region.bottom));
    },
  );
}

class _NoopRealtimeHub extends RealtimeHub {
  _NoopRealtimeHub(super.ref);

  @override
  Stream<RealtimeFrame> get events => const Stream.empty();

  @override
  Future<void> ensureConnected() async {}

  @override
  void ensureSubscribed(String chatId) {}
}

class _GroupChatListController extends ChatListController {
  _GroupChatListController(super.ref) : super() {
    state = ChatListState(
      profileId: 'prof-test',
      items: [
        ChatListItem(
          chat: VoiceChat(
            id: 'chat-abc',
            type: 'CHAT_TYPE_GROUP',
            creatorProfileId: 'owner',
            spaceId: 'space-1',
          ),
          unreadCount: 0,
        ),
      ],
    );
  }

  @override
  Future<void> loadInitial() async {}
}

class _DmChatListController extends ChatListController {
  _DmChatListController(super.ref, {this.unreadCount = 0}) : super() {
    state = ChatListState(
      profileId: 'prof-test',
      items: [
        ChatListItem(
          chat: VoiceChat(
            id: 'chat-abc',
            type: 'CHAT_TYPE_DM',
            creatorProfileId: 'prof-test',
          ),
          dmPeerProfileId: 'peer-1',
          unreadCount: unreadCount,
        ),
      ],
    );
  }

  final int unreadCount;

  @override
  Future<void> loadInitial() async {}
}

class _TwoMessageRoomController extends ChatRoomController {
  _TwoMessageRoomController(super.ref, super.chatId) : super() {
    state = ChatRoomState(
      messages: [
        VoiceMessage(
          id: '$chatId-mine',
          chatId: chatId,
          senderProfileId: 'prof-test',
          content: 'Mine to batch',
          createdAt: DateTime.parse('2026-09-20T10:00:00Z'),
        ),
        VoiceMessage(
          id: '$chatId-peer',
          chatId: chatId,
          senderProfileId: 'peer-1',
          content: 'Peer to batch',
          createdAt: DateTime.parse('2026-09-20T10:10:00Z'),
        ),
      ],
      historyProfileId: 'prof-test',
    );
  }

  final deleteMessageCalls = <(String, bool)>[];

  @override
  Future<String?> deleteMessage(String messageId, {required bool forMe}) async {
    deleteMessageCalls.add((messageId, forMe));
    return null;
  }

  @override
  Future<void> loadInitial() async {}
}

class _ManyMessagesRoomController extends ChatRoomController {
  _ManyMessagesRoomController(super.ref, super.chatId) : super() {
    state = ChatRoomState(
      messages: [
        for (var i = 0; i < 24; i++)
          VoiceMessage(
            id: '$chatId-m$i',
            chatId: chatId,
            senderProfileId: i % 2 == 0 ? 'prof-test' : 'peer-1',
            content: 'Chat message $i of the bounded many-message room fixture',
            createdAt: DateTime(2026, 9, 20),
          ),
      ],
      historyProfileId: 'prof-test',
    );
  }

  @override
  Future<void> loadInitial() async {}
}
