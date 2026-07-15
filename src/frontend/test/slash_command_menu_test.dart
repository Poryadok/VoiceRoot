import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/bots_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/bot_providers.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/ui/chat/slash_command_menu.dart';

import 'support/auth_test_overrides.dart';
import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

const _commands = [
  BotSlashCommand(
    botId: 'bot-1',
    botName: 'PingBot',
    name: 'ping',
    description: 'Replies with pong',
  ),
  BotSlashCommand(
    botId: 'bot-2',
    botName: 'HelpBot',
    name: 'help',
    description: 'Shows help',
  ),
  BotSlashCommand(
    botId: 'bot-offline',
    botName: 'DownBot',
    name: 'slow',
    description: 'Unavailable',
    online: false,
  ),
];

void main() {
  Widget slashMenuApp({
    required List<Override> overrides,
    required void Function(BotSlashCommand command) onSelected,
    String filter = '',
  }) {
    return ProviderScope(
      overrides: [
        ...voiceThemeTestOverrides(),
        profileAccentStorageProvider.overrideWithValue(
          testProfileAccentStorage,
        ),
        ...overrides,
      ],
      child: MaterialApp(
        theme: voiceTestTheme(),
        locale: const Locale('en'),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: Scaffold(
          body: SlashCommandMenuSheet(
            chatId: 'chat-1',
            filter: filter,
            onSelected: onSelected,
          ),
        ),
      ),
    );
  }

  testWidgets('SlashCommandMenuSheet lists commands and handles selection', (
    tester,
  ) async {
    BotSlashCommand? picked;
    await tester.pumpWidget(
      slashMenuApp(
        overrides: [
          slashCommandsForChatProvider('chat-1').overrideWith(
            (ref) async => _commands,
          ),
        ],
        onSelected: (command) => picked = command,
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Commands'), findsOneWidget);
    expect(find.text('/ping'), findsOneWidget);
    expect(find.text('Replies with pong'), findsOneWidget);

    await tester.tap(find.text('/ping'));
    await tester.pumpAndSettle();

    expect(picked?.name, 'ping');
    expect(picked?.botId, 'bot-1');
  });

  testWidgets('SlashCommandMenuSheet filters commands by query', (
    tester,
  ) async {
    await tester.pumpWidget(
      slashMenuApp(
        overrides: [
          slashCommandsForChatProvider('chat-1').overrideWith(
            (ref) async => _commands,
          ),
        ],
        onSelected: (_) {},
        filter: 'help',
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('/help'), findsOneWidget);
    expect(find.text('/ping'), findsNothing);
  });

  testWidgets('SlashCommandMenuSheet greys out offline bot commands', (
    tester,
  ) async {
    await tester.pumpWidget(
      slashMenuApp(
        overrides: [
          slashCommandsForChatProvider('chat-1').overrideWith(
            (ref) async => _commands,
          ),
        ],
        onSelected: (_) {},
      ),
    );
    await tester.pumpAndSettle();

    final slowTile = find.byKey(
      const ValueKey('slash_command_bot-offline_slow'),
    );
    expect(slowTile, findsOneWidget);
    final tile = tester.widget<ListTile>(slowTile);
    expect(tile.enabled, isFalse);
  });

  testWidgets('SlashCommandMenuSheet shows help footer when commands exist', (
    tester,
  ) async {
    await tester.pumpWidget(
      slashMenuApp(
        overrides: [
          slashCommandsForChatProvider('chat-1').overrideWith(
            (ref) async => _commands,
          ),
        ],
        onSelected: (_) {},
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(SlashCommandMenuSheet.helpFooterKey), findsOneWidget);
    expect(
      find.text(
        'Type / in the message box to open this menu. Greyed-out commands mean the bot is offline.',
      ),
      findsOneWidget,
    );
  });

  testWidgets('SlashCommandMenuSheet empty state explains next step in space chat', (
    tester,
  ) async {
    await tester.pumpWidget(
      slashMenuApp(
        overrides: [
          slashCommandsForChatProvider('chat-1').overrideWith(
            (ref) async => const [],
          ),
          spaceIdForChatProvider('chat-1').overrideWith((ref) => 'space-1'),
        ],
        onSelected: (_) {},
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(SlashCommandMenuSheet.emptyStateKey), findsOneWidget);
    expect(find.text('No bot commands in this chat.'), findsOneWidget);
    expect(
      find.text(
        'Install bots in Space settings, or enable them for this chat in Chat info.',
      ),
      findsOneWidget,
    );
  });

  testWidgets('SlashCommandMenuSheet empty state explains DM limitation', (
    tester,
  ) async {
    await tester.pumpWidget(
      slashMenuApp(
        overrides: [
          slashCommandsForChatProvider('chat-1').overrideWith(
            (ref) async => const [],
          ),
          spaceIdForChatProvider('chat-1').overrideWith((ref) => null),
        ],
        onSelected: (_) {},
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(SlashCommandMenuSheet.emptyStateKey), findsOneWidget);
    expect(
      find.text('Bot slash commands are available only in space text chats.'),
      findsOneWidget,
    );
  });

  testWidgets('SlashCommandMenuSheet filter miss shows search empty state', (
    tester,
  ) async {
    await tester.pumpWidget(
      slashMenuApp(
        overrides: [
          slashCommandsForChatProvider('chat-1').overrideWith(
            (ref) async => _commands,
          ),
        ],
        onSelected: (_) {},
        filter: 'missing',
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(SlashCommandMenuSheet.noMatchStateKey), findsOneWidget);
    expect(find.text('No matching commands'), findsOneWidget);
    expect(
      find.text(
        'Keep typing after / or try another command or bot name.',
      ),
      findsOneWidget,
    );
    expect(find.byKey(SlashCommandMenuSheet.helpFooterKey), findsNothing);
  });
}
