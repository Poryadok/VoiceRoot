import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;

/// Live compose e2e: player profile CRUD via Gateway.
///
/// Run: `VOICE_RUN_LIVE_COMPOSE=true flutter test test/player_profile_e2e_live_test.dart`
void main() {
  const runLive = bool.fromEnvironment('VOICE_RUN_LIVE_COMPOSE') ||
      String.fromEnvironment('VOICE_RUN_LIVE_COMPOSE') == 'true';

  test('live compose player profile upsert and list', () async {
    if (!runLive) {
      return;
    }

    final base = String.fromEnvironment(
      'VOICE_API_BASE_URL',
      defaultValue: 'http://127.0.0.1:8080',
    );
    final client = http.Client();
    addTearDown(client.close);

    final email = 'mm-profile-${DateTime.now().millisecondsSinceEpoch}@voice.test';
    final register = await client.post(
      Uri.parse('$base/api/v1/auth/register'),
      headers: {'Content-Type': 'application/json'},
      body: jsonEncode({
        'email': email,
        'password': 'VoiceQaTest1!',
        'display_name': 'MM Profile E2E',
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
    final gameId = dota['id'] as String;

    final putResp = await client.put(
      Uri.parse('$base/api/v1/matchmaking/profile/games/$gameId'),
      headers: {
        'Authorization': 'Bearer $token',
        'Content-Type': 'application/json',
      },
      body: jsonEncode({
        'region': 'eu',
        'role': 'Carry',
        'rank': 'Herald',
      }),
    );
    expect(putResp.statusCode, 200, reason: putResp.body);

    final meResp = await client.get(
      Uri.parse('$base/api/v1/matchmaking/profile/me'),
      headers: {'Authorization': 'Bearer $token'},
    );
    expect(meResp.statusCode, 200, reason: meResp.body);
    expect(meResp.body, contains(gameId));
    expect(meResp.body, contains('Herald'));

    final delResp = await client.delete(
      Uri.parse('$base/api/v1/matchmaking/profile/games/$gameId'),
      headers: {'Authorization': 'Bearer $token'},
    );
    expect(delResp.statusCode, 200);
  });
}
