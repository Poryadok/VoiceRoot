import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/gifs_client.dart';

import 'support/gateway_test_client.dart';

void main() {
  test('GIF search preserves Chat cursor and pending File identity', () async {
    final requests = <http.Request>[];
    final client = VoiceGifsClient(
      gateway: gatewayHttpForTest(
        MockClient((request) async {
          requests.add(request);
          return http.Response(
            jsonEncode({
              'items': [
                {
                  'provider': 'fixture',
                  'provider_id': 'wave',
                  'preview_url': 'https://cdn.test/wave.gif',
                  'width': 120,
                  'height': 80,
                },
              ],
              'next_cursor': 'opaque/+==',
            }),
            200,
          );
        }),
        config: const GatewayConfig(baseUrl: 'http://api.test'),
      ),
    );
    final result = await client.search(
      authorization: 'Bearer a',
      query: 'hello world',
      cursor: 'opaque/+==',
    );
    expect(result, isA<ChatsApiOk<GifCatalogPage>>());
    final page = (result as ChatsApiOk<GifCatalogPage>).data;
    expect(requests.single.url.path, '/api/v1/gifs/search');
    expect(requests.single.url.queryParameters, {
      'q': 'hello world',
      'limit': '20',
      'cursor': 'opaque/+==',
    });
    expect(requests.single.headers['authorization'], 'Bearer a');
    expect(page.nextCursor, 'opaque/+==');
    expect(page.items.single.providerId, 'wave');
    expect(
      page.items.single.fileId,
      isEmpty,
      reason:
          'A provider result must never acquire an invented sendable File ID',
    );
  });

  test(
    'trending uses its actual response and unavailable remains failure',
    () async {
      var calls = 0;
      final client = VoiceGifsClient(
        gateway: gatewayHttpForTest(
          MockClient((request) async {
            calls++;
            expect(request.url.path, '/api/v1/gifs/trending');
            if (calls == 1) {
              return http.Response('{"items":[],"next_cursor":"next"}', 200);
            }
            return http.Response(
              '{"error_code":"unavailable","message":"GIF search unavailable"}',
              503,
              headers: {'X-Voice-GRPC-Code': 'Unavailable'},
            );
          }),
          config: const GatewayConfig(baseUrl: 'http://api.test'),
        ),
      );
      final first = await client.trending(authorization: 'Bearer a');
      expect((first as ChatsApiOk<GifCatalogPage>).data.nextCursor, 'next');
      final failure = await client.trending(
        authorization: 'Bearer a',
        cursor: 'next',
      );
      expect(failure, isA<ChatsApiFailure>());
      expect((failure as ChatsApiFailure).statusCode, 503);
      expect(failure.errorCode, 'unavailable');
    },
  );
}
