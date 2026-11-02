import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/ui/chat/composer_panels.dart';

import 'support/voice_test_theme.dart';

void main() {
  setUp(() => SharedPreferences.setMockInitialValues({}));

  for (final exit in ['selection', 'barrier', 'escape']) {
    testWidgets('mobile attach restores focus once after $exit', (
      tester,
    ) async {
      tester.view.physicalSize = const Size(390, 844);
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      final focus = FocusNode();
      addTearDown(focus.dispose);
      var dismissals = 0;
      ComposerAttachAction? selected;
      await tester.pumpWidget(
        ProviderScope(
          child: MaterialApp(
            theme: voiceTestTheme(),
            locale: const Locale('en'),
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: Builder(
              builder: (context) => Scaffold(
                body: TextButton(
                  focusNode: focus,
                  onPressed: () async {
                    selected = await showComposerAttachMenu(
                      context,
                      onDismiss: () {
                        dismissals++;
                        focus.requestFocus();
                      },
                    );
                  },
                  child: const Text('open attach'),
                ),
              ),
            ),
          ),
        ),
      );
      focus.requestFocus();
      await tester.pump();
      await tester.sendKeyEvent(LogicalKeyboardKey.enter);
      await tester.pumpAndSettle();
      expect(find.byType(BottomSheet), findsOneWidget);
      final hasTitle = find.text('Attach file').evaluate().isNotEmpty;
      if (exit == 'selection') {
        await tester.tap(find.byKey(const Key('composer_attach_document')));
      } else if (exit == 'barrier') {
        await tester.tapAt(const Offset(380, 20));
      } else {
        await tester.sendKeyEvent(LogicalKeyboardKey.escape);
      }
      await tester.pumpAndSettle();
      expect(find.byType(BottomSheet), findsNothing);
      expect(dismissals, 1);
      expect(focus.hasFocus, isTrue);
      expect(
        selected,
        exit == 'selection' ? ComposerAttachAction.document : null,
      );
      expect(hasTitle, isTrue);
      expect(tester.takeException(), isNull);
    });
  }

  testWidgets(
    'mobile attach scrolls to Music with large text in short viewport',
    (tester) async {
      tester.view.physicalSize = const Size(390, 400);
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      ComposerAttachAction? selected;
      await tester.pumpWidget(
        ProviderScope(
          child: MaterialApp(
            theme: voiceTestTheme(),
            locale: const Locale('en'),
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            builder: (context, child) => MediaQuery(
              data: MediaQuery.of(
                context,
              ).copyWith(textScaler: TextScaler.linear(2)),
              child: child!,
            ),
            home: Builder(
              builder: (context) => Scaffold(
                body: TextButton(
                  onPressed: () async {
                    selected = await showComposerAttachMenu(context);
                  },
                  child: const Text('open attach'),
                ),
              ),
            ),
          ),
        ),
      );
      await tester.tap(find.text('open attach'));
      await tester.pumpAndSettle();
      final music = find.byKey(const Key('composer_attach_music'));
      await tester.ensureVisible(music);
      await tester.pumpAndSettle();
      await tester.tap(music);
      await tester.pumpAndSettle();
      expect(selected, ComposerAttachAction.music);
      expect(tester.takeException(), isNull);
    },
  );
}
