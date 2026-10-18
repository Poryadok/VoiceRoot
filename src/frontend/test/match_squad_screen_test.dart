import 'dart:async';
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/matchmaking_client.dart';
import 'package:voice_frontend/backend/users_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/state/matchmaking_rating_controller.dart';
import 'package:voice_frontend/state/social_providers.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/ui/matchmaking/match_squad_screen.dart';
import 'package:voice_frontend/ui/chat/chat_room_panel.dart';
import 'package:voice_frontend/ui/matchmaking/match_rating_overlay.dart';
import 'package:voice_frontend/ui/matchmaking/match_history_screen.dart';
import 'package:voice_frontend/ui/matchmaking/queue_search_screen.dart';
import 'package:voice_frontend/ui/social/social_panel.dart';

import 'support/auth_test_overrides.dart';
import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

Future<void> _loadGoldenFonts() async {
  await (FontLoader(
    'MaterialIcons',
  )..addFont(rootBundle.load('fonts/MaterialIcons-Regular.otf'))).load();
  await (FontLoader('Noto Sans')
        ..addFont(rootBundle.load('assets/fonts/NotoSans-Regular.ttf'))
        ..addFont(rootBundle.load('assets/fonts/NotoSans-Medium.ttf'))
        ..addFont(rootBundle.load('assets/fonts/NotoSans-SemiBold.ttf'))
        ..addFont(rootBundle.load('assets/fonts/NotoSans-Bold.ttf')))
      .load();
}

Widget _postMatchFixture(http.Client client) => ProviderScope(
  overrides: [
    ...voiceThemeTestOverrides(),
    profileAccentStorageProvider.overrideWithValue(testProfileAccentStorage),
    authSessionStorageProvider.overrideWithValue(InMemoryAuthSessionStorage()),
    authControllerProvider.overrideWith(authenticatedAuthController),
    selectedChatIdProvider.overrideWith((ref) => 'inactive-chat'),
    realtimeAutoConnectProvider.overrideWithValue(false),
    gatewayConfigProvider.overrideWithValue(
      const GatewayConfig(baseUrl: 'http://localhost:9999'),
    ),
    httpClientProvider.overrideWithValue(client),
    profileProvider.overrideWith(
      (ref, id) async => VoiceProfile(
        id: id,
        accountId: 'fixture-account-$id',
        username: id,
        discriminator: '0001',
        displayName: _fixtureDisplayNames[id] ?? id,
      ),
    ),
  ],
  child: MaterialApp(
    theme: voiceTestTheme().copyWith(
      textTheme: voiceTestTheme().textTheme.apply(fontFamily: 'Noto Sans'),
      primaryTextTheme: voiceTestTheme().primaryTextTheme.apply(
        fontFamily: 'Noto Sans',
      ),
    ),
    locale: const Locale('en'),
    localizationsDelegates: AppLocalizations.localizationsDelegates,
    supportedLocales: AppLocalizations.supportedLocales,
    home: Stack(
      children: const [
        Scaffold(body: SocialPanel()),
        MatchRatingOverlayHost(),
      ],
    ),
  ),
);

MatchData _activeMatch() => MatchData(
  id: 'match-1',
  gameId: 'game-1',
  mode: 'ranked',
  region: 'eu',
  status: 'active',
  profileIds: const ['prof-test', 'profile-2'],
);

const _fixtureDisplayNames = {
  'prof-test': 'Alex Chen',
  'profile-2': 'Maria Santos',
  'profile-3': 'Dima Petrov',
};

Future<void> _openSquadAndLeave(WidgetTester tester) async {
  Navigator.of(tester.element(find.byKey(SocialPanel.panelKey))).push<void>(
    MaterialPageRoute<void>(
      builder: (_) => MatchSquadScreen(match: _activeMatch()),
    ),
  );
  await tester.pumpAndSettle();
  await tester.tap(find.byKey(MatchSquadScreen.leaveButtonKey));
  await tester.pumpAndSettle();
}

