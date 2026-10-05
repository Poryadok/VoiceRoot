import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'dart:ui' as ui;

import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/messages_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/ui/chat/chat_info_panel.dart';
import 'package:voice_frontend/ui/chat/pinned_messages_panel.dart';
import 'package:voice_frontend/ui/chat/chat_room_panel.dart';

import 'support/auth_test_overrides.dart';
import 'support/fake_voice_api_clients.dart';
import 'support/gateway_test_client.dart';
import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

void main() {
  setUpAll(() async {
    SharedPreferences.setMockInitialValues({});
    await _loadCaptureFonts();
  });

  testWidgets('pinned bar exposes a distinct open-list action', (tester) async {
    await _setDesktopViewport(tester);
    await tester.pumpWidget(_pinnedRoomApp());
    await tester.pumpAndSettle();

    expect(find.byKey(ChatRoomPanel.pinnedBarKey), findsOneWidget);
    expect(
      find.descendant(
        of: find.byKey(ChatRoomPanel.pinnedBarKey),
        matching: find.byTooltip('Open all pinned messages'),
      ),
      findsOneWidget,
    );
  });

  testWidgets('pinned photo bar shows its canonical media type', (
    tester,
  ) async {
    await _setDesktopViewport(tester);
    await tester.pumpWidget(
      _pinnedRoomApp(contentType: 'MESSAGE_CONTENT_TYPE_PHOTO'),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(ChatRoomPanel.pinnedBarKey), findsOneWidget);
    expect(find.text('Photo'), findsOneWidget);
  });

  testWidgets('bar advances through pins and wraps to the newest', (
    tester,
  ) async {
    await _setDesktopViewport(tester);
    await tester.pumpWidget(_pinnedRoomApp(pinCount: 2));
    await tester.pumpAndSettle();

    expect(find.text('Pinned announcement 0'), findsOneWidget);
    await tester.tap(find.byKey(ChatRoomPanel.pinnedBarKey));
    await tester.pumpAndSettle(const Duration(milliseconds: 300));
    expect(find.text('Pinned announcement 1'), findsOneWidget);

    await tester.tap(find.byKey(ChatRoomPanel.pinnedBarKey));
    await tester.pumpAndSettle(const Duration(milliseconds: 300));
    expect(find.text('Pinned announcement 0'), findsOneWidget);
  });

  testWidgets('hide control removes the bar and header control restores it', (
    tester,
  ) async {
    await _setDesktopViewport(tester);
    await tester.pumpWidget(_pinnedRoomApp());
    await tester.pumpAndSettle();

    await tester.tap(
      find.descendant(
        of: find.byKey(ChatRoomPanel.pinnedBarKey),
        matching: find.byTooltip('Hide pinned messages'),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(ChatRoomPanel.pinnedBarKey), findsNothing);
    expect(find.byTooltip('Show pinned messages'), findsOneWidget);
    await tester.tap(find.byKey(ChatRoomPanel.pinnedMessagesHeaderKey));
    await tester.pumpAndSettle();
    expect(find.byKey(ChatRoomPanel.pinnedBarKey), findsOneWidget);
  });

  testWidgets('left swipe hides the mobile pin bar and header restores it', (
    tester,
  ) async {
    await _setViewport(tester, const Size(390, 844));
    await tester.pumpWidget(_pinnedRoomApp());
    await tester.pumpAndSettle();

    final bar = find.byKey(ChatRoomPanel.pinnedBarKey);
    expect(bar, findsOneWidget);
    await tester.fling(bar, const Offset(-500, 0), 1000);
    await tester.pumpAndSettle();

    expect(find.byKey(ChatRoomPanel.pinnedBarKey), findsNothing);
    final restore = find.byTooltip('Show pinned messages');
    expect(restore, findsOneWidget);
    await tester.tap(restore);
    await tester.pumpAndSettle();
    expect(find.byKey(ChatRoomPanel.pinnedBarKey), findsOneWidget);
  });

  testWidgets('room list opens all five pins and selects the tapped row', (
    tester,
  ) async {
    await _setDesktopViewport(tester);
    String? selectedId;
    await tester.pumpWidget(
      MaterialApp(
        theme: voiceTestTheme(),
        locale: const Locale('en'),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: Builder(
          builder: (context) => Scaffold(
            body: Center(
              child: TextButton(
                onPressed: () => PinnedMessagesPanel.show(
                  context,
                  messages: List.generate(5, (index) => _message(index)),
                  onOpenMessage: (id) async {
                    selectedId = id;
                    return true;
                  },
                ),
                child: const Text('Open pins'),
              ),
            ),
          ),
        ),
      ),
    );

    await tester.tap(find.text('Open pins'));
    await tester.pumpAndSettle();
    expect(find.byKey(PinnedMessagesPanel.panelKey), findsOneWidget);
    expect(find.text('Pinned messages'), findsOneWidget);
    for (var index = 0; index < 5; index++) {
      expect(
        find.byKey(ValueKey('chat_pinned_message_pin-$index')),
        findsOneWidget,
      );
    }

    await tester.tap(find.byKey(const ValueKey('chat_pinned_message_pin-3')));
    await tester.pumpAndSettle();
    expect(selectedId, 'pin-3');
    expect(find.byKey(PinnedMessagesPanel.panelKey), findsNothing);
  });

  testWidgets('pin jump loads an older cursor page before closing the list', (
    tester,
  ) async {
    await _setDesktopViewport(tester);
    await tester.pumpWidget(_pinnedRoomApp(pageHistoryForPinnedTarget: true));
    await tester.pumpAndSettle();

    await tester.tap(find.byTooltip('Open all pinned messages'));
    await tester.pumpAndSettle();
    expect(find.byKey(PinnedMessagesPanel.panelKey), findsOneWidget);
    await tester.tap(find.byKey(const ValueKey('chat_pinned_message_pin-0')));
    await tester.pumpAndSettle();

    expect(find.byKey(PinnedMessagesPanel.panelKey), findsNothing);
    expect(find.byKey(const Key('chat_room_messages')), findsOneWidget);
  });

  testWidgets('failed older-page pin jump stays open and retries its cursor', (
    tester,
  ) async {
    await _setDesktopViewport(tester);
    var olderRequests = 0;
    final cursors = <String?>[];
    await tester.pumpWidget(
      _pinnedRoomApp(
        pageHistoryForPinnedTarget: true,
        failFirstOlderHistoryRequest: true,
        onOlderHistoryRequest: (cursor) {
          olderRequests++;
          cursors.add(cursor);
        },
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byTooltip('Open all pinned messages'));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const ValueKey('chat_pinned_message_pin-0')));
    await tester.pumpAndSettle();

    expect(olderRequests, 1);
    expect(find.byKey(PinnedMessagesPanel.panelKey), findsOneWidget);
    expect(find.byKey(const Key('chat_pinned_jump_error')), findsOneWidget);
    await tester.tap(find.byKey(const Key('chat_pinned_jump_retry')));
    await tester.pumpAndSettle();

    expect(olderRequests, 2);
    expect(cursors, ['older-pins', 'older-pins']);
    expect(find.byKey(PinnedMessagesPanel.panelKey), findsNothing);
  });

  testWidgets('cancelling a pending older-page jump prevents follow-up pages', (
    tester,
  ) async {
    await _setDesktopViewport(tester);
    final releaseOlderPage = Completer<void>();
    var olderRequests = 0;
    await tester.pumpWidget(
      _pinnedRoomApp(
        pageHistoryForPinnedTarget: true,
        holdFirstOlderHistoryRequest: releaseOlderPage,
        onOlderHistoryRequest: (_) => olderRequests++,
      ),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.byTooltip('Open all pinned messages'));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const ValueKey('chat_pinned_message_pin-0')));
    await tester.pump();

    expect(olderRequests, 1);
    expect(find.byKey(PinnedMessagesPanel.panelKey), findsOneWidget);
    await tester.tap(find.byKey(PinnedMessagesPanel.closeKey));
    await tester.pumpAndSettle();
    expect(find.byKey(PinnedMessagesPanel.panelKey), findsNothing);

    releaseOlderPage.complete();
    await tester.pumpAndSettle();
    expect(olderRequests, 1);
  });

  testWidgets('pinned list supports Escape and mobile swipe dismissal', (
    tester,
  ) async {
    await _setDesktopViewport(tester);
    await tester.pumpWidget(_pinnedRoomApp());
    await tester.pumpAndSettle();
    await tester.tap(find.byTooltip('Open all pinned messages'));
    await tester.pumpAndSettle();
    expect(find.byKey(PinnedMessagesPanel.panelKey), findsOneWidget);
    await tester.sendKeyEvent(LogicalKeyboardKey.escape);
    await tester.pumpAndSettle();
    expect(find.byKey(PinnedMessagesPanel.panelKey), findsNothing);

    tester.view.resetPhysicalSize();
    tester.view.resetDevicePixelRatio();
    await _setViewport(tester, const Size(390, 844));
    await tester.tap(find.byTooltip('Open all pinned messages'));
    await tester.pumpAndSettle();
    expect(find.byKey(PinnedMessagesPanel.panelKey), findsOneWidget);
    await tester.drag(find.text('Pinned messages'), const Offset(0, 500));
    await tester.pumpAndSettle();
    expect(find.byKey(PinnedMessagesPanel.panelKey), findsNothing);
  });

  testWidgets('pinned list traps Tab focus and restores the live trigger', (
    tester,
  ) async {
    await _setDesktopViewport(tester);
    await tester.pumpWidget(_pinnedRoomApp(pinCount: 5));
    await tester.pumpAndSettle();

    final trigger = find.byTooltip('Open all pinned messages');
    var triggerFocused = false;
    for (var index = 0; index < 80; index++) {
      if (_primaryFocusWithin(tester, trigger)) {
        triggerFocused = true;
        break;
      }
      await tester.sendKeyEvent(LogicalKeyboardKey.tab);
      await tester.pump();
    }
    expect(triggerFocused || _primaryFocusWithin(tester, trigger), isTrue);
    expect(_primaryFocusWithin(tester, trigger), isTrue);
    await tester.sendKeyEvent(LogicalKeyboardKey.enter);
    await tester.pumpAndSettle();

    final panel = find.byKey(PinnedMessagesPanel.panelKey);
    expect(panel, findsOneWidget);
    for (var index = 0; index < 8; index++) {
      await tester.sendKeyEvent(LogicalKeyboardKey.tab);
      await tester.pump();
      expect(_primaryFocusWithin(tester, panel), isTrue);
    }
    for (var index = 0; index < 4; index++) {
      await tester.sendKeyDownEvent(LogicalKeyboardKey.shiftLeft);
      await tester.sendKeyEvent(LogicalKeyboardKey.tab);
      await tester.sendKeyUpEvent(LogicalKeyboardKey.shiftLeft);
      await tester.pump();
      expect(_primaryFocusWithin(tester, panel), isTrue);
    }

    await tester.sendKeyEvent(LogicalKeyboardKey.escape);
    await tester.pumpAndSettle();
    expect(find.byKey(PinnedMessagesPanel.panelKey), findsNothing);
    expect(_primaryFocusWithin(tester, trigger), isTrue);
  });

  testWidgets('failed pin fetch is not treated as a confirmed empty list', (
    tester,
  ) async {
    await _setDesktopViewport(tester);
    var pinRequests = 0;
    await tester.pumpWidget(
      _pinnedRoomApp(
        pinCount: 0,
        failFirstPinnedRequest: true,
        onPinnedRequest: () => pinRequests++,
      ),
    );
    await tester.pumpAndSettle();

    expect(pinRequests, 1);
    final error = find.byKey(const Key('chat_room_pins_error'));
    expect(error, findsOneWidget);
    final l10n = AppLocalizations.of(tester.element(error))!;
    expect(
      find.descendant(of: error, matching: find.text(l10n.backendUnavailable)),
      findsOneWidget,
    );
    await tester.tap(
      find.descendant(of: error, matching: find.text('Try again')),
    );
    await tester.pumpAndSettle();

    expect(pinRequests, 2);
    expect(find.byKey(const Key('chat_room_pins_error')), findsNothing);
    expect(find.byKey(ChatRoomPanel.pinnedBarKey), findsNothing);
  });

  testWidgets(
    'pin-load error keeps full accessible retry target at 1.5x in H and V',
    (tester) async {
      final captureDir = Platform.environment['VOICE_PINNED_CAPTURE_DIR'];
      for (final viewport in const [Size(1280, 800), Size(390, 844)]) {
        final orientation = viewport.width > viewport.height ? 'h' : 'v';
        await _setViewport(tester, viewport);
        await tester.pumpWidget(
          _pinnedRoomApp(
            pinCount: 0,
            failFirstPinnedRequest: true,
            textScale: 1.5,
            captureBoundary: captureDir != null && captureDir.isNotEmpty,
          ),
        );
        await tester.pumpAndSettle();

        final error = find.byKey(const Key('chat_room_pins_error'));
        expect(error, findsOneWidget);
        final l10n = AppLocalizations.of(tester.element(error))!;
        final message = find.descendant(
          of: error,
          matching: find.text(l10n.backendUnavailable),
        );
        expect(message, findsOneWidget);
        expect(find.bySemanticsLabel(l10n.backendUnavailable), findsOneWidget);
        final retry = find.descendant(
          of: error,
          matching: find.byType(TextButton),
        );
        expect(retry, findsOneWidget);
        expect(tester.getSize(retry).height, greaterThanOrEqualTo(48));
        expect(tester.takeException(), isNull);
        if (captureDir != null && captureDir.isNotEmpty) {
          await _writeCapture(
            tester,
            captureDir,
            'pin_error_${orientation}_1_5',
          );
        }
        await tester.pumpWidget(const SizedBox.shrink());
      }
    },
  );

  testWidgets('standalone Chat Info pin jump can be cancelled safely', (
    tester,
  ) async {
    await _setDesktopViewport(tester);
    _NoopRealtimeHub? realtimeHub;
    var pinRequests = 0;
    await tester.pumpWidget(
      _pinnedRoomApp(
        standaloneInfo: true,
        onRealtimeHubCreated: (hub) => realtimeHub = hub,
        onPinnedRequest: () => pinRequests++,
      ),
    );
    await tester.pumpAndSettle();

    expect(pinRequests, 1);
    expect(realtimeHub, isNull);
    expect(find.byKey(ChatInfoPanel.pinnedMessagesKey), findsOneWidget);
    await tester.tap(find.byKey(ChatInfoPanel.pinnedMessagesKey));
    await tester.pumpAndSettle();
    expect(find.byKey(PinnedMessagesPanel.panelKey), findsOneWidget);
    expect(find.text('Pinned messages'), findsNWidgets(2));

    await tester.tap(find.byKey(const ValueKey('chat_pinned_message_pin-0')));
    await tester.pump();
    expect(realtimeHub, isNotNull);
    expect(find.byKey(PinnedMessagesPanel.panelKey), findsOneWidget);
    await tester.tap(find.byKey(PinnedMessagesPanel.closeKey));
    await tester.pumpAndSettle();
    expect(find.byKey(PinnedMessagesPanel.panelKey), findsNothing);
  });

  testWidgets('standalone Chat Info retries a failed pinned-message load', (
    tester,
  ) async {
    await _setDesktopViewport(tester);
    var pinRequests = 0;
    await tester.pumpWidget(
      _pinnedRoomApp(
        standaloneInfo: true,
        onPinnedRequest: () => pinRequests++,
        failFirstPinnedRequest: true,
      ),
    );
    await tester.pumpAndSettle();

    expect(pinRequests, 1);
    final errorPanel = find.byKey(const Key('chat_info_pins_error'));
    final l10n = AppLocalizations.of(tester.element(errorPanel))!;
    expect(
      find.descendant(
        of: errorPanel,
        matching: find.text(l10n.backendUnavailable),
      ),
      findsOneWidget,
    );
    final retry = find.descendant(
      of: errorPanel,
      matching: find.byTooltip('Try again'),
    );
    expect(retry, findsOneWidget);
    await tester.tap(retry);
    await tester.pumpAndSettle();

    expect(pinRequests, 2);
    expect(find.byKey(ChatInfoPanel.pinnedMessagesKey), findsOneWidget);
    expect(retry, findsNothing);
  });

  testWidgets('production-font pinned bar capture H', (tester) async {
    final captureDir = Platform.environment['VOICE_PINNED_CAPTURE_DIR'];
    if (captureDir == null || captureDir.isEmpty) return;
    await _setViewport(tester, const Size(1280, 800));
    await tester.pumpWidget(_pinnedRoomApp(pinCount: 5, captureBoundary: true));
    await _pumpCaptureRoom(tester);
    await _writeCapture(tester, captureDir, 'pinned_navigation_h');
  });

  testWidgets('production-font pinned bar capture V', (tester) async {
    final captureDir = Platform.environment['VOICE_PINNED_CAPTURE_DIR'];
    if (captureDir == null || captureDir.isEmpty) return;
    await _setViewport(tester, const Size(390, 844));
    await tester.pumpWidget(_pinnedRoomApp(pinCount: 5, captureBoundary: true));
    await _pumpCaptureRoom(tester);
    await _writeCapture(tester, captureDir, 'pinned_navigation_v');
  });

  testWidgets('production-font open pin list capture H', (tester) async {
    final captureDir = Platform.environment['VOICE_PINNED_CAPTURE_DIR'];
    if (captureDir == null || captureDir.isEmpty) return;
    await _setViewport(tester, const Size(1280, 800));
    await tester.pumpWidget(_pinnedRoomApp(pinCount: 5, captureBoundary: true));
    await _pumpCaptureRoom(tester);
    await tester.tap(find.byTooltip('Open all pinned messages'));
    await tester.pumpAndSettle();
    await _writeCapture(tester, captureDir, 'pinned_list_h');
  });

  testWidgets('production-font open pin list capture V', (tester) async {
    final captureDir = Platform.environment['VOICE_PINNED_CAPTURE_DIR'];
    if (captureDir == null || captureDir.isEmpty) return;
    await _setViewport(tester, const Size(390, 844));
    await tester.pumpWidget(_pinnedRoomApp(pinCount: 5, captureBoundary: true));
    await _pumpCaptureRoom(tester);
    await tester.tap(find.byTooltip('Open all pinned messages'));
    await tester.pumpAndSettle();
    await _writeCapture(tester, captureDir, 'pinned_list_v');
  });
}

