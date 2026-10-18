import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter/semantics.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/ui/matchmaking/game_catalog_screen.dart';
import 'package:voice_frontend/ui/matchmaking/match_history_screen.dart';
import 'package:voice_frontend/ui/matchmaking/match_rating_overlay.dart';
import 'package:voice_frontend/ui/report/report_sheet.dart';

import 'support/auth_test_overrides.dart';
import 'support/voice_test_theme.dart';

void main() {
  for (final language in ['en', 'ru']) {
    for (final statusCode in [500, 503]) {
      testWidgets(
        'MatchHistoryScreen $language $statusCode has a localized failure and recovers on Retry',
        (tester) async {
          final requests = <http.Request>[];
          final diagnostic = 'private_match_history_${language}_$statusCode';
          final client = MockClient((request) async {
            if (request.url.path == '/api/v1/matchmaking/profile/me/matches') {
              requests.add(request);
              if (requests.length == 1) {
                return http.Response(
                  jsonEncode({'error': 'internal', 'message': diagnostic}),
                  statusCode,
                );
              }
              return http.Response(
                jsonEncode({
                  'matchList': {'matches': [], 'nextCursor': ''},
                }),
                200,
              );
            }
            if (request.url.path == '/api/v1/matchmaking/games') {
              return http.Response(
                jsonEncode({
                  'gameList': {'games': [], 'nextCursor': ''},
                }),
                200,
              );
            }
            return http.Response('{}', 404);
          });
          await tester.pumpWidget(
            ProviderScope(
              overrides: voiceAppTestOverrides(client: client),
              child: MaterialApp(
                theme: voiceTestTheme(),
                locale: Locale(language),
                localizationsDelegates: AppLocalizations.localizationsDelegates,
                supportedLocales: AppLocalizations.supportedLocales,
                home: const MatchHistoryScreen(),
              ),
            ),
          );
          await tester.pumpAndSettle();

          final panel = find.byKey(MatchHistoryScreen.errorKey);
          expect(panel, findsOneWidget);
          final l10n = AppLocalizations.of(tester.element(panel))!;
          final title = language == 'ru'
              ? 'Не удалось загрузить историю матчей'
              : 'Could not load match history';
          expect(find.text(title), findsOneWidget);
          if (language == 'ru') {
            expect(find.text('Could not load match history'), findsNothing);
          }
          expect(
            find.text(l10n.backendUnavailable),
            statusCode == 503 ? findsOneWidget : findsNothing,
          );
          expect(find.textContaining(diagnostic), findsNothing);
          final retry = find.widgetWithText(OutlinedButton, l10n.commonRetry);
          expect(retry, findsOneWidget);
          expect(tester.widget<OutlinedButton>(retry).onPressed, isNotNull);
          expect(requests, hasLength(1));
          await tester.pump(const Duration(seconds: 1));
          expect(requests, hasLength(1), reason: 'no automatic retry');

          await tester.tap(retry);
          await tester.pumpAndSettle();
          expect(requests, hasLength(2));
          for (final request in requests) {
            expect(request.method, 'GET');
            expect(
              request.url.toString(),
              'http://localhost:9999/api/v1/matchmaking/profile/me/matches?page_size=20',
            );
            expect(request.headers['Authorization'], 'Bearer test-access');
            expect(request.body, isEmpty);
          }
          expect(find.byKey(MatchHistoryScreen.errorKey), findsNothing);
          expect(find.byKey(MatchHistoryScreen.emptyKey), findsOneWidget);
          expect(find.textContaining(diagnostic), findsNothing);
          expect(retry, findsNothing);
        },
      );
    }
  }

  testWidgets('MatchHistoryScreen shows empty state when no matches', (
    tester,
  ) async {
    final client = MockClient((request) async {
      if (request.url.path == '/api/v1/matchmaking/profile/me/matches') {
        return http.Response(
          jsonEncode({
            'matchList': {'matches': [], 'nextCursor': ''},
          }),
          200,
        );
      }
      if (request.url.path == '/api/v1/matchmaking/games') {
        return http.Response(
          jsonEncode({
            'gameList': {'games': [], 'nextCursor': ''},
          }),
          200,
        );
      }
      return http.Response('{}', 404);
    });

    await tester.pumpWidget(
      ProviderScope(
        overrides: [...voiceAppTestOverrides(client: client)],
        child: MaterialApp(
          theme: voiceTestTheme(),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const MatchHistoryScreen(),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(MatchHistoryScreen.emptyKey), findsOneWidget);
  });

  testWidgets('MatchHistoryScreen shows match list from API', (tester) async {
    final client = MockClient((request) async {
      if (request.url.path == '/api/v1/matchmaking/profile/me/matches') {
        return http.Response(
          jsonEncode({
            'matchList': {
              'matches': [
                {
                  'id': 'match-42',
                  'gameId': 'g1',
                  'mode': 'Duo',
                  'region': 'eu',
                  'status': 'completed',
                  'profileIds': ['profile-1', 'profile-2'],
                },
              ],
            },
          }),
          200,
        );
      }
      if (request.url.path == '/api/v1/matchmaking/games') {
        return http.Response(
          jsonEncode({
            'gameList': {
              'games': [
                {
                  'id': 'g1',
                  'name': 'Valorant',
                  'status': 'active',
                  'configJson': '{}',
                },
              ],
            },
          }),
          200,
        );
      }
      if (request.url.path == '/api/v1/users/profiles/profile-1') {
        return http.Response(
          jsonEncode({
            'profile': {
              'id': 'profile-1',
              'displayName': 'Player One',
              'username': 'one',
            },
          }),
          200,
        );
      }
      if (request.url.path == '/api/v1/users/profiles/profile-2') {
        return http.Response(
          jsonEncode({
            'profile': {
              'id': 'profile-2',
              'displayName': 'Teammate',
              'username': 'teammate',
            },
          }),
          200,
        );
      }
      return http.Response('{}', 404);
    });

    await tester.pumpWidget(
      ProviderScope(
        overrides: [...voiceAppTestOverrides(client: client)],
        child: MaterialApp(
          theme: voiceTestTheme(),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const MatchHistoryScreen(),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(MatchHistoryScreen.listKey), findsOneWidget);
    expect(
      find.byKey(MatchHistoryScreen.matchTileKey('match-42')),
      findsOneWidget,
    );
    expect(find.text('Valorant'), findsOneWidget);

    await tester.tap(find.byKey(MatchHistoryScreen.matchTileKey('match-42')));
    await tester.pumpAndSettle();

    expect(find.text('Teammate'), findsOneWidget);
    expect(find.text('Player One'), findsOneWidget);
    expect(
      find.byKey(MatchHistoryScreen.addFriendKey('profile-2')),
      findsOneWidget,
    );
    expect(
      find.byKey(MatchHistoryScreen.addFriendKey('profile-1')),
      findsOneWidget,
    );
    expect(find.byKey(MatchHistoryScreen.banKey('profile-2')), findsOneWidget);
  });

  testWidgets(
    'MatchHistoryScreen hides friend request API details and keeps match selected',
    (tester) async {
      const diagnostic = 'friend-request-private-diagnostic';
      final invitationRequests = <http.Request>[];
      final client = MockClient((request) async {
        if (request.url.path == '/api/v1/matchmaking/profile/me/matches') {
          return http.Response(
            jsonEncode({
              'matchList': {
                'matches': [
                  {
                    'id': 'match-42',
                    'gameId': 'g1',
                    'mode': 'Duo',
                    'region': 'eu',
                    'status': 'completed',
                    'profileIds': ['profile-1', 'profile-2'],
                  },
                ],
              },
            }),
            200,
          );
        }
        if (request.url.path == '/api/v1/matchmaking/games') {
          return http.Response(
            jsonEncode({
              'gameList': {
                'games': [
                  {
                    'id': 'g1',
                    'name': 'Valorant',
                    'status': 'active',
                    'configJson': '{}',
                  },
                ],
              },
            }),
            200,
          );
        }
        if (request.url.path == '/api/v1/users/profiles/profile-1') {
          return http.Response(
            jsonEncode({
              'profile': {
                'id': 'profile-1',
                'displayName': 'Player One',
                'username': 'one',
              },
            }),
            200,
          );
        }
        if (request.url.path == '/api/v1/users/profiles/profile-2') {
          return http.Response(
            jsonEncode({
              'profile': {
                'id': 'profile-2',
                'displayName': 'Teammate',
                'username': 'teammate',
              },
            }),
            200,
          );
        }
        if (request.url.path == '/api/v1/friends/invitations' &&
            request.method == 'POST') {
          invitationRequests.add(request);
          return http.Response(
            jsonEncode({'error': 'internal', 'message': diagnostic}),
            500,
          );
        }
        return http.Response('{}', 404);
      });

      await tester.pumpWidget(
        ProviderScope(
          overrides: [...voiceAppTestOverrides(client: client)],
          child: MaterialApp(
            theme: voiceTestTheme(),
            locale: const Locale('en'),
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: const MatchHistoryScreen(),
          ),
        ),
      );
      await tester.pumpAndSettle();

      await tester.tap(find.byKey(MatchHistoryScreen.matchTileKey('match-42')));
      await tester.pumpAndSettle();
      await tester.tap(
        find.byKey(MatchHistoryScreen.addFriendKey('profile-2')),
      );
      await tester.pumpAndSettle();

      expect(invitationRequests, hasLength(1));
      expect(invitationRequests.single.method, 'POST');
      expect(invitationRequests.single.url.path, '/api/v1/friends/invitations');
      expect(
        (jsonDecode(invitationRequests.single.body)
            as Map<String, dynamic>)['target_profile_id'],
        'profile-2',
      );
      final l10n = AppLocalizations.of(
        tester.element(find.byKey(MatchHistoryScreen.screenKey)),
      )!;
      expect(find.text(l10n.commonActionFailed), findsOneWidget);
      expect(find.textContaining(diagnostic), findsNothing);
      expect(
        find.byKey(MatchHistoryScreen.matchTileKey('match-42')),
        findsOneWidget,
      );
      expect(
        find.byKey(MatchHistoryScreen.addFriendKey('profile-2')),
        findsOneWidget,
      );
    },
  );

  testWidgets('MatchHistoryScreen shows error with retry on failure', (
    tester,
  ) async {
    const diagnostic = 'match-history-secret-diagnostic';
    final client = MockClient((request) async {
      return http.Response(jsonEncode({'message': diagnostic}), 503);
    });

    await tester.pumpWidget(
      ProviderScope(
        overrides: [...voiceAppTestOverrides(client: client)],
        child: MaterialApp(
          theme: voiceTestTheme(),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const MatchHistoryScreen(),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(MatchHistoryScreen.errorKey), findsOneWidget);
    expect(find.text(diagnostic), findsNothing);
    final l10n = AppLocalizations.of(
      tester.element(find.byKey(MatchHistoryScreen.errorKey)),
    )!;
    expect(find.text(l10n.backendUnavailable), findsOneWidget);
    expect(find.text('Try again'), findsOneWidget);
  });

  testWidgets('MatchHistoryScreen hides ordinary API error details', (
    tester,
  ) async {
    const diagnostic = 'match-history-internal-detail';
    final client = MockClient((request) async {
      return http.Response(jsonEncode({'message': diagnostic}), 500);
    });

    await tester.pumpWidget(
      ProviderScope(
        overrides: [...voiceAppTestOverrides(client: client)],
        child: MaterialApp(
          theme: voiceTestTheme(),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const MatchHistoryScreen(),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(MatchHistoryScreen.errorKey), findsOneWidget);
    expect(find.text(diagnostic), findsNothing);
    final l10n = AppLocalizations.of(
      tester.element(find.byKey(MatchHistoryScreen.errorKey)),
    )!;
    expect(find.text(l10n.matchHistoryLoadError), findsOneWidget);
    expect(find.text('Try again'), findsOneWidget);
  });

  testWidgets(
    'MatchHistory empty state keeps Find players available and opens the catalog',
    (tester) async {
      final client = MockClient((request) async {
        if (request.url.path == '/api/v1/matchmaking/profile/me/matches') {
          return http.Response(
            jsonEncode({
              'matchList': {'matches': [], 'nextCursor': ''},
            }),
            200,
          );
        }
        if (request.url.path == '/api/v1/matchmaking/games') {
          return http.Response(
            jsonEncode({
              'gameList': {'games': [], 'nextCursor': ''},
            }),
            200,
          );
        }
        return http.Response('{}', 404);
      });

      await tester.pumpWidget(
        ProviderScope(
          overrides: voiceAppTestOverrides(client: client),
          child: MaterialApp(
            theme: voiceTestTheme(),
            locale: const Locale('en'),
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: const MatchHistoryScreen(),
          ),
        ),
      );
      await tester.pumpAndSettle();

      final findPlayers = find.text('Find players');
      expect(findPlayers, findsOneWidget);
      await tester.tap(findPlayers);
      await tester.pumpAndSettle();

      expect(find.byKey(GameCatalogScreen.screenKey), findsOneWidget);
    },
  );

  testWidgets(
    'MatchHistory exposes per-player rate and report only for completed matches',
    (tester) async {
      final rateRequests = <http.Request>[];
      final client = MockClient((request) async {
        if (request.url.path == '/api/v1/matchmaking/profile/me/matches') {
          return http.Response(
            jsonEncode({
              'matchList': {
                'matches': [
                  {
                    'id': 'completed-match',
                    'gameId': 'game-1',
                    'mode': 'Duo',
                    'region': 'eu',
                    'status': 'completed',
                    'profileIds': ['prof-test', 'completed-player'],
                  },
                  {
                    'id': 'active-match',
                    'gameId': 'game-1',
                    'mode': 'Duo',
                    'region': 'eu',
                    'status': 'active',
                    'profileIds': ['prof-test', 'active-player'],
                  },
                ],
                'nextCursor': '',
              },
            }),
            200,
          );
        }
        if (request.url.path == '/api/v1/matchmaking/games') {
          return http.Response(
            jsonEncode({
              'gameList': {'games': [], 'nextCursor': ''},
            }),
            200,
          );
        }
        if (request.url.path.startsWith('/api/v1/users/profiles/')) {
          final profileId = request.url.path.split('/').last;
          return http.Response(
            jsonEncode({
              'profile': {'id': profileId, 'displayName': profileId},
            }),
            200,
          );
        }
        if (request.url.path ==
                '/api/v1/matchmaking/matches/completed-match/rate' &&
            request.method == 'POST') {
          rateRequests.add(request);
          return http.Response('{}', 200);
        }
        return http.Response('{}', 404);
      });

      await tester.pumpWidget(
        ProviderScope(
          overrides: voiceAppTestOverrides(client: client),
          child: MaterialApp(
            theme: voiceTestTheme(),
            locale: const Locale('en'),
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: const MatchHistoryScreen(),
          ),
        ),
      );
      await tester.pumpAndSettle();

      final completedHeader = find.text('Duo · eu · Completed');
      await tester.tap(completedHeader);
      await tester.pumpAndSettle();
      expect(find.text('Rate'), findsOneWidget);
      expect(find.text('Report'), findsOneWidget);

      await tester.tap(
        find.byKey(MatchHistoryScreen.rateKey('completed-player')),
      );
      await tester.pumpAndSettle();
      expect(find.byType(MatchRatingOverlay), findsOneWidget);
      await tester.tap(find.byKey(MatchRatingOverlay.starButtonKey(4)));
      await tester.tap(find.byKey(MatchRatingOverlay.submitButtonKey));
      await tester.pumpAndSettle();
      expect(rateRequests, hasLength(1));
      expect(
        rateRequests.single.url.path,
        '/api/v1/matchmaking/matches/completed-match/rate',
      );
      expect(
        jsonDecode(rateRequests.single.body),
        containsPair('ratedProfileId', 'completed-player'),
      );
      expect(jsonDecode(rateRequests.single.body), containsPair('stars', 4));

      await tester.tap(
        find.byKey(MatchHistoryScreen.reportKey('completed-player')),
      );
      await tester.pumpAndSettle();
      final report = tester.widget<ReportSheet>(find.byType(ReportSheet));
      expect(report.target, isA<ReportUserTarget>());
      expect((report.target as ReportUserTarget).profileId, 'completed-player');

      Navigator.of(tester.element(find.byType(ReportSheet))).pop();
      await tester.pumpAndSettle();
      await tester.tap(completedHeader);
      await tester.pumpAndSettle();
      expect(find.text('Rate'), findsNothing);
      expect(find.text('Report'), findsNothing);
      final activeHeader = find.text('Duo · eu · Active');
      await tester.ensureVisible(activeHeader);
      await tester.tap(activeHeader);
      await tester.pumpAndSettle();
      expect(find.text('Rate'), findsNothing);
      expect(find.text('Report'), findsNothing);
    },
  );

  testWidgets(
    'MatchHistory keeps Find players available while loading and after failure',
    (tester) async {
      final pending = Completer<http.Response>();
      final client = MockClient((request) async {
        if (request.url.path == '/api/v1/matchmaking/profile/me/matches') {
          return pending.future;
        }
        if (request.url.path == '/api/v1/matchmaking/games') {
          return http.Response(
            jsonEncode({
              'gameList': {'games': [], 'nextCursor': ''},
            }),
            200,
          );
        }
        return http.Response('{}', 404);
      });

      await tester.pumpWidget(
        ProviderScope(
          overrides: voiceAppTestOverrides(client: client),
          child: MaterialApp(
            theme: voiceTestTheme(),
            locale: const Locale('en'),
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: const MatchHistoryScreen(),
          ),
        ),
      );
      await tester.pump();
      expect(find.text('Find players'), findsOneWidget);

      pending.complete(http.Response('{}', 503));
      await tester.pumpAndSettle();
      expect(find.byKey(MatchHistoryScreen.errorKey), findsOneWidget);
      expect(find.text('Find players'), findsOneWidget);
    },
  );

  testWidgets(
    'MatchHistory hides pagination when the server returns an empty cursor',
    (tester) async {
      final client = MockClient((request) async {
        if (request.url.path == '/api/v1/matchmaking/profile/me/matches') {
          return http.Response(
            jsonEncode({
              'matchList': {
                'matches': [
                  {
                    'id': 'final-match',
                    'gameId': 'game-1',
                    'mode': 'Duo',
                    'region': 'eu',
                    'status': 'completed',
                    'profileIds': ['prof-test', 'player-2'],
                  },
                ],
                'nextCursor': '',
              },
            }),
            200,
          );
        }
        if (request.url.path == '/api/v1/matchmaking/games') {
          return http.Response(
            jsonEncode({
              'gameList': {'games': [], 'nextCursor': ''},
            }),
            200,
          );
        }
        return http.Response('{}', 404);
      });

      await tester.pumpWidget(
        ProviderScope(
          overrides: voiceAppTestOverrides(client: client),
          child: MaterialApp(
            theme: voiceTestTheme(),
            locale: const Locale('en'),
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: const MatchHistoryScreen(),
          ),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.byKey(MatchHistoryScreen.loadMoreKey), findsNothing);
    },
  );

  testWidgets(
    'MatchHistory shows a retry action when loading another page fails',
    (tester) async {
      final pageRequests = <http.Request>[];
      var continuationAttempts = 0;
      final client = MockClient((request) async {
        if (request.url.path == '/api/v1/matchmaking/profile/me/matches') {
          pageRequests.add(request);
          if (request.url.queryParameters['cursor'] == 'cursor-2') {
            continuationAttempts++;
            if (continuationAttempts == 1) return http.Response('{}', 503);
            return http.Response(
              jsonEncode({
                'matchList': {
                  'matches': [
                    {
                      'id': 'match-2',
                      'gameId': 'game-1',
                      'mode': 'Duo',
                      'region': 'eu',
                      'status': 'completed',
                      'profileIds': ['prof-test', 'player-2'],
                    },
                  ],
                  'nextCursor': '',
                },
              }),
              200,
            );
          }
          return http.Response(
            jsonEncode({
              'matchList': {
                'matches': [
                  {
                    'id': 'match-1',
                    'gameId': 'game-1',
                    'mode': 'Duo',
                    'region': 'eu',
                    'status': 'completed',
                    'profileIds': ['prof-test', 'player-1'],
                  },
                ],
                'nextCursor': 'cursor-2',
              },
            }),
            200,
          );
        }
        if (request.url.path == '/api/v1/matchmaking/games') {
          return http.Response(
            jsonEncode({
              'gameList': {'games': [], 'nextCursor': ''},
            }),
            200,
          );
        }
        return http.Response('{}', 404);
      });

      await tester.pumpWidget(
        ProviderScope(
          overrides: voiceAppTestOverrides(client: client),
          child: MaterialApp(
            theme: voiceTestTheme(),
            locale: const Locale('en'),
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: const MatchHistoryScreen(),
          ),
        ),
      );
      await tester.pumpAndSettle();

      await tester.tap(find.byKey(MatchHistoryScreen.loadMoreKey));
      await tester.pumpAndSettle();
      final l10n = AppLocalizations.of(
        tester.element(find.byKey(MatchHistoryScreen.screenKey)),
      )!;
      expect(find.text(l10n.backendUnavailable), findsOneWidget);
      final retry = find.byKey(MatchHistoryScreen.loadMoreKey);
      expect(find.text(l10n.commonRetry), findsOneWidget);
      await tester.tap(retry);
      await tester.pumpAndSettle();

      expect(continuationAttempts, 2);
      expect(pageRequests, hasLength(3));
      expect(
        pageRequests[1].url.queryParameters['cursor'],
        pageRequests[2].url.queryParameters['cursor'],
      );
      expect(
        find.byKey(MatchHistoryScreen.matchTileKey('match-1')),
        findsOneWidget,
      );
      expect(
        find.byKey(MatchHistoryScreen.matchTileKey('match-2')),
        findsOneWidget,
      );
    },
  );

  testWidgets(
    'MatchHistory actions remain usable and labeled in portrait and landscape',
    (tester) async {
      await _loadMatchHistoryCaptureFonts();
      final client = MockClient((request) async {
        if (request.url.path == '/api/v1/matchmaking/profile/me/matches') {
          return http.Response(
            jsonEncode({
              'matchList': {
                'matches': [
                  {
                    'id': 'completed-match',
                    'gameId': 'game-1',
                    'mode': 'Duo',
                    'region': 'eu',
                    'status': 'completed',
                    'profileIds': ['prof-test', 'teammate'],
                  },
                ],
                'nextCursor': '',
              },
            }),
            200,
          );
        }
        if (request.url.path == '/api/v1/matchmaking/games') {
          return http.Response(
            jsonEncode({
              'gameList': {'games': [], 'nextCursor': ''},
            }),
            200,
          );
        }
        if (request.url.path == '/api/v1/users/profiles/teammate') {
          return http.Response(
            jsonEncode({
              'profile': {
                'id': 'teammate',
                'displayName':
                    'Alexandria Example Teammate With a Long Display Name',
                'username': 'alexandria_example',
              },
            }),
            200,
          );
        }
        return http.Response('{}', 404);
      });
      final semantics = tester.ensureSemantics();

      for (final size in [const Size(1280, 800), const Size(390, 844)]) {
        tester.view.physicalSize = size;
        tester.view.devicePixelRatio = 1;
        await tester.pumpWidget(
          ProviderScope(
            overrides: voiceAppTestOverrides(client: client),
            child: MaterialApp(
              theme: voiceTestTheme(),
              locale: const Locale('en'),
              localizationsDelegates: AppLocalizations.localizationsDelegates,
              supportedLocales: AppLocalizations.supportedLocales,
              home: const MatchHistoryScreen(),
            ),
          ),
        );
        await tester.pumpAndSettle();

        final findPlayers = find.bySemanticsLabel('Find players');
        await tester.ensureVisible(findPlayers);
        expect(findPlayers, findsOneWidget);
        expect(
          tester
              .getSemantics(findPlayers)
              .getSemanticsData()
              .hasAction(SemanticsAction.tap),
          isTrue,
        );

        final orientation = size.width > size.height
            ? 'horizontal'
            : 'vertical';
        final hostDirectory = Platform.isLinux ? 'linux/' : '';
        if (size.width > size.height) {
          await expectLater(
            find.byKey(MatchHistoryScreen.screenKey),
            matchesGoldenFile(
              'goldens/${hostDirectory}match_history_$orientation.png',
            ),
          );
          continue;
        }

        final matchHeader = find.text('Duo · eu · Completed');
        await tester.ensureVisible(matchHeader);
        await tester.tap(matchHeader);
        await tester.pumpAndSettle();
        final rate = find.byKey(MatchHistoryScreen.rateKey('teammate'));
        final report = find.byKey(MatchHistoryScreen.reportKey('teammate'));
        await tester.ensureVisible(rate);
        for (final action in [rate, report]) {
          expect(action, findsOneWidget);
          final data = tester.getSemantics(action).getSemanticsData();
          expect(data.label, isNotEmpty);
          expect(data.hasAction(SemanticsAction.tap), isTrue);
        }

        var rateFocused = false;
        for (var index = 0; index < 80; index++) {
          if (_primaryFocusWithin(tester, rate)) {
            rateFocused = true;
            break;
          }
          await tester.sendKeyEvent(LogicalKeyboardKey.tab);
          await tester.pump();
        }
        expect(rateFocused || _primaryFocusWithin(tester, rate), isTrue);
        await tester.sendKeyEvent(LogicalKeyboardKey.tab);
        await tester.pump();
        expect(_primaryFocusWithin(tester, report), isTrue);
        await tester.sendKeyDownEvent(LogicalKeyboardKey.shiftLeft);
        await tester.sendKeyEvent(LogicalKeyboardKey.tab);
        await tester.sendKeyUpEvent(LogicalKeyboardKey.shiftLeft);
        await tester.pump();
        expect(_primaryFocusWithin(tester, rate), isTrue);

        await expectLater(
          find.byKey(MatchHistoryScreen.screenKey),
          matchesGoldenFile(
            'goldens/${hostDirectory}match_history_$orientation.png',
          ),
        );
      }
      semantics.dispose();
      tester.view.resetPhysicalSize();
      tester.view.resetDevicePixelRatio();
    },
  );
}

bool _primaryFocusWithin(WidgetTester tester, Finder ancestor) {
  final primaryFocus = tester.binding.focusManager.primaryFocus?.context;
  if (primaryFocus is! Element) return false;
  final target = tester.element(ancestor);
  if (identical(primaryFocus, target)) return true;
  var found = false;
  primaryFocus.visitAncestorElements((element) {
    if (identical(element, target)) {
      found = true;
      return false;
    }
    return true;
  });
  return found;
}

Future<void> _loadMatchHistoryCaptureFonts() async {
  final materialIcons = FontLoader('MaterialIcons')
    ..addFont(rootBundle.load('fonts/MaterialIcons-Regular.otf'));
  await materialIcons.load();
  final notoSans = FontLoader('Noto Sans')
    ..addFont(rootBundle.load('assets/fonts/NotoSans-Regular.ttf'))
    ..addFont(rootBundle.load('assets/fonts/NotoSans-Medium.ttf'))
    ..addFont(rootBundle.load('assets/fonts/NotoSans-SemiBold.ttf'))
    ..addFont(rootBundle.load('assets/fonts/NotoSans-Bold.ttf'));
  await notoSans.load();
}
