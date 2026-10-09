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
import 'package:voice_frontend/backend/guest_credentials_storage.dart';
import 'package:voice_frontend/backend/messages_client.dart';
import 'package:voice_frontend/backend/realtime_client.dart';
import 'package:voice_frontend/backend/space_permissions.dart';
import 'package:voice_frontend/e2e/e2e_message_service.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/connectivity_providers.dart';
import 'package:voice_frontend/state/e2e_providers.dart';
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
  });

  testWidgets(
    'Send when online retries the same draft and id without an immediate send',
    (tester) async {
      final firstAttemptResult = Completer<String?>();
      var immediateSendRequests = 0;
      late _ScheduledRoomController room;
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
              MockClient((request) async {
                if (request.url.path == '/api/v1/messages/send') {
                  immediateSendRequests++;
                }
                return http.Response('{}', 404);
              }),
            ),
            realtimeHubProvider.overrideWith((ref) => _NoopRealtimeHub(ref)),
            selectedChatIdProvider.overrideWith((ref) => 'chat-abc'),
            chatListControllerProvider.overrideWith(_DmChatListController.new),
            chatListProvider.overrideWith(
              (ref) async => const ChatListData(
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
              ),
            ),
            chatRoomControllerProvider('chat-abc').overrideWith((ref) {
              return room = _ScheduledRoomController(
                ref,
                'chat-abc',
                firstAttemptResult: firstAttemptResult,
              );
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

      final composer = find.descendant(
        of: find.byKey(ChatRoomPanel.inputKey),
        matching: find.byType(TextField),
      );
      await tester.enterText(composer, 'Keep this scheduled draft');
      final chatRoomProviderContainer = ProviderScope.containerOf(
        tester.element(find.byType(ChatRoomPanel)),
        listen: false,
      );
      final dmReadyBeforeLongPress =
          chatRoomProviderContainer
              .read(chatListProvider)
              .valueOrNull
              ?.items
              .any(
                (item) =>
                    item.chatId == 'chat-abc' &&
                    item.chat.type == 'CHAT_TYPE_DM',
              ) ??
          false;
      expect(
        dmReadyBeforeLongPress,
        isTrue,
        reason: 'DM chat state is ready before scheduled-send menu trigger',
      );
      await tester.longPress(find.byKey(ChatRoomPanel.sendKey));
      await tester.pumpAndSettle();
      expect(find.byType(BottomSheet), findsOneWidget);
      final selectedChatIsDm =
          chatRoomProviderContainer
              .read(chatListProvider)
              .valueOrNull
              ?.items
              .any(
                (item) =>
                    item.chatId == 'chat-abc' &&
                    item.chat.type == 'CHAT_TYPE_DM',
              ) ??
          false;
      expect(selectedChatIsDm, isTrue);
      await tester.tap(find.text('Send when online'));
      await tester.pumpAndSettle();

      expect(room.scheduledAttempts, hasLength(1));
      expect(room.state.isSending, isTrue);
      expect(find.text('Retry'), findsNothing);
      firstAttemptResult.complete('unavailable');
      await tester.pumpAndSettle();

      expect(
        room.scheduledAttempts.single.content,
        'Keep this scheduled draft',
      );
      expect(room.scheduledAttempts.single.sendWhenOnline, isTrue);
      expect(room.scheduledAttempts.single.scheduledAt, isNull);
      expect(
        find.text(
          'Could not complete the scheduled-message request. Try again.',
        ),
        findsOneWidget,
      );
      expect(immediateSendRequests, 0);
      expect(find.text('Keep this scheduled draft'), findsWidgets);

      await tester.tap(
        find.descendant(
          of: find.byKey(ChatRoomPanel.scheduledCreateRetryKey),
          matching: find.text('Retry'),
        ),
      );
      await tester.pumpAndSettle();

      expect(room.scheduledAttempts, hasLength(2));
      expect(
        room.scheduledAttempts[1].clientMessageId,
        room.scheduledAttempts[0].clientMessageId,
      );
      expect(
        room.scheduledAttempts[1].content,
        room.scheduledAttempts[0].content,
      );
      expect(room.scheduledAttempts[1].sendWhenOnline, isTrue);
      expect(immediateSendRequests, 0);
      expect(
        find.text(
          'Could not complete the scheduled-message request. Try again.',
        ),
        findsNothing,
      );
      expect(find.text('Keep this scheduled draft'), findsNothing);
    },
  );

  test('scheduled list drops a response after auth context changes', () async {
    final requestStarted = Completer<void>();
    final deferredResponse = Completer<http.Response>();
    late AuthController auth;
    final container = ProviderContainer(
      overrides: [
        authSessionStorageProvider.overrideWithValue(
          InMemoryAuthSessionStorage(),
        ),
        authControllerProvider.overrideWith((ref) {
          return auth = authenticatedAuthController(ref);
        }),
        gatewayConfigProvider.overrideWithValue(
          const GatewayConfig(baseUrl: 'http://api.test'),
        ),
        httpClientProvider.overrideWithValue(
          MockClient((request) async {
            if (request.url.path == '/api/v1/messages/scheduled') {
              if (!requestStarted.isCompleted) requestStarted.complete();
              return deferredResponse.future;
            }
            return http.Response('{}', 404);
          }),
        ),
        realtimeAutoConnectProvider.overrideWithValue(false),
        realtimeEventProvider.overrideWith(
          (ref) => const Stream<RealtimeFrame>.empty(),
        ),
        realtimeHubProvider.overrideWith((ref) => _NoopRealtimeHub(ref)),
        selectedChatIdProvider.overrideWith((ref) => 'chat-abc'),
      ],
    );
    addTearDown(container.dispose);
    final roomProvider = chatRoomControllerProvider('chat-abc');
    final roomSubscription = container.listen(roomProvider, (_, _) {});
    addTearDown(roomSubscription.close);
    final room = container.read(roomProvider.notifier);

    final load = room.loadScheduledMessages();
    await requestStarted.future;
    auth.state = const AuthState(
      session: AuthSession(
        accessToken: 'replacement-access',
        refreshToken: 'replacement-refresh',
        accountId: 'account-b',
        activeProfileId: 'profile-b',
        expiresInSeconds: 900,
      ),
    );
    deferredResponse.complete(
      http.Response(
        jsonEncode({
          'scheduled_messages': [
            {
              'id': 'old-account-schedule',
              'chat': {'id': 'chat-abc'},
              'sender_profile_id': 'profile-a',
              'send_when_online': true,
              'status': 'SCHEDULED_MESSAGE_STATUS_PENDING',
            },
          ],
          'page': {'has_more': false},
        }),
        200,
      ),
    );
    await load;

    expect(room.state.scheduledMessages, isEmpty);
    expect(room.state.isLoadingScheduledMessages, isFalse);
    expect(room.state.scheduledMessagesError, isNull);
  });

  test(
    'scheduled E2E retry replays the exact encrypted request after an ambiguous failure',
    () async {
      var encryptionCalls = 0;
      var scheduleCalls = 0;
      String? committedRequestBody;
      final requestBodies = <String>[];
      final client = MockClient((request) async {
        if (request.url.path != '/api/v1/messages/send') {
          return http.Response('{}', 404);
        }
        scheduleCalls++;
        requestBodies.add(request.body);
        if (scheduleCalls == 1) {
          // Model an accepted request whose response was lost. The server's
          // idempotency key can replay only the exact original request body.
          committedRequestBody = request.body;
          return http.Response('{}', 503);
        }
        if (request.body != committedRequestBody) {
          return http.Response('{}', 409);
        }
        return http.Response(
          jsonEncode({
            'scheduled_message': {
              'id': 'scheduled-e2e-1',
              'chat': {'id': 'chat-abc'},
              'sender_profile_id': 'prof-test',
              'client_message_id': 'e2e-scheduled-attempt-1',
              'send_when_online': true,
              'is_e2e': true,
              'status': 'SCHEDULED_MESSAGE_STATUS_PENDING',
              'payload': {'content': 'wire-ciphertext-1'},
            },
          }),
          200,
        );
      });
      final container = ProviderContainer(
        overrides: [
          authSessionStorageProvider.overrideWithValue(
            InMemoryAuthSessionStorage(),
          ),
          guestCredentialsStorageProvider.overrideWithValue(
            InMemoryGuestCredentialsStorage(),
          ),
          authControllerProvider.overrideWith(authenticatedAuthController),
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(baseUrl: 'http://api.test'),
          ),
          httpClientProvider.overrideWithValue(client),
          isDeviceOfflineProvider.overrideWith((ref) => false),
          realtimeAutoConnectProvider.overrideWithValue(false),
          realtimeEventProvider.overrideWith(
            (ref) => const Stream<RealtimeFrame>.empty(),
          ),
          realtimeHubProvider.overrideWith((ref) => _NoopRealtimeHub(ref)),
          selectedChatIdProvider.overrideWith((ref) => 'another-chat'),
          chatListControllerProvider.overrideWith((ref) {
            return _E2eDmChatListController(ref);
          }),
          e2eMessageServiceProvider.overrideWithValue(
            _CountingE2eMessageService(() => ++encryptionCalls),
          ),
        ],
      );
      addTearDown(container.dispose);
      final roomProvider = chatRoomControllerProvider('chat-abc');
      final roomSubscription = container.listen(roomProvider, (_, _) {});
      addTearDown(roomSubscription.close);
      final room = container.read(roomProvider.notifier);

      final firstResult = await room.createScheduledMessage(
        content: 'Encrypted later',
        clientMessageId: 'e2e-scheduled-attempt-1',
        scheduledAt: null,
        sendWhenOnline: true,
      );
      expect(firstResult, isNotNull);
      expect(room.state.scheduledMessages, isEmpty);

      final retryResult = await room.createScheduledMessage(
        content: 'Encrypted later',
        clientMessageId: 'e2e-scheduled-attempt-1',
        scheduledAt: null,
        sendWhenOnline: true,
      );

      expect(retryResult, isNull);
      expect(encryptionCalls, 1);
      expect(scheduleCalls, 2);
      expect(requestBodies, hasLength(2));
      expect(requestBodies[1], requestBodies[0]);
      final sentBody = jsonDecode(requestBodies.first) as Map<String, dynamic>;
      expect(sentBody['content'], 'wire-ciphertext-1');
      expect(sentBody['is_e2e'], isTrue);
      expect(sentBody['client_message_id'], 'e2e-scheduled-attempt-1');
      expect(room.state.scheduledMessages.single.id, 'scheduled-e2e-1');
    },
  );

  test(
    'scheduled E2E create is single-flight while its response is pending',
    () async {
      final requestStarted = Completer<void>();
      final deferredResponse = Completer<http.Response>();
      var encryptionCalls = 0;
      var scheduleCalls = 0;
      final requestBodies = <String>[];
      final container = ProviderContainer(
        overrides: [
          authSessionStorageProvider.overrideWithValue(
            InMemoryAuthSessionStorage(),
          ),
          guestCredentialsStorageProvider.overrideWithValue(
            InMemoryGuestCredentialsStorage(),
          ),
          authControllerProvider.overrideWith(authenticatedAuthController),
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(baseUrl: 'http://api.test'),
          ),
          httpClientProvider.overrideWithValue(
            MockClient((request) async {
              if (request.url.path != '/api/v1/messages/send') {
                return http.Response('{}', 404);
              }
              scheduleCalls++;
              requestBodies.add(request.body);
              if (!requestStarted.isCompleted) requestStarted.complete();
              return deferredResponse.future;
            }),
          ),
          isDeviceOfflineProvider.overrideWith((ref) => false),
          realtimeAutoConnectProvider.overrideWithValue(false),
          realtimeEventProvider.overrideWith(
            (ref) => const Stream<RealtimeFrame>.empty(),
          ),
          realtimeHubProvider.overrideWith((ref) => _NoopRealtimeHub(ref)),
          selectedChatIdProvider.overrideWith((ref) => 'another-chat'),
          chatListControllerProvider.overrideWith((ref) {
            return _E2eDmChatListController(ref);
          }),
          e2eMessageServiceProvider.overrideWithValue(
            _CountingE2eMessageService(() => ++encryptionCalls),
          ),
        ],
      );
      addTearDown(container.dispose);
      final roomProvider = chatRoomControllerProvider('chat-abc');
      final roomSubscription = container.listen(roomProvider, (_, _) {});
      addTearDown(roomSubscription.close);
      final room = container.read(roomProvider.notifier);

      final firstRequest = room.createScheduledMessage(
        content: 'Encrypted later',
        clientMessageId: 'e2e-scheduled-attempt-2',
        scheduledAt: null,
        sendWhenOnline: true,
      );
      await requestStarted.future;
      expect(room.state.isSending, isTrue);

      final concurrentAttempt = await room.createScheduledMessage(
        content: 'Encrypted later',
        clientMessageId: 'e2e-scheduled-attempt-2',
        scheduledAt: null,
        sendWhenOnline: true,
      );
      expect(concurrentAttempt, 'scheduled_send_in_progress');
      expect(scheduleCalls, 1);
      expect(encryptionCalls, 1);
      expect(requestBodies, hasLength(1));

      deferredResponse.complete(
        http.Response(
          jsonEncode({
            'scheduled_message': {
              'id': 'scheduled-e2e-2',
              'chat': {'id': 'chat-abc'},
              'sender_profile_id': 'prof-test',
              'client_message_id': 'e2e-scheduled-attempt-2',
              'send_when_online': true,
              'is_e2e': true,
              'status': 'SCHEDULED_MESSAGE_STATUS_PENDING',
              'payload': {'content': 'wire-ciphertext-1'},
            },
          }),
          200,
        ),
      );
      expect(await firstRequest, isNull);

      expect(room.state.isSending, isFalse);
      expect(room.state.scheduledMessages.single.id, 'scheduled-e2e-2');
      expect(scheduleCalls, 1);
      expect(encryptionCalls, 1);
    },
  );

  test(
    'scheduled list completion after controller disposal is ignored',
    () async {
      final requestStarted = Completer<void>();
      final deferredResponse = Completer<http.Response>();
      final container = ProviderContainer(
        overrides: [
          authSessionStorageProvider.overrideWithValue(
            InMemoryAuthSessionStorage(),
          ),
          authControllerProvider.overrideWith(authenticatedAuthController),
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(baseUrl: 'http://api.test'),
          ),
          httpClientProvider.overrideWithValue(
            MockClient((request) async {
              if (request.url.path == '/api/v1/messages/scheduled') {
                if (!requestStarted.isCompleted) requestStarted.complete();
                return deferredResponse.future;
              }
              return http.Response('{}', 404);
            }),
          ),
          realtimeAutoConnectProvider.overrideWithValue(false),
          realtimeEventProvider.overrideWith(
            (ref) => const Stream<RealtimeFrame>.empty(),
          ),
          realtimeHubProvider.overrideWith((ref) => _NoopRealtimeHub(ref)),
        ],
      );
      final roomProvider = chatRoomControllerProvider('chat-abc');
      final roomSubscription = container.listen(roomProvider, (_, _) {});
      final room = container.read(roomProvider.notifier);
      final load = room.loadScheduledMessages();
      await requestStarted.future;

      roomSubscription.close();
      container.dispose();
      deferredResponse.complete(http.Response('{}', 200));
      await expectLater(load, completes);
    },
  );
}

