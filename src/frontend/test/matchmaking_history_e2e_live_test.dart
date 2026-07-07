import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;

/// Live compose e2e: completed match appears in match history for participants.
///
/// Run: `VOICE_RUN_LIVE_COMPOSE=true flutter test test/matchmaking_history_e2e_live_test.dart`
void main() {
  const runLive = bool.fromEnvironment('VOICE_RUN_LIVE_COMPOSE') ||
      String.fromEnvironment('VOICE_RUN_LIVE_COMPOSE') == 'true';

  test('live compose match history lists completed squad', () async {
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
    final tokenA = await _register(client, base, 'mm-history-a-$n@voice.test');
    final tokenB = await _register(client, base, 'mm-history-b-$n@voice.test');

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

    await _respond(client, base, tokenA, matchId!, accept: true);
    await _respond(client, base, tokenB, matchId, accept: true);

    await _complete(client, base, tokenA, matchId);
    await _complete(client, base, tokenB, matchId);

    final historyA = await _listHistory(client, base, tokenA);
    final historyB = await _listHistory(client, base, tokenB);

    expect(_historyContainsMatch(historyA, matchId), isTrue);
    expect(_historyContainsMatch(historyB, matchId), isTrue);

    final entry = _findHistoryMatch(historyA, matchId);
    expect(entry['status'], 'completed');
    final profiles = (entry['profileIds'] as List<dynamic>? ??
            entry['profile_ids'] as List<dynamic>? ??
            const [])
        .cast<String>();
    expect(profiles, hasLength(2));
  });
}

bool _historyContainsMatch(Map<String, dynamic> history, String matchId) {
  return _findHistoryMatchOrNull(history, matchId) != null;
}

Map<String, dynamic> _findHistoryMatch(
  Map<String, dynamic> history,
  String matchId,
) {
  final found = _findHistoryMatchOrNull(history, matchId);
  expect(found, isNotNull);
  return found!;
}

Map<String, dynamic>? _findHistoryMatchOrNull(
  Map<String, dynamic> history,
  String matchId,
) {
  final list = history['matchList'] as Map<String, dynamic>? ??
      history['match_list'] as Map<String, dynamic>?;
  final matches = list?['matches'] as List<dynamic>? ?? const [];
  for (final item in matches) {
    if (item is! Map<String, dynamic>) continue;
    if (item['id'] == matchId) return item;
  }
  return null;
}

Future<Map<String, dynamic>> _listHistory(
  http.Client client,
  String base,
  String token,
) async {
  final resp = await client.get(
    Uri.parse('$base/api/v1/matchmaking/profile/me/matches'),
    headers: {'Authorization': 'Bearer $token'},
  );
  expect(resp.statusCode, 200, reason: resp.body);
  return jsonDecode(resp.body) as Map<String, dynamic>;
}

Future<void> _complete(
  http.Client client,
  String base,
  String token,
  String matchId,
) async {
  final resp = await client.post(
    Uri.parse('$base/api/v1/matchmaking/matches/$matchId/complete'),
    headers: {'Authorization': 'Bearer $token'},
  );
  expect(resp.statusCode, 200);
}

Future<String> _register(http.Client client, String base, String email) async {
  final register = await client.post(
    Uri.parse('$base/api/v1/auth/register'),
    headers: {'Content-Type': 'application/json'},
    body: jsonEncode({
      'email': email,
      'password': 'VoiceQaTest1!',
      'display_name': 'MM History E2E',
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
