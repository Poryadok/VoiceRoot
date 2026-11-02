import 'dart:io';
import 'dart:ui' as ui;

import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/matchmaking_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/ui/matchmaking/match_rating_overlay.dart';
import 'package:voice_frontend/theme/voice_theme.dart';
import 'package:voice_frontend/theme/voice_token_catalog.dart';

import 'support/voice_test_theme.dart';

Widget fixture({int count = 1, ThemeData? theme}) => MaterialApp(
  debugShowCheckedModeBanner: false,
  theme: theme ?? voiceTestTheme(),
  localizationsDelegates: AppLocalizations.localizationsDelegates,
  supportedLocales: AppLocalizations.supportedLocales,
  home: Scaffold(
    body: MatchRatingOverlay(
      match: const MatchData(
        id: 'm',
        gameId: 'g',
        mode: 'Duo',
        region: 'eu',
        status: 'completed',
        profileIds: ['self', 'p0'],
      ),
      raterProfileId: 'self',
      onDone: () {},
      teammates: List.generate(
        count,
        (i) => RatedTeammate(profileId: 'p$i', displayName: 'Player $i'),
      ),
    ),
  ),
);

void main() {
  testWidgets('keyboard traversal stays inside the rating modal', (
    tester,
  ) async {
    await tester.pumpWidget(fixture());
    await tester.pumpAndSettle();
    final scope = FocusScope.of(
      tester.element(find.byKey(MatchRatingOverlay.submitButtonKey)),
    );
    for (var step = 0; step < 20; step++) {
      await tester.sendKeyEvent(LogicalKeyboardKey.tab);
      await tester.pump();
      expect(scope.hasFocus, isTrue);
    }
  });
  testWidgets('H/V light token capture of rating and ban suggestion', (
    tester,
  ) async {
    final output = Platform.environment['VOICE_RATING_CAPTURE_DIR'];
    if (output == null || output.isEmpty) return;
    final previousShadows = debugDisableShadows;
    debugDisableShadows = false;
    await tester.runAsync(() async {
      await (FontLoader(VoiceTheme.fontFamily)
            ..addFont(rootBundle.load('assets/fonts/NotoSans-Regular.ttf'))
            ..addFont(rootBundle.load('assets/fonts/NotoSans-Medium.ttf'))
            ..addFont(rootBundle.load('assets/fonts/NotoSans-SemiBold.ttf')))
          .load();
      final icons = await File(
        '${Platform.environment['FLUTTER_ROOT']}/bin/cache/artifacts/material_fonts/MaterialIcons-Regular.otf',
      ).readAsBytes();
      await (FontLoader(
        'MaterialIcons',
      )..addFont(Future.value(ByteData.sublistView(icons)))).load();
    });
    final catalog = await VoiceTokenCatalog.load();
    final theme = await VoiceTheme.build(
      catalog: catalog,
      mode: VoiceThemeMode.light,
      profileAccent: catalog.profileAccentAt(0),
    );
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    for (final size in [const Size(1280, 800), const Size(390, 844)]) {
      tester.view.physicalSize = size;
      tester.view.devicePixelRatio = 1;
      await tester.pumpWidget(
        RepaintBoundary(
          key: const Key('capture'),
          child: fixture(count: 2, theme: theme),
        ),
      );
      await tester.pumpAndSettle();
      await tester.tap(find.byTooltip('5').at(0));
      await tester.tap(find.byTooltip('2').at(1));
      await tester.pumpAndSettle();
      expect(tester.takeException(), isNull);
      final boundary = tester.renderObject<RenderRepaintBoundary>(
        find.byKey(const Key('capture')),
      );
      await tester.runAsync(() async {
        final image = await boundary.toImage(pixelRatio: 1);
        final bytes = await image.toByteData(format: ui.ImageByteFormat.png);
        await Directory(output).create(recursive: true);
        await File(
          '$output/rating-${size.width.toInt()}.png',
        ).writeAsBytes(bytes!.buffer.asUint8List());
        image.dispose();
      });
      await tester.pumpWidget(const SizedBox.shrink());
      await tester.pumpAndSettle();
    }
    debugDisableShadows = previousShadows;
  });
  testWidgets('submit requires a rating or every teammate skipped', (
    tester,
  ) async {
    await tester.pumpWidget(fixture());
    await tester.pumpAndSettle();
    FilledButton submit() =>
        tester.widget(find.byKey(MatchRatingOverlay.submitButtonKey));
    expect(submit().onPressed, isNull);
    await tester.tap(find.byKey(MatchRatingOverlay.skipTeammateKey('p0')));
    await tester.pump();
    expect(submit().onPressed, isNotNull);
  });

  for (final size in [const Size(1280, 800), const Size(390, 844)]) {
    testWidgets('rating modal fits $size and scrolls a full squad', (
      tester,
    ) async {
      tester.view.physicalSize = size;
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      await tester.pumpWidget(fixture(count: 10));
      await tester.pumpAndSettle();
      final card = find.byKey(const Key('match_rating_dialog'));
      expect(tester.getSize(card).width, size.width > 600 ? 480 : 358);
      await tester.scrollUntilVisible(
        find.text('Player 9'),
        150,
        scrollable: find.byType(Scrollable).first,
      );
      expect(find.text('Player 9').hitTestable(), findsOneWidget);
      expect(
        find.byKey(MatchRatingOverlay.submitButtonKey).hitTestable(),
        findsOneWidget,
      );
      expect(tester.takeException(), isNull);
    });
  }

  testWidgets('low rating offers inline ban and selected star semantics', (
    tester,
  ) async {
    final semantics = tester.ensureSemantics();
    await tester.pumpWidget(fixture());
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(MatchRatingOverlay.starButtonKey(2)));
    await tester.pump();
    expect(find.byKey(const Key('match_rating_suggestion_p0')), findsOneWidget);
    expect(find.bySemanticsLabel('Player 0: 2 / 5'), findsOneWidget);
    semantics.dispose();
  });
}
