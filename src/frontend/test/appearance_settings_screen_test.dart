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
import 'package:voice_frontend/backend/subscription_client.dart';
import 'package:voice_frontend/backend/users_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/social_providers.dart';
import 'package:voice_frontend/state/subscription_providers.dart';
import 'package:voice_frontend/settings/theme_preference.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/theme/voice_theme.dart';
import 'package:voice_frontend/theme/voice_token_catalog.dart';
import 'package:voice_frontend/ui/settings/appearance_settings_screen.dart';
import 'package:voice_frontend/ui/settings/app_icon_settings_screen.dart';
import 'package:voice_frontend/ui/settings/chat_themes_settings_screen.dart';
import 'package:voice_frontend/ui/settings/settings_sheet.dart';
import 'package:voice_frontend/ui/core/voice_bottom_sheet.dart';

import 'support/auth_test_overrides.dart';
import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

const _captureDirectoryKey = 'VOICE_APPEARANCE_CAPTURE_DIR';
const _captureBoundaryKey = Key('appearance_capture');

var _captureFontsLoaded = false;

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  setUp(() => SharedPreferences.setMockInitialValues({}));

  testWidgets('Settings opens a dedicated Appearance screen', (tester) async {
    final openerFocus = FocusNode(debugLabel: 'settings opener');
    addTearDown(openerFocus.dispose);
    final container = ProviderContainer(
      overrides: voiceAppTestOverrides(
        client: MockClient((_) async => http.Response('', 404)),
      ),
    );
    addTearDown(container.dispose);

    await tester.pumpWidget(
      _settingsLauncher(container, openerFocus: openerFocus),
    );
    openerFocus.requestFocus();
    await tester.pump();
    expect(openerFocus.hasPrimaryFocus, isTrue);
    await tester.sendKeyEvent(LogicalKeyboardKey.enter);
    await tester.pumpAndSettle();
    await tester.ensureVisible(find.byKey(const Key('settings_appearance')));
    await tester.tap(find.byKey(const Key('settings_appearance')));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('appearance_settings_screen')), findsOneWidget);
    expect(find.byKey(const Key('appearance_theme_picker')), findsOneWidget);
    expect(find.byKey(AppearanceSettingsScreen.languageKey), findsOneWidget);
    await container
        .read(appThemePreferenceProvider.notifier)
        .setPreference(AppThemePreference.dark);
    await tester.pumpAndSettle();
    final systemOption = find.byKey(
      AppearanceSettingsScreen.themeOptionKey(AppThemePreference.system),
    );
    final systemInkWell = find.descendant(
      of: systemOption,
      matching: find.byType(InkWell),
    );
    final appearanceFocus = Focus.of(tester.element(systemInkWell));
    appearanceFocus.requestFocus();
    await tester.pump();
    expect(appearanceFocus.hasFocus, isTrue);
    await tester.sendKeyEvent(LogicalKeyboardKey.enter);
    await tester.pumpAndSettle();
    expect(
      container.read(appThemePreferenceProvider),
      AppThemePreference.system,
    );
    await tester.tap(find.byKey(AppearanceSettingsScreen.backKey));
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('test_open_settings')), findsOneWidget);
    expect(openerFocus.hasPrimaryFocus, isTrue);
  });

  testWidgets('the four theme choices update and persist their selection', (
    tester,
  ) async {
    final semantics = tester.ensureSemantics();
    final container = ProviderContainer(
      overrides: voiceAppTestOverrides(
        client: MockClient((_) async => http.Response('', 404)),
      ),
    );
    addTearDown(container.dispose);

    await tester.pumpWidget(_settingsLauncher(container));
    await _openAppearance(tester);

    final prefs = await SharedPreferences.getInstance();
    for (final mode in AppThemePreference.values) {
      final key = Key('appearance_theme_${mode.name}');
      await tester.ensureVisible(find.byKey(key));
      await tester.tap(find.byKey(key));
      await tester.pumpAndSettle();

      expect(container.read(appThemePreferenceProvider), mode);
      expect(prefs.getString(themePreferencePrefKey), mode.name);
      expect(
        tester
            .getSemantics(find.byKey(key))
            .flagsCollection
            .isSelected
            .toString(),
        'Tristate.isTrue',
      );
    }

    final restored = ProviderContainer(
      overrides: voiceAppTestOverrides(
        client: MockClient((_) async => http.Response('', 404)),
      ),
    );
    addTearDown(restored.dispose);
    restored.read(appThemePreferenceProvider);
    await tester.pump();
    await tester.pump();
    expect(
      restored.read(appThemePreferenceProvider),
      AppThemePreference.highContrast,
    );
    semantics.dispose();
  });

  testWidgets('theme choices support keyboard focus and activation', (
    tester,
  ) async {
    final container = ProviderContainer(
      overrides: voiceAppTestOverrides(
        client: MockClient((_) async => http.Response('', 404)),
      ),
    );
    addTearDown(container.dispose);

    await tester.pumpWidget(_settingsLauncher(container));
    await _openAppearance(tester);
    await container
        .read(appThemePreferenceProvider.notifier)
        .setPreference(AppThemePreference.dark);
    await tester.pumpAndSettle();

    final option = find.byKey(
      AppearanceSettingsScreen.themeOptionKey(AppThemePreference.system),
    );
    final detector = find.descendant(
      of: option,
      matching: find.byType(FocusableActionDetector),
    );
    expect(detector, findsOneWidget);
    expect(tester.widget<FocusableActionDetector>(detector).enabled, isTrue);

    final inkWell = find.descendant(of: option, matching: find.byType(InkWell));
    final focus = Focus.of(tester.element(inkWell));
    focus.requestFocus();
    await tester.pump();
    expect(focus.hasFocus, isTrue);

    await tester.sendKeyEvent(LogicalKeyboardKey.enter);
    await tester.pumpAndSettle();
    expect(
      container.read(appThemePreferenceProvider),
      AppThemePreference.system,
    );
    await container
        .read(appThemePreferenceProvider.notifier)
        .setPreference(AppThemePreference.dark);
    await tester.pumpAndSettle();
    focus.requestFocus();
    await tester.pump();
    await tester.sendKeyEvent(LogicalKeyboardKey.space);
    await tester.pumpAndSettle();
    expect(
      container.read(appThemePreferenceProvider),
      AppThemePreference.system,
    );
  });

  testWidgets('Appearance fits horizontal and vertical viewports', (
    tester,
  ) async {
    final captureTheme = await _loadCaptureTheme(tester);
    final unexpectedRequests = <String>[];
    final client = _noNetworkClient(unexpectedRequests);
    final container = _appearanceContainer(client);
    addTearDown(container.dispose);
    addTearDown(client.close);
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    addTearDown(() => tester.binding.setSurfaceSize(null));

    for (final size in [const Size(1280, 800), const Size(390, 844)]) {
      tester.view.physicalSize = size;
      tester.view.devicePixelRatio = 1;
      await tester.binding.setSurfaceSize(size);
      await tester.pumpWidget(
        _settingsLauncher(container, theme: captureTheme),
      );
      await _openAppearance(tester);
      await tester.pumpAndSettle();

      expect(
        find.byKey(const Key('appearance_settings_screen')),
        findsOneWidget,
      );
      expect(tester.takeException(), isNull);
      _expectPlusDestinations(tester);
      for (final key in [
        AppearanceSettingsScreen.chatThemesKey,
        AppearanceSettingsScreen.appIconKey,
      ]) {
        final rect = tester.getRect(find.byKey(key));
        expect(rect.left, greaterThanOrEqualTo(24));
        expect(rect.right, lessThanOrEqualTo(size.width - 24));
        expect(rect.bottom, lessThanOrEqualTo(size.height));
      }
      await _captureAppearance(
        tester,
        'appearance-${size.width > size.height ? 'h' : 'v'}.png',
      );
      await tester.tap(find.byKey(AppearanceSettingsScreen.backKey));
      await tester.pumpAndSettle();
    }
    await tester.pumpWidget(const SizedBox.shrink());
    await tester.pumpAndSettle();
    expect(unexpectedRequests, isEmpty);
  });

  for (final locale in [const Locale('en'), const Locale('ru')]) {
    for (final isPlus in [false, true]) {
      testWidgets('Plus destinations stay accessible and keyboard-operable '
          'for ${isPlus ? 'Plus' : 'free'} viewers in ${locale.languageCode}', (
        tester,
      ) async {
        final semantics = tester.ensureSemantics();
        try {
          final unexpectedRequests = <String>[];
          final client = _noNetworkClient(unexpectedRequests);
          final container = _appearanceContainer(client, isPlus: isPlus);
          addTearDown(container.dispose);
          addTearDown(client.close);
          container.read(selectedChatIdProvider.notifier).state = 'chat-a';
          await tester.pumpWidget(_settingsLauncher(container, locale: locale));
          await _openAppearance(tester);
          _expectPlusDestinations(tester);

          final l10n = AppLocalizations.of(
            tester.element(find.byKey(AppearanceSettingsScreen.screenKey)),
          )!;
          for (final entry in [
            (
              AppearanceSettingsScreen.chatThemesKey,
              l10n.settingsChatThemes,
              ChatThemesSettingsScreen.screenKey,
              const Key('chat_themes_back'),
            ),
            (
              AppearanceSettingsScreen.appIconKey,
              l10n.appIconTitle,
              AppIconSettingsScreen.screenKey,
              const Key('app_icon_back'),
            ),
          ]) {
            final tile = find.byKey(entry.$1);
            await tester.ensureVisible(tile);
            await tester.pumpAndSettle();
            final label = tester.getSemantics(tile).label;
            expect(label, contains(entry.$2));
            expect(label, contains(l10n.chatThemesPlusLabel));
            final title = find.descendant(
              of: tile,
              matching: find.text(entry.$2),
            );
            final focus = Focus.of(tester.element(title));
            focus.requestFocus();
            await tester.pump();
            expect(focus.hasFocus, isTrue);
            await tester.sendKeyEvent(LogicalKeyboardKey.enter);
            await tester.pumpAndSettle();
            expect(find.byKey(entry.$3), findsOneWidget);
            if (entry.$3 == ChatThemesSettingsScreen.screenKey) {
              expect(
                tester
                    .widget<ChatThemesSettingsScreen>(
                      find.byType(ChatThemesSettingsScreen),
                    )
                    .chatId,
                'chat-a',
              );
              expect(
                find.text(l10n.chatThemesPlusNotice),
                isPlus ? findsNothing : findsOneWidget,
              );
            }
            await tester.tap(find.byKey(entry.$4));
            await tester.pumpAndSettle();
            expect(
              find.byKey(AppearanceSettingsScreen.screenKey),
              findsOneWidget,
            );
          }
          expect(tester.takeException(), isNull);
          await tester.pumpWidget(const SizedBox.shrink());
          await tester.pumpAndSettle();
          expect(unexpectedRequests, isEmpty);
        } finally {
          semantics.dispose();
        }
      });
    }
  }

  test(
    'System uses the platform brightness when resolving the app theme',
    () async {
      final binding = TestWidgetsFlutterBinding.ensureInitialized();
      binding.platformDispatcher.platformBrightnessTestValue = Brightness.dark;
      addTearDown(binding.platformDispatcher.clearPlatformBrightnessTestValue);
      final container = ProviderContainer(
        overrides: [
          voiceTokenCatalogProvider.overrideWith(
            (ref) async => testVoiceTokenCatalog,
          ),
          activeProfileAccentColorProvider.overrideWith(
            (ref) => const AsyncValue.data(Color(0xFF7EC8E3)),
          ),
        ],
      );
      addTearDown(container.dispose);

      await container
          .read(appThemePreferenceProvider.notifier)
          .setPreference(AppThemePreference.system);
      expect(
        (await container.read(voiceMaterialThemeProvider.future)).brightness,
        Brightness.dark,
      );

      binding.platformDispatcher.platformBrightnessTestValue = Brightness.light;
      container.invalidate(voiceMaterialThemeProvider);
      expect(
        (await container.read(voiceMaterialThemeProvider.future)).brightness,
        Brightness.light,
      );
    },
  );
}

