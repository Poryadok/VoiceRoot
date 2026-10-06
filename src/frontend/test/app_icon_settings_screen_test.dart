import 'dart:io';
import 'dart:ui' as ui;

import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:voice_frontend/backend/subscription_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/services/windows_desktop_host.dart';
import 'package:voice_frontend/settings/app_icon_preference.dart';
import 'package:voice_frontend/state/subscription_providers.dart';
import 'package:voice_frontend/theme/voice_theme.dart';
import 'package:voice_frontend/theme/voice_token_catalog.dart';
import 'package:voice_frontend/ui/settings/app_icon_settings_screen.dart';
import 'package:voice_frontend/ui/settings/appearance_settings_screen.dart';

import 'support/auth_test_overrides.dart';
import 'support/voice_test_theme.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  setUp(() {
    SharedPreferences.setMockInitialValues({});
    debugDefaultTargetPlatformOverride = TargetPlatform.windows;
  });

  testWidgets(
    'Appearance opens the six-icon picker and Apply persists to Windows',
    (tester) async {
      final host = RecordingWindowsDesktopHost();
      final container = ProviderContainer(
        overrides: [
          ...voiceAppTestOverrides(
            client: MockClient((_) async => http.Response('', 404)),
          ),
          windowsDesktopHostProvider.overrideWithValue(host),
          subscriptionProvider.overrideWith(
            (ref) async => _subscription(status: 'active'),
          ),
        ],
      );
      addTearDown(container.dispose);

      await tester.pumpWidget(_app(container));
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(AppearanceSettingsScreen.appIconKey));
      await tester.pumpAndSettle();

      expect(find.byKey(AppIconSettingsScreen.screenKey), findsOneWidget);
      expect(find.byKey(AppIconSettingsScreen.gridKey), findsOneWidget);
      expect(find.byType(Image), findsNWidgets(6));

      await tester.tap(
        find.byKey(AppIconSettingsScreen.optionKey(AppIconPreference.coral)),
      );
      await tester.pump();
      await tester.tap(find.byKey(AppIconSettingsScreen.applyKey));
      await tester.pumpAndSettle();

      final preferences = await SharedPreferences.getInstance();
      expect(preferences.getString(appIconPreferencePrefKey), 'coral');
      expect(host.lastAppIcon, 'coral');
      expect(tester.takeException(), isNull);

      await tester.tap(find.byKey(const Key('app_icon_back')));
      await tester.pumpAndSettle();
      expect(find.byKey(AppearanceSettingsScreen.screenKey), findsOneWidget);
      debugDefaultTargetPlatformOverride = null;
    },
  );

  testWidgets('a lapsed Plus keeps the saved choice but applies Voice Sky', (
    tester,
  ) async {
    SharedPreferences.setMockInitialValues({appIconPreferencePrefKey: 'mint'});
    final host = RecordingWindowsDesktopHost();
    final container = ProviderContainer(
      overrides: [
        ...voiceAppTestOverrides(
          client: MockClient((_) async => http.Response('', 404)),
        ),
        windowsDesktopHostProvider.overrideWithValue(host),
        subscriptionProvider.overrideWith((ref) async => null),
      ],
    );
    addTearDown(container.dispose);

    await tester.pumpWidget(_app(container));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(AppearanceSettingsScreen.appIconKey));
    await tester.pumpAndSettle();

    expect(
      find.text('Your saved icon will return when Voice Plus is active again.'),
      findsOneWidget,
    );
    expect(
      tester
          .getSemantics(
            find.byKey(AppIconSettingsScreen.optionKey(AppIconPreference.mint)),
          )
          .flagsCollection
          .isSelected
          .toString(),
      'Tristate.isTrue',
    );
    expect(
      tester.widget<FilledButton>(find.byType(FilledButton)).onPressed,
      isNull,
    );
    expect(host.lastAppIcon, 'voice_sky');

    await tester.tap(
      find.byKey(AppIconSettingsScreen.optionKey(AppIconPreference.voiceSky)),
    );
    await tester.pump();
    expect(
      tester.widget<FilledButton>(find.byType(FilledButton)).onPressed,
      isNotNull,
    );
    debugDefaultTargetPlatformOverride = null;
  });

  testWidgets('captures the complete horizontal and vertical reference views', (
    tester,
  ) async {
    final captureTheme = await _loadCaptureTheme(tester);
    final captureDirectory = Directory(
      '${Directory.systemTemp.path}${Platform.pathSeparator}voice-app-icon-captures',
    );
    await tester.runAsync(() => captureDirectory.create(recursive: true));
    for (final viewport in [const Size(1280, 800), const Size(390, 844)]) {
      tester.view.physicalSize = viewport;
      tester.view.devicePixelRatio = 1;
      tester.binding.handleMetricsChanged();
      SharedPreferences.setMockInitialValues({});
      final host = RecordingWindowsDesktopHost();
      final container = ProviderContainer(
        overrides: [
          ...voiceAppTestOverrides(
            client: MockClient((_) async => http.Response('', 404)),
          ),
          windowsDesktopHostProvider.overrideWithValue(host),
          subscriptionProvider.overrideWith(
            (ref) async => _subscription(status: 'active'),
          ),
        ],
      );

      await tester.pumpWidget(_app(container, theme: captureTheme));
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(AppearanceSettingsScreen.appIconKey));
      await tester.pumpAndSettle();

      expect(find.byKey(AppIconSettingsScreen.screenKey), findsOneWidget);
      final first = tester.getTopLeft(
        find.byKey(AppIconSettingsScreen.optionKey(AppIconPreference.voiceSky)),
      );
      final second = tester.getTopLeft(
        find.byKey(AppIconSettingsScreen.optionKey(AppIconPreference.midnight)),
      );
      final third = tester.getTopLeft(
        find.byKey(AppIconSettingsScreen.optionKey(AppIconPreference.violet)),
      );
      expect(first.dy, second.dy);
      if (viewport.width == 390) {
        expect(third.dy, greaterThan(first.dy));
      } else {
        expect(third.dy, first.dy);
      }
      expect(tester.takeException(), isNull);

      final orientation = viewport.width > viewport.height ? 'h' : 'v';
      final file = File(
        '${captureDirectory.path}${Platform.pathSeparator}app-icon-$orientation.png',
      );
      final captured = await tester.runAsync(() async {
        final boundary = tester.renderObject<RenderRepaintBoundary>(
          find.byKey(_captureBoundaryKey),
        );
        final image = await boundary.toImage(
          pixelRatio: tester.view.devicePixelRatio,
        );
        try {
          final data = await image.toByteData(format: ui.ImageByteFormat.png);
          if (data == null) throw StateError('PNG encoding returned null');
          await file.writeAsBytes(data.buffer.asUint8List(), flush: true);
          return await file.length();
        } finally {
          image.dispose();
        }
      });
      expect(captured, greaterThan(0));
      // ignore: avoid_print
      print('APP_ICON_CAPTURE ${file.path} $captured bytes');

      await tester.pumpWidget(const SizedBox.shrink());
      container.dispose();
    }
    tester.view.resetPhysicalSize();
    tester.view.resetDevicePixelRatio();
    debugDefaultTargetPlatformOverride = null;
  });
}

