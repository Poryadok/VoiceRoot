import 'dart:io';
import 'dart:ui' as ui;

import 'package:flutter/foundation.dart' show DiagnosticableTreeNode;
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
import 'package:voice_frontend/ui/core/voice_compact_banner.dart';
import 'package:voice_frontend/ui/shell/chat_list_body.dart';

import 'support/auth_test_overrides.dart';
import 'support/fake_voice_api_clients.dart';

const _captureDirectoryVariable = 'VOICE_NETWORK_OFFLINE_CAPTURE_DIR';
const _captureBoundaryKey = Key('network_offline_app_capture');
const _captureChatId = 'network-offline-capture-chat';

String _renderFlexFailureEvidence(FlutterErrorDetails? details) {
  if (details == null || details.exception is! FlutterError) {
    return 'layout diagnostic unavailable';
  }
  final summary = (details.exception as FlutterError).toString();
  if (!summary.startsWith('A RenderFlex overflowed by')) {
    return 'layout diagnostic unavailable';
  }
  final diagnostics =
      details.informationCollector?.call() ?? const <DiagnosticsNode>[];
  final renderFlexNode = diagnostics
      .where((node) => node.name == 'The specific RenderFlex in question is')
      .firstOrNull;
  final renderFlexValue = renderFlexNode is DiagnosticableTreeNode
      ? renderFlexNode.value
      : null;
  final renderFlex = renderFlexValue is RenderFlex ? renderFlexValue : null;
  if (renderFlex == null) return 'layout diagnostic unavailable';

  final creatorNode = diagnostics
      .whereType<DiagnosticsDebugCreator>()
      .firstOrNull;
  final creator = creatorNode?.value;
  final owner = creator is DebugCreator
      ? _knownLayoutOwner(creator.element)
      : 'unavailable';
  final flexAxis = renderFlex.direction == Axis.horizontal
      ? 'horizontal'
      : renderFlex.direction == Axis.vertical
      ? 'vertical'
      : 'unavailable';
  final appFrame = RegExp(
    r'(src/frontend/lib/[A-Za-z0-9_./-]+\.dart):(\d+)',
  ).firstMatch(details.stack?.toString() ?? '');
  final location = appFrame == null
      ? 'unavailable'
      : '${appFrame.group(1)}:${appFrame.group(2)}';
  return 'layout overflow; render=RenderFlex; '
      'geometry=${_renderFlexGeometry(renderFlex)}; '
      'flexAxis=$flexAxis; creator=$owner; appFrame=$location';
}

String _knownLayoutOwner(Element element) {
  var owner = 'unavailable';
  final currentOwner = _fixedLayoutOwner(element.widget);
  if (currentOwner != null) return currentOwner;
  element.visitAncestorElements((ancestor) {
    final ancestorOwner = _fixedLayoutOwner(ancestor.widget);
    if (ancestorOwner != null) {
      owner = ancestorOwner;
      return false;
    }
    return true;
  });
  return owner;
}

String? _fixedLayoutOwner(Widget widget) {
  if (widget is VoiceCompactBanner) return 'VoiceCompactBanner';
  if (widget is Row) return 'Row';
  if (widget is Column) return 'Column';
  if (widget is Wrap) return 'Wrap';
  if (widget is ChatRoomPanel) return 'ChatRoomPanel';
  if (widget is ChatListBody) return 'ChatListBody';
  if (widget is VoiceApp) return 'VoiceApp';
  return null;
}

String _renderFlexGeometry(RenderFlex? renderFlex) {
  if (renderFlex == null) return 'unavailable';
  String dimension(double value) => value.isFinite
      ? value.clamp(0.0, 100000.0).toStringAsFixed(1)
      : 'unbounded';
  final size = renderFlex.size;
  final constraints = renderFlex.constraints;
  return 'size=${dimension(size.width)}x${dimension(size.height)}; '
      'max=${dimension(constraints.maxWidth)}x${dimension(constraints.maxHeight)}';
}

