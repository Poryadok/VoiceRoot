import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/ui/shell/side_panel.dart';
import 'package:voice_frontend/ui/chat/composer_panels.dart';

import 'support/voice_test_theme.dart';
import 'support/auth_test_overrides.dart';

void main() {
  setUp(() => SharedPreferences.setMockInitialValues({}));

  testWidgets('actual emoji selections populate recents on reopening', (
    tester,
  ) async {
    final container = ProviderContainer(
      overrides: [
        authSessionStorageProvider.overrideWithValue(
          InMemoryAuthSessionStorage(),
        ),
        authControllerProvider.overrideWith(authenticatedAuthController),
      ],
    );
    addTearDown(container.dispose);
    final picked = <String>[];
    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: Consumer(
            builder: (context, ref, _) => Scaffold(
              body: TextButton(
                onPressed: () =>
                    openEmojiPanel(context, ref, onSelected: picked.add),
                child: const Text('open'),
              ),
            ),
          ),
        ),
      ),
    );
    await tester.tap(find.text('open'));
    await tester.pumpAndSettle();
    expect(find.text('Recent'), findsNothing);
    await tester.tap(find.byKey(const Key('composer_emoji_😀')));
    await tester.pumpAndSettle();
    expect(picked, ['😀']);
    await tester.tap(find.text('open'));
    await tester.pumpAndSettle();
    expect(find.text('Recent'), findsOneWidget);
    expect(find.byKey(const Key('composer_recent_😀')), findsOneWidget);
    await tester.tap(find.byKey(const Key('composer_recent_😀')));
    await tester.pumpAndSettle();
    expect(picked, ['😀', '😀']);
    expect(container.read(composerRecentEmojiProvider('prof-test')), ['😀']);
    expect(
      container.read(composerRecentEmojiProvider('other-profile')),
      isEmpty,
    );
    await tester.tap(find.text('open'));
    await tester.pumpAndSettle();
    container.read(authControllerProvider.notifier).state = const AuthState();
    await tester.tap(find.byKey(const Key('composer_emoji_😂')));
    await tester.pumpAndSettle();
    expect(picked, [
      '😀',
      '😀',
    ], reason: 'An old overlay must not insert into another session');
    expect(container.read(composerRecentEmojiProvider('prof-test')), ['😀']);
  });

  for (final width in [1280.0, 390.0]) {
    testWidgets('emoji grid fits $width and supports keyboard dismissal', (
      tester,
    ) async {
      tester.view.physicalSize = Size(width, 800);
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      final triggerFocus = FocusNode();
      addTearDown(triggerFocus.dispose);
      var dismissals = 0;
      String? selected;
      await tester.pumpWidget(
        ProviderScope(
          child: MaterialApp(
            theme: voiceTestTheme(),
            locale: const Locale('en'),
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: Builder(
              builder: (context) => Scaffold(
                body: Align(
                  alignment: Alignment.bottomRight,
                  child: TextButton(
                    focusNode: triggerFocus,
                    onPressed: () => showComposerEmojiPanel(
                      context,
                      onSelected: (value) => selected = value,
                      onDismiss: () {
                        dismissals++;
                        triggerFocus.requestFocus();
                      },
                    ),
                    child: const Text('open emoji'),
                  ),
                ),
              ),
            ),
          ),
        ),
      );
      triggerFocus.requestFocus();
      await tester.pump();
      await tester.sendKeyEvent(LogicalKeyboardKey.enter);
      await tester.pumpAndSettle();
      final panel = find.byKey(ComposerEmojiPanelBody.panelKey);
      expect(tester.getSize(panel).width, width < 600 ? width : 360);
      expect(find.byKey(const Key('composer_emoji_😀')), findsOneWidget);
      expect(
        find.byType(BottomSheet),
        width < 600 ? findsOneWidget : findsNothing,
      );
      bool emojiFocused(String value) {
        var found = false;
        FocusManager.instance.primaryFocus?.context?.visitAncestorElements((
          element,
        ) {
          if (element.widget.key == Key('composer_emoji_$value')) found = true;
          return !found;
        });
        return found;
      }

      expect(emojiFocused('😀'), isTrue);
      await tester.sendKeyDownEvent(LogicalKeyboardKey.shiftLeft);
      await tester.sendKeyEvent(LogicalKeyboardKey.tab);
      await tester.sendKeyUpEvent(LogicalKeyboardKey.shiftLeft);
      await tester.pumpAndSettle();
      expect(emojiFocused('😢'), isTrue);
      await tester.sendKeyEvent(LogicalKeyboardKey.tab);
      await tester.pumpAndSettle();
      expect(emojiFocused('😀'), isTrue);
      expect(tester.takeException(), isNull);
      await tester.sendKeyEvent(LogicalKeyboardKey.escape);
      await tester.pumpAndSettle();
      expect(panel, findsNothing);
      expect(selected, isNull);
      expect(dismissals, 1);
      expect(triggerFocus.hasFocus, isTrue);
      await tester.tap(find.text('open emoji'));
      await tester.pumpAndSettle();
      await tester.tapAt(const Offset(1, 1));
      await tester.pumpAndSettle();
      expect(panel, findsNothing);
      expect(dismissals, 2);
      expect(triggerFocus.hasFocus, isTrue);
    });
  }

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
        final music = find.byKey(const Key('composer_attach_music'));
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
        expect(focused(music), isTrue);
        await tester.sendKeyEvent(LogicalKeyboardKey.tab);
        await tester.pump();
        expect(focused(photo), isTrue);
        await tester.sendKeyDownEvent(LogicalKeyboardKey.shiftLeft);
        await tester.sendKeyEvent(LogicalKeyboardKey.tab);
        await tester.sendKeyUpEvent(LogicalKeyboardKey.shiftLeft);
        await tester.pump();
        expect(focused(music), isTrue);
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
      ProviderScope(
        child: MaterialApp(
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

  testWidgets(
    'composer attach menu exposes Music without replacing existing actions',
    (tester) async {
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
      expect(find.text('Music'), findsOneWidget);

      await tester.tap(find.byKey(const Key('composer_attach_document')));
      await tester.pumpAndSettle();

      expect(picked, ComposerAttachAction.document);
      await tester.tap(find.text('attach'));
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const Key('composer_attach_music')));
      await tester.pumpAndSettle();
      expect(picked, ComposerAttachAction.music);
    },
  );
}