void main() {
  setUpAll(_loadGoldenFonts);

  Widget testApp({required Widget home, String gatewayBaseUrl = ''}) {
    return ProviderScope(
      overrides: [
        ...voiceThemeTestOverrides(),
        profileAccentStorageProvider.overrideWithValue(
          testProfileAccentStorage,
        ),
        authSessionStorageProvider.overrideWithValue(
          InMemoryAuthSessionStorage(),
        ),
        authControllerProvider.overrideWith(authenticatedAuthController),
        selectedChatIdProvider.overrideWith((ref) => 'inactive-chat'),
        realtimeAutoConnectProvider.overrideWithValue(false),
        gatewayConfigProvider.overrideWithValue(
          GatewayConfig(baseUrl: gatewayBaseUrl),
        ),
        profileProvider.overrideWith(
          (ref, id) async => VoiceProfile(
            id: id,
            accountId: 'fixture-account-$id',
            username: id,
            discriminator: '0001',
            displayName: _fixtureDisplayNames[id] ?? id,
          ),
        ),
      ],
      child: MaterialApp(
        theme: voiceTestTheme().copyWith(
          textTheme: voiceTestTheme().textTheme.apply(fontFamily: 'Noto Sans'),
          primaryTextTheme: voiceTestTheme().primaryTextTheme.apply(
            fontFamily: 'Noto Sans',
          ),
        ),
        locale: const Locale('en'),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: home,
      ),
    );
  }

  testWidgets('MatchSquadScreen offers explicit voice join and chat actions', (
    tester,
  ) async {
    await tester.pumpWidget(
      testApp(
        gatewayBaseUrl: 'http://localhost:9999',
        home: MatchSquadScreen(
          match: MatchData(
            id: 'match-1',
            gameId: 'game-1',
            gameName: 'Valorant',
            mode: 'ranked',
            region: 'eu',
            status: 'active',
            profileIds: const ['profile-1'],
            chatId: 'chat-1',
            voiceRoomId: 'room-1',
          ),
        ),
      ),
    );
    await tester.pump();

    expect(find.byKey(MatchSquadScreen.voiceSectionKey), findsNothing);
    expect(find.byKey(MatchSquadScreen.leaveButtonKey), findsOneWidget);
    expect(find.byKey(MatchSquadScreen.joinVoiceButtonKey), findsOneWidget);
  });

  for (final layout in [
    (name: 'horizontal', size: const Size(1280, 800)),
    (name: 'vertical', size: const Size(390, 844)),
  ]) {
    testWidgets('MatchSquad ${layout.name} layout matches its screenshot', (
      tester,
    ) async {
      tester.view.physicalSize = layout.size;
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);

      await tester.pumpWidget(
        testApp(
          gatewayBaseUrl: 'http://localhost:9999',
          home: MatchSquadScreen(
            match: MatchData(
              id: 'match-visual',
              gameId: 'game-1',
              gameName: 'Valorant',
              mode: 'ranked',
              region: 'eu',
              status: 'active',
              profileIds: const ['prof-test', 'profile-2', 'profile-3'],
              chatId: 'chat-visual',
              voiceRoomId: 'room-visual',
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();

      expect(tester.takeException(), isNull);
      expect(find.byKey(MatchSquadScreen.joinVoiceButtonKey), findsOneWidget);
      expect(find.byKey(MatchSquadScreen.openChatButtonKey), findsOneWidget);
      expect(find.byKey(MatchSquadScreen.leaveButtonKey), findsOneWidget);
      await expectLater(
        find.byType(MatchSquadScreen),
        matchesGoldenFile('goldens/match_squad_${layout.name}.png'),
      );
      expect(find.text('Valorant · ranked'), findsOneWidget);
      expect(find.text('Alex Chen'), findsOneWidget);
      expect(find.text('Maria Santos'), findsOneWidget);
      expect(find.text('Dima Petrov'), findsOneWidget);
    });
  }

  testWidgets('MatchSquad controls expose labels and keyboard focus', (
    tester,
  ) async {
    final semantics = tester.ensureSemantics();
    // Dispose explicitly because the test binding checks semantics handles
    // before invoking registered tear-down callbacks.
    await tester.pumpWidget(
      testApp(
        home: MatchSquadScreen(
          match: MatchData(
            id: 'match-accessibility',
            gameId: 'game-1',
            mode: 'ranked',
            region: 'eu',
            status: 'active',
            profileIds: const ['profile-1'],
            chatId: 'chat-accessibility',
            voiceRoomId: 'room-accessibility',
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.bySemanticsLabel('Join voice'), findsOneWidget);
    expect(find.bySemanticsLabel('Open chat'), findsOneWidget);
    expect(find.bySemanticsLabel('Leave squad'), findsOneWidget);

    await tester.sendKeyEvent(LogicalKeyboardKey.tab);
    await tester.pump();
    final firstTabFocus = tester.binding.focusManager.primaryFocus;
    expect(firstTabFocus, isNotNull);
    semantics.dispose();
  });

  testWidgets(
    'MatchSquad joins voice only after the documented Join voice action',
    (tester) async {
      final voicePaths = <String>[];
      final client = MockClient((request) async {
        if (request.url.path.contains('/voice/')) {
          voicePaths.add(request.url.path);
        }
        switch (request.url.path) {
          case '/api/v1/matchmaking/matches/match-voice/voice/join':
            return http.Response(
              jsonEncode({
                'call_session': {
                  'room_id': 'room-voice',
                  'livekit_room_name': 'squad-room',
                  'room_type_enum': 'VOICE_SESSION_KIND_GROUP_VOICE',
                  'status': 'CALL_STATUS_ACTIVE',
                },
                'media_epoch': 'epoch-current',
                'membership_state': 'MATCH_SQUAD_MEMBERSHIP_STATE_JOINED',
              }),
              200,
            );
          case '/api/v1/matchmaking/matches/match-voice/voice/token':
            return http.Response(
              jsonEncode({
                'token': {'jwt': 'opaque'},
                'media_epoch': 'stale-epoch',
              }),
              200,
            );
          default:
            return http.Response('{}', 404);
        }
      });

      await tester.pumpWidget(_postMatchFixture(client));
      await tester.pumpAndSettle();
      Navigator.of(tester.element(find.byKey(SocialPanel.panelKey))).push<void>(
        MaterialPageRoute<void>(
          builder: (_) => MatchSquadScreen(
            match: MatchData(
              id: 'match-voice',
              gameId: 'game-1',
              mode: 'ranked',
              region: 'eu',
              status: 'active',
              profileIds: const ['prof-test', 'profile-2'],
              voiceRoomId: 'room-voice',
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();

      expect(voicePaths, isEmpty);
      expect(find.text('Join voice'), findsOneWidget);

      await tester.tap(find.text('Join voice'));
      await tester.pumpAndSettle();

      expect(voicePaths, [
        '/api/v1/matchmaking/matches/match-voice/voice/join',
        '/api/v1/matchmaking/matches/match-voice/voice/token',
      ]);
    },
  );

  testWidgets('MatchSquad opens its existing chat only on request', (
    tester,
  ) async {
    final client = MockClient((_) async => http.Response('{}', 404));
    await tester.pumpWidget(_postMatchFixture(client));
    await tester.pumpAndSettle();
    Navigator.of(tester.element(find.byKey(SocialPanel.panelKey))).push<void>(
      MaterialPageRoute<void>(
        builder: (_) => MatchSquadScreen(
          match: MatchData(
            id: 'match-chat',
            gameId: 'game-1',
            mode: 'ranked',
            region: 'eu',
            status: 'active',
            profileIds: const ['prof-test', 'profile-2'],
            chatId: 'chat-match',
            voiceRoomId: 'room-chat',
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Open chat'), findsOneWidget);
    expect(find.byType(ChatRoomPanel), findsNothing);

    await tester.tap(find.text('Open chat'));
    await tester.pumpAndSettle();

    expect(find.byType(ChatRoomPanel), findsOneWidget);
    await tester.tap(find.byTooltip('Back'));
    await tester.pumpAndSettle();
    expect(find.text('Join voice'), findsOneWidget);
  });

  testWidgets('MatchSquad lets the user retry a failed voice join', (
    tester,
  ) async {
    var joinAttempts = 0;
    final voicePaths = <String>[];
    final client = MockClient((request) async {
      if (request.url.path.contains('/voice/')) {
        voicePaths.add(request.url.path);
      }
      if (request.url.path.endsWith('/voice/join')) {
        joinAttempts++;
        if (joinAttempts == 1) return http.Response('{}', 503);
        return http.Response(
          jsonEncode({
            'call_session': {
              'room_id': 'room-voice',
              'livekit_room_name': 'squad-room',
              'room_type_enum': 'VOICE_SESSION_KIND_GROUP_VOICE',
              'status': 'CALL_STATUS_ACTIVE',
            },
            'media_epoch': 'epoch-current',
            'membership_state': 'MATCH_SQUAD_MEMBERSHIP_STATE_JOINED',
          }),
          200,
        );
      }
      if (request.url.path.endsWith('/voice/token')) {
        return http.Response(
          jsonEncode({
            'token': {'jwt': 'opaque'},
            'media_epoch': 'stale-epoch',
          }),
          200,
        );
      }
      return http.Response('{}', 404);
    });

    await tester.pumpWidget(_postMatchFixture(client));
    await tester.pumpAndSettle();
    Navigator.of(tester.element(find.byKey(SocialPanel.panelKey))).push<void>(
      MaterialPageRoute<void>(
        builder: (_) => MatchSquadScreen(
          match: MatchData(
            id: 'match-voice',
            gameId: 'game-1',
            mode: 'ranked',
            region: 'eu',
            status: 'active',
            profileIds: const ['prof-test', 'profile-2'],
            voiceRoomId: 'room-voice',
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    final joinButton = find.byKey(MatchSquadScreen.joinVoiceButtonKey);
    await tester.tap(joinButton);
    await tester.pumpAndSettle();
    expect(joinAttempts, 1);
    expect(joinButton, findsOneWidget);
    expect(tester.widget<FilledButton>(joinButton).onPressed, isNotNull);

    await tester.tap(joinButton);
    await tester.pumpAndSettle();
    expect(joinAttempts, 2);
    expect(
      voicePaths.where((path) => path.endsWith('/voice/join')),
      hasLength(2),
    );
    expect(
      voicePaths.where((path) => path.endsWith('/voice/token')),
      hasLength(1),
    );
    expect(voicePaths.where((path) => path.endsWith('/voice/leave')), isEmpty);
    expect(voicePaths.where((path) => path.endsWith('/complete')), isEmpty);
  });

  testWidgets('MatchSquad leaves Voice membership before completing the match', (
    tester,
  ) async {
    final paths = <String>[];
    final client = MockClient((request) async {
      paths.add(request.url.path);
      switch (request.url.path) {
        case '/api/v1/matchmaking/matches/match-voice/voice/join':
          return http.Response(
            jsonEncode({
              'call_session': {
                'room_id': 'room-voice',
                'livekit_room_name': 'squad-room',
                'room_type_enum': 'VOICE_SESSION_KIND_GROUP_VOICE',
                'status': 'CALL_STATUS_ACTIVE',
              },
              'media_epoch': 'epoch-current',
              'membership_state': 'MATCH_SQUAD_MEMBERSHIP_STATE_JOINED',
            }),
            200,
          );
        case '/api/v1/matchmaking/matches/match-voice/voice/token':
          // A stale token binding prevents media connection while retaining the
          // successful server membership for the explicit Leave call below.
          return http.Response(
            jsonEncode({
              'token': {'jwt': 'opaque'},
              'media_epoch': 'stale-epoch',
            }),
            200,
          );
        case '/api/v1/matchmaking/matches/match-voice/voice/leave':
          return http.Response(
            jsonEncode({
              'call_session': {
                'room_id': 'room-voice',
                'livekit_room_name': 'squad-room',
                'room_type_enum': 'VOICE_SESSION_KIND_GROUP_VOICE',
                'status': 'CALL_STATUS_ACTIVE',
              },
              'media_epoch': 'epoch-current',
              'membership_state': 'MATCH_SQUAD_MEMBERSHIP_STATE_LEFT',
            }),
            200,
          );
        case '/api/v1/matchmaking/matches/match-voice/complete':
          return http.Response(
            jsonEncode({
              'match': {
                'id': 'match-voice',
                'game_id': 'game-1',
                'mode': 'ranked',
                'region': 'eu',
                'status': 'completed',
                'profile_ids': ['prof-test', 'profile-2'],
              },
            }),
            200,
          );
        default:
          return http.Response('{}', 404);
      }
    });

    await tester.pumpWidget(_postMatchFixture(client));
    await tester.pumpAndSettle();
    Navigator.of(tester.element(find.byKey(SocialPanel.panelKey))).push<void>(
      MaterialPageRoute<void>(
        builder: (_) => MatchSquadScreen(
          match: MatchData(
            id: 'match-voice',
            gameId: 'game-1',
            mode: 'ranked',
            region: 'eu',
            status: 'active',
            profileIds: const ['prof-test', 'profile-2'],
            voiceRoomId: 'room-voice',
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(MatchSquadScreen.joinVoiceButtonKey));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 300));
    await tester.tap(find.byKey(MatchSquadScreen.leaveButtonKey));
    await tester.pumpAndSettle();

    expect(paths.where((path) => path.contains('/voice/')).toList(), [
      '/api/v1/matchmaking/matches/match-voice/voice/join',
      '/api/v1/matchmaking/matches/match-voice/voice/token',
      '/api/v1/matchmaking/matches/match-voice/voice/leave',
    ]);
    expect(
      paths.indexOf('/api/v1/matchmaking/matches/match-voice/voice/leave'),
      lessThan(
        paths.indexOf('/api/v1/matchmaking/matches/match-voice/complete'),
      ),
    );
  });

  testWidgets('CompleteMatch retries reuse the same actor operation ID', (
    tester,
  ) async {
    final operationIds = <String>[];
    var completeAttempts = 0;
    final client = MockClient((request) async {
      if (request.url.path.endsWith('/complete')) {
        final payload = jsonDecode(request.body) as Map<String, dynamic>;
        operationIds.add(payload['operationId'] as String);
        completeAttempts++;
        if (completeAttempts == 1) return http.Response('{}', 503);
        return http.Response(
          jsonEncode({
            'match': {
              'id': 'match-1',
              'gameId': 'game-1',
              'mode': 'ranked',
              'region': 'eu',
              'status': 'completed',
              'profileIds': ['prof-test', 'profile-2'],
            },
          }),
          200,
        );
      }
      return http.Response('{}', 200);
    });
    await tester.pumpWidget(_postMatchFixture(client));
    await tester.pumpAndSettle();
    await _openSquadAndLeave(tester);
    await tester.tap(find.byKey(MatchSquadScreen.leaveButtonKey));
    await tester.pumpAndSettle();

    expect(operationIds, hasLength(2));
    expect(operationIds.first, isNotEmpty);
    expect(operationIds[1], operationIds.first);
  });

  testWidgets('MatchSquadScreen exposes another participant profile entry', (
    tester,
  ) async {
    await tester.pumpWidget(
      testApp(
        home: MatchSquadScreen(
          match: MatchData(
            id: 'match-1',
            gameId: 'game-1',
            mode: 'ranked',
            region: 'eu',
            status: 'active',
            profileIds: const ['prof-test', 'profile-2'],
          ),
        ),
      ),
    );
    await tester.scrollUntilVisible(
      find.byKey(const Key('match_squad_player_profile-2')),
      160,
    );

    expect(
      find.byKey(const Key('match_squad_player_profile-2')),
      findsOneWidget,
    );
    final selfRow = tester.widget<ListTile>(
      find.byKey(const Key('match_squad_player_prof-test')),
    );
    expect(selfRow.onTap, isNull);
    final teammateRow = tester.widget<ListTile>(
      find.byKey(const Key('match_squad_player_profile-2')),
    );
    expect(teammateRow.onTap, isNotNull);
  });

  testWidgets('leaving a squad enters rating and the Social history consumer', (
    tester,
  ) async {
    final requests = <http.Request>[];
    final client = MockClient((request) async {
      requests.add(request);
      switch (request.url.path) {
        case '/api/v1/matchmaking/matches/match-1/complete':
          return http.Response(
            jsonEncode({
              'match': {
                'id': 'match-1',
                'gameId': 'game-1',
                'mode': 'ranked',
                'region': 'eu',
                'status': 'completed',
                'profileIds': ['prof-test', 'profile-2'],
              },
            }),
            200,
          );
        case '/api/v1/matchmaking/matches/match-1/rate':
        case '/api/v1/matchmaking/bans':
          return http.Response('{}', 200);
        case '/api/v1/matchmaking/profile/me/matches':
          return http.Response(
            jsonEncode({
              'matchList': {
                'matches': [
                  {
                    'id': 'match-1',
                    'gameId': 'game-1',
                    'mode': 'ranked',
                    'region': 'eu',
                    'status': 'completed',
                    'profileIds': ['prof-test', 'profile-2'],
                  },
                ],
                'nextCursor': '',
              },
            }),
            200,
          );
        case '/api/v1/matchmaking/search':
          return http.Response(
            jsonEncode({
              'search_session': {
                'id': 'session-next',
                'profile_id': 'prof-test',
                'game_id': 'game-1',
                'mode': 'ranked',
                'criteria_json': '{}',
                'status': 'searching',
              },
            }),
            200,
          );
        case '/api/v1/matchmaking/games':
          return http.Response(
            jsonEncode({
              'gameList': {
                'games': [
                  {
                    'id': 'game-1',
                    'name': 'Valorant',
                    'status': 'active',
                    'configJson': jsonEncode({
                      'regions': ['eu'],
                      'modes': [
                        {
                          'name': 'ranked',
                          'slots': 2,
                          'party_size_min': 1,
                          'party_size_max': 2,
                        },
                      ],
                    }),
                  },
                ],
                'nextCursor': '',
              },
            }),
            200,
          );
        default:
          return http.Response('{}', 404);
      }
    });

    await tester.pumpWidget(_postMatchFixture(client));
    await tester.pumpAndSettle();
    await _openSquadAndLeave(tester);

    expect(find.byKey(MatchRatingOverlay.submitButtonKey), findsOneWidget);
    await tester.tap(find.byKey(MatchRatingOverlay.starButtonKey(1)));
    await tester.tap(find.byKey(MatchRatingOverlay.submitButtonKey));
    await tester.pump();
    await tester.pump(const Duration(seconds: 1));
    expect(find.byType(AlertDialog), findsOneWidget);
    final l10n = AppLocalizations.of(tester.element(find.byType(AlertDialog)))!;
    await tester.tap(find.text(l10n.matchRatingBanConfirm));
    await tester.pump();
    await tester.pump(const Duration(seconds: 1));

    expect(
      requests.map((request) => request.url.path),
      containsAll([
        '/api/v1/matchmaking/matches/match-1/complete',
        '/api/v1/matchmaking/matches/match-1/rate',
        '/api/v1/matchmaking/bans',
      ]),
    );
    final ratingRequest = requests.singleWhere(
      (request) => request.url.path.endsWith('/rate'),
    );
    expect(jsonDecode(ratingRequest.body), {
      'ratedProfileId': 'profile-2',
      'stars': 1,
    });
    final banRequest = requests.singleWhere(
      (request) => request.url.path == '/api/v1/matchmaking/bans',
    );
    expect(jsonDecode(banRequest.body), {
      'targetProfileId': 'profile-2',
      'reason': 'low_match_rating',
    });

    await tester.tap(find.byKey(const Key('social_match_history_entry')));
    await tester.pumpAndSettle();
    expect(find.byKey(MatchHistoryScreen.listKey), findsOneWidget);
    expect(
      find.byKey(MatchHistoryScreen.matchTileKey('match-1')),
      findsOneWidget,
    );
    expect(find.text('Valorant'), findsOneWidget);
    expect(
      requests.map((request) => request.url.path),
      contains('/api/v1/matchmaking/profile/me/matches'),
    );

    await tester.pageBack();
    await tester.pumpAndSettle();
    final socialL10n = AppLocalizations.of(
      tester.element(find.byKey(SocialPanel.panelKey)),
    )!;
    await tester.tap(find.text(socialL10n.gameCatalogEntry));
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('game_catalog_card_game-1')), findsOneWidget);
    await tester.tap(find.byKey(const Key('game_catalog_card_game-1')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('game_detail_start_queue_ranked')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(QueueSearchScreen.startButtonKey));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 200));

    expect(find.byKey(QueueSearchScreen.searchingStateKey), findsOneWidget);
    final nextSearch = requests.singleWhere(
      (request) => request.url.path == '/api/v1/matchmaking/search',
    );
    expect(nextSearch.method, 'POST');
    expect(jsonDecode(nextSearch.body), containsPair('gameId', 'game-1'));
    expect(jsonDecode(nextSearch.body), containsPair('mode', 'ranked'));
  });

  testWidgets('rating skip-all cancels all teammate requests', (tester) async {
    final requests = <http.Request>[];
    final client = MockClient((request) async {
      requests.add(request);
      if (request.url.path.endsWith('/complete')) {
        return http.Response(
          jsonEncode({
            'match': {
              'id': 'match-1',
              'gameId': 'game-1',
              'mode': 'ranked',
              'region': 'eu',
              'status': 'completed',
              'profileIds': ['prof-test', 'profile-2'],
            },
          }),
          200,
        );
      }
      return http.Response('{}', 200);
    });
    await tester.pumpWidget(_postMatchFixture(client));
    await tester.pumpAndSettle();
    await _openSquadAndLeave(tester);

    await tester.tap(find.byKey(MatchRatingOverlay.skipAllButtonKey));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 200));

    expect(
      requests.where(
        (request) =>
            request.url.path.endsWith('/rate') ||
            request.url.path == '/api/v1/matchmaking/bans',
      ),
      isEmpty,
    );
    expect(find.byKey(MatchRatingOverlay.submitButtonKey), findsNothing);
  });

  testWidgets('failed rating prevents ban and duplicate submission', (
    tester,
  ) async {
    final rateResponse = Completer<http.Response>();
    final requests = <http.Request>[];
    final client = MockClient((request) async {
      requests.add(request);
      if (request.url.path.endsWith('/complete')) {
        return http.Response(
          jsonEncode({
            'match': {
              'id': 'match-1',
              'gameId': 'game-1',
              'mode': 'ranked',
              'region': 'eu',
              'status': 'completed',
              'profileIds': ['prof-test', 'profile-2'],
            },
          }),
          200,
        );
      }
      if (request.url.path.endsWith('/rate')) return rateResponse.future;
      return http.Response('{}', 200);
    });
    await tester.pumpWidget(_postMatchFixture(client));
    await tester.pumpAndSettle();
    await _openSquadAndLeave(tester);

    await tester.tap(find.byKey(MatchRatingOverlay.starButtonKey(1)));
    await tester.tap(find.byKey(MatchRatingOverlay.submitButtonKey));
    await tester.pump();
    await tester.tap(find.byKey(MatchRatingOverlay.submitButtonKey));
    await tester.pump();
    expect(
      requests.where((request) => request.url.path.endsWith('/rate')),
      hasLength(1),
    );

    rateResponse.complete(
      http.Response(jsonEncode({'message': 'private rating failure'}), 500),
    );
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 300));
    final l10n = AppLocalizations.of(
      tester.element(find.byKey(SocialPanel.panelKey)),
    )!;
    expect(find.text(l10n.matchRatingSubmitError), findsOneWidget);
    expect(find.text('private rating failure'), findsNothing);
    expect(find.byType(AlertDialog), findsNothing);
    expect(
      requests.where(
        (request) => request.url.path == '/api/v1/matchmaking/bans',
      ),
      isEmpty,
    );
  });
}
