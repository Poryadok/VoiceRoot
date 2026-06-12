import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/notifications_client.dart';

import 'support/live_gateway_harness.dart';

/// Phase-8 FCM Android E2E: device token registration via Gateway (compose stack).
///
/// ```text
/// flutter test test/phase8_fcm_android_e2e_live_test.dart ^
///   --dart-define=VOICE_RUN_LIVE_INTEGRATION=true ^
///   --dart-define=VOICE_API_BASE_URL=http://127.0.0.1:18080
/// ```
void main() {
  test(
    'register-device accepts FCM token for android platform',
    () async {
      final probe = await probeLiveGateway();
      expect(
        probe,
        isA<LiveGatewayReady>(),
        reason: probe is LiveGatewayUnavailable ? probe.reason : null,
      );
      final ctx = (probe as LiveGatewayReady).context;

      final user = await ctx.registerUser('fcm-phase8-android');
      final notifications = VoiceNotificationsClient(gateway: ctx.gatewayHttp());

      final result = await notifications.registerDevice(
        authorization: user.authorizationHeader,
        platform: 'android',
        token: 'qa-fcm-android-${user.activeProfileId}-${DateTime.now().millisecondsSinceEpoch}',
        pushService: 'fcm',
      );
      expect(result, isA<NotificationsApiOk<String?>>());
    },
    skip: runLiveIntegration
        ? null
        : 'Opt in with --dart-define=VOICE_RUN_LIVE_INTEGRATION=true',
  );
}
