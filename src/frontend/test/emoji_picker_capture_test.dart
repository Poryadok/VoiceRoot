import 'dart:io';
import 'dart:ui' as ui;

import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/theme/voice_emoji_style.dart';
import 'package:voice_frontend/theme/voice_theme.dart';
import 'package:voice_frontend/theme/voice_token_catalog.dart';
import 'package:voice_frontend/ui/chat/composer_panels.dart';

import 'support/voice_test_theme.dart';

void main() {
  for (final wide in [true, false]) {
    testWidgets('emoji reference panel ${wide ? 'H' : 'V'}', (tester) async {
      tester.view.physicalSize = wide
          ? const Size(1280, 800)
          : const Size(390, 844);
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      final directory = Platform.environment['VOICE_EMOJI_CAPTURE_DIR'];
      var theme = voiceTestTheme();
      if (directory != null && directory.isNotEmpty) {
        theme = (await tester.runAsync(() async {
          await (FontLoader(VoiceTheme.fontFamily)
                ..addFont(rootBundle.load('assets/fonts/NotoSans-Regular.ttf'))
                ..addFont(
                  rootBundle.load('assets/fonts/NotoSans-SemiBold.ttf'),
                ))
              .load();
          await (FontLoader(VoiceEmojiStyle.fontFamily)
                ..addFont(rootBundle.load('assets/fonts/NotoColorEmoji.ttf')))
              .load();
          final catalog = await VoiceTokenCatalog.load();
          return VoiceTheme.build(
            catalog: catalog,
            mode: VoiceThemeMode.dark,
            profileAccent: catalog.profileAccentAt(0),
          );
        }))!;
      }
      const rootKey = Key('emoji_capture_root');
      final trigger = GlobalKey();
      final selected = <String>[];
      await tester.pumpWidget(
        RepaintBoundary(
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
      );
      await tester.tap(find.byKey(trigger));
      await tester.pumpAndSettle();
      expect(find.byKey(ComposerEmojiPanelBody.panelKey), findsOneWidget);
      expect(find.text('Недавние'), findsOneWidget);
      expect(tester.takeException(), isNull);
      if (directory != null && directory.isNotEmpty) {
        final boundary = tester.renderObject<RenderRepaintBoundary>(
          find.byKey(rootKey),
        );
        await tester.runAsync(() async {
          final image = await boundary.toImage(pixelRatio: 1);
          try {
            final data = await image.toByteData(format: ui.ImageByteFormat.png);
            final file = File('$directory/emoji-${wide ? 'h' : 'v'}.png');
            await file.parent.create(recursive: true);
            await file.writeAsBytes(data!.buffer.asUint8List(), flush: true);
          } finally {
            image.dispose();
          }
        });
      }
      await tester.tap(find.byKey(const Key('composer_recent_😀')));
      await tester.pumpAndSettle();
      expect(selected, ['😀']);
      expect(find.byKey(ComposerEmojiPanelBody.panelKey), findsNothing);
    });
  }
}
