import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/spaces_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/shell/app_space_shell.dart';
import 'package:voice_frontend/state/space_providers.dart';
import 'package:voice_frontend/ui/space/space_tree_panel.dart';

import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

SpaceTreeData _emptyTree() {
  return const SpaceTreeData(
    categories: [],
    nodes: [],
    voiceRooms: [],
  );
}

/// PLAN Phase 5: authenticated shell shows space tree in list column when a space is selected.
void main() {
  testWidgets('selected space shows SpaceTreePanel in list column', (
    tester,
  ) async {
    await tester.binding.setSurfaceSize(const Size(1280, 800));
    addTearDown(() => tester.binding.setSurfaceSize(null));

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceThemeTestOverrides(),
          selectedSpaceIdProvider.overrideWith((_) => 'space-1'),
          spaceTreeProvider('space-1').overrideWith((_) async => _emptyTree()),
          mySpacesProvider.overrideWith((_) async {
            return const SpaceListData(
              spaces: [
                VoiceSpace(
                  id: 'space-1',
                  name: 'Raid HQ',
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
          home: const Scaffold(body: AppSpaceShell()),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(AppSpaceShell.shellKey), findsOneWidget);
    expect(find.byKey(SpaceTreePanel.panelKey), findsOneWidget);
  });
}
