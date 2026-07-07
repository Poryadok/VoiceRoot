import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/spaces_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/space_providers.dart';
import 'package:voice_frontend/ui/space/space_rail.dart';

import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

/// Widget tests for spaces.md space rail (sidebar icons, selected space).
void main() {
  testWidgets('space icons render and tap sets selectedSpaceIdProvider', (
    tester,
  ) async {
    String? selectedSpaceId;

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceThemeTestOverrides(),
          mySpacesProvider.overrideWith((_) async {
            return const SpaceListData(
              spaces: [
                VoiceSpace(
                  id: 'space-1',
                  name: 'Raid HQ',
                  iconUrl: 'https://cdn.voice.gg/spaces/1.webp',
                  visibility: 'private',
                  ownerProfileId: 'profile-owner',
                ),
                VoiceSpace(
                  id: 'space-2',
                  name: 'Study',
                  visibility: 'private',
                  ownerProfileId: 'profile-owner',
                ),
              ],
            );
          }),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: Consumer(
            builder: (context, ref, _) {
              ref.listen<String?>(selectedSpaceIdProvider, (_, next) {
                selectedSpaceId = next;
              });
              return const Scaffold(body: SpaceRail());
            },
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(SpaceRail.railKey), findsOneWidget);
    expect(find.byKey(SpaceRail.spaceIconKey('space-1')), findsOneWidget);
    expect(find.byKey(SpaceRail.spaceIconKey('space-2')), findsOneWidget);

    await tester.tap(find.byKey(SpaceRail.spaceIconKey('space-1')));
    await tester.pumpAndSettle();

    expect(selectedSpaceId, 'space-1');
  });
}
