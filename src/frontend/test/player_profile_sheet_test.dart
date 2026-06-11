import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:voice_frontend/backend/matchmaking_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/matchmaking_providers.dart';
import 'package:voice_frontend/ui/matchmaking/player_profile_sheet.dart';

import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

CatalogGame _dotaGame() {
  return CatalogGame(
    id: 'g1',
    name: 'Dota 2',
    status: 'active',
    config: GameConfig.fromJson({
      'regions': ['eu'],
      'modes': [
        {
          'name': '5v5 Ranked',
          'slots': 10,
          'roles': [
            {'name': 'Carry', 'required': true},
          ],
          'ranks': [
            {'name': 'Herald', 'value': 0},
          ],
        },
      ],
    }),
  );
}

Widget _wrap(Widget child) {
  return ProviderScope(
    overrides: [
      ...voiceThemeTestOverrides(),
      myPlayerProfileProvider.overrideWith(
        (ref) async => const PlayerProfileData(entries: []),
      ),
      gameCatalogProvider.overrideWith(
        (ref) async => GameListData(games: [_dotaGame()]),
      ),
    ],
    child: MaterialApp(
      theme: voiceTestTheme(),
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      home: Scaffold(body: SingleChildScrollView(child: child)),
    ),
  );
}

void main() {
  testWidgets('player profile sheet shows game pickers when game preset', (tester) async {
    await tester.pumpWidget(
      _wrap(PlayerProfileSheet(initialGame: _dotaGame())),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(PlayerProfileSheet.sheetKey), findsOneWidget);
    expect(find.text('Dota 2'), findsOneWidget);
    expect(find.text('Carry'), findsOneWidget);
    expect(find.text('Herald'), findsOneWidget);
    expect(find.byKey(PlayerProfileSheet.saveButtonKey), findsOneWidget);
  });
}
