import 'dart:ui' show Tristate;

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/settings/voice_input_settings.dart';
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

  testWidgets('Accessibility settings expose persistent PTT/VAD and keybind', (
    tester,
  ) async {
    final container = ProviderContainer(
      overrides: [
        ...voiceAppTestOverrides(
          client: MockClient((_) async => http.Response('{}', 200)),
        ),
        voiceInputSettingsProvider.overrideWith(VoiceInputSettingsNotifier.new),
      ],
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
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('open_settings')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(SettingsSheet.accessibilityKey));
    await tester.pumpAndSettle();

    expect(find.byKey(AccessibilitySettingsScreen.screenKey), findsOneWidget);
    final semantics = tester.ensureSemantics();

    final pttChoice = find.byKey(AccessibilitySettingsScreen.pttModeKey);
    expect(
      tester
          .getSemantics(pttChoice)
          .getSemanticsData()
          .flagsCollection
          .isSelected,
      Tristate.isFalse,
    );
    await tester.tap(pttChoice);
    await tester.pumpAndSettle();
    expect(tester.widget<ChoiceChip>(pttChoice).selected, isTrue);
    expect(
      tester
          .getSemantics(pttChoice)
          .getSemanticsData()
          .flagsCollection
          .isSelected,
      Tristate.isTrue,
    );
    expect(container.read(voiceInputSettingsProvider).mode, VoiceInputMode.ptt);
    await container
        .read(voiceInputSettingsProvider.notifier)
        .setMode(VoiceInputMode.ptt);

    await tester.tap(find.byKey(AccessibilitySettingsScreen.pttKeybindKey));
    await tester.pumpAndSettle();
    await tester.sendKeyEvent(LogicalKeyboardKey.keyK);
    await tester.pumpAndSettle();
    expect(
      container.read(voiceInputSettingsProvider).pttKey,
      LogicalKeyboardKey.keyK,
    );
    await container
        .read(voiceInputSettingsProvider.notifier)
        .setPttKey(LogicalKeyboardKey.keyK);
    expect(
      (await SharedPreferences.getInstance()).getString('voice_input_mode'),
      'ptt',
    );
    expect(
      (await SharedPreferences.getInstance()).getInt('voice_ptt_key_id'),
      LogicalKeyboardKey.keyK.keyId,
    );
    semantics.dispose();
  });
}
