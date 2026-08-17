import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/notification_settings_models.dart';
import 'package:voice_frontend/backend/notifications_client.dart';

import 'support/live_gateway_harness.dart';

/// NT-04: Quiet hours Set/Get round-trip via Notification client.
void main() {
  test(
    'quiet hours set/get round-trip (NT-04)',
    () async {
      final probe = await probeLiveGateway();
      expect(
        probe,
        isA<LiveGatewayReady>(),
        reason: probe is LiveGatewayUnavailable ? probe.reason : null,
      );
      final ctx = (probe as LiveGatewayReady).context;
      final user = await ctx.registerUser('nt04-qh');

      final notifications = VoiceNotificationsClient(gateway: ctx.gatewayHttp());
      const want = VoiceQuietHours(
        enabled: true,
        startTime: '22:00',
        endTime: '08:00',
        timezone: 'UTC',
        overrideMentions: false,
      );
      final set = await notifications.setQuietHours(
        authorization: user.authorizationHeader,
        quietHours: want,
      );
      expect(set, isA<NotificationsApiOk<void>>());

      final got = await notifications.getQuietHours(
        authorization: user.authorizationHeader,
      );
      expect(got, isA<NotificationsApiOk<VoiceQuietHours>>());
      final qh = (got as NotificationsApiOk<VoiceQuietHours>).data;
      expect(qh.enabled, isTrue);
      expect(qh.startTime, '22:00');
      expect(qh.endTime, '08:00');
      expect(qh.timezone, 'UTC');
      expect(qh.overrideMentions, isFalse);
    },
    skip: runLiveIntegration
        ? null
        : 'Opt in with --dart-define=VOICE_RUN_LIVE_INTEGRATION=true',
  );
}
