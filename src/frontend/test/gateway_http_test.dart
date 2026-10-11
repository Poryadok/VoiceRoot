import 'dart:async';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/gateway_http.dart';
import 'package:voice_frontend/backend/gateway_request_id.dart';
import 'package:voice_frontend/gen/voice/messaging/v1/messaging.pb.dart'
    as messaging_pb;

import 'support/gateway_test_client.dart';

void main() {
  test('newGatewayRequestId returns 32 lowercase hex chars', () {
    expect(newGatewayRequestId(), matches(RegExp(r'^[0-9a-f]{32}$')));
  });

  test('GatewayHttpClient sends X-Request-Id per request', () async {
    const config = GatewayConfig(baseUrl: 'http://api.test');
    String? capturedRequestId;
    final mock = MockClient((req) async {
      capturedRequestId = req.headers['X-Request-Id'];
      return http.Response('', 204);
    });
    final client = gatewayHttpForTest(mock, config: config);
    final result = await client.deleteEmpty(
      uri: Uri.parse('http://api.test/api/v1/chats/chat-1'),
      authorization: 'Bearer token',
    );
    expect(result, isA<GatewayHttpOk<void>>());
    expect(capturedRequestId, isNotNull);
    expect(capturedRequestId, matches(RegExp(r'^[0-9a-f]{32}$')));
  });

  for (final operation in _Mutation.values) {
    test(
      '${operation.name} cancels 401 replay after a profile switch',
      () async {
        var generation = 7;
        var profileId = 'profile-a';
        var token = 'Bearer profile-a';
        var attempts = 0;
        final refreshStarted = Completer<void>();
        final finishRefresh = Completer<void>();
        final client = GatewayHttpClient(
          httpClient: MockClient((request) async {
            attempts++;
            return attempts == 1
                ? http.Response('{"error":"invalid_token"}', 401)
                : http.Response('{}', 200);
          }),
          config: const GatewayConfig(baseUrl: 'http://api.test'),
          authorizationProvider: () => token,
          requestIdentityProvider: () => GatewayRequestIdentity(
            generation: generation,
            accountId: 'account-1',
            profileId: profileId,
          ),
          onUnauthorized: (identity) async {
            expect(identity.profileId, 'profile-a');
            refreshStarted.complete();
            await finishRefresh.future;
            return true;
          },
        );

        final request = _sendMutation(client, operation);
        await refreshStarted.future;
        generation++;
        profileId = 'profile-b';
        token = 'Bearer profile-b';
        finishRefresh.complete();
        await request;

        expect(attempts, 1, reason: 'profile A mutation must not be replayed');
      },
    );

    test(
      '${operation.name} cancels replay when logout removes the session',
      () async {
        var generation = 7;
        var isLoggedIn = true;
        var attempts = 0;
        final refreshStarted = Completer<void>();
        final finishRefresh = Completer<void>();
        final client = GatewayHttpClient(
          httpClient: MockClient((request) async {
            attempts++;
            return attempts == 1
                ? http.Response('{"error":"invalid_token"}', 401)
                : http.Response('{}', 200);
          }),
          config: const GatewayConfig(baseUrl: 'http://api.test'),
          authorizationProvider: () => isLoggedIn ? 'Bearer profile-a' : null,
          requestIdentityProvider: () => isLoggedIn
              ? GatewayRequestIdentity(
                  generation: generation,
                  accountId: 'account-1',
                  profileId: 'profile-a',
                )
              : null,
          onUnauthorized: (_) async {
            refreshStarted.complete();
            await finishRefresh.future;
            return true;
          },
        );

        final request = _sendMutation(client, operation);
        await refreshStarted.future;
        generation++;
        isLoggedIn = false;
        finishRefresh.complete();
        await request;

        expect(attempts, 1, reason: 'logout cancels the pending replay');
      },
    );

    test(
      '${operation.name} retries after same-profile token refresh',
      () async {
        var generation = 11;
        var token = 'Bearer profile-a-expired';
        var attempts = 0;
        final observedAuthorization = <String?>[];
        final client = GatewayHttpClient(
          httpClient: MockClient((request) async {
            attempts++;
            observedAuthorization.add(request.headers['authorization']);
            return attempts == 1
                ? http.Response('{"error":"invalid_token"}', 401)
                : http.Response('{}', 200);
          }),
          config: const GatewayConfig(baseUrl: 'http://api.test'),
          authorizationProvider: () => token,
          requestIdentityProvider: () => GatewayRequestIdentity(
            generation: generation,
            accountId: 'account-1',
            profileId: 'profile-a',
          ),
          onUnauthorized: (requestGeneration) async {
            expect(requestGeneration.profileId, 'profile-a');
            token = 'Bearer profile-a-renewed';
            return true;
          },
        );

        await _sendMutation(
          client,
          operation,
          authorization: 'Bearer profile-a-expired',
        );

        expect(attempts, 2);
        expect(observedAuthorization, [
          'Bearer profile-a-expired',
          'Bearer profile-a-renewed',
        ]);
      },
    );
  }

  test(
    'logout during refresh never falls back to the request bearer',
    () async {
      var generation = 4;
      String? token = 'Bearer profile-a';
      var attempts = 0;
      final refreshStarted = Completer<void>();
      final finishRefresh = Completer<void>();
      final client = GatewayHttpClient(
        httpClient: MockClient((request) async {
          attempts++;
          return attempts == 1
              ? http.Response('{"error":"invalid_token"}', 401)
              : http.Response('{}', 200);
        }),
        config: const GatewayConfig(baseUrl: 'http://api.test'),
        authorizationProvider: () => token,
        requestIdentityProvider: () => token == null
            ? null
            : GatewayRequestIdentity(
                generation: generation,
                accountId: 'account-1',
                profileId: 'profile-a',
              ),
        onUnauthorized: (_) async {
          refreshStarted.complete();
          await finishRefresh.future;
          return true;
        },
      );

      final request = client.postJson(
        uri: Uri.parse('http://api.test/api/v1/messages'),
        authorization: 'Bearer profile-a',
        body: {'content': 'private A message'},
      );
      await refreshStarted.future;
      generation++;
      token = null;
      finishRefresh.complete();
      await request;

      expect(attempts, 1);
    },
  );

  test(
    'late profile A 401 cannot refresh or replay after switching to B',
    () async {
      var generation = 2;
      var profileId = 'profile-a';
      var token = 'Bearer profile-a';
      var attempts = 0;
      var refreshCalls = 0;
      final firstResponse = Completer<http.Response>();
      final requestStarted = Completer<void>();
      final client = GatewayHttpClient(
        httpClient: MockClient((request) async {
          attempts++;
          if (attempts == 1) {
            requestStarted.complete();
            return firstResponse.future;
          }
          return http.Response('{}', 200);
        }),
        config: const GatewayConfig(baseUrl: 'http://api.test'),
        authorizationProvider: () => token,
        requestIdentityProvider: () => GatewayRequestIdentity(
          generation: generation,
          accountId: 'account-1',
          profileId: profileId,
        ),
        onUnauthorized: (_) async {
          refreshCalls++;
          return true;
        },
      );

      final request = client.postJson(
        uri: Uri.parse('http://api.test/api/v1/messages'),
        authorization: 'Bearer profile-a',
        body: {'content': 'private A message'},
      );
      await requestStarted.future;
      generation++;
      profileId = 'profile-b';
      token = 'Bearer profile-b';
      firstResponse.complete(http.Response('{"error":"invalid_token"}', 401));
      await request;

      expect(refreshCalls, 0);
      expect(attempts, 1);
    },
  );
}

enum _Mutation { proto, json, voidRequest }

Future<void> _sendMutation(
  GatewayHttpClient client,
  _Mutation operation, {
  String? authorization,
}) async {
  final uri = Uri.parse('http://api.test/api/v1/messages');
  switch (operation) {
    case _Mutation.proto:
      await client.postProto<messaging_pb.SendMessageResponse>(
        uri: uri,
        authorization: authorization,
        body: messaging_pb.SendMessageRequest(),
        createEmpty: messaging_pb.SendMessageResponse.new,
      );
      break;
    case _Mutation.json:
      await client.postJson(
        uri: uri,
        authorization: authorization,
        body: {'content': 'profile A message'},
      );
      break;
    case _Mutation.voidRequest:
      await client.postEmpty(
        uri: uri,
        authorization: authorization,
        jsonBody: {'read': true},
      );
      break;
  }
}
