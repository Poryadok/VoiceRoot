import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/gateway_config.dart';

import 'support/gateway_test_client.dart';

void main() {
  const config = GatewayConfig(baseUrl: 'http://api.test');
  const auth = 'Bearer access-token';

  test('sticker packs remain isolated to the active profile request', () async {
    final paths = <String>[];
    final client = VoiceStickerPacksClient(
      gateway: gatewayHttpForTest(
        MockClient((request) async {
          paths.add(request.url.path);
          expect(request.headers['Authorization'], auth);
          if (request.method == 'GET') {
            return http.Response(jsonEncode({'packs': []}), 200);
          }
          return http.Response('', 204);
        }),
        config: config,
      ),
    );

    expect(
      await client.listInstalled(authorization: auth),
      isA<ChatsApiOk<StickerPackListData>>(),
    );
    expect(
      await client.install(authorization: auth, packId: 'user-pack'),
      isA<ChatsApiOk<void>>(),
    );
    expect(
      await client.uninstall(authorization: auth, packId: 'user-pack'),
      isA<ChatsApiOk<void>>(),
    );
    expect(paths, [
      '/api/v1/sticker-packs',
      '/api/v1/sticker-packs/user-pack/install',
      '/api/v1/sticker-packs/user-pack',
    ]);
  });
}
