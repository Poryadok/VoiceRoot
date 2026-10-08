import 'dart:async';
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session.dart';
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
import 'package:voice_frontend/state/space_providers.dart';
import 'package:voice_frontend/shell/three_column_shell.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/ui/chat/chat_room_panel.dart';
import 'package:voice_frontend/ui/chat/chat_message_list.dart';
import 'package:voice_frontend/ui/core/voice_state_panel.dart';
import 'package:voice_frontend/ui/core/voice_skeleton.dart';
import 'package:voice_frontend/ui/a11y/voice_shortcuts.dart';
import 'package:voice_frontend/ui/shell/chat_list_body.dart';

import 'support/auth_test_overrides.dart';
import 'support/markdown_test_helpers.dart';
import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

void main() {
  testWidgets(
    'space message moderation has no delete affordance while permission is unresolved or denied',
    (tester) async {
      final moderationPermission = Completer<bool>();
      final query = (
        spaceId: 'space-1',
        permission: SpacePermissions.textChatManageMessages,
        chatId: 'chat-abc',
        voiceRoomId: null,
      );
      final container = ProviderContainer(
        overrides: [
          ...voiceThemeTestOverrides(),
          profileAccentStorageProvider.overrideWithValue(
            testProfileAccentStorage,
          ),
          authSessionStorageProvider.overrideWithValue(
            InMemoryAuthSessionStorage(),
          ),
          authControllerProvider.overrideWith(authenticatedAuthController),
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(baseUrl: 'http://api.test'),
          ),
          httpClientProvider.overrideWithValue(
            MockClient((_) async => http.Response('{}', 404)),
          ),
          realtimeHubProvider.overrideWith((ref) => _NoopRealtimeHub(ref)),
          chatListControllerProvider.overrideWith(_SpaceChatListController.new),
          chatRoomControllerProvider('chat-abc').overrideWith(
            (ref) => _SingleMessageRoomController(ref, 'chat-abc'),
          ),
          spacePermissionProvider(query).overrideWith(
            (ref) => moderationPermission.future,
          ),
        ],
      );
      addTearDown(container.dispose);

      await tester.pumpWidget(
        UncontrolledProviderScope(
          container: container,
          child: MaterialApp(
            theme: voiceTestTheme(),
            locale: const Locale('en'),
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: const Scaffold(body: ChatRoomPanel(chatId: 'chat-abc')),
          ),
        ),
      );
      await tester.pumpAndSettle();

      container
          .read(chatMessageContextMenuRequestProvider('chat-abc').notifier)
          .state = 'chat-abc-message';
      await tester.pumpAndSettle();

      final checkingPermission = find.ancestor(
        of: find.text('Checking permissions…'),
        matching: find.byType(ListTile),
      );
      expect(checkingPermission, findsOneWidget);
      expect(find.text('Delete for everyone'), findsNothing);
      expect(
        tester.widget<ListTile>(checkingPermission).onTap,
        isNull,
        reason: 'an unresolved permission check must expose no delete action',
      );
      expect(find.byTooltip('Checking permissions…'), findsOneWidget);

      moderationPermission.complete(false);
      await tester.pumpAndSettle();

      expect(find.text('Delete for everyone'), findsNothing);
      final unavailableModeration = find.ancestor(
        of: find.text('Message moderation is unavailable.'),
        matching: find.byType(ListTile),
      );
      expect(unavailableModeration, findsOneWidget);
      expect(tester.widget<ListTile>(unavailableModeration).onTap, isNull);
      expect(
        find.byTooltip(
          'Message moderation is unavailable.',
        ),
        findsOneWidget,
      );
    },
  );

  testWidgets(
    'Space moderator can delete another member message for everyone',
    (tester) async {
      final query = (
        spaceId: 'space-1',
        permission: SpacePermissions.textChatManageMessages,
        chatId: 'chat-abc',
        voiceRoomId: null,
      );
      late _SingleMessageRoomController roomController;
      final container = ProviderContainer(
        overrides: [
          ...voiceThemeTestOverrides(),
          profileAccentStorageProvider.overrideWithValue(
            testProfileAccentStorage,
          ),
          authSessionStorageProvider.overrideWithValue(
            InMemoryAuthSessionStorage(),
          ),
          authControllerProvider.overrideWith(authenticatedAuthController),
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(baseUrl: 'http://api.test'),
          ),
          httpClientProvider.overrideWithValue(
            MockClient((_) async => http.Response('{}', 404)),
          ),
          realtimeHubProvider.overrideWith((ref) => _NoopRealtimeHub(ref)),
          chatListControllerProvider.overrideWith(_SpaceChatListController.new),
          chatRoomControllerProvider('chat-abc').overrideWith((ref) {
            return roomController = _SingleMessageRoomController(
              ref,
              'chat-abc',
            );
          }),
          spacePermissionProvider(query).overrideWith((ref) async => true),
        ],
      );
      addTearDown(container.dispose);

      await tester.pumpWidget(
        UncontrolledProviderScope(
          container: container,
          child: MaterialApp(
            theme: voiceTestTheme(),
            locale: const Locale('en'),
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: const Scaffold(body: ChatRoomPanel(chatId: 'chat-abc')),
          ),
        ),
      );
      await tester.pumpAndSettle();

      container
          .read(chatMessageContextMenuRequestProvider('chat-abc').notifier)
          .state = 'chat-abc-message';
      await tester.pumpAndSettle();

      final deleteForEveryone = find.ancestor(
        of: find.text('Delete for everyone'),
        matching: find.byType(ListTile),
      );
      expect(deleteForEveryone, findsOneWidget);
      expect(tester.widget<ListTile>(deleteForEveryone).onTap, isNotNull);
      await tester.ensureVisible(deleteForEveryone);
      await tester.tap(deleteForEveryone);
      await tester.pumpAndSettle();

      expect(roomController.deleteMessageCalls, [('chat-abc-message', false)]);
    },
  );

  testWidgets(
    'switching to a chat with no unread messages hides the previous unread separator',
    (tester) async {
      addTearDown(() => tester.binding.setSurfaceSize(null));
      await tester.binding.setSurfaceSize(const Size(900, 600));
      final container = ProviderContainer(
        overrides: [
          ...voiceThemeTestOverrides(),
          profileAccentStorageProvider.overrideWithValue(
            testProfileAccentStorage,
          ),
          authSessionStorageProvider.overrideWithValue(
            InMemoryAuthSessionStorage(),
          ),
          authControllerProvider.overrideWith(authenticatedAuthController),
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(baseUrl: 'http://api.test'),
          ),
          httpClientProvider.overrideWithValue(
            MockClient((_) async => http.Response('{}', 404)),
          ),
          realtimeHubProvider.overrideWith((ref) => _NoopRealtimeHub(ref)),
          selectedChatIdProvider.overrideWith((ref) => 'chat-a'),
          chatListControllerProvider.overrideWith(
            _UnreadTwoChatListController.new,
          ),
          chatRoomControllerProvider('chat-a').overrideWith(
            (ref) => _SingleMessageRoomController(ref, 'chat-a'),
          ),
          chatRoomControllerProvider('chat-b').overrideWith(
            (ref) => _SingleMessageRoomController(ref, 'chat-b'),
          ),
        ],
      );
      addTearDown(container.dispose);
      await tester.pumpWidget(
        UncontrolledProviderScope(
          container: container,
          child: MaterialApp(
            theme: voiceTestTheme(),
            locale: const Locale('en'),
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: Scaffold(
              body: ThreeColumnShell(
                navigationChild: const ChatListBody(showHeader: false),
                mainChild: Consumer(
                  builder: (context, ref, _) {
                    return ChatRoomPanel(
                      chatId: ref.watch(selectedChatIdProvider)!,
                    );
                  },
                ),
              ),
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();

      expect(container.read(selectedChatIdProvider), 'chat-a');
      expectMessagePlainText(tester, 'Message for chat-a');
      expect(find.byType(ChatUnreadSeparator), findsOneWidget);

      await tester.tap(find.byKey(ChatListBody.tileKey('chat-b')));
      await tester.pumpAndSettle();

      expect(container.read(selectedChatIdProvider), 'chat-b');
      expectMessagePlainText(tester, 'Message for chat-b');
      expect(find.byType(ChatUnreadSeparator), findsNothing);
      await tester.pumpWidget(const SizedBox.shrink());
    },
  );

  testWidgets(
    'switching chats keeps a visible room loading state without a progress line',
    (tester) async {
      addTearDown(() => tester.binding.setSurfaceSize(null));
      await tester.binding.setSurfaceSize(const Size(900, 600));
      final container = ProviderContainer(
        overrides: [
          ...voiceThemeTestOverrides(),
          profileAccentStorageProvider.overrideWithValue(
            testProfileAccentStorage,
          ),
          authSessionStorageProvider.overrideWithValue(
            InMemoryAuthSessionStorage(),
          ),
          authControllerProvider.overrideWith(authenticatedAuthController),
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(baseUrl: 'http://api.test'),
          ),
          httpClientProvider.overrideWithValue(
            MockClient((_) async => http.Response('{}', 404)),
          ),
          realtimeHubProvider.overrideWith((ref) => _NoopRealtimeHub(ref)),
          selectedChatIdProvider.overrideWith((ref) => 'chat-a'),
          chatListControllerProvider.overrideWith(_TwoChatListController.new),
          chatRoomControllerProvider(
            'chat-a',
          ).overrideWith((ref) => _EmptyRoomController(ref, 'chat-a')),
          chatRoomControllerProvider(
            'chat-b',
          ).overrideWith((ref) => _LoadingRoomController(ref, 'chat-b')),
        ],
      );
      addTearDown(container.dispose);
      await tester.pumpWidget(
        UncontrolledProviderScope(
          container: container,
          child: MaterialApp(
            theme: voiceTestTheme(),
            locale: const Locale('en'),
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: Scaffold(
              body: ThreeColumnShell(
                navigationChild: const ChatListBody(showHeader: false),
                mainChild: Consumer(
                  builder: (context, ref, _) {
                    return ChatRoomPanel(
                      chatId: ref.watch(selectedChatIdProvider)!,
                    );
                  },
                ),
              ),
            ),
          ),
        ),
      );
      await tester.pump(const Duration(milliseconds: 1));
      final navigationElement = tester.element(
        find.byKey(ThreeColumnShell.navActiveRail),
      );
      expect(find.byKey(ChatListBody.tileKey('chat-b')), findsOneWidget);
      await tester.tap(find.byKey(ChatListBody.tileKey('chat-b')));
      await tester.pump(const Duration(milliseconds: 1));

      expect(container.read(selectedChatIdProvider), 'chat-b');
      expect(tester.element(find.byKey(ThreeColumnShell.navActiveRail)),
          same(navigationElement));
      expect(find.byKey(ChatListBody.tileKey('chat-a')), findsOneWidget);
      expect(find.byKey(ChatListBody.tileKey('chat-b')), findsOneWidget);
      expect(
        find.descendant(
          of: find.byKey(ThreeColumnShell.navOpenChat),
          matching: find.byType(VoiceListSkeleton),
        ),
        findsOneWidget,
      );
      expect(find.byType(LinearProgressIndicator), findsNothing);
      await tester.pumpWidget(const SizedBox.shrink());
      await tester.pump(const Duration(milliseconds: 1));
    },
  );

  testWidgets(
    'hides previous-profile history until active-profile history binds',
    (tester) async {
      late _ProfileBoundRoomController room;
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            ...voiceThemeTestOverrides(),
            profileAccentStorageProvider.overrideWithValue(
              testProfileAccentStorage,
            ),
            authSessionStorageProvider.overrideWithValue(
              InMemoryAuthSessionStorage(),
            ),
            authControllerProvider.overrideWith(_ProfileBAuthController.new),
            gatewayConfigProvider.overrideWithValue(
              const GatewayConfig(baseUrl: 'http://api.test'),
            ),
            httpClientProvider.overrideWithValue(
              MockClient((_) async => http.Response('{}', 404)),
            ),
            realtimeHubProvider.overrideWith((ref) => _NoopRealtimeHub(ref)),
            selectedChatIdProvider.overrideWith((ref) => 'other-chat'),
            chatRoomControllerProvider('chat-abc').overrideWith((ref) {
              room = _ProfileBoundRoomController(ref, 'chat-abc');
              return room;
            }),
          ],
          child: MaterialApp(
            theme: voiceTestTheme(),
            locale: const Locale('en'),
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: const Scaffold(body: ChatRoomPanel(chatId: 'chat-abc')),
          ),
        ),
      );
      await tester.pump();

      expect(room.state.messages.single.content, 'from profile A');
      expect(room.state.nextCursor, 'profile-a-cursor');
      expect(find.text('from profile A'), findsNothing);
      expect(find.byType(VoiceStatePanel), findsNothing);

      room.bindProfileB();
      await tester.pump();

      expectMessagePlainText(tester, 'from profile B');
      expect(richPlainText(tester), isNot(contains('from profile A')));
      expect(find.byType(VoiceStatePanel), findsNothing);
    },
  );

  testWidgets('ChatRoomPanel composer has no @ mention button', (tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceThemeTestOverrides(),
          profileAccentStorageProvider.overrideWithValue(
            testProfileAccentStorage,
          ),
          authSessionStorageProvider.overrideWithValue(
            InMemoryAuthSessionStorage(),
          ),
          authControllerProvider.overrideWith(authenticatedAuthController),
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(baseUrl: 'http://api.test'),
          ),
          httpClientProvider.overrideWithValue(
            MockClient((req) async {
              if (req.url.path == '/api/v1/messages') {
                return http.Response(
                  jsonEncode({'message_list': {'messages': []}}),
                  200,
                );
              }
              if (req.url.path == '/api/v1/messages/read') {
                return http.Response('{}', 200);
              }
              return http.Response('{}', 404);
            }),
          ),
          realtimeHubProvider.overrideWith((ref) => _NoopRealtimeHub(ref)),
          chatRoomControllerProvider('chat-abc').overrideWith(
            (ref) => _EmptyRoomController(ref, 'chat-abc'),
          ),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(body: ChatRoomPanel(chatId: 'chat-abc')),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('chat_room_mention_button')), findsNothing);
    expect(find.byIcon(Icons.alternate_email), findsNothing);
  });

  testWidgets('blocked DM send keeps draft and shows neutral error', (
    tester,
  ) async {
    var sendCalls = 0;
    final sentBodies = <String>[];
    late _EmptyRoomController room;
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceThemeTestOverrides(),
          profileAccentStorageProvider.overrideWithValue(
            testProfileAccentStorage,
          ),
          authSessionStorageProvider.overrideWithValue(
            InMemoryAuthSessionStorage(),
          ),
          authControllerProvider.overrideWith(authenticatedAuthController),
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(baseUrl: 'http://api.test'),
          ),
          httpClientProvider.overrideWithValue(
            MockClient((req) async {
              if (req.url.path == '/api/v1/messages/send') {
                sendCalls++;
                sentBodies.add(utf8.decode(req.bodyBytes));
                return http.Response(
                  jsonEncode({
                    'error_code': 'permission_denied',
                    'message': 'cannot send messages between blocked accounts',
                  }),
                  403,
                );
              }
              return http.Response('{}', 404);
            }),
          ),
          realtimeHubProvider.overrideWith((ref) => _NoopRealtimeHub(ref)),
          chatListControllerProvider.overrideWith(_BlockedDmChatListController.new),
          chatListProvider.overrideWith(
            (ref) async => const ChatListData(items: [
              ChatListItem(
                chat: VoiceChat(
                  id: 'chat-abc',
                  type: 'CHAT_TYPE_DM',
                  creatorProfileId: 'prof-test',
                ),
                dmPeerProfileId: 'peer-1',
              ),
            ]),
          ),
          chatRoomControllerProvider(
            'chat-abc',
          ).overrideWith((ref) {
            room = _EmptyRoomController(ref, 'chat-abc');
            return room;
          }),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(body: ChatRoomPanel(chatId: 'chat-abc')),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('User unavailable'), findsOneWidget);
    expect(find.text('Chat chat-abc'), findsNothing);

    await tester.enterText(
      find.byKey(ChatRoomPanel.inputKey),
      'Draft stays here',
    );
    await tester.tap(find.byKey(ChatRoomPanel.sendKey));
    await tester.pumpAndSettle();

    final input = tester.widget<TextField>(
      find.descendant(
        of: find.byKey(ChatRoomPanel.inputKey),
        matching: find.byType(TextField),
      ),
    );
    expect(sendCalls, 1);
    expect(input.controller?.text, 'Draft stays here');
    expect(
      find.text('Messages are disabled between these accounts'),
      findsOneWidget,
    );
    expect(
      find.textContaining('cannot send messages between blocked accounts'),
      findsNothing,
    );
    expect(room.state.messages, isEmpty);
    expect(find.byType(ChatMessageBubbleTile), findsNothing);
    expect(find.byKey(ChatRoomPanel.messagesKey), findsNothing);
    expect(sendCalls, 1, reason: 'a failed send must not retry automatically');
    expect(find.byKey(ChatRoomPanel.sendFailureBannerKey), findsOneWidget);

    await tester.tap(
      find.descendant(
        of: find.byKey(ChatRoomPanel.sendFailureBannerKey),
        matching: find.text('Retry'),
      ),
    );
    await tester.pumpAndSettle();

    expect(sendCalls, 2, reason: 'Retry sends exactly one new request');
    expect(sentBodies, hasLength(2));
    expect(sentBodies[1], sentBodies[0]);
    expect(
      tester
          .widget<TextField>(
            find.descendant(
              of: find.byKey(ChatRoomPanel.inputKey),
              matching: find.byType(TextField),
            ),
          )
          .controller
          ?.text,
      'Draft stays here',
    );

    room.state = ChatRoomState(
      messages: [
        VoiceMessage(
          id: 'chat-abc-existing',
          chatId: 'chat-abc',
          senderProfileId: 'peer-1',
          content: 'Existing history',
          createdAt: DateTime.utc(2026, 9, 20),
        ),
      ],
      historyProfileId: 'prof-test',
    );
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(ChatRoomPanel.sendKey));
    await tester.pumpAndSettle();

    expect(sendCalls, 3);
    expect(find.byKey(ChatRoomPanel.messagesKey), findsOneWidget);
    expect(find.byKey(ChatRoomPanel.sendFailureBannerKey), findsOneWidget);
    expect(room.state.messages.single.content, 'Existing history');

    await tester.tap(
      find.descendant(
        of: find.byKey(ChatRoomPanel.sendFailureBannerKey),
        matching: find.text('Retry'),
      ),
    );
    await tester.pumpAndSettle();

    expect(sendCalls, 4);
    expect(sentBodies, hasLength(4));
    expect(sentBodies[2], sentBodies[3]);
    expect(room.state.messages.single.content, 'Existing history');
    expect(
      find.text('cannot send messages between blocked accounts'),
      findsNothing,
    );
  });
}

