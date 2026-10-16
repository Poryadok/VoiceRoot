import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/ui/chat/composer_panels.dart';

import 'support/voice_test_theme.dart';

void main() {
  for (final left in [360.0, 1220.0]) {
    testWidgets(
      'desktop attach popup follows trigger at $left and traps focus',
      (tester) async {
        tester.view.physicalSize = const Size(1280, 800);
        tester.view.devicePixelRatio = 1;
        addTearDown(tester.view.resetPhysicalSize);
        addTearDown(tester.view.resetDevicePixelRatio);
        final trigger = GlobalKey();
        final focus = FocusNode();
        addTearDown(focus.dispose);
        ComposerAttachAction? picked;
        var dismissals = 0;
        await tester.pumpWidget(
          MaterialApp(
            theme: voiceTestTheme(),
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: Builder(
              builder: (context) => Scaffold(
                body: Stack(
                  children: [
                    Positioned(
                      left: left,
                      top: 700,
                      child: IconButton(
                        key: trigger,
                        focusNode: focus,
                        icon: const Icon(Icons.attach_file),
                        onPressed: () async {
                          picked = await showComposerAttachMenu(
                            context,
                            anchorContext: trigger.currentContext,
                            onDismiss: () {
                              dismissals++;
                              focus.requestFocus();
                            },
                          );
                        },
                      ),
                    ),
                  ],
                ),
              ),
            ),
          ),
        );
        await tester.pumpAndSettle();
        focus.requestFocus();
        await tester.pump();
        await tester.sendKeyEvent(LogicalKeyboardKey.enter);
        await tester.pumpAndSettle();
        final popup = tester.getRect(
          find.byKey(ComposerAttachMenuBody.menuKey),
        );
        expect(popup.width, 260);
        expect(popup.bottom, 692);
        expect(popup.left, left == 360 ? 360 : 1012);
        expect(popup.right, lessThanOrEqualTo(1272));
        final photo = find.byKey(const Key('composer_attach_photo_or_video'));
        final document = find.byKey(const Key('composer_attach_document'));
        bool focused(Finder item) {
          final key = tester.widget(item).key;
          var inside = false;
          FocusManager.instance.primaryFocus?.context?.visitAncestorElements((
            element,
          ) {
            if (element.widget.key == key) inside = true;
            return !inside;
          });
          return inside;
        }

        expect(focused(photo), isTrue);
        await tester.sendKeyEvent(LogicalKeyboardKey.tab);
        await tester.pump();
        expect(focused(document), isTrue);
        await tester.sendKeyEvent(LogicalKeyboardKey.tab);
        await tester.pump();
        expect(focused(photo), isTrue);
        await tester.sendKeyDownEvent(LogicalKeyboardKey.shiftLeft);
        await tester.sendKeyEvent(LogicalKeyboardKey.tab);
        await tester.sendKeyUpEvent(LogicalKeyboardKey.shiftLeft);
        await tester.pump();
        expect(focused(document), isTrue);
        await tester.sendKeyEvent(LogicalKeyboardKey.escape);
        await tester.pumpAndSettle();
        expect(picked, isNull);
        expect(dismissals, 1);
        expect(focus.hasFocus, isTrue);
        await tester.tap(find.byKey(trigger));
        await tester.pumpAndSettle();
        await tester.tapAt(const Offset(50, 50));
        await tester.pumpAndSettle();
        expect(dismissals, 2);
        expect(focus.hasFocus, isTrue);
      },
    );
  }

  testWidgets('mobile attach remains a bottom sheet with photo selection', (
    tester,
  ) async {
    tester.view.physicalSize = const Size(390, 844);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    ComposerAttachAction? picked;
    await tester.pumpWidget(
      MaterialApp(
        theme: voiceTestTheme(),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: Builder(
          builder: (context) => Scaffold(
            body: TextButton(
              onPressed: () async {
                picked = await showComposerAttachMenu(context);
              },
              child: const Text('attach'),
            ),
          ),
        ),
      ),
    );
    await tester.tap(find.text('attach'));
    await tester.pumpAndSettle();
    expect(find.byType(BottomSheet), findsOneWidget);
    await tester.tap(find.byKey(const Key('composer_attach_photo_or_video')));
    await tester.pumpAndSettle();
    expect(picked, ComposerAttachAction.photoOrVideo);
  });

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
