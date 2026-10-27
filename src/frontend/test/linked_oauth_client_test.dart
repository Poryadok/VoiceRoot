import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/gateway_http.dart';
import 'package:voice_frontend/backend/linked_oauth_client.dart';

// Consumer of immutable fixture addd4b3be9d05e821075de1def75560dec598c78:
// src/backend/auth/src/test/resources/linked-oauth-transport-v1.json.
void main() {
  test(
    'no configured provider endpoint means unavailable before any request',
    () async {
      var requests = 0;
      final c = LinkedOAuthClient(
        httpClient: MockClient((r) async {
          requests++;
          return http.Response('{}', 200);
        }),
        config: const GatewayConfig(baseUrl: 'https://gateway.test'),
        authorizationEndpoints: {},
      );
      expect(
        await c.start(
          provider: 'twitch',
          authorization: 'Bearer original',
          surface: 'web',
          challenge: 'c' * 43,
        ),
        isA<GatewayHttpFailure>(),
      );
      expect(requests, 0);
    },
  );
  for (final url in [
    'https://attacker.test/oauth2/authorize',
    'https://id.twitch.tv/wrong-path',
  ]) {
    test('rejects unexpected authorization endpoint $url', () async {
      final c = LinkedOAuthClient(
        httpClient: MockClient(
          (r) async => http.Response(
            jsonEncode({
              'attempt_id': '11111111-1111-4111-8111-111111111111',
              'authorization_url': url,
              'expires_at': '2026-10-26T01:05:00Z',
            }),
            200,
          ),
        ),
        config: const GatewayConfig(baseUrl: 'https://gateway.test'),
        authorizationEndpoints: {
          'twitch': Uri.parse('https://id.twitch.tv/oauth2/authorize'),
        },
      );
      expect(
        await c.start(
          provider: 'twitch',
          authorization: 'Bearer original',
          surface: 'web',
          challenge: 'c' * 43,
        ),
        isA<GatewayHttpFailure>(),
      );
    });
  }
  LinkedOAuthClient client(
    Future<http.Response> Function(http.Request) reply,
  ) => LinkedOAuthClient(
    httpClient: MockClient(reply),
    config: const GatewayConfig(baseUrl: 'https://gateway.test'),
    authorizationEndpoints: {
      'twitch': Uri.parse('https://id.twitch.tv/oauth2/authorize'),
    },
  );

  for (final status in [400, 401, 409, 410, 503]) {
    test(
      'consume preserves original bearer on $status and never retries',
      () async {
        var requests = 0;
        final c = client((r) async {
          requests++;
          expect(r.method, 'POST');
          expect(
            r.url.path,
            '/api/v1/auth/linked-accounts/twitch/results/consume',
          );
          expect(r.headers['authorization'], 'Bearer original');
          expect(jsonDecode(r.body), {
            'attempt_id': '11111111-1111-4111-8111-111111111111',
            'result_handle': 'h' * 43,
            'return_verifier': 'v' * 43,
          });
          return http.Response('{"error":"invalid_token"}', status);
        });
        final result = await c.consume(
          provider: 'twitch',
          authorization: 'Bearer original',
          attemptId: '11111111-1111-4111-8111-111111111111',
          resultHandle: 'h' * 43,
          verifier: 'v' * 43,
        );
        expect(result, isA<GatewayHttpFailure>());
        expect((result as GatewayHttpFailure).error.statusCode, status);
        expect(requests, 1);
      },
    );
  }

  test(
    'start exact frozen body and successful DTO, no arbitrary redirect/state',
    () async {
      final c = client((r) async {
        expect(r.url.path, '/api/v1/auth/linked-accounts/twitch/link');
        expect(jsonDecode(r.body), {
          'client_surface': 'web',
          'return_challenge': 'c' * 43,
          'return_challenge_method': 'S256',
        });
        return http.Response(
          jsonEncode({
            'attempt_id': '11111111-1111-4111-8111-111111111111',
            'authorization_url':
                'https://id.twitch.tv/oauth2/authorize?state=opaque',
            'expires_at': '2026-10-26T01:05:00Z',
          }),
          200,
        );
      });
      final result = await c.start(
        provider: 'twitch',
        authorization: 'Bearer original',
        surface: 'web',
        challenge: 'c' * 43,
      );
      expect(result, isA<GatewayHttpOk<LinkedOAuthAttempt>>());
    },
  );

  test(
    'disabled surface/provider rejects without network or fallback',
    () async {
      var requests = 0;
      final c = client((r) async {
        requests++;
        return http.Response('{}', 200);
      });
      for (final pair in [('steam', 'web'), ('twitch', 'android')]) {
        expect(
          await c.start(
            provider: pair.$1,
            authorization: 'Bearer original',
            surface: pair.$2,
            challenge: 'c' * 43,
          ),
          isA<GatewayHttpFailure>(),
        );
      }
      expect(requests, 0);
    },
  );

  test(
    'unknown outcome or malformed authorization URL never means success',
    () async {
      final c = client(
        (r) async => http.Response('{"outcome":"verified"}', 200),
      );
      expect(
        await c.consume(
          provider: 'twitch',
          authorization: 'Bearer original',
          attemptId: '11111111-1111-4111-8111-111111111111',
          resultHandle: 'h' * 43,
          verifier: 'v' * 43,
        ),
        isA<GatewayHttpFailure>(),
      );
      final bad = client(
        (r) async => http.Response(
          jsonEncode({
            'attempt_id': '11111111-1111-4111-8111-111111111111',
            'authorization_url': 'javascript:alert(1)',
            'expires_at': '2026-10-26T01:05:00Z',
          }),
          200,
        ),
      );
      expect(
        await bad.start(
          provider: 'twitch',
          authorization: 'Bearer original',
          surface: 'web',
          challenge: 'c' * 43,
        ),
        isA<GatewayHttpFailure>(),
      );
    },
  );
}
