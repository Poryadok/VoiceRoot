import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/users_client.dart';

void main() {
  const config = GatewayConfig(baseUrl: 'http://api.test');
  const auth = 'Bearer access-token';

  group('VoiceUsersClient.searchProfiles', () {
    test('GET /api/v1/users/search?q= with Authorization', () async {
      final mock = MockClient((req) async {
        expect(req.method, 'GET');
        expect(req.url.path, '/api/v1/users/search');
        expect(req.url.queryParameters['q'], 'alice');
        expect(req.headers['Authorization'], auth);
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
            'page': {'next_cursor': 'cur-2', 'has_more': true},
          }),
          200,
        );
      });
      final client = VoiceUsersClient(httpClient: mock, config: config);
      final r = await client.searchProfiles(
        authorization: auth,
        query: 'alice',
      );
      expect(r, isA<UsersApiOk<SearchProfilesData>>());
      final data = (r as UsersApiOk<SearchProfilesData>).data;
      expect(data.profiles, hasLength(1));
      expect(data.profiles.first.username, 'alice');
      expect(data.nextCursor, 'cur-2');
      expect(data.hasMore, isTrue);
    });

    test('missing base URL', () async {
      final client = VoiceUsersClient(
        httpClient: MockClient((_) async => http.Response('', 500)),
        config: const GatewayConfig(baseUrl: ''),
      );
      final r = await client.searchProfiles(authorization: auth, query: 'x');
      expect(r, isA<UsersApiFailure>());
    });
  });

  group('VoiceUsersClient.getProfile', () {
    test('GET /api/v1/users/profiles/{id}', () async {
      final mock = MockClient((req) async {
        expect(req.url.path, '/api/v1/users/profiles/p-99');
        return http.Response(
          jsonEncode({
            'profile': {
              'id': 'p-99',
              'account_id': 'a-9',
              'username': 'bob',
              'discriminator': '0042',
              'display_name': 'Bob',
              'locale': 'en',
              'theme': 'dark',
              'is_primary': true,
              'verification_type': 'none',
            },
          }),
          200,
        );
      });
      final client = VoiceUsersClient(httpClient: mock, config: config);
      final r = await client.getProfile(
        authorization: auth,
        profileId: 'p-99',
      );
      expect(r, isA<UsersApiOk<VoiceProfile>>());
      expect((r as UsersApiOk<VoiceProfile>).data.displayName, 'Bob');
    });
  });

  group('VoiceUsersClient.getPresence', () {
    test('GET /api/v1/users/profiles/{id}/presence', () async {
      final mock = MockClient((req) async {
        expect(req.url.path, '/api/v1/users/profiles/p-1/presence');
        return http.Response(
          jsonEncode({
            'presenceStatus': {
              'profileId': 'p-1',
              'status': 'online',
            },
          }),
          200,
        );
      });
      final client = VoiceUsersClient(httpClient: mock, config: config);
      final r = await client.getPresence(
        authorization: auth,
        profileId: 'p-1',
      );
      expect(r, isA<UsersApiOk<VoicePresence>>());
      expect((r as UsersApiOk<VoicePresence>).data.isOnline, isTrue);
    });
  });

  group('VoiceUsersClient.getBulkPresence', () {
    test('POST /api/v1/users/presence/bulk', () async {
      final mock = MockClient((req) async {
        expect(req.method, 'POST');
        expect(req.url.path, '/api/v1/users/presence/bulk');
        final body = jsonDecode(req.body) as Map<String, dynamic>;
        expect(body['profileIds'], ['p-1', 'p-2']);
        return http.Response(
          jsonEncode({
            'byProfileId': {
              'p-1': {'profileId': 'p-1', 'status': 'online'},
              'p-2': {'profileId': 'p-2', 'status': 'idle'},
            },
          }),
          200,
        );
      });
      final client = VoiceUsersClient(httpClient: mock, config: config);
      final r = await client.getBulkPresence(
        authorization: auth,
        profileIds: const ['p-1', 'p-2'],
      );
      expect(r, isA<UsersApiOk<Map<String, VoicePresence>>>());
      final map = (r as UsersApiOk<Map<String, VoicePresence>>).data;
      expect(map['p-1']?.isOnline, isTrue);
      expect(map['p-2']?.isIdle, isTrue);
    });
  });
}
