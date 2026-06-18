import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/stories_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/stories_providers.dart';
import 'package:voice_frontend/ui/stories/highlights_section.dart';

import 'support/auth_test_overrides.dart';
import 'support/voice_test_theme.dart';

void main() {
  Widget wrap(Widget child) {
    return ProviderScope(
      overrides: [
        ...voiceAppTestOverrides(
          client: MockClient((_) async => throw UnimplementedError()),
        ),
        profileHighlightsProvider('profile-1').overrideWith(
          (ref) async => const [
            HighlightData(
              id: 'hl-1',
              name: 'Wins',
              storyIds: ['story-1'],
            ),
          ],
        ),
      ],
      child: MaterialApp(
        theme: voiceTestTheme(),
        locale: const Locale('en'),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: Scaffold(body: child),
      ),
    );
  }

  testWidgets('HighlightsSection shows per-highlight visibility badge', (tester) async {
    await tester.pumpWidget(
      wrap(const HighlightsSection(profileId: 'profile-1')),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('highlight_visibility_badge')), findsOneWidget);
  });
}
