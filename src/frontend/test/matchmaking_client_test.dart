import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/gateway_http.dart';
import 'package:voice_frontend/backend/matchmaking_client.dart';

void main() {
  test('GameConfig parses roles and ranks from config_json', () {
    const raw = '''
    {
      "genre": "MOBA",
      "regions": ["eu"],
      "modes": [{
        "name": "5v5 Ranked",
        "slots": 10,
        "party_size_min": 1,
        "party_size_max": 5,
        "roles_required": true,
        "rank_required": true,
        "roles": [
          {"name": "Carry", "required": true},
          {"name": "Support", "required": false}
        ],
        "ranks": [
          {"name": "Herald", "value": 0},
          {"name": "Ancient", "value": 3850}
        ]
      }]
    }
    ''';
    final cfg = GameConfig.fromJson(jsonDecode(raw) as Map<String, dynamic>);
    expect(cfg.genre, 'MOBA');
    expect(cfg.modes, hasLength(1));
    expect(cfg.modes.first.roles.map((r) => r.name), ['Carry', 'Support']);
    expect(cfg.modes.first.ranks.last.name, 'Ancient');
  });

  test('listGames loads catalog from gateway', () async {
    String? path;
    final client = VoiceMatchmakingClient(
      gateway: GatewayHttpClient(
        httpClient: MockClient((request) async {
          path = request.url.path;
          return http.Response(
            jsonEncode({
              'gameList': {
                'games': [
                  {
                    'id': 'g1',
                    'name': 'Dota 2',
                    'status': 'active',
                    'configJson': jsonEncode({
                      'regions': ['eu'],
                      'modes': [
                        {
                          'name': '5v5 Ranked',
                          'slots': 10,
                          'party_size_min': 1,
                          'party_size_max': 5,
                          'roles': [
                            {'name': 'Carry', 'required': true},
                          ],
                          'ranks': [
                            {'name': 'Herald', 'value': 0},
                          ],
                        },
                      ],
                    }),
                  },
                ],
              },
            }),
            200,
          );
        }),
        config: const GatewayConfig(baseUrl: 'http://api.test'),
      ),
    );

    final result = await client.listGames(authorization: 'Bearer t');
    expect(path, '/api/v1/matchmaking/games');
    expect(result, isA<MatchmakingApiOk<GameListData>>());
    final data = (result as MatchmakingApiOk<GameListData>).data;
    expect(data.games.first.name, 'Dota 2');
    expect(data.games.first.config.modes.first.roles.first.name, 'Carry');
  });

  test('searchGames uses search route', () async {
    String? path;
    final client = VoiceMatchmakingClient(
      gateway: GatewayHttpClient(
        httpClient: MockClient((request) async {
          path = request.url.path;
          return http.Response(
            jsonEncode({
              'gameList': {
                'games': [
                  {
                    'id': 'g1',
                    'name': 'Dota 2',
                    'status': 'active',
                    'configJson': '{"regions":["eu"],"modes":[]}',
                  },
                ],
              },
            }),
            200,
          );
        }),
        config: const GatewayConfig(baseUrl: 'http://api.test'),
      ),
    );

    final result = await client.searchGames(
      authorization: 'Bearer t',
      query: 'dota',
    );
    expect(path, '/api/v1/matchmaking/games/search');
    expect((result as MatchmakingApiOk<GameListData>).data.games, hasLength(1));
  });
}
