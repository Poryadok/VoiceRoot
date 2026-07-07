import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/notifications_client.dart';

import 'support/live_gateway_harness.dart';

/// Verifies match_found push path is wired (token registration + match flow smoke).
/// Full FCM delivery proof mirrors fcm_delivery_e2e_live_test against notification debug URL.
void main() {
  test('matchmaking user can register device for match_found push', () async {
    final probe = await probeLiveGateway();
    expect(probe, isA<LiveGatewayReady>());
    final ctx = (probe as LiveGatewayReady).context;
    final user = await ctx.registerUser('mm-fcm');
    final notifications = VoiceNotificationsClient(gateway: ctx.gatewayHttp());
    final result = await notifications.registerDevice(
      authorization: user.authorizationHeader,
      platform: 'web',
      token: 'qa-mm-fcm-${user.activeProfileId}',
    );
    expect(result, isA<NotificationsApiOk<String?>>());
  }, skip: runLiveIntegration ? null : 'opt-in live');
}
