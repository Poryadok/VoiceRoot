import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/matchmaking_client.dart';

void main() {
  test('PlayerGameEntry parses gateway JSON', () {
    final entry = PlayerGameEntry.fromGatewayJson({
      'gameId': 'g1',
      'region': 'eu',
      'role': 'Carry',
      'rank': 'Herald',
      'updatedAt': '2026-01-01T00:00:00Z',
    });
    expect(entry.gameId, 'g1');
    expect(entry.region, 'eu');
    expect(entry.role, 'Carry');
    expect(entry.rank, 'Herald');
    expect(entry.updatedAt, isNotNull);
  });

  test('PlayerProfileData parses entries list', () {
    final json = {
      'entries': [
        {'game_id': 'g2', 'region': 'cis', 'rank': 'Ancient'},
      ],
    };
    final entries = (json['entries'] as List)
        .cast<Map<String, dynamic>>()
        .map(PlayerGameEntry.fromGatewayJson)
        .toList();
    expect(entries, hasLength(1));
    expect(entries.first.gameId, 'g2');
    expect(entries.first.region, 'cis');
  });
}
