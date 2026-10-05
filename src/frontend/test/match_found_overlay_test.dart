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
    profileIds: ['prof-test', 'p2'],
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
                    return false;
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
                    return false;
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
                        profileId: 'prof-test',
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

  testWidgets('H and V MatchFound fit at 1.5x with bundled Noto font', (
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
            builder: (context, child) => MediaQuery(
              data: MediaQuery.of(
                context,
              ).copyWith(textScaler: TextScaler.linear(1.5)),
              child: child!,
            ),
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
      expect(
        MediaQuery.textScalerOf(
          tester.element(find.byKey(MatchFoundOverlay.timerKey)),
        ).scale(10),
        15,
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
              'profileIds': ['prof-test', 'p2'],
            },
            'searchSession': {
              'id': 'recovered-session',
              'profileId': 'prof-test',
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
            'profileIds': ['prof-test', 'p2'],
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
      profileId: 'prof-test',
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
              'profileIds': ['prof-test', 'p2'],
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
              'profileIds': ['prof-test', 'p2'],
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
        isNull,
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
              'profileIds': ['prof-test', 'p2'],
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
              'profileIds': ['prof-test', 'p2'],
              'chatId': 'chat-1',
              'voiceRoomId': 'room-1',
            },
            'searchSession': {
              'id': 'session-1',
              'profileId': 'prof-test',
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
      expect(container.read(matchmakingMatchControllerProvider).match, isNull);
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
          builder: (context, child) => MediaQuery(
            data: MediaQuery.of(
              context,
            ).copyWith(textScaler: TextScaler.linear(1.5)),
            child: child!,
          ),
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
                              profileIds: ['prof-test', 'p2'],
                            ),
                            searchSession: SearchSessionData(
                              id: 'sess-1',
                              profileId: 'prof-test',
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

  test('a failed or mismatched preflight never posts a response', () async {
    for (final preflight in ['failed', 'different-match']) {
      var gets = 0;
      var posts = 0;
      final client = MockClient((request) async {
        if (request.url.path.endsWith('/respond')) {
          posts++;
          return http.Response('{}', 500);
        }
        gets++;
        if (gets == 1) {
          return _matchResponse(id: 'match-1', deadline: true);
        }
        if (preflight == 'failed') {
          return http.Response('{}', 503);
        }
        return _matchResponse(id: 'different-match', deadline: true);
      });
      final container = ProviderContainer(
        overrides: voiceAppTestOverrides(client: client),
      );
      final controller = container.read(
        matchmakingMatchControllerProvider.notifier,
      );
      controller.onPushNotificationData({
        'type': 'match_found',
        'match_id': 'match-1',
      });
      await _waitForMatch(container, 'match-1');
      await controller.respond(true);
      expect(posts, 0, reason: preflight);
      container.dispose();
    }
  });

  test(
    'response flight is controller-owned across duplicate actions',
    () async {
      final respondStarted = Completer<void>();
      final response = Completer<http.Response>();
      var posts = 0;
      final client = MockClient((request) async {
        if (request.url.path.endsWith('/respond')) {
          posts++;
          if (!respondStarted.isCompleted) respondStarted.complete();
          return response.future;
        }
        return _matchResponse(id: 'match-1', deadline: true);
      });
      final container = ProviderContainer(
        overrides: voiceAppTestOverrides(client: client),
      );
      final controller = container.read(
        matchmakingMatchControllerProvider.notifier,
      );
      controller.onPushNotificationData({
        'type': 'match_found',
        'match_id': 'match-1',
      });
      await _waitForMatch(container, 'match-1');

      final first = controller.respond(true);
      expect(
        container.read(matchmakingMatchControllerProvider).isResponding,
        isTrue,
      );
      final duplicate = controller.respond(true);
      await respondStarted.future;
      await Future<void>.delayed(const Duration(milliseconds: 20));
      expect(posts, 1);
      expect(await controller.respond(false), isNull);

      response.complete(_acceptedMatchResponse());
      await Future.wait([first, duplicate]);
      expect(container.read(activeSquadMatchProvider)?.id, 'match-1');
      container.dispose();
    },
  );

  testWidgets(
    'held MatchFound response survives host remount and stale auth flight cannot clear replacement busy state',
    (tester) async {
      final firstPostStarted = Completer<void>();
      final secondPostStarted = Completer<void>();
      final firstResponse = Completer<http.Response>();
      final secondResponse = Completer<http.Response>();
      final backgroundFocus = FocusNode(debugLabel: 'MatchFound background');
      addTearDown(backgroundFocus.dispose);
      var postCount = 0;
      final client = MockClient((request) async {
        if (request.url.path.endsWith('/respond')) {
          postCount++;
          if (postCount == 1) {
            firstPostStarted.complete();
            return firstResponse.future;
          }
          if (postCount == 2) {
            secondPostStarted.complete();
            return secondResponse.future;
          }
          fail('Unexpected extra MatchFound response POST');
        }
        return _matchResponse(id: 'match-1', deadline: true);
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

      var hostVisible = false;
      late StateSetter setHostState;
      await tester.pumpWidget(
        UncontrolledProviderScope(
          container: container,
          child: MaterialApp(
            theme: voiceTestTheme(),
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: StatefulBuilder(
              builder: (context, setState) {
                setHostState = setState;
                return Scaffold(
                  body: Column(
                    children: [
                      TextButton(
                        key: const Key('match_found_background_action'),
                        focusNode: backgroundFocus,
                        onPressed: () {},
                        child: const Text('Background action'),
                      ),
                      Expanded(
                        child: Stack(
                          children: [
                            if (hostVisible)
                              const MatchmakingMatchOverlayHost(),
                          ],
                        ),
                      ),
                    ],
                  ),
                );
              },
            ),
          ),
        ),
      );
      await tester.tap(find.byKey(const Key('match_found_background_action')));
      backgroundFocus.requestFocus();
      await tester.pump();
      expect(backgroundFocus.hasFocus, isTrue);
      setHostState(() => hostVisible = true);
      await tester.pump();
      await tester.pump();
      expect(find.byKey(MatchFoundOverlay.acceptButtonKey), findsOneWidget);

      // Verify the modal focus loop as part of the real controller host.
      await tester.sendKeyEvent(LogicalKeyboardKey.tab);
      expect(
        tester
            .widget<OutlinedButton>(
              find.byKey(MatchFoundOverlay.declineButtonKey),
            )
            .focusNode
            ?.hasFocus,
        isTrue,
      );
      await tester.sendKeyDownEvent(LogicalKeyboardKey.shiftLeft);
      await tester.sendKeyEvent(LogicalKeyboardKey.tab);
      await tester.sendKeyUpEvent(LogicalKeyboardKey.shiftLeft);
      expect(
        tester
            .widget<FilledButton>(find.byKey(MatchFoundOverlay.acceptButtonKey))
            .focusNode
            ?.hasFocus,
        isTrue,
      );

      final firstClock = container
          .read(matchmakingMatchControllerProvider)
          .deadlineClock!;
      final initialRemaining = firstClock.remaining;
      await tester.tap(find.byKey(MatchFoundOverlay.acceptButtonKey));
      await firstPostStarted.future.timeout(const Duration(seconds: 2));
      final firstFlight = controller.respond(true);
      expect(postCount, 1);
      expect(
        container.read(matchmakingMatchControllerProvider).isResponding,
        isTrue,
      );
      expect(await controller.respond(false), isNull);

      setHostState(() => hostVisible = false);
      await tester.pump();
      await tester.pump();
      expect(backgroundFocus.hasFocus, isTrue);
      setHostState(() => hostVisible = true);
      await tester.pump();
      await tester.pump();
      final modalFocusScope = FocusScope.of(
        tester.element(find.byKey(MatchFoundOverlay.modalSemanticsKey)),
      );
      expect(modalFocusScope.hasFocus, isTrue);
      expect(
        container
            .read(matchmakingMatchControllerProvider)
            .deadlineClock!
            .remaining,
        lessThanOrEqualTo(initialRemaining),
      );
      expect(
        tester
            .widget<FilledButton>(find.byKey(MatchFoundOverlay.acceptButtonKey))
            .onPressed,
        isNull,
      );
      final remountedDuplicate = controller.respond(true);
      expect(await controller.respond(false), isNull);
      expect(postCount, 1);
      await tester.sendKeyEvent(LogicalKeyboardKey.tab);
      await tester.pump();
      expect(modalFocusScope.hasFocus, isTrue);
      expect(backgroundFocus.hasFocus, isFalse);

      final replacementSearch = container.read(activeSearchSessionProvider);
      container.read(authControllerProvider.notifier).state = const AuthState(
        session: AuthSession(
          accessToken: 'replacement-access',
          refreshToken: 'replacement-refresh',
          accountId: 'acc-test',
          activeProfileId: 'prof-test',
          expiresInSeconds: 900,
        ),
      );
      await tester.pump();
      controller.onPushNotificationData({
        'type': 'match_found',
        'match_id': 'match-1',
      });
      await _waitForMatch(container, 'match-1');
      await tester.pump();
      final secondClock = container
          .read(matchmakingMatchControllerProvider)
          .deadlineClock!;

      await tester.tap(find.byKey(MatchFoundOverlay.acceptButtonKey));
      await secondPostStarted.future.timeout(const Duration(seconds: 2));
      final secondFlight = controller.respond(true);
      expect(postCount, 2);
      expect(
        container.read(matchmakingMatchControllerProvider).isResponding,
        isTrue,
      );

      firstResponse.complete(_acceptedMatchResponse());
      await Future.wait([firstFlight, remountedDuplicate]);
      await tester.pump();
      expect(
        container.read(matchmakingMatchControllerProvider).isResponding,
        isTrue,
        reason: 'The stale first flight must not clear the replacement flight.',
      );
      expect(
        tester
            .widget<FilledButton>(find.byKey(MatchFoundOverlay.acceptButtonKey))
            .onPressed,
        isNull,
      );
      expect(container.read(activeSquadMatchProvider), isNull);
      expect(container.read(activeSearchSessionProvider), replacementSearch);
      expect(postCount, 2);
      expect(
        container.read(matchmakingMatchControllerProvider).deadlineClock,
        same(secondClock),
      );

      secondResponse.complete(_acceptedMatchResponse());
      await secondFlight;
      await tester.pump();
      expect(container.read(activeSquadMatchProvider)?.id, 'match-1');
      expect(container.read(matchmakingMatchControllerProvider).match, isNull);
      await tester.pump();
      expect(backgroundFocus.hasFocus, isTrue);
      expect(tester.takeException(), isNull);
    },
  );

  testWidgets('overlay disables actions when controller response is active', (
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
            body: Stack(
              children: [
                MatchFoundOverlay(match: _pendingMatch(), isResponding: true),
              ],
            ),
          ),
        ),
      ),
    );
    await tester.pump();
    expect(
      tester
          .widget<FilledButton>(find.byKey(MatchFoundOverlay.acceptButtonKey))
          .onPressed,
      isNull,
    );
    expect(
      tester
          .widget<OutlinedButton>(
            find.byKey(MatchFoundOverlay.declineButtonKey),
          )
          .onPressed,
      isNull,
    );
  });

  test('disposed container ignores late GetMatch results', () async {
    final getStarted = Completer<void>();
    final getResponse = Completer<http.Response>();
    final client = MockClient((request) async {
      getStarted.complete();
      return getResponse.future;
    });
    final container = ProviderContainer(
      overrides: voiceAppTestOverrides(client: client),
    );
    final controller = container.read(
      matchmakingMatchControllerProvider.notifier,
    );
    controller.onPushNotificationData({
      'type': 'match_found',
      'match_id': 'match-1',
    });
    await getStarted.future;
    container.dispose();
    getResponse.complete(_matchResponse(id: 'match-1', deadline: true));
    await Future<void>.delayed(const Duration(milliseconds: 20));
  });

  test(
    'partial accepted response preserves the server deadline clock',
    () async {
      var gets = 0;
      final client = MockClient((request) async {
        if (request.url.path.endsWith('/respond')) {
          return http.Response(
            jsonEncode({
              'match': {
                'id': 'match-1',
                'gameId': 'g-val',
                'mode': 'Duo',
                'region': 'eu',
                'status': 'active',
                'profileIds': ['prof-test', 'p2'],
              },
            }),
            200,
          );
        }
        gets++;
        return _matchResponse(id: 'match-1', deadline: true);
      });
      final container = ProviderContainer(
        overrides: voiceAppTestOverrides(client: client),
      );
      final controller = container.read(
        matchmakingMatchControllerProvider.notifier,
      );
      controller.onPushNotificationData({
        'type': 'match_found',
        'match_id': 'match-1',
      });
      await _waitForMatch(container, 'match-1');
      final originalClock = container
          .read(matchmakingMatchControllerProvider)
          .deadlineClock;
      await controller.respond(true);
      expect(gets, greaterThanOrEqualTo(2));
      expect(
        container.read(matchmakingMatchControllerProvider).deadlineClock,
        same(originalClock),
      );
      container.dispose();
    },
  );

  test('server-time refresh cannot increase the remaining countdown', () async {
    var gets = 0;
    final client = MockClient((request) async {
      gets++;
      return _matchResponse(id: 'match-1', deadline: true);
    });
    final container = ProviderContainer(
      overrides: voiceAppTestOverrides(client: client),
    );
    final controller = container.read(
      matchmakingMatchControllerProvider.notifier,
    );
    controller.onPushNotificationData({
      'type': 'match_found',
      'match_id': 'match-1',
    });
    await _waitForMatch(container, 'match-1');
    final before = container
        .read(matchmakingMatchControllerProvider)
        .deadlineClock!
        .remaining;
    await Future<void>.delayed(const Duration(milliseconds: 80));
    await controller.refreshMatch('match-1');
    final after = container
        .read(matchmakingMatchControllerProvider)
        .deadlineClock!
        .remaining;
    expect(gets, greaterThanOrEqualTo(2));
    expect(after, lessThanOrEqualTo(before));
    container.dispose();
  });

  testWidgets('deadline refresh retries pending results without overlap', (
    tester,
  ) async {
    tester.view.physicalSize = const Size(390, 844);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    var active = 0;
    var maximumActive = 0;
    var attempts = 0;
    final first = Completer<bool>();
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
                    serverNow: DateTime.utc(2026, 10, 5, 12),
                    deadline: DateTime.utc(2026, 10, 5, 12),
                    elapsed: () => Duration.zero,
                  ),
                  onRefresh: () async {
                    active++;
                    if (active > maximumActive) maximumActive = active;
                    attempts++;
                    if (attempts == 1) {
                      return first.future.whenComplete(() {
                        active--;
                      });
                    }
                    active--;
                    return attempts < 3;
                  },
                ),
              ],
            ),
          ),
        ),
      ),
    );
    await tester.pump();
    expect(attempts, 1);
    await tester.pump(const Duration(seconds: 1));
    expect(attempts, 1);
    first.complete(true);
    await tester.pump();
    await tester.pump(const Duration(seconds: 1));
    expect(attempts, 2);
    await tester.pump(const Duration(seconds: 2));
    expect(attempts, 3);
    await tester.pump(const Duration(seconds: 10));
    expect(attempts, 3);
    expect(maximumActive, 1);
    expect(tester.takeException(), isNull);
  });

  testWidgets('MatchFound is modal, keyboard contained, and restores focus', (
    tester,
  ) async {
    final backgroundFocus = FocusNode(debugLabel: 'background trigger');
    addTearDown(backgroundFocus.dispose);
    var showOverlay = false;
    late StateSetter update;
    await tester.pumpWidget(
      ProviderScope(
        overrides: voiceThemeTestOverrides(),
        child: MaterialApp(
          theme: voiceTestTheme(),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: StatefulBuilder(
            builder: (context, setState) {
              update = setState;
              return Scaffold(
                body: Stack(
                  children: [
                    Align(
                      alignment: Alignment.topLeft,
                      child: Focus(
                        focusNode: backgroundFocus,
                        child: TextButton(
                          key: const Key('background-trigger'),
                          onPressed: () {},
                          child: const Text('Background'),
                        ),
                      ),
                    ),
                    if (showOverlay) MatchFoundOverlay(match: _pendingMatch()),
                  ],
                ),
              );
            },
          ),
        ),
      ),
    );
    backgroundFocus.requestFocus();
    await tester.pump();
    update(() => showOverlay = true);
    await tester.pump();
    expect(
      tester
          .getSemantics(find.byKey(MatchFoundOverlay.modalSemanticsKey))
          .flagsCollection
          .scopesRoute,
      isTrue,
    );
    final acceptFocus = tester
        .widget<FilledButton>(find.byKey(MatchFoundOverlay.acceptButtonKey))
        .focusNode!;
    final declineFocus = tester
        .widget<OutlinedButton>(find.byKey(MatchFoundOverlay.declineButtonKey))
        .focusNode!;
    declineFocus.requestFocus();
    await tester.pump();
    await tester.sendKeyEvent(LogicalKeyboardKey.tab);
    await tester.pump();
    expect(FocusManager.instance.primaryFocus, same(acceptFocus));
    await tester.sendKeyDownEvent(LogicalKeyboardKey.shiftLeft);
    await tester.sendKeyEvent(LogicalKeyboardKey.tab);
    await tester.sendKeyUpEvent(LogicalKeyboardKey.shiftLeft);
    await tester.pump();
    expect(FocusManager.instance.primaryFocus, same(declineFocus));
    expect(
      tester
          .getSemantics(find.byKey(MatchFoundOverlay.timerKey))
          .flagsCollection
          .isLiveRegion,
      isTrue,
    );
    update(() => showOverlay = false);
    await tester.pumpAndSettle();
    expect(FocusManager.instance.primaryFocus, same(backgroundFocus));
  });
}

http.Response _matchResponse({required String id, required bool deadline}) {
  final serverNow = DateTime.now().toUtc();
  return http.Response(
    jsonEncode({
      'match': {
        'id': id,
        'gameId': 'g-val',
        'mode': 'Duo',
        'region': 'eu',
        'status': 'pending_accept',
        'profileIds': ['prof-test', 'p2'],
      },
      if (deadline) ...{
        'serverNow': serverNow.toIso8601String(),
        'acceptanceDeadlineAt': serverNow
            .add(const Duration(seconds: 30))
            .toIso8601String(),
      },
      'ownProposalResponse': 'pending',
    }),
    200,
  );
}

http.Response _acceptedMatchResponse() => http.Response(
  jsonEncode({
    'match': {
      'id': 'match-1',
      'gameId': 'g-val',
      'mode': 'Duo',
      'region': 'eu',
      'status': 'active',
      'profileIds': ['prof-test', 'p2'],
      'chatId': 'chat-1',
      'voiceRoomId': 'room-1',
    },
    'searchSession': {
      'id': 'session-1',
      'profileId': 'prof-test',
      'gameId': 'g-val',
      'mode': 'Duo',
      'criteriaJson': '{}',
      'status': 'matched',
    },
  }),
  200,
);
