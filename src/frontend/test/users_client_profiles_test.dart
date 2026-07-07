import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/users_client.dart';

import 'support/gateway_test_client.dart';

void main() {
  const config = GatewayConfig(baseUrl: 'http://api.test');
  const auth = 'Bearer access-token';

  group('app stack3 VoiceUsersClient.listMyProfiles', () {
    test('GET /api/v1/users/profiles (not /users/me/profiles)', () async {
      final mock = MockClient((req) async {
        expect(req.method, 'GET');
        expect(
          req.url.path,
          '/api/v1/users/profiles',
          reason: 'listMyProfiles must use canonical app stack3 path',
        );
        return http.Response(
          jsonEncode({
            'profile_list': {
              'profiles': [
                {
                  'id': 'p-1',
                  'account_id': 'a-1',
                  'username': 'alice',
                  'discriminator': '0001',
                  'display_name': 'Alice',
                  'locale': 'en',
                  'theme': 'dark',
                  'is_primary': true,
                  'verification_type': 'none',
                },
              ],
            },
          }),
          200,
        );
      });
      final client = VoiceUsersClient(
        gateway: gatewayHttpForTest(mock, config: config),
      );
      final r = await client.listMyProfiles(authorization: auth);
      expect(r, isA<UsersApiOk<List<VoiceProfile>>>());
    });
  });
}