void main() {
  testWidgets('keeps one reconnect banner in the visible network context', (
    tester,
  ) async {
    const voipChannel = MethodChannel('voice/voip');
    final messenger = tester.binding.defaultBinaryMessenger;
    var stoppedVoipControllers = 0;
    messenger.setMockMethodCallHandler(voipChannel, (call) async {
      if (call.method != 'stop') {
        throw StateError('Unexpected VoIP method: ${call.method}');
      }
      stoppedVoipControllers += 1;
      return null;
    });
    // Registered before the containers so their LIFO teardown runs with the
    // native stop boundary installed, including its asynchronous completion.
    addTearDown(() async {
      try {
        await Future<void>.delayed(Duration.zero);
        expect(stoppedVoipControllers, 4);
      } finally {
        messenger.setMockMethodCallHandler(voipChannel, null);
      }
    });
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
      String? reconnectOverflowEvidence;
      final previousFlutterErrorHandler = FlutterError.onError;
      if (viewport.name == 'v') {
        if (previousFlutterErrorHandler == null) {
          throw StateError('Flutter test error handler is unavailable');
        }
        FlutterError.onError = (details) {
          try {
            if (reconnectOverflowEvidence == null &&
                details.exception is FlutterError &&
                (details.exception as FlutterError).toString().startsWith(
                  'A RenderFlex overflowed by',
                )) {
              try {
                final evidence = _renderFlexFailureEvidence(details);
                if (evidence != 'layout diagnostic unavailable') {
                  reconnectOverflowEvidence = evidence;
                }
              } catch (_) {
                reconnectOverflowEvidence = 'layout diagnostic unavailable';
              }
            }
          } finally {
            previousFlutterErrorHandler(details);
          }
        };
      }
      try {
        container.read(realtimeLinkStatusProvider.notifier).state =
            RealtimeLinkStatus.reconnecting;
        await tester.pump(reconnectBannerShowDelay);
      } finally {
        if (viewport.name == 'v') {
          FlutterError.onError = previousFlutterErrorHandler;
        }
      }
      expect(
        tester.takeException(),
        isNull,
        reason:
            'reconnect state must lay out without overflowing (${viewport.name}); '
            '${reconnectOverflowEvidence ?? 'layout diagnostic unavailable'}',
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
        final seededRow = find.byKey(ChatListBody.tileKey(_captureChatId));
        expect(
          seededRow,
          findsOneWidget,
          reason: 'the route fixture contains a real visible chat row',
        );
        final bannerTop = tester.getTopLeft(find.byKey(globalBannerKey)).dy;
        final rowTop = tester.getTopLeft(seededRow).dy;
        if (viewport.size.width > VoiceLayout.narrowBreakpoint) {
          expect(
            bannerTop,
            greaterThan(rowTop),
            reason: 'desktop status follows the visible chat-list rows',
          );
        } else {
          expect(
            bannerTop,
            lessThan(rowTop),
            reason: 'phone status precedes the scrollable chat-list rows',
          );
        }
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
      final isPhoneRoute = viewport.size.width <= VoiceLayout.narrowBreakpoint;
      final retryButton = find.descendant(
        of: find.byKey(bannerKey),
        matching: find.byType(OutlinedButton),
      );
      expect(
        retryButton,
        findsOneWidget,
        reason: 'Network retry uses the outlined reference control',
      );
      expect(
        find.descendant(
          of: find.byKey(bannerKey),
          matching: find.byIcon(Icons.refresh),
        ),
        findsOneWidget,
        reason: 'Network retry includes its refresh glyph',
      );
      final retryStyle = tester.widget<OutlinedButton>(retryButton).style!;
      expect(retryStyle.minimumSize!.resolve(const {})!.height, 36);
      expect(
        find.descendant(
          of: find.byKey(bannerKey),
          matching: find.byTooltip('Close'),
        ),
        findsOneWidget,
      );
      final titleText = tester.widget<Text>(
        find.descendant(
          of: find.byKey(bannerKey),
          matching: find.text('Reconnecting…'),
        ),
      );
      expect(titleText.style?.fontSize, 14);
      expect(titleText.style?.fontWeight, FontWeight.w500);
      expect(titleText.style?.height, closeTo(20 / 14, 0.02));
      final detailText = tester.widget<Text>(
        find.descendant(
          of: find.byKey(bannerKey),
          matching: find.text(
            'Drafts stay on this device. Realtime updates resume after reconnection.',
          ),
        ),
      );
      expect(detailText.style?.fontSize, 12);
      expect(detailText.style?.height, closeTo(16 / 12, 0.02));
      if (isPhoneRoute) {
        final progress = find.descendant(
          of: find.byKey(bannerKey),
          matching: find.byKey(const Key('network_reconnecting_progress')),
        );
        expect(progress, findsOneWidget);
        expect(tester.getSize(progress), const Size(22, 22));
      } else {
        final mark = find.descendant(
          of: find.byKey(bannerKey),
          matching: find.byKey(const Key('network_offline_mark')),
        );
        expect(mark, findsOneWidget);
        expect(tester.getSize(mark), const Size(36, 36));
      }
      final tokenColors = catalog.colorsFor(
        viewport.themeMode == VoiceThemeMode.light ? 'light' : 'dark',
      );
      final networkSurface = find.descendant(
        of: find.byKey(bannerKey),
        matching: find.byWidgetPredicate(
          (widget) =>
              widget is Container &&
              widget.decoration is BoxDecoration &&
              (widget.decoration! as BoxDecoration).border != null,
          description: 'Network banner card surface',
        ),
      );
      expect(
        networkSurface,
        findsOneWidget,
        reason: 'Network status uses the bordered named-reference surface',
      );
      final surfaceContainer = tester.widget<Container>(networkSurface);
      final surfaceDecoration = surfaceContainer.decoration! as BoxDecoration;
      expect(
        surfaceContainer.margin,
        isPhoneRoute
            ? const EdgeInsets.fromLTRB(16, 12, 16, 4)
            : const EdgeInsets.fromLTRB(12, 10, 12, 12),
      );
      expect(
        surfaceContainer.padding,
        isPhoneRoute
            ? const EdgeInsets.fromLTRB(12, 10, 8, 10)
            : const EdgeInsets.fromLTRB(12, 10, 10, 10),
      );
      expect(surfaceDecoration.color, tokenColors['color.background.canvas']);
      expect(
        surfaceDecoration.border,
        Border.all(color: tokenColors['color.divider.rail']!, width: 1),
      );
      expect(
        surfaceDecoration.borderRadius,
        BorderRadius.circular(catalog.radius['md']!),
      );
      expect(
        surfaceDecoration.boxShadow,
        isPhoneRoute
            ? [
                BoxShadow(
                  color: Colors.black.withValues(alpha: 0.09),
                  offset: const Offset(0, 5),
                  blurRadius: 16,
                ),
              ]
            : [
                BoxShadow(
                  color: Colors.black.withValues(alpha: 0.12),
                  offset: const Offset(0, 8),
                  blurRadius: 24,
                ),
              ],
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
