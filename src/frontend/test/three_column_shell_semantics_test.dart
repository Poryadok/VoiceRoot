import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/shell/three_column_shell.dart';

import 'support/voice_test_theme.dart';

/// app stack8 a11y: navigation and chat regions labeled for screen readers.
void main() {
  testWidgets('three column shell exposes nav and chat semantics', (tester) async {
    await tester.binding.setSurfaceSize(const Size(1280, 800));
    addTearDown(() => tester.binding.setSurfaceSize(null));

    await tester.pumpWidget(
      MaterialApp(
        theme: voiceTestTheme(),
        locale: const Locale('en'),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: ThreeColumnShell(
          navigationChild: Container(key: ThreeColumnShell.navActiveRail),
          middleChild: Container(key: ThreeColumnShell.navChatList),
          mainChild: Container(key: ThreeColumnShell.navOpenChat),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.bySemanticsLabel('Navigation'), findsOneWidget);
    expect(find.bySemanticsLabel('Chat list'), findsOneWidget);
    expect(find.bySemanticsLabel('Conversation'), findsOneWidget);
  });
}
