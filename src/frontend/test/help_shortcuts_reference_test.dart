import 'dart:io';
import 'dart:ui' as ui;

import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/settings/voice_input_settings.dart';
import 'package:voice_frontend/theme/voice_theme.dart';
import 'package:voice_frontend/theme/voice_token_catalog.dart';
import 'package:voice_frontend/ui/settings/help_sheet.dart';
import 'package:voice_frontend/ui/settings/settings_sheet.dart';

import 'support/auth_test_overrides.dart';
import 'support/voice_test_theme.dart';

const _captureRootKey = ValueKey('help_shortcut_capture_root');
var _captureFontsLoaded = false;

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  setUp(() {
    SharedPreferences.setMockInitialValues({});
  });

  testWidgets('Help exposes shortcuts with accessible labels and closes', (
    tester,
  ) async {
    tester.view.physicalSize = const Size(1280, 800);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    final theme = await _captureTheme(tester);
    final openHelpFocus = FocusNode();
    addTearDown(openHelpFocus.dispose);
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          voiceInputSettingsProvider.overrideWith(
            _PttVoiceInputSettingsNotifier.new,
          ),
        ],
        child: RepaintBoundary(
          key: _captureRootKey,
          child: MaterialApp(
            debugShowCheckedModeBanner: false,
            theme: theme,
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: Builder(
              builder: (context) => Scaffold(
                body: TextButton(
                  focusNode: openHelpFocus,
                  onPressed: () => HelpSheet.show(context),
                  child: const Text('Open help'),
                ),
              ),
            ),
          ),
        ),
      ),
    );

    openHelpFocus.requestFocus();
    await tester.pump();
    await tester.tap(find.text('Open help'));
    await tester.pumpAndSettle();
    await _captureIfRequested(tester, 'help-shortcuts-desktop.png');

    expect(find.text('Keyboard shortcuts'), findsOneWidget);
    expect(find.text('Ctrl+K'), findsOneWidget);
    expect(find.text('Ctrl+,'), findsOneWidget);
    expect(find.text('Alt+Up / Alt+Down'), findsOneWidget);
    expect(find.text('Escape'), findsOneWidget);
    expect(find.text('Up / Down'), findsOneWidget);
    expect(find.text('Enter'), findsOneWidget);
    expect(find.text('R'), findsOneWidget);
    expect(find.text('E'), findsOneWidget);
    expect(find.text('`'), findsOneWidget);
    expect(find.bySemanticsLabel('Ctrl+K, Open search'), findsOneWidget);
    expect(
      find.bySemanticsLabel('`, Hold to talk when push-to-talk is enabled'),
      findsOneWidget,
    );
    expect(find.byTooltip('Close help'), findsOneWidget);
    expect(find.byIcon(Icons.close), findsOneWidget);
    expect(find.text('Help'), findsOneWidget);
    expect(find.text('How can we help?'), findsOneWidget);
    expect(tester.getSize(find.byTooltip('Close help')), const Size(34, 34));
    expect(
      tester.getTopLeft(find.byTooltip('Close help')).dx,
      lessThan(tester.getTopLeft(find.text('Help')).dx),
    );

    await tester.tap(find.byTooltip('Close help'));
    await tester.pumpAndSettle();
    expect(find.text('Keyboard shortcuts'), findsNothing);
    expect(FocusManager.instance.primaryFocus, same(openHelpFocus));

    await tester.tap(find.text('Open help'));
    await tester.pumpAndSettle();
    expect(find.text('Keyboard shortcuts'), findsOneWidget);
    await tester.binding.handlePopRoute();
    await tester.pumpAndSettle();
    expect(find.text('Keyboard shortcuts'), findsNothing);
    expect(FocusManager.instance.primaryFocus, same(openHelpFocus));
  });

  testWidgets('Help reference fits a narrow viewport without overflow', (
    tester,
  ) async {
    tester.view.physicalSize = const Size(390, 844);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    final theme = await _captureTheme(tester);
    final openHelpFocus = FocusNode();
    addTearDown(openHelpFocus.dispose);
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          voiceInputSettingsProvider.overrideWith(
            _PttVoiceInputSettingsNotifier.new,
          ),
        ],
        child: RepaintBoundary(
          key: _captureRootKey,
          child: MaterialApp(
            debugShowCheckedModeBanner: false,
            theme: theme,
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: Builder(
              builder: (context) => Scaffold(
                body: TextButton(
                  focusNode: openHelpFocus,
                  onPressed: () => HelpSheet.show(context),
                  child: const Text('Open help'),
                ),
              ),
            ),
          ),
        ),
      ),
    );

    openHelpFocus.requestFocus();
    await tester.pump();
    await tester.tap(find.text('Open help'));
    await tester.pumpAndSettle();
    await _captureIfRequested(tester, 'help-shortcuts-mobile.png');

    expect(tester.takeException(), isNull);
    expect(find.text('Help'), findsOneWidget);
    expect(find.byIcon(Icons.help_outline), findsOneWidget);
    expect(find.byTooltip('Back'), findsOneWidget);
    expect(find.byIcon(Icons.arrow_back), findsOneWidget);
    expect(tester.getSize(find.byTooltip('Back')), const Size(36, 36));
    expect(
      tester.getTopLeft(find.byTooltip('Back')).dx,
      lessThan(tester.getTopLeft(find.text('Help')).dx),
    );
    expect(find.text('How can we help?'), findsOneWidget);
    expect(
      find.text('Find an answer or contact the Voice team.'),
      findsOneWidget,
    );
    expect(find.text('Keyboard shortcuts'), findsNothing);
    expect(find.text('Ctrl+K'), findsNothing);

    await tester.tap(find.byTooltip('Back'));
    await tester.pumpAndSettle();
    expect(find.text('How can we help?'), findsNothing);
    expect(FocusManager.instance.primaryFocus, same(openHelpFocus));
  });

  testWidgets(
    'Help from Settings closes with Escape to the live settings trigger',
    (tester) async {
      final container = ProviderContainer(
        overrides: voiceAppTestOverrides(
          client: MockClient((_) async => http.Response('{}', 200)),
        ),
      );
      addTearDown(container.dispose);
      final settingsFocus = FocusNode();
      addTearDown(settingsFocus.dispose);

      await tester.pumpWidget(
        UncontrolledProviderScope(
          container: container,
          child: MaterialApp(
            theme: voiceTestTheme(),
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: Builder(
              builder: (context) => Scaffold(
                body: TextButton(
                  focusNode: settingsFocus,
                  onPressed: () => showModalBottomSheet<void>(
                    context: context,
                    isScrollControlled: true,
                    builder: (_) => const SettingsSheet(),
                  ),
                  child: const Text('Open settings'),
                ),
              ),
            ),
          ),
        ),
      );

      settingsFocus.requestFocus();
      await tester.pump();
      await tester.tap(find.text('Open settings'));
      await tester.pumpAndSettle();
      await tester.ensureVisible(find.byKey(const Key('settings_help')));
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const Key('settings_help')));
      await tester.pumpAndSettle();

      expect(find.text('Keyboard shortcuts'), findsOneWidget);
      await tester.sendKeyEvent(LogicalKeyboardKey.escape);
      await tester.pumpAndSettle();

      expect(find.text('Keyboard shortcuts'), findsNothing);
      expect(FocusManager.instance.primaryFocus, same(settingsFocus));
    },
  );
}