Future<void> _pumpCaptureRoom(WidgetTester tester) async {
  final roomHeader = find.text('Pinned chat partner');
  for (var frame = 0; frame < 40; frame++) {
    await tester.pump(const Duration(milliseconds: 50));
    if (roomHeader.evaluate().isNotEmpty &&
        find.byKey(ChatRoomPanel.pinnedBarKey).evaluate().isNotEmpty) {
      break;
    }
  }
  expect(roomHeader, findsOneWidget);
  expect(find.byKey(ChatRoomPanel.pinnedBarKey), findsOneWidget);
  await tester.pump(const Duration(milliseconds: 100));
}

Future<void> _setDesktopViewport(WidgetTester tester) async {
  tester.view.physicalSize = const Size(900, 900);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);
}

Widget _pinnedRoomApp({
  String? contentType,
  int pinCount = 1,
  double textScale = 1,
  bool standaloneInfo = false,
  bool captureBoundary = false,
  bool failFirstPinnedRequest = false,
  bool failFirstOlderHistoryRequest = false,
  Completer<void>? holdFirstOlderHistoryRequest,
  bool pageHistoryForPinnedTarget = false,
  void Function()? onPinnedRequest,
  void Function(String?)? onOlderHistoryRequest,
  void Function(_NoopRealtimeHub)? onRealtimeHubCreated,
}) {
  final pinned = List.generate(pinCount, (index) {
    final contentTypes = [
      contentType ?? 'MESSAGE_CONTENT_TYPE_PHOTO',
      'MESSAGE_CONTENT_TYPE_VOICE',
      'MESSAGE_CONTENT_TYPE_DOCUMENT',
      'MESSAGE_CONTENT_TYPE_ARTICLE',
      'MESSAGE_CONTENT_TYPE_MUSIC',
    ];
    return <String, Object?>{
      'id': 'pin-$index',
      'chat': {'id': 'chat-pins'},
      'sender_profile_id': 'profile-b',
      'content': 'Pinned announcement $index',
      'is_pinned': true,
      'reactions_json': '[]',
      'mentions_json': '[]',
      'attachments_json': '[]',
      'type': 'regular',
      'content_type': contentTypes[index % contentTypes.length],
      'created_at': '2024-01-0${index + 1}T00:00:00Z',
    };
  });
  var pinnedRequestCount = 0;
  var olderHistoryRequestCount = 0;
  final client = MockClient((request) async {
    if (request.url.path == '/api/v1/messages') {
      final cursor = request.url.queryParameters['cursor'];
      if (cursor == 'older-pins') {
        olderHistoryRequestCount++;
        onOlderHistoryRequest?.call(cursor);
        if (olderHistoryRequestCount == 1 &&
            holdFirstOlderHistoryRequest != null) {
          await holdFirstOlderHistoryRequest.future;
        }
        if (failFirstOlderHistoryRequest && olderHistoryRequestCount == 1) {
          return http.Response('{}', 503);
        }
        final olderPage = pinned.first;
        return utf8JsonResponse(
          jsonEncode({
            'message_list': {
              'messages': [olderPage],
              'has_more': false,
            },
          }),
        );
      }
      return utf8JsonResponse(
        jsonEncode({
          'message_list': pageHistoryForPinnedTarget
              ? {
                  'messages': [
                    {
                      'id': 'newest',
                      'chat': {'id': 'chat-pins'},
                      'sender_profile_id': 'profile-b',
                      'content': 'Newest message',
                      'is_pinned': false,
                      'reactions_json': '[]',
                      'mentions_json': '[]',
                      'attachments_json': '[]',
                      'type': 'regular',
                      'created_at': '2024-01-10T00:00:00Z',
                    },
                  ],
                  'next_cursor': 'older-pins',
                  'has_more': true,
                }
              : {'messages': pinned},
        }),
      );
    }
    if (request.url.path == '/api/v1/chats/chat-pins/pinned-messages') {
      pinnedRequestCount++;
      onPinnedRequest?.call();
      if (failFirstPinnedRequest && pinnedRequestCount == 1) {
        return http.Response('{}', 503);
      }
      return utf8JsonResponse(
        jsonEncode({
          'message_list': {'messages': pinned},
        }),
      );
    }
    if (request.url.path == '/api/v1/messages/read') {
      return http.Response('{}', 200);
    }
    return http.Response('{}', 404);
  });

  final room = Scaffold(
    body: standaloneInfo
        ? const ChatInfoPanel(chatId: 'chat-pins')
        : const ChatRoomPanel(chatId: 'chat-pins'),
  );

  final overrides = [
    ...voiceThemeTestOverrides(),
    profileAccentStorageProvider.overrideWithValue(testProfileAccentStorage),
    authSessionStorageProvider.overrideWithValue(InMemoryAuthSessionStorage()),
    authControllerProvider.overrideWith(authenticatedAuthController),
    voiceChatsClientProvider.overrideWithValue(
      FakeVoiceChatsClient(
        pages: [
          ChatListData(
            items: [
              ChatListItem(
                chat: const VoiceChat(
                  id: 'chat-pins',
                  type: 'CHAT_TYPE_DM',
                  creatorProfileId: 'profile-a',
                ),
                dmPeerProfileId: 'profile-b',
                dmPeerDisplayName: 'Pinned chat partner',
              ),
            ],
          ),
        ],
      ),
    ),
    gatewayConfigProvider.overrideWithValue(
      const GatewayConfig(baseUrl: 'http://api.test'),
    ),
    httpClientProvider.overrideWithValue(client),
    realtimeHubProvider.overrideWith((ref) {
      final hub = _NoopRealtimeHub(ref);
      onRealtimeHubCreated?.call(hub);
      return hub;
    }),
    selectedChatIdProvider.overrideWith((ref) => 'chat-pins'),
  ];
  final app = MaterialApp(
    theme: voiceTestTheme().copyWith(
      textTheme: voiceTestTheme().textTheme.apply(fontFamily: 'Noto Sans'),
    ),
    debugShowCheckedModeBanner: false,
    locale: const Locale('en'),
    localizationsDelegates: AppLocalizations.localizationsDelegates,
    supportedLocales: AppLocalizations.supportedLocales,
    builder: (context, child) => MediaQuery(
      data: MediaQuery.of(
        context,
      ).copyWith(textScaler: TextScaler.linear(textScale)),
      child: child!,
    ),
    home: room,
  );
  return ProviderScope(
    overrides: overrides,
    child: captureBoundary
        ? RepaintBoundary(key: _captureBoundaryKey, child: app)
        : app,
  );
}

