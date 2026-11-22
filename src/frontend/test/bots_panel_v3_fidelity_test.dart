import 'dart:async';
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/bots_client.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/bot_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/state/space_providers.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/ui/chat/chat_bots_panel.dart';
import 'package:voice_frontend/ui/chat/chat_info_panel.dart';
import 'package:voice_frontend/ui/core/voice_skeleton.dart';
import 'package:voice_frontend/ui/core/voice_state_panel.dart';

import 'support/auth_test_overrides.dart';
import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

const chatId = 'bots-chat';
const spaceId = 'space-1';

void main() {
  setUp(() => SharedPreferences.setMockInitialValues({}));

  const offlineBot = ChatBotSettings(
    bot: VoiceBotSummary(
      id: 'bot-offline',
      name: 'OfflineBot',
      description: 'counts messages',
      scopesJson: '[]',
    ),
    enabled: true,
    whitelisted: true,
  );
  const unknownBot = ChatBotSettings(
    bot: VoiceBotSummary(
      id: 'bot-unknown',
      name: 'UnknownBot',
      description: '',
      scopesJson: '[]',
    ),
    enabled: false,
    whitelisted: true,
  );
  const outsideBot = ChatBotSettings(
    bot: VoiceBotSummary(
      id: 'bot-outside',
      name: 'OutsideBot',
      description: 'not whitelisted',
      scopesJson: '[]',
    ),
    enabled: true,
    whitelisted: false,
  );
  const installedBots = [
    InstalledBotInfo(
      bot: VoiceBotSummary(
        id: 'bot-offline',
        name: 'OfflineBot',
        description: 'counts messages',
        scopesJson: '[]',
      ),
      installationId: 'install-offline',
      allowedChatIds: [chatId],
      online: false,
    ),
    InstalledBotInfo(
      bot: VoiceBotSummary(
        id: 'bot-unknown',
        name: 'UnknownBot',
        description: '',
        scopesJson: '[]',
      ),
      installationId: 'install-unknown',
      allowedChatIds: [chatId],
      online: null,
    ),
  ];

  List<Override> botOverrides({bool canManage = true}) {
    return [
      spacePermissionProvider((
        spaceId: spaceId,
        permission: 'SPACE_MANAGE_BOTS',
        chatId: null,
        voiceRoomId: null,
      )).overrideWith((ref) async => canManage),
      botsInChatProvider((
        chatId: chatId,
        spaceId: spaceId,
      )).overrideWith((ref) async => [offlineBot, unknownBot, outsideBot]),
      installedBotsProvider(spaceId).overrideWith((ref) async => installedBots),
    ];
  }

  Widget testApp({
    required Widget home,
    required http.Client client,
    List<Override> extraOverrides = const [],
  }) {
    return ProviderScope(
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
        httpClientProvider.overrideWithValue(client),
        ...extraOverrides,
      ],
      child: MaterialApp(
        theme: voiceTestTheme(),
        locale: const Locale('en'),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: Scaffold(body: home),
      ),
    );
  }

  Widget panelHost(Widget panel) =>
      SizedBox(width: 300, height: 800, child: panel);

  http.Client chatInfoClient() {
    return MockClient((req) async {
      if (req.url.path == '/api/v1/chats') {
        return http.Response(
          jsonEncode({
            'chat_list': {
              'items': [
                {
                  'chat': {
                    'id': chatId,
                    'type': 'CHAT_TYPE_GROUP',
                    'space_id': spaceId,
                    'creator_profile_id': 'prof-test',
                    'allow_guests': false,
                  },
                },
              ],
            },
          }),
          200,
        );
      }
      if (req.url.path == '/api/v1/chats/$chatId/members') {
        return http.Response(
          jsonEncode({
            'member_list': {
              'members': [
                {'profile_id': 'prof-test', 'role': 'owner'},
              ],
            },
          }),
          200,
        );
      }
      if (req.url.path.contains('/shared-media')) {
        return http.Response(
          jsonEncode({
            'shared_media_list': {'items': []},
          }),
          200,
        );
      }
      return http.Response('{}', 404);
    });
  }

  testWidgets(
    'chat info bots row opens the wide bots panel and close returns to info',
    (tester) async {
      tester.view.physicalSize = const Size(1280, 800);
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.reset);

      await tester.pumpWidget(
        testApp(
          home: ChatInfoPanel(chatId: chatId, isGroup: true),
          client: chatInfoClient(),
          extraOverrides: botOverrides(),
        ),
      );
      await tester.pumpAndSettle();

      final botsRow = find.byKey(const Key('chat_info_summary_bots_row'));
      expect(botsRow, findsOneWidget);
      await tester.ensureVisible(botsRow);
      await tester.tap(botsRow);
      await tester.pumpAndSettle();

      expect(find.byKey(ChatBotsPanel.panelKey), findsOneWidget);
      expect(find.byKey(ChatBotsPanel.closeKey), findsOneWidget);
      expect(
        find.byKey(ChatBotsPanel.backKey),
        findsNothing,
        reason: 'back is mobile-only per §53',
      );
      expect(
        find.byKey(const Key('chat_bot_offline_indicator_bot-offline')),
        findsOneWidget,
      );
      expect(
        find.byKey(const Key('chat_bot_offline_indicator_bot-unknown')),
        findsNothing,
        reason: 'unknown presence must stay unknown, not offline',
      );
      expect(
        tester
            .widget<SwitchListTile>(
              find.byKey(const Key('chat_bot_toggle_bot-offline')),
            )
            .value,
        isTrue,
      );
      expect(
        tester
            .widget<SwitchListTile>(
              find.byKey(const Key('chat_bot_toggle_bot-outside')),
            )
            .value,
        isFalse,
        reason: 'bots outside the whitelisted chats stay disabled',
      );
      expect(
        tester
            .widget<SwitchListTile>(
              find.byKey(const Key('chat_bot_toggle_bot-offline')),
            )
            .onChanged,
        isNotNull,
      );

      await tester.tap(find.byKey(ChatBotsPanel.closeKey));
      await tester.pumpAndSettle();
      expect(find.byKey(ChatBotsPanel.panelKey), findsNothing);
      expect(
        find.byKey(const Key('chat_info_summary_bots_row')),
        findsOneWidget,
      );
      expect(tester.takeException(), isNull);
    },
  );

  testWidgets(
    'a group without manage-bots permission does not open a dead-end panel',
    (tester) async {
      tester.view.physicalSize = const Size(1280, 800);
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.reset);

      await tester.pumpWidget(
        testApp(
          home: ChatInfoPanel(chatId: chatId, isGroup: true),
          client: chatInfoClient(),
          extraOverrides: botOverrides(canManage: false),
        ),
      );
      await tester.pumpAndSettle();

      final botsRow = find.byKey(const Key('chat_info_summary_bots_row'));
      expect(
        botsRow,
        findsOneWidget,
        reason: 'the summary row remains informational for every member',
      );
      await tester.tap(botsRow);
      await tester.pumpAndSettle();

      expect(find.byKey(ChatBotsPanel.panelKey), findsNothing);
      expect(tester.takeException(), isNull);
    },
  );

  testWidgets(
    'narrow chat info opens the bots panel as a bottom sheet with back',
    (tester) async {
      tester.view.physicalSize = const Size(390, 844);
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.reset);

      Widget openPanelButton(BuildContext context) => ElevatedButton(
        onPressed: () => unawaited(
          ChatBotsPanel.show(context, chatId: chatId, spaceId: spaceId),
        ),
        child: const Text('open bots panel'),
      );

      await tester.pumpWidget(
        testApp(
          home: Builder(builder: openPanelButton),
          client: chatInfoClient(),
          extraOverrides: botOverrides(),
        ),
      );
      await tester.pumpAndSettle();
      await tester.tap(find.byType(ElevatedButton));
      await tester.pumpAndSettle();

      expect(find.byKey(ChatBotsPanel.panelKey), findsOneWidget);
      expect(find.byKey(ChatBotsPanel.backKey), findsOneWidget);
      expect(find.byKey(ChatBotsPanel.closeKey), findsOneWidget);
      expect(
        find.byKey(const Key('chat_bot_offline_indicator_bot-offline')),
        findsOneWidget,
      );

      await tester.tap(find.byKey(ChatBotsPanel.backKey));
      await tester.pumpAndSettle();
      expect(find.byKey(ChatBotsPanel.panelKey), findsNothing);
      expect(find.byType(ElevatedButton), findsOneWidget);
      expect(tester.takeException(), isNull);
    },
  );

  testWidgets('hidden bots panel keeps manage-gated fetches suppressed', (
    tester,
  ) async {
    var botCalls = 0;
    var statusCalls = 0;
    await tester.pumpWidget(
      testApp(
        home: panelHost(ChatBotsPanel(chatId: chatId, spaceId: spaceId)),
        client: MockClient((_) async => http.Response('{}', 404)),
        extraOverrides: [
          spacePermissionProvider((
            spaceId: spaceId,
            permission: 'SPACE_MANAGE_BOTS',
            chatId: null,
            voiceRoomId: null,
          )).overrideWith((ref) async => false),
          botsInChatProvider((chatId: chatId, spaceId: spaceId)).overrideWith((
            ref,
          ) async {
            botCalls++;
            return const [];
          }),
          installedBotsProvider(spaceId).overrideWith((ref) async {
            statusCalls++;
            return const [];
          }),
        ],
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(ChatBotsPanel.panelKey), findsNothing);
    expect(botCalls, 0);
    expect(statusCalls, 0);
    expect(tester.takeException(), isNull);
  });

  testWidgets('bots panel keeps the canonical skeleton while loading', (
    tester,
  ) async {
    final pending = Completer<List<ChatBotSettings>>();
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
            MockClient((_) async => http.Response('{}', 404)),
          ),
          spacePermissionProvider((
            spaceId: spaceId,
            permission: 'SPACE_MANAGE_BOTS',
            chatId: null,
            voiceRoomId: null,
          )).overrideWith((ref) async => true),
          botsInChatProvider((
            chatId: chatId,
            spaceId: spaceId,
          )).overrideWith((ref) => pending.future),
          installedBotsProvider(
            spaceId,
          ).overrideWith((ref) async => installedBots),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: Scaffold(
            body: panelHost(ChatBotsPanel(chatId: chatId, spaceId: spaceId)),
          ),
        ),
      ),
    );
    await tester.pump();
    await tester.pump();

    expect(find.byType(VoiceListSkeleton), findsOneWidget);
    expect(find.byKey(ChatBotsPanel.closeKey), findsOneWidget);
    pending.complete(const []);
  });

  testWidgets('bots panel error state retries through the action control', (
    tester,
  ) async {
    var attempts = 0;
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
            MockClient((_) async => http.Response('{}', 404)),
          ),
          spacePermissionProvider((
            spaceId: spaceId,
            permission: 'SPACE_MANAGE_BOTS',
            chatId: null,
            voiceRoomId: null,
          )).overrideWith((ref) async => true),
          botsInChatProvider((chatId: chatId, spaceId: spaceId)).overrideWith((
            ref,
          ) async {
            attempts++;
            if (attempts == 1) throw 'bots load failed';
            return [offlineBot];
          }),
          installedBotsProvider(
            spaceId,
          ).overrideWith((ref) async => installedBots),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: Scaffold(
            body: panelHost(ChatBotsPanel(chatId: chatId, spaceId: spaceId)),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byType(VoiceStatePanel), findsOneWidget);
    expect(find.byKey(const Key('chat_bot_toggle_bot-offline')), findsNothing);

    await tester.tap(find.byType(OutlinedButton));
    await tester.pumpAndSettle();
    expect(find.byType(VoiceStatePanel), findsNothing);
    expect(
      find.byKey(const Key('chat_bot_toggle_bot-offline')),
      findsOneWidget,
    );
    expect(
      find.byKey(const Key('chat_bot_offline_indicator_bot-offline')),
      findsOneWidget,
    );
    expect(tester.takeException(), isNull);
  });

  testWidgets(
    'installed presence failure offers retry and the badge appears after it',
    (tester) async {
      var attempts = 0;
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
              MockClient((_) async => http.Response('{}', 404)),
            ),
            spacePermissionProvider((
              spaceId: spaceId,
              permission: 'SPACE_MANAGE_BOTS',
              chatId: null,
              voiceRoomId: null,
            )).overrideWith((ref) async => true),
            botsInChatProvider((
              chatId: chatId,
              spaceId: spaceId,
            )).overrideWith((ref) async => [offlineBot, unknownBot]),
            installedBotsProvider(spaceId).overrideWith((ref) async {
              attempts++;
              if (attempts == 1) throw 'presence load failed';
              return installedBots;
            }),
          ],
          child: MaterialApp(
            theme: voiceTestTheme(),
            locale: const Locale('en'),
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: Scaffold(
              body: panelHost(ChatBotsPanel(chatId: chatId, spaceId: spaceId)),
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.byKey(ChatBotsPanel.installedRetryKey), findsOneWidget);
      expect(
        find.byKey(const Key('chat_bot_offline_indicator_bot-offline')),
        findsNothing,
        reason: 'no authoritative presence while the status fetch failed',
      );

      await tester.tap(find.byKey(ChatBotsPanel.installedRetryKey));
      await tester.pumpAndSettle();
      expect(find.byKey(ChatBotsPanel.installedRetryKey), findsNothing);
      expect(
        find.byKey(const Key('chat_bot_offline_indicator_bot-offline')),
        findsOneWidget,
      );
      expect(tester.takeException(), isNull);
    },
  );

  testWidgets('bots panel keeps the per-space empty state', (tester) async {
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
            MockClient((_) async => http.Response('{}', 404)),
          ),
          spacePermissionProvider((
            spaceId: spaceId,
            permission: 'SPACE_MANAGE_BOTS',
            chatId: null,
            voiceRoomId: null,
          )).overrideWith((ref) async => true),
          botsInChatProvider((
            chatId: chatId,
            spaceId: spaceId,
          )).overrideWith((ref) async => const []),
          installedBotsProvider(spaceId).overrideWith((ref) async => const []),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: Scaffold(
            body: panelHost(ChatBotsPanel(chatId: chatId, spaceId: spaceId)),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(ChatBotsPanel.emptyKey), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets('narrow panel renders back, title, and close in its header', (
    tester,
  ) async {
    tester.view.physicalSize = const Size(390, 844);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.reset);

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
            MockClient((_) async => http.Response('{}', 404)),
          ),
          ...botOverrides(),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: Scaffold(
            body: SizedBox(
              width: 390,
              height: 844,
              child: ChatBotsPanel(chatId: chatId, spaceId: spaceId),
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(ChatBotsPanel.panelKey), findsOneWidget);
    expect(find.byKey(ChatBotsPanel.backKey), findsOneWidget);
    expect(find.byKey(ChatBotsPanel.closeKey), findsOneWidget);
    expect(tester.takeException(), isNull);
  });
}
