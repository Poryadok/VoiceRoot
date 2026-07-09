import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/e2e_client.dart';
import 'package:voice_frontend/backend/gateway_http.dart';
import 'package:voice_frontend/e2e/e2e_crypto_adapter.dart';

import 'support/in_memory_secure_signal_storage.dart';
import 'support/live_gateway_harness.dart';

/// encryption (docs/features/encryption.md) live E2E key backup: PUT/GET roundtrip via [VoiceE2eClient].
///
/// Run when full compose stack is up:
/// ```text
/// flutter test test/e2e_key_backup_live_test.dart ^
///   --dart-define=VOICE_RUN_LIVE_INTEGRATION=true ^
///   --dart-define=VOICE_API_BASE_URL=http://127.0.0.1:18080
/// ```
void main() {
  test(
    'PUT then GET e2e key backup roundtrip',
    () async {
      final probe = await probeLiveGateway();
      expect(
        probe,
        isA<LiveGatewayReady>(),
        reason: probe is LiveGatewayUnavailable ? probe.reason : null,
      );
      final ctx = (probe as LiveGatewayReady).context;

      final session = await ctx.registerUser('e2e-key-backup');

      final gateway = GatewayHttpClient(
        httpClient: ctx.httpClient,
        config: ctx.config,
      );
      final e2e = VoiceE2eClient(
        gateway: gateway,
        adapter: E2eCryptoAdapter.inMemoryForTest(),
        backupStorage: InMemorySecureSignalStorage(),
      );

      const password = 'VoiceQaBackup1!';
      const hint = 'qa hint';

      expect(
        await e2e.putKeyBackup(
          authorization: session.authorizationHeader,
          password: password,
          passwordHint: hint,
        ),
        isA<E2eApiOk<void>>(),
      );

      final got = await e2e.getKeyBackup(
        authorization: session.authorizationHeader,
      );
      expect(got, isA<E2eApiOk<E2eKeyBackupData>>());
      final data = (got as E2eApiOk<E2eKeyBackupData>).data;
      expect(data.encryptedBlob, isNotEmpty);
      expect(data.passwordHint, equals(hint));
    },
    skip: runLiveIntegration
        ? null
        : 'Opt in with --dart-define=VOICE_RUN_LIVE_INTEGRATION=true',
    timeout: const Timeout(Duration(minutes: 2)),
  );
}
