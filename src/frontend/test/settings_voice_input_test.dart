import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/settings/voice_input_settings.dart';
import 'package:voice_frontend/ui/settings/settings_sheet.dart';

import 'support/auth_test_overrides.dart';
import 'support/voice_test_theme.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  setUp(() {
    SharedPreferences.setMockInitialValues({});
  });

  testWidgets('settings expose PTT/VAD mode and keybind', (tester) async {
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
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(body: SettingsSheet()),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(SettingsSheet.pttModeKey), findsOneWidget);
    expect(find.byKey(SettingsSheet.pttKeybindKey), findsOneWidget);

    await tester.ensureVisible(find.byKey(SettingsSheet.pttModeKey));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(SettingsSheet.pttModeKey));
    await tester.pumpAndSettle();

    expect(container.read(voiceInputSettingsProvider).mode, VoiceInputMode.ptt);
  });
}
