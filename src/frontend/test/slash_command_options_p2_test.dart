import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/bots_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/bot_providers.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/ui/chat/slash_command_options_sheet.dart';

import 'support/auth_test_overrides.dart';
import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

BotSlashCommand _commandWithOptions(List<BotSlashCommandOption> options) {
  return BotSlashCommand(
    botId: 'bot-p2',
    botName: 'PickerBot',
    name: 'assign',
    description: 'Assign with pickers',
    options: options,
  );
}

void main() {
  const auth = 'Bearer access-token';

  Future<void> pumpOptionsSheet(
    WidgetTester tester, {
    required BotSlashCommand command,
    List<Override> extraOverrides = const [],
  }) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceThemeTestOverrides(),
          profileAccentStorageProvider.overrideWithValue(
            testProfileAccentStorage,
          ),
          authorizationHeaderProvider.overrideWithValue(auth),
          chatTypeForChatProvider('chat-1').overrideWith(
            (ref) => 'CHAT_TYPE_CHANNEL',
          ),
          ...extraOverrides,
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
  }

  testWidgets('slash options sheet shows user picker for type=user (BOT-B P2)', (
    tester,
  ) async {
    await pumpOptionsSheet(
      tester,
      command: _commandWithOptions(const [
        BotSlashCommandOption(name: 'member', type: 'user', required: true),
      ]),
    );

    expect(
      find.byKey(const Key('slash_option_user_picker_member')),
      findsOneWidget,
      reason: 'user option must render dedicated picker (BOT-B P2)',
    );
  });

  testWidgets('slash options sheet shows channel picker for type=channel', (
    tester,
  ) async {
    await pumpOptionsSheet(
      tester,
      command: _commandWithOptions(const [
        BotSlashCommandOption(name: 'target', type: 'channel', required: true),
      ]),
    );

    expect(
      find.byKey(const Key('slash_option_channel_picker_target')),
      findsOneWidget,
    );
  });

  testWidgets('slash options sheet shows role picker for type=role', (
    tester,
  ) async {
    await pumpOptionsSheet(
      tester,
      command: _commandWithOptions(const [
        BotSlashCommandOption(name: 'rank', type: 'role', required: true),
      ]),
    );

    expect(
      find.byKey(const Key('slash_option_role_picker_rank')),
      findsOneWidget,
    );
  });

  testWidgets('slash options sheet shows attachment picker for type=attachment', (
    tester,
  ) async {
    await pumpOptionsSheet(
      tester,
      command: _commandWithOptions(const [
        BotSlashCommandOption(name: 'file', type: 'attachment', required: true),
      ]),
    );

    expect(
      find.byKey(const Key('slash_option_attachment_picker_file')),
      findsOneWidget,
    );
  });
}
