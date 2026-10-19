import 'dart:convert';
import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/auth_session.dart';
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
  for (final transition in ['other', 'aba', 'logout']) {
    testWidgets(
      'late search response cannot promote the prior actor session $transition',
      (tester) async {
        final game = _valorantGame();
        final started = Completer<void>();
        final response = Completer<http.Response>();
        final client = MockClient((r) async {
          if (r.method == 'POST' &&
              r.url.path == '/api/v1/matchmaking/search') {
            started.complete();
            return response.future;
          }
          return http.Response('{}', 404);
        });
        final container = ProviderContainer(
          overrides: voiceAppTestOverrides(client: client),
        );
        addTearDown(container.dispose);
        await tester.pumpWidget(
          UncontrolledProviderScope(
            container: container,
            child: MaterialApp(
              theme: voiceTestTheme(),
              localizationsDelegates: AppLocalizations.localizationsDelegates,
              supportedLocales: AppLocalizations.supportedLocales,
              home: QueueSearchScreen(
                game: game,
                mode: game.config.modes.first,
              ),
            ),
          ),
        );
        await tester.pumpAndSettle();
        await tester.tap(find.byKey(QueueSearchScreen.startButtonKey));
        await tester.pump();
        expect(started.isCompleted, isTrue);
        final original = container.read(authControllerProvider).session!;
        if (transition == 'logout') {
          container.read(authControllerProvider.notifier).state =
              const AuthState();
        } else {
          container
              .read(authControllerProvider.notifier)
              .state = const AuthState(
            session: AuthSession(
              accessToken: 'new',
              refreshToken: 'new',
              accountId: 'new-account',
              activeProfileId: 'new-profile',
              expiresInSeconds: 900,
            ),
          );
        }
        if (transition != 'other') {
          container.read(authControllerProvider.notifier).state = AuthState(
            session: original,
          );
        }
        response.complete(
          http.Response(
            jsonEncode({
              'searchSession': {
                'id': 'old-session',
                'profileId': 'prof-test',
                'gameId': game.id,
                'mode': game.config.modes.first.name,
                'criteriaJson': '{}',
                'status': 'searching',
              },
            }),
            200,
          ),
        );
        await tester.pumpAndSettle();
        expect(container.read(activeSearchSessionProvider), isNull);
        expect(find.byKey(QueueSearchScreen.searchingStateKey), findsNothing);
      },
    );
  }

  testWidgets(
    'ambiguous search retry retains intent and edited criteria changes it',
    (tester) async {
      final game = _valorantGame();
      final operations = <String?>[];
      final client = MockClient((r) async {
        if (r.method == 'POST' && r.url.path == '/api/v1/matchmaking/search') {
          operations.add(
            (jsonDecode(r.body) as Map<String, dynamic>)['operationId']
                as String?,
          );
          return http.Response('{}', 503);
        }
        return http.Response('{}', 404);
      });
      await tester.pumpWidget(
        ProviderScope(
          overrides: voiceAppTestOverrides(client: client),
          child: MaterialApp(
            theme: voiceTestTheme(),
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: QueueSearchScreen(game: game, mode: game.config.modes.first),
          ),
        ),
      );
      await tester.pumpAndSettle();
      for (var i = 0; i < 2; i++) {
        await tester.ensureVisible(
          find.byKey(QueueSearchScreen.startButtonKey),
        );
        await tester.tap(find.byKey(QueueSearchScreen.startButtonKey));
        await tester.pumpAndSettle();
      }
      expect(operations, hasLength(2));
      expect(operations.first, isNotNull);
      expect(operations[1], operations[0]);
      await tester.ensureVisible(find.text('eu').first);
      await tester.tap(find.text('eu').first);
      await tester.pumpAndSettle();
      await tester.tap(find.text('na').last);
      await tester.pumpAndSettle();
      await tester.ensureVisible(find.byKey(QueueSearchScreen.startButtonKey));
      await tester.tap(find.byKey(QueueSearchScreen.startButtonKey));
      await tester.pumpAndSettle();
      expect(operations, hasLength(3));
      expect(operations[2], isNot(operations[0]));
      final container = ProviderScope.containerOf(
        tester.element(find.byType(QueueSearchScreen)),
      );
      final original = container.read(authControllerProvider).session!;
      container.read(authControllerProvider.notifier).state = AuthState(
        session: AuthSession(
          accessToken: 'refreshed-token',
          refreshToken: 'refreshed-token',
          accountId: original.accountId,
          activeProfileId: original.activeProfileId,
          expiresInSeconds: 900,
          accountType: original.accountType,
        ),
      );
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(QueueSearchScreen.startButtonKey));
      await tester.pumpAndSettle();
      expect(operations, hasLength(4));
      expect(
        operations[3],
        operations[2],
        reason: 'ordinary token refresh retains the unresolved action',
      );
      for (final transition in ['aba', 'logout']) {
        if (transition == 'logout') {
          container.read(authControllerProvider.notifier).state =
              const AuthState();
        } else {
          container
              .read(authControllerProvider.notifier)
              .state = const AuthState(
            session: AuthSession(
              accessToken: 'other',
              refreshToken: 'other',
              accountId: 'other',
              activeProfileId: 'other',
              expiresInSeconds: 900,
            ),
          );
        }
        container.read(authControllerProvider.notifier).state = AuthState(
          session: original,
        );
        await tester.pumpAndSettle();
        final prior = operations.last;
        await tester.tap(find.byKey(QueueSearchScreen.startButtonKey));
        await tester.pumpAndSettle();
        expect(operations.last, isNot(prior), reason: transition);
      }
      expect(operations, hasLength(6));
    },
  );

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
          activeSearchSessionProvider.overrideWith(
            (ref) => SearchSessionData(
              id: 'sess-1',
              profileId: 'p1',
              gameId: game.id,
              mode: game.config.modes.first.name,
              criteriaJson: '{}',
              status: 'searching',
            ),
          ),
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

  testWidgets('queue search prefills criteria from player profile', (
    tester,
  ) async {
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
      if (req.method == 'POST' &&
          req.url.path == '/api/v1/matchmaking/search') {
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

  testWidgets('queue search shows declined recovery while searching', (
    tester,
  ) async {
    final game = _valorantGame();
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceThemeTestOverrides(),
          ..._queueSearchRealtimeOverrides(),
          authControllerProvider.overrideWith(authenticatedAuthController),
          activeSearchSessionProvider.overrideWith(
            (ref) => SearchSessionData(
              id: 'sess-1',
              profileId: 'p1',
              gameId: game.id,
              mode: game.config.modes.first.name,
              criteriaJson: '{}',
              status: 'searching',
            ),
          ),
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
  MatchmakingSearchState build() => const MatchmakingSearchState(
    recoveryReason: SearchRecoveryReason.timeout,
  );
}

class _NudgeSearchController extends MatchmakingSearchController {
  @override
  MatchmakingSearchState build() =>
      const MatchmakingSearchState(nudgeVisible: true);
}

class _DeclinedRecoverySearchController extends MatchmakingSearchController {
  @override
  MatchmakingSearchState build() => const MatchmakingSearchState(
    recoveryReason: SearchRecoveryReason.declined,
  );
}
