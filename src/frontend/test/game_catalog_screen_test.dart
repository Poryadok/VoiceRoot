import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:voice_frontend/backend/matchmaking_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/matchmaking_providers.dart';
import 'package:voice_frontend/ui/matchmaking/game_catalog_screen.dart';
import 'package:voice_frontend/ui/matchmaking/game_detail_screen.dart';

import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

CatalogGame _dotaGame() {
  return CatalogGame(
    id: 'g1',
    name: 'Dota 2',
    status: 'active',
    config: GameConfig.fromJson({
      'genre': 'MOBA',
      'regions': ['eu'],
      'modes': [
        {
          'name': '5v5 Ranked',
          'slots': 10,
          'party_size_min': 1,
          'party_size_max': 5,
          'roles': [
            {'name': 'Carry', 'required': true},
            {'name': 'Support', 'required': false},
          ],
          'ranks': [
            {'name': 'Herald', 'value': 0},
            {'name': 'Ancient', 'value': 3850},
          ],
        },
      ],
    }),
  );
}

Widget _wrap(Widget child, List<Override> overrides) {
  return ProviderScope(
    overrides: [...voiceThemeTestOverrides(), ...overrides],
    child: MaterialApp(
      theme: voiceTestTheme(),
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      home: child,
    ),
  );
}

void main() {
  testWidgets('catalog list renders game cards', (tester) async {
    await tester.pumpWidget(
      _wrap(
        const GameCatalogScreen(),
        [
          gameCatalogSearchProvider.overrideWith(
            (ref) async => GameListData(games: [_dotaGame()]),
          ),
        ],
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(GameCatalogScreen.listKey), findsOneWidget);
    expect(find.text('Dota 2'), findsOneWidget);
  });

  testWidgets('detail shows mode roles and ranks', (tester) async {
    await tester.pumpWidget(
      MaterialApp(
        theme: voiceTestTheme(),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: GameDetailScreen(game: _dotaGame()),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(GameDetailScreen.rolesSectionKey), findsOneWidget);
    expect(find.byKey(GameDetailScreen.ranksSectionKey), findsOneWidget);
    expect(find.text('Carry'), findsOneWidget);
    expect(find.textContaining('Ancient'), findsOneWidget);
  });
}