class _EmptyRoomController extends ChatRoomController {
  _EmptyRoomController(super.ref, super.chatId) : super() {
    state = const ChatRoomState(messages: []);
  }

  @override
  Future<void> loadInitial() async {}
}

class _DmChatListController extends ChatListController {
  _DmChatListController(super.ref) : super() {
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

class _ScheduledRoomController extends ChatRoomController {
  _ScheduledRoomController(super.ref, super.chatId, {this.firstAttemptResult})
    : super() {
    state = const ChatRoomState(messages: []);
  }

  final Completer<String?>? firstAttemptResult;

  final scheduledAttempts =
      <
        ({
          String content,
          String clientMessageId,
          DateTime? scheduledAt,
          bool sendWhenOnline,
        })
      >[];

  @override
  Future<void> loadInitial() async {}

  @override
  Future<void> loadScheduledMessages({bool loadMore = false}) async {}

  @override
  Future<String?> createScheduledMessage({
    required String content,
    required String clientMessageId,
    required DateTime? scheduledAt,
    required bool sendWhenOnline,
    List<MessageAttachment> attachments = const [],
    List<MessageMention> mentions = const [],
    String? threadParentId,
  }) async {
    scheduledAttempts.add((
      content: content,
      clientMessageId: clientMessageId,
      scheduledAt: scheduledAt,
      sendWhenOnline: sendWhenOnline,
    ));
    if (scheduledAttempts.length == 1 && firstAttemptResult != null) {
      state = state.copyWith(isSending: true, clearScheduledActionError: true);
      final error = await firstAttemptResult!.future;
      state = state.copyWith(
        isSending: false,
        scheduledActionError: error,
        clearScheduledActionError: error == null,
      );
      return error;
    }
    final error = scheduledAttempts.length == 1 ? 'unavailable' : null;
    state = state.copyWith(
      isSending: false,
      scheduledActionError: error,
      clearScheduledActionError: error == null,
    );
    return error;
  }
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

class _E2eDmChatListController extends ChatListController {
  _E2eDmChatListController(super.ref) : super() {
    state = const ChatListState(
      profileId: 'prof-test',
      items: [
        ChatListItem(
          chat: VoiceChat(
            id: 'chat-abc',
            type: 'CHAT_TYPE_DM',
            creatorProfileId: 'prof-test',
            e2eEnabled: true,
          ),
          dmPeerProfileId: 'peer-1',
        ),
      ],
    );
  }

  @override
  Future<void> loadInitial() async {}
}

class _CountingE2eMessageService extends E2eMessageService {
  _CountingE2eMessageService(this._nextCall);

  final int Function() _nextCall;

  @override
  Future<String> encryptOutgoing({
    required String localProfileId,
    required String peerProfileId,
    required String plaintext,
    String? authorization,
    String? chatId,
  }) async => 'wire-ciphertext-${_nextCall()}';
}