Future<ThemeData?> _captureTheme(WidgetTester tester) async {
  if ((Platform.environment['VOICE_HELP_CAPTURE_DIR'] ?? '').isEmpty) {
    return null;
  }
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
      throw StateError('Production font loading did not complete');
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
    throw StateError('Voice design tokens did not load for capture');
  }
  return theme;
}

Future<void> _captureIfRequested(WidgetTester tester, String filename) async {
  final directory = Platform.environment['VOICE_HELP_CAPTURE_DIR'];
  if (directory == null || directory.isEmpty) return;

  final boundary = tester.renderObject<RenderRepaintBoundary>(
    find.byKey(_captureRootKey),
  );
  final captured = await tester.runAsync(() async {
    final image = await boundary.toImage(pixelRatio: 1);
    try {
      final data = await image.toByteData(format: ui.ImageByteFormat.png);
      if (data == null) throw StateError('PNG encoding returned null');
      final output = File('$directory${Platform.pathSeparator}$filename');
      await output.parent.create(recursive: true);
      await output.writeAsBytes(data.buffer.asUint8List(), flush: true);
      return data.buffer.asUint8List();
    } finally {
      image.dispose();
    }
  });
  if (captured == null || captured.isEmpty) {
    throw StateError('Help screenshot capture did not complete');
  }
}

class _PttVoiceInputSettingsNotifier extends VoiceInputSettingsNotifier {
  @override
  VoiceInputSettings build() => const VoiceInputSettings(
    mode: VoiceInputMode.ptt,
    pttKey: LogicalKeyboardKey.backquote,
  );
}