Widget _settingsLauncher(
  ProviderContainer container, {
  ThemeData? theme,
  FocusNode? openerFocus,
  Locale? locale,
}) => UncontrolledProviderScope(
  container: container,
  child: RepaintBoundary(
    key: _captureBoundaryKey,
    child: MaterialApp(
      theme: theme ?? voiceTestTheme(),
      locale: locale,
      debugShowCheckedModeBanner: false,
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      home: Builder(
        builder: (context) => Scaffold(
          body: TextButton(
            key: const Key('test_open_settings'),
            focusNode: openerFocus,
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
);

void _expectPlusDestinations(WidgetTester tester) {
  expect(find.text('Voice Plus'), findsOneWidget);
  for (final key in [
    AppearanceSettingsScreen.chatThemesKey,
    AppearanceSettingsScreen.appIconKey,
  ]) {
    final tile = find.byKey(key);
    expect(tile, findsOneWidget);
    final star = find.descendant(of: tile, matching: find.byIcon(Icons.star));
    expect(star, findsOneWidget);
    expect(tester.widget<Icon>(star).icon!.fontFamily, 'MaterialIcons');
    expect(find.descendant(of: tile, matching: find.text('★')), findsNothing);
    expect(
      find.descendant(of: tile, matching: find.byIcon(Icons.chevron_right)),
      findsOneWidget,
    );
    expect(tester.widget<ListTile>(tile).onTap, isNotNull);
  }
}

MockClient _noNetworkClient(List<String> requests) =>
    MockClient((request) async {
      requests.add('${request.method} ${request.url}');
      return http.Response('', 500);
    });

ProviderContainer _appearanceContainer(
  http.Client client, {
  bool isPlus = false,
}) => ProviderContainer(
  overrides: [
    ...voiceAppTestOverrides(client: client),
    activeProfileProvider.overrideWith((ref) async => _appearanceProfile),
    profileProvider.overrideWith((ref, id) async => _appearanceProfile),
    subscriptionProvider.overrideWith((ref) async => isPlus ? _plus : null),
  ],
);

const _appearanceProfile = VoiceProfile(
  id: 'prof-test',
  accountId: 'acc-test',
  username: 'voiceuser',
  discriminator: '4242',
  displayName: 'Voice User',
  isPrimary: true,
);
const _plus = VoiceSubscription(
  id: 'subscription-1',
  accountId: 'acc-test',
  plan: 'premium',
  billingPeriod: 'month',
  status: 'active',
);

Future<ThemeData?> _loadCaptureTheme(WidgetTester tester) async {
  final directory = Platform.environment[_captureDirectoryKey];
  if (directory == null || directory.isEmpty) return null;

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
      final iconData = ByteData.sublistView(
        Uint8List.fromList(await iconFont.readAsBytes()),
      );
      await (FontLoader(
        'MaterialIcons',
      )..addFont(Future<ByteData>.value(iconData))).load();
      return true;
    });
    if (loaded != true) throw StateError('Production font loading failed');
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
  if (theme == null) throw StateError('Production tokens did not load');
  return theme;
}

Future<void> _captureAppearance(WidgetTester tester, String filename) async {
  final directory = Platform.environment[_captureDirectoryKey];
  if (directory == null || directory.isEmpty) return;
  final boundary = tester.renderObject<RenderRepaintBoundary>(
    find.byKey(_captureBoundaryKey),
  );
  final captured = await tester.runAsync(() async {
    final image = await boundary.toImage(
      pixelRatio: tester.view.devicePixelRatio,
    );
    try {
      final bytes = await image.toByteData(format: ui.ImageByteFormat.png);
      if (bytes == null) throw StateError('PNG encoding returned null');
      final output = File('$directory${Platform.pathSeparator}$filename');
      await output.parent.create(recursive: true);
      await output.writeAsBytes(bytes.buffer.asUint8List(), flush: true);
      return true;
    } finally {
      image.dispose();
    }
  });
  if (captured != true) throw StateError('Appearance capture failed');
}

Future<void> _openAppearance(WidgetTester tester) async {
  await tester.tap(find.byKey(const Key('test_open_settings')));
  await tester.pumpAndSettle();
  await tester.ensureVisible(find.byKey(const Key('settings_appearance')));
  await tester.tap(find.byKey(const Key('settings_appearance')));
  await tester.pumpAndSettle();
}
