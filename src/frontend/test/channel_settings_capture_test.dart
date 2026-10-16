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
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/state/shell_providers.dart';
import 'package:voice_frontend/theme/voice_theme.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/theme/voice_token_catalog.dart';
import 'package:voice_frontend/ui/chat/channel_settings_panel.dart';
import 'package:voice_frontend/ui/chat/chat_info_panel.dart';
import 'package:voice_frontend/ui/shell/side_panel.dart';

import 'support/auth_test_overrides.dart';
import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

const _captureEnv = 'VOICE_CHANNEL_SETTINGS_CAPTURE_DIR';
const _captureRootKey = ValueKey('channel_settings_capture_root');
const _chatId = 'channel-settings-capture';

var _captureFontsLoaded = false;

Future<void> _loadCaptureFonts(WidgetTester tester) async {
  if (_captureFontsLoaded) return;
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
    final bytes = await iconFont.readAsBytes();
    final iconData = ByteData.sublistView(Uint8List.fromList(bytes));
    await (FontLoader(
      'MaterialIcons',
    )..addFont(Future<ByteData>.value(iconData))).load();
    return true;
  });
  if (loaded != true) {
    throw StateError('Capture font loading did not finish');
  }
  _captureFontsLoaded = true;
}

Future<ThemeData> _themeForCapture(WidgetTester tester, bool capture) async {
  if (!capture) return voiceTestTheme();
  await _loadCaptureFonts(tester);
  final theme = await tester.runAsync(() async {
    final catalog = await VoiceTokenCatalog.load();
    return VoiceTheme.build(
      catalog: catalog,
      mode: VoiceThemeMode.dark,
      profileAccent: catalog.profileAccentAt(0),
    );
  });
  if (theme == null) {
    throw StateError('Voice theme asset loading did not finish');
  }
  return theme;
}

MockClient _ownerChannelClient() => MockClient((request) async {
  if (request.url.path == '/api/v1/chats') {
    return http.Response(
      '{"chat_list":{"items":[{"chat":{"id":"$_chatId","type":"CHAT_TYPE_CHANNEL","creator_profile_id":"prof-test","threads_enabled":false,"allow_user_main_feed":false}}]}}',
      200,
    );
  }
  if (request.url.path == '/api/v1/chats/$_chatId/members') {
    return http.Response(
      '{"member_list":{"members":[{"profile_id":"prof-test","role":"owner"}]}}',
      200,
    );
  }
  if (request.url.path.contains('/shared-media')) {
    return http.Response('{"shared_media_list":{"items":[]}}', 200);
  }
  return http.Response('{}', 404);
});

ProviderContainer _container(MockClient client, {bool wide = false}) {
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
      httpClientProvider.overrideWithValue(client),
    ],
  );
  if (wide) {
    container.read(selectedChatIdProvider.notifier).state = _chatId;
    container.read(shellSidePanelProvider.notifier).state =
        ShellSidePanel.chatInfo;
  }
  return container;
}

Widget _app({
  required ProviderContainer container,
  required ThemeData theme,
  required Widget body,
}) => RepaintBoundary(
  key: _captureRootKey,
  child: UncontrolledProviderScope(
    container: container,
    child: MaterialApp(
      debugShowCheckedModeBanner: false,
      theme: theme,
      locale: const Locale('en'),
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      home: body,
    ),
  ),
);

Future<void> _capture(
  WidgetTester tester,
  String? directory,
  String filename,
) async {
  if (directory == null || directory.isEmpty) {
    return;
  }
  final boundary = tester.renderObject<RenderRepaintBoundary>(
    find.byKey(_captureRootKey),
  );
  final bytes = await tester.runAsync(() async {
    final image = await boundary.toImage(pixelRatio: 1);
    try {
      final data = await image.toByteData(format: ui.ImageByteFormat.png);
      if (data == null) {
        throw StateError('PNG encoding returned no bytes');
      }
      final output = File('$directory${Platform.pathSeparator}$filename');
      await output.parent.create(recursive: true);
      final png = data.buffer.asUint8List();
      await output.writeAsBytes(png, flush: true);
      return png;
    } finally {
      image.dispose();
    }
  });
  if (bytes == null || bytes.isEmpty) {
    throw StateError('Capture did not finish');
  }
}

void main() {
  testWidgets('channel settings wide and narrow production captures', (
    tester,
  ) async {
    final captureDir = Platform.environment[_captureEnv];
    final capturing = captureDir != null && captureDir.isNotEmpty;
    final theme = await _themeForCapture(tester, capturing);
    final sizes = [
      (
        size: const Size(1280, 800),
        wide: true,
        file: 'channel-settings-wide.png',
      ),
      (
        size: const Size(390, 844),
        wide: false,
        file: 'channel-settings-narrow.png',
      ),
    ];

    for (final scenario in sizes) {
      tester.view.physicalSize = scenario.size;
      tester.view.devicePixelRatio = 1;
      final container = _container(_ownerChannelClient(), wide: scenario.wide);
      addTearDown(container.dispose);
      final body = scenario.wide
          ? Scaffold(
              body: Row(
                children: [
                  const Expanded(child: ColoredBox(color: Color(0xFF17191F))),
                  SizedBox(
                    width: 360,
                    child: SidePanelHost(
                      key: const Key('channel_settings_side_host'),
                    ),
                  ),
                ],
              ),
            )
          : Scaffold(body: ChatInfoPanel(chatId: _chatId));
      await tester.pumpWidget(
        _app(container: container, theme: theme, body: body),
      );
      await tester.pumpAndSettle();
      final entry = find.byKey(StandaloneChannelSettingsEntry.entryKey);
      expect(entry, findsOneWidget);
      await tester.ensureVisible(entry);
      await tester.tap(entry);
      await tester.pumpAndSettle();
      expect(find.byKey(ChannelSettingsPanel.panelKey), findsOneWidget);
      expect(find.byKey(ChannelSettingsPanel.threadsToggleKey), findsOneWidget);
      expect(
        find.byKey(ChannelSettingsPanel.memberPostsToggleKey),
        findsOneWidget,
      );
      if (scenario.wide) {
        expect(
          find.byKey(const Key('channel_settings_side_host')),
          findsOneWidget,
        );
        expect(find.byType(BottomSheet), findsNothing);
      } else {
        expect(find.byType(BottomSheet), findsOneWidget);
      }
      expect(tester.takeException(), isNull);
      await _capture(tester, captureDir, scenario.file);
      await tester.pumpWidget(const SizedBox.shrink());
    }
    tester.view.resetPhysicalSize();
    tester.view.resetDevicePixelRatio();
  });
}
