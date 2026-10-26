import 'dart:io';
import 'dart:ui' as ui;

import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/ui/chat/composer_send_menu.dart';
import 'package:voice_frontend/theme/voice_theme.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/theme/voice_token_catalog.dart';

import 'support/voice_test_theme.dart';

void main() {
  testWidgets('capture SendMenu H/V using actual light theme tokens', (
    tester,
  ) async {
    const directory = String.fromEnvironment('VOICE_SEND_MENU_CAPTURE_DIR');
    if (directory.isEmpty) return;
    final theme = await tester.runAsync(() async {
      await (FontLoader(VoiceTheme.fontFamily)
            ..addFont(rootBundle.load('assets/fonts/NotoSans-Regular.ttf'))
            ..addFont(rootBundle.load('assets/fonts/NotoSans-SemiBold.ttf')))
          .load();
      final sdk = Platform.environment['FLUTTER_ROOT']!;
      final bytes = await File(
        '$sdk/bin/cache/artifacts/material_fonts/MaterialIcons-Regular.otf',
      ).readAsBytes();
      await (FontLoader(
        'MaterialIcons',
      )..addFont(Future.value(ByteData.sublistView(bytes)))).load();
      final catalog = await VoiceTokenCatalog.load();
      return VoiceTheme.build(
        catalog: catalog,
        mode: VoiceThemeMode.light,
        profileAccent: catalog.profileAccentAt(0),
      );
    });
    for (final width in [1280.0, 390.0]) {
      tester.view.physicalSize = Size(width, 844);
      tester.view.devicePixelRatio = 1;
      final boundaryKey = GlobalKey();
      final anchorKey = GlobalKey();
      await tester.pumpWidget(
        RepaintBoundary(
          key: boundaryKey,
          child: MaterialApp(
            theme: theme,
            locale: const Locale('ru'),
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: Builder(
              builder: (context) => Scaffold(
                appBar: AppBar(
                  title: Text(width > 600 ? 'Alex' : 'Design Team'),
                ),
                body: Align(
                  alignment: Alignment.bottomRight,
                  child: Padding(
                    padding: const EdgeInsets.all(16),
                    child: IconButton(
                      key: anchorKey,
                      icon: const Icon(Icons.send),
                      onPressed: () => showComposerSendMenu(
                        context,
                        anchorContext: anchorKey.currentContext,
                        isDm: width > 600,
                        scheduledMessagesEnabled: true,
                      ),
                    ),
                  ),
                ),
              ),
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(anchorKey));
      await tester.pumpAndSettle();
      expect(tester.takeException(), isNull);
      final boundary = tester.renderObject<RenderRepaintBoundary>(
        find.byKey(boundaryKey),
      );
      await tester.runAsync(() async {
        final image = await boundary.toImage(pixelRatio: 1);
        try {
          final bytes = await image.toByteData(format: ui.ImageByteFormat.png);
          final file = File('$directory/sendmenu-${width.toInt()}.png');
          await file.parent.create(recursive: true);
          await file.writeAsBytes(bytes!.buffer.asUint8List());
        } finally {
          image.dispose();
        }
      });
      await tester.sendKeyEvent(LogicalKeyboardKey.escape);
      await tester.pumpAndSettle();
    }
    tester.view.resetPhysicalSize();
    tester.view.resetDevicePixelRatio();
  });
  for (final width in [1280.0, 390.0]) {
    for (final isDm in [true, false]) {
      testWidgets(
        'send options fit $width DM=$isDm, dismiss and return focus',
        (tester) async {
          tester.view.physicalSize = Size(width, 844);
          tester.view.devicePixelRatio = 1;
          addTearDown(tester.view.resetPhysicalSize);
          addTearDown(tester.view.resetDevicePixelRatio);
          final focus = FocusNode();
          final anchor = GlobalKey();
          addTearDown(focus.dispose);
          ComposerSendAction? selected;
          await tester.pumpWidget(
            MaterialApp(
              theme: voiceTestTheme(),
              localizationsDelegates: AppLocalizations.localizationsDelegates,
              supportedLocales: AppLocalizations.supportedLocales,
              home: Builder(
                builder: (context) => Scaffold(
                  body: Align(
                    alignment: Alignment.bottomRight,
                    child: TextButton(
                      key: anchor,
                      focusNode: focus,
                      onPressed: () async {
                        selected = await showComposerSendMenu(
                          context,
                          anchorContext: anchor.currentContext,
                          isDm: isDm,
                          scheduledMessagesEnabled: true,
                        );
                      },
                      child: const Text('options'),
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
          final menu = find.byKey(ComposerSendMenu.menuKey);
          expect(menu, findsOneWidget);
          expect(
            find.byType(BottomSheet),
            width < 600 ? findsOneWidget : findsNothing,
          );
          expect(
            find.byKey(const Key('composer_send_online')),
            isDm ? findsOneWidget : findsNothing,
          );
          if (width > 600) expect(tester.getSize(menu).width, 260);
          bool focused(String key) {
            var found = false;
            FocusManager.instance.primaryFocus?.context?.visitAncestorElements((
              element,
            ) {
              if (element.widget.key == Key(key)) found = true;
              return !found;
            });
            return found;
          }

          expect(focused('composer_send_silent'), isTrue);
          await tester.sendKeyDownEvent(LogicalKeyboardKey.shiftLeft);
          await tester.sendKeyEvent(LogicalKeyboardKey.tab);
          await tester.sendKeyUpEvent(LogicalKeyboardKey.shiftLeft);
          await tester.pump();
          expect(
            focused(isDm ? 'composer_send_online' : 'composer_send_schedule'),
            isTrue,
          );
          await tester.sendKeyEvent(LogicalKeyboardKey.tab);
          await tester.pump();
          expect(focused('composer_send_silent'), isTrue);
          expect(tester.takeException(), isNull);
          await tester.sendKeyEvent(LogicalKeyboardKey.escape);
          await tester.pumpAndSettle();
          expect(menu, findsNothing);
          expect(focus.hasFocus, isTrue);
          expect(selected, isNull);
          await tester.tap(find.text('options'));
          await tester.pumpAndSettle();
          await tester.tap(find.byKey(const Key('composer_send_silent')));
          await tester.pumpAndSettle();
          expect(selected, ComposerSendAction.silent);
          expect(focus.hasFocus, isTrue);
        },
      );
    }
  }

  testWidgets('deployment gate hides both scheduling actions', (tester) async {
    await tester.pumpWidget(
      MaterialApp(
        theme: voiceTestTheme(),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: Scaffold(
          body: ComposerSendMenu(
            isDm: true,
            scheduledMessagesEnabled: false,
            onSelected: (_) {},
          ),
        ),
      ),
    );
    expect(find.byKey(const Key('composer_send_silent')), findsOneWidget);
    expect(find.byKey(const Key('composer_send_schedule')), findsNothing);
    expect(find.byKey(const Key('composer_send_online')), findsNothing);
  });
}
