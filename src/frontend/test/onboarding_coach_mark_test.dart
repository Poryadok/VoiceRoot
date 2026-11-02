import 'dart:io';
import 'dart:ui' as ui;

import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/ui/onboarding/onboarding_coach_mark.dart';
import 'package:voice_frontend/theme/voice_theme.dart';
import 'package:voice_frontend/theme/voice_token_catalog.dart';

import 'support/voice_test_theme.dart';

void main() {
  for (final size in [const Size(1280, 800), const Size(390, 844)]) {
    testWidgets('coachmark fits $size and exposes tour progress', (
      tester,
    ) async {
      final semantics = tester.ensureSemantics();
      tester.view.physicalSize = size;
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      final anchor = GlobalKey();
      final capture = GlobalKey();
      final triggerFocus = FocusNode();
      addTearDown(triggerFocus.dispose);
      late BuildContext overlayContext;
      var theme = voiceTestTheme();
      final directory = Platform.environment['VOICE_COACHMARK_CAPTURE_DIR'];
      if (directory != null) {
        debugDisableShadows = false;
        theme = (await tester.runAsync(() async {
          await (FontLoader(VoiceTheme.fontFamily)
                ..addFont(rootBundle.load('assets/fonts/NotoSans-Regular.ttf'))
                ..addFont(
                  rootBundle.load('assets/fonts/NotoSans-SemiBold.ttf'),
                ))
              .load();
          final catalog = await VoiceTokenCatalog.load();
          return VoiceTheme.build(
            catalog: catalog,
            mode: VoiceThemeMode.light,
            profileAccent: catalog.profileAccentAt(0),
          );
        }))!;
      }
      await tester.pumpWidget(
        RepaintBoundary(
          key: capture,
          child: MaterialApp(
            debugShowCheckedModeBanner: false,
            theme: theme,
            home: Builder(
              builder: (context) {
                overlayContext = context;
                return Scaffold(
                  body: Align(
                    alignment: Alignment.centerLeft,
                    child: SizedBox(
                      key: anchor,
                      width: 56,
                      height: 72,
                      child: TextButton(
                        focusNode: triggerFocus,
                        onPressed: () {},
                        child: const Text('Chats'),
                      ),
                    ),
                  ),
                );
              },
            ),
          ),
        ),
      );
      triggerFocus.requestFocus();
      await tester.pump();
      var continued = false;
      late OverlayEntry entry;
      entry = OnboardingCoachMark.show(
        context: overlayContext,
        anchorKey: anchor,
        title: 'Discover spaces',
        body: 'Find communities and voice rooms.',
        continueLabel: 'Find a space',
        skipLabel: 'Skip',
        onContinue: () {
          continued = true;
          entry.remove();
        },
        onSkip: () {},
        step: 2,
        totalSteps: 4,
      )!;
      addTearDown(() {
        if (entry.mounted) entry.remove();
      });
      await tester.pumpAndSettle();
      final card = tester.getRect(find.byKey(OnboardingCoachMark.cardKey));
      expect(card.left, greaterThanOrEqualTo(8));
      expect(card.right, lessThanOrEqualTo(size.width - 8));
      expect(card.bottom, lessThanOrEqualTo(size.height - 8));
      expect(find.bySemanticsLabel('2 / 4'), findsOneWidget);
      if (directory != null) {
        await tester.runAsync(() async {
          final boundary =
              capture.currentContext!.findRenderObject()!
                  as RenderRepaintBoundary;
          final image = await boundary.toImage();
          final bytes = await image.toByteData(format: ui.ImageByteFormat.png);
          await Directory(directory).create(recursive: true);
          await File(
            '$directory/coach-${size.width.toInt()}.png',
          ).writeAsBytes(bytes!.buffer.asUint8List());
          image.dispose();
        });
      }
      await tester.sendKeyEvent(LogicalKeyboardKey.tab);
      await tester.sendKeyEvent(LogicalKeyboardKey.tab);
      await tester.sendKeyEvent(LogicalKeyboardKey.enter);
      await tester.pump();
      expect(continued, isTrue);
      await tester.pump();
      expect(triggerFocus.hasFocus, isTrue);
      expect(tester.takeException(), isNull);
      debugDisableShadows = true;
      semantics.dispose();
    });
  }
}
