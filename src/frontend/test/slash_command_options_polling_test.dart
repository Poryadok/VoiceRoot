import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/bots_client.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/bot_providers.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/ui/chat/slash_command_options_sheet.dart';

import 'support/auth_test_overrides.dart';
import 'support/gateway_test_client.dart';
import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

void main() {
  const config = GatewayConfig(baseUrl: 'http://api.test');
  const auth = 'Bearer access-token';
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

  testWidgets('SlashCommandOptionsSheet retries autocomplete while pending', (
    tester,
  ) async {
    var calls = 0;
    final mock = MockClient((req) async {
      expect(req.url.path, '/api/v1/bots/autocomplete');
      calls++;
      if (calls == 1) {
        return http.Response(
          jsonEncode({'choices': [], 'pending': true}),
          200,
          headers: const {'content-type': 'application/json'},
        );
      }
      return http.Response(
        jsonEncode({
          'choices': [
            {'name': 'CS2', 'value': 'cs2'},
          ],
        }),
        200,
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
          chatTypeForChatProvider('chat-1').overrideWith(
            (ref) => 'CHAT_TYPE_CHANNEL',
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

    await tester.enterText(find.byType(TextField), 'cs');
    await tester.pump(const Duration(milliseconds: 300));
    await tester.pump(const Duration(milliseconds: 250));

    expect(calls, greaterThanOrEqualTo(2));
    expect(find.text('CS2'), findsOneWidget);
  });
}
