import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/shell_providers.dart';
import 'package:voice_frontend/ui/shell/mobile_shell_tab_bar.dart';

import 'support/voice_test_theme.dart';

void main() {
  testWidgets('MobileShellTabBar switches navigation section', (tester) async {
    final container = ProviderContainer();
    addTearDown(container.dispose);

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(body: MobileShellTabBar()),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(MobileShellTabBar.barKey), findsOneWidget);
    await tester.tap(find.text('Friends'));
    await tester.pumpAndSettle();

    expect(
      container.read(navigationSectionProvider),
      NavigationSection.social,
    );
  });
}
