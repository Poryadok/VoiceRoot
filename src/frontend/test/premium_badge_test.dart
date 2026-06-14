import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/ui/core/chat_author_label.dart';

import 'support/voice_test_theme.dart';

void main() {
  testWidgets('ChatAuthorLabel shows premium badge for premium tier', (tester) async {
    await tester.pumpWidget(
      MaterialApp(
        theme: voiceTestTheme(),
        locale: const Locale('en'),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: const Scaffold(
          body: ChatAuthorLabel(
            displayName: 'Premium User',
            isPremium: true,
            premiumBadgeSemanticLabel: 'Premium',
          ),
        ),
      ),
    );

    expect(find.byKey(const Key('premium_badge')), findsOneWidget);
    expect(find.text('Premium User'), findsOneWidget);
    expect(find.text('★'), findsOneWidget);
  });

  testWidgets('ChatAuthorLabel hides premium badge when not premium', (tester) async {
    await tester.pumpWidget(
      MaterialApp(
        theme: voiceTestTheme(),
        home: const Scaffold(
          body: ChatAuthorLabel(
            displayName: 'Regular User',
            isPremium: false,
          ),
        ),
      ),
    );

    expect(find.byKey(const Key('premium_badge')), findsNothing);
    expect(find.text('Regular User'), findsOneWidget);
  });
}
