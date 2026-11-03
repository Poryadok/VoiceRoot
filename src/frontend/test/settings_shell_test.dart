import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/ui/settings/settings_sheet.dart';
import 'package:voice_frontend/ui/settings/settings_scaffold.dart';
import 'package:voice_frontend/ui/settings/security_settings_screen.dart';
import 'package:voice_frontend/ui/settings/appearance_settings_screen.dart';
import 'package:voice_frontend/ui/shell/desktop_shell_rail.dart';

import 'support/auth_test_overrides.dart';
import 'support/voice_test_theme.dart';

Future<void> _open(WidgetTester tester, Size size) async {
  tester.view.physicalSize = size;
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);
  await tester.pumpWidget(
    ProviderScope(
      overrides: voiceAppTestOverrides(
        client: MockClient((_) async => http.Response('{}', 404)),
      ),
      child: MaterialApp(
        theme: voiceTestTheme(),
        locale: const Locale('en'),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: Builder(
          builder: (context) => Scaffold(
            body: TextButton(
              key: const Key('open_settings'),
              onPressed: () => Navigator.of(context).push(
                MaterialPageRoute<void>(
                  builder: (_) => const Scaffold(body: SettingsSheet()),
                ),
              ),
              child: const Text('Open'),
            ),
          ),
        ),
      ),
    ),
  );
  await tester.tap(find.byKey(const Key('open_settings')));
  await tester.pumpAndSettle();
}

void main() {
  testWidgets(
    'desktop awaits editor confirmation before replacing or closing detail',
    (tester) async {
      await _open(tester, const Size(1280, 800));
      final scope = SettingsDetailScope.maybeOf(
        tester.element(find.byKey(SecuritySettingsScreen.screenKey)),
      )!;
      final answer = Completer<bool>();
      final owner = Object();
      scope.registerLeaveGuard(owner, () => answer.future);
      await tester.ensureVisible(find.byKey(const Key('settings_appearance')));
      await tester.tap(find.byKey(const Key('settings_appearance')));
      await tester.pump();
      expect(find.byKey(SecuritySettingsScreen.screenKey), findsOneWidget);
      expect(find.byKey(AppearanceSettingsScreen.screenKey), findsNothing);
      answer.complete(false);
      await tester.pumpAndSettle();
      expect(find.byKey(SecuritySettingsScreen.screenKey), findsOneWidget);
      scope.registerLeaveGuard(owner, () async => true);
      await tester.tap(find.byKey(const Key('settings_close')));
      await tester.pumpAndSettle();
      expect(find.byKey(SettingsSheet.sheetKey), findsNothing);
    },
  );

  testWidgets('desktop keeps real rail and destinations when detail changes', (
    tester,
  ) async {
    await _open(tester, const Size(1280, 800));
    expect(find.byKey(DesktopShellRail.railKey), findsOneWidget);
    final navigation = find.byKey(const Key('settings_navigation'));
    expect(tester.getSize(navigation).width, 320);
    expect(tester.getTopLeft(navigation).dx, 56);
    await tester.ensureVisible(find.byKey(const Key('settings_appearance')));
    await tester.tap(find.byKey(const Key('settings_appearance')));
    await tester.pumpAndSettle();
    expect(find.byKey(AppearanceSettingsScreen.screenKey), findsOneWidget);
    expect(find.byKey(DesktopShellRail.railKey), findsOneWidget);
    await tester.scrollUntilVisible(
      find.byKey(const Key('settings_privacy')),
      -200,
      scrollable: find.descendant(
        of: navigation,
        matching: find.byType(Scrollable),
      ),
    );
    expect(find.byKey(const Key('settings_privacy')), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets('mobile pushes details and back returns to settings hub', (
    tester,
  ) async {
    await _open(tester, const Size(390, 844));
    expect(find.byKey(DesktopShellRail.railKey), findsNothing);
    await tester.ensureVisible(find.byKey(const Key('settings_appearance')));
    await tester.tap(find.byKey(const Key('settings_appearance')));
    await tester.pumpAndSettle();
    await tester.pageBack();
    await tester.pumpAndSettle();
    expect(find.byKey(SettingsSheet.sheetKey), findsOneWidget);
    expect(find.byKey(const Key('settings_notifications')), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets('Escape dismisses the desktop settings route', (tester) async {
    await _open(tester, const Size(1280, 800));
    await tester.sendKeyEvent(LogicalKeyboardKey.escape);
    await tester.pumpAndSettle();
    expect(find.byKey(SettingsSheet.sheetKey), findsNothing);
    expect(find.byKey(const Key('open_settings')), findsOneWidget);
  });

  testWidgets('logout requests confirmation and cancel preserves settings', (
    tester,
  ) async {
    await _open(tester, const Size(390, 844));
    await tester.ensureVisible(find.byKey(const Key('settings_logout')));
    await tester.tap(find.byKey(const Key('settings_logout')));
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('settings_logout_confirm')), findsOneWidget);
    await tester.tap(find.text('Cancel'));
    await tester.pumpAndSettle();
    expect(find.byKey(SettingsSheet.sheetKey), findsOneWidget);
  });
}
