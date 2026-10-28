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
  testWidgets('popular preview expands the complete first catalog page', (
    tester,
  ) async {
    final games = List.generate(
      5,
      (i) => CatalogGame(
        id: 'game$i',
        name: 'Game $i',
        status: 'active',
        config: _dotaGame().config,
      ),
    );
    await tester.pumpWidget(
      _wrap(const GameCatalogScreen(), [
        gameCatalogSearchProvider.overrideWith(
          (ref) async => GameListData(games: games),
        ),
      ]),
    );
    await tester.pumpAndSettle();
    expect(find.text('Popular'), findsOneWidget);
    expect(find.text('Game 3'), findsNothing);
    await tester.tap(find.text('Browse all'));
    await tester.pumpAndSettle();
    expect(find.text('Game 3'), findsOneWidget);
    expect(find.text('Game 4'), findsOneWidget);
  });

  testWidgets(
    'search restores its query and clear resets the actual provider',
    (tester) async {
      await tester.pumpWidget(
        _wrap(const GameCatalogScreen(), [
          gameCatalogSearchQueryProvider.overrideWith((ref) => 'Dota'),
          gameCatalogSearchProvider.overrideWith(
            (ref) async => GameListData(games: [_dotaGame()]),
          ),
        ]),
      );
      await tester.pumpAndSettle();
      expect(
        tester
            .widget<TextField>(find.byKey(GameCatalogScreen.searchFieldKey))
            .controller!
            .text,
        'Dota',
      );
      await tester.tap(find.byTooltip('Clear search'));
      await tester.pumpAndSettle();
      final container = ProviderScope.containerOf(
        tester.element(find.byType(GameCatalogScreen)),
      );
      expect(container.read(gameCatalogSearchQueryProvider), '');
      expect(find.text('Popular'), findsOneWidget);
    },
  );

  testWidgets('catalog retry requests fresh data and recovers', (tester) async {
    var attempts = 0;
    await tester.pumpWidget(
      _wrap(const GameCatalogScreen(), [
        gameCatalogSearchProvider.overrideWith((ref) async {
          if (++attempts == 1) throw Exception('catalog failed');
          return GameListData(games: [_dotaGame()]);
        }),
      ]),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.text('Try again'));
    await tester.pumpAndSettle();
    expect(attempts, 2);
    expect(find.text('Dota 2'), findsOneWidget);
  });

  testWidgets('catalog list renders game cards', (tester) async {
    await tester.pumpWidget(
      _wrap(const GameCatalogScreen(), [
        gameCatalogSearchProvider.overrideWith(
          (ref) async => GameListData(games: [_dotaGame()]),
        ),
      ]),
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

  testWidgets('shows empty state when catalog has no games', (tester) async {
    await tester.pumpWidget(
      _wrap(const GameCatalogScreen(), [
        gameCatalogSearchProvider.overrideWith(
          (ref) async => const GameListData(games: []),
        ),
      ]),
    );
    await tester.pumpAndSettle();

    expect(find.text('No games found'), findsOneWidget);
  });

  testWidgets('shows error state with retry when catalog load fails', (
    tester,
  ) async {
    await tester.pumpWidget(
      _wrap(const GameCatalogScreen(), [
        gameCatalogSearchProvider.overrideWith((ref) async {
          throw Exception('catalog failed');
        }),
      ]),
    );
    await tester.pumpAndSettle();

    expect(find.text('Could not load game catalog'), findsWidgets);
    expect(find.text('Try again'), findsOneWidget);
  });
}
