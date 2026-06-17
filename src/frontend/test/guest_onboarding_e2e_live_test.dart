import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/auth_client.dart';
import 'package:voice_frontend/backend/jwt_claims.dart';
import 'package:voice_frontend/backend/onboarding_client.dart';

import 'support/live_gateway_harness.dart';

/// Live guest onboarding: JWT account_type and server onboarding state after register.
void main() {
  test('guest onboarding live: account_type guest and onboarding incomplete', () async {
    final probe = await probeLiveGateway();
    expect(
      probe,
      isA<LiveGatewayReady>(),
      reason: probe is LiveGatewayUnavailable ? probe.reason : null,
    );
    final ctx = (probe as LiveGatewayReady).context;

    final guestResult = await ctx.authClient().registerGuest(password: qaPassword);
    expect(guestResult, isA<AuthSessionOk>());
    final session = (guestResult as AuthSessionOk).session;

    expect(session.accountType, 'guest');
    expect(isGuestAccountType(accountTypeFromAccessToken(session.accessToken)), isTrue);

    final onboarding = VoiceOnboardingClient(gateway: ctx.gatewayHttp());
    final state = await onboarding.getState(authorization: session.authorizationHeader);
    expect(state, isA<OnboardingApiOk<OnboardingState>>());
    final data = (state as OnboardingApiOk<OnboardingState>).data;
    expect(data.completed, isFalse);
    expect(data.completedSteps, isEmpty);
  }, skip: runLiveIntegration
      ? null
      : 'Opt in with --dart-define=VOICE_RUN_LIVE_INTEGRATION=true');
}
