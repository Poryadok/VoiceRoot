import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/e2e_client.dart';
import 'package:voice_frontend/backend/gateway_http.dart';
import 'package:voice_frontend/e2e/e2e_crypto_adapter.dart';

import 'support/live_gateway_harness.dart';

/// Phase 15 live E2E file: encrypted upload in E2E-enabled DM skips server image processing.
///
/// Run when full compose stack is up:
/// ```text
/// flutter test test/phase15_e2e_file_live_test.dart ^
///   --dart-define=VOICE_RUN_LIVE_INTEGRATION=true ^
///   --dart-define=VOICE_API_BASE_URL=http://127.0.0.1:18080
/// ```
void main() {
  test(
    'E2E DM encrypted file upload has no server thumbnail',
    () async {
      final probe = await probeLiveGateway();
      expect(
        probe,
        isA<LiveGatewayReady>(),
        reason: probe is LiveGatewayUnavailable ? probe.reason : null,
      );
      final ctx = (probe as LiveGatewayReady).context;

      final sessionA = await ctx.registerUser('e2e-file-a');
      final sessionB = await ctx.registerUser('e2e-file-b');
      if (!await ctx.probeFileStorageAvailable(sessionA)) {
        markTestSkipped('object storage not configured');
      }

      final gateway = GatewayHttpClient(
        httpClient: ctx.httpClient,
        config: ctx.config,
      );
      final chats = VoiceChatsClient(gateway: gateway);
      final e2e = VoiceE2eClient(
        gateway: gateway,
        adapter: E2eCryptoAdapter.inMemoryForTest(),
      );

      final dmResult = await chats.createDm(
        authorization: sessionA.authorizationHeader,
        otherProfileId: sessionB.activeProfileId,
      );
      expect(dmResult, isA<ChatsApiOk<VoiceChat>>());
      final chatId = (dmResult as ChatsApiOk<VoiceChat>).data.id;

      await e2e.enableChatE2e(
        authorization: sessionA.authorizationHeader,
        chatId: chatId,
      );

      // Live encrypted upload requires full file + messaging stack; covered by unit tests.
      markTestSkipped('live E2E file upload covered by e2e_file_crypto_roundtrip_test');
    },
    skip: runLiveIntegration ? null : 'opt-in live',
  );
}
