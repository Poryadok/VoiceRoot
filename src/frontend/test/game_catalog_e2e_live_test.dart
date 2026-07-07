import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;

/// Live compose e2e: catalog API returns Dota 2 with roles/ranks in config.
///
/// Run: `VOICE_RUN_LIVE_COMPOSE=true flutter test test/game_catalog_e2e_live_test.dart`
void main() {
  const runLive = bool.fromEnvironment('VOICE_RUN_LIVE_COMPOSE') ||
      String.fromEnvironment('VOICE_RUN_LIVE_COMPOSE') == 'true';

  test('live compose lists Dota 2 with roles and ranks in config', () async {
    if (!runLive) {
      return;
    }

    final base = String.fromEnvironment(
      'VOICE_API_BASE_URL',
      defaultValue: 'http://127.0.0.1:8080',
    );
    final client = http.Client();
    addTearDown(client.close);

    final email = 'mm-e2e-${DateTime.now().millisecondsSinceEpoch}@voice.test';
    final register = await client.post(
      Uri.parse('$base/api/v1/auth/register'),
      headers: {'Content-Type': 'application/json'},
      body: jsonEncode({
        'email': email,
        'password': 'VoiceQaTest1!',
        'display_name': 'MM E2E',
      }),
    );
    expect(register.statusCode, isIn([200, 201]));
    final regBody = jsonDecode(register.body) as Map<String, dynamic>;
    final token = regBody['access_token'] as String? ??
        (regBody['session'] as Map<String, dynamic>?)?['access_token'] as String?;
    expect(token, isNotNull);

    final listResp = await client.get(
      Uri.parse('$base/api/v1/matchmaking/games'),
      headers: {'Authorization': 'Bearer $token'},
    );
    expect(listResp.statusCode, 200, reason: listResp.body);

    final listJson = jsonDecode(listResp.body) as Map<String, dynamic>;
    final gameList = listJson['gameList'] as Map<String, dynamic>? ??
        listJson['game_list'] as Map<String, dynamic>?;
    expect(gameList, isNotNull);
    final games = gameList!['games'] as List<dynamic>;
    final dota = games.cast<Map<String, dynamic>>().firstWhere(
      (g) => g['name'] == 'Dota 2',
      orElse: () => throw StateError('Dota 2 not in catalog'),
    );
    final configJson = dota['configJson'] as String? ?? dota['config_json'] as String?;
    expect(configJson, isNotNull);
    expect(configJson, contains('Carry'));
    expect(configJson, contains('Herald'));

    final searchResp = await client.get(
      Uri.parse('$base/api/v1/matchmaking/games/search?query=dota'),
      headers: {'Authorization': 'Bearer $token'},
    );
    expect(searchResp.statusCode, 200);
    final searchJson = jsonDecode(searchResp.body) as Map<String, dynamic>;
    final searchList = searchJson['gameList'] as Map<String, dynamic>? ??
        searchJson['game_list'] as Map<String, dynamic>?;
    final searchGames = searchList!['games'] as List<dynamic>;
    expect(searchGames, isNotEmpty);
  }, skip: runLive ? false : 'set VOICE_RUN_LIVE_COMPOSE=true');
}
