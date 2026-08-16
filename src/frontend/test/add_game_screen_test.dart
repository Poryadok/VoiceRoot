import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/gateway_http.dart';
import 'package:voice_frontend/backend/matchmaking_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/state/matchmaking_providers.dart';
import 'package:voice_frontend/ui/matchmaking/add_game_screen.dart';
import 'package:voice_frontend/ui/matchmaking/game_catalog_screen.dart';

import 'support/auth_test_overrides.dart';
import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

void main() {
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
          home: const AddGameScreen(),
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
    await tester.tap(find.byKey(AddGameScreen.submitKey));
    await tester.pumpAndSettle();

    expect(captured, isNotNull);
    expect(captured!.url.path, contains('/game-requests'));
    final body = jsonDecode(captured!.body) as Map<String, dynamic>;
    expect(body['name'], 'Apex Legends');
    expect(body['config_json'], contains('Duos'));
  });
}
