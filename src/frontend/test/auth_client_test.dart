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

  group('VoiceAuthClient.guestReminder', () {
    test('reads protojson camelCase shouldShow from gateway', () async {
      final mock = MockClient((req) async {
        expect(req.method, 'GET');
        expect(req.url.path, '/api/v1/auth/guest-reminder');
        return http.Response(jsonEncode({'shouldShow': true}), 200);
      });
      final client = VoiceAuthClient(
        gateway: gatewayHttpForTest(mock, config: config),
      );
      final show = await client.getGuestReminderShouldShow(
        authorization: 'Bearer token',
      );
      expect(show, isTrue);
    });

    test('treats omitted protojson should_show as false', () async {
      final mock = MockClient((_) async {
        return http.Response(jsonEncode(<String, dynamic>{}), 200);
      });
      final client = VoiceAuthClient(
        gateway: gatewayHttpForTest(mock, config: config),
      );
      final show = await client.getGuestReminderShouldShow(
        authorization: 'Bearer token',
      );
      expect(show, isFalse);
    });
  });

  // T056 RED seam: `totpCode` is intentionally absent from production until
  // the accepted client contract is implemented.
  group('VoiceAuthClient.deleteAccount', () {
    test('POST /api/v1/auth/delete-account with password', () async {
      var deleteCalled = false;
      final mock = MockClient((req) async {
        expect(req.method, 'POST');
        expect(req.url.path, '/api/v1/auth/delete-account');
        expect(req.headers['authorization'], 'Bearer access-abc');
        final body = jsonDecode(req.body) as Map<String, dynamic>;
        expect(body['password'], 'secret');
        expect(body.containsKey('totp_code'), isFalse);
        deleteCalled = true;
        return http.Response('', 204);
      });
      final client = VoiceAuthClient(
        gateway: gatewayHttpForTest(mock, config: config),
      );
      final session = AuthSession(
        accessToken: 'access-abc',
        refreshToken: 'refresh-xyz',
        expiresInSeconds: 900,
        accountId: 'acc-1',
        activeProfileId: 'prof-1',
      );
      final result = await client.deleteAccount(
        session: session,
        password: 'secret',
      );
      expect(deleteCalled, isTrue);
      expect(result, isA<AuthApiOk<void>>());
    });

    test('includes optional totp_code when supplied', () async {
      final mock = MockClient((req) async {
        expect(req.method, 'POST');
        expect(req.url.path, '/api/v1/auth/delete-account');
        final body = jsonDecode(req.body) as Map<String, dynamic>;
        expect(body['password'], 'secret');
        expect(body['totp_code'], '654321');
        return http.Response('', 204);
      });
      final client = VoiceAuthClient(
        gateway: gatewayHttpForTest(mock, config: config),
      );
      final session = AuthSession(
        accessToken: 'access-abc',
        refreshToken: 'refresh-xyz',
        expiresInSeconds: 900,
        accountId: 'acc-1',
        activeProfileId: 'prof-1',
      );

      final result = await client.deleteAccount(
        session: session,
        password: 'secret',
        totpCode: '654321',
      );

      expect(result, isA<AuthApiOk<void>>());
    });
  });

  group('VoiceAuthClient.listSessions', () {
    test('GET /api/v1/auth/sessions returns device sessions', () async {
      final mock = MockClient((req) async {
        expect(req.method, 'GET');
        expect(req.url.path, '/api/v1/auth/sessions');
        return http.Response(
          jsonEncode({
            'sessions': [
              {
                'id': 'sess-current',
                'device_info_json': '{"platform":"flutter"}',
                'current': true,
              },
              {
                'id': 'sess-other',
                'device_info_json': '{"platform":"web"}',
                'current': false,
              },
            ],
          }),
          200,
        );
      });
      final client = VoiceAuthClient(
        gateway: gatewayHttpForTest(mock, config: config),
      );
      final session = AuthSession(
        accessToken: 'access-abc',
        refreshToken: 'refresh-xyz',
        expiresInSeconds: 900,
        accountId: 'acc-1',
        activeProfileId: 'prof-1',
      );
      final result = await client.listSessions(session: session);
      expect(result, isA<AuthApiOk<List<AuthDeviceSession>>>());
      final data = (result as AuthApiOk<List<AuthDeviceSession>>).data;
      expect(data, hasLength(2));
      expect(data.first.current, isTrue);
      expect(data.first.deviceLabel, 'flutter');
      expect(data.last.id, 'sess-other');
    });
  });

  group('VoiceAuthClient.passwordReset', () {
    test('POST /api/v1/auth/otp/send for password_reset', () async {
      var sendCalled = false;
      final mock = MockClient((req) async {
        expect(req.method, 'POST');
        expect(req.url.path, '/api/v1/auth/otp/send');
        final body = jsonDecode(req.body) as Map<String, dynamic>;
        expect(body['email'], 'user@example.com');
        expect(body['otp_type'], 'password_reset');
        sendCalled = true;
        return http.Response('', 204);
      });
      final client = VoiceAuthClient(
        gateway: gatewayHttpForTest(mock, config: config),
      );
      final result = await client.sendPasswordResetOtp(
        email: 'user@example.com',
      );
      expect(sendCalled, isTrue);
      expect(result, isA<AuthApiOk<void>>());
    });

    test('POST /api/v1/auth/password/reset', () async {
      var resetCalled = false;
      final mock = MockClient((req) async {
        expect(req.method, 'POST');
        expect(req.url.path, '/api/v1/auth/password/reset');
        final body = jsonDecode(req.body) as Map<String, dynamic>;
        expect(body['email'], 'user@example.com');
        expect(body['code'], '123456');
        expect(body['new_password'], 'newpass99');
        resetCalled = true;
        return http.Response('', 204);
      });
      final client = VoiceAuthClient(
        gateway: gatewayHttpForTest(mock, config: config),
      );
      final result = await client.resetPassword(
        email: 'user@example.com',
        code: '123456',
        newPassword: 'newpass99',
      );
      expect(resetCalled, isTrue);
      expect(result, isA<AuthApiOk<void>>());
    });

    test('maps invalid_otp on reset failure', () async {
      final mock = MockClient((_) async {
        return http.Response(jsonEncode({'error': 'invalid_otp'}), 401);
      });
      final client = VoiceAuthClient(
        gateway: gatewayHttpForTest(mock, config: config),
      );
      final result = await client.resetPassword(
        email: 'user@example.com',
        code: '000000',
        newPassword: 'newpass99',
      );
      expect(result, isA<AuthApiFailure>());
      expect((result as AuthApiFailure).errorCode, 'invalid_otp');
    });
  });

  group('VoiceAuthClient.guestConversionOtp', () {
    test(
      'sends and verifies email_verify OTP with the guest session',
      () async {
        var sent = false;
        var verified = false;
        final mock = MockClient((req) async {
          expect(req.headers['authorization'], 'Bearer guest-access');
          final body = jsonDecode(req.body) as Map<String, dynamic>;
          expect(body['email'], 'guest@example.com');
          expect(body['otp_type'], 'email_verify');
          if (req.url.path == '/api/v1/auth/otp/send') {
            sent = true;
            return http.Response('', 204);
          }
          expect(req.url.path, '/api/v1/auth/otp/verify');
          expect(body['code'], '123456');
          verified = true;
          return http.Response('', 204);
        });
        final client = VoiceAuthClient(
          gateway: gatewayHttpForTest(mock, config: config),
        );
        const guest = AuthSession(
          accessToken: 'guest-access',
          refreshToken: 'guest-refresh',
          accountId: 'acc-1',
          activeProfileId: 'prof-1',
          expiresInSeconds: 900,
          accountType: 'guest',
        );

        expect(
          await client.sendGuestConversionEmailOtp(
            session: guest,
            email: 'guest@example.com',
          ),
          isA<AuthApiOk<void>>(),
        );
        expect(
          await client.verifyGuestConversionEmailOtp(
            session: guest,
            email: 'guest@example.com',
            code: '123456',
          ),
          isA<AuthApiOk<void>>(),
        );
        expect(sent, isTrue);
        expect(verified, isTrue);
      },
    );
  });

  group('VoiceAuthClient.revokeSession', () {
    test('POST /api/v1/auth/sessions/{id}/revoke', () async {
      var revokeCalled = false;
      final mock = MockClient((req) async {
        expect(req.method, 'POST');
        expect(req.url.path, '/api/v1/auth/sessions/sess-other/revoke');
        revokeCalled = true;
        return http.Response('', 204);
      });
      final client = VoiceAuthClient(
        gateway: gatewayHttpForTest(mock, config: config),
      );
      final session = AuthSession(
        accessToken: 'access-abc',
        refreshToken: 'refresh-xyz',
        expiresInSeconds: 900,
        accountId: 'acc-1',
        activeProfileId: 'prof-1',
      );
      final result = await client.revokeSession(
        session: session,
        sessionId: 'sess-other',
      );
      expect(revokeCalled, isTrue);
      expect(result, isA<AuthApiOk<void>>());
    });
  });
}
