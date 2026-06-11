import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/gateway_http.dart';
import 'package:voice_frontend/backend/matchmaking_client.dart';

void main() {
  test('completeMatch posts leave to gateway', () async {
    String? path;
    final client = VoiceMatchmakingClient(
      gateway: GatewayHttpClient(
        httpClient: MockClient((request) async {
          path = request.url.path;
          return http.Response(
            jsonEncode({
              'match': {
                'id': 'match-1',
                'gameId': 'g1',
                'mode': 'Duo',
                'region': 'eu',
                'status': 'completed',
                'profileIds': ['p1', 'p2'],
              },
            }),
            200,
          );
        }),
        config: const GatewayConfig(baseUrl: 'http://api.test'),
      ),
    );

    final result = await client.completeMatch(
      authorization: 'Bearer t',
      matchId: 'match-1',
    );
    expect(path, '/api/v1/matchmaking/matches/match-1/complete');
    expect(result, isA<MatchmakingApiOk<MatchData>>());
    final match = (result as MatchmakingApiOk<MatchData>).data;
    expect(match.status, 'completed');
  });

  test('rateMatch posts stars for rated teammate', () async {
    String? path;
    String? body;
    final client = VoiceMatchmakingClient(
      gateway: GatewayHttpClient(
        httpClient: MockClient((request) async {
          path = request.url.path;
          body = request.body;
          return http.Response('{}', 200);
        }),
        config: const GatewayConfig(baseUrl: 'http://api.test'),
      ),
    );

    final result = await client.rateMatch(
      authorization: 'Bearer t',
      matchId: 'match-1',
      ratedProfileId: 'p2',
      stars: 5,
    );
    expect(path, '/api/v1/matchmaking/matches/match-1/rate');
    expect(body, contains('"ratedProfileId":"p2"'));
    expect(body, contains('"stars":5'));
    expect(result, isA<MatchmakingApiOk<void>>());
  });

  test('getPlayerRating loads aggregate from gateway', () async {
    String? path;
    String? gameIdQuery;
    final client = VoiceMatchmakingClient(
      gateway: GatewayHttpClient(
        httpClient: MockClient((request) async {
          path = request.url.path;
          gameIdQuery = request.url.queryParameters['game_id'];
          return http.Response(
            jsonEncode({
              'playerRating': {
                'profileId': 'p2',
                'gameId': 'g1',
                'ratingValue': 4.5,
                'gamesPlayed': 3,
              },
            }),
            200,
          );
        }),
        config: const GatewayConfig(baseUrl: 'http://api.test'),
      ),
    );

    final result = await client.getPlayerRating(
      authorization: 'Bearer t',
      profileId: 'p2',
      gameId: 'g1',
    );
    expect(path, '/api/v1/matchmaking/players/p2/rating');
    expect(gameIdQuery, 'g1');
    expect(result, isA<MatchmakingApiOk<PlayerRatingData>>());
    final rating = (result as MatchmakingApiOk<PlayerRatingData>).data;
    expect(rating.profileId, 'p2');
    expect(rating.gameId, 'g1');
    expect(rating.ratingValue, 4.5);
    expect(rating.gamesPlayed, 3);
  });

  test('banFromMM posts target profile id', () async {
    String? path;
    String? body;
    final client = VoiceMatchmakingClient(
      gateway: GatewayHttpClient(
        httpClient: MockClient((request) async {
          path = request.url.path;
          body = request.body;
          return http.Response('{}', 200);
        }),
        config: const GatewayConfig(baseUrl: 'http://api.test'),
      ),
    );

    final result = await client.banFromMM(
      authorization: 'Bearer t',
      targetProfileId: 'p2',
      reason: 'toxic',
    );
    expect(path, '/api/v1/matchmaking/bans');
    expect(body, contains('"targetProfileId":"p2"'));
    expect(result, isA<MatchmakingApiOk<void>>());
  });

  test('PlayerRatingData parses gateway JSON', () {
    final rating = PlayerRatingData.fromGatewayJson({
      'profile_id': 'p1',
      'game_id': 'g1',
      'rating_value': 3.5,
      'games_played': 2,
    });
    expect(rating.profileId, 'p1');
    expect(rating.gameId, 'g1');
    expect(rating.ratingValue, 3.5);
    expect(rating.gamesPlayed, 2);
  });
}
