import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/deep_links_client.dart';
import 'package:voice_frontend/backend/onboarding_client.dart';
import 'package:voice_frontend/backend/spaces_client.dart';

import 'support/live_gateway_harness.dart';

/// ON-03 coach-marks MM/space + invite deep link (docs/features/onboarding.md, deep-links.md).
///
/// ```text
/// flutter test test/onboarding_coach_e2e_live_test.dart ^
///   --dart-define=VOICE_RUN_LIVE_INTEGRATION=true ^
///   --dart-define=VOICE_API_BASE_URL=http://127.0.0.1:18080
/// ```
void main() {
  test(
    'onboarding coach: spaces and matchmaking steps persist; invite resolve keeps state',
    () async {
      final probe = await probeLiveGateway();
      expect(
        probe,
        isA<LiveGatewayReady>(),
        reason: probe is LiveGatewayUnavailable ? probe.reason : null,
      );
      final ctx = (probe as LiveGatewayReady).context;

      final newbie = await ctx.registerUser('on03-coach');
      final onboarding = VoiceOnboardingClient(gateway: ctx.gatewayHttp());
      final auth = newbie.authorizationHeader;

      Future<OnboardingState> complete(String stepId) async {
        final result = await onboarding.completeStep(
          authorization: auth,
          stepId: stepId,
        );
        expect(result, isA<OnboardingApiOk<OnboardingState>>());
        return (result as OnboardingApiOk<OnboardingState>).data;
      }

      final afterChats = await complete('chats_nav');
      expect(afterChats.completed, isFalse);
      expect(afterChats.completedSteps, contains('chats_nav'));

      final afterSpaces = await complete('spaces');
      expect(afterSpaces.completed, isFalse);
      expect(afterSpaces.completedSteps, containsAll(['chats_nav', 'spaces']));

      // Invite deep link mid-tour must not reset coach-mark progress (П.20 / ON-03).
      final owner = await ctx.registerUser('on03-invite-owner');
      final spaces = ctx.spacesClient();
      final created = await spaces.createSpace(
        authorization: owner.authorizationHeader,
        name: 'ON-03 Coach Invite',
      );
      expect(created, isA<SpacesApiOk<VoiceSpace>>());
      final spaceId = (created as SpacesApiOk<VoiceSpace>).data.id;

      final invite = await spaces.createInvite(
        authorization: owner.authorizationHeader,
        spaceId: spaceId,
      );
      expect(invite, isA<SpacesApiOk<SpaceInvite>>());
      final code = (invite as SpacesApiOk<SpaceInvite>).data.code;

      final links = VoiceDeepLinksClient(gateway: ctx.gatewayHttp());
      final resolved = await links.resolve(
        authorization: auth,
        url: 'https://voice.gg/invite/$code',
      );
      expect(resolved, isA<DeepLinksApiOk<ResolvedDeepLink>>());
      final target = (resolved as DeepLinksApiOk<ResolvedDeepLink>).data;
      expect(target.kind, 'invite');
      expect(target.inviteCode, code);
      expect(target.spaceId, spaceId);

      final midTour = await onboarding.getState(authorization: auth);
      expect(midTour, isA<OnboardingApiOk<OnboardingState>>());
      final mid = (midTour as OnboardingApiOk<OnboardingState>).data;
      expect(mid.completed, isFalse);
      expect(mid.completedSteps, containsAll(['chats_nav', 'spaces']));
      expect(mid.completedSteps, isNot(contains('matchmaking')));

      final afterMm = await complete('matchmaking');
      expect(afterMm.completed, isFalse);
      expect(
        afterMm.completedSteps,
        containsAll(['chats_nav', 'spaces', 'matchmaking']),
      );

      final afterWrap = await complete('wrap_up');
      expect(afterWrap.completed, isTrue);
      expect(
        afterWrap.completedSteps,
        containsAll(['chats_nav', 'spaces', 'matchmaking', 'wrap_up']),
      );

      final refetch = await onboarding.getState(authorization: auth);
      expect(refetch, isA<OnboardingApiOk<OnboardingState>>());
      expect((refetch as OnboardingApiOk<OnboardingState>).data.completed, isTrue);
    },
    skip: runLiveIntegration
        ? null
        : 'Opt in with --dart-define=VOICE_RUN_LIVE_INTEGRATION=true',
  );
}
