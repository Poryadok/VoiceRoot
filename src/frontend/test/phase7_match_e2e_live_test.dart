import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;

/// Live compose e2e: two users match → accept → active squad with chat + voice ids.
///
/// Run: `VOICE_RUN_LIVE_COMPOSE=true flutter test test/phase7_match_e2e_live_test.dart`
void main() {
  const runLive = bool.fromEnvironment('VOICE_RUN_LIVE_COMPOSE') ||
      String.fromEnvironment('VOICE_RUN_LIVE_COMPOSE') == 'true';

  test('live compose match found accept flow', () async {
    if (!runLive) {
      return;
    }

    final base = String.fromEnvironment(
      'VOICE_API_BASE_URL',
      defaultValue: 'http://127.0.0.1:18080',
    );
    final client = http.Client();
    addTearDown(client.close);

    final n = DateTime.now().millisecondsSinceEpoch;
    final tokenA = await _register(client, base, 'mm-match-a-$n@voice.test');
    final tokenB = await _register(client, base, 'mm-match-b-$n@voice.test');

    final gameId = await _findGameId(client, base, tokenA, 'MM Duo Live');
    const criteria = '{"region":"eu"}';

    final sessionA = await _startSearch(client, base, tokenA, gameId, criteria);
    final sessionB = await _startSearch(client, base, tokenB, gameId, criteria);

    String? matchId;
    final deadline = DateTime.now().add(const Duration(seconds: 30));
    while (DateTime.now().isBefore(deadline)) {
      final statusA = await _searchStatus(client, base, tokenA, sessionA);
      matchId = statusA['matchId'] as String? ?? statusA['match_id'] as String?;
      if (matchId != null && matchId.isNotEmpty) break;
      final statusB = await _searchStatus(client, base, tokenB, sessionB);
      matchId = statusB['matchId'] as String? ?? statusB['match_id'] as String?;
      if (matchId != null && matchId.isNotEmpty) break;
      await Future<void>.delayed(const Duration(seconds: 2));
    }
    expect(matchId, isNotNull);
    expect(matchId, isNotEmpty);

    final matchA = await _getMatch(client, base, tokenA, matchId!);
    expect(matchA['status'], 'pending_accept');

    await _respond(client, base, tokenA, matchId, accept: true);
    final active = await _respond(client, base, tokenB, matchId, accept: true);
    final match = active['match'] as Map<String, dynamic>? ??
        active['Match'] as Map<String, dynamic>;
    expect(match['status'], 'active');
    final chatId = match['chatId'] as String? ?? match['chat_id'] as String?;
    final voiceRoomId =
        match['voiceRoomId'] as String? ?? match['voice_room_id'] as String?;
    expect(chatId, isNotNull);
    expect(voiceRoomId, isNotNull);
  });
}

Future<String> _register(http.Client client, String base, String email) async {
  final register = await client.post(
    Uri.parse('$base/api/v1/auth/register'),
    headers: {'Content-Type': 'application/json'},
    body: jsonEncode({
      'email': email,
      'password': 'VoiceQaTest1!',
      'display_name': 'MM Match E2E',
    }),
  );
  expect(register.statusCode, isIn([200, 201]));
  final regBody = jsonDecode(register.body) as Map<String, dynamic>;
  final token = regBody['access_token'] as String? ??
      (regBody['session'] as Map<String, dynamic>?)?['access_token'] as String?;
  expect(token, isNotNull);
  return token!;
}

Future<String> _findGameId(
  http.Client client,
  String base,
  String token,
  String name,
) async {
  final resp = await client.get(
    Uri.parse('$base/api/v1/matchmaking/games'),
    headers: {'Authorization': 'Bearer $token'},
  );
  expect(resp.statusCode, 200);
  final body = jsonDecode(resp.body) as Map<String, dynamic>;
  final gameList = body['gameList'] as Map<String, dynamic>? ??
      body['game_list'] as Map<String, dynamic>;
  final games = gameList['games'] as List<dynamic>;
  final game = games.cast<Map<String, dynamic>>().firstWhere(
        (g) => g['name'] == name,
      );
  return game['id'] as String;
}

Future<String> _startSearch(
  http.Client client,
  String base,
  String token,
  String gameId,
  String criteriaJson,
) async {
  final resp = await client.post(
    Uri.parse('$base/api/v1/matchmaking/search'),
    headers: {
      'Authorization': 'Bearer $token',
      'Content-Type': 'application/json',
    },
    body: jsonEncode({
      'gameId': gameId,
      'mode': 'Duo',
      'criteriaJson': criteriaJson,
    }),
  );
  expect(resp.statusCode, 200, reason: resp.body);
  final body = jsonDecode(resp.body) as Map<String, dynamic>;
  final session = body['searchSession'] as Map<String, dynamic>? ??
      body['search_session'] as Map<String, dynamic>;
  return session['id'] as String;
}

Future<Map<String, dynamic>> _searchStatus(
  http.Client client,
  String base,
  String token,
  String sessionId,
) async {
  final resp = await client.get(
    Uri.parse('$base/api/v1/matchmaking/search/$sessionId'),
    headers: {'Authorization': 'Bearer $token'},
  );
  expect(resp.statusCode, 200);
  final body = jsonDecode(resp.body) as Map<String, dynamic>;
  return body['searchSession'] as Map<String, dynamic>? ??
      body['search_session'] as Map<String, dynamic>;
}

Future<Map<String, dynamic>> _getMatch(
  http.Client client,
  String base,
  String token,
  String matchId,
) async {
  final resp = await client.get(
    Uri.parse('$base/api/v1/matchmaking/matches/$matchId'),
    headers: {'Authorization': 'Bearer $token'},
  );
  expect(resp.statusCode, 200, reason: resp.body);
  final body = jsonDecode(resp.body) as Map<String, dynamic>;
  return body['match'] as Map<String, dynamic>? ??
      body['Match'] as Map<String, dynamic>;
}

Future<Map<String, dynamic>> _respond(
  http.Client client,
  String base,
  String token,
  String matchId, {
  required bool accept,
}) async {
  final resp = await client.post(
    Uri.parse('$base/api/v1/matchmaking/matches/$matchId/respond'),
    headers: {
      'Authorization': 'Bearer $token',
      'Content-Type': 'application/json',
    },
    body: jsonEncode({'accept': accept}),
  );
  expect(resp.statusCode, 200, reason: resp.body);
  return jsonDecode(resp.body) as Map<String, dynamic>;
}
