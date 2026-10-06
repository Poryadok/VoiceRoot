import 'dart:io';
import 'dart:ui' as ui;

import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/app.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/messages_client.dart';
import 'package:voice_frontend/backend/realtime_client.dart';
import 'package:voice_frontend/routing/deep_link_listener.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/connectivity_providers.dart';
import 'package:voice_frontend/theme/voice_layout.dart';
import 'package:voice_frontend/theme/voice_theme.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/theme/voice_token_catalog.dart';
import 'package:voice_frontend/ui/chat/chat_room_panel.dart';
import 'package:voice_frontend/ui/shell/chat_list_body.dart';

import 'support/auth_test_overrides.dart';
import 'support/fake_voice_api_clients.dart';

const _captureDirectoryVariable = 'VOICE_NETWORK_OFFLINE_CAPTURE_DIR';
const _captureBoundaryKey = Key('network_offline_app_capture');
const _captureChatId = 'network-offline-capture-chat';

void main() {
  testWidgets('keeps one reconnect banner in the visible network context', (
    tester,
  ) async {
    addTearDown(() {
      tester.view.resetPhysicalSize();
      tester.view.resetDevicePixelRatio();
    });
    final captureDirectory = Platform.environment[_captureDirectoryVariable]
        ?.trim();
    final shouldCapture = captureDirectory?.isNotEmpty ?? false;

    if (shouldCapture) {
      await _loadProductionFonts(tester);
    }
    final catalog = await VoiceTokenCatalog.load();

    for (final viewport in [
      (
        name: 'h',
        size: const Size(1280, 800),
        selectedChat: false,
        themeMode: VoiceThemeMode.dark,
      ),
      (
        name: 'v',
        size: const Size(390, 844),
        selectedChat: true,
        themeMode: VoiceThemeMode.dark,
      ),
      (
        name: 'light-h',
        size: const Size(1280, 800),
        selectedChat: true,
        themeMode: VoiceThemeMode.light,
      ),
      (
        name: 'light-v',
        size: const Size(390, 844),
        selectedChat: false,
        themeMode: VoiceThemeMode.light,
      ),
    ]) {
      final theme = await VoiceTheme.build(
        catalog: catalog,
        mode: viewport.themeMode,
        profileAccent: catalog.profileAccentAt(0),
      );
      tester.view.devicePixelRatio = 1;
      tester.view.physicalSize = viewport.size;
      final container = ProviderContainer(
        overrides: [
          ...voiceAppTestOverrides(
            client: MockClient((_) async => httpResponseOk),
          ),
          voiceMaterialThemeProvider.overrideWith((ref) async => theme),
          deepLinkListenerProvider.overrideWith(
            _CaptureNoopDeepLinkListener.new,
          ),
          if (viewport.selectedChat)
            voiceChatsClientProvider.overrideWithValue(
              FakeVoiceChatsClient(
                pages: [
                  const ChatListData(
                    items: [
                      ChatListItem(
                        chat: VoiceChat(
                          id: _captureChatId,
                          type: 'CHAT_TYPE_GROUP',
                          creatorProfileId: 'capture-owner',
                          name: 'Offline capture room',
                        ),
                      ),
                    ],
                  ),
                  const ChatListData(
                    items: [
                      ChatListItem(
                        chat: VoiceChat(
                          id: _captureChatId,
                          type: 'CHAT_TYPE_GROUP',
                          creatorProfileId: 'capture-owner',
                          name: 'Offline capture room',
                        ),
                      ),
                    ],
                  ),
                ],
              ),
            ),
          if (viewport.selectedChat)
            voiceMessagesClientProvider.overrideWithValue(
              _CaptureVoiceMessagesClient(),
            ),
          connectivityWatcherProvider.overrideWith((ref) {}),
          realtimeHubProvider.overrideWith((ref) => _CaptureRealtimeHub(ref)),
        ],
      );
      addTearDown(container.dispose);
      container.read(realtimeLinkStatusProvider.notifier).state =
          RealtimeLinkStatus.connected;
      if (viewport.selectedChat) {
        container.read(selectedChatIdProvider.notifier).state = _captureChatId;
      }

      await tester.pumpWidget(
        UncontrolledProviderScope(
          container: container,
          child: RepaintBoundary(
            key: _captureBoundaryKey,
            child: MediaQuery(
              data: MediaQueryData(
                textScaler: const TextScaler.linear(1.5),
                size: viewport.size,
                devicePixelRatio: 1,
              ),
              child: const VoiceApp(locale: Locale('en')),
            ),
          ),
        ),
      );
      await tester.pump();
      expect(
        tester.takeException(),
        isNull,
        reason: 'selected-room fixture must lay out before reconnect begins',
      );
      container.read(realtimeLinkStatusProvider.notifier).state =
          RealtimeLinkStatus.reconnecting;
      await tester.pump(reconnectBannerShowDelay);
      expect(
        tester.takeException(),
        isNull,
        reason:
            'reconnect state must lay out without overflowing (${viewport.name})',
      );

      final navigationOwnsBanner =
          viewport.size.width > VoiceLayout.narrowBreakpoint ||
          !viewport.selectedChat;
      const globalBannerKey = Key('global_reconnect_banner');
      final roomBannerKey = ChatRoomPanel.reconnectBannerKey;
      final bannerKey = navigationOwnsBanner ? globalBannerKey : roomBannerKey;
      expect(
        find.byKey(globalBannerKey).evaluate().length,
        navigationOwnsBanner ? 1 : 0,
      );
      expect(
        find.byKey(roomBannerKey).evaluate().length,
        navigationOwnsBanner ? 0 : 1,
      );
      expect(
        find.byKey(bannerKey),
        findsOneWidget,
        reason: 'the actual mounted network context owns the single banner',
      );
      if (navigationOwnsBanner) {
        expect(
          find.descendant(
            of: find.byType(ChatListBody),
            matching: find.byKey(globalBannerKey),
          ),
          findsOneWidget,
          reason: 'the desktop/mobile list route owns the status slot',
        );
      }
      expect(
        find.descendant(
          of: find.byKey(bannerKey),
          matching: find.text('Reconnecting…'),
        ),
        findsOneWidget,
      );
      expect(
        find.text(
          'Drafts stay on this device. Realtime updates resume after reconnection.',
        ),
        findsOneWidget,
        reason: 'status copy belongs to the active reconnect banner',
      );
      expect(
        find.descendant(
          of: find.byKey(bannerKey),
          matching: find.text('Try again'),
        ),
        findsOneWidget,
      );
      expect(
        find.descendant(
          of: find.byKey(bannerKey),
          matching: find.byTooltip('Close'),
        ),
        findsOneWidget,
      );
      if (viewport.selectedChat) {
        final visibleSurface = Offset.zero & viewport.size;
        final roomContent = [
          find.text('No messages yet'),
          find.text('Send the first message when you are ready.'),
          find.descendant(
            of: find.byKey(bannerKey),
            matching: find.text('Try again'),
          ),
          find.descendant(
            of: find.byKey(bannerKey),
            matching: find.byTooltip('Close'),
          ),
        ];
        expect(roomContent[0], findsOneWidget);
        expect(roomContent[1], findsOneWidget);
        for (final finder in roomContent) {
          final rect = tester.getRect(finder);
          expect(
            rect.left >= visibleSurface.left &&
                rect.top >= visibleSurface.top &&
                rect.right <= visibleSurface.right &&
                rect.bottom <= visibleSurface.bottom,
            isTrue,
            reason: 'room recovery content must remain inside the viewport',
          );
        }
        expect(
          find.byKey(const Key('chat_room_pins_error')),
          findsNothing,
          reason:
              'capture should not include an unrelated pinned-messages error',
        );
      }
      expect(tester.takeException(), isNull);

      if (shouldCapture) {
        await _writeCapture(
          tester,
          '${captureDirectory!}${Platform.pathSeparator}network-offline-${viewport.name}.png',
          expectedSize: viewport.size,
        );
      }
      await tester.pumpWidget(const SizedBox.shrink());
    }
  });
}

