import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/shell_providers.dart';
import 'package:voice_frontend/ui/shell/side_panel.dart';

import 'support/voice_test_theme.dart';

void main() {
  testWidgets('SidePanelHost shows emoji picker when panel is emoji', (
    tester,
  ) async {
    String? picked;
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          shellSidePanelProvider.overrideWith((ref) => ShellSidePanel.emoji),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: Scaffold(
            body: SizedBox(
              width: 300,
              height: 400,
              child: SidePanelHost(onEmojiSelected: (e) => picked = e),
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(SidePanelHost.hostKey), findsOneWidget);
    expect(find.byKey(SidePanelHost.emojiPickerKey), findsOneWidget);

    await tester.tap(find.text('👍'));
    await tester.pump();

    expect(picked, '👍');
  });
}
