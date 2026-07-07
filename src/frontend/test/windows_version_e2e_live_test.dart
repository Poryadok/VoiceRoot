import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;

/// Live compose e2e: Windows desktop version policy and dynamic 426 enforcement.
///
/// Run: `VOICE_RUN_LIVE_COMPOSE=true flutter test test/windows_version_e2e_live_test.dart`
void main() {
  const runLive = bool.fromEnvironment('VOICE_RUN_LIVE_COMPOSE') ||
      String.fromEnvironment('VOICE_RUN_LIVE_COMPOSE') == 'true';

  test('live compose serves windows version policy and blocks outdated client', () async {
    if (!runLive) {
      return;
    }

    final base = String.fromEnvironment(
      'VOICE_API_BASE_URL',
      defaultValue: 'http://127.0.0.1:8080',
    );
    final client = http.Client();
    addTearDown(client.close);

    final version = await client.get(
      Uri.parse('$base/api/v1/version?platform=windows&version=0.0.1'),
    );
    expect(version.statusCode, 200);
    final body = jsonDecode(version.body) as Map<String, dynamic>;
    expect(body['force_update'], isTrue);
    expect(body['update_url'], isNotEmpty);

    final blocked = await client.get(
      Uri.parse('$base/api/v1/users/me'),
      headers: const {
        'X-Voice-Client-Platform': 'windows',
        'X-Voice-Client-Version': '0.0.1',
      },
    );
    expect(blocked.statusCode, 426);
    final err = jsonDecode(blocked.body) as Map<String, dynamic>;
    expect(err['error'], 'client_outdated');
  }, skip: runLive ? false : 'set VOICE_RUN_LIVE_COMPOSE=true');
}
