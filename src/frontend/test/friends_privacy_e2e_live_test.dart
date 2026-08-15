import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/friends_client.dart';
import 'package:voice_frontend/backend/user_privacy_client.dart';

import 'support/live_gateway_harness.dart';

void main() {
  test(
    'friends: allow_friend_requests nobody denies stranger invite',
    () async {
      final probe = await probeLiveGateway();
      expect(
        probe,
        isA<LiveGatewayReady>(),
        reason: probe is LiveGatewayUnavailable ? probe.reason : null,
      );
      final ctx = (probe as LiveGatewayReady).context;

      final target = await ctx.registerUser('fr-priv-target');
      final stranger = await ctx.registerUser('fr-priv-stranger');

      final privacy = VoiceUserPrivacyClient(gateway: ctx.gatewayHttp());
      final current = await privacy.getPrivacy(
        authorization: target.authorizationHeader,
      );
      expect(current, isA<UserPrivacyApiOk<VoicePrivacySettings>>());
      final update = await privacy.updatePrivacy(
        authorization: target.authorizationHeader,
        settings: (current as UserPrivacyApiOk<VoicePrivacySettings>).data
            .copyWith(allowFriendRequests: VoicePrivacyAudience.nobody),
      );
      expect(update, isA<UserPrivacyApiOk<VoicePrivacySettings>>());

      final friends = ctx.friendsClient();
      final invite = await friends.sendFriendInvitation(
        authorization: stranger.authorizationHeader,
        targetProfileId: target.activeProfileId,
      );
      expect(invite, isA<FriendsApiFailure>());
      expect((invite as FriendsApiFailure).statusCode, 403);
    },
    skip: runLiveIntegration
        ? null
        : 'Opt in with --dart-define=VOICE_RUN_LIVE_INTEGRATION=true',
  );
}
