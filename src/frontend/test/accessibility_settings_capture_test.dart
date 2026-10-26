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
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/theme/voice_theme.dart';
import 'package:voice_frontend/theme/voice_token_catalog.dart';
import 'package:voice_frontend/ui/core/voice_bottom_sheet.dart';
import 'package:voice_frontend/ui/settings/accessibility_settings_screen.dart';
import 'package:voice_frontend/ui/settings/settings_sheet.dart';

import 'support/auth_test_overrides.dart';

const _captureEnabled = bool.fromEnvironment('VOICE_ACCESSIBILITY_CAPTURE');
const _captureBoundaryKey = Key('accessibility_capture_boundary');
var _captureFontsLoaded = false;

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  setUp(() => SharedPreferences.setMockInitialValues({}));

  testWidgets('capture H/V accessibility settings screenshots', (tester) async {
    if (!_captureEnabled) return;

    final container = ProviderContainer(
      overrides: voiceAppTestOverrides(
        client: MockClient((_) async => http.Response('{}', 200)),
      ),
    );
    addTearDown(container.dispose);
    addTearDown(() => tester.binding.setSurfaceSize(null));

    final output = Directory('../../tmp/anton122-accessibility');
    final directoryCreated = await tester.runAsync(() async {
      await output.create(recursive: true);
      return true;
    });
    if (directoryCreated != true) {
      throw StateError('Capture directory unavailable');
    }
    await _loadCaptureFonts(tester);
    final catalog = await tester.runAsync(VoiceTokenCatalog.load);
    if (catalog == null) throw StateError('Design tokens did not load');
    final theme = await VoiceTheme.build(
      catalog: catalog,
      mode: VoiceThemeMode.dark,
      profileAccent: catalog.profileAccentAt(0),
    );

    for (final viewport in const [
      (name: 'H', size: Size(1280, 800)),
      (name: 'V', size: Size(390, 844)),
    ]) {
      await tester.binding.setSurfaceSize(viewport.size);
      await tester.pumpWidget(
        UncontrolledProviderScope(
          container: container,
          child: MaterialApp(
            theme: theme,
            locale: const Locale('en'),
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            builder: (context, child) => RepaintBoundary(
              key: _captureBoundaryKey,
              child: MediaQuery(
                data: MediaQuery.of(context).copyWith(
                  size: viewport.size,
                  textScaler: const TextScaler.linear(1.5),
                ),
                child: child!,
              ),
            ),
            home: Builder(
              builder: (context) => Scaffold(
                body: Center(
                  child: TextButton(
                    key: const Key('open_settings'),
                    onPressed: () => showVoiceBottomSheet<void>(
                      context: context,
                      child: const SettingsSheet(),
                    ),
                    child: const Text('Open settings'),
                  ),
                ),
              ),
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const Key('open_settings')));
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(SettingsSheet.accessibilityKey));
      await tester.pumpAndSettle();
      expect(find.byKey(AccessibilitySettingsScreen.screenKey), findsOneWidget);

      final boundary = tester.renderObject<RenderRepaintBoundary>(
        find.byKey(_captureBoundaryKey),
      );
      final written = await tester.runAsync(() async {
        final image = await boundary.toImage(pixelRatio: 1);
        try {
          if (image.width != viewport.size.width.toInt() ||
              image.height != viewport.size.height.toInt()) {
            throw StateError('Capture did not match requested viewport');
          }
          final png = await image.toByteData(format: ui.ImageByteFormat.png);
          if (png == null) throw StateError('PNG encoding returned null');
          await File(
            '${output.path}/accessibility-${viewport.name}.png',
          ).writeAsBytes(png.buffer.asUint8List(), flush: true);
          return true;
        } finally {
          image.dispose();
        }
      });
      if (written != true) throw StateError('Capture did not complete');

      await tester.pumpWidget(const SizedBox.shrink());
    }
  }, skip: !_captureEnabled);
}

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
    final iconPath = [
      flutterRoot,
      'bin',
      'cache',
      'artifacts',
      'material_fonts',
      'MaterialIcons-Regular.otf',
    ].join(Platform.pathSeparator);
    final iconBytes = await File(iconPath).readAsBytes();
    final iconData = ByteData.sublistView(Uint8List.fromList(iconBytes));
    await (FontLoader(
      'MaterialIcons',
    )..addFont(Future<ByteData>.value(iconData))).load();
    return true;
  });
  if (loaded != true) throw StateError('Capture fonts did not load');
  _captureFontsLoaded = true;
}
