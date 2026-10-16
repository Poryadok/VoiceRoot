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
import 'package:shared_preferences/shared_preferences.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/bots_client.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/gateway_proto_json.dart';
import 'package:voice_frontend/backend/guest_credentials_storage.dart';
import 'package:voice_frontend/backend/realtime_client.dart';
import 'package:voice_frontend/backend/spaces_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/gen/voice/common/v1/common.pb.dart' as common_pb;
import 'package:voice_frontend/gen/voice/messaging/v1/messaging.pb.dart'
    as messaging_pb;
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/bot_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/call_providers.dart';
import 'package:voice_frontend/state/connectivity_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/state/space_providers.dart';
import 'package:voice_frontend/state/subscription_providers.dart';
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
const _typedCommand = BotSlashCommand(
  botId: 'bot-stats',
  botName: 'StatsBot',
  name: 'assign',
  description: 'Assign a value to a member and channel',
  options: [
    BotSlashCommandOption(name: 'reason', type: 'string', required: true),
    BotSlashCommandOption(name: 'target', type: 'channel', required: true),
    BotSlashCommandOption(name: 'member', type: 'user', required: true),
    BotSlashCommandOption(name: 'urgent', type: 'boolean'),
  ],
);
const _offlineCommand = BotSlashCommand(
  botId: 'bot-games',
  botName: 'Game Stats',
  name: 'stats',
  description: 'Bot unavailable',
  online: false,
);

var _captureFontsLoaded = false;

