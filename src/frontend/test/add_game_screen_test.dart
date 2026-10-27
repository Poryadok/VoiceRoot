import 'dart:convert';
import 'dart:async';
import 'dart:io';
import 'dart:ui' as ui;

import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/gateway_http.dart';
import 'package:voice_frontend/backend/matchmaking_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/matchmaking_providers.dart';
import 'package:voice_frontend/theme/voice_theme.dart';
import 'package:voice_frontend/theme/voice_token_catalog.dart';
import 'package:voice_frontend/ui/matchmaking/add_game_screen.dart';
import 'package:voice_frontend/ui/matchmaking/game_catalog_screen.dart';

import 'support/auth_test_overrides.dart';
import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

late ThemeData _editorTheme;

void main() {
  setUpAll(() async {
    final catalog = VoiceTokenCatalog.parseJson(
      await rootBundle.loadString('assets/design/voice.tokens.json'),
    );
    _editorTheme = await VoiceTheme.build(
      catalog: catalog,
      mode: VoiceThemeMode.dark,
      profileAccent: catalog.profileAccentAt(0),
    );
    if (const bool.fromEnvironment('VOICE_CAPTURE_ADD_GAME')) {
      final text = FontLoader('Noto Sans')
        ..addFont(rootBundle.load('assets/fonts/NotoSans-Regular.ttf'));
      final icons = FontLoader('MaterialIcons')
        ..addFont(rootBundle.load('fonts/MaterialIcons-Regular.otf'));
      await text.load();
      await icons.load();
    }
  });
  testWidgets('similar search ignores a stale result after name is cleared', (
    tester,
  ) async {
    final pending = Completer<http.Response>();
    await _pumpEditor(tester, MockClient((_) => pending.future));
    await tester.enterText(find.byKey(AddGameScreen.nameFieldKey), 'Finals');
    await tester.pump();
    expect(find.byType(LinearProgressIndicator), findsOneWidget);
    await tester.enterText(find.byKey(AddGameScreen.nameFieldKey), 'F');
    pending.complete(
      http.Response(
        '{"game_list":{"games":[{"id":"g1","name":"Stale Finals","config_json":"{}"}]}}',
        200,
      ),
    );
    await tester.pumpAndSettle();
    expect(find.text('Stale Finals'), findsNothing);
    expect(find.byType(LinearProgressIndicator), findsNothing);
  });

  testWidgets('similar search recovers with retry without changing the draft', (
    tester,
  ) async {
    int searches = 0;
    await _pumpEditor(
      tester,
      MockClient((_) async {
        searches++;
        return searches == 1
            ? http.Response('{}', 503)
            : http.Response('{"game_list":{"games":[]}}', 200);
      }),
    );
    await tester.enterText(find.byKey(AddGameScreen.nameFieldKey), 'Finals');
    await tester.pumpAndSettle();
    expect(find.text('Could not load game catalog'), findsOneWidget);
    await tester.tap(find.text('Try again'));
    await tester.pumpAndSettle();
    expect(find.text('No similar games found'), findsOneWidget);
    expect(searches, 2);
    expect(
      tester
          .widget<TextField>(find.byKey(AddGameScreen.nameFieldKey))
          .controller!
          .text,
      'Finals',
    );
  });

  testWidgets('submission freezes the draft and prevents duplicate requests', (
    tester,
  ) async {
    final pending = Completer<http.Response>();
    int submissions = 0;
    await _pumpEditor(
      tester,
      MockClient((request) async {
        if (request.method == 'POST') {
          submissions++;
          return pending.future;
        }
        return http.Response('{"game_list":{"games":[]}}', 200);
      }),
    );
    await tester.enterText(find.byKey(AddGameScreen.nameFieldKey), 'Finals');
    await tester.enterText(find.byKey(AddGameScreen.modeFieldKey), 'Ranked');
    await tester.scrollUntilVisible(
      find.byKey(AddGameScreen.submitKey),
      250,
      scrollable: find.byType(Scrollable).first,
    );
    await tester.tap(find.byKey(AddGameScreen.submitKey));
    await tester.pump();
    expect(
      tester
          .widget<FilledButton>(find.byKey(AddGameScreen.submitKey))
          .onPressed,
      isNull,
    );
    expect(tester.widget<PopScope>(find.byType(PopScope)).canPop, isFalse);
    expect(submissions, 1);
    pending.complete(http.Response('{}', 503));
    await tester.pumpAndSettle();
    expect(
      tester
          .widget<FilledButton>(find.byKey(AddGameScreen.submitKey))
          .onPressed,
      isNotNull,
    );
  });

  for (final size in [const Size(1120, 800), const Size(390, 844)]) {
    testWidgets('AddGame accessible layout at ${size.width}', (tester) async {
      tester.view.devicePixelRatio = 1;
      tester.view.physicalSize = size;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      final semantics = tester.ensureSemantics();
      await _pumpEditor(
        tester,
        MockClient(
          (_) async => http.Response('{"game_list":{"games":[]}}', 200),
        ),
      );
      await tester.enterText(
        find.byKey(AddGameScreen.nameFieldKey),
        'The Finals',
      );
      await tester.enterText(find.byKey(AddGameScreen.modeFieldKey), 'Ranked');
      await tester.enterText(find.byKey(AddGameScreen.slotsFieldKey), '3');
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(AddGameScreen.nameFieldKey));
      await tester.sendKeyEvent(LogicalKeyboardKey.tab);
      expect(tester.takeException(), isNull);
      expect(find.bySemanticsLabel(RegExp('Game name')), findsOneWidget);
      await expectLater(tester, meetsGuideline(labeledTapTargetGuideline));
      semantics.dispose();
      if (const bool.fromEnvironment('VOICE_CAPTURE_ADD_GAME')) {
        await _capture(tester, 'addgame-${size.width.toInt()}');
      }
      await tester.scrollUntilVisible(
        find.byKey(AddGameScreen.submitKey),
        250,
        scrollable: find.byType(Scrollable).first,
      );
      if (size.width < 560) {
        FocusManager.instance.primaryFocus?.unfocus();
        await tester.pumpAndSettle();
        final position = tester
            .state<ScrollableState>(find.byType(Scrollable).first)
            .position;
        position.jumpTo(position.maxScrollExtent);
      }
      await tester.pumpAndSettle();
      expect(find.byKey(AddGameScreen.submitKey).hitTestable(), findsOneWidget);
      expect(
        tester.getRect(find.byKey(AddGameScreen.submitKey)).bottom,
        lessThanOrEqualTo(size.height),
      );
      expect(tester.takeException(), isNull);
      if (const bool.fromEnvironment('VOICE_CAPTURE_ADD_GAME')) {
        await _capture(tester, 'addgame-${size.width.toInt()}-footer');
      }
    });
  }

  testWidgets('incomplete request cannot be submitted', (tester) async {
    await _pumpEditor(
      tester,
      MockClient((_) async => http.Response('{}', 200)),
    );
    expect(
      tester
          .widget<FilledButton>(find.byKey(AddGameScreen.submitKey))
          .onPressed,
      isNull,
    );
  });

  testWidgets('invalid slots and duplicate mode names show recovery guidance', (
    tester,
  ) async {
    await _pumpEditor(
      tester,
      MockClient((_) async => http.Response('{"game_list":{"games":[]}}', 200)),
    );
    await tester.enterText(find.byKey(AddGameScreen.nameFieldKey), 'Finals');
    await tester.enterText(find.byKey(AddGameScreen.modeFieldKey), 'Ranked');
    await tester.enterText(find.byKey(AddGameScreen.slotsFieldKey), '1.5');
    await tester.pumpAndSettle();
    expect(find.text('Enter a positive whole number.'), findsOneWidget);
    await tester.enterText(find.byKey(AddGameScreen.slotsFieldKey), '3');
    await tester.ensureVisible(find.byKey(const Key('add_game_add_mode')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('add_game_add_mode')));
    await tester.pump();
    await tester.enterText(find.byKey(const Key('add_game_mode_1')), 'Ranked');
    await tester.pumpAndSettle();
    expect(find.text('Use a different name for each mode.'), findsWidgets);
    await tester.scrollUntilVisible(
      find.byKey(AddGameScreen.submitKey),
      250,
      scrollable: find.byType(Scrollable).first,
    );
    expect(
      tester
          .widget<FilledButton>(find.byKey(AddGameScreen.submitKey))
          .onPressed,
      isNull,
    );
    await tester.scrollUntilVisible(
      find.byKey(const Key('add_game_remove_mode_1')),
      -250,
      scrollable: find.byType(Scrollable).first,
    );
    await tester.tap(find.byKey(const Key('add_game_remove_mode_1')));
    await tester.pumpAndSettle();
    await tester.scrollUntilVisible(
      find.byKey(AddGameScreen.submitKey),
      250,
      scrollable: find.byType(Scrollable).first,
    );
    expect(
      tester
          .widget<FilledButton>(find.byKey(AddGameScreen.submitKey))
          .onPressed,
      isNotNull,
    );
  });

  testWidgets('multiple modes are preserved in the pending request', (
    tester,
  ) async {
    Map<String, dynamic>? sent;
    final client = MockClient((request) async {
      if (request.method == 'POST') {
        sent = jsonDecode(request.body) as Map<String, dynamic>;
        return http.Response('{"error":"temporarily_unavailable"}', 503);
      }
      return http.Response('{"game_list":{"games":[]}}', 200);
    });
    await _pumpEditor(tester, client);
    await tester.enterText(
      find.byKey(AddGameScreen.nameFieldKey),
      'The Finals',
    );
    await tester.enterText(find.byKey(AddGameScreen.modeFieldKey), 'Ranked');
    await tester.enterText(find.byKey(AddGameScreen.slotsFieldKey), '3');
    await tester.tap(find.byKey(const Key('add_game_add_mode')));
    await tester.pump();
    await tester.enterText(find.byKey(const Key('add_game_mode_1')), 'Casual');
    await tester.enterText(find.byKey(const Key('add_game_slots_1')), '6');
    await tester.scrollUntilVisible(
      find.byKey(AddGameScreen.submitKey),
      250,
      scrollable: find.byType(Scrollable).first,
    );
    await tester.pump();
    await tester.tap(find.byKey(AddGameScreen.submitKey));
    await tester.pumpAndSettle();
    final config =
        jsonDecode(sent!['config_json'] as String) as Map<String, dynamic>;
    expect((config['modes'] as List).map((m) => m['name']), [
      'Ranked',
      'Casual',
    ]);
    expect((config['modes'] as List).map((m) => m['slots']), [3, 6]);
    expect(find.byKey(AddGameScreen.screenKey), findsOneWidget);
    await tester.scrollUntilVisible(
      find.byKey(AddGameScreen.nameFieldKey),
      -250,
      scrollable: find.byType(Scrollable).first,
    );
    expect(
      tester
          .widget<TextField>(find.byKey(AddGameScreen.nameFieldKey))
          .controller!
          .text,
      'The Finals',
    );
  });

  testWidgets('catalog shows add game action', (tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceThemeTestOverrides(),
          gameCatalogSearchProvider.overrideWith(
            (ref) async => const GameListData(games: []),
          ),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const GameCatalogScreen(),
        ),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.byKey(GameCatalogScreen.addGameButtonKey), findsOneWidget);
  });

  testWidgets('add game wizard submits pending request', (tester) async {
    http.Request? captured;
    final mock = MockClient((request) async {
      captured = request;
      if (request.url.path.endsWith('/game-requests') &&
          request.method == 'POST') {
        return http.Response(
          jsonEncode({
            'game': {
              'id': 'pending-1',
              'name': 'Apex Legends',
              'status': 'pending_moderation',
              'config_json': '{"regions":["eu"],"modes":[]}',
            },
          }),
          200,
          headers: {'content-type': 'application/json'},
        );
      }
      if (request.url.path.contains('/games/search')) {
        return http.Response(
          jsonEncode({
            'game_list': {'games': <dynamic>[]},
          }),
          200,
          headers: {'content-type': 'application/json'},
        );
      }
      return http.Response('not found', 404);
    });

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceAppTestOverrides(client: mock),
          voiceMatchmakingClientProvider.overrideWith((ref) {
            return VoiceMatchmakingClient(
              gateway: GatewayHttpClient(
                config: const GatewayConfig(baseUrl: 'http://localhost:9999'),
                httpClient: mock,
              ),
            );
          }),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const RepaintBoundary(
            key: Key('add_game_capture'),
            child: AddGameScreen(),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.enterText(
      find.byKey(AddGameScreen.nameFieldKey),
      'Apex Legends',
    );
    await tester.enterText(find.byKey(AddGameScreen.modeFieldKey), 'Duos');
    await tester.enterText(find.byKey(AddGameScreen.slotsFieldKey), '2');
    await tester.scrollUntilVisible(
      find.byKey(AddGameScreen.submitKey),
      250,
      scrollable: find.byType(Scrollable).first,
    );
    await tester.pump();
    await tester.tap(find.byKey(AddGameScreen.submitKey));
    await tester.pumpAndSettle();

    expect(captured, isNotNull);
    expect(captured!.url.path, contains('/game-requests'));
    final body = jsonDecode(captured!.body) as Map<String, dynamic>;
    expect(body['name'], 'Apex Legends');
    expect(body['config_json'], contains('Duos'));
  });
}

Future<void> _capture(WidgetTester tester, String name) =>
    tester.runAsync(() async {
      final boundary = tester.renderObject<RenderRepaintBoundary>(
        find.byKey(const Key('add_game_capture')),
      );
      final image = await boundary.toImage();
      final bytes = await image.toByteData(format: ui.ImageByteFormat.png);
      final output = File('../../tmp/anton59-addgame/$name.png');
      await output.parent.create(recursive: true);
      await output.writeAsBytes(bytes!.buffer.asUint8List());
      image.dispose();
    });

Future<void> _pumpEditor(WidgetTester tester, http.Client client) async {
  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        ...voiceAppTestOverrides(client: client),
        voiceMatchmakingClientProvider.overrideWith(
          (ref) => VoiceMatchmakingClient(
            gateway: GatewayHttpClient(
              config: const GatewayConfig(baseUrl: 'http://localhost:9999'),
              httpClient: client,
            ),
          ),
        ),
      ],
      child: MaterialApp(
        theme: _editorTheme,
        locale: const Locale('en'),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: const RepaintBoundary(
          key: Key('add_game_capture'),
          child: AddGameScreen(),
        ),
      ),
    ),
  );
  await tester.pumpAndSettle();
}