const _captureBoundaryKey = Key('app_icon_capture_boundary');

Future<ThemeData> _loadCaptureTheme(WidgetTester tester) async {
  final loaded = await tester.runAsync(() async {
    final noto = FontLoader(VoiceTheme.fontFamily)
      ..addFont(rootBundle.load('assets/fonts/NotoSans-Regular.ttf'))
      ..addFont(rootBundle.load('assets/fonts/NotoSans-Medium.ttf'))
      ..addFont(rootBundle.load('assets/fonts/NotoSans-SemiBold.ttf'))
      ..addFont(rootBundle.load('assets/fonts/NotoSans-Bold.ttf'));
    await noto.load();

    var flutterRoot = File(Platform.resolvedExecutable).parent;
    File iconFontForRoot(Directory root) => File(
      [
        root.path,
        'bin',
        'cache',
        'artifacts',
        'material_fonts',
        'MaterialIcons-Regular.otf',
      ].join(Platform.pathSeparator),
    );
    var iconFont = iconFontForRoot(flutterRoot);
    while (!iconFont.existsSync() &&
        flutterRoot.path != flutterRoot.parent.path) {
      flutterRoot = flutterRoot.parent;
      iconFont = iconFontForRoot(flutterRoot);
    }
    if (!iconFont.existsSync()) {
      throw StateError('Flutter MaterialIcons font is not cached');
    }
    final iconData = ByteData.sublistView(
      Uint8List.fromList(await iconFont.readAsBytes()),
    );
    await (FontLoader('MaterialIcons')..addFont(Future.value(iconData))).load();
    return true;
  });
  if (loaded != true) throw StateError('Production font loading failed');

  final theme = await tester.runAsync(() async {
    final catalog = await VoiceTokenCatalog.load();
    return VoiceTheme.build(
      catalog: catalog,
      mode: VoiceThemeMode.dark,
      profileAccent: catalog.profileAccentAt(0),
    );
  });
  if (theme == null) throw StateError('Production tokens did not load');
  return theme;
}

Widget _app(ProviderContainer container, {ThemeData? theme}) => RepaintBoundary(
  key: _captureBoundaryKey,
  child: UncontrolledProviderScope(
    container: container,
    child: MaterialApp(
      theme: theme ?? voiceTestTheme(),
      debugShowCheckedModeBanner: false,
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      home: const AppearanceSettingsScreen(),
    ),
  ),
);

VoiceSubscription _subscription({required String status}) => VoiceSubscription(
  id: 'subscription-1',
  accountId: 'account-1',
  plan: 'premium',
  billingPeriod: 'monthly',
  status: status,
);
