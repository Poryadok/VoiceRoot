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
import 'package:voice_frontend/backend/matchmaking_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/game_catalog_recent_provider.dart';
import 'package:voice_frontend/theme/voice_theme.dart';
import 'package:voice_frontend/theme/voice_token_catalog.dart';
import 'package:voice_frontend/ui/matchmaking/add_game_screen.dart';
import 'package:voice_frontend/ui/matchmaking/game_catalog_screen.dart';
import 'package:voice_frontend/ui/matchmaking/game_detail_screen.dart';

import 'support/auth_test_overrides.dart';
import 'support/test_voice_token_catalog.dart';

late ThemeData _testTheme;

Map<String, Object?> _game(
  String id,
  String name, {
  String status = 'active',
}) => {
  'id': id,
  'name': name,
  'status': status,
  'config_json': jsonEncode({
    'genre': 'MOBA',
    'regions': ['eu'],
    'modes': [
      {
        'name': '5v5 Ranked',
        'slots': 10,
        'party_size_min': 1,
        'party_size_max': 5,
      },
    ],
  }),
};

http.Response _json(Object body, [int status = 200]) => http.Response(
  jsonEncode(body),
  status,
  headers: {'content-type': 'application/json'},
);

Widget _app(
  http.Client client, {
  ThemeData? theme,
  Widget? home,
  Locale? locale,
}) => ProviderScope(
  overrides: voiceAppTestOverrides(client: client),
  child: MaterialApp(
    debugShowCheckedModeBanner: false,
    theme: theme ?? _testTheme,
    locale: locale,
    localizationsDelegates: AppLocalizations.localizationsDelegates,
    supportedLocales: AppLocalizations.supportedLocales,
    home: home ?? const GameCatalogScreen(),
  ),
);