void main() {
  setUp(() => SharedPreferences.setMockInitialValues({}));
  test('slash fixture scheduled response preserves the terminal page', () {
    final decoded = decodeGatewayProto(
      messaging_pb.ListScheduledMessagesResponse.create,
      _emptyScheduledResponse(),
    );
    expect(decoded.hasPage(), isTrue);
    expect(decoded.page.hasMore, isFalse);
    expect(decoded.page.nextCursor, isEmpty);
    expect(decoded.scheduledMessages, isEmpty);
  });
  for (final leftInset in [376.0, 900.0]) {
    testWidgets(
      'wide slash menu anchors above the actual composer and stays on screen at inset $leftInset',
      (tester) async {
        tester.view.physicalSize = const Size(1280, 800);
        tester.view.devicePixelRatio = 1;
        addTearDown(tester.view.resetPhysicalSize);
        addTearDown(tester.view.resetDevicePixelRatio);
        final executions = <_RecordedExecution>[];
        final container = await _pumpChatRoom(
          tester,
          executions: executions,
          onRequest: (_) async => http.Response('{}', 404),
          roomPadding: EdgeInsets.only(left: leftInset),
        );
        addTearDown(container.dispose);

        await _openComposerMenu(tester);
        final composer = tester.getRect(find.byKey(ChatRoomPanel.inputKey));
        final menu = tester.getRect(find.byKey(SlashCommandMenuSheet.sheetKey));
        expect(menu.width, closeTo(430, 0.1));
        expect(menu.bottom, closeTo(composer.top - 8, 0.1));
        expect(menu.left, closeTo(composer.left.clamp(8.0, 842.0), 0.1));
        expect(menu.top, greaterThanOrEqualTo(8));
        expect(menu.right, lessThanOrEqualTo(1272));
        expect(
          tester
              .widget<EditableText>(
                find.descendant(
                  of: find.byKey(SlashCommandMenuSheet.searchFieldKey),
                  matching: find.byType(EditableText),
                ),
              )
              .focusNode
              .hasFocus,
          isTrue,
        );
        expect(tester.takeException(), isNull);

        await tester.tapAt(const Offset(20, 20));
        await tester.pumpAndSettle();
        expect(find.byKey(SlashCommandMenuSheet.sheetKey), findsNothing);
        expect(executions, isEmpty);
        _expectComposer(tester, '$_draftPrefix /');
        expect(_composerHasFocus(tester), isTrue);
      },
    );
  }

  testWidgets('narrow slash menu retains its keyboard-height sheet', (
    tester,
  ) async {
    tester.view.physicalSize = const Size(390, 844);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    addTearDown(tester.view.resetViewInsets);
    final container = await _pumpChatRoom(
      tester,
      executions: <_RecordedExecution>[],
      onRequest: (_) async => http.Response('{}', 404),
    );
    addTearDown(container.dispose);
    await _openComposerMenu(tester);
    final menu = tester.getRect(find.byKey(SlashCommandMenuSheet.sheetKey));
    expect(menu.width, 390);
    expect(menu.bottom, 844);
    expect(menu.height, lessThan(844));
    tester.view.viewInsets = const FakeViewPadding(bottom: 300);
    await tester.pumpAndSettle();
    final aboveKeyboard = tester.getRect(
      find.byKey(SlashCommandMenuSheet.sheetKey),
    );
    expect(aboveKeyboard.bottom, closeTo(544, 0.1));
    expect(aboveKeyboard.top, greaterThanOrEqualTo(0));
    await tester.sendKeyEvent(LogicalKeyboardKey.escape);
    await tester.pumpAndSettle();
    expect(_composerHasFocus(tester), isTrue);
    expect(tester.takeException(), isNull);
  });

  for (final size in [const Size(1280, 800), const Size(390, 844)]) {
    testWidgets(
      'long composer command list scrolls within the viewport at $size',
      (tester) async {
        tester.view.physicalSize = size;
        tester.view.devicePixelRatio = 1;
        addTearDown(tester.view.resetPhysicalSize);
        addTearDown(tester.view.resetDevicePixelRatio);
        final container = await _pumpChatRoom(
          tester,
          executions: <_RecordedExecution>[],
          onRequest: (_) async => http.Response('{}', 404),
          commands: [
            for (var i = 0; i < 30; i++)
              BotSlashCommand(
                botId: 'bot-list',
                botName: 'ListBot',
                name: 'item${i.toString().padLeft(2, '0')}',
                description: 'Command $i',
              ),
          ],
        );
        addTearDown(container.dispose);
        await _openComposerMenu(tester);
        final menu = tester.getRect(find.byKey(SlashCommandMenuSheet.sheetKey));
        expect(menu.top, greaterThanOrEqualTo(0));
        expect(menu.bottom, lessThanOrEqualTo(size.height));
        final lastCommand = find.text('/item29');
        await tester.ensureVisible(lastCommand);
        await tester.pumpAndSettle();
        expect(tester.getRect(lastCommand).top, greaterThanOrEqualTo(menu.top));
        expect(
          tester.getRect(lastCommand).bottom,
          lessThanOrEqualTo(menu.bottom),
        );
        expect(tester.takeException(), isNull);
        await tester.sendKeyEvent(LogicalKeyboardKey.escape);
        await tester.pumpAndSettle();
        expect(_composerHasFocus(tester), isTrue);
      },
    );
  }

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
      final previousDisableShadows = debugDisableShadows;
      debugDisableShadows = false;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      try {
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
            commands: const [_typedCommand, _offlineCommand],
            extraOverrides: _typedOptionOverrides(),
          );
          addTearDown(container.dispose);

          await _openComposerMenu(tester);
          expect(find.text('S'), findsOneWidget);
          expect(find.text('G'), findsOneWidget);
          await _capture(
            tester,
            captureDirectory,
            'menu-${scenario.suffix}.png',
          );
          await tester.enterText(
            find.byKey(SlashCommandMenuSheet.searchFieldKey),
            'assign',
          );
          await tester.pumpAndSettle();
          await tester.tap(find.text('/assign'));
          await tester.pumpAndSettle();
          await _fillTypedOptions(tester);
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

          await tester.tap(find.byTooltip('Close'));
          await tester.pumpAndSettle();
          expect(find.text('Run command'), findsNothing);
          _expectComposer(tester, _draftPrefix);
          expect(_composerHasFocus(tester), isTrue);
          expect(tester.takeException(), isNull);
          await container.dispose();
        }
      } finally {
        debugDisableShadows = previousDisableShadows;
      }
    },
  );

  for (final size in [const Size(1280, 800), const Size(390, 844)]) {
    testWidgets(
      'options header Close restores the composer without execution at $size',
      (tester) async {
        tester.view.physicalSize = size;
        tester.view.devicePixelRatio = 1;
        addTearDown(tester.view.resetPhysicalSize);
        addTearDown(tester.view.resetDevicePixelRatio);
        final executions = <_RecordedExecution>[];
        final container = await _pumpChatRoom(
          tester,
          executions: executions,
          onRequest: (_) async => http.Response('{}', 404),
        );
        addTearDown(container.dispose);
        await _openComposerMenu(tester);
        await tester.tap(find.text('/assign'));
        await tester.pumpAndSettle();
        await tester.tap(find.byTooltip('Close'));
        await tester.pumpAndSettle();
        expect(executions, isEmpty);
        _expectComposer(tester, _draftPrefix);
        expect(_composerHasFocus(tester), isTrue);
        expect(tester.takeException(), isNull);
      },
    );

    testWidgets('real composer executes typed options exactly once at $size', (
      tester,
    ) async {
      tester.view.physicalSize = size;
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      final executions = <_RecordedExecution>[];
      var ordinarySends = 0;
      final container = await _pumpChatRoom(
        tester,
        executions: executions,
        commands: const [_typedCommand],
        extraOverrides: _typedOptionOverrides(),
        onRequest: (request) async {
          if (request.url.path == '/api/v1/messages/send') ordinarySends++;
          return http.Response('{}', 404);
        },
      );
      addTearDown(container.dispose);
      await _openComposerMenu(tester);
      await tester.tap(find.text('/assign'));
      await tester.pumpAndSettle();
      await _fillTypedOptions(tester);
      await tester.tap(find.widgetWithText(FilledButton, 'Run command'));
      await tester.pumpAndSettle();
      expect(executions, hasLength(1));
      expect(jsonDecode(executions.single.optionsJson), {
        'reason': 'work item',
        'target': 'channel-general',
        'member': 'profile-alice',
        'urgent': true,
      });
      expect(ordinarySends, 0);
      _expectComposer(tester, _draftPrefix);
      expect(_composerHasFocus(tester), isTrue);
      expect(tester.takeException(), isNull);
    });
  }
}

