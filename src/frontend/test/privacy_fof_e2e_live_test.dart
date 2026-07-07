import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/user_privacy_client.dart';

import 'package:voice_frontend/ui/settings/privacy_presets.dart';

import 'support/live_gateway_harness.dart';

void main() {
  test(
    'phase 11 privacy fof: stranger denied, friend-of-friend allowed',
    () async {
      final probe = await probeLiveGateway();
      expect(
        probe,
        isA<LiveGatewayReady>(),
        reason: probe is LiveGatewayUnavailable ? probe.reason : null,
      );
      final ctx = (probe as LiveGatewayReady).context;

      final userA = await ctx.registerUser('p11fof-a');
      final userC = await ctx.registerUser('p11fof-c');
      final userB = await ctx.registerUser('p11fof-b');

      await ctx.inviteAndAcceptFriends(userA, userC);
      await ctx.inviteAndAcceptFriends(userC, userB);

      final privacy = VoiceUserPrivacyClient(gateway: ctx.gatewayHttp());
      final privacyUpdate = await privacy.updatePrivacy(
        authorization: userB.authorizationHeader,
        settings: PrivacyPresetDefaults.forPreset(
          'personal',
          profileId: userB.activeProfileId,
        ).copyWith(allowDm: VoicePrivacyAudience.friendsAndFoF),
      );
      expect(privacyUpdate, isA<UserPrivacyApiOk<VoicePrivacySettings>>());

      final stranger = await ctx.registerUser('p11fof-stranger');
      final chats = ctx.chatsClient();
      final strangerAttempt = await chats.createDm(
        authorization: stranger.authorizationHeader,
        otherProfileId: userB.activeProfileId,
      );
      expect(strangerAttempt, isA<ChatsApiFailure>());

      final fofAttempt = await chats.createDm(
        authorization: userA.authorizationHeader,
        otherProfileId: userB.activeProfileId,
      );
      expect(fofAttempt, isA<ChatsApiOk>());
    },
    skip: 'opt-in live: set VOICE_RUN_LIVE_COMPOSE=true',
  );
}
