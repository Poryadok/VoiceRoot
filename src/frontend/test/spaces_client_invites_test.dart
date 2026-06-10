import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/gateway_http.dart';
import 'package:voice_frontend/backend/spaces_client.dart';

void main() {
  test('createInvite posts to space invites route', () async {
    String? path;
    String? body;
    final client = VoiceSpacesClient(
      gateway: GatewayHttpClient(
        httpClient: MockClient((request) async {
          path = request.url.path;
          body = request.body;
          return http.Response(
            jsonEncode({
              'invite': {
                'id': 'inv-1',
                'space_id': 'space-1',
                'code': 'abc123',
                'creator_profile_id': 'profile-1',
                'use_count': 0,
              },
            }),
            200,
          );
        }),
        config: const GatewayConfig(baseUrl: 'http://api.test'),
      ),
    );

    final result = await client.createInvite(
      authorization: 'Bearer t',
      spaceId: 'space-1',
      maxUses: 5,
    );

    expect(path, '/api/v1/spaces/space-1/invites');
    expect(body, contains('"max_uses":5'));
    expect(result, isA<SpacesApiOk<SpaceInvite>>());
    expect((result as SpacesApiOk<SpaceInvite>).data.code, 'abc123');
  });

  test('listInvites parses invite list', () async {
    final client = VoiceSpacesClient(
      gateway: GatewayHttpClient(
        httpClient: MockClient((request) async {
          expect(request.url.path, '/api/v1/spaces/space-1/invites');
          return http.Response(
            jsonEncode({
              'invite_list': {
                'invites': [
                  {
                    'id': 'inv-1',
                    'space_id': 'space-1',
                    'code': 'code-a',
                    'creator_profile_id': 'p1',
                    'use_count': 2,
                  },
                ],
              },
            }),
            200,
          );
        }),
        config: const GatewayConfig(baseUrl: 'http://api.test'),
      ),
    );

    final result = await client.listInvites(
      authorization: 'Bearer t',
      spaceId: 'space-1',
    );

    expect(result, isA<SpacesApiOk<List<SpaceInvite>>>());
    expect((result as SpacesApiOk<List<SpaceInvite>>).data, hasLength(1));
  });

  test('revokeInvite deletes invite route', () async {
    String? method;
    String? path;
    final client = VoiceSpacesClient(
      gateway: GatewayHttpClient(
        httpClient: MockClient((request) async {
          method = request.method;
          path = request.url.path;
          return http.Response('', 204);
        }),
        config: const GatewayConfig(baseUrl: 'http://api.test'),
      ),
    );

    final result = await client.revokeInvite(
      authorization: 'Bearer t',
      spaceId: 'space-1',
      inviteId: 'inv-9',
    );

    expect(method, 'DELETE');
    expect(path, '/api/v1/spaces/space-1/invites/inv-9');
    expect(result, isA<SpacesApiOk<void>>());
  });

  test('getInvite loads invite by code', () async {
    final client = VoiceSpacesClient(
      gateway: GatewayHttpClient(
        httpClient: MockClient((request) async {
          expect(request.url.path, '/api/v1/invites/secret');
          return http.Response(
            jsonEncode({
              'invite': {
                'id': 'inv-1',
                'space_id': 'space-1',
                'code': 'secret',
                'creator_profile_id': 'p1',
                'use_count': 0,
              },
            }),
            200,
          );
        }),
        config: const GatewayConfig(baseUrl: 'http://api.test'),
      ),
    );

    final result = await client.getInvite(
      authorization: 'Bearer t',
      code: 'secret',
    );

    expect(result, isA<SpacesApiOk<SpaceInvite>>());
    expect((result as SpacesApiOk<SpaceInvite>).data.inviteLink,
        'https://voice.gg/invite/secret');
  });

  test('joinByInvite posts join route', () async {
    String? path;
    final client = VoiceSpacesClient(
      gateway: GatewayHttpClient(
        httpClient: MockClient((request) async {
          path = request.url.path;
          return http.Response(
            jsonEncode({
              'space_membership': {
                'space_id': 'space-1',
                'profile_id': 'profile-2',
              },
            }),
            200,
          );
        }),
        config: const GatewayConfig(baseUrl: 'http://api.test'),
      ),
    );

    final result = await client.joinByInvite(
      authorization: 'Bearer t',
      code: 'secret',
    );

    expect(path, '/api/v1/invites/secret/join');
    expect(result, isA<SpacesApiOk<SpaceMembershipData>>());
    expect(
      (result as SpacesApiOk<SpaceMembershipData>).data.spaceId,
      'space-1',
    );
  });
}
