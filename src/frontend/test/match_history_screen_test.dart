import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/ui/matchmaking/match_history_screen.dart';

import 'support/auth_test_overrides.dart';
import 'support/voice_test_theme.dart';

void main() {
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
}
