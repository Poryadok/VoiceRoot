import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/ui/core/chat_author_label.dart';
import 'package:voice_frontend/ui/core/verified_badge.dart';

import 'support/voice_test_theme.dart';

void main() {
  test('VerifiedBadge widget file exists for Phase 13', () {
    expect(
      File('lib/ui/core/verified_badge.dart').existsSync(),
      isTrue,
      reason: 'system icon badge widget required by verification.md',
    );
  });

  test('ChatAuthorLabel supports verificationType parameter', () {
    const label = ChatAuthorLabel(
      displayName: 'Alice',
      verificationType: 'personal',
    );
    expect(label.verificationType, 'personal');
  });

  testWidgets('VerifiedBadge personal verification semantic label', (tester) async {
    await tester.pumpWidget(
      MaterialApp(
        theme: voiceTestTheme(),
        locale: const Locale('en'),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: const Scaffold(
          body: VerifiedBadge(
            verificationType: 'personal',
            semanticLabel: 'Verified',
          ),
        ),
      ),
    );

    expect(find.byKey(VerifiedBadge.personalKey), findsOneWidget);
    expect(find.byIcon(Icons.verified), findsOneWidget);
  });

  testWidgets('ChatAuthorLabel shows verified badge for personal verification', (
    tester,
  ) async {
    await tester.pumpWidget(
      MaterialApp(
        theme: voiceTestTheme(),
        locale: const Locale('en'),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: const Scaffold(
          body: ChatAuthorLabel(
            displayName: 'Streamer',
            verificationType: 'personal',
            verifiedBadgeSemanticLabel: 'Verified',
          ),
        ),
      ),
    );

    expect(find.text('Streamer'), findsOneWidget);
    expect(find.byKey(VerifiedBadge.personalKey), findsOneWidget);
    expect(find.byKey(const Key('premium_badge')), findsNothing);
  });
}
