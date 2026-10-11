import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'dart:ui' as ui;

import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/bots_client.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/realtime_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/bot_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/connectivity_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/settings/voice_input_settings.dart';
import 'package:voice_frontend/theme/voice_theme.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/theme/voice_token_catalog.dart';
import 'package:voice_frontend/ui/chat/chat_room_panel.dart';
import 'package:voice_frontend/ui/chat/slash_command_menu.dart';

import 'support/auth_test_overrides.dart';
import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

const _chatId = 'slash-chat';
const _draftPrefix = 'Draft prefix';
const _captureDirectoryKey = 'VOICE_SLASH_COMPOSER_CAPTURE_DIR';
const _captureBoundaryKey = ValueKey('slash_composer_capture_root');
const _command = BotSlashCommand(
  botId: 'bot-stats',
  botName: 'StatsBot',
  name: 'assign',
  description: 'Assign a value',
  options: [
    BotSlashCommandOption(name: 'reason', type: 'string', required: true),
  ],
);

var _captureFontsLoaded = false;

void main() {
  testWidgets(
    'composer slash entry runs a selected command with its typed option and restores the draft',
    (tester) async {
      final executions = <_RecordedExecution>[];
      var ordinaryMessageSends = 0;
      final providerContainer = await _pumpChatRoom(
        tester,
        executions: executions,
        onRequest: (request) async {
          if (request.url.path == '/api/v1/messages/send') {
            ordinaryMessageSends++;
            return http.Response('{}', 200);
          }
          return http.Response('{}', 404);
        },
      );
      addTearDown(providerContainer.dispose);
      final initialAuthorization = providerContainer.read(
        authorizationHeaderProvider,
      );
      expect(initialAuthorization, isNotNull);

      await _openComposerMenu(tester);
      await tester.enterText(
        find.byKey(SlashCommandMenuSheet.searchFieldKey),
        'assign',
      );
      await tester.pumpAndSettle();
      expect(find.text('/assign'), findsOneWidget);
      await tester.tap(find.text('/assign'));
      await tester.pumpAndSettle();

      expect(find.text('Run command'), findsOneWidget);
      final optionField = find.byWidgetPredicate(
        (widget) =>
            widget is TextField && widget.decoration?.labelText == 'reason *',
      );
      expect(optionField, findsOneWidget);
      await tester.enterText(optionField, 'work item');
      expect(
        tester.widget<TextField>(optionField).controller?.text,
        'work item',
      );
      await tester.pumpAndSettle();
      expect(
        initialAuthorization != null &&
            providerContainer.read(authorizationHeaderProvider) ==
                initialAuthorization,
        isTrue,
      );
      expect(
        tester
            .widget<FilledButton>(
              find.widgetWithText(FilledButton, 'Run command'),
            )
            .onPressed,
        isNotNull,
      );
      await tester.tap(find.widgetWithText(FilledButton, 'Run command'));
      await tester.pumpAndSettle();

      expect(executions, hasLength(1));
      expect(executions.single.chatId, _chatId);
      expect(executions.single.command.fullCommandName, 'assign');
      expect(jsonDecode(executions.single.optionsJson), {
        'reason': 'work item',
      });
      expect(ordinaryMessageSends, 0);
      _expectComposer(tester, _draftPrefix);
      expect(_composerHasFocus(tester), isTrue);
    },
  );

  testWidgets(
    'cancel from the composer-driven options sheet invokes nothing and restores the draft',
    (tester) async {
      final executions = <_RecordedExecution>[];
      var ordinaryMessageSends = 0;
      final providerContainer = await _pumpChatRoom(
        tester,
        executions: executions,
        onRequest: (request) async {
          if (request.url.path == '/api/v1/messages/send') {
            ordinaryMessageSends++;
            return http.Response('{}', 200);
          }
          return http.Response('{}', 404);
        },
      );
      addTearDown(providerContainer.dispose);

      await _openComposerMenu(tester);
      await tester.enterText(
        find.byKey(SlashCommandMenuSheet.searchFieldKey),
        'assign',
      );
      await tester.pumpAndSettle();
      await tester.tap(find.text('/assign'));
      await tester.pumpAndSettle();

      expect(find.text('Cancel'), findsOneWidget);
      await tester.tap(find.text('Cancel'));
      await tester.pumpAndSettle();

      expect(executions, isEmpty);
      expect(ordinaryMessageSends, 0);
      _expectComposer(tester, _draftPrefix);
      expect(_composerHasFocus(tester), isTrue);
    },
  );

  testWidgets(
    'captures the mounted composer slash menu and options in production H/V layout',
    (tester) async {
      final captureDirectory = Platform.environment[_captureDirectoryKey];
      if (captureDirectory == null || captureDirectory.isEmpty) return;
      final theme = await _loadCaptureTheme(tester);
      final scenarios = [
        (size: const Size(1280, 800), suffix: 'h'),
        (size: const Size(390, 844), suffix: 'v'),
      ];

      for (final scenario in scenarios) {
        tester.view.physicalSize = scenario.size;
        tester.view.devicePixelRatio = 1;
        final container = await _pumpChatRoom(
          tester,
          executions: <_RecordedExecution>[],
          onRequest: (_) async => http.Response('{}', 404),
          theme: theme,
        );

        await _openComposerMenu(tester);
        await _capture(tester, captureDirectory, 'menu-${scenario.suffix}.png');
        await tester.enterText(
          find.byKey(SlashCommandMenuSheet.searchFieldKey),
          'assign',
        );
        await tester.pumpAndSettle();
        await tester.tap(find.text('/assign'));
        await tester.pumpAndSettle();
        final optionField = find.byWidgetPredicate(
          (widget) =>
              widget is TextField && widget.decoration?.labelText == 'reason *',
        );
        expect(optionField, findsOneWidget);
        await tester.enterText(optionField, 'work item');
        await tester.pumpAndSettle();
        expect(
          tester
              .widget<FilledButton>(
                find.widgetWithText(FilledButton, 'Run command'),
              )
              .onPressed,
          isNotNull,
        );
        await _capture(
          tester,
          captureDirectory,
          'options-${scenario.suffix}.png',
        );

        await tester.pumpWidget(const SizedBox.shrink());
        container.dispose();
      }
      tester.view.resetPhysicalSize();
      tester.view.resetDevicePixelRatio();
    },
  );
}

