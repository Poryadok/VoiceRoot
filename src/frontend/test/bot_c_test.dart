import 'dart:async';
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/bots_client.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/spaces_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/bot_providers.dart';
import 'package:voice_frontend/state/space_providers.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/ui/chat/slash_command_menu.dart';
import 'package:voice_frontend/ui/chat/slash_command_options_sheet.dart';
import 'package:voice_frontend/ui/space/space_bots_sheet.dart';

import 'support/auth_test_overrides.dart';
import 'support/gateway_test_client.dart';
import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

void main() {
  const config = GatewayConfig(baseUrl: 'http://api.test');
  const auth = 'Bearer access-token';

  Future<void> pumpCommandJourney(
    WidgetTester tester, {
    required BotSlashCommand command,
    required http.Client httpClient,
    void Function(
      SlashInteractionFailure? failure,
      List<EphemeralBotMessage> messages,
      DeferredBotInteraction? deferred,
    )?
    onExecuted,
  }) async {
    final botsClient = VoiceBotsClient(
      gateway: gatewayHttpForTest(httpClient, config: config),
    );
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceThemeTestOverrides(),
          profileAccentStorageProvider.overrideWithValue(
            testProfileAccentStorage,
          ),
          authorizationHeaderProvider.overrideWithValue(auth),
          voiceBotsClientProvider.overrideWithValue(botsClient),
          chatTypeForChatProvider(
            'chat-1',
          ).overrideWith((ref) => 'CHAT_TYPE_CHANNEL'),
          slashCommandsForChatProvider(
            'chat-1',
          ).overrideWith((ref) async => [command]),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: Consumer(
            builder: (context, ref, _) => Scaffold(
              body: SlashCommandMenuSheet(
                chatId: 'chat-1',
                onSelected: (selected) async {
                  var options = const <String, dynamic>{};
                  if (selected.options.isNotEmpty) {
                    final collected = await showSlashCommandOptionsSheet(
                      context: context,
                      ref: ref,
                      chatId: 'chat-1',
                      command: selected,
                    );
                    if (collected == null) return;
                    options = collected;
                  }
                  final failure = await ref
                      .read(slashInteractionExecutorProvider)
                      .execute(
                        chatId: 'chat-1',
                        command: selected,
                        optionsJson: jsonEncode(options),
                      );
                  onExecuted?.call(
                    failure,
                    ref.read(ephemeralMessagesProvider('chat-1')),
                    ref.read(deferredBotInteractionProvider('chat-1')),
                  );
                },
              ),
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
  }

  testWidgets('no-options command invokes once with an empty options object', (
    tester,
  ) async {
    var invocationCount = 0;
    Map<String, dynamic>? requestBody;
    SlashInteractionFailure? executionFailure;
    List<EphemeralBotMessage> messages = const [];
    final mock = MockClient((request) async {
      expect(request.url.path, '/api/v1/bots/interactions');
      invocationCount++;
      requestBody = jsonDecode(request.body) as Map<String, dynamic>;
      return http.Response(
        jsonEncode({
          'interaction_token': 'interaction-1',
          'content': 'Command accepted',
          'is_ephemeral': true,
        }),
        200,
        headers: const {'content-type': 'application/json'},
      );
    });

    await pumpCommandJourney(
      tester,
      command: const BotSlashCommand(
        botId: 'bot-1',
        botName: 'StatsBot',
        name: 'status',
        description: 'Show status',
      ),
      httpClient: mock,
      onExecuted: (failure, updatedMessages, _) {
        executionFailure = failure;
        messages = updatedMessages;
      },
    );

    await tester.tap(find.text('/status'));
    await tester.pumpAndSettle();

    expect(invocationCount, 1);
    expect(requestBody?['options_json'], '{}');
    expect(executionFailure, isNull);
    expect(find.text('Run command'), findsNothing);
    expect(messages.single.content, 'Command accepted');
  });

  testWidgets('canceling options causes no interaction request', (
    tester,
  ) async {
    var invocationCount = 0;
    final mock = MockClient((request) async {
      invocationCount++;
      return http.Response('{}', 200);
    });

    await pumpCommandJourney(
      tester,
      command: const BotSlashCommand(
        botId: 'bot-1',
        botName: 'StatsBot',
        name: 'status',
        description: 'Show status',
        options: [
          BotSlashCommandOption(name: 'detail', type: 'string', required: true),
        ],
      ),
      httpClient: mock,
    );

    await tester.tap(find.text('/status'));
    await tester.pumpAndSettle();
    expect(find.text('Cancel'), findsOneWidget);
    await tester.tap(find.text('Cancel'));
    await tester.pumpAndSettle();

    expect(invocationCount, 0);
  });

  testWidgets('deferred command records its interaction for follow-up', (
    tester,
  ) async {
    SlashInteractionFailure? executionFailure;
    List<EphemeralBotMessage> messages = const [];
    DeferredBotInteraction? deferred;
    final mock = MockClient((request) async {
      expect(request.url.path, '/api/v1/bots/interactions');
      return http.Response(
        jsonEncode({
          'interaction_token': 'interaction-deferred',
          'deferred': true,
        }),
        200,
        headers: const {'content-type': 'application/json'},
      );
    });

    await pumpCommandJourney(
      tester,
      command: const BotSlashCommand(
        botId: 'bot-1',
        botName: 'StatsBot',
        name: 'status',
        description: 'Show status',
      ),
      httpClient: mock,
      onExecuted: (failure, updatedMessages, deferredInteraction) {
        executionFailure = failure;
        messages = updatedMessages;
        deferred = deferredInteraction;
      },
    );

    await tester.tap(find.text('/status'));
    await tester.pumpAndSettle();

    expect(executionFailure, isNull);
    expect(messages, isEmpty);
    expect(find.text('Run command'), findsNothing);
    expect(deferred?.botName, 'StatsBot');
    expect(deferred?.interactionToken, 'interaction-deferred');
  });

  test(
    'timeout and unavailable command responses map to their outcomes',
    () async {
      for (final responseCase in [
        (
          error: 'bot_timeout',
          status: 504,
          expected: SlashInteractionFailure.botTimeout,
        ),
        (
          error: kBotUnavailableErrorCode,
          status: 503,
          expected: SlashInteractionFailure.botUnavailable,
        ),
      ]) {
        final client = MockClient((request) async {
          expect(request.url.path, '/api/v1/bots/interactions');
          return http.Response(
            jsonEncode({'error': responseCase.error}),
            responseCase.status,
            headers: const {'content-type': 'application/json'},
          );
        });
        final container = ProviderContainer(
          overrides: [
            authorizationHeaderProvider.overrideWithValue(auth),
            voiceBotsClientProvider.overrideWithValue(
              VoiceBotsClient(
                gateway: gatewayHttpForTest(client, config: config),
              ),
            ),
            chatTypeForChatProvider(
              'chat-1',
            ).overrideWith((ref) => 'CHAT_TYPE_CHANNEL'),
          ],
        );
        addTearDown(container.dispose);

        final outcome = await container
            .read(slashInteractionExecutorProvider)
            .execute(
              chatId: 'chat-1',
              command: const BotSlashCommand(
                botId: 'bot-1',
                botName: 'StatsBot',
                name: 'status',
                description: 'Show status',
              ),
            );

        expect(outcome, responseCase.expected);
      }
    },
  );

  test('stale slash response cannot update the switched session', () async {
    final sessionAuthorization = StateProvider<String?>((ref) => auth);
    final response = Completer<http.Response>();
    final requestStarted = Completer<void>();
    final client = MockClient((request) {
      expect(request.url.path, '/api/v1/bots/interactions');
      expect(request.headers['authorization'], auth);
      requestStarted.complete();
      return response.future;
    });
    final botsClient = VoiceBotsClient(
      gateway: gatewayHttpForTest(client, config: config),
    );
    final container = ProviderContainer(
      overrides: [
        authorizationHeaderProvider.overrideWith(
          (ref) => ref.watch(sessionAuthorization),
        ),
        voiceBotsClientProvider.overrideWithValue(botsClient),
        chatTypeForChatProvider(
          'chat-1',
        ).overrideWith((ref) => 'CHAT_TYPE_CHANNEL'),
      ],
    );
    addTearDown(container.dispose);
    container.read(ephemeralMessagesProvider('chat-1'));
    container.read(deferredBotInteractionProvider('chat-1'));

    final execution = container
        .read(slashInteractionExecutorProvider)
        .execute(
          chatId: 'chat-1',
          command: const BotSlashCommand(
            botId: 'bot-1',
            botName: 'StatsBot',
            name: 'stats',
            description: 'Stats',
          ),
        );
    await requestStarted.future;
    container.read(sessionAuthorization.notifier).state = 'Bearer new-session';
    response.complete(
      http.Response(
        jsonEncode({
          'interactionToken': 'interaction-1',
          'content': 'result for old session',
          'isEphemeral': true,
        }),
        200,
        headers: const {'content-type': 'application/json'},
      ),
    );

    expect(
      await execution,
      SlashInteractionFailure.requestFailed,
      reason: 'a completion from the previous authorization is stale',
    );
    expect(container.read(ephemeralMessagesProvider('chat-1')), isEmpty);
    expect(container.read(deferredBotInteractionProvider('chat-1')), isNull);
  });

  testWidgets('slash commands grey out when backend reports offline (BOT-C)', (
    tester,
  ) async {
    // Current backend still returns online:true without heartbeat; BOT-C must
    // drive offline until TouchPresence / heartbeat.
    const commandsFromBackend = [
      BotSlashCommand(
        botId: 'bot-offline',
        botName: 'DownBot',
        name: 'slow',
        description: 'Unavailable',
        online: false,
      ),
    ];

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceThemeTestOverrides(),
          profileAccentStorageProvider.overrideWithValue(
            testProfileAccentStorage,
          ),
          authorizationHeaderProvider.overrideWithValue(auth),
          slashCommandsForChatProvider(
            'chat-1',
          ).overrideWith((ref) async => commandsFromBackend),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: Scaffold(
            body: SlashCommandMenuSheet(chatId: 'chat-1', onSelected: (_) {}),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    final slowTile = find.byKey(
      const ValueKey('slash_command_bot-offline_slow'),
    );
    expect(slowTile, findsOneWidget);
    final tile = tester.widget<ListTile>(slowTile);
    expect(
      tile.enabled,
      isFalse,
      reason: 'bot without presence must be greyed out (BOT-C)',
    );
  });

  testWidgets('autocomplete does not swallow BotsApiFailure (BOT-C)', (
    tester,
  ) async {
    const command = BotSlashCommand(
      botId: 'bot-1',
      botName: 'StatsBot',
      name: 'stats',
      description: 'Stats',
      options: [
        BotSlashCommandOption(
          name: 'game',
          type: 'string',
          required: true,
          autocomplete: true,
        ),
      ],
    );

    final mock = MockClient((req) async {
      expect(req.url.path, '/api/v1/bots/autocomplete');
      return http.Response(
        jsonEncode({'error': 'bot unavailable'}),
        503,
        headers: const {'content-type': 'application/json'},
      );
    });
    final botsClient = VoiceBotsClient(
      gateway: gatewayHttpForTest(mock, config: config),
    );

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceThemeTestOverrides(),
          profileAccentStorageProvider.overrideWithValue(
            testProfileAccentStorage,
          ),
          authorizationHeaderProvider.overrideWithValue(auth),
          voiceBotsClientProvider.overrideWithValue(botsClient),
          chatTypeForChatProvider(
            'chat-1',
          ).overrideWith((ref) => 'CHAT_TYPE_CHANNEL'),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: Consumer(
            builder: (context, ref, _) => Scaffold(
              body: Center(
                child: ElevatedButton(
                  onPressed: () => showSlashCommandOptionsSheet(
                    context: context,
                    ref: ref,
                    chatId: 'chat-1',
                    command: command,
                  ),
                  child: const Text('open'),
                ),
              ),
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.text('open'));
    await tester.pumpAndSettle();

    final gameField = find.byType(TextField);
    expect(gameField, findsOneWidget);
    await tester.enterText(gameField, 'cs');
    await tester.pump(const Duration(milliseconds: 300));
    await tester.pump(const Duration(seconds: 3));

    expect(
      find.text('bot unavailable'),
      findsOneWidget,
      reason:
          'BotsApiFailure from autocomplete must surface to the user (BOT-C)',
    );
  });

  testWidgets('privileged bot install requires explicit acknowledgment (BOT-C)', (
    tester,
  ) async {
    const privilegedBot = VoiceBotSummary(
      id: 'bot-history',
      name: 'HistoryBot',
      description: 'Reads history',
      scopesJson: '["TEXT_CHAT_SEND_MESSAGES","TEXT_CHAT_READ_HISTORY"]',
    );

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceAppTestOverrides(
            client: MockClient((_) async => http.Response('{}', 404)),
          ),
          spacePermissionProvider.overrideWith((ref, query) async => true),
          discoverableBotsProvider.overrideWith((ref) async => [privilegedBot]),
          installedBotsProvider(
            'space-1',
          ).overrideWith((ref) async => const []),
          spaceTreeProvider('space-1').overrideWith(
            (ref) async => const SpaceTreeData(
              categories: [],
              voiceRooms: [],
              nodes: [
                SpaceTreeNodeData(
                  id: 'node-1',
                  spaceId: 'space-1',
                  kind: 'text_chat',
                  sortOrder: 0,
                  displayName: 'general',
                  linkedChatId: 'chat-1',
                  chatType: 'CHAT_TYPE_CHANNEL',
                ),
              ],
            ),
          ),
          voiceBotsClientProvider.overrideWith((ref) {
            return VoiceBotsClient(
              gateway: gatewayHttpForTest(
                MockClient((req) async {
                  if (req.url.path.contains('/bots/bot-history')) {
                    return utf8JsonResponse(
                      jsonEncode({
                        'bot': {
                          'id': 'bot-history',
                          'name': 'HistoryBot',
                          'description': 'Reads history',
                          'scopes_json': privilegedBot.scopesJson,
                          'status': 'live',
                        },
                      }),
                    );
                  }
                  return http.Response('{}', 404);
                }),
                config: config,
              ),
            );
          }),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(body: SpaceBotsSheet(spaceId: 'space-1')),
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(const Key('space_bots_install_picker')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('HistoryBot').last);
    await tester.pumpAndSettle();

    expect(
      find.byKey(const Key('space_bots_privileged_ack_checkbox')),
      findsOneWidget,
      reason:
          'TEXT_CHAT_READ_HISTORY install must require acknowledge checkbox (BOT-C)',
    );

    final installButton = tester.widget<FilledButton>(
      find.byKey(const Key('space_bots_install_confirm')),
    );
    expect(
      installButton.onPressed,
      isNull,
      reason:
          'install must stay disabled until privileged scopes are acknowledged (BOT-C)',
    );
  });

  testWidgets(
    'SPACE_MANAGE_ROLES bot install requires explicit acknowledgment (BOT-C)',
    (tester) async {
      const privilegedBot = VoiceBotSummary(
        id: 'bot-roles',
        name: 'RolesBot',
        description: 'Manages roles',
        scopesJson: '["TEXT_CHAT_SEND_MESSAGES","SPACE_MANAGE_ROLES"]',
      );

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            ...voiceAppTestOverrides(
              client: MockClient((_) async => http.Response('{}', 404)),
            ),
            spacePermissionProvider.overrideWith((ref, query) async => true),
            discoverableBotsProvider.overrideWith(
              (ref) async => [privilegedBot],
            ),
            installedBotsProvider(
              'space-1',
            ).overrideWith((ref) async => const []),
            spaceTreeProvider('space-1').overrideWith(
              (ref) async => const SpaceTreeData(
                categories: [],
                voiceRooms: [],
                nodes: [
                  SpaceTreeNodeData(
                    id: 'node-1',
                    spaceId: 'space-1',
                    kind: 'text_chat',
                    sortOrder: 0,
                    displayName: 'general',
                    linkedChatId: 'chat-1',
                    chatType: 'CHAT_TYPE_CHANNEL',
                  ),
                ],
              ),
            ),
            voiceBotsClientProvider.overrideWith((ref) {
              return VoiceBotsClient(
                gateway: gatewayHttpForTest(
                  MockClient((req) async {
                    if (req.url.path.contains('/bots/bot-roles')) {
                      return utf8JsonResponse(
                        jsonEncode({
                          'bot': {
                            'id': 'bot-roles',
                            'name': 'RolesBot',
                            'description': 'Manages roles',
                            'scopes_json': privilegedBot.scopesJson,
                            'status': 'live',
                          },
                        }),
                      );
                    }
                    return http.Response('{}', 404);
                  }),
                  config: config,
                ),
              );
            }),
          ],
          child: MaterialApp(
            theme: voiceTestTheme(),
            locale: const Locale('en'),
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: const Scaffold(body: SpaceBotsSheet(spaceId: 'space-1')),
          ),
        ),
      );
      await tester.pumpAndSettle();

      await tester.tap(find.byKey(const Key('space_bots_install_picker')));
      await tester.pumpAndSettle();
      await tester.tap(find.text('RolesBot').last);
      await tester.pumpAndSettle();

      expect(
        find.byKey(const Key('space_bots_privileged_ack_checkbox')),
        findsOneWidget,
        reason:
            'SPACE_MANAGE_ROLES install must require acknowledge checkbox (BOT-C)',
      );

      final installButton = tester.widget<FilledButton>(
        find.byKey(const Key('space_bots_install_confirm')),
      );
      expect(
        installButton.onPressed,
        isNull,
        reason:
            'install must stay disabled until SPACE_MANAGE_ROLES is acknowledged (BOT-C)',
      );
    },
  );
}
