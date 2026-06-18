import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/stories_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/stories_providers.dart';
import 'package:voice_frontend/ui/stories/story_archive_screen.dart';

import 'support/auth_test_overrides.dart';
import 'support/voice_test_theme.dart';

void main() {
  Widget wrap(Widget child) {
    return ProviderScope(
      overrides: [
        ...voiceAppTestOverrides(
          client: MockClient((_) async => throw UnimplementedError()),
        ),
        storyArchiveProvider.overrideWith(
          (ref) async => const [
            StoryData(
              id: 'arch-1',
              authorProfileId: 'prof-test',
              type: 'text',
              textContent: 'Expired story',
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

  testWidgets('StoryArchiveScreen shows archived story and add action',
      (tester) async {
    await tester.pumpWidget(wrap(const StoryArchiveScreen()));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('story_archive_screen')), findsOneWidget);
    expect(find.text('Expired story'), findsOneWidget);
    expect(find.byKey(const Key('story_archive_add_highlight_arch-1')),
        findsOneWidget);
  });
}