class _EmptyRoomController extends ChatRoomController {
  _EmptyRoomController(super.ref, super.chatId) : super() {
    state = const ChatRoomState(messages: []);
  }

  @override
  Future<void> loadInitial() async {}
}

class _LoadingRoomController extends ChatRoomController {
  _LoadingRoomController(super.ref, super.chatId) : super() {
    state = const ChatRoomState(isLoading: true);
  }

  @override
  Future<void> loadInitial() async {}
}

class _BlockedDmChatListController extends ChatListController {
  _BlockedDmChatListController(super.ref) : super() {
    state = const ChatListState(
      profileId: 'prof-test',
      items: [
        ChatListItem(
          chat: VoiceChat(
            id: 'chat-abc',
            type: 'CHAT_TYPE_DM',
            creatorProfileId: 'prof-test',
          ),
          dmPeerProfileId: 'peer-1',
        ),
      ],
    );
  }

  @override
  Future<void> loadInitial() async {}
}

class _SpaceChatListController extends ChatListController {
  _SpaceChatListController(super.ref) : super() {
    state = const ChatListState(
      profileId: 'prof-test',
      items: [
        ChatListItem(
          chat: VoiceChat(
            id: 'chat-abc',
            type: 'CHAT_TYPE_GROUP',
            creatorProfileId: 'owner',
            spaceId: 'space-1',
          ),
        ),
      ],
    );
  }

