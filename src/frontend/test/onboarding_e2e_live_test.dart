import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/onboarding_client.dart';

import 'support/live_gateway_harness.dart';

/// deep-links/platforms (docs/features/deep-links.md) onboarding E2E (API-level): backend state persists across fetch.
void main() {
  test('deep-links onboarding: steps persist on server', () async {
    final probe = await probeLiveGateway();
    expect(
      probe,
      isA<LiveGatewayReady>(),
      reason: probe is LiveGatewayUnavailable ? probe.reason : null,
    );
    final ctx = (probe as LiveGatewayReady).context;

    final user = await ctx.registerUser('p18-onboarding');
    final onboarding = VoiceOnboardingClient(gateway: ctx.gatewayHttp());

    final initial = await onboarding.getState(authorization: user.authorizationHeader);
    expect(initial, isA<OnboardingApiOk<OnboardingState>>());
    expect((initial as OnboardingApiOk<OnboardingState>).data.completed, isFalse);

    final afterStep = await onboarding.completeStep(
      authorization: user.authorizationHeader,
      stepId: 'save_account',
    );
    expect(afterStep, isA<OnboardingApiOk<OnboardingState>>());
    expect(
      (afterStep as OnboardingApiOk<OnboardingState>).data.completedSteps,
      contains('save_account'),
    );

    final refetch = await onboarding.getState(authorization: user.authorizationHeader);
    expect(refetch, isA<OnboardingApiOk<OnboardingState>>());
    expect(
      (refetch as OnboardingApiOk<OnboardingState>).data.completedSteps,
      contains('save_account'),
    );

    final dismissed = await onboarding.completeStep(
      authorization: user.authorizationHeader,
      stepId: 'dismiss',
    );
    expect(dismissed, isA<OnboardingApiOk<OnboardingState>>());
    expect((dismissed as OnboardingApiOk<OnboardingState>).data.completed, isTrue);
  }, skip: runLiveIntegration
      ? null
      : 'Opt in with --dart-define=VOICE_RUN_LIVE_INTEGRATION=true');
}
