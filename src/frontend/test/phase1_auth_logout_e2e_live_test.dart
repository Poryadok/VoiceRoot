import 'package:flutter_test/flutter_test.dart';

import 'support/live_gateway_harness.dart';

void main() {
  test(
    'auth: logout blacklists JWT on protected routes',
    () async {
      final probe = await probeLiveGateway();
      expect(
        probe,
        isA<LiveGatewayReady>(),
        reason: probe is LiveGatewayUnavailable ? probe.reason : null,
      );
      final ctx = (probe as LiveGatewayReady).context;

      final session = await ctx.registerUser('auth-logout');
      expect(await ctx.protectedRouteStatus(session.accessToken), 200);

      await ctx.logoutSession(session);
      expect(await ctx.protectedRouteStatus(session.accessToken), 401);
    },
    skip: runLiveIntegration
        ? null
        : 'Opt in with --dart-define=VOICE_RUN_LIVE_INTEGRATION=true',
  );
}
