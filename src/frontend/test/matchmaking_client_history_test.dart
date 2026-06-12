import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/gateway_http.dart';
import 'package:voice_frontend/backend/matchmaking_client.dart';

void main() {
  test('listMatchHistory loads paginated matches from gateway', () async {
    String? path;
    String? cursor;
    final client = VoiceMatchmakingClient(
      gateway: GatewayHttpClient(
        httpClient: MockClient((request) async {
          path = request.url.path;
          cursor = request.url.queryParameters['cursor'];
          return http.Response(
            jsonEncode({
              'matchList': {
                'matches': [
                  {
                    'id': 'match-1',
                    'gameId': 'g1',
                    'mode': 'Duo',
                    'region': 'eu',
                    'status': 'completed',
                    'profileIds': ['p1', 'p2'],
                    'createdAt': '2026-06-01T12:00:00Z',
                  },
                ],
                'nextCursor': 'cursor-2',
              },
            }),
            200,
          );
        }),
        config: const GatewayConfig(baseUrl: 'http://api.test'),
      ),
    );

    final result = await client.listMatchHistory(
      authorization: 'Bearer t',
      cursor: 'cursor-1',
      pageSize: 10,
    );
    expect(path, '/api/v1/matchmaking/profile/me/matches');
    expect(cursor, 'cursor-1');
    expect(result, isA<MatchmakingApiOk<MatchListData>>());
    final data = (result as MatchmakingApiOk<MatchListData>).data;
    expect(data.matches, hasLength(1));
    expect(data.matches.first.id, 'match-1');
    expect(data.matches.first.status, 'completed');
    expect(data.matches.first.profileIds, ['p1', 'p2']);
    expect(data.nextCursor, 'cursor-2');
  });
}
