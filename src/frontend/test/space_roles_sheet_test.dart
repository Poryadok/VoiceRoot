import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:voice_frontend/backend/roles_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/space_providers.dart';
import 'package:voice_frontend/ui/space/space_roles_sheet.dart';

import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

void main() {
  testWidgets('SpaceRolesSheet lists roles and create button when allowed', (tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceThemeTestOverrides(),
          spaceRolesProvider('space-1').overrideWith((ref) async => [
            const SpaceRole(
              id: 'r1',
              spaceId: 'space-1',
              name: 'Owner',
              position: 4,
              managed: true,
            ),
            const SpaceRole(
              id: 'r2',
              spaceId: 'space-1',
              name: 'Raid Leader',
              position: 2,
            ),
          ]),
          defaultJoinRoleProvider('space-1').overrideWith((ref) async => const SpaceRole(
            id: 'r3',
            spaceId: 'space-1',
            name: 'Member',
          )),
          spacePermissionProvider.overrideWith((ref, query) async => true),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(body: SpaceRolesSheet(spaceId: 'space-1')),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(SpaceRolesSheet.sheetKey), findsOneWidget);
    expect(find.text('Raid Leader'), findsOneWidget);
    expect(find.byKey(const Key('create_space_role')), findsOneWidget);
  });
}