Future<ProviderContainer> _pumpChatRoom(
  WidgetTester tester, {
  required List<_RecordedExecution> executions,
  required Future<http.Response> Function(http.Request) onRequest,
  ThemeData? theme,
}) async {
  final container = ProviderContainer(
    overrides: [
      ...voiceThemeTestOverrides(),
      profileAccentStorageProvider.overrideWithValue(testProfileAccentStorage),
      authSessionStorageProvider.overrideWithValue(
        InMemoryAuthSessionStorage(),
      ),
      authControllerProvider.overrideWith(authenticatedAuthController),
      gatewayConfigProvider.overrideWithValue(
        const GatewayConfig(baseUrl: 'http://api.test'),
      ),
      httpClientProvider.overrideWithValue(MockClient(onRequest)),
      realtimeHubProvider.overrideWith(_NoopRealtimeHub.new),
      realtimeAutoConnectProvider.overrideWithValue(false),
      isDeviceOfflineProvider.overrideWith((ref) => false),
      chatListControllerProvider.overrideWith(_TestChatListController.new),
      chatListProvider.overrideWith(
        (ref) async => const ChatListData(
          items: [
            ChatListItem(
              chat: VoiceChat(
                id: _chatId,
                type: 'CHAT_TYPE_GROUP',
                name: 'Slash test room',
                creatorProfileId: 'prof-test',
              ),
            ),
          ],
        ),
      ),
      chatRoomControllerProvider(
        _chatId,
      ).overrideWith((ref) => _TestChatRoomController(ref, _chatId)),
      chatTypeForChatProvider(_chatId).overrideWith((ref) => 'CHAT_TYPE_GROUP'),
      slashCommandsForChatProvider(
        _chatId,
      ).overrideWith((ref) async => const [_command]),
      slashInteractionExecutorProvider.overrideWith(
        (ref) => _RecordingSlashInteractionExecutor(ref, executions),
      ),
      voiceInputSettingsProvider.overrideWith(
        TestVoiceInputSettingsNotifier.new,
      ),
    ],
  );

  await tester.pumpWidget(
    UncontrolledProviderScope(
      container: container,
      child: MaterialApp(
        builder: (context, child) =>
            RepaintBoundary(key: _captureBoundaryKey, child: child!),
        theme: theme ?? voiceTestTheme(),
        locale: const Locale('en'),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: const Scaffold(body: ChatRoomPanel(chatId: _chatId)),
      ),
    ),
  );
  await tester.pumpAndSettle();
  return container;
}

