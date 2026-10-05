import 'dart:ui' as ui;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/bots_client.dart';
import 'package:voice_frontend/backend/roles_client.dart';
import 'package:voice_frontend/backend/spaces_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/bot_providers.dart';
import 'package:voice_frontend/state/space_providers.dart';
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
    ValueChanged<Map<String, dynamic>?>? onResult,
    StateProvider<String?>? sessionAuthorization,
    FocusNode? openerFocusNode,
    List<Override> extraOverrides = const [],
  }) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceThemeTestOverrides(),
          profileAccentStorageProvider.overrideWithValue(
            testProfileAccentStorage,
          ),
          authorizationHeaderProvider.overrideWith(
            (ref) => sessionAuthorization == null
                ? auth
                : ref.watch(sessionAuthorization),
          ),
          chatTypeForChatProvider(
            'chat-1',
          ).overrideWith((ref) => 'CHAT_TYPE_CHANNEL'),
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
                  focusNode: openerFocusNode,
                  onPressed: () async {
                    final result = await showSlashCommandOptionsSheet(
                      context: context,
                      ref: ref,
                      chatId: 'chat-1',
                      command: command,
                    );
                    onResult?.call(result);
                  },
                  child: const Text('open'),
                ),
              ),
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
    openerFocusNode?.requestFocus();
    await tester.pump();
    await tester.tap(find.text('open'));
    await tester.pumpAndSettle();
  }

  testWidgets(
    'slash options sheet shows user picker for type=user (BOT-B P2)',
    (tester) async {
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
    },
  );

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

  testWidgets(
    'slash options sheet shows attachment picker for type=attachment',
    (tester) async {
      await pumpOptionsSheet(
        tester,
        command: _commandWithOptions(const [
          BotSlashCommandOption(
            name: 'file',
            type: 'attachment',
            required: true,
          ),
        ]),
      );

      expect(
        find.byKey(const Key('slash_option_attachment_picker_file')),
        findsOneWidget,
      );
    },
  );

  testWidgets(
    'slash options sheet Cancel dismisses without returning options',
    (tester) async {
      Map<String, dynamic>? result;
      var completed = false;
      await pumpOptionsSheet(
        tester,
        command: _commandWithOptions(const [
          BotSlashCommandOption(name: 'reason', type: 'string'),
        ]),
        onResult: (value) {
          result = value;
          completed = true;
        },
      );

      expect(find.text('Cancel'), findsOneWidget);
      await tester.tap(find.text('Cancel'));
      await tester.pumpAndSettle();

      expect(completed, isTrue);
      expect(result, isNull);
    },
  );

  testWidgets('slash options sheet blocks an invalid required integer', (
    tester,
  ) async {
    await pumpOptionsSheet(
      tester,
      command: _commandWithOptions(const [
        BotSlashCommandOption(name: 'count', type: 'integer', required: true),
      ]),
    );

    await tester.enterText(find.byType(TextField), '42');
    await tester.pumpAndSettle();
    expect(
      tester
          .widget<FilledButton>(
            find.widgetWithText(FilledButton, 'Run command'),
          )
          .onPressed,
      isNotNull,
    );

    await tester.enterText(find.byType(TextField), 'not-a-number');
    await tester.pumpAndSettle();

    expect(
      tester.widget<TextField>(find.byType(TextField)).controller!.text,
      'not-a-number',
    );
    final run = tester.widget<FilledButton>(
      find.widgetWithText(FilledButton, 'Run command'),
    );
    expect(run.onPressed, isNull);
  });

  testWidgets('slash options sheet enables Run after required string entry', (
    tester,
  ) async {
    await pumpOptionsSheet(
      tester,
      command: _commandWithOptions(const [
        BotSlashCommandOption(name: 'reason', type: 'string', required: true),
      ]),
    );

    await tester.enterText(find.byType(TextField), 'valid');
    await tester.pumpAndSettle();

    final run = tester.widget<FilledButton>(
      find.widgetWithText(FilledButton, 'Run command'),
    );
    expect(run.onPressed, isNotNull);
  });

  testWidgets('slash options sheet returns an integer as a number', (
    tester,
  ) async {
    Map<String, dynamic>? result;
    await pumpOptionsSheet(
      tester,
      command: _commandWithOptions(const [
        BotSlashCommandOption(name: 'count', type: 'integer', required: true),
      ]),
      onResult: (value) => result = value,
    );

    await tester.enterText(find.byType(TextField), '42');
    await tester.pumpAndSettle();
    final run = tester.widget<FilledButton>(
      find.widgetWithText(FilledButton, 'Run command'),
    );
    expect(run.onPressed, isNotNull);

    await tester.tap(find.widgetWithText(FilledButton, 'Run command'));
    await tester.pumpAndSettle();
    expect(result, {'count': 42});
  });

  testWidgets('slash options sheet returns an explicit false boolean', (
    tester,
  ) async {
    Map<String, dynamic>? result;
    await pumpOptionsSheet(
      tester,
      command: _commandWithOptions(const [
        BotSlashCommandOption(name: 'enabled', type: 'boolean', required: true),
      ]),
      onResult: (value) => result = value,
    );

    final runFinder = find.widgetWithText(FilledButton, 'Run command');
    expect(tester.widget<FilledButton>(runFinder).onPressed, isNull);
    await tester.tap(find.byType(SwitchListTile));
    await tester.pumpAndSettle();
    await tester.tap(find.byType(SwitchListTile));
    await tester.pumpAndSettle();
    expect(tester.widget<FilledButton>(runFinder).onPressed, isNotNull);

    await tester.tap(runFinder);
    await tester.pumpAndSettle();
    expect(result, {'enabled': false});
  });

  testWidgets(
    'slash options sheet returns selected member, channel and role IDs',
    (tester) async {
      Map<String, dynamic>? result;
      await pumpOptionsSheet(
        tester,
        command: _commandWithOptions(const [
          BotSlashCommandOption(name: 'member', type: 'user', required: true),
          BotSlashCommandOption(
            name: 'target',
            type: 'channel',
            required: true,
          ),
          BotSlashCommandOption(name: 'rank', type: 'role', required: true),
        ]),
        onResult: (value) => result = value,
        extraOverrides: [
          spaceIdForChatProvider('chat-1').overrideWith((ref) => 'space-1'),
          spaceMembersProvider('space-1').overrideWith(
            (ref) async => [
              SpaceMemberRosterEntry(
                profileId: 'profile-1',
                roleNames: const [],
                joinedAt: DateTime.utc(2026),
                nickname: 'Alice',
              ),
            ],
          ),
          spaceTreeProvider('space-1').overrideWith(
            (ref) async => const SpaceTreeData(
              categories: [],
              voiceRooms: [],
              nodes: [
                SpaceTreeNodeData(
                  id: 'node-1',
                  spaceId: 'space-1',
                  kind: 'text_chat',
                  linkedChatId: 'chat-2',
                  sortOrder: 0,
                  displayName: 'general',
                ),
              ],
            ),
          ),
          spaceRolesProvider('space-1').overrideWith(
            (ref) async => [
              const SpaceRole(id: 'role-1', spaceId: 'space-1', name: 'Admin'),
            ],
          ),
        ],
      );
      await tester.pumpAndSettle();

      final selectors = find.byType(DropdownButtonFormField<String>);
      for (final (index, label) in [
        (0, 'Alice'),
        (1, 'general'),
        (2, 'Admin'),
      ]) {
        await tester.tap(selectors.at(index));
        await tester.pumpAndSettle();
        await tester.tap(find.text(label).last);
        await tester.pumpAndSettle();
      }

      final run = find.widgetWithText(FilledButton, 'Run command');
      expect(tester.widget<FilledButton>(run).onPressed, isNotNull);
      await tester.tap(run);
      await tester.pumpAndSettle();
      expect(result, {
        'member': 'profile-1',
        'target': 'chat-2',
        'rank': 'role-1',
      });
    },
  );

  testWidgets('Run disables if authorization changes while options are open', (
    tester,
  ) async {
    final sessionAuthorization = StateProvider<String?>((ref) => auth);
    await pumpOptionsSheet(
      tester,
      command: _commandWithOptions(const [
        BotSlashCommandOption(name: 'reason', type: 'string', required: true),
      ]),
      sessionAuthorization: sessionAuthorization,
    );

    await tester.enterText(find.byType(TextField), 'valid');
    await tester.pumpAndSettle();
    final runFinder = find.widgetWithText(FilledButton, 'Run command');
    expect(tester.widget<FilledButton>(runFinder).onPressed, isNotNull);

    final container = ProviderScope.containerOf(
      tester.element(find.text('open')),
    );
    container.read(sessionAuthorization.notifier).state =
        'Bearer switched-session';
    await tester.pumpAndSettle();
    expect(tester.widget<FilledButton>(runFinder).onPressed, isNull);
  });

  testWidgets(
    'options sheet supports keyboard focus, Escape restoration, and semantics',
    (tester) async {
      final openerFocusNode = FocusNode(debugLabel: 'slash-options-trigger');
      addTearDown(openerFocusNode.dispose);
      await pumpOptionsSheet(
        tester,
        command: _commandWithOptions(const [
          BotSlashCommandOption(name: 'reason', type: 'string', required: true),
        ]),
        openerFocusNode: openerFocusNode,
      );

      final textField = tester.state<EditableTextState>(
        find.byType(EditableText),
      );
      expect(
        textField.widget.focusNode.hasFocus,
        isTrue,
        reason: 'the first actionable form control receives initial focus',
      );
      expect(openerFocusNode.hasFocus, isFalse);

      final run = tester.getSemantics(find.text('Run command'));
      expect(run.label, 'Run command');
      expect(run.hasFlag(ui.SemanticsFlag.hasEnabledState), isTrue);
      expect(run.hasFlag(ui.SemanticsFlag.isEnabled), isFalse);
      expect(tester.getSemantics(find.text('Cancel')).label, 'Cancel');

      for (var i = 0; i < 6; i++) {
        await tester.sendKeyEvent(LogicalKeyboardKey.tab);
        await tester.pump();
        expect(openerFocusNode.hasFocus, isFalse);
      }

      await tester.sendKeyEvent(LogicalKeyboardKey.escape);
      await tester.pumpAndSettle();
      expect(find.text('Run command'), findsNothing);
      expect(openerFocusNode.hasFocus, isTrue);

      await tester.sendKeyEvent(LogicalKeyboardKey.enter);
      await tester.pumpAndSettle();
      expect(find.text('Run command'), findsOneWidget);
      await tester.sendKeyEvent(LogicalKeyboardKey.escape);
      await tester.pumpAndSettle();
      expect(openerFocusNode.hasFocus, isTrue);

      await tester.sendKeyEvent(LogicalKeyboardKey.space);
      await tester.pumpAndSettle();
      expect(find.text('Run command'), findsOneWidget);
      await tester.sendKeyEvent(LogicalKeyboardKey.escape);
      await tester.pumpAndSettle();
      expect(openerFocusNode.hasFocus, isTrue);
    },
  );
}
