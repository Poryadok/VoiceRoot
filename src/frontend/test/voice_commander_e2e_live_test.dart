import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/voice_client.dart';

import 'support/live_gateway_harness.dart';

/// VC-07: commander mode + raise hand + GrantFloor on group voice.
void main() {
  test(
    'group voice commander grant floor (VC-07)',
    () async {
      final probe = await probeLiveGateway();
      expect(
        probe,
        isA<LiveGatewayReady>(),
        reason: probe is LiveGatewayUnavailable ? probe.reason : null,
      );
      final ctx = (probe as LiveGatewayReady).context;

      final owner = await ctx.registerUser('vc07-owner');
      final member = await ctx.registerUser('vc07-member');
      final filler = await ctx.registerUser('vc07-filler');

      final chats = ctx.chatsClient();
      final created = await chats.createGroup(
        authorization: owner.authorizationHeader,
        name: 'Commander Flutter',
      );
      expect(created, isA<ChatsApiOk<VoiceChat>>());
      final group = (created as ChatsApiOk<VoiceChat>).data;

      final invite = await chats.addGroupMembers(
        authorization: owner.authorizationHeader,
        chatId: group.id,
        profileIds: [member.activeProfileId, filler.activeProfileId],
      );
      expect(invite, isA<ChatsApiOk<void>>());

      final voice = VoiceCallsClient(gateway: ctx.gatewayHttp());
      final start = await voice.startGroupVoice(
        authorization: owner.authorizationHeader,
        groupChatId: group.id,
      );
      expect(start, isA<VoiceApiOk<VoiceCallSession>>());
      final roomId = (start as VoiceApiOk<VoiceCallSession>).data.roomId;

      final join = await voice.joinCall(
        authorization: member.authorizationHeader,
        roomId: roomId,
      );
      expect(join, isA<VoiceApiOk<VoiceCallSession>>());

      expect(
        await voice.raiseHand(
          authorization: member.authorizationHeader,
          roomId: roomId,
        ),
        isA<VoiceApiOk<void>>(),
      );
      expect(
        await voice.setCommanderMode(
          authorization: owner.authorizationHeader,
          roomId: roomId,
          enabled: true,
        ),
        isA<VoiceApiOk<void>>(),
      );
      expect(
        await voice.grantFloor(
          authorization: owner.authorizationHeader,
          roomId: roomId,
          profileId: member.activeProfileId,
        ),
        isA<VoiceApiOk<void>>(),
      );

      final states = await voice.getCallVoiceStates(
        authorization: owner.authorizationHeader,
        roomId: roomId,
      );
      expect(states, isA<VoiceApiOk<List<VoiceRoomParticipantState>>>());
      final participants =
          (states as VoiceApiOk<List<VoiceRoomParticipantState>>).data;
      final memberState = participants.firstWhere(
        (p) => p.profileId == member.activeProfileId,
      );
      expect(memberState.hasFloor, isTrue);
      expect(memberState.handRaised, isFalse);

      final ownerState = participants.firstWhere(
        (p) => p.profileId == owner.activeProfileId,
      );
      expect(ownerState.isCommander, isTrue);

      expect(
        await voice.setBroadcasting(
          authorization: owner.authorizationHeader,
          roomId: roomId,
          enabled: true,
        ),
        isA<VoiceApiOk<void>>(),
      );
      expect(
        await voice.revokeFloor(
          authorization: owner.authorizationHeader,
          roomId: roomId,
          profileId: member.activeProfileId,
        ),
        isA<VoiceApiOk<void>>(),
      );

      await voice.endCall(
        authorization: owner.authorizationHeader,
        roomId: roomId,
      );
    },
    skip: runLiveIntegration
        ? null
        : 'Opt in with --dart-define=VOICE_RUN_LIVE_INTEGRATION=true',
  );
}
