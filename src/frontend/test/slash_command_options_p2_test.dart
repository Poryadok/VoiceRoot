import 'dart:async';
import 'dart:ui' as ui;

import 'package:file_selector/file_selector.dart';
// The in-memory picker fixture uses file_selector's platform contract directly.
// ignore: depend_on_referenced_packages
import 'package:file_selector_platform_interface/file_selector_platform_interface.dart'
    show FileSelectorPlatform;
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/bots_client.dart';
import 'package:voice_frontend/backend/files_client.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/roles_client.dart';
import 'package:voice_frontend/backend/spaces_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/bot_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/space_providers.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/ui/chat/slash_command_options_sheet.dart';

import 'support/auth_test_overrides.dart';
import 'support/gateway_test_client.dart';
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

class _ScriptedFilesClient extends VoiceFilesClient {
  _ScriptedFilesClient({
    Set<String> failOnce = const {},
    Set<String> throwOnce = const {},
    this.pendingTicket,
    this.ticketStarted,
  }) : failOnce = Set.of(failOnce),
       throwOnce = Set.of(throwOnce),
       super(
         gateway: gatewayHttpForTest(
           MockClient((_) async => http.Response('{}', 200)),
           config: const GatewayConfig(baseUrl: 'http://api.test'),
         ),
       );

  final Set<String> failOnce;
  final Set<String> throwOnce;
  final Completer<FilesApiResult<FileUploadTicket>>? pendingTicket;
  final Completer<void>? ticketStarted;
  final Map<String, int> calls = {};

  FilesApiFailure? _failure(String stage) {
    final count = (calls[stage] ?? 0) + 1;
    calls[stage] = count;
    if (count != 1) return null;
    if (throwOnce.remove(stage)) throw StateError('synthetic upload failure');
    return failOnce.remove(stage)
        ? const FilesApiFailure(message: 'private upload diagnostic')
        : null;
  }

  @override
  Future<FilesApiResult<FileUploadTicket>> requestUpload({
    required String authorization,
    required String originalName,
    required String mimeType,
    required int sizeBytes,
    String? chatId,
    String? chatType,
    String? storyId,
    bool isE2e = false,
  }) async {
    final failure = _failure('ticket');
    if (failure != null) return failure;
    ticketStarted?.complete();
    final pending = pendingTicket;
    if (pending != null) return pending.future;
    return FilesApiOk(
      FileUploadTicket(
        fileId: 'file-1',
        presignedPutUrl: Uri.https('upload.test', '/upload'),
        r2Key: 'synthetic-object-key',
      ),
    );
  }

  @override
  Future<FilesApiResult<void>> putBytes({
    required Uri uploadUrl,
    required Uint8List bytes,
    required String mimeType,
  }) async {
    final failure = _failure('put');
    if (failure != null) return failure;
    return const FilesApiOk(null);
  }

  @override
  Future<FilesApiResult<FileMetadataData>> confirmUpload({
    required String authorization,
    required String fileId,
    required Uint8List bytes,
  }) async {
    final failure = _failure('confirm');
    if (failure != null) return failure;
    return const FilesApiOk(
      FileMetadataData(
        fileId: 'file-1',
        fileType: 'document',
        status: 'ready',
        originalName: 'attachment.txt',
      ),
    );
  }
}

class _TestFileSelectorPlatform extends FileSelectorPlatform {
  _TestFileSelectorPlatform(this.open);

  final Future<XFile?> Function() open;

  @override
  Future<XFile?> openFile({
    List<XTypeGroup>? acceptedTypeGroups,
    String? initialDirectory,
    String? confirmButtonText,
  }) => open();
}

void _useFileSelector(Future<XFile?> Function() open) {
  final previous = FileSelectorPlatform.instance;
  FileSelectorPlatform.instance = _TestFileSelectorPlatform(open);
  addTearDown(() => FileSelectorPlatform.instance = previous);
}

XFile _memoryAttachment() => XFile.fromData(
  Uint8List.fromList('synthetic test attachment'.codeUnits),
  name: 'attachment.txt',
  mimeType: 'text/plain',
);

