import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/gateway_http.dart';
import 'package:voice_frontend/backend/roles_client.dart';

void main() {
  test('listRoles loads space role hierarchy', () async {
    String? path;
    final client = VoiceRolesClient(
      gateway: GatewayHttpClient(
        httpClient: MockClient((request) async {
          path = request.url.path;
          return http.Response(
            jsonEncode({
              'role_list': {
                'roles': [
                  {'id': 'r1', 'space_id': 'space-1', 'name': 'Owner', 'position': 4},
                  {'id': 'r2', 'space_id': 'space-1', 'name': 'Member', 'position': 1},
                ],
              },
            }),
            200,
          );
        }),
        config: const GatewayConfig(baseUrl: 'http://api.test'),
      ),
    );

    final result = await client.listRoles(
      authorization: 'Bearer t',
      spaceId: 'space-1',
    );

    expect(path, '/api/v1/roles');
    expect(result, isA<RolesApiOk<List<SpaceRole>>>());
    final roles = (result as RolesApiOk<List<SpaceRole>>).data;
    expect(roles, hasLength(2));
    expect(roles.first.name, 'Owner');
  });

  test('assignRole posts assign route', () async {
    String? path;
    String? body;
    final client = VoiceRolesClient(
      gateway: GatewayHttpClient(
        httpClient: MockClient((request) async {
          path = request.url.path;
          body = request.body;
          return http.Response('', 200);
        }),
        config: const GatewayConfig(baseUrl: 'http://api.test'),
      ),
    );

    final result = await client.assignRole(
      authorization: 'Bearer t',
      spaceId: 'space-1',
      profileId: 'profile-2',
      roleId: 'role-member',
    );

    expect(path, '/api/v1/roles/assign');
    expect(body, contains('"profile_id":"profile-2"'));
    expect(result, isA<RolesApiOk<void>>());
  });

  test('checkPermission parses allowed flag', () async {
    final client = VoiceRolesClient(
      gateway: GatewayHttpClient(
        httpClient: MockClient((request) async {
          expect(request.url.path, '/api/v1/roles/check');
          expect(request.url.query, contains('permission_name=SPACE_MANAGE_INVITES'));
          return http.Response(jsonEncode({'allowed': false}), 200);
        }),
        config: const GatewayConfig(baseUrl: 'http://api.test'),
      ),
    );

    final result = await client.checkPermission(
      authorization: 'Bearer t',
      spaceId: 'space-1',
      profileId: 'profile-1',
      permissionName: 'SPACE_MANAGE_INVITES',
    );

    expect(result, isA<RolesApiOk<bool>>());
    expect((result as RolesApiOk<bool>).data, isFalse);
  });
}
