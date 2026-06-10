import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/spaces_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/space_providers.dart';
import 'package:voice_frontend/ui/space/space_invites_sheet.dart';

import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

void main() {
  final sampleInvites = [
    SpaceInvite(
      id: 'inv-1',
      spaceId: 'space-1',
      code: 'abc123',
      creatorProfileId: 'owner',
      useCount: 1,
      maxUses: 5,
      createdAt: DateTime.utc(2026, 1, 1),
    ),
  ];

  testWidgets('SpaceInvitesSheet lists invites with copy and revoke actions',
      (tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceThemeTestOverrides(),
          spaceInvitesProvider('space-1').overrideWith(
            (ref) async => sampleInvites,
          ),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(
            body: SpaceInvitesSheet(spaceId: 'space-1'),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('abc123'), findsOneWidget);
    expect(find.byKey(const Key('copy_invite_inv-1')), findsOneWidget);
    expect(find.byKey(const Key('revoke_invite_inv-1')), findsOneWidget);
    expect(find.byKey(SpaceInvitesSheet.createButtonKey), findsOneWidget);
  });
}
