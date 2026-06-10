import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/gateway_http.dart';

/// HTTP contract tests for space moderation routes (implementation in VoiceSpacesClient follows).
void main() {
  test('SpaceModeration banMember posts to bans route', () async {
    String? method;
    String? path;
    String? body;
    final gateway = GatewayHttpClient(
      httpClient: MockClient((request) async {
        method = request.method;
        path = request.url.path;
        body = request.body;
        return http.Response('', 204);
      }),
      config: const GatewayConfig(baseUrl: 'http://api.test'),
    );

    final uri = gateway.resolve('/api/v1/spaces/space-1/bans');
    final result = await gateway.postJson(
      uri: uri,
      authorization: 'Bearer t',
      body: {'account_id': 'account-b', 'reason': 'spam'},
    );

    expect(method, 'POST');
    expect(path, '/api/v1/spaces/space-1/bans');
    expect(body, contains('"account_id":"account-b"'));
    expect(result, isA<GatewayHttpOk<void>>());
  });

  test('SpaceModeration unbanMember deletes ban route', () async {
    String? method;
    String? path;
    final gateway = GatewayHttpClient(
      httpClient: MockClient((request) async {
        method = request.method;
        path = request.url.path;
        return http.Response('', 204);
      }),
      config: const GatewayConfig(baseUrl: 'http://api.test'),
    );

    final result = await gateway.deleteEmpty(
      uri: gateway.resolve('/api/v1/spaces/space-1/bans/account-b'),
      authorization: 'Bearer t',
    );

    expect(method, 'DELETE');
    expect(path, '/api/v1/spaces/space-1/bans/account-b');
    expect(result, isA<GatewayHttpOk<void>>());
  });

  test('SpaceModeration listBans parses ban list', () async {
    final gateway = GatewayHttpClient(
      httpClient: MockClient((request) async {
        expect(request.url.path, '/api/v1/spaces/space-1/bans');
        return http.Response(
          jsonEncode({
            'ban_list': {
              'bans': [
                {
                  'space_id': 'space-1',
                  'account_id': 'account-b',
                  'banned_by_profile_id': 'mod-1',
                  'reason': 'abuse',
                },
              ],
            },
          }),
          200,
        );
      }),
      config: const GatewayConfig(baseUrl: 'http://api.test'),
    );

    final result = await gateway.getJson(
      gateway.resolve('/api/v1/spaces/space-1/bans'),
      authorization: 'Bearer t',
    );

    expect(result, isA<GatewayHttpOk<Map<String, dynamic>>>());
    final data = (result as GatewayHttpOk<Map<String, dynamic>>).data;
    final bans = (data['ban_list'] as Map<String, dynamic>)['bans'] as List;
    expect(bans, hasLength(1));
    expect(bans.first['account_id'], 'account-b');
  });

  test('SpaceModeration timeoutMember posts timeout route', () async {
    String? path;
    String? body;
    final gateway = GatewayHttpClient(
      httpClient: MockClient((request) async {
        path = request.url.path;
        body = request.body;
        return http.Response('', 204);
      }),
      config: const GatewayConfig(baseUrl: 'http://api.test'),
    );

    final result = await gateway.postJson(
      uri: gateway.resolve(
        '/api/v1/spaces/space-1/members/profile-b/timeout',
      ),
      authorization: 'Bearer t',
      body: {'duration_seconds': 600, 'reason': 'heated'},
    );

    expect(path, '/api/v1/spaces/space-1/members/profile-b/timeout');
    expect(body, contains('"duration_seconds":600'));
    expect(result, isA<GatewayHttpOk<void>>());
  });

  test('SpaceModeration removeMemberTimeout deletes timeout route', () async {
    String? path;
    final gateway = GatewayHttpClient(
      httpClient: MockClient((request) async {
        path = request.url.path;
        return http.Response('', 204);
      }),
      config: const GatewayConfig(baseUrl: 'http://api.test'),
    );

    final result = await gateway.deleteEmpty(
      uri: gateway.resolve(
        '/api/v1/spaces/space-1/members/profile-b/timeout',
      ),
      authorization: 'Bearer t',
    );

    expect(path, '/api/v1/spaces/space-1/members/profile-b/timeout');
    expect(result, isA<GatewayHttpOk<void>>());
  });
}
