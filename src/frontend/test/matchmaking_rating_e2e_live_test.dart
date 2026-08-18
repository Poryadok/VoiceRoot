import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;

/// Live compose: rate teammates after match complete + skip-rating path (MM-05).
///
/// Run: `VOICE_RUN_LIVE_COMPOSE=true flutter test test/matchmaking_rating_e2e_live_test.dart`
void main() {
  const runLive = bool.fromEnvironment('VOICE_RUN_LIVE_COMPOSE') ||
      String.fromEnvironment('VOICE_RUN_LIVE_COMPOSE') == 'true' ||
      bool.fromEnvironment('VOICE_RUN_LIVE_INTEGRATION') ||
      String.fromEnvironment('VOICE_RUN_LIVE_INTEGRATION') == 'true';

  test('rate teammates persists player rating', () async {
    if (!runLive) {
      return;
    }

    final base = String.fromEnvironment(
      'VOICE_API_BASE_URL',
      defaultValue: 'http://127.0.0.1:18080',
    );
    final client = http.Client();
    addTearDown(client.close);

    final completed = await _completeDuoMatch(client, base, 'mm-rate');
    await _rate(
      client,
      base,
      completed.tokenA,
      completed.matchId,
      completed.profileB,
      stars: 5,
    );
    final rating = await _playerRating(
      client,
      base,
      completed.tokenA,
      completed.profileB,
      completed.gameId,
    );
    final value = (rating['ratingValue'] as num?)?.toDouble() ??
        (rating['rating_value'] as num?)?.toDouble();
    expect(value, 5.0);
  }, timeout: const Timeout(Duration(minutes: 2)));

  test('skip rating leaves player rating unset for teammate', () async {
    if (!runLive) {
      return;
    }

    final base = String.fromEnvironment(
      'VOICE_API_BASE_URL',
      defaultValue: 'http://127.0.0.1:18080',
    );
    final client = http.Client();
    addTearDown(client.close);

    final completed = await _completeDuoMatch(client, base, 'mm-skip');
    // Spec: rating can be skipped — do not call /rate; history/complete still OK.
    final rating = await _playerRating(
      client,
      base,
      completed.tokenA,
      completed.profileB,
      completed.gameId,
    );
    final value = (rating['ratingValue'] as num?)?.toDouble() ??
        (rating['rating_value'] as num?)?.toDouble() ??
        0;
    final games = (rating['gamesPlayed'] as num?)?.toInt() ??
        (rating['games_played'] as num?)?.toInt() ??
        0;
    expect(value, 0);
    expect(games, 0);
  }, timeout: const Timeout(Duration(minutes: 2)));
}

class _CompletedMatch {
  const _CompletedMatch({
    required this.tokenA,
    required this.tokenB,
    required this.profileB,
    required this.matchId,
    required this.gameId,
  });

  final String tokenA;
  final String tokenB;
  final String profileB;
  final String matchId;
  final String gameId;
}

Future<_CompletedMatch> _completeDuoMatch(
  http.Client client,
  String base,
  String prefix,
) async {
  final n = DateTime.now().millisecondsSinceEpoch;
  final tokenA = await _register(client, base, '$prefix-a-$n@voice.test');
  final tokenB = await _register(client, base, '$prefix-b-$n@voice.test');
  final gameId = await _findGameId(client, base, tokenA, 'MM Duo Live');
  const criteria = '{"region":"eu"}';

  final sessionA = await _startSearch(client, base, tokenA, gameId, criteria);
  await _startSearch(client, base, tokenB, gameId, criteria);

  String? matchId;
  final deadline = DateTime.now().add(const Duration(seconds: 30));
  while (DateTime.now().isBefore(deadline)) {
    final statusA = await _searchStatus(client, base, tokenA, sessionA);
    matchId = statusA['matchId'] as String? ?? statusA['match_id'] as String?;
    if (matchId != null && matchId.isNotEmpty) break;
    await Future<void>.delayed(const Duration(seconds: 2));
  }
  expect(matchId, isNotNull);

  await _respond(client, base, tokenA, matchId!, accept: true);
  final active = await _respond(client, base, tokenB, matchId, accept: true);
  final match = active['match'] as Map<String, dynamic>? ??
      active['Match'] as Map<String, dynamic>;
  expect(match['status'], 'active');

  await _complete(client, base, tokenA, matchId);
  await _complete(client, base, tokenB, matchId);

  final profileIds = (match['profileIds'] as List<dynamic>? ??
          match['profile_ids'] as List<dynamic>? ??
          const [])
      .cast<String>();
  final selfA = await _profileId(client, base, tokenA);
  final profileB = profileIds.firstWhere(
    (id) => id != selfA,
    orElse: () => profileIds.last,
  );

  return _CompletedMatch(
    tokenA: tokenA,
    tokenB: tokenB,
    profileB: profileB,
    matchId: matchId,
    gameId: gameId,
  );
}

Future<String> _profileId(http.Client client, String base, String token) async {
  final resp = await client.get(
    Uri.parse('$base/api/v1/users/me'),
    headers: {'Authorization': 'Bearer $token'},
  );
  expect(resp.statusCode, 200);
  final body = jsonDecode(resp.body) as Map<String, dynamic>;
  final profile = body['profile'] as Map<String, dynamic>? ??
      body['active_profile'] as Map<String, dynamic>?;
  return profile?['id'] as String? ?? '';
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

Future<void> _rate(
  http.Client client,
  String base,
  String token,
  String matchId,
  String ratedProfileId, {
  required int stars,
}) async {
  final resp = await client.post(
    Uri.parse('$base/api/v1/matchmaking/matches/$matchId/rate'),
    headers: {
      'Authorization': 'Bearer $token',
      'Content-Type': 'application/json',
    },
    body: jsonEncode({
      'ratedProfileId': ratedProfileId,
      'stars': stars,
    }),
  );
  expect(resp.statusCode, 200);
}

Future<Map<String, dynamic>> _playerRating(
  http.Client client,
  String base,
  String token,
  String profileId,
  String gameId,
) async {
  final resp = await client.get(
    Uri.parse(
      '$base/api/v1/matchmaking/players/$profileId/rating?game_id=$gameId',
    ),
    headers: {'Authorization': 'Bearer $token'},
  );
  expect(resp.statusCode, 200);
  final body = jsonDecode(resp.body) as Map<String, dynamic>;
  return body['playerRating'] as Map<String, dynamic>? ??
      body['player_rating'] as Map<String, dynamic>? ??
      body;
}

Future<String> _register(http.Client client, String base, String email) async {
  final register = await client.post(
    Uri.parse('$base/api/v1/auth/register'),
    headers: {'Content-Type': 'application/json'},
    body: jsonEncode({
      'email': email,
      'password': 'VoiceQaTest1!',
      'display_name': 'MM Rating E2E',
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
  String criteria,
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
      'criteriaJson': criteria,
    }),
  );
  expect(resp.statusCode, 200);
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
      body['search_session'] as Map<String, dynamic>? ??
      body;
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
  expect(resp.statusCode, 200);
  return jsonDecode(resp.body) as Map<String, dynamic>;
}
