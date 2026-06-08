import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_client.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/gateway_config.dart';

import 'support/gateway_test_client.dart';

void main() {
  const baseUrl = 'http://api.test';
  const config = GatewayConfig(baseUrl: baseUrl);

  Map<String, dynamic> sessionJson() => {
    'session': {
      'access_token': 'access-abc',
      'refresh_token': 'refresh-xyz',
      'expires_in_seconds': 900,
      'account_id': 'acc-1',
      'profile_id': 'prof-1',
    },
  };

  group('VoiceAuthClient.register', () {
    test('POST /api/v1/auth/register returns session', () async {
      final mock = MockClient((req) async {
        expect(req.method, 'POST');
        expect(req.url.path, '/api/v1/auth/register');
        final body = jsonDecode(req.body) as Map<String, dynamic>;
        expect(body['email'], 'user@example.com');
        expect(body['password'], 'secret');
        expect(body['guest'], false);
        return http.Response(jsonEncode(sessionJson()), 200);
      });
      final client = VoiceAuthClient(
        gateway: gatewayHttpForTest(mock, config: config),
      );
      final r = await client.register(
        email: 'user@example.com',
        password: 'secret',
      );
      expect(r, isA<AuthSessionOk>());
      final session = (r as AuthSessionOk).session;
      expect(session.accessToken, 'access-abc');
      expect(session.refreshToken, 'refresh-xyz');
      expect(session.accountId, 'acc-1');
      expect(session.activeProfileId, 'prof-1');
      expect(session.expiresInSeconds, 900);
    });

    test('missing base URL', () async {
      final client = VoiceAuthClient(
        gateway: gatewayHttpForTest(
          MockClient((_) async => http.Response('', 500)),
          config: const GatewayConfig(baseUrl: ''),
        ),
      );
      final r = await client.register(email: 'a@b.com', password: 'x');
      expect(r, isA<AuthSessionFailure>());
      expect((r as AuthSessionFailure).message, kAuthMissingBaseUrlDetail);
    });

    test('maps error JSON on 401', () async {
      final mock = MockClient((_) async {
        return http.Response(jsonEncode({'error': 'invalid_credentials'}), 401);
      });
      final client = VoiceAuthClient(
        gateway: gatewayHttpForTest(mock, config: config),
      );
      final r = await client.login(email: 'a@b.com', password: 'bad');
      expect(r, isA<AuthSessionFailure>());
      expect((r as AuthSessionFailure).errorCode, 'invalid_credentials');
    });
  });

  group('VoiceAuthClient.login', () {
    test('POST /api/v1/auth/login', () async {
      final mock = MockClient((req) async {
        expect(req.url.path, '/api/v1/auth/login');
        return http.Response(jsonEncode(sessionJson()), 200);
      });
      final client = VoiceAuthClient(
        gateway: gatewayHttpForTest(mock, config: config),
      );
      final r = await client.login(email: 'u@x.com', password: 'pw');
      expect(r, isA<AuthSessionOk>());
      final session = (r as AuthSessionOk).session;
      expect(session.accessToken, 'access-abc');
      expect(session.activeProfileId, 'prof-1');
    });
  });

  group('VoiceAuthClient.refresh', () {
    test('POST /api/v1/auth/refresh with refresh_token', () async {
      final mock = MockClient((req) async {
        expect(req.url.path, '/api/v1/auth/refresh');
        final body = jsonDecode(req.body) as Map<String, dynamic>;
        expect(body['refresh_token'], 'old-refresh');
        return http.Response(jsonEncode(sessionJson()), 200);
      });
      final client = VoiceAuthClient(
        gateway: gatewayHttpForTest(mock, config: config),
      );
      final r = await client.refresh(refreshToken: 'old-refresh');
      expect(r, isA<AuthSessionOk>());
    });
  });

  group('VoiceAuthClient.logout', () {
    test('POST /api/v1/auth/logout with Bearer and refresh_token', () async {
      final mock = MockClient((req) async {
        expect(req.url.path, '/api/v1/auth/logout');
        expect(req.headers['Authorization'], 'Bearer access-abc');
        final body = jsonDecode(req.body) as Map<String, dynamic>;
        expect(body['refresh_token'], 'refresh-xyz');
        return http.Response('', 204);
      });
      final client = VoiceAuthClient(
        gateway: gatewayHttpForTest(mock, config: config),
      );
      final err = await client.logout(
        session: AuthSession(
          accessToken: 'access-abc',
          refreshToken: 'refresh-xyz',
          accountId: 'acc-1',
          activeProfileId: 'prof-1',
          expiresInSeconds: 900,
        ),
      );
      expect(err, isNull);
    });
  });
}
