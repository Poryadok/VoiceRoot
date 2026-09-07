import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/gateway_config.dart';

import 'support/live_gateway_harness.dart';

void main() {
  test(
    'polls recipient-scoped mail and extracts the six-digit verification code',
    () async {
      const email = 'verification-unique@voice-qa.test';
      var attempts = 0;
      final client = MockClient((request) async {
        attempts += 1;
        expect(request.method, 'GET');
        expect(request.url.path, '/emails/latest');
        expect(request.url.queryParameters['to'], email);
        if (attempts == 1) {
          return http.Response(jsonEncode({'error': 'mail_not_found'}), 404);
        }
        return http.Response(
          jsonEncode({
            'id': 're_latest',
            'to': [email],
            'subject': 'Verify your Voice email',
            'text':
                'Your Voice verification code is 654321 (expires in 10 minutes).',
          }),
          200,
          headers: const {'content-type': 'application/json'},
        );
      });

      final code = await waitForLiveVerificationCode(
        httpClient: client,
        stubBaseUrl: Uri.parse('http://mail-stub.test:4180'),
        email: email,
        pollInterval: Duration.zero,
        maxAttempts: 2,
      );

      expect(code, '654321');
      expect(attempts, 2);
    },
  );

  test(
    'refreshes a pending conversion without sending or verifying OTP again',
    () async {
      var sendCalls = 0;
      var verifyCalls = 0;
      var refreshCalls = 0;
      var visibilityCalls = 0;
      final refreshTokens = <String>[];
      final client = MockClient((request) async {
        switch (request.url.path) {
          case '/api/v1/auth/otp/send':
            sendCalls += 1;
            expect(request.headers['Authorization'], 'Bearer pending-access');
            return http.Response('', 204);
          case '/emails/latest':
            return http.Response(
              jsonEncode({
                'id': 're_latest',
                'to': ['pending@voice-qa.test'],
                'subject': 'Verify your Voice email',
                'text': 'Your Voice verification code is 654321.',
              }),
              200,
            );
          case '/api/v1/auth/otp/verify':
            verifyCalls += 1;
            return http.Response(
              jsonEncode({'error': 'verification_pending'}),
              503,
            );
          case '/api/v1/auth/refresh':
            refreshCalls += 1;
            final requestBody =
                jsonDecode(request.body) as Map<String, dynamic>;
            refreshTokens.add(requestBody['refresh_token'] as String);
            return http.Response(
              jsonEncode({
                'session': {
                  'access_token': 'access-$refreshCalls',
                  'refresh_token': 'refresh-$refreshCalls',
                  'expires_in_seconds': 900,
                  'account_id': 'account-1',
                  'profile_id': 'profile-1',
                  'account_type': refreshCalls == 1 ? 'guest' : 'regular',
                },
              }),
              200,
            );
          case '/api/v1/users/me':
            visibilityCalls += 1;
            expect(request.headers['Authorization'], 'Bearer access-2');
            return http.Response(jsonEncode({'id': 'profile-1'}), 200);
          default:
            return http.Response('not found', 404);
        }
      });
      final context = LiveGatewayContext(
        config: const GatewayConfig(baseUrl: 'http://gateway.test'),
        httpClient: client,
      );
      const pending = AuthSession(
        accessToken: 'pending-access',
        refreshToken: 'pending-refresh',
        accountId: 'account-1',
        activeProfileId: 'profile-1',
        expiresInSeconds: 900,
        accountType: 'guest',
      );

      final verified = await context.completeEmailVerification(
        email: 'pending@voice-qa.test',
        pendingSession: pending,
        pendingRefreshInterval: Duration.zero,
        maxPendingRefreshAttempts: 2,
      );

      expect(verified.accountType, 'regular');
      expect(verified.accountId, pending.accountId);
      expect(verified.activeProfileId, pending.activeProfileId);
      expect(sendCalls, 1);
      expect(verifyCalls, 1);
      expect(refreshCalls, 2);
      expect(visibilityCalls, 1);
      expect(refreshTokens, ['pending-refresh', 'refresh-1']);
    },
  );

  test(
    'account visibility failure omits JSON error_code and message from diagnostics',
    () async {
      const sensitiveErrorCode = 'visibility_denied_token_must_not_appear';
      const sensitiveMessage = 'Bearer access-token-must-not-appear';
      final client = MockClient((request) async {
        switch (request.url.path) {
          case '/api/v1/auth/otp/send':
            return http.Response('', 204);
          case '/emails/latest':
            return http.Response(
              jsonEncode({
                'to': ['failure@voice-qa.test'],
                'text': 'Your Voice verification code is 654321.',
              }),
              200,
            );
          case '/api/v1/auth/otp/verify':
            return http.Response(
              jsonEncode({
                'session': {
                  'access_token': 'verified-access',
                  'refresh_token': 'verified-refresh',
                  'expires_in_seconds': 900,
                  'account_id': 'account-1',
                  'profile_id': 'profile-1',
                  'account_type': 'regular',
                },
              }),
              200,
            );
          case '/api/v1/users/me':
            return http.Response(
              jsonEncode({
                'error_code': sensitiveErrorCode,
                'message': sensitiveMessage,
              }),
              403,
            );
          default:
            return http.Response('not found', 404);
        }
      });
      final context = LiveGatewayContext(
        config: const GatewayConfig(baseUrl: 'http://gateway.test'),
        httpClient: client,
      );
      const pending = AuthSession(
        accessToken: 'pending-access',
        refreshToken: 'pending-refresh',
        accountId: 'account-1',
        activeProfileId: 'profile-1',
        expiresInSeconds: 900,
        accountType: 'guest',
      );

      await expectLater(
        context.completeEmailVerification(
          email: 'failure@voice-qa.test',
          pendingSession: pending,
        ),
        throwsA(
          isA<TestFailure>()
              .having((failure) => failure.message, 'message', contains('403'))
              .having(
                (failure) => failure.message,
                'message',
                isNot(contains(sensitiveErrorCode)),
              )
              .having(
                (failure) => failure.message,
                'message',
                isNot(contains(sensitiveMessage)),
              ),
        ),
      );
    },
  );
}
