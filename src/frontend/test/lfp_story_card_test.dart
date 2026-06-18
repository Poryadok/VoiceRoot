import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/stories_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/ui/stories/lfp_story_card.dart';

import 'support/voice_test_theme.dart';

void main() {
  const story = StoryData(
    id: 'lfp-1',
    authorProfileId: 'author-1',
    type: 'text',
    textContent: 'Need duo',
    gameTag: 'dota-2',
    lfpCriteriaJson: '{"mode":"5v5"}',
    isLookingForParty: true,
    visibility: 'everyone',
  );

  testWidgets('LfpStoryCard shows join action per stories.md', (tester) async {
    await tester.pumpWidget(
      MaterialApp(
        theme: voiceTestTheme(),
        locale: const Locale('en'),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: const Scaffold(
          body: LfpStoryCard(story: story),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('lfp_story_join')), findsOneWidget);
  });
}