bool _primaryFocusWithin(WidgetTester tester, Finder ancestor) {
  final primaryFocus = tester.binding.focusManager.primaryFocus?.context;
  if (primaryFocus is! Element) return false;
  final target = tester.element(ancestor);
  if (identical(primaryFocus, target)) return true;
  var found = false;
  primaryFocus.visitAncestorElements((element) {
    if (identical(element, target)) {
      found = true;
      return false;
    }
    return true;
  });
  return found;
}

const _captureBoundaryKey = Key('pinned_navigation_capture');

Future<void> _setViewport(WidgetTester tester, Size size) async {
  tester.view.physicalSize = size;
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);
}

Future<void> _loadCaptureFonts() async {
  final materialIcons = FontLoader('MaterialIcons')
    ..addFont(rootBundle.load('fonts/MaterialIcons-Regular.otf'));
  await materialIcons.load();
  final notoSans = FontLoader('Noto Sans')
    ..addFont(rootBundle.load('assets/fonts/NotoSans-Regular.ttf'))
    ..addFont(rootBundle.load('assets/fonts/NotoSans-Medium.ttf'))
    ..addFont(rootBundle.load('assets/fonts/NotoSans-SemiBold.ttf'))
    ..addFont(rootBundle.load('assets/fonts/NotoSans-Bold.ttf'));
  await notoSans.load();
}