final httpResponseOk = http.Response('OK', 200);

Future<void> _loadProductionFonts(WidgetTester tester) async {
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
    final iconFont = File(
      [
        flutterRoot,
        'bin',
        'cache',
        'artifacts',
        'material_fonts',
        'MaterialIcons-Regular.otf',
      ].join(Platform.pathSeparator),
    );
    final iconBytes = await iconFont.readAsBytes();
    final iconData = ByteData.sublistView(iconBytes);
    await (FontLoader(
      'MaterialIcons',
    )..addFont(Future<ByteData>.value(iconData))).load();
    return true;
  });
  if (loaded != true) {
    throw StateError('Production capture fonts did not load');
  }
}

Future<void> _writeCapture(
  WidgetTester tester,
  String path, {
  required Size expectedSize,
}) async {
  final boundary = tester.renderObject<RenderRepaintBoundary>(
    find.byKey(_captureBoundaryKey),
  );
  final written = await tester.runAsync(() async {
    final image = await boundary.toImage(pixelRatio: 1);
    try {
      if (image.width != expectedSize.width.toInt() ||
          image.height != expectedSize.height.toInt()) {
        throw StateError('Capture did not match the requested viewport size');
      }
      final png = await image.toByteData(format: ui.ImageByteFormat.png);
      if (png == null) throw StateError('PNG encoding returned null');
      await File(path).writeAsBytes(png.buffer.asUint8List(), flush: true);
      return true;
    } finally {
      image.dispose();
    }
  });
  if (written != true) {
    throw StateError('Network banner capture did not complete');
  }
}

class _CaptureRealtimeHub extends RealtimeHub {
  _CaptureRealtimeHub(super.ref);

  @override
  Stream<RealtimeFrame> get events => const Stream.empty();

  @override
  bool get canRetryCurrentSession => true;

  @override
  Future<void> ensureConnected() async {}

  @override
  Future<void> retryCurrentSession() async {}

  @override
  void ensureSubscribed(String chatId) {}

  @override
  Future<void> dispose() async {}
}

class _CaptureVoiceMessagesClient extends FakeVoiceMessagesClient {
  @override
  Future<MessagesApiResult<MessageListData>> getPinnedMessages({
    required String authorization,
    required String chatId,
  }) async => const MessagesApiOk(MessageListData(messages: []));
}

class _CaptureNoopDeepLinkListener extends DeepLinkListener {
  _CaptureNoopDeepLinkListener(super.ref);

  @override
  Future<void> start() async {}
}