Future<ThemeData> _loadCaptureTheme(WidgetTester tester) async {
  if (!_captureFontsLoaded) {
    final loaded = await tester.runAsync(() async {
      final noto = FontLoader(VoiceTheme.fontFamily)
        ..addFont(rootBundle.load('assets/fonts/NotoSans-Regular.ttf'))
        ..addFont(rootBundle.load('assets/fonts/NotoSans-Medium.ttf'))
        ..addFont(rootBundle.load('assets/fonts/NotoSans-SemiBold.ttf'))
        ..addFont(rootBundle.load('assets/fonts/NotoSans-Bold.ttf'));
      await noto.load();

      final flutterRoot = Platform.environment['FLUTTER_ROOT'];
      if (flutterRoot == null || flutterRoot.isEmpty) {
        throw StateError('FLUTTER_ROOT is required to load Material Icons');
      }
      final iconFile = File(
        [
          flutterRoot,
          'bin',
          'cache',
          'artifacts',
          'material_fonts',
          'MaterialIcons-Regular.otf',
        ].join(Platform.pathSeparator),
      );
      final iconBytes = await iconFile.readAsBytes();
      final iconData = ByteData.sublistView(Uint8List.fromList(iconBytes));
      await (FontLoader(
        'MaterialIcons',
      )..addFont(Future<ByteData>.value(iconData))).load();
      return true;
    });
    if (loaded != true) {
      throw StateError('Production font loading did not finish');
    }
    _captureFontsLoaded = true;
  }

  final theme = await tester.runAsync(() async {
    final catalog = await VoiceTokenCatalog.load();
    return VoiceTheme.build(
      catalog: catalog,
      mode: VoiceThemeMode.dark,
      profileAccent: catalog.profileAccentAt(0),
    );
  });
  if (theme == null) {
    throw StateError('Production Voice theme loading did not finish');
  }
  return theme;
}

Future<void> _capture(
  WidgetTester tester,
  String directory,
  String fileName,
) async {
  final boundary = tester.renderObject<RenderRepaintBoundary>(
    find.byKey(_captureBoundaryKey),
  );
  final png = await tester.runAsync(() async {
    final image = await boundary.toImage(pixelRatio: 1);
    try {
      final data = await image.toByteData(format: ui.ImageByteFormat.png);
      if (data == null) throw StateError('PNG encoding returned no bytes');
      final bytes = data.buffer.asUint8List();
      final file = File('$directory${Platform.pathSeparator}$fileName');
      await file.parent.create(recursive: true);
      await file.writeAsBytes(bytes, flush: true);
      return bytes;
    } finally {
      image.dispose();
    }
  });
  if (png == null || png.isEmpty) {
    throw StateError('Production UI capture did not finish');
  }
}

Future<void> _openComposerMenu(WidgetTester tester) async {
  await tester.enterText(
    find.descendant(
      of: find.byKey(ChatRoomPanel.inputKey),
      matching: find.byType(TextField),
    ),
    '$_draftPrefix /',
  );
  await tester.pumpAndSettle();
  expect(find.byKey(SlashCommandMenuSheet.sheetKey), findsOneWidget);
}

void _expectComposer(WidgetTester tester, String expectedText) {
  final input = tester.widget<TextField>(
    find.descendant(
      of: find.byKey(ChatRoomPanel.inputKey),
      matching: find.byType(TextField),
    ),
  );
  expect(input.controller?.text, expectedText);
}

bool _composerHasFocus(WidgetTester tester) {
  final editable = tester.widget<EditableText>(find.byType(EditableText).last);
  return editable.focusNode.hasFocus;
}

class _RecordedExecution {
  const _RecordedExecution({
    required this.chatId,
    required this.command,
    required this.optionsJson,
  });

  final String chatId;
  final BotSlashCommand command;
  final String optionsJson;
}

class _RecordingSlashInteractionExecutor extends SlashInteractionExecutor {
  _RecordingSlashInteractionExecutor(super.ref, this.executions);

  final List<_RecordedExecution> executions;

  @override
  Future<SlashInteractionFailure?> execute({
    required String chatId,
    required BotSlashCommand command,
    String optionsJson = '{}',
  }) async {
    executions.add(
      _RecordedExecution(
        chatId: chatId,
        command: command,
        optionsJson: optionsJson,
      ),
    );
    return null;
  }
}

class _TestChatRoomController extends ChatRoomController {
  _TestChatRoomController(super.ref, super.chatId) : super() {
    state = ChatRoomState(messages: [], historyProfileId: 'prof-test');
  }

  @override
  Future<void> loadInitial() async {}
}

class _TestChatListController extends ChatListController {
  _TestChatListController(super.ref) : super() {
    state = const ChatListState(
      profileId: 'prof-test',
      items: [
        ChatListItem(
          chat: VoiceChat(
            id: _chatId,
            type: 'CHAT_TYPE_GROUP',
            name: 'Slash test room',
            creatorProfileId: 'prof-test',
          ),
        ),
      ],
    );
  }

  @override
  Future<void> loadInitial() async {}
}

class _NoopRealtimeHub extends RealtimeHub {
  _NoopRealtimeHub(super.ref);

  @override
  Stream<RealtimeFrame> get events => const Stream.empty();

  @override
  Future<void> ensureConnected() async {}

  @override
  void ensureSubscribed(String chatId) {}
}
