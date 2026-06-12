import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;

/// Live compose e2e: Search namespace responds through Gateway.
///
/// Run: `VOICE_RUN_LIVE_COMPOSE=true flutter test test/phase9_search_e2e_live_test.dart`
void main() {
  const runLive = bool.fromEnvironment('VOICE_RUN_LIVE_COMPOSE') ||
      String.fromEnvironment('VOICE_RUN_LIVE_COMPOSE') == 'true';

  test('live compose exposes search global endpoint', () async {
    if (!runLive) {
      return;
    }

    final base = String.fromEnvironment(
      'VOICE_API_BASE_URL',
      defaultValue: 'http://127.0.0.1:8080',
    );
    final client = http.Client();
    addTearDown(client.close);

    final health = await client.get(Uri.parse('$base/health'));
    expect(health.statusCode, 200);

    final search = await client.get(
      Uri.parse('$base/api/v1/search/global?q=live-smoke'),
      headers: const {'Authorization': 'Bearer invalid-for-smoke'},
    );
    expect(search.statusCode, isNot(404));
    if (search.statusCode == 200) {
      final body = jsonDecode(search.body) as Map<String, dynamic>;
      expect(body.containsKey('global_search_results'), isTrue);
    }
  }, skip: runLive ? false : 'set VOICE_RUN_LIVE_COMPOSE=true');
}