  @override
  Future<void> loadInitial() async {}
}

class _UnreadTwoChatListController extends ChatListController {
  _UnreadTwoChatListController(super.ref) : super() {
    state = const ChatListState(
      profileId: 'prof-test',
      items: [
        ChatListItem(
          chat: VoiceChat(
            id: 'chat-a',
            type: 'CHAT_TYPE_GROUP',
            name: 'Chat A',
            creatorProfileId: 'prof-test',
          ),
          unreadCount: 1,
        ),
        ChatListItem(
          chat: VoiceChat(
            id: 'chat-b',
            type: 'CHAT_TYPE_GROUP',
            name: 'Chat B',
            creatorProfileId: 'prof-test',
          ),
        ),
      ],
    );
  }

  @override
  Future<void> loadInitial() async {}

  @override
  Future<void> loadMore() async {}
}

class _SingleMessageRoomController extends ChatRoomController {
  _SingleMessageRoomController(super.ref, super.chatId) : super() {
    state = ChatRoomState(
      messages: [
        VoiceMessage(
          id: '$chatId-message',
          chatId: chatId,
          senderProfileId: 'peer-1',
          content: 'Message for $chatId',
          createdAt: DateTime.parse('2026-09-20T00:00:00Z'),
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

class _TwoChatListController extends ChatListController {
  _TwoChatListController(super.ref) : super() {
    state = ChatListState(profileId: 'prof-test', items: [
      ChatListItem(chat: VoiceChat(
        id: 'chat-a', type: 'CHAT_TYPE_GROUP', name: 'Chat A',
        creatorProfileId: 'prof-test',
      )),
      ChatListItem(chat: VoiceChat(
        id: 'chat-b', type: 'CHAT_TYPE_GROUP', name: 'Chat B',
        creatorProfileId: 'prof-test',
      )),
    ]);
  }

  @override
  Future<void> loadInitial() async {}

  @override
  Future<void> loadMore() async {}
}

class _ProfileBoundRoomController extends ChatRoomController {
  _ProfileBoundRoomController(super.ref, super.chatId) : super() {
    state = ChatRoomState(
      messages: [_message('profile-a-message', 'from profile A')],
      isLoading: true,
      nextCursor: 'profile-a-cursor',
      hasMore: true,
      historyProfileId: 'profile-a',
    );
  }

  void bindProfileB() {
    state = ChatRoomState(
      messages: [_message('profile-b-message', 'from profile B')],
      nextCursor: 'profile-b-cursor',
      hasMore: true,
      historyProfileId: 'profile-b',
    );
  }
}

class _ProfileBAuthController extends AuthController {
  _ProfileBAuthController(Ref ref)
    : super(
        authClient: ref.watch(voiceAuthClientProvider),
        storage: ref.watch(authSessionStorageProvider),
        guestCredentialsStorage: ref.watch(guestCredentialsStorageProvider),
      ) {
    state = const AuthState(
      session: AuthSession(
        accessToken: 'profile-b-access',
        refreshToken: 'profile-b-refresh',
        accountId: 'account-1',
        activeProfileId: 'profile-b',
        expiresInSeconds: 900,
      ),
    );
  }
}

VoiceMessage _message(String id, String content) => VoiceMessage(
  id: id,
  chatId: 'chat-abc',
  senderProfileId: 'peer-1',
  content: content,
  createdAt: DateTime.parse('2026-09-20T00:00:00Z'),
);

class _NoopRealtimeHub extends RealtimeHub {
  _NoopRealtimeHub(super.ref);

  @override
  Stream<RealtimeFrame> get events => const Stream.empty();

  @override
  Future<void> ensureConnected() async {}

  @override
  void ensureSubscribed(String chatId) {}
}
