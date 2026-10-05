import 'dart:async';
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/matchmaking_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/matchmaking_match_controller.dart';
import 'package:voice_frontend/state/matchmaking_providers.dart';
import 'package:voice_frontend/state/matchmaking_search_controller.dart';
import 'package:voice_frontend/ui/matchmaking/match_found_overlay.dart';

import 'support/auth_test_overrides.dart';
import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

MatchData _pendingMatch() {
  return const MatchData(
    id: 'match-1',
    gameId: 'g-val',
    mode: 'Duo',
    region: 'eu',
    status: 'pending_accept',
    profileIds: ['p1', 'p2'],
    gameName: 'Valorant',
  );
}

MatchDeadlineClock _deadlineClock() {
  final serverNow = DateTime.now().toUtc();
  return MatchDeadlineClock(
    serverNow: serverNow,
    deadline: serverNow.add(const Duration(seconds: 30)),
  );
}

ThemeData _notoTheme() => voiceTestTheme().copyWith(
  textTheme: voiceTestTheme().textTheme.apply(fontFamily: 'Noto Sans'),
);

Future<void> _loadNotoSans() async {
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

Future<void> _waitForMatch(ProviderContainer container, String id) async {
  if (container.read(matchmakingMatchControllerProvider).match?.id == id) {
    return;
  }
  final loaded = Completer<void>();
  final subscription = container.listen(matchmakingMatchControllerProvider, (
    previous,
    next,
  ) {
    if (next.match?.id == id && !loaded.isCompleted) loaded.complete();
  });
  await loaded.future.timeout(const Duration(seconds: 2));
  subscription.close();
}

void main() {
  setUpAll(_loadNotoSans);

  testWidgets('MatchFound deadline refresh is monotonic across remounts', (
    tester,
  ) async {
    tester.view.physicalSize = const Size(390, 844);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    var elapsed = Duration.zero;
    final clock = MatchDeadlineClock(
      serverNow: DateTime.utc(2026, 10, 5, 12),
      deadline: DateTime.utc(2026, 10, 5, 12, 0, 2),
      elapsed: () => elapsed,
    );
    var refreshes = 0;
    await tester.pumpWidget(
      ProviderScope(
        overrides: voiceThemeTestOverrides(),
        child: MaterialApp(
          theme: voiceTestTheme(),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: Scaffold(
            body: Stack(
              children: [
                MatchFoundOverlay(
                  match: _pendingMatch(),
                  deadlineClock: clock,
                  onRefresh: () async {
                    refreshes++;
                    return true;
                  },
                ),
              ],
            ),
          ),
        ),
      ),
    );
    expect(find.text('2s'), findsOneWidget);
    elapsed = const Duration(seconds: 1);
    await tester.pump(const Duration(milliseconds: 200));
    expect(find.text('1s'), findsOneWidget);
    await tester.pumpWidget(const SizedBox.shrink());
    elapsed = const Duration(seconds: 2);
    await tester.pump(const Duration(milliseconds: 200));
    await tester.pumpWidget(
      ProviderScope(
        overrides: voiceThemeTestOverrides(),
        child: MaterialApp(
          theme: _notoTheme(),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: Scaffold(
            body: Stack(
              children: [
                MatchFoundOverlay(
                  match: _pendingMatch(),
                  deadlineClock: clock,
                  onRefresh: () async {
                    refreshes++;
                    return true;
                  },
                ),
              ],
            ),
          ),
        ),
      ),
    );
    expect(find.text('0s'), findsOneWidget);
    await tester.pump(const Duration(milliseconds: 800));
    expect(refreshes, 1);
  });

  testWidgets('MatchFound refreshes server state when the app resumes', (
    tester,
  ) async {
    var refreshes = 0;
    await tester.pumpWidget(
      ProviderScope(
        overrides: voiceThemeTestOverrides(),
        child: MaterialApp(
          theme: voiceTestTheme(),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: Scaffold(
            body: Stack(
              children: [
                MatchFoundOverlay(
                  match: _pendingMatch(),
                  deadlineClock: MatchDeadlineClock(
                    serverNow: DateTime.now().toUtc(),
                    deadline: DateTime.now().toUtc().add(
                      const Duration(seconds: 30),
                    ),
                  ),
                  onRefresh: () async {
                    refreshes++;
                    return true;
                  },
                ),
              ],
            ),
          ),
        ),
      ),
    );
    tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.resumed);
    await tester.pump();
    expect(refreshes, 1);
  });

  testWidgets('failed MatchFound response can be retried without duplicates', (
    tester,
  ) async {
    var attempts = 0;
    await tester.pumpWidget(
      ProviderScope(
        overrides: voiceThemeTestOverrides(),
        child: MaterialApp(
          theme: voiceTestTheme(),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: Scaffold(
            body: Stack(
              children: [
                MatchFoundOverlay(
                  match: _pendingMatch(),
                  onRespond: (accept) async {
                    attempts++;
                    if (attempts == 1) return null;
                    return RespondToMatchData(
                      match: _pendingMatch(),
                      searchSession: const SearchSessionData(
                        id: 'session-1',
                        profileId: 'p1',
                        gameId: 'g-val',
                        mode: 'Duo',
                        criteriaJson: '{}',
                        status: 'pending_accept',
                      ),
                    );
                  },
                ),
              ],
            ),
          ),
        ),
      ),
    );
    await tester.tap(find.byKey(MatchFoundOverlay.acceptButtonKey));
    await tester.pump();
    expect(attempts, 1);
    expect(
      tester
          .widget<FilledButton>(find.byKey(MatchFoundOverlay.acceptButtonKey))
          .onPressed,
      isNotNull,
    );
    await tester.tap(find.byKey(MatchFoundOverlay.acceptButtonKey));
    await tester.pump();
    expect(attempts, 2);
  });

  testWidgets('late response after overlay disposal is ignored safely', (
    tester,
  ) async {
    final response = Completer<RespondToMatchData?>();
    var showOverlay = true;
    await tester.pumpWidget(
      ProviderScope(
        overrides: voiceThemeTestOverrides(),
        child: MaterialApp(
          theme: voiceTestTheme(),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: StatefulBuilder(
            builder: (context, setState) => Scaffold(
              body: Stack(
                children: [
                  if (showOverlay)
                    MatchFoundOverlay(
                      match: _pendingMatch(),
                      onRespond: (_) => response.future,
                    ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
    await tester.tap(find.byKey(MatchFoundOverlay.acceptButtonKey));
    await tester.pump();
    await tester.pumpWidget(const SizedBox.shrink());
    response.complete(null);
    await tester.pump();
    expect(tester.takeException(), isNull);
  });

  testWidgets('H and V MatchFound layouts fit with the bundled Noto font', (
    tester,
  ) async {
    for (final size in [const Size(1280, 800), const Size(390, 844)]) {
      tester.view.physicalSize = size;
      tester.view.devicePixelRatio = 1;
      await tester.pumpWidget(
        ProviderScope(
          overrides: voiceThemeTestOverrides(),
          child: MaterialApp(
            debugShowCheckedModeBanner: false,
            theme: _notoTheme(),
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: Scaffold(
              body: Stack(
                children: [
                  MatchFoundOverlay(
                    match: _pendingMatch(),
                    deadlineClock: _deadlineClock(),
                  ),
                ],
              ),
            ),
          ),
        ),
      );
      await tester.pump();
      expect(
        Theme.of(
          tester.element(find.byKey(MatchFoundOverlay.timerKey)),
        ).textTheme.bodyMedium?.fontFamily,
        'Noto Sans',
      );
      for (final key in [
        MatchFoundOverlay.timerKey,
        MatchFoundOverlay.acceptButtonKey,
        MatchFoundOverlay.declineButtonKey,
      ]) {
        final rect = tester.getRect(find.byKey(key));
        expect(rect.left, greaterThanOrEqualTo(0));
        expect(rect.top, greaterThanOrEqualTo(0));
        expect(rect.right, lessThanOrEqualTo(size.width));
        expect(rect.bottom, lessThanOrEqualTo(size.height));
      }
      expect(tester.takeException(), isNull);
    }
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
  });

  test('decline restores the server returned party session', () async {
    final responded = Completer<void>();
    final client = MockClient((request) async {
      if (request.url.path.endsWith('/respond')) {
        responded.complete();
        return http.Response(
          jsonEncode({
            'match': {
              'id': 'match-1',
              'gameId': 'g-val',
              'mode': 'Duo',
              'region': 'eu',
              'status': 'abandoned',
              'profileIds': ['p1', 'p2'],
            },
            'searchSession': {
              'id': 'recovered-session',
              'profileId': 'p1',
              'gameId': 'g-val',
              'mode': 'Duo',
              'criteriaJson': '{"partyId":"party-1"}',
              'status': 'searching',
            },
          }),
          200,
        );
      }
      return http.Response(
        jsonEncode({
          'match': {
            'id': 'match-1',
            'gameId': 'g-val',
            'mode': 'Duo',
            'region': 'eu',
            'status': 'pending_accept',
            'profileIds': ['p1', 'p2'],
          },
          'serverNow': '2026-10-05T12:00:00Z',
          'acceptanceDeadlineAt': '2026-10-05T12:00:30Z',
          'ownProposalResponse': 'pending',
        }),
        200,
      );
    });
    final container = ProviderContainer(
      overrides: voiceAppTestOverrides(client: client),
    );
    addTearDown(container.dispose);
    final priorSession = const SearchSessionData(
      id: 'old-session',
      profileId: 'p1',
      gameId: 'g-val',
      mode: 'Duo',
      criteriaJson: '{}',
      status: 'matched',
    );
    container.read(activeSearchSessionProvider.notifier).state = priorSession;
    final controller = container.read(
      matchmakingMatchControllerProvider.notifier,
    );
    controller.onPushNotificationData({
      'type': 'match_found',
      'match_id': 'match-1',
    });
    await _waitForMatch(container, 'match-1');
    await controller.respond(false);
    await responded.future;

    expect(
      container.read(activeSearchSessionProvider)?.id,
      'recovered-session',
    );
    expect(
      container.read(matchmakingSearchControllerProvider).recoveryReason,
      SearchRecoveryReason.declined,
    );
    expect(container.read(matchmakingMatchControllerProvider).match, isNull);
  });

  test(
    'late GetMatch cannot replace a newer match or a changed auth session',
    () async {
      var oldMatchResponse = Completer<http.Response>();
      var oldMatchStarted = Completer<void>();
      final client = MockClient((request) async {
        if (request.url.path.endsWith('/old-match')) {
          oldMatchStarted.complete();
          return oldMatchResponse.future;
        }
        return http.Response(
          jsonEncode({
            'match': {
              'id': 'new-match',
              'gameId': 'g-val',
              'mode': 'Duo',
              'region': 'eu',
              'status': 'pending_accept',
              'profileIds': ['p1', 'p2'],
            },
            'serverNow': '2026-10-05T12:00:00Z',
            'acceptanceDeadlineAt': '2026-10-05T12:00:30Z',
            'ownProposalResponse': 'pending',
          }),
          200,
        );
      });
      final container = ProviderContainer(
        overrides: voiceAppTestOverrides(client: client),
      );
      addTearDown(container.dispose);
      final controller = container.read(
        matchmakingMatchControllerProvider.notifier,
      );
      controller.onPushNotificationData({
        'type': 'match_found',
        'match_id': 'old-match',
      });
      await oldMatchStarted.future;
      controller.onPushNotificationData({
        'type': 'match_found',
        'match_id': 'new-match',
      });
      await _waitForMatch(container, 'new-match');
      expect(
        container.read(matchmakingMatchControllerProvider).match?.id,
        'new-match',
      );

      oldMatchResponse.complete(
        http.Response(
          jsonEncode({
            'match': {
              'id': 'old-match',
              'gameId': 'g-val',
              'mode': 'Duo',
              'region': 'eu',
              'status': 'pending_accept',
              'profileIds': ['p1', 'p2'],
            },
            'serverNow': '2026-10-05T12:00:00Z',
            'acceptanceDeadlineAt': '2026-10-05T12:00:30Z',
          }),
          200,
        ),
      );
      await Future<void>.delayed(const Duration(milliseconds: 20));
      expect(
        container.read(matchmakingMatchControllerProvider).match?.id,
        'new-match',
      );

      oldMatchResponse = Completer<http.Response>();
      oldMatchStarted = Completer<void>();
      controller.onPushNotificationData({
        'type': 'match_found',
        'match_id': 'old-match',
      });
      await oldMatchStarted.future;
      container.read(authControllerProvider.notifier).state = const AuthState(
        session: AuthSession(
          accessToken: 'changed-access',
          refreshToken: 'changed-refresh',
          accountId: 'acc-test',
          activeProfileId: 'prof-test',
          expiresInSeconds: 900,
        ),
      );
      oldMatchResponse.complete(http.Response('{}', 200));
      await Future<void>.delayed(Duration.zero);
      expect(
        container.read(matchmakingMatchControllerProvider).match?.id,
        'new-match',
      );
    },
  );

  test(
    'late accepted response is ignored after the auth session changes',
    () async {
      final respondStarted = Completer<void>();
      final respondResponse = Completer<http.Response>();
      final client = MockClient((request) async {
        if (request.url.path.endsWith('/respond')) {
          respondStarted.complete();
          return respondResponse.future;
        }
        return http.Response(
          jsonEncode({
            'match': {
              'id': 'match-1',
              'gameId': 'g-val',
              'mode': 'Duo',
              'region': 'eu',
              'status': 'pending_accept',
              'profileIds': ['p1', 'p2'],
            },
            'serverNow': '2026-10-05T12:00:00Z',
            'acceptanceDeadlineAt': '2026-10-05T12:00:30Z',
            'ownProposalResponse': 'pending',
          }),
          200,
        );
      });
      final container = ProviderContainer(
        overrides: voiceAppTestOverrides(client: client),
      );
      addTearDown(container.dispose);
      final controller = container.read(
        matchmakingMatchControllerProvider.notifier,
      );
      controller.onPushNotificationData({
        'type': 'match_found',
        'match_id': 'match-1',
      });
      await _waitForMatch(container, 'match-1');

      final response = controller.respond(true);
      await respondStarted.future;
      container.read(authControllerProvider.notifier).state = const AuthState(
        session: AuthSession(
          accessToken: 'changed-access',
          refreshToken: 'changed-refresh',
          accountId: 'acc-test',
          activeProfileId: 'prof-test',
          expiresInSeconds: 900,
        ),
      );
      respondResponse.complete(
        http.Response(
          jsonEncode({
            'match': {
              'id': 'match-1',
              'gameId': 'g-val',
              'mode': 'Duo',
              'region': 'eu',
              'status': 'active',
              'profileIds': ['p1', 'p2'],
              'chatId': 'chat-1',
              'voiceRoomId': 'room-1',
            },
            'searchSession': {
              'id': 'session-1',
              'profileId': 'p1',
              'gameId': 'g-val',
              'mode': 'Duo',
              'criteriaJson': '{}',
              'status': 'matched',
            },
          }),
          200,
        ),
      );

      expect(await response, isNull);
      expect(
        container.read(matchmakingMatchControllerProvider).match?.status,
        'pending_accept',
      );
      expect(container.read(activeSquadMatchProvider), isNull);
    },
  );

  testWidgets('match found overlay shows accept and decline actions', (
    tester,
  ) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: voiceThemeTestOverrides(),
        child: MaterialApp(
          theme: voiceTestTheme(),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: Scaffold(
            body: Stack(children: [MatchFoundOverlay(match: _pendingMatch())]),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(MatchFoundOverlay.acceptButtonKey), findsOneWidget);
    expect(find.byKey(MatchFoundOverlay.declineButtonKey), findsOneWidget);
    expect(find.textContaining('Valorant'), findsOneWidget);
  });

  testWidgets('accept button triggers respondToMatch with accept true', (
    tester,
  ) async {
    bool? accepted;
    var showOverlay = true;
    await tester.pumpWidget(
      ProviderScope(
        overrides: voiceThemeTestOverrides(),
        child: MaterialApp(
          theme: voiceTestTheme(),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: StatefulBuilder(
            builder: (context, setState) {
              return Scaffold(
                body: Stack(
                  children: [
                    if (showOverlay)
                      MatchFoundOverlay(
                        match: _pendingMatch(),
                        onRespond: (accept) async {
                          accepted = accept;
                          setState(() => showOverlay = false);
                          return const RespondToMatchData(
                            match: MatchData(
                              id: 'match-1',
                              gameId: 'g-val',
                              mode: 'Duo',
                              region: 'eu',
                              status: 'active',
                              profileIds: ['p1', 'p2'],
                            ),
                            searchSession: SearchSessionData(
                              id: 'sess-1',
                              profileId: 'p1',
                              gameId: 'g-val',
                              mode: 'Duo',
                              criteriaJson: '{}',
                              status: 'matched',
                            ),
                          );
                        },
                      ),
                  ],
                ),
              );
            },
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(MatchFoundOverlay.acceptButtonKey));
    await tester.pumpAndSettle();

    expect(accepted, isTrue);
    expect(find.byType(MatchFoundOverlay), findsNothing);
  });
}
