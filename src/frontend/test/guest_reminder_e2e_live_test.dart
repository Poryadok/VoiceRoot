import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/auth_client.dart';

import 'support/live_gateway_harness.dart';

/// AU-07: guest reminder server cadence via Gateway.
void main() {
  test(
    'guest reminder: show then mark suppresses same day',
    () async {
      final probe = await probeLiveGateway();
      expect(
        probe,
        isA<LiveGatewayReady>(),
        reason: probe is LiveGatewayUnavailable ? probe.reason : null,
      );
      final ctx = (probe as LiveGatewayReady).context;
      final auth = ctx.authClient();

      final guestResult = await auth.registerGuest(password: qaPassword);
      expect(guestResult, isA<AuthSessionOk>());
      final guest = (guestResult as AuthSessionOk).session;

      final first = await auth.getGuestReminderShouldShow(
        authorization: guest.authorizationHeader,
      );
      expect(first, isTrue);

      await auth.markGuestReminderShown(
        authorization: guest.authorizationHeader,
      );

      final second = await auth.getGuestReminderShouldShow(
        authorization: guest.authorizationHeader,
      );
      expect(second, isFalse);
    },
    skip: runLiveIntegration
        ? null
        : 'Opt in with --dart-define=VOICE_RUN_LIVE_INTEGRATION=true',
  );
}
