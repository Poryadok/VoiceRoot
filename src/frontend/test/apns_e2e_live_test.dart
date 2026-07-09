import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/notifications_client.dart';

import 'support/live_gateway_harness.dart';

/// platforms (docs/features/platforms.md) APNs E2E: iOS-style device token registration via Gateway (compose stack).
///
/// ```text
/// flutter test test/apns_e2e_live_test.dart ^
///   --dart-define=VOICE_RUN_LIVE_INTEGRATION=true ^
///   --dart-define=VOICE_API_BASE_URL=http://127.0.0.1:18080
/// ```
void main() {
  test(
    'register-device accepts APNs token for authenticated user',
    () async {
      final probe = await probeLiveGateway();
      expect(
        probe,
        isA<LiveGatewayReady>(),
        reason: probe is LiveGatewayUnavailable ? probe.reason : null,
      );
      final ctx = (probe as LiveGatewayReady).context;

      final user = await ctx.registerUser('apns-phase8');
      final notifications = VoiceNotificationsClient(gateway: ctx.gatewayHttp());

      final result = await notifications.registerDevice(
        authorization: user.authorizationHeader,
        platform: 'ios',
        token: 'qa-apns-${user.activeProfileId}-${DateTime.now().millisecondsSinceEpoch}',
        pushService: 'apns',
      );
      expect(result, isA<NotificationsApiOk<void>>());
    },
    skip: runLiveIntegration
        ? null
        : 'Opt in with --dart-define=VOICE_RUN_LIVE_INTEGRATION=true',
  );
}
