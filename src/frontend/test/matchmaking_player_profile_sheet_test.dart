import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'dart:ui' as ui;

import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/matchmaking_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/theme/voice_theme.dart';
import 'package:voice_frontend/ui/matchmaking/match_squad_screen.dart';
import 'package:voice_frontend/ui/matchmaking/matchmaking_player_profile_sheet.dart';

import 'support/auth_test_overrides.dart';
import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

const _match = MatchData(
  id: 'match-1',
  gameId: 'game-1',
  mode: 'ranked',
  region: 'eu',
  status: 'active',
  profileIds: ['prof-test', 'profile-2'],
);

const _captureBoundaryKey = ValueKey('matchmaking_player_profile_capture');
var _captureFontsLoaded = false;

class _RecordingNavigatorObserver extends NavigatorObserver {
  final popped = <Route<dynamic>>[];
  final removed = <Route<dynamic>>[];

  @override
  void didPop(Route<dynamic> route, Route<dynamic>? previousRoute) {
    popped.add(route);
    super.didPop(route, previousRoute);
  }

  @override
  void didRemove(Route<dynamic> route, Route<dynamic>? previousRoute) {
    removed.add(route);
    super.didRemove(route, previousRoute);
  }
}

Widget _app(
  http.Client client, {
  ThemeData? theme,
  NavigatorObserver? navigatorObserver,
}) => RepaintBoundary(
  key: _captureBoundaryKey,
  child: ProviderScope(
    overrides: [
      ...voiceThemeTestOverrides(),
      profileAccentStorageProvider.overrideWithValue(testProfileAccentStorage),
      authSessionStorageProvider.overrideWithValue(
        InMemoryAuthSessionStorage(),
      ),
      authControllerProvider.overrideWith(authenticatedAuthController),
      realtimeAutoConnectProvider.overrideWithValue(false),
      gatewayConfigProvider.overrideWithValue(
        const GatewayConfig(baseUrl: 'http://api.test'),
      ),
      httpClientProvider.overrideWithValue(client),
    ],
    child: MaterialApp(
      debugShowCheckedModeBanner: false,
      theme: theme ?? voiceTestTheme(),
      locale: const Locale('en'),
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      navigatorObservers: [?navigatorObserver],
      home: const MatchSquadScreen(match: _match),
    ),
  ),
);

http.Response _profileResponse() => http.Response(
  jsonEncode({
    'profile': {
      'id': 'profile-2',
      'account_id': 'account-2',
      'username': 'teammate',
      'discriminator': '0042',
      'display_name': 'Teammate',
      'locale': 'en',
      'is_primary': true,
      'verification_type': 'none',
    },
  }),
  200,
);

http.Response _friendsResponse() => http.Response(
  jsonEncode({
    'friend_list': {'profile_ids': []},
  }),
  200,
);

http.Response _friendRequestsResponse() => http.Response(
  jsonEncode({
    'friend_request_list': {'incoming': [], 'outgoing': []},
  }),
  200,
);

http.Response _ratingResponse(double value) => http.Response(
  jsonEncode({
    'player_rating': {
      'profile_id': 'profile-2',
      'game_id': 'game-1',
      'rating_value': value,
      'games_played': 5,
    },
  }),
  200,
);

http.Response _fallbackResponse(http.Request request) {
  if (request.url.path.startsWith('/api/v1/chats/dm-permission/')) {
    return http.Response(jsonEncode({'allowed': true}), 200);
  }
  return http.Response('{}', 200);
}

