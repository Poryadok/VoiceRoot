import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/gateway_http.dart';
import 'package:voice_frontend/backend/matchmaking_client.dart';

void main() {
  test('getMatch loads match from gateway', () async {
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
                'status': 'pending_accept',
                'profileIds': ['p1', 'p2'],
              },
            }),
            200,
          );
        }),
        config: const GatewayConfig(baseUrl: 'http://api.test'),
      ),
    );

    final result = await client.getMatch(
      authorization: 'Bearer t',
      matchId: 'match-1',
    );
    expect(path, '/api/v1/matchmaking/matches/match-1');
    expect(result, isA<MatchmakingApiOk<MatchData>>());
    final match = (result as MatchmakingApiOk<MatchData>).data;
    expect(match.id, 'match-1');
    expect(match.status, 'pending_accept');
    expect(match.profileIds, ['p1', 'p2']);
  });

  test('respondToMatch posts accept to gateway', () async {
    String? path;
    String? body;
    final client = VoiceMatchmakingClient(
      gateway: GatewayHttpClient(
        httpClient: MockClient((request) async {
          path = request.url.path;
          body = request.body;
          return http.Response(
            jsonEncode({
              'match': {
                'id': 'match-1',
                'gameId': 'g1',
                'mode': 'Duo',
                'region': 'eu',
                'status': 'active',
                'voiceRoomId': 'voice-1',
                'chatId': 'chat-1',
                'profileIds': ['p1', 'p2'],
              },
              'searchSession': {
                'id': 'sess-1',
                'profileId': 'p1',
                'gameId': 'g1',
                'mode': 'Duo',
                'status': 'matched',
                'matchId': 'match-1',
              },
            }),
            200,
          );
        }),
        config: const GatewayConfig(baseUrl: 'http://api.test'),
      ),
    );

    final result = await client.respondToMatch(
      authorization: 'Bearer t',
      matchId: 'match-1',
      accept: true,
    );
    expect(path, '/api/v1/matchmaking/matches/match-1/respond');
    expect(body, contains('"accept":true'));
    expect(result, isA<MatchmakingApiOk<RespondToMatchData>>());
    final data = (result as MatchmakingApiOk<RespondToMatchData>).data;
    expect(data.match.status, 'active');
    expect(data.searchSession.status, 'matched');
  });

  test('respondToMatch decline returns searching session', () async {
    final client = VoiceMatchmakingClient(
      gateway: GatewayHttpClient(
        httpClient: MockClient((request) async {
          return http.Response(
            jsonEncode({
              'match': {
                'id': 'match-1',
                'gameId': 'g1',
                'mode': 'Duo',
                'region': 'eu',
                'status': 'abandoned',
                'profileIds': ['p1', 'p2'],
              },
              'searchSession': {
                'id': 'sess-1',
                'profileId': 'p1',
                'gameId': 'g1',
                'mode': 'Duo',
                'status': 'searching',
              },
            }),
            200,
          );
        }),
        config: const GatewayConfig(baseUrl: 'http://api.test'),
      ),
    );

    final result = await client.respondToMatch(
      authorization: 'Bearer t',
      matchId: 'match-1',
      accept: false,
    );
    final data = (result as MatchmakingApiOk<RespondToMatchData>).data;
    expect(data.searchSession.status, 'searching');
    expect(data.searchSession.matchId, isNull);
  });
}