List<Override> _typedOptionOverrides() => [
  spaceIdForChatProvider(_chatId).overrideWith((ref) => 'space-1'),
  spaceMembersProvider('space-1').overrideWith(
    (ref) async => [
      SpaceMemberRosterEntry(
        profileId: 'profile-alice',
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
          id: 'node-general',
          spaceId: 'space-1',
          kind: 'text_chat',
          linkedChatId: 'channel-general',
          sortOrder: 0,
          displayName: 'general',
        ),
      ],
    ),
  ),
];

Future<void> _fillTypedOptions(WidgetTester tester) async {
  final reason = find.byWidgetPredicate(
    (widget) =>
        widget is TextField && widget.decoration?.labelText == 'reason *',
  );
  await tester.enterText(reason, 'work item');
  await tester.tap(find.byKey(const Key('slash_option_channel_picker_target')));
  await tester.pumpAndSettle();
  await tester.tap(find.text('general').last);
  await tester.pumpAndSettle();
  await tester.tap(find.byKey(const Key('slash_option_user_picker_member')));
  await tester.pumpAndSettle();
  await tester.tap(find.text('Alice').last);
  await tester.pumpAndSettle();
  await tester.tap(find.byType(SwitchListTile));
  await tester.pumpAndSettle();
}

String _emptyScheduledResponse() => encodeGatewayProto(
  messaging_pb.ListScheduledMessagesResponse(
    page: common_pb.CursorPageResponse(hasMore: false, nextCursor: ''),
  ),
);

class _ChatRoomFixture {
  _ChatRoomFixture(this.container, this.dispose);
  final ProviderContainer container;
  final Future<void> Function() dispose;
  T read<T>(ProviderListenable<T> provider) => container.read(provider);
}

