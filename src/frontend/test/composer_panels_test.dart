import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/ui/chat/composer_panels.dart';

import 'support/voice_test_theme.dart';

void main() {
  testWidgets('composer emoji panel is transient overlay with emoji choices', (
    tester,
  ) async {
    String? picked;
    await tester.pumpWidget(
      MaterialApp(
        theme: voiceTestTheme(),
        locale: const Locale('en'),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: Builder(
          builder: (context) => Scaffold(
            body: FilledButton(
              onPressed: () => showComposerEmojiPanel(
                context,
                onSelected: (emoji) => picked = emoji,
              ),
              child: const Text('open'),
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.text('open'));
    await tester.pumpAndSettle();

    expect(find.byKey(ComposerEmojiPanelBody.panelKey), findsOneWidget);

    await tester.tap(find.byKey(const Key('composer_emoji_👍')));
    await tester.pumpAndSettle();

    expect(picked, '👍');
  });

  testWidgets('composer attach menu exposes photo and document actions', (
    tester,
  ) async {
    ComposerAttachAction? picked;
    await tester.pumpWidget(
      MaterialApp(
        theme: voiceTestTheme(),
        locale: const Locale('en'),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: Builder(
          builder: (context) => Scaffold(
            body: FilledButton(
              onPressed: () async {
                picked = await showComposerAttachMenu(context);
              },
              child: const Text('attach'),
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.text('attach'));
    await tester.pumpAndSettle();

    expect(find.byKey(ComposerAttachMenuBody.menuKey), findsOneWidget);
    expect(find.text('Photo or video'), findsOneWidget);
    expect(find.text('Document'), findsOneWidget);

    await tester.tap(find.byKey(const Key('composer_attach_document')));
    await tester.pumpAndSettle();

    expect(picked, ComposerAttachAction.document);
  });
}
