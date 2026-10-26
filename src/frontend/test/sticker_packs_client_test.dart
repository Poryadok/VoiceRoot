import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/gateway_config.dart';

import 'support/gateway_test_client.dart';

void main() {
  test(
    'installed catalog preserves real stickers and premium metadata',
    () async {
      final client = VoiceStickerPacksClient(
        gateway: gatewayHttpForTest(
          MockClient(
            (_) async => http.Response(
              jsonEncode({
                'packs': [
                  {
                    'id': 'alpha',
                    'title': 'Alpha',
                    'is_system': true,
                    'stickers': [
                      {'id': 'a1', 'file_id': 'file-a', 'emoji': '👋'},
                    ],
                  },
                  {
                    'id': 'premium',
                    'title': 'Premium pack',
                    'is_premium': true,
                  },
                ],
              }),
              200,
              headers: {'content-type': 'application/json; charset=utf-8'},
            ),
          ),
          config: const GatewayConfig(baseUrl: 'http://api.test'),
        ),
      );
      final result = await client.listInstalled(authorization: 'Bearer a');
      expect(
        result,
        isA<ChatsApiOk<StickerPackListData>>(),
        reason: result is ChatsApiFailure ? result.message : null,
      );
      final packs = (result as ChatsApiOk<StickerPackListData>).data.packs;
      expect(packs.first.stickers.single.id, 'a1');
      expect(packs.first.stickers.single.emoji, '👋');
      expect(packs.last.isPremium, isTrue);
    },
  );
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
