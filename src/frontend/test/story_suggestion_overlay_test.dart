import 'dart:convert';
import 'dart:io';
import 'dart:ui' as ui;

import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter/services.dart';
import 'package:voice_frontend/ui/matchmaking/story_suggestion_overlay.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/backend/matchmaking_client.dart';
import 'package:voice_frontend/state/matchmaking_rating_controller.dart';
import 'package:voice_frontend/ui/stories/story_create_screen.dart';

import 'support/auth_test_overrides.dart';
import 'support/voice_test_theme.dart';

void main() {
  setUpAll(() async {
    await (FontLoader(
      'MaterialIcons',
    )..addFont(rootBundle.load('fonts/MaterialIcons-Regular.otf'))).load();
    await (FontLoader('Noto Sans')
          ..addFont(rootBundle.load('assets/fonts/NotoSans-Regular.ttf'))
          ..addFont(rootBundle.load('assets/fonts/NotoSans-SemiBold.ttf'))
          ..addFont(rootBundle.load('assets/fonts/NotoSans-Bold.ttf')))
        .load();
  });
  test('rating completion offers draft and dismiss clears the match', () {
    final container = ProviderContainer();
    addTearDown(container.dispose);
    final controller = container.read(
      matchmakingRatingControllerProvider.notifier,
    );
    const match = MatchData(
      id: 'match',
      gameId: 'game',
      mode: 'Duo',
      region: 'eu',
      status: 'completed',
      profileIds: ['self', 'mate'],
    );
    controller.showRatingForMatch(match);
    expect(
      container.read(matchmakingRatingControllerProvider).suggestStory,
      isFalse,
    );
    controller.finishRating();
    expect(
      container.read(matchmakingRatingControllerProvider).suggestStory,
      isTrue,
    );
    expect(
      container.read(matchmakingRatingControllerProvider).match,
      same(match),
    );
    controller.clear();
    expect(container.read(matchmakingRatingControllerProvider).match, isNull);
    controller.finishRating();
    expect(
      container.read(matchmakingRatingControllerProvider).suggestStory,
      isFalse,
    );
  });
  testWidgets('post-match draft survives publish failure and retry', (
    tester,
  ) async {
    final requests = <Map<String, dynamic>>[];
    final client = MockClient((request) async {
      if (request.method == 'POST' && request.url.path == '/api/v1/stories') {
        requests.add(jsonDecode(request.body) as Map<String, dynamic>);
        return http.Response('{}', 503);
      }
      return http.Response('{}', 404);
    });
    await tester.pumpWidget(
      ProviderScope(
        overrides: voiceAppTestOverrides(client: client),
        child: MaterialApp(
          theme: voiceTestTheme(),
          debugShowCheckedModeBanner: false,
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const StoryCreateScreen(
            initialText: 'Match completed · Valorant',
            initialGameTag: 'valorant',
            initialGameTagLabel: 'Valorant',
            initialMentionProfileIds: ['maria', 'dima'],
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.text('Match completed · Valorant'), findsOneWidget);
    await tester.enterText(
      find.byKey(StoryCreateScreen.textFieldKey),
      'Great match',
    );
    await tester.drag(find.byType(ListView), const Offset(0, -700));
    await tester.pumpAndSettle();
    await tester.ensureVisible(find.byKey(StoryCreateScreen.submitKey));
    await tester.tap(find.byKey(StoryCreateScreen.submitKey));
    await tester.pumpAndSettle();
    expect(requests.single['game_tag'], 'valorant');
    expect(requests.single['mention_profile_ids'], ['maria', 'dima']);
    expect(requests.single['text_content'], 'Great match');
    expect(find.byKey(StoryCreateScreen.screenKey), findsOneWidget);
    await tester.drag(find.byType(ListView), const Offset(0, -300));
    await tester.pumpAndSettle();
    await tester.ensureVisible(find.byKey(StoryCreateScreen.submitKey));
    await tester.tap(find.byKey(StoryCreateScreen.submitKey));
    await tester.pumpAndSettle();
    expect(requests, hasLength(2));
    expect(requests.last, requests.first);
  });
  for (final size in [const Size(1200, 800), const Size(360, 640)]) {
    testWidgets('story suggestion actions and layout at $size', (tester) async {
      tester.view.physicalSize = size;
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      var edited = 0;
      var dismissed = 0;
      final captureKey = GlobalKey();
      await tester.pumpWidget(
        RepaintBoundary(
          key: captureKey,
          child: MaterialApp(
            debugShowCheckedModeBanner: false,
            theme: voiceTestTheme().copyWith(
              textTheme: voiceTestTheme().textTheme.apply(
                fontFamily: 'Noto Sans',
              ),
            ),
            home: Scaffold(
              body: Stack(
                children: [
                  Positioned.fill(
                    child: StorySuggestionOverlay(
                      gameName: 'Valorant',
                      teammateHandles: const ['maria', 'dima'],
                      onEdit: () => edited++,
                      onDismiss: () => dismissed++,
                    ),
                  ),
                ],
              ),
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();
      final evidenceDir = Platform.environment['VOICE_STORY_EVIDENCE_DIR'];
      if (evidenceDir != null) {
        await tester.runAsync(() async {
          final boundary =
              captureKey.currentContext!.findRenderObject()!
                  as RenderRepaintBoundary;
          final image = await boundary.toImage();
          final bytes = await image.toByteData(format: ui.ImageByteFormat.png);
          await Directory(evidenceDir).create(recursive: true);
          await File(
            '$evidenceDir/story-suggestion-${size.width.toInt()}.png',
          ).writeAsBytes(bytes!.buffer.asUint8List());
          image.dispose();
        });
      }
      expect(find.text('Valorant'), findsOneWidget);
      expect(find.text('@maria · @dima'), findsOneWidget);
      expect(find.text('Match completed'), findsOneWidget);
      expect(find.textContaining('13'), findsNothing);
      await tester.tap(find.byKey(StorySuggestionOverlay.editKey));
      expect(edited, 1);
      await tester.tap(find.byTooltip('Dismiss'));
      expect(dismissed, 1);
      await tester.sendKeyEvent(LogicalKeyboardKey.escape);
      expect(dismissed, 2);
      expect(tester.takeException(), isNull);
    });
  }
}
