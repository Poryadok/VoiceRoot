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

import 'support/auth_test_overrides.dart';
import 'support/gateway_test_client.dart';
import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

void main() {
  const config = GatewayConfig(baseUrl: 'http://api.test');
  const auth = 'Bearer access-token';
  const command = BotSlashCommand(
    botId: 'bot-1',
    botName: 'SlowBot',
    name: 'ping',
    description: 'Ping',
  );

  testWidgets('slash flow shows botTimeoutError SnackBar on bot_timeout', (
    tester,
  ) async {
    final mock = MockClient((req) async {
      expect(req.url.path, '/api/v1/bots/interactions');
      return http.Response(
        jsonEncode({'error_code': 'bot_timeout', 'content': ''}),
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
            (ref) => 'CHAT_TYPE_GROUP',
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
                  onPressed: () async {
                    final failure = await ref
                        .read(slashInteractionExecutorProvider)
                        .execute(
                          chatId: 'chat-1',
                          command: command,
                        );
                    if (!context.mounted) return;
                    if (failure == SlashInteractionFailure.botTimeout) {
                      ScaffoldMessenger.of(context).showSnackBar(
                        SnackBar(
                          content: Text(
                            AppLocalizations.of(context)!.botTimeoutError,
                          ),
                        ),
                      );
                    }
                  },
                  child: const Text('run'),
                ),
              ),
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.text('run'));
    await tester.pumpAndSettle();

    expect(
      find.text('The bot did not respond in time. Try again later.'),
      findsOneWidget,
    );
  });
}
