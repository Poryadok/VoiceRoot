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
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/shell/three_column_shell.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/ui/chat/chat_room_panel.dart';
import 'package:voice_frontend/ui/core/voice_state_panel.dart';
import 'package:voice_frontend/ui/core/voice_skeleton.dart';
import 'package:voice_frontend/ui/shell/chat_list_body.dart';

import 'support/auth_test_overrides.dart';
import 'support/markdown_test_helpers.dart';
import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

void main() {
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
