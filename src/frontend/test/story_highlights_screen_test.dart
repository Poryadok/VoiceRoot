import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/stories_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/stories_providers.dart';
import 'package:voice_frontend/ui/stories/story_highlights_screen.dart';

import 'support/auth_test_overrides.dart';
import 'support/voice_test_theme.dart';

void main() {
  Widget wrap(Widget child) {
    return ProviderScope(
      overrides: [
        ...voiceAppTestOverrides(
          client: MockClient((_) async => throw UnimplementedError()),
        ),
        profileHighlightsProvider('prof-test').overrideWith(
          (ref) async => const [
            HighlightData(
              id: 'hl-1',
              name: 'Clips',
              storyIds: ['story-1', 'story-2'],
            ),
          ],
        ),
      ],
      child: MaterialApp(
        theme: voiceTestTheme(),
        locale: const Locale('en'),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: child,
      ),
    );
  }

  testWidgets('StoryHighlightsScreen lists highlights with create FAB',
      (tester) async {
    await tester.pumpWidget(
      wrap(const StoryHighlightsScreen(profileId: 'prof-test')),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('story_highlights_screen')), findsOneWidget);
    expect(find.text('Clips'), findsOneWidget);
    expect(find.byKey(const Key('story_highlight_create_fab')), findsOneWidget);
  });
}
