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
      for (final heading in ['Motion and text', 'Voice calls']) {
        expect(find.bySemanticsLabel(heading), findsOneWidget);
        expect(
          tester
              .getSemantics(find.bySemanticsLabel(heading))
              .getSemanticsData()
              .flagsCollection
              .isHeader,
          isTrue,
        );
      }
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
      expect(find.text('Animations and GIFs show statically'), findsOneWidget);

      expect(
        find.byKey(AccessibilitySettingsScreen.textScaleKey),
        findsOneWidget,
      );
      expect(find.text('Text scale'), findsOneWidget);
      expect(find.text('100%'), findsNWidgets(2));
      expect(find.text('200%'), findsOneWidget);
      expect(
        find.text(
          'All controls are reachable by keyboard and keep a visible focus '
          'outline.',
        ),
        findsOneWidget,
      );
      semantics.dispose();
    },
  );

  testWidgets(
    'Accessibility settings show v3 text-scale band, pills and keyboard notice',
    (tester) async {
      final container = ProviderContainer(
        overrides: voiceAppTestOverrides(
          client: MockClient((_) async => http.Response('{}', 200)),
        ),
      );
      addTearDown(container.dispose);

      await tester.binding.setSurfaceSize(const Size(1280, 800));
      addTearDown(() => tester.binding.setSurfaceSize(null));
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
                  size: const Size(1280, 800),
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
      await tester.tap(find.byKey(const Key('open_settings')));
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const Key('settings_accessibility')));
      await tester.pumpAndSettle();

      expect(
        find.byKey(AccessibilitySettingsScreen.textScaleKey),
        findsOneWidget,
      );
      expect(find.text('Text scale'), findsOneWidget);
      expect(find.text('150%'), findsOneWidget);
      expect(find.text('100%'), findsAtLeastNWidgets(1));
      expect(find.text('200%'), findsAtLeastNWidgets(1));
      expect(
        find.byKey(AccessibilitySettingsScreen.pttKeybindKey),
        findsOneWidget,
      );
      expect(
        find.byKey(AccessibilitySettingsScreen.pttModeKey),
        findsOneWidget,
      );
      expect(
        find.byKey(AccessibilitySettingsScreen.vadModeKey),
        findsOneWidget,
      );
      expect(
        find.text(
          'All controls are reachable by keyboard and keep a visible focus '
          'outline.',
        ),
        findsOneWidget,
      );
      expect(tester.takeException(), isNull);
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
      expect(
        find.byKey(AccessibilitySettingsScreen.textScaleKey),
        findsOneWidget,
      );
      expect(find.text('150%'), findsOneWidget);
      expect(tester.takeException(), isNull);
      await tester.pumpWidget(const SizedBox.shrink());
    }
  });
}
