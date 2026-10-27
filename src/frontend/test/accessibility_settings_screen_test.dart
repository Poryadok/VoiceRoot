import 'dart:ui' show Tristate;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/settings/reduced_motion.dart';
import 'package:voice_frontend/ui/core/voice_bottom_sheet.dart';
import 'package:voice_frontend/ui/settings/accessibility_settings_screen.dart';
import 'package:voice_frontend/ui/settings/settings_sheet.dart';

import 'support/auth_test_overrides.dart';
import 'support/voice_test_theme.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  setUp(() {
    SharedPreferences.setMockInitialValues({});
  });

  testWidgets(
    'Settings opens accessibility controls backed by saved settings',
    (tester) async {
      final container = ProviderContainer(
        overrides: voiceAppTestOverrides(
          client: MockClient((_) async => http.Response('{}', 200)),
        ),
      );
      addTearDown(container.dispose);

      await tester.pumpWidget(
        UncontrolledProviderScope(
          container: container,
          child: MaterialApp(
            theme: voiceTestTheme(),
            locale: const Locale('en'),
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
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
      await tester.tap(find.byKey(const Key('open_settings')));
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const Key('settings_accessibility')));
      await tester.pumpAndSettle();

      expect(find.byKey(AccessibilitySettingsScreen.screenKey), findsOneWidget);
      final semantics = tester.ensureSemantics();
      final motionToggle = find.byKey(
        AccessibilitySettingsScreen.reducedMotionKey,
      );
      expect(
        tester
            .getSemantics(motionToggle)
            .getSemanticsData()
            .flagsCollection
            .isToggled,
        isNot(Tristate.none),
      );

      await tester.tap(motionToggle);
      await tester.pumpAndSettle();
      expect(container.read(reducedMotionEnabledProvider), isTrue);
      expect(
        (await SharedPreferences.getInstance()).getBool('voice_reduced_motion'),
        isTrue,
      );
      semantics.dispose();
    },
  );

  testWidgets('Accessibility settings fit desktop and mobile reference sizes', (
    tester,
  ) async {
    final container = ProviderContainer(
      overrides: voiceAppTestOverrides(
        client: MockClient((_) async => http.Response('{}', 200)),
      ),
    );
    addTearDown(container.dispose);
    addTearDown(() => tester.binding.setSurfaceSize(null));

    for (final size in const [Size(1280, 800), Size(390, 844)]) {
      await tester.binding.setSurfaceSize(size);
      await tester.pumpWidget(
        UncontrolledProviderScope(
          container: container,
          child: MaterialApp(
            theme: voiceTestTheme(),
            locale: const Locale('en'),
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            builder: (context, child) {
              final media = MediaQuery.of(context);
              return MediaQuery(
                data: media.copyWith(
                  size: size,
                  textScaler: const TextScaler.linear(1.5),
                ),
                child: child!,
              );
            },
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
      await tester.tap(find.byKey(const Key('settings_accessibility')));
      await tester.pumpAndSettle();

      expect(find.byKey(AccessibilitySettingsScreen.screenKey), findsOneWidget);
      expect(tester.takeException(), isNull);
      await tester.pumpWidget(const SizedBox.shrink());
    }
  });
}
