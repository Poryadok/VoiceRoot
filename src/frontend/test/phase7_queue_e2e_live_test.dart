import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;

/// Live compose e2e: start MM search → status → cancel.
///
/// Run: `VOICE_RUN_LIVE_COMPOSE=true flutter test test/phase7_queue_e2e_live_test.dart`
void main() {
  const runLive = bool.fromEnvironment('VOICE_RUN_LIVE_COMPOSE') ||
      String.fromEnvironment('VOICE_RUN_LIVE_COMPOSE') == 'true';

  test('live compose queue search start status cancel', () async {
    if (!runLive) {
      return;
    }

    final base = String.fromEnvironment(
      'VOICE_API_BASE_URL',
      defaultValue: 'http://127.0.0.1:8080',
    );
    final client = http.Client();
    addTearDown(client.close);

    final email = 'mm-queue-${DateTime.now().millisecondsSinceEpoch}@voice.test';
    final register = await client.post(
      Uri.parse('$base/api/v1/auth/register'),
      headers: {'Content-Type': 'application/json'},
      body: jsonEncode({
        'email': email,
        'password': 'VoiceQaTest1!',
        'display_name': 'MM Queue E2E',
      }),
    );
    expect(register.statusCode, isIn([200, 201]));
    final regBody = jsonDecode(register.body) as Map<String, dynamic>;
    final token = regBody['access_token'] as String? ??
        (regBody['session'] as Map<String, dynamic>?)?['access_token'] as String?;
    expect(token, isNotNull);

    final gamesResp = await client.get(
      Uri.parse('$base/api/v1/matchmaking/games'),
      headers: {'Authorization': 'Bearer $token'},
    );
    expect(gamesResp.statusCode, 200);
    final gamesBody = jsonDecode(gamesResp.body) as Map<String, dynamic>;
    final gameList = gamesBody['gameList'] as Map<String, dynamic>? ??
        gamesBody['game_list'] as Map<String, dynamic>;
    final games = gameList['games'] as List<dynamic>;
    final dota = games.cast<Map<String, dynamic>>().firstWhere(
      (g) => g['name'] == 'Dota 2',
    );
    final gameId = dota['id'] as String;

    final criteria = jsonEncode({
      'region': 'eu',
      'self': {'role': 'Carry', 'rank': 'Herald'},
      'sought': {'rank_min': 'Herald', 'rank_max': 'Guardian'},
    });
    final startResp = await client.post(
      Uri.parse('$base/api/v1/matchmaking/search'),
      headers: {
        'Authorization': 'Bearer $token',
        'Content-Type': 'application/json',
      },
      body: jsonEncode({
        'gameId': gameId,
        'mode': '5v5 Ranked',
        'criteriaJson': criteria,
      }),
    );
    expect(startResp.statusCode, 200, reason: startResp.body);
    final startBody = jsonDecode(startResp.body) as Map<String, dynamic>;
    final session = startBody['searchSession'] as Map<String, dynamic>? ??
        startBody['search_session'] as Map<String, dynamic>;
    final sessionId = session['id'] as String;
    expect(session['status'], 'searching');

    final statusResp = await client.get(
      Uri.parse('$base/api/v1/matchmaking/search/$sessionId'),
      headers: {'Authorization': 'Bearer $token'},
    );
    expect(statusResp.statusCode, 200);

    final cancelResp = await client.delete(
      Uri.parse('$base/api/v1/matchmaking/search/$sessionId'),
      headers: {'Authorization': 'Bearer $token'},
    );
    expect(cancelResp.statusCode, 200);
  }, skip: runLive ? false : 'set VOICE_RUN_LIVE_COMPOSE=true');
}
