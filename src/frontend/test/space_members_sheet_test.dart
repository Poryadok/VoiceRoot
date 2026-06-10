import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/spaces_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/space_providers.dart';
import 'package:voice_frontend/ui/space/space_members_sheet.dart';

import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

void main() {
  final sampleMembers = [
    SpaceMemberRosterEntry(
      profileId: 'owner',
      roleNames: const ['Owner'],
      joinedAt: DateTime.utc(2026, 1, 1),
    ),
    SpaceMemberRosterEntry(
      profileId: 'member-1',
      roleNames: const ['Member'],
      joinedAt: DateTime.utc(2026, 1, 2),
    ),
  ];

  testWidgets('SpaceMembersSheet lists members with role badges', (tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceThemeTestOverrides(),
          spaceMembersProvider('space-1').overrideWith((ref) async => sampleMembers),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(
            body: SpaceMembersSheet(spaceId: 'space-1'),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Owner'), findsOneWidget);
    expect(find.text('Member'), findsWidgets);
    expect(find.byKey(const Key('kick_member_member-1')), findsOneWidget);
    expect(find.byKey(const Key('assign_role_member-1')), findsOneWidget);
  });
}