void main() {
  testWidgets('profile load error retries and shows the selected profile', (
    tester,
  ) async {
    var profileRequests = 0;
    await tester.pumpWidget(
      _app(
        MockClient((request) async {
          switch (request.url.path) {
            case '/api/v1/users/profiles/profile-2':
              profileRequests++;
              return profileRequests == 1
                  ? http.Response('{}', 503)
                  : _profileResponse();
            case '/api/v1/matchmaking/players/profile-2/rating':
              return http.Response('{}', 404);
            case '/api/v1/friends':
              return _friendsResponse();
            case '/api/v1/friends/requests':
              return _friendRequestsResponse();
            default:
              return _fallbackResponse(request);
          }
        }),
      ),
    );

    await tester.tap(
      find.byKey(MatchSquadScreen.playerProfileKey('profile-2')),
    );
    await tester.pumpAndSettle();

    expect(find.text('Could not load profile'), findsOneWidget);
    expect(find.text('Try again'), findsOneWidget);
    expect(profileRequests, 1);

    await tester.tap(find.text('Try again'));
    await tester.pumpAndSettle();

    expect(profileRequests, 2);
    expect(find.text('Could not load profile'), findsNothing);
    expect(find.text('Teammate'), findsNWidgets(2));
    expect(find.byKey(MatchmakingPlayerProfileSheet.sheetKey), findsOneWidget);
  });

  testWidgets('participant opens selected profile with current match rating', (
    tester,
  ) async {
    final requests = <http.Request>[];
    await tester.pumpWidget(
      _app(
        MockClient((request) async {
          requests.add(request);
          switch (request.url.path) {
            case '/api/v1/users/profiles/profile-2':
              return _profileResponse();
            case '/api/v1/matchmaking/players/profile-2/rating':
              return _ratingResponse(4.8);
            case '/api/v1/friends':
              return _friendsResponse();
            case '/api/v1/friends/requests':
              return _friendRequestsResponse();
            default:
              return _fallbackResponse(request);
          }
        }),
      ),
    );

    await tester.tap(
      find.byKey(MatchSquadScreen.playerProfileKey('profile-2')),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(MatchmakingPlayerProfileSheet.sheetKey), findsOneWidget);
    expect(find.text('Teammate'), findsNWidgets(2));
    expect(find.byKey(MatchmakingPlayerProfileSheet.ratingKey), findsOneWidget);
    expect(find.byKey(MatchmakingPlayerProfileSheet.friendKey), findsOneWidget);
    expect(
      find.byKey(MatchmakingPlayerProfileSheet.messageKey),
      findsOneWidget,
    );
    expect(find.byKey(MatchmakingPlayerProfileSheet.banKey), findsOneWidget);
    final ratingRequest = requests.singleWhere(
      (request) => request.url.path.endsWith('/rating'),
    );
    expect(ratingRequest.url.queryParameters['game_id'], 'game-1');
    expect(ratingRequest.headers['authorization'], 'Bearer test-access');
    final semantics = tester.ensureSemantics();
    try {
      expect(
        tester
            .getSemantics(find.byKey(MatchmakingPlayerProfileSheet.messageKey))
            .flagsCollection
            .isButton,
        isTrue,
      );
    } finally {
      semantics.dispose();
    }
    await tester.sendKeyEvent(LogicalKeyboardKey.escape);
    await tester.pumpAndSettle();
    expect(find.byKey(MatchmakingPlayerProfileSheet.sheetKey), findsNothing);
  });

  testWidgets('DM permission denial hides the message action', (tester) async {
    final permissionRequests = <http.Request>[];
    await tester.pumpWidget(
      _app(
        MockClient((request) async {
          if (request.url.path.startsWith('/api/v1/chats/dm-permission/')) {
            permissionRequests.add(request);
            return http.Response(jsonEncode({'allowed': false}), 200);
          }
          switch (request.url.path) {
            case '/api/v1/users/profiles/profile-2':
              return _profileResponse();
            case '/api/v1/matchmaking/players/profile-2/rating':
              return _ratingResponse(4.8);
            case '/api/v1/friends':
              return _friendsResponse();
            case '/api/v1/friends/requests':
              return _friendRequestsResponse();
            default:
              return _fallbackResponse(request);
          }
        }),
      ),
    );

    await tester.tap(
      find.byKey(MatchSquadScreen.playerProfileKey('profile-2')),
    );
    await tester.pumpAndSettle();

    expect(permissionRequests, hasLength(1));
    expect(permissionRequests.single.method, 'GET');
    expect(
      permissionRequests.single.url.path,
      '/api/v1/chats/dm-permission/profile-2',
    );
    expect(find.byKey(MatchmakingPlayerProfileSheet.messageKey), findsNothing);
    expect(find.byKey(MatchmakingPlayerProfileSheet.friendKey), findsOneWidget);
  });

  testWidgets('DM permission failure can retry into the allowed action', (
    tester,
  ) async {
    var permissionRequests = 0;
    final requests = <http.Request>[];
    await tester.pumpWidget(
      _app(
        MockClient((request) async {
          requests.add(request);
          if (request.url.path.startsWith('/api/v1/chats/dm-permission/')) {
            permissionRequests++;
            return permissionRequests == 1
                ? http.Response('{}', 503)
                : http.Response(jsonEncode({'allowed': true}), 200);
          }
          switch (request.url.path) {
            case '/api/v1/users/profiles/profile-2':
              return _profileResponse();
            case '/api/v1/matchmaking/players/profile-2/rating':
              return _ratingResponse(4.8);
            case '/api/v1/friends':
              return _friendsResponse();
            case '/api/v1/friends/requests':
              return _friendRequestsResponse();
            default:
              return _fallbackResponse(request);
          }
        }),
      ),
    );

    await tester.tap(
      find.byKey(MatchSquadScreen.playerProfileKey('profile-2')),
    );
    await tester.pumpAndSettle();
    expect(find.text('Could not load chats'), findsOneWidget);
    expect(find.byKey(MatchmakingPlayerProfileSheet.messageKey), findsNothing);

    await tester.tap(
      find.byKey(MatchmakingPlayerProfileSheet.messagePermissionRetryKey),
    );
    await tester.pumpAndSettle();

    expect(permissionRequests, 2);
    expect(find.text('Could not load chats'), findsNothing);
    expect(
      find.byKey(MatchmakingPlayerProfileSheet.messageKey),
      findsOneWidget,
    );
    expect(requests.where((r) => r.url.path == '/api/v1/chats/dm'), isEmpty);
  });

  testWidgets('profile panel fits horizontal and vertical viewports', (
    tester,
  ) async {
    final captureDirectory = Platform.environment['MATCH_PROFILE_CAPTURE_DIR'];
    if (captureDirectory != null && captureDirectory.isNotEmpty) {
      await _loadProfileCaptureFonts(tester);
    }
    tester.view.physicalSize = const Size(390, 844);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    await tester.binding.setSurfaceSize(const Size(390, 844));
    addTearDown(() async => tester.binding.setSurfaceSize(null));

    await tester.pumpWidget(
      _app(
        MockClient((request) async {
          switch (request.url.path) {
            case '/api/v1/users/profiles/profile-2':
              return _profileResponse();
            case '/api/v1/matchmaking/players/profile-2/rating':
              return _ratingResponse(4.8);
            case '/api/v1/friends':
              return _friendsResponse();
            case '/api/v1/friends/requests':
              return _friendRequestsResponse();
            default:
              return _fallbackResponse(request);
          }
        }),
        theme: captureDirectory == null || captureDirectory.isEmpty
            ? null
            : _profileCaptureTheme(),
      ),
    );

    await tester.tap(
      find.byKey(MatchSquadScreen.playerProfileKey('profile-2')),
    );
    await tester.pumpAndSettle();
    expect(find.byKey(MatchmakingPlayerProfileSheet.sheetKey), findsOneWidget);
    expect(
      find.byKey(MatchmakingPlayerProfileSheet.ratingStarKey),
      findsOneWidget,
    );
    expect(tester.takeException(), isNull);
    await _captureProfileIfRequested(tester, 'V-390x844');

    tester.view.physicalSize = const Size(844, 390);
    tester.binding.handleMetricsChanged();
    await tester.binding.setSurfaceSize(const Size(844, 390));
    await tester.pumpAndSettle();
    expect(find.byKey(MatchmakingPlayerProfileSheet.sheetKey), findsOneWidget);
    expect(tester.takeException(), isNull);
    await _captureProfileIfRequested(tester, 'H-844x390');
  });

  testWidgets('ban requires confirmation and targets only selected teammate', (
    tester,
  ) async {
    final requests = <http.Request>[];
    await tester.pumpWidget(
      _app(
        MockClient((request) async {
          requests.add(request);
          switch (request.url.path) {
            case '/api/v1/users/profiles/profile-2':
              return _profileResponse();
            case '/api/v1/matchmaking/players/profile-2/rating':
              return _ratingResponse(4.8);
            case '/api/v1/friends':
              return _friendsResponse();
            case '/api/v1/friends/requests':
              return _friendRequestsResponse();
            case '/api/v1/matchmaking/bans':
              return http.Response('{}', 200);
            default:
              return _fallbackResponse(request);
          }
        }),
      ),
    );
    await tester.tap(
      find.byKey(MatchSquadScreen.playerProfileKey('profile-2')),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(MatchmakingPlayerProfileSheet.banKey));
    await tester.pumpAndSettle();
    expect(requests.where((r) => r.url.path.endsWith('/bans')), isEmpty);
    final l10n = AppLocalizations.of(tester.element(find.byType(AlertDialog)))!;
    await tester.tap(find.text(l10n.matchRatingBanCancel));
    await tester.pumpAndSettle();
    expect(requests.where((r) => r.url.path.endsWith('/bans')), isEmpty);

    await tester.tap(find.byKey(MatchmakingPlayerProfileSheet.banKey));
    await tester.pumpAndSettle();
    await tester.tap(find.text(l10n.matchRatingBanConfirm));
    await tester.pumpAndSettle();

    final ban = requests.singleWhere(
      (r) => r.url.path == '/api/v1/matchmaking/bans',
    );
    expect(ban.method, 'POST');
    expect(jsonDecode(ban.body), {'targetProfileId': 'profile-2'});
  });

  testWidgets('profile switch closes owned ban dialog and profile sheet', (
    tester,
  ) async {
    final requests = <http.Request>[];
    final navigatorObserver = _RecordingNavigatorObserver();
    await tester.pumpWidget(
      _app(
        MockClient((request) async {
          requests.add(request);
          switch (request.url.path) {
            case '/api/v1/users/profiles/profile-2':
              return _profileResponse();
            case '/api/v1/matchmaking/players/profile-2/rating':
              return _ratingResponse(4.8);
            case '/api/v1/friends':
              return _friendsResponse();
            case '/api/v1/friends/requests':
              return _friendRequestsResponse();
            default:
              return _fallbackResponse(request);
          }
        }),
        navigatorObserver: navigatorObserver,
      ),
    );

    await tester.tap(
      find.byKey(MatchSquadScreen.playerProfileKey('profile-2')),
    );
    await tester.pumpAndSettle();
    final sheetRoute = ModalRoute.of(
      tester.element(find.byKey(MatchmakingPlayerProfileSheet.sheetKey)),
    )!;
    await tester.tap(find.byKey(MatchmakingPlayerProfileSheet.banKey));
    await tester.pumpAndSettle();
    expect(find.byType(AlertDialog), findsOneWidget);
    final dialogRoute = ModalRoute.of(
      tester.element(find.byType(AlertDialog)),
    )!;

    final container = ProviderScope.containerOf(
      tester.element(find.byType(MatchSquadScreen)),
      listen: false,
    );
    container.read(authControllerProvider.notifier).state = const AuthState(
      session: AuthSession(
        accessToken: 'test-access',
        refreshToken: 'test-refresh',
        accountId: 'acc-test',
        activeProfileId: 'secondary-profile',
        expiresInSeconds: 900,
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byType(AlertDialog), findsNothing);
    expect(find.byKey(MatchmakingPlayerProfileSheet.sheetKey), findsNothing);
    expect([
      ...navigatorObserver.popped,
      ...navigatorObserver.removed,
    ], contains(same(dialogRoute)));
    expect([
      ...navigatorObserver.popped,
      ...navigatorObserver.removed,
    ], contains(same(sheetRoute)));
    expect(requests.where((r) => r.url.path.endsWith('/bans')), isEmpty);
  });

  testWidgets('friend and message actions use the selected profile ID', (
    tester,
  ) async {
    final requests = <http.Request>[];
    await tester.pumpWidget(
      _app(
        MockClient((request) async {
          requests.add(request);
          switch (request.url.path) {
            case '/api/v1/users/profiles/profile-2':
              return _profileResponse();
            case '/api/v1/matchmaking/players/profile-2/rating':
              return _ratingResponse(4.8);
            case '/api/v1/friends':
              return _friendsResponse();
            case '/api/v1/friends/requests':
              return _friendRequestsResponse();
            case '/api/v1/friends/invitations':
              return http.Response('{}', 200);
            case '/api/v1/chats/dm':
              return http.Response(
                jsonEncode({
                  'chat': {
                    'id': 'dm-1',
                    'type': 'CHAT_TYPE_DM',
                    'creator_profile_id': 'prof-test',
                  },
                }),
                200,
              );
            default:
              return _fallbackResponse(request);
          }
        }),
      ),
    );
    await tester.tap(
      find.byKey(MatchSquadScreen.playerProfileKey('profile-2')),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(MatchmakingPlayerProfileSheet.friendKey));
    await tester.pumpAndSettle();
    final invitation = requests.singleWhere(
      (r) => r.url.path == '/api/v1/friends/invitations',
    );
    expect(invitation.method, 'POST');
    expect(jsonDecode(invitation.body)['target_profile_id'], 'profile-2');

    await tester.tap(find.byKey(MatchmakingPlayerProfileSheet.messageKey));
    await tester.pumpAndSettle();
    final dm = requests.singleWhere((r) => r.url.path == '/api/v1/chats/dm');
    expect(dm.method, 'POST');
    expect(jsonDecode(dm.body)['other_profile_id'], 'profile-2');
  });

  testWidgets('changing viewer never reuses the previous viewer rating', (
    tester,
  ) async {
    final viewerBRating = Completer<http.Response>();
    var ratingRequests = 0;
    await tester.pumpWidget(
      _app(
        MockClient((request) async {
          switch (request.url.path) {
            case '/api/v1/users/profiles/profile-2':
              return _profileResponse();
            case '/api/v1/matchmaking/players/profile-2/rating':
              ratingRequests++;
              if (ratingRequests > 1) {
                return viewerBRating.future;
              }
              return _ratingResponse(4.8);
            case '/api/v1/friends':
              return _friendsResponse();
            case '/api/v1/friends/requests':
              return _friendRequestsResponse();
            default:
              return _fallbackResponse(request);
          }
        }),
      ),
    );

    await tester.tap(
      find.byKey(MatchSquadScreen.playerProfileKey('profile-2')),
    );
    await tester.pumpAndSettle();
    expect(find.text('MM rating: 4.8'), findsOneWidget);
    expect(find.byIcon(Icons.star), findsOneWidget);
    expect(
      tester
          .getSemantics(find.byKey(MatchmakingPlayerProfileSheet.ratingKey))
          .label,
      'MM rating: 4.8 ★',
    );

    final container = ProviderScope.containerOf(
      tester.element(find.byType(MatchSquadScreen)),
      listen: false,
    );
    container.read(authControllerProvider.notifier).state = const AuthState(
      session: AuthSession(
        accessToken: 'test-access',
        refreshToken: 'test-refresh',
        accountId: 'acc-test',
        activeProfileId: 'secondary-profile',
        expiresInSeconds: 900,
      ),
    );
    await tester.pump();
    expect(find.text('MM rating: 4.8'), findsNothing);
    expect(find.byKey(MatchmakingPlayerProfileSheet.sheetKey), findsNothing);

    await tester.tap(
      find.byKey(MatchSquadScreen.playerProfileKey('profile-2')),
    );
    await tester.pump();
    expect(find.text('MM rating: 4.8'), findsNothing);

    viewerBRating.complete(
      http.Response(jsonEncode({'error': 'permission_denied'}), 403),
    );
    await tester.pumpAndSettle();
    expect(find.text('MM rating: 4.8'), findsNothing);
  });

  testWidgets('late DM permission cannot cross active-profile switch', (
    tester,
  ) async {
    final viewerAPermission = Completer<http.Response>();
    var permissionRequests = 0;
    await tester.pumpWidget(
      _app(
        MockClient((request) async {
          if (request.url.path.startsWith('/api/v1/chats/dm-permission/')) {
            permissionRequests++;
            if (permissionRequests == 1) return viewerAPermission.future;
            return http.Response(jsonEncode({'allowed': false}), 200);
          }
          switch (request.url.path) {
            case '/api/v1/users/profiles/profile-2':
              return _profileResponse();
            case '/api/v1/matchmaking/players/profile-2/rating':
              return _ratingResponse(4.8);
            case '/api/v1/friends':
              return _friendsResponse();
            case '/api/v1/friends/requests':
              return _friendRequestsResponse();
            default:
              return _fallbackResponse(request);
          }
        }),
      ),
    );

    await tester.tap(
      find.byKey(MatchSquadScreen.playerProfileKey('profile-2')),
    );
    await tester.pump();
    expect(permissionRequests, 1);

    final container = ProviderScope.containerOf(
      tester.element(find.byType(MatchSquadScreen)),
      listen: false,
    );
    container.read(authControllerProvider.notifier).state = const AuthState(
      session: AuthSession(
        accessToken: 'test-access',
        refreshToken: 'test-refresh',
        accountId: 'acc-test',
        activeProfileId: 'secondary-profile',
        expiresInSeconds: 900,
      ),
    );
    await tester.pumpAndSettle();
    expect(find.byKey(MatchmakingPlayerProfileSheet.sheetKey), findsNothing);

    await tester.tap(
      find.byKey(MatchSquadScreen.playerProfileKey('profile-2')),
    );
    await tester.pumpAndSettle();
    expect(permissionRequests, 2);
    expect(find.byKey(MatchmakingPlayerProfileSheet.messageKey), findsNothing);

    viewerAPermission.complete(
      http.Response(jsonEncode({'allowed': true}), 200),
    );
    await tester.pumpAndSettle();
    expect(find.byKey(MatchmakingPlayerProfileSheet.messageKey), findsNothing);
  });

  testWidgets(
    'late rating cannot cross active-profile switch on same account',
    (tester) async {
      final viewerARating = Completer<http.Response>();
      final viewerBRating = Completer<http.Response>();
      var ratingRequests = 0;
      await tester.pumpWidget(
        _app(
          MockClient((request) async {
            switch (request.url.path) {
              case '/api/v1/users/profiles/profile-2':
                return _profileResponse();
              case '/api/v1/matchmaking/players/profile-2/rating':
                ratingRequests++;
                return ratingRequests == 1
                    ? viewerARating.future
                    : viewerBRating.future;
              case '/api/v1/friends':
                return _friendsResponse();
              case '/api/v1/friends/requests':
                return _friendRequestsResponse();
              default:
                return _fallbackResponse(request);
            }
          }),
        ),
      );

      await tester.tap(
        find.byKey(MatchSquadScreen.playerProfileKey('profile-2')),
      );
      await tester.pump();
      expect(ratingRequests, 1);

      final container = ProviderScope.containerOf(
        tester.element(find.byType(MatchSquadScreen)),
        listen: false,
      );
      container.read(authControllerProvider.notifier).state = const AuthState(
        session: AuthSession(
          accessToken: 'test-access',
          refreshToken: 'test-refresh',
          accountId: 'acc-test',
          activeProfileId: 'secondary-profile',
          expiresInSeconds: 900,
        ),
      );
      await tester.pump();
      expect(find.byKey(MatchmakingPlayerProfileSheet.sheetKey), findsNothing);

      await tester.tap(
        find.byKey(MatchSquadScreen.playerProfileKey('profile-2')),
      );
      await tester.pump();
      expect(ratingRequests, greaterThanOrEqualTo(2));
      expect(find.text('MM rating: 4.8'), findsNothing);

      viewerARating.complete(_ratingResponse(4.8));
      await tester.pump();
      expect(find.text('MM rating: 4.8'), findsNothing);

      viewerBRating.complete(
        http.Response(jsonEncode({'error': 'permission_denied'}), 403),
      );
      await tester.pumpAndSettle();
      expect(find.text('MM rating: 4.8'), findsNothing);
    },
  );
}

Future<void> _loadProfileCaptureFonts(WidgetTester tester) async {
  if (_captureFontsLoaded) return;
  final loaded = await tester.runAsync(() async {
    final noto = FontLoader(VoiceTheme.fontFamily)
      ..addFont(rootBundle.load('assets/fonts/NotoSans-Regular.ttf'))
      ..addFont(rootBundle.load('assets/fonts/NotoSans-Medium.ttf'))
      ..addFont(rootBundle.load('assets/fonts/NotoSans-SemiBold.ttf'))
      ..addFont(rootBundle.load('assets/fonts/NotoSans-Bold.ttf'));
    await noto.load();

    final flutterRoot = Platform.environment['FLUTTER_ROOT'];
    if (flutterRoot == null || flutterRoot.isEmpty) {
      throw StateError('FLUTTER_ROOT is required to load Material Icons');
    }
    final iconFont = File(
      [
        flutterRoot,
        'bin',
        'cache',
        'artifacts',
        'material_fonts',
        'MaterialIcons-Regular.otf',
      ].join(Platform.pathSeparator),
    );
    final bytes = await iconFont.readAsBytes();
    final iconData = ByteData.sublistView(Uint8List.fromList(bytes));
    await (FontLoader(
      'MaterialIcons',
    )..addFont(Future<ByteData>.value(iconData))).load();
    return true;
  });
  if (loaded != true) throw StateError('Flutter font loading did not complete');
  _captureFontsLoaded = true;
}

ThemeData _profileCaptureTheme() {
  final theme = voiceTestTheme();
  return theme.copyWith(
    textTheme: theme.textTheme.apply(fontFamily: VoiceTheme.fontFamily),
    primaryTextTheme: theme.primaryTextTheme.apply(
      fontFamily: VoiceTheme.fontFamily,
    ),
  );
}

Future<void> _captureProfileIfRequested(
  WidgetTester tester,
  String filename,
) async {
  final captureDirectory = Platform.environment['MATCH_PROFILE_CAPTURE_DIR'];
  if (captureDirectory == null || captureDirectory.isEmpty) return;
  final boundary = tester.renderObject<RenderRepaintBoundary>(
    find.byKey(_captureBoundaryKey),
  );
  final written = await tester.runAsync(() async {
    final image = await boundary.toImage(pixelRatio: 1);
    try {
      final png = await image.toByteData(format: ui.ImageByteFormat.png);
      if (png == null) throw StateError('Flutter did not encode the capture');
      final directory = Directory(captureDirectory);
      await directory.create(recursive: true);
      await File(
        '${directory.path}${Platform.pathSeparator}$filename.png',
      ).writeAsBytes(png.buffer.asUint8List(), flush: true);
      return true;
    } finally {
      image.dispose();
    }
  });
  if (written != true) throw StateError('Flutter capture did not complete');
}
