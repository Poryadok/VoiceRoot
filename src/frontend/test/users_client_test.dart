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
      final client = VoiceUsersClient(gateway: gatewayHttpForTest(mock, config: config));
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
        gateway: gatewayHttpForTest(
          MockClient((_) async => http.Response('', 500)),
          config: const GatewayConfig(baseUrl: ''),
        ),
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
      final client = VoiceUsersClient(gateway: gatewayHttpForTest(mock, config: config));
      final r = await client.getProfile(authorization: auth, profileId: 'p-99');
      expect(r, isA<UsersApiOk<VoiceProfile>>());
      expect((r as UsersApiOk<VoiceProfile>).data.displayName, 'Bob');
    });
  });

  group('VoiceUsersClient.getMe', () {
    test('GET /api/v1/users/me with Authorization', () async {
      final mock = MockClient((req) async {
        expect(req.method, 'GET');
        expect(req.url.path, '/api/v1/users/me');
        expect(req.headers['Authorization'], auth);
        return http.Response(
          jsonEncode({
            'profile': {
              'id': 'p-me',
              'account_id': 'a-me',
              'username': 'me',
              'discriminator': '0007',
              'display_name': 'Me',
              'avatar_url': 'https://cdn.example/avatars/p-me/a.png',
              'bio': 'About me',
              'locale': 'en',
              'theme': 'dark',
              'is_primary': true,
              'verification_type': 'none',
            },
          }),
          200,
        );
      });
      final client = VoiceUsersClient(gateway: gatewayHttpForTest(mock, config: config));
      final r = await client.getMe(authorization: auth);
      expect(r, isA<UsersApiOk<VoiceProfile>>());
      final profile = (r as UsersApiOk<VoiceProfile>).data;
      expect(profile.id, 'p-me');
      expect(profile.bio, 'About me');
      expect(profile.avatarUrl, 'https://cdn.example/avatars/p-me/a.png');
    });
  });

  group('VoiceUsersClient.updateProfile', () {
    test('PATCH /api/v1/users/me with editable profile fields', () async {
      final mock = MockClient((req) async {
        expect(req.method, 'PATCH');
        expect(req.url.path, '/api/v1/users/me');
        expect(req.headers['Authorization'], auth);
        expect(req.headers['Content-Type'], contains('application/json'));
        final body = jsonDecode(req.body) as Map<String, dynamic>;
        expect(body, {
          'display_name': 'Alice II',
          'bio': 'Ready for ranked',
          'avatar_url': 'https://cdn.example/avatars/p-1/a.png',
        });
        return http.Response(
          jsonEncode({
            'profile': {
              'id': 'p-1',
              'account_id': 'a-1',
              'username': 'alice',
              'discriminator': '0001',
              'display_name': 'Alice II',
              'avatar_url': 'https://cdn.example/avatars/p-1/a.png',
              'bio': 'Ready for ranked',
              'locale': 'en',
              'theme': 'dark',
              'is_primary': true,
              'verification_type': 'none',
            },
          }),
          200,
        );
      });
      final client = VoiceUsersClient(gateway: gatewayHttpForTest(mock, config: config));
      final r = await client.updateProfile(
        authorization: auth,
        displayName: 'Alice II',
        bio: 'Ready for ranked',
        avatarUrl: 'https://cdn.example/avatars/p-1/a.png',
      );
      expect(r, isA<UsersApiOk<VoiceProfile>>());
      expect((r as UsersApiOk<VoiceProfile>).data.displayName, 'Alice II');
    });
  });

  group('VoiceUsersClient.createAvatarPresignedUpload', () {
    test('POST /api/v1/users/me/avatar/presigned-upload', () async {
      final mock = MockClient((req) async {
        expect(req.method, 'POST');
        expect(req.url.path, '/api/v1/users/me/avatar/presigned-upload');
        expect(req.headers['Authorization'], auth);
        final body = jsonDecode(req.body) as Map<String, dynamic>;
        expect(body, {'content_type': 'image/png', 'content_length': '2048'});
        return http.Response(
          jsonEncode({
            'http_method': 'PUT',
            'upload_url': 'https://r2.example/presigned',
            'required_headers': {'Content-Type': 'image/png'},
            'max_bytes': 5242880,
            'expires_at': '2026-06-02T18:00:00Z',
            'public_url': 'https://cdn.example/avatars/p-1/a.png',
            'object_key': 'avatars/p-1/a.png',
          }),
          200,
        );
      });
      final client = VoiceUsersClient(gateway: gatewayHttpForTest(mock, config: config));
      final r = await client.createAvatarPresignedUpload(
        authorization: auth,
        contentType: 'image/png',
        contentLength: 2048,
      );
      expect(r, isA<UsersApiOk<AvatarPresignedUpload>>());
      final presign = (r as UsersApiOk<AvatarPresignedUpload>).data;
      expect(presign.httpMethod, 'PUT');
      expect(presign.requiredHeaders['Content-Type'], 'image/png');
      expect(presign.publicUrl, 'https://cdn.example/avatars/p-1/a.png');
    });

    test('rejects gif before calling gateway in app stack', () async {
      var called = false;
      final client = VoiceUsersClient(
        gateway: gatewayHttpForTest(
          MockClient((_) async {
            called = true;
            return http.Response('{}', 200);
          }),
          config: config,
        ),
      );
      final r = await client.createAvatarPresignedUpload(
        authorization: auth,
        contentType: 'image/gif',
        contentLength: 2048,
      );
      expect(r, isA<UsersApiFailure>());
      expect(called, isFalse);
    });
  });

  group('VoiceUsersClient.uploadAvatarBytes', () {
    test('PUTs bytes to R2 using required headers', () async {
      final mock = MockClient((req) async {
        expect(req.method, 'PUT');
        expect(req.url.toString(), 'https://r2.example/presigned');
        expect(req.headers['Content-Type'], 'image/png');
        expect(req.bodyBytes, [1, 2, 3]);
        return http.Response('', 204);
      });
      final client = VoiceUsersClient(gateway: gatewayHttpForTest(mock, config: config));
      final r = await client.uploadAvatarBytes(
        uploadUrl: Uri.parse('https://r2.example/presigned'),
        requiredHeaders: const {'Content-Type': 'image/png'},
        bytes: const [1, 2, 3],
      );
      expect(r, isA<UsersApiOk<void>>());
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
              'lastSeen': '2026-06-02T18:30:00Z',
            },
          }),
          200,
        );
      });
      final client = VoiceUsersClient(gateway: gatewayHttpForTest(mock, config: config));
      final r = await client.getPresence(authorization: auth, profileId: 'p-1');
      expect(r, isA<UsersApiOk<VoicePresence>>());
      final presence = (r as UsersApiOk<VoicePresence>).data;
      expect(presence.isOnline, isTrue);
      expect(presence.lastSeen, DateTime.utc(2026, 6, 2, 18, 30));
    });
  });

  group('VoiceUsersClient.getBulkPresence', () {
    test('POST /api/v1/users/presence/bulk', () async {
      final mock = MockClient((req) async {
        expect(req.method, 'POST');
        expect(req.url.path, '/api/v1/users/presence/bulk');
        final body = jsonDecode(req.body) as Map<String, dynamic>;
        expect(body['profile_ids'], ['p-1', 'p-2']);
        return http.Response(
          jsonEncode({
            'by_profile_id': {
              'p-1': {'profile_id': 'p-1', 'status': 'online'},
              'p-2': {
                'profile_id': 'p-2',
                'status': 'invisible',
                'last_seen': '2026-06-02T18:45:00Z',
              },
            },
          }),
          200,
        );
      });
      final client = VoiceUsersClient(gateway: gatewayHttpForTest(mock, config: config));
      final r = await client.getBulkPresence(
        authorization: auth,
        profileIds: const ['p-1', 'p-2'],
      );
      expect(r, isA<UsersApiOk<Map<String, VoicePresence>>>());
      final map = (r as UsersApiOk<Map<String, VoicePresence>>).data;
      expect(map['p-1']?.isOnline, isTrue);
      expect(map['p-2']?.status, 'invisible');
      expect(map['p-2']?.lastSeen, DateTime.utc(2026, 6, 2, 18, 45));
    });
  });
}
