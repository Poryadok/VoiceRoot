import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/testing.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/ui/stories/story_create_screen.dart';

import 'support/auth_test_overrides.dart';
import 'support/voice_test_theme.dart';

void main() {
  Widget wrap(Widget child) {
    return ProviderScope(
      overrides: voiceAppTestOverrides(
        client: MockClient((_) async => throw UnimplementedError()),
      ),
      child: MaterialApp(
        theme: voiceTestTheme(),
        locale: const Locale('en'),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: child,
      ),
    );
  }

  testWidgets('StoryCreateScreen shows mention picker for @username', (tester) async {
    await tester.pumpWidget(wrap(const StoryCreateScreen()));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('story_create_mention_picker')), findsOneWidget);
  });

  testWidgets('StoryCreateScreen shows visibility audience selector', (tester) async {
    await tester.pumpWidget(wrap(const StoryCreateScreen()));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('story_create_visibility')), findsOneWidget);
  });
}
