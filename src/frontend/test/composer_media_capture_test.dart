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
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/theme/voice_emoji_style.dart';
import 'package:voice_frontend/theme/voice_theme.dart';
import 'package:voice_frontend/theme/voice_token_catalog.dart';
import 'package:voice_frontend/ui/shell/side_panel.dart';

import 'support/auth_test_overrides.dart';
import 'support/voice_test_theme.dart';

void main() {
  testWidgets('real catalog unavailable and installed-pack states fit H and V', (
    tester,
  ) async {
    final directory = Platform.environment['VOICE_MEDIA_CAPTURE_DIR'];
    var theme = voiceTestTheme();
    if (directory != null && directory.isNotEmpty) {
      theme = (await tester.runAsync(() async {
        await (FontLoader(VoiceTheme.fontFamily)
              ..addFont(rootBundle.load('assets/fonts/NotoSans-Regular.ttf'))
              ..addFont(rootBundle.load('assets/fonts/NotoSans-SemiBold.ttf')))
            .load();
        await (FontLoader(
          VoiceEmojiStyle.fontFamily,
        )..addFont(rootBundle.load('assets/fonts/NotoColorEmoji.ttf'))).load();
        final flutterRoot = Platform.environment['FLUTTER_ROOT'];
        if (flutterRoot == null) {
          throw StateError('FLUTTER_ROOT required for capture fonts');
        }
        final icons = await File(
          '$flutterRoot/bin/cache/artifacts/material_fonts/MaterialIcons-Regular.otf',
        ).readAsBytes();
        await (FontLoader(
          'MaterialIcons',
        )..addFont(Future.value(ByteData.sublistView(icons)))).load();
        final catalog = await VoiceTokenCatalog.load();
        return VoiceTheme.build(
          catalog: catalog,
          mode: VoiceThemeMode.dark,
          profileAccent: catalog.profileAccentAt(0),
        );
      }))!;
    }
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    const root = Key('media_capture_root');
    for (final wide in [true, false]) {
      final trigger = GlobalKey();
      tester.view.physicalSize = wide
          ? const Size(1280, 800)
          : const Size(390, 844);
      tester.view.devicePixelRatio = 1;
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            authSessionStorageProvider.overrideWithValue(
              InMemoryAuthSessionStorage(),
            ),
            authControllerProvider.overrideWith(authenticatedAuthController),
            gatewayConfigProvider.overrideWithValue(
              const GatewayConfig(baseUrl: 'http://api.test'),
            ),
            httpClientProvider.overrideWithValue(
              MockClient((request) async {
                if (request.url.path == '/api/v1/sticker-packs') {
                  return http.Response(
                    jsonEncode({
                      'packs': [
                        {
                          'id': 'system',
                          'title': 'System',
                          'is_system': true,
                          'stickers': [
                            {
                              'id': 'wave',
                              'file_id': 'unresolved-catalog-file',
                              'emoji': '👋',
                            },
                            {
                              'id': 'party',
                              'file_id': 'unresolved-catalog-file-2',
                              'emoji': '🎉',
                            },
                          ],
                        },
                      ],
                    }),
                    200,
                    headers: {
                      'content-type': 'application/json; charset=utf-8',
                    },
                  );
                }
                return http.Response(
                  '{"error_code":"unavailable","message":"GIF search unavailable"}',
                  503,
                );
              }),
            ),
          ],
          child: RepaintBoundary(
            key: root,
            child: MaterialApp(
              theme: theme,
              debugShowCheckedModeBanner: false,
              locale: const Locale('en'),
              localizationsDelegates: AppLocalizations.localizationsDelegates,
              supportedLocales: AppLocalizations.supportedLocales,
              home: Consumer(
                builder: (context, ref, _) => Scaffold(
                  body: Align(
                    alignment: Alignment.bottomRight,
                    child: TextButton(
                      key: trigger,
                      onPressed: () => openEmojiPanel(
                        context,
                        ref,
                        onSelected: (_) {},
                        anchorContext: trigger.currentContext,
                      ),
                      child: const Text('open'),
                    ),
                  ),
                ),
              ),
            ),
          ),
        ),
      );
      await tester.tap(find.text('open'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Stickers'));
      await tester.pumpAndSettle();
      expect(find.byKey(const Key('sticker_result_wave')), findsOneWidget);
      expect(find.text('Sending is currently unavailable'), findsOneWidget);
      expect(tester.takeException(), isNull);
      await _capture(
        tester,
        directory,
        root,
        'media-stickers-${wide ? 'h' : 'v'}',
      );
      await tester.tap(find.text('GIFs'));
      await tester.pumpAndSettle();
      expect(find.text('GIF search unavailable'), findsOneWidget);
      expect(find.text('Retry'), findsOneWidget);
      expect(tester.takeException(), isNull);
      await _capture(
        tester,
        directory,
        root,
        'media-gifs-unavailable-${wide ? 'h' : 'v'}',
      );
      await tester.pumpWidget(const SizedBox.shrink());
      await tester.pumpAndSettle();
    }
  });
}

Future<void> _capture(
  WidgetTester tester,
  String? directory,
  Key root,
  String name,
) async {
  if (directory == null || directory.isEmpty) return;
  final boundary = tester.renderObject<RenderRepaintBoundary>(find.byKey(root));
  await tester.runAsync(() async {
    final image = await boundary.toImage(pixelRatio: 1);
    try {
      final data = await image.toByteData(format: ui.ImageByteFormat.png);
      final file = File('$directory/$name.png');
      await file.parent.create(recursive: true);
      await file.writeAsBytes(data!.buffer.asUint8List(), flush: true);
    } finally {
      image.dispose();
    }
  });
}