Future<_ChatRoomFixture> _pumpChatRoom(
  WidgetTester tester, {
  required List<_RecordedExecution> executions,
  required Future<http.Response> Function(http.Request) onRequest,
  ThemeData? theme,
  EdgeInsets roomPadding = EdgeInsets.zero,
  List<BotSlashCommand> commands = const [_command],
  List<Override> extraOverrides = const [],
}) async {
  final scheduledRequests = <http.Request>[];
  final unexpectedRequests = <String>[];
  final client = MockClient((request) async {
    if (request.method == 'GET' &&
        request.url.path == '/api/v1/messages/scheduled') {
      scheduledRequests.add(request);
      expect(request.url.queryParameters['chat_id'], _chatId);
      expect(request.headers['authorization'], 'Bearer test-access');
      return http.Response(_emptyScheduledResponse(), 200);
    }
    unexpectedRequests.add('${request.method} ${request.url}');
    return onRequest(request);
  });
  _NoopRealtimeHub? hub;
  final container = ProviderContainer(
    overrides: [
      ...voiceThemeTestOverrides(),
      profileAccentStorageProvider.overrideWithValue(testProfileAccentStorage),
      authSessionStorageProvider.overrideWithValue(
        InMemoryAuthSessionStorage(),
      ),
      guestCredentialsStorageProvider.overrideWithValue(
        InMemoryGuestCredentialsStorage(),
      ),
      authControllerProvider.overrideWith(authenticatedAuthController),
      gatewayConfigProvider.overrideWithValue(
        const GatewayConfig(baseUrl: 'http://api.test'),
      ),
      httpClientProvider.overrideWithValue(client),
      realtimeHubProvider.overrideWith((ref) => hub = _NoopRealtimeHub(ref)),
      realtimeAutoConnectProvider.overrideWithValue(false),
      realtimeLinkStatusProvider.overrideWith(
        (ref) => RealtimeLinkStatus.connected,
      ),
      isDeviceOfflineProvider.overrideWith((ref) => false),
      groupActiveCallProvider(_chatId).overrideWith((ref) async => null),
      subscriptionProvider.overrideWith((ref) async => null),
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
      ).overrideWith((ref) async => commands),
      slashInteractionExecutorProvider.overrideWith(
        (ref) => _RecordingSlashInteractionExecutor(ref, executions),
      ),
      voiceInputSettingsProvider.overrideWith(
        TestVoiceInputSettingsNotifier.new,
      ),
      ...extraOverrides,
    ],
  );
  var disposed = false;
  Future<void> disposeFixture() async {
    if (disposed) return;
    disposed = true;
    try {
      await tester.pumpWidget(const SizedBox.shrink());
      await tester.pumpAndSettle();
    } finally {
      container.dispose();
      await hub?.dispose();
      client.close();
    }
    expect(unexpectedRequests, isEmpty);
  }

  addTearDown(disposeFixture);

  container.read(selectedChatIdProvider.notifier).state = _chatId;

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
        home: Scaffold(
          body: Padding(
            padding: roomPadding,
            child: const ChatRoomPanel(chatId: _chatId),
          ),
        ),
      ),
    ),
  );
  await tester.pumpAndSettle();
  final room = container.read(chatRoomControllerProvider(_chatId));
  expect(scheduledRequests, hasLength(1));
  expect(unexpectedRequests, isEmpty);
  expect(room.isLoadingScheduledMessages, isFalse);
  expect(room.scheduledMessagesError, isNull);
  expect(room.realtimeStatus, RealtimeLinkStatus.connected);
  expect(container.read(callControllerProvider).isActive, isFalse);
  expect(find.byKey(ChatRoomPanel.scheduledMessagesKey), findsNothing);
  return _ChatRoomFixture(container, disposeFixture);
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
      mode: VoiceThemeMode.light,
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
    state = ChatRoomState(
      messages: [],
      historyProfileId: 'prof-test',
      realtimeStatus: RealtimeLinkStatus.connected,
    );
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
