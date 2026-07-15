import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/guest_credentials_storage.dart';
import 'package:voice_frontend/backend/matchmaking_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/state/matchmaking_providers.dart';
import 'package:voice_frontend/state/matchmaking_search_controller.dart';
import 'package:voice_frontend/ui/matchmaking/queue_search_screen.dart';

import 'package:http/http.dart' as http;
import 'package:http/testing.dart';

import 'support/auth_test_overrides.dart';
import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

List<Override> _queueSearchRealtimeOverrides() => [
  realtimeAutoConnectProvider.overrideWithValue(false),
  realtimeHubProvider.overrideWith((ref) => _NoopRealtimeHub(ref)),
];

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
          ..._queueSearchRealtimeOverrides(),
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
          ..._queueSearchRealtimeOverrides(),
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

  testWidgets('queue search prefills criteria from player profile', (tester) async {
    final game = _valorantGame();
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceThemeTestOverrides(),
          ..._queueSearchRealtimeOverrides(),
          authControllerProvider.overrideWith(authenticatedAuthController),
          myPlayerProfileProvider.overrideWith(
            (ref) async => PlayerProfileData(
              entries: [
                PlayerGameEntry(
                  gameId: game.id,
                  region: 'na',
                  role: 'Duelist',
                  rank: 'Gold',
                ),
              ],
            ),
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

    expect(find.text('na'), findsOneWidget);
    expect(find.text('Gold'), findsWidgets);
    expect(find.text('Iron'), findsNothing);
  });

  testWidgets('queue search start and cancel flow', (tester) async {
    final game = _valorantGame();
    var searchPosts = 0;
    var cancelDeletes = 0;
    final client = MockClient((req) async {
      if (req.method == 'POST' && req.url.path == '/api/v1/matchmaking/search') {
        searchPosts++;
        return http.Response(
          jsonEncode({
            'search_session': {
              'id': 'sess-flow',
              'profile_id': 'prof-test',
              'game_id': game.id,
              'mode': game.config.modes.first.name,
              'criteria_json': '{}',
              'status': 'searching',
            },
          }),
          200,
        );
      }
      if (req.method == 'DELETE' &&
          req.url.path == '/api/v1/matchmaking/search/sess-flow') {
        cancelDeletes++;
        return http.Response('{}', 200);
      }
      return http.Response('not found', 404);
    });

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceThemeTestOverrides(),
          ..._queueSearchRealtimeOverrides(),
          authSessionStorageProvider.overrideWithValue(
            InMemoryAuthSessionStorage(),
          ),
          guestCredentialsStorageProvider.overrideWithValue(
            InMemoryGuestCredentialsStorage(),
          ),
          authControllerProvider.overrideWith(authenticatedAuthController),
          gatewayConfigProvider.overrideWithValue(
            const GatewayConfig(baseUrl: 'http://api.test'),
          ),
          httpClientProvider.overrideWithValue(client),
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

    await tester.tap(find.byKey(QueueSearchScreen.startButtonKey));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 200));

    expect(searchPosts, 1);
    expect(find.byKey(QueueSearchScreen.searchingStateKey), findsOneWidget);

    await tester.tap(find.byKey(QueueSearchScreen.cancelButtonKey));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 200));

    expect(cancelDeletes, 1);
    expect(find.byKey(QueueSearchScreen.startButtonKey), findsOneWidget);
  });

  testWidgets('queue search shows timeout card', (tester) async {
    final game = _valorantGame();
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceThemeTestOverrides(),
          ..._queueSearchRealtimeOverrides(),
          authControllerProvider.overrideWith(authenticatedAuthController),
          matchmakingSearchControllerProvider.overrideWith(
            () => _TimedOutSearchController(),
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

    expect(find.byKey(QueueSearchScreen.timeoutStateKey), findsOneWidget);
    await tester.tap(find.text('Return to queue'));
    await tester.pump();
    expect(find.byKey(QueueSearchScreen.startButtonKey), findsOneWidget);
  });

  testWidgets('queue search shows declined recovery while searching', (tester) async {
    final game = _valorantGame();
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceThemeTestOverrides(),
          ..._queueSearchRealtimeOverrides(),
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
            () => _DeclinedRecoverySearchController(),
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

    expect(find.byKey(QueueSearchScreen.declinedStateKey), findsOneWidget);
    expect(find.byKey(QueueSearchScreen.searchingStateKey), findsOneWidget);
    await tester.tap(find.text('Continue searching'));
    await tester.pump();
    expect(find.byKey(QueueSearchScreen.declinedStateKey), findsNothing);
    expect(find.byKey(QueueSearchScreen.searchingStateKey), findsOneWidget);
  });
}

class _NoopRealtimeHub extends RealtimeHub {
  _NoopRealtimeHub(super.ref);

  @override
  Future<void> ensureConnected() async {}

  @override
  void ensureSubscribed(String chatId) {}
}

class _TimedOutSearchController extends MatchmakingSearchController {
  @override
  MatchmakingSearchState build() =>
      const MatchmakingSearchState(recoveryReason: SearchRecoveryReason.timeout);
}

class _NudgeSearchController extends MatchmakingSearchController {
  @override
  MatchmakingSearchState build() => const MatchmakingSearchState(nudgeVisible: true);
}

class _DeclinedRecoverySearchController extends MatchmakingSearchController {
  @override
  MatchmakingSearchState build() => const MatchmakingSearchState(
        recoveryReason: SearchRecoveryReason.declined,
      );
}
