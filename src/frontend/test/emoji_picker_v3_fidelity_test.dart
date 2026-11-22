import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/ui/chat/composer_panels.dart';

import 'support/voice_test_theme.dart';

void main() {
  setUp(() => SharedPreferences.setMockInitialValues({}));

  for (final wide in [true, false]) {
    testWidgets('emoji panel v3 chip grid ${wide ? 'H' : 'V'}', (tester) async {
      tester.view.physicalSize = wide
          ? const Size(1280, 800)
          : const Size(390, 844);
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      final columns = wide ? 8 : 6;
      const rootKey = const Key('emoji_v3_fidelity_root');
      final trigger = GlobalKey();
      final selected = <String>[];
      final theme = voiceTestTheme();
      await tester.pumpWidget(
        ProviderScope(
          child: RepaintBoundary(
            key: rootKey,
            child: MaterialApp(
              theme: theme,
              locale: const Locale('ru'),
              localizationsDelegates: AppLocalizations.localizationsDelegates,
              supportedLocales: AppLocalizations.supportedLocales,
              home: Builder(
                builder: (context) => Scaffold(
                  body: Align(
                    alignment: Alignment.bottomRight,
                    child: Padding(
                      padding: const EdgeInsets.all(16),
                      child: TextButton(
                        key: trigger,
                        onPressed: () => showComposerEmojiPanel(
                          context,
                          anchorContext: trigger.currentContext,
                          recentEmoji: const ['😀', '😂', '❤️'],
                          onSelected: selected.add,
                        ),
                        child: const Text('Эмодзи'),
                      ),
                    ),
                  ),
                ),
              ),
            ),
          ),
        ),
      );
      await tester.tap(find.byKey(trigger));
      await tester.pumpAndSettle();
      expect(find.byKey(ComposerEmojiPanelBody.panelKey), findsOneWidget);
      expect(find.text('Недавние'), findsOneWidget);
      expect(tester.takeException(), isNull);

      final panelRect = tester.getRect(
        find.byKey(ComposerEmojiPanelBody.panelKey),
      );
      // V sheets clip the last row offscreen: GridView lays out visible slivers only.
      final rects = [
        for (final emoji in ComposerEmojiPanelBody.choices.take(columns * 4))
          tester.getRect(find.byKey(Key('composer_emoji_$emoji'))),
      ];
      final firstRect = rects.first;
      expect(firstRect.width, closeTo(firstRect.height, 0.1));
      for (var i = 1; i < columns; i++) {
        expect(rects[i].top, closeTo(firstRect.top, 0.1));
        expect(rects[i].width, closeTo(firstRect.width, 0.1));
        expect(rects[i].height, closeTo(firstRect.height, 0.1));
        expect(
          rects[i].left - rects[i - 1].left,
          closeTo(firstRect.width + 8, 0.1),
        );
      }
      // The (columns+1)-th choice wraps to a lower row, proving the column count.
      expect(rects[columns].top, greaterThan(firstRect.top));
      for (final rect in rects) {
        expect(rect.left, greaterThanOrEqualTo(panelRect.left));
        expect(rect.right, lessThanOrEqualTo(panelRect.right));
        expect(rect.top, greaterThanOrEqualTo(panelRect.top));
      }

      await tester.tap(find.byKey(const Key('composer_emoji_❤️')));
      await tester.pumpAndSettle();
      expect(tester.takeException(), isNull);
      expect(selected, ['❤️']);
      expect(find.byKey(ComposerEmojiPanelBody.panelKey), findsNothing);
    });
  }
}
