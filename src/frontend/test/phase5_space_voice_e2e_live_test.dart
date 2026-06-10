import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/spaces_client.dart';
import 'package:voice_frontend/backend/voice_client.dart';

import 'support/live_gateway_harness.dart';

/// Phase-5 space voice E2E (API-level): join/leave voice room, participant states.
///
/// ```text
/// flutter test test/phase5_space_voice_e2e_live_test.dart ^
///   --dart-define=VOICE_RUN_LIVE_INTEGRATION=true ^
///   --dart-define=VOICE_API_BASE_URL=http://127.0.0.1:18080
/// ```
void main() {
  test(
    'space voice room: join, states, leave',
    () async {
      final probe = await probeLiveGateway();
      expect(
        probe,
        isA<LiveGatewayReady>(),
        reason: probe is LiveGatewayUnavailable ? probe.reason : null,
      );
      final ctx = (probe as LiveGatewayReady).context;

      final owner = await ctx.registerUser('space-voice-owner');
      final member = await ctx.registerUser('space-voice-member');
      final spaces = ctx.spacesClient();
      final voice = VoiceCallsClient(gateway: ctx.gatewayHttp());

      final created = await spaces.createSpace(
        authorization: owner.authorizationHeader,
        name: 'Voice E2E',
      );
      expect(created, isA<SpacesApiOk<VoiceSpace>>());
      final spaceId = (created as SpacesApiOk<VoiceSpace>).data.id;

      final vr = await spaces.createVoiceRoom(
        authorization: owner.authorizationHeader,
        spaceId: spaceId,
        name: 'Lobby',
      );
      expect(vr, isA<SpacesApiOk<VoiceRoomData>>());
      final voiceRoomId = (vr as SpacesApiOk<VoiceRoomData>).data.id;

      final invite = await spaces.createInvite(
        authorization: owner.authorizationHeader,
        spaceId: spaceId,
      );
      expect(invite, isA<SpacesApiOk<SpaceInvite>>());
      await spaces.joinByInvite(
        authorization: member.authorizationHeader,
        code: (invite as SpacesApiOk<SpaceInvite>).data.code,
      );

      final ownerJoin = await voice.joinVoiceRoom(
        authorization: owner.authorizationHeader,
        voiceRoomId: voiceRoomId,
        spaceId: spaceId,
      );
      expect(ownerJoin, isA<VoiceApiOk<VoiceRoomSession>>());
      final session = (ownerJoin as VoiceApiOk<VoiceRoomSession>).data;
      expect(session.voiceRoomId, voiceRoomId);
      expect(session.roomId, isNotEmpty);
      expect(session.livekitRoomName, isNotEmpty);

      final statesAfterOwner = await voice.getVoiceRoomStates(
        authorization: owner.authorizationHeader,
        voiceRoomId: voiceRoomId,
      );
      expect(statesAfterOwner, isA<VoiceApiOk<List<VoiceRoomParticipantState>>>());
      final ownerStates =
          (statesAfterOwner as VoiceApiOk<List<VoiceRoomParticipantState>>).data;
      expect(ownerStates, hasLength(1));
      expect(ownerStates.first.profileId, owner.activeProfileId);

      final memberJoin = await voice.joinVoiceRoom(
        authorization: member.authorizationHeader,
        voiceRoomId: voiceRoomId,
        spaceId: spaceId,
      );
      expect(memberJoin, isA<VoiceApiOk<VoiceRoomSession>>());

      final statesAfterBoth = await voice.getVoiceRoomStates(
        authorization: owner.authorizationHeader,
        voiceRoomId: voiceRoomId,
      );
      expect(statesAfterBoth, isA<VoiceApiOk<List<VoiceRoomParticipantState>>>());
      final bothStates =
          (statesAfterBoth as VoiceApiOk<List<VoiceRoomParticipantState>>).data;
      expect(bothStates, hasLength(2));
      expect(
        bothStates.map((s) => s.profileId).toSet(),
        {owner.activeProfileId, member.activeProfileId},
      );

      final memberLeave = await voice.leaveVoiceRoom(
        authorization: member.authorizationHeader,
        voiceRoomId: voiceRoomId,
      );
      expect(memberLeave, isA<VoiceApiOk<void>>());

      final statesAfterMemberLeave = await voice.getVoiceRoomStates(
        authorization: owner.authorizationHeader,
        voiceRoomId: voiceRoomId,
      );
      expect(
        statesAfterMemberLeave,
        isA<VoiceApiOk<List<VoiceRoomParticipantState>>>(),
      );
      final afterMemberLeave =
          (statesAfterMemberLeave as VoiceApiOk<List<VoiceRoomParticipantState>>)
              .data;
      expect(afterMemberLeave, hasLength(1));
      expect(afterMemberLeave.first.profileId, owner.activeProfileId);

      final ownerLeave = await voice.leaveVoiceRoom(
        authorization: owner.authorizationHeader,
        voiceRoomId: voiceRoomId,
      );
      expect(ownerLeave, isA<VoiceApiOk<void>>());

      final statesAfterAllLeave = await voice.getVoiceRoomStates(
        authorization: owner.authorizationHeader,
        voiceRoomId: voiceRoomId,
      );
      expect(
        statesAfterAllLeave,
        isA<VoiceApiOk<List<VoiceRoomParticipantState>>>(),
      );
      expect(
        (statesAfterAllLeave as VoiceApiOk<List<VoiceRoomParticipantState>>)
            .data,
        isEmpty,
      );
    },
    skip: runLiveIntegration
        ? null
        : 'Opt in with --dart-define=VOICE_RUN_LIVE_INTEGRATION=true',
  );
}