Future<void> _settlePickedFile(WidgetTester tester) async {
  for (var attempt = 0; attempt < 20; attempt++) {
    await tester.pump(const Duration(milliseconds: 50));
  }
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
    await tester.pump();
    expect(tester.widget<FilledButton>(runFinder).onPressed, isNull);
  });

  testWidgets('attachment picker cancellation makes no upload requests', (
    tester,
  ) async {
    _useFileSelector(() async => null);
    final files = _ScriptedFilesClient();
    await pumpOptionsSheet(
      tester,
      command: _commandWithOptions(const [
        BotSlashCommandOption(name: 'file', type: 'attachment', required: true),
      ]),
      extraOverrides: [voiceFilesClientProvider.overrideWithValue(files)],
    );

    await tester.tap(
      find.descendant(
        of: find.byKey(const Key('slash_option_attachment_picker_file')),
        matching: find.byType(OutlinedButton),
      ),
    );
    await tester.pumpAndSettle();

    expect(files.calls, isEmpty);
    expect(find.text('Could not upload file. Try again.'), findsNothing);
  });

  testWidgets('attachment upload success selects the file and enables Run', (
    tester,
  ) async {
    var pickerCalls = 0;
    _useFileSelector(() async {
      pickerCalls++;
      return _memoryAttachment();
    });
    final files = _ScriptedFilesClient();
    await pumpOptionsSheet(
      tester,
      command: _commandWithOptions(const [
        BotSlashCommandOption(name: 'file', type: 'attachment', required: true),
      ]),
      extraOverrides: [voiceFilesClientProvider.overrideWithValue(files)],
    );

    await tester.tap(
      find.descendant(
        of: find.byKey(const Key('slash_option_attachment_picker_file')),
        matching: find.byType(OutlinedButton),
      ),
    );
    await _settlePickedFile(tester);

    expect(pickerCalls, 1);
    expect(find.text('Could not upload file. Try again.'), findsNothing);
    expect(files.calls, {'ticket': 1, 'put': 1, 'confirm': 1});
    expect(find.textContaining('Selected:'), findsOneWidget);
    expect(
      tester
          .widget<FilledButton>(
            find.widgetWithText(FilledButton, 'Run command'),
          )
          .onPressed,
      isNotNull,
    );
  });

  testWidgets(
    'ticket, PUT, and confirmation failures show mapped copy and recover on retry',
    (tester) async {
      _useFileSelector(() async => _memoryAttachment());

      for (final stage in ['ticket', 'put', 'confirm']) {
        for (final throws in [false, true]) {
          final files = _ScriptedFilesClient(
            failOnce: throws ? const {} : {stage},
            throwOnce: throws ? {stage} : const {},
          );
          await pumpOptionsSheet(
            tester,
            command: _commandWithOptions(const [
              BotSlashCommandOption(
                name: 'file',
                type: 'attachment',
                required: true,
              ),
            ]),
            extraOverrides: [voiceFilesClientProvider.overrideWithValue(files)],
          );

          final picker = find.descendant(
            of: find.byKey(const Key('slash_option_attachment_picker_file')),
            matching: find.byType(OutlinedButton),
          );
          await tester.tap(picker);
          await _settlePickedFile(tester);
          expect(
            find.text('Could not upload file. Try again.'),
            findsOneWidget,
          );
          expect(find.text('private upload diagnostic'), findsNothing);
          expect(find.textContaining('Selected:'), findsNothing);

          await tester.tap(picker);
          await _settlePickedFile(tester);
          expect(find.text('Could not upload file. Try again.'), findsNothing);
          expect(find.textContaining('Selected:'), findsOneWidget);
          expect(files.calls['ticket'], 2);
          expect(files.calls['put'], stage == 'ticket' ? 1 : 2);
          expect(files.calls['confirm'], stage == 'confirm' ? 2 : 1);

          await tester.tap(find.text('Cancel'));
          await tester.pumpAndSettle();
        }
      }
    },
  );

  testWidgets('attachment picker exception shows mapped copy and can retry', (
    tester,
  ) async {
    var pickerCalls = 0;
    _useFileSelector(() async {
      pickerCalls++;
      if (pickerCalls == 1) {
        throw StateError('synthetic picker error');
      }
      return _memoryAttachment();
    });
    final files = _ScriptedFilesClient();
    await pumpOptionsSheet(
      tester,
      command: _commandWithOptions(const [
        BotSlashCommandOption(name: 'file', type: 'attachment', required: true),
      ]),
      extraOverrides: [voiceFilesClientProvider.overrideWithValue(files)],
    );
    final picker = find.descendant(
      of: find.byKey(const Key('slash_option_attachment_picker_file')),
      matching: find.byType(OutlinedButton),
    );

    await tester.tap(picker);
    await tester.pumpAndSettle();
    expect(find.text('Could not upload file. Try again.'), findsOneWidget);
    expect(files.calls, isEmpty);

    await tester.tap(picker);
    await _settlePickedFile(tester);
    expect(find.text('Could not upload file. Try again.'), findsNothing);
    expect(find.textContaining('Selected:'), findsOneWidget);
    expect(files.calls, {'ticket': 1, 'put': 1, 'confirm': 1});
  });

  testWidgets('late attachment ticket after session switch is discarded', (
    tester,
  ) async {
    _useFileSelector(() async => _memoryAttachment());
    final sessionAuthorization = StateProvider<String?>((ref) => auth);
    final ticket = Completer<FilesApiResult<FileUploadTicket>>();
    final ticketStarted = Completer<void>();
    final files = _ScriptedFilesClient(
      pendingTicket: ticket,
      ticketStarted: ticketStarted,
    );
    await pumpOptionsSheet(
      tester,
      command: _commandWithOptions(const [
        BotSlashCommandOption(name: 'file', type: 'attachment', required: true),
      ]),
      sessionAuthorization: sessionAuthorization,
      extraOverrides: [voiceFilesClientProvider.overrideWithValue(files)],
    );

    await tester.tap(
      find.descendant(
        of: find.byKey(const Key('slash_option_attachment_picker_file')),
        matching: find.byType(OutlinedButton),
      ),
    );
    await tester.runAsync(
      () => Future.any([
        ticketStarted.future,
        Future<void>.delayed(const Duration(seconds: 1)),
      ]),
    );
    await tester.pump();
    final container = ProviderScope.containerOf(
      tester.element(find.text('open')),
    );
    container.read(sessionAuthorization.notifier).state =
        'Bearer switched-session';
    await tester.pump();
    ticket.complete(
      FilesApiOk(
        FileUploadTicket(
          fileId: 'file-1',
          presignedPutUrl: Uri.https('upload.test', '/upload'),
          r2Key: 'synthetic-object-key',
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(files.calls, {'ticket': 1});
    expect(find.textContaining('Selected:'), findsNothing);
    expect(find.text('Could not upload file. Try again.'), findsNothing);
    expect(
      tester
          .widget<FilledButton>(
            find.widgetWithText(FilledButton, 'Run command'),
          )
          .onPressed,
      isNull,
    );
  });

  testWidgets(
    'options sheet gives the first control keyboard focus for every option type',
    (tester) async {
      var attachmentPickerCalls = 0;
      _useFileSelector(() async {
        attachmentPickerCalls++;
        return null;
      });

      final types = [
        ('string', 'reason'),
        ('integer', 'count'),
        ('boolean', 'enabled'),
        ('user', 'member'),
        ('channel', 'target'),
        ('role', 'rank'),
        ('attachment', 'file'),
      ];
      for (final (type, name) in types) {
        final openerFocusNode = FocusNode(debugLabel: 'open-$type-options');
        addTearDown(openerFocusNode.dispose);
        final option = BotSlashCommandOption(
          name: name,
          type: type,
          required: true,
        );
        await pumpOptionsSheet(
          tester,
          command: _commandWithOptions([option]),
          openerFocusNode: openerFocusNode,
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
                const SpaceRole(
                  id: 'role-1',
                  spaceId: 'space-1',
                  name: 'Admin',
                ),
              ],
            ),
          ],
        );
        await tester.pumpAndSettle();

        expect(openerFocusNode.hasFocus, isFalse, reason: type);
        switch (type) {
          case 'string':
          case 'integer':
            expect(
              tester
                  .state<EditableTextState>(find.byType(EditableText))
                  .widget
                  .focusNode
                  .hasFocus,
              isTrue,
              reason: '$type field should receive initial focus',
            );
          case 'boolean':
            await tester.sendKeyEvent(LogicalKeyboardKey.space);
            await tester.pump();
            expect(
              tester.widget<SwitchListTile>(find.byType(SwitchListTile)).value,
              isTrue,
            );
          case 'user':
          case 'channel':
          case 'role':
            await tester.sendKeyEvent(LogicalKeyboardKey.enter);
            await tester.pumpAndSettle();
            final expectedChoice = switch (type) {
              'user' => 'Alice',
              'channel' => 'general',
              _ => 'Admin',
            };
            expect(
              find.text(expectedChoice),
              findsOneWidget,
              reason: '$type dropdown should open from initial focus',
            );
            await tester.sendKeyEvent(LogicalKeyboardKey.escape);
            await tester.pumpAndSettle();
          case 'attachment':
            await tester.sendKeyEvent(LogicalKeyboardKey.enter);
            await tester.pumpAndSettle();
            expect(attachmentPickerCalls, 1);
        }

        expect(find.text('Run command'), findsOneWidget, reason: type);
        await tester.sendKeyEvent(LogicalKeyboardKey.escape);
        await tester.pumpAndSettle();
        expect(openerFocusNode.hasFocus, isTrue, reason: '$type Escape return');
      }
    },
  );

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
      expect(run.flagsCollection.isEnabled, ui.Tristate.isFalse);
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
