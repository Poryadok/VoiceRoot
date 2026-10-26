import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/gateway_http.dart';
import 'package:voice_frontend/backend/space_catalog_client.dart';

void main() {
  test(
    'public join sends one empty-body request and requires matching membership',
    () async {
      var calls = 0;
      final client = SpaceCatalogClient(
        GatewayHttpClient(
          config: const GatewayConfig(baseUrl: 'https://api.test'),
          httpClient: MockClient((req) async {
            calls++;
            expect(req.method, 'POST');
            expect(req.url.path, '/api/v1/spaces/public-space/join');
            expect(req.body, isEmpty);
            return http.Response(
              '{"space_membership":{"space_id":"public-space"}}',
              200,
            );
          }),
        ),
      );
      await client.join(
        authorization: 'Bearer member',
        spaceId: 'public-space',
      );
      expect(calls, 1);
    },
  );
  test(
    'catalog browse preserves server order and proto omitted defaults',
    () async {
      final client = SpaceCatalogClient(
        GatewayHttpClient(
          config: const GatewayConfig(baseUrl: 'https://api.test'),
          httpClient: MockClient((request) async {
            expect(request.url.path, '/api/v1/spaces/catalog');
            expect(request.headers['authorization'], 'Bearer catalog');
            expect(request.url.queryParameters, {'page_size': '50'});
            return http.Response(
              jsonEncode({
                'space_list': {
                  'spaces': [
                    {
                      'id': 'verified',
                      'name': 'Verified',
                      'is_verified': true,
                      'member_count': 7,
                      'visibility': 'public',
                    },
                    {'id': 'ordinary', 'name': 'Ordinary'},
                  ],
                  'next_cursor': 'opaque',
                },
              }),
              200,
            );
          }),
        ),
      );
      final page = await client.list(authorization: 'Bearer catalog');
      expect(page.spaces.map((s) => s.id), ['verified', 'ordinary']);
      expect(page.spaces.first.isVerified, isTrue);
      expect(page.spaces.first.visibility, 'public');
      expect(page.spaces.last.visibility, '');
      expect(page.spaces.last.isVerified, isFalse);
      expect(page.spaces.last.memberCount, 0);
      expect(page.nextCursor, 'opaque');
    },
  );

  test(
    'query and cursor use exact authenticated contract; failures propagate',
    () async {
      final client = SpaceCatalogClient(
        GatewayHttpClient(
          config: const GatewayConfig(baseUrl: 'https://api.test'),
          httpClient: MockClient((request) async {
            expect(request.url.queryParameters, {
              'q': 'raid',
              'cursor': 'opaque+/=',
              'page_size': '50',
            });
            return http.Response('{"message":"unavailable"}', 503);
          }),
        ),
      );
      await expectLater(
        client.list(
          authorization: 'Bearer catalog',
          query: ' raid ',
          cursor: 'opaque+/=',
        ),
        throwsA(isA<SpaceCatalogFailure>()),
      );
    },
  );
}