Future<void> _writeCapture(
  WidgetTester tester,
  String captureDir,
  String filename,
) async {
  final boundary = tester.renderObject<RenderRepaintBoundary>(
    find.byKey(_captureBoundaryKey),
  );
  final captured = await tester.runAsync(() async {
    final image = await boundary.toImage(pixelRatio: 1);
    try {
      final png = await image.toByteData(format: ui.ImageByteFormat.png);
      if (png == null) throw StateError('Flutter did not encode the capture');
      final directory = Directory(captureDir)..createSync(recursive: true);
      await File(
        '${directory.path}${Platform.pathSeparator}$filename.png',
      ).writeAsBytes(png.buffer.asUint8List(), flush: true);
      return true;
    } finally {
      image.dispose();
    }
  });
  if (captured != true) throw StateError('Flutter capture did not complete');
}

VoiceMessage _message(int index) => VoiceMessage(
  id: 'pin-$index',
  chatId: 'chat-pins',
  senderProfileId: 'profile-b',
  content: 'Pinned announcement $index',
);

class _NoopRealtimeHub extends RealtimeHub {
  _NoopRealtimeHub(super.ref);

  @override
  Future<void> ensureConnected() async {}

  @override
  Future<void> disconnect() async {}

  @override
  void ensureSubscribed(String chatId) {}

  @override
  void typingStart(String chatId) {}

  @override
  void typingStop(String chatId) {}

  @override
  Future<void> markRead(String chatId, String messageId) async {}

  @override
  Future<void> deliveryAck({
    required String chatId,
    required String messageId,
    required String senderProfileId,
  }) async {}
}
