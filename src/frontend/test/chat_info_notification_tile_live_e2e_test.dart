import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/guest_credentials_storage.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/ui/chat/chat_info_panel.dart';
import 'package:voice_frontend/ui/settings/notification_settings_screen.dart';

import 'support/live_gateway_harness.dart';
import 'support/auth_test_overrides.dart';
import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

void main() {
  _LiveBinding();
  testWidgets(
    'live Compose chat notification tile fits narrow chat info and opens chat settings',
    (tester) async {
      final bootstrap = await tester.runAsync(() async {
        final probe = await probeLiveGateway();
        expect(
          probe,
          isA<LiveGatewayReady>(),
          reason: probe is LiveGatewayUnavailable ? probe.reason : null,
        );
        final live = (probe as LiveGatewayReady).context;
        final owner = await live.registerUser('chat-notification-tile-owner');
        final peer = await live.registerUser('chat-notification-tile-peer');
        final dmResult = await live.chatsClient().createDm(
          authorization: owner.authorizationHeader,
          otherProfileId: peer.activeProfileId,
        );
        expect(dmResult, isA<ChatsApiOk<VoiceChat>>());
        final chat = (dmResult as ChatsApiOk<VoiceChat>).data;
        return (live: live, owner: owner, peer: peer, chat: chat);
      });
      expect(bootstrap, isNotNull);
      final live = bootstrap!.live;
      final owner = bootstrap.owner;
      final peer = bootstrap.peer;
      final chat = bootstrap.chat;
      addTearDown(live.httpClient.close);

      tester.view.devicePixelRatio = 1;
      tester.view.physicalSize = const Size(360, 800);
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      final client = http.Client();
      addTearDown(client.close);

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
            guestCredentialsStorageProvider.overrideWithValue(
              InMemoryGuestCredentialsStorage(),
            ),
            authControllerProvider.overrideWith((ref) {
              final controller = AuthController(
                authClient: ref.watch(voiceAuthClientProvider),
                storage: ref.watch(authSessionStorageProvider),
                guestCredentialsStorage: ref.watch(
                  guestCredentialsStorageProvider,
                ),
              );
              controller.state = AuthState(session: owner);
              return controller;
            }),
            gatewayConfigProvider.overrideWithValue(
              GatewayConfig(baseUrl: live.config.baseUrl),
            ),
            httpClientProvider.overrideWithValue(client),
            chatListControllerProvider.overrideWith(
              (ref) => _LiveChatListController(ref, owner, chat, peer),
            ),
          ],
          child: MaterialApp(
            theme: voiceTestTheme(),
            locale: const Locale('en'),
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: Consumer(
              builder: (context, ref, _) => Scaffold(
                body: Center(
                  child: TextButton(
                    key: const Key('open_chat_info'),
                    onPressed: () =>
                        openChatInfoPanel(context, ref, chatId: chat.id),
                    child: const Text('Open chat info'),
                  ),
                ),
              ),
            ),
          ),
        ),
      );
      await tester.tap(find.byKey(const Key('open_chat_info')));
      final tile = find.byKey(const Key('chat_info_notification_settings'));
      await _waitFor(tester, () => tile.evaluate().isNotEmpty);

      await tester.ensureVisible(tile);
      await tester.pumpAndSettle();
      expect(tile, findsOneWidget);
      final tileRect = tester.getRect(tile);
      expect(tileRect.left, greaterThanOrEqualTo(0));
      expect(tileRect.right, lessThanOrEqualTo(360));
      expect(tester.takeException(), isNull);

      await tester.tap(tile);
      await _waitFor(
        tester,
        () => find
            .byKey(NotificationSettingsScreen.screenKey)
            .evaluate()
            .isNotEmpty,
      );
      expect(find.byKey(NotificationSettingsScreen.screenKey), findsOneWidget);
      final settingsScreen = tester.widget<NotificationSettingsScreen>(
        find.byType(NotificationSettingsScreen),
      );
      expect(settingsScreen.chatId, chat.id);
      expect(tester.takeException(), isNull);
    },
    skip: !runLiveIntegration,
    timeout: const Timeout(Duration(minutes: 3)),
  );
}

Future<void> _waitFor(WidgetTester tester, bool Function() ready) async {
  final deadline = DateTime.now().add(const Duration(seconds: 60));
  while (!ready() && DateTime.now().isBefore(deadline)) {
    await tester.runAsync(
      () => Future<void>.delayed(const Duration(milliseconds: 100)),
    );
    await tester.pump();
  }
  expect(
    ready(),
    isTrue,
    reason: 'Live chat info UI did not reach the expected state',
  );
  expect(tester.takeException(), isNull);
}

class _LiveBinding extends AutomatedTestWidgetsFlutterBinding {
  @override
  bool get overrideHttpClient => false;
}

class _LiveChatListController extends ChatListController {
  _LiveChatListController(
    super.ref,
    AuthSession owner,
    VoiceChat chat,
    AuthSession peer,
  ) : super() {
    state = ChatListState(
      profileId: owner.activeProfileId,
      items: [ChatListItem(chat: chat, dmPeerProfileId: peer.activeProfileId)],
    );
  }

  @override
  Future<void> loadInitial() async {}

  @override
  Future<void> loadMore() async {}
}