void main() {
  setUpAll(() async {
    _testTheme = await VoiceTheme.build(
      catalog: testVoiceTokenCatalog,
      mode: VoiceThemeMode.light,
      profileAccent: testVoiceTokenCatalog.profileAccentAt(0),
    );
  });
  testWidgets(
    'Add game stays reachable through catalog and search loading or failure in H/V',
    (tester) async {
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      for (final size in [const Size(1280, 800), const Size(390, 844)]) {
        tester.view.physicalSize = size;
        tester.view.devicePixelRatio = 1;
        await tester.binding.setSurfaceSize(size);
        for (final scenario in [
          'loading',
          'error',
          'search-loading',
          'search-error',
        ]) {
          final waiting = Completer<http.Response>();
          final client = MockClient((request) async {
            if (request.url.path.endsWith('/matches')) {
              return _json({'matches': []});
            }
            final failedRequest =
                !scenario.startsWith('search-') ||
                request.url.path.endsWith('/search');
            if (failedRequest) {
              return scenario.endsWith('loading')
                  ? waiting.future
                  : _json({'message': 'unavailable'}, 503);
            }
            return _json({
              'games': [_game('initial', 'Initial game')],
            });
          });
          await tester.pumpWidget(const SizedBox.shrink());
          await tester.pumpWidget(_app(client));
          if (scenario.startsWith('search-')) {
            await tester.pumpAndSettle();
            await tester.enterText(
              find.byKey(GameCatalogScreen.searchFieldKey),
              'new game',
            );
          }
          await tester.pump();
          await tester.pump(const Duration(milliseconds: 200));
          final addGame = find.byKey(GameCatalogScreen.addGameButtonKey);
          expect(
            addGame.hitTestable(),
            findsOneWidget,
            reason: '$size $scenario',
          );
          await tester.tap(addGame);
          await tester.pump();
          await tester.pump(const Duration(milliseconds: 350));
          expect(
            find.byType(AddGameScreen),
            findsOneWidget,
            reason: '$size $scenario',
          );
          await tester.pumpWidget(const SizedBox.shrink());
          if (!waiting.isCompleted) waiting.complete(_json({'games': []}));
          await tester.pump();
        }
      }
      await tester.binding.setSurfaceSize(null);
    },
  );
  testWidgets(
    'recent failure has an independent retry without hiding the public catalog',
    (tester) async {
      var historyCalls = 0;
      final client = MockClient((request) async {
        if (request.url.path.endsWith('/matches')) {
          if (++historyCalls == 1) {
            return _json({'message': 'private error'}, 503);
          }
          return _json({
            'matches': [
              {'game_id': 'recent'},
            ],
          });
        }
        if (request.url.path.endsWith('/recent')) {
          return _json({'game': _game('recent', 'Recent Game')});
        }
        return _json({
          'games': [_game('popular', 'Popular Game')],
        });
      });
      await tester.pumpWidget(_app(client));
      await tester.pumpAndSettle();
      expect(find.text('Popular Game'), findsOneWidget);
      expect(find.text('Could not load recent games'), findsOneWidget);
      expect(find.textContaining('private error'), findsNothing);
      await tester.tap(find.text('Try again'));
      await tester.pumpAndSettle();
      expect(find.text('Recent Game'), findsOneWidget);
      expect(historyCalls, 2);
    },
  );

  testWidgets(
    'selection route returns the catalog game and exposes named card semantics',
    (tester) async {
      final semantics = tester.ensureSemantics();
      CatalogGame? selected;
      final client = MockClient(
        (request) async => request.url.path.endsWith('/matches')
            ? _json({'matches': []})
            : _json({
                'games': [_game('chosen', 'Choose this game')],
              }),
      );
      await tester.pumpWidget(
        _app(
          client,
          home: Builder(
            builder: (context) => Scaffold(
              body: TextButton(
                onPressed: () async {
                  selected = await Navigator.of(context).push<CatalogGame>(
                    MaterialPageRoute(
                      builder: (_) => const GameCatalogScreen(selectMode: true),
                    ),
                  );
                },
                child: const Text('Choose game'),
              ),
            ),
          ),
        ),
      );
      await tester.tap(find.text('Choose game'));
      await tester.pumpAndSettle();
      expect(find.byKey(GameCatalogScreen.addGameButtonKey), findsNothing);
      expect(find.bySemanticsLabel(RegExp('Choose this game')), findsOneWidget);
      Focus.of(tester.element(find.text('Choose this game'))).requestFocus();
      await tester.pump();
      await tester.sendKeyEvent(LogicalKeyboardKey.enter);
      await tester.pumpAndSettle();
      expect(selected?.id, 'chosen');
      expect(find.text('Choose game'), findsOneWidget);
      semantics.dispose();
    },
  );
  test(
    'recent games hydrate authenticated history in order, skipping duplicates and unavailable games',
    () async {
      final paths = <String>[];
      final client = MockClient((request) async {
        expect(request.headers['authorization'], 'Bearer test-access');
        paths.add(request.url.path);
        if (request.url.path.endsWith('/matches')) {
          expect(request.url.queryParameters['page_size'], '20');
          return _json({
            'matches': [
              for (final id in ['b', 'b', 'gone', 'pending', 'a'])
                {'game_id': id},
            ],
          });
        }
        final id = request.url.path.split('/').last;
        if (id == 'gone') return _json({'message': 'not found'}, 404);
        return _json({
          'game': _game(
            id,
            id,
            status: id == 'pending' ? 'pending_moderation' : 'active',
          ),
        });
      });
      final container = ProviderContainer(
        overrides: voiceAppTestOverrides(client: client),
      );
      addTearDown(container.dispose);
      final games = await container.read(gameCatalogRecentProvider.future);
      expect(games.map((g) => g.id), ['b', 'a']);
      expect(paths.where((p) => p.endsWith('/b')).length, 1);
    },
  );

  testWidgets(
    'search uses the backend and keeps unrelated recent games out of results',
    (tester) async {
      final requests = <Uri>[];
      final client = MockClient((request) async {
        requests.add(request.url);
        if (request.url.path.endsWith('/matches')) {
          return _json({
            'matches': [
              {'game_id': 'recent'},
            ],
          });
        }
        if (request.url.path.endsWith('/recent')) {
          return _json({'game': _game('recent', 'Recent Game')});
        }
        if (request.url.path.endsWith('/search')) {
          return _json({
            'games': [_game('found', 'Found Game')],
          });
        }
        return _json({
          'games': [_game('popular', 'Popular Game')],
        });
      });
      await tester.pumpWidget(_app(client));
      await tester.pumpAndSettle();
      expect(find.text('Recent Game'), findsOneWidget);
      await tester.enterText(
        find.byKey(GameCatalogScreen.searchFieldKey),
        '  Found  ',
      );
      await tester.pumpAndSettle();
      expect(requests.last.queryParameters['query'], 'Found');
      expect(find.text('Found Game'), findsOneWidget);
      expect(find.text('Recent Game'), findsNothing);
      await tester.tap(find.byTooltip('Clear search'));
      await tester.pumpAndSettle();
      expect(find.text('Recent Game'), findsOneWidget);
      expect(
        tester
            .widget<TextField>(find.byKey(GameCatalogScreen.searchFieldKey))
            .focusNode!
            .hasFocus,
        isTrue,
      );
    },
  );

  testWidgets(
    'stale catalog cursor discards accumulated pages and explicitly restarts',
    (tester) async {
      var initial = 0;
      final cursors = <String?>[];
      final client = MockClient((request) async {
        if (request.url.path.endsWith('/matches')) {
          return _json({'matches': []});
        }
        final cursor = request.url.queryParameters['cursor'];
        cursors.add(cursor);
        if (cursor == 'next2') {
          return _json({'message': 'games_cursor_stale'}, 412);
        }
        if (cursor == 'next1') {
          return _json({
            'games': [_game('page2', 'Second page')],
            'next_cursor': 'next2',
          });
        }
        initial++;
        return _json({
          'games': [
            _game('first', initial == 1 ? 'Original first' : 'Fresh first'),
          ],
          'next_cursor': 'next1',
        });
      });
      await tester.pumpWidget(_app(client));
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(GameCatalogScreen.browseAllKey));
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(GameCatalogScreen.loadMoreKey));
      await tester.pumpAndSettle();
      expect(find.text('Second page'), findsOneWidget);
      await tester.tap(find.byKey(GameCatalogScreen.loadMoreKey));
      await tester.pumpAndSettle();
      expect(find.textContaining('The catalog changed'), findsOneWidget);
      expect(cursors, [null, 'next1', 'next2']);
      await tester.tap(find.byKey(GameCatalogScreen.loadMoreKey));
      await tester.pumpAndSettle();
      expect(find.text('Fresh first'), findsOneWidget);
      expect(find.text('Second page'), findsNothing);
      expect(cursors, [null, 'next1', 'next2', null]);
    },
  );

  testWidgets(
    'card opens real detail with space scope and Add game opens the existing panel',
    (tester) async {
      final client = MockClient(
        (request) async => request.url.path.endsWith('/matches')
            ? _json({'matches': []})
            : _json({
                'games': [_game('a', 'Catalog Game')],
              }),
      );
      await tester.pumpWidget(
        _app(client, home: const GameCatalogScreen(spaceId: 'space-a')),
      );
      await tester.pumpAndSettle();
      await tester.tap(find.text('Catalog Game'));
      await tester.pumpAndSettle();
      expect(
        tester.widget<GameDetailScreen>(find.byType(GameDetailScreen)).spaceId,
        'space-a',
      );
      await tester.pageBack();
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(GameCatalogScreen.addGameButtonKey));
      await tester.pumpAndSettle();
      expect(find.byType(AddGameScreen), findsOneWidget);
    },
  );

  testWidgets(
    'actual catalog H/V layouts and states with shipped fonts and tokens',
    (tester) async {
      final dir = Platform.environment['VOICE_GAME_CATALOG_CAPTURE_DIR'];
      ThemeData? theme;
      if (dir?.isNotEmpty ?? false) {
        theme = await tester.runAsync(() async {
          await (FontLoader(VoiceTheme.fontFamily)
                ..addFont(rootBundle.load('assets/fonts/NotoSans-Regular.ttf'))
                ..addFont(rootBundle.load('assets/fonts/NotoSans-Medium.ttf'))
                ..addFont(
                  rootBundle.load('assets/fonts/NotoSans-SemiBold.ttf'),
                ))
              .load();
          final root = Platform.environment['FLUTTER_ROOT']!;
          final bytes = await File(
            '$root/bin/cache/artifacts/material_fonts/MaterialIcons-Regular.otf',
          ).readAsBytes();
          await (FontLoader(
            'MaterialIcons',
          )..addFont(Future.value(ByteData.sublistView(bytes)))).load();
          final tokens = await VoiceTokenCatalog.load();
          return VoiceTheme.build(
            catalog: tokens,
            mode: VoiceThemeMode.light,
            profileAccent: tokens.profileAccentAt(0),
          );
        });
      }
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      for (final size in [const Size(1280, 800), const Size(390, 844)]) {
        tester.view.physicalSize = size;
        tester.view.devicePixelRatio = 1;
        await tester.binding.setSurfaceSize(size);
        for (final state in ['populated', 'empty', 'error', 'loading']) {
          final waiting = Completer<http.Response>();
          final client = MockClient((request) async {
            if (request.url.path.endsWith('/matches')) {
              return _json({
                'matches': state == 'populated'
                    ? [
                        for (final id in ['v', 'd', 'r']) {'game_id': id},
                      ]
                    : [],
              });
            }
            final names = {
              'v': 'Valorant',
              'd': 'Dota 2',
              'r': 'Deep Rock Galactic',
            };
            final id = request.url.path.split('/').last;
            if (names.containsKey(id)) {
              return _json({'game': _game(id, names[id]!)});
            }
            if (state == 'error') {
              return _json({'message': 'private diagnostic'}, 503);
            }
            if (state == 'loading') return waiting.future;
            return _json({
              'games': state == 'empty'
                  ? []
                  : [
                      _game('cs', 'Counter-Strike 2'),
                      _game('apex', 'Apex Legends'),
                      _game('ow', 'Overwatch 2'),
                    ],
            });
          });
          await tester.pumpWidget(const SizedBox.shrink());
          await tester.pumpWidget(
            RepaintBoundary(
              key: const Key('catalog-capture'),
              child: _app(client, theme: theme, locale: const Locale('ru')),
            ),
          );
          if (state == 'loading') {
            await tester.pump();
            await tester.pump(const Duration(milliseconds: 200));
          } else {
            await tester.pumpAndSettle();
          }
          expect(tester.takeException(), isNull, reason: '$size $state');
          expect(
            find.byKey(GameCatalogScreen.addGameButtonKey).hitTestable(),
            findsOneWidget,
            reason:
                'Submission navigation is independent of catalog state: $size $state',
          );
          if (state == 'populated') {
            expect(
              find.byKey(GameCatalogScreen.browseAllKey).hitTestable(),
              findsOneWidget,
            );
            expect(
              find.byKey(GameCatalogScreen.addGameButtonKey).hitTestable(),
              findsOneWidget,
            );
          }
          if (dir?.isNotEmpty ?? false) {
            final boundary = tester.renderObject<RenderRepaintBoundary>(
              find.byKey(const Key('catalog-capture')),
            );
            await tester.runAsync(() async {
              final image = await boundary.toImage(pixelRatio: 1);
              final bytes = await image.toByteData(
                format: ui.ImageByteFormat.png,
              );
              final file = File(
                '$dir/${size.width == 1280 ? 'h' : 'v'}-$state.png',
              );
              await file.parent.create(recursive: true);
              await file.writeAsBytes(bytes!.buffer.asUint8List());
              image.dispose();
            });
          }
          await tester.pumpWidget(const SizedBox.shrink());
          if (!waiting.isCompleted) waiting.complete(_json({'games': []}));
          await tester.pump();
        }
      }
      await tester.binding.setSurfaceSize(null);
    },
  );
}
