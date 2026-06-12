import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/matchmaking_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/matchmaking_providers.dart';
import 'package:voice_frontend/state/matchmaking_search_controller.dart';
import 'package:voice_frontend/ui/matchmaking/queue_search_screen.dart';

import 'support/auth_test_overrides.dart';
import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

CatalogGame _valorantGame() {
  return CatalogGame(
    id: 'g-val',
    name: 'Valorant',
    status: 'active',
    config: GameConfig(
      regions: const ['eu', 'na'],
      modes: [
        GameMode(
          name: 'Competitive',
          slots: 10,
          partySizeMin: 1,
          partySizeMax: 5,
          rolesRequired: true,
          rankRequired: true,
          roles: const [GameRole(name: 'Duelist', required: true)],
          ranks: const [
            GameRank(name: 'Iron', value: 0),
            GameRank(name: 'Gold', value: 10),
          ],
        ),
      ],
    ),
  );
}

void main() {
  testWidgets('queue search form shows start button', (tester) async {
    final game = _valorantGame();
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceThemeTestOverrides(),
          authControllerProvider.overrideWith(authenticatedAuthController),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: QueueSearchScreen(game: game, mode: game.config.modes.first),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(QueueSearchScreen.startButtonKey), findsOneWidget);
    expect(find.byKey(QueueSearchScreen.searchingStateKey), findsNothing);
  });

  testWidgets('shows nudge banner while searching', (tester) async {
    final game = _valorantGame();
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceThemeTestOverrides(),
          authControllerProvider.overrideWith(authenticatedAuthController),
          activeSearchSessionProvider.overrideWith((ref) => SearchSessionData(
                id: 'sess-1',
                profileId: 'p1',
                gameId: game.id,
                mode: game.config.modes.first.name,
                criteriaJson: '{}',
                status: 'searching',
              )),
          matchmakingSearchControllerProvider.overrideWith(
            () => _NudgeSearchController(),
          ),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: QueueSearchScreen(game: game, mode: game.config.modes.first),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(QueueSearchScreen.nudgeBannerKey), findsOneWidget);
    expect(find.byKey(QueueSearchScreen.searchingStateKey), findsOneWidget);
  });
}

class _NudgeSearchController extends MatchmakingSearchController {
  @override
  MatchmakingSearchState build() => const MatchmakingSearchState(nudgeVisible: true);
}

