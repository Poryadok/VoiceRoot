import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/gateway_http.dart';
import 'package:voice_frontend/backend/voice_client.dart';

import 'support/live_gateway_harness.dart';

/// Group voice API E2E: leaving removes one participant, preserving the session.
///
/// ```text
/// flutter test test/group_voice_e2e_live_test.dart ^
///   --dart-define=VOICE_RUN_LIVE_INTEGRATION=true ^
///   --dart-define=VOICE_API_BASE_URL=http://127.0.0.1:18080
/// ```
void main() {
  test(
    'group voice: member leaves and rejoins without ending others call',
    () async {
      final probe = await probeLiveGateway();
      expect(
        probe,
        isA<LiveGatewayReady>(),
        reason: probe is LiveGatewayUnavailable ? probe.reason : null,
      );
      final ctx = (probe as LiveGatewayReady).context;
      addTearDown(ctx.httpClient.close);

      final owner = await ctx.registerUser('gv-owner');
      final member = await ctx.registerUser('gv-member');
      final filler = await ctx.registerUser('gv-filler');

      final chats = ctx.chatsClient();
      final created = await chats.createGroup(
        authorization: owner.authorizationHeader,
        name: 'Voice squad',
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
      final session = (start as VoiceApiOk<VoiceCallSession>).data;
      expect(session.roomId, isNotEmpty);
      expect(session.status, VoiceCallStatus.active);
      expect(session.chatId, group.id);
      addTearDown(() async {
        final end = await voice.endCall(
          authorization: owner.authorizationHeader,
          roomId: session.roomId,
        );
        expect(end, isA<VoiceApiOk<void>>());
      });

      final join = await voice.joinCall(
        authorization: member.authorizationHeader,
        roomId: session.roomId,
      );
      expect(join, isA<VoiceApiOk<VoiceCallSession>>());
      expect(
        (join as VoiceApiOk<VoiceCallSession>).data.status,
        VoiceCallStatus.active,
      );

      final joinFiller = await voice.joinCall(
        authorization: filler.authorizationHeader,
        roomId: session.roomId,
      );
      expect(joinFiller, isA<VoiceApiOk<VoiceCallSession>>());

      Future<void> expectRoster(Set<String> expected) async {
        final states = await voice.getCallVoiceStates(
          authorization: owner.authorizationHeader,
          roomId: session.roomId,
        );
        expect(states, isA<VoiceApiOk<List<VoiceRoomParticipantState>>>());
        expect(
          (states as VoiceApiOk<List<VoiceRoomParticipantState>>).data.map(
            (state) => state.profileId,
          ),
          unorderedEquals(expected),
        );
      }

      await expectRoster({
        owner.activeProfileId,
        member.activeProfileId,
        filler.activeProfileId,
      });

      final tokenOwner = await voice.getJoinToken(
        authorization: owner.authorizationHeader,
        roomId: session.roomId,
      );
      expect(tokenOwner, isA<VoiceApiOk<VoiceJoinToken>>());
      expect((tokenOwner as VoiceApiOk<VoiceJoinToken>).data.jwt, isNotEmpty);

      final tokenMember = await voice.getJoinToken(
        authorization: member.authorizationHeader,
        roomId: session.roomId,
      );
      expect(tokenMember, isA<VoiceApiOk<VoiceJoinToken>>());
      expect((tokenMember as VoiceApiOk<VoiceJoinToken>).data.jwt, isNotEmpty);

      final gateway = ctx.gatewayHttp();
      final leave = await gateway.postEmpty(
        uri: gateway.resolve('/api/v1/voice/calls/${session.roomId}/leave'),
        authorization: member.authorizationHeader,
      );
      expect(leave, isA<GatewayHttpOk<void>>());
      await expectRoster({owner.activeProfileId, filler.activeProfileId});

      final active = await voice.getActiveCall(
        authorization: filler.authorizationHeader,
      );
      expect(active, isA<VoiceApiOk<VoiceCallSession?>>());
      final remainingSession = (active as VoiceApiOk<VoiceCallSession?>).data;
      expect(remainingSession?.roomId, session.roomId);
      expect(remainingSession?.status, VoiceCallStatus.active);

      final remainingToken = await voice.getJoinToken(
        authorization: owner.authorizationHeader,
        roomId: session.roomId,
      );
      expect(remainingToken, isA<VoiceApiOk<VoiceJoinToken>>());
      expect(
        (remainingToken as VoiceApiOk<VoiceJoinToken>).data.jwt,
        isNotEmpty,
      );

      final leaverActive = await voice.getActiveCall(
        authorization: member.authorizationHeader,
      );
      expect(leaverActive, isA<VoiceApiOk<VoiceCallSession?>>());
      expect((leaverActive as VoiceApiOk<VoiceCallSession?>).data, isNull);
      final leaverToken = await voice.getJoinToken(
        authorization: member.authorizationHeader,
        roomId: session.roomId,
      );
      expect(leaverToken, isA<VoiceApiFailure>());
      expect((leaverToken as VoiceApiFailure).statusCode, 403);

      final rejoin = await voice.joinCall(
        authorization: member.authorizationHeader,
        roomId: session.roomId,
      );
      expect(rejoin, isA<VoiceApiOk<VoiceCallSession>>());
      expect(
        (rejoin as VoiceApiOk<VoiceCallSession>).data.roomId,
        session.roomId,
      );
      await expectRoster({
        owner.activeProfileId,
        member.activeProfileId,
        filler.activeProfileId,
      });
      final rejoinedToken = await voice.getJoinToken(
        authorization: member.authorizationHeader,
        roomId: session.roomId,
      );
      expect(rejoinedToken, isA<VoiceApiOk<VoiceJoinToken>>());
      expect(
        (rejoinedToken as VoiceApiOk<VoiceJoinToken>).data.jwt,
        isNotEmpty,
      );
    },
    skip: runLiveIntegration
        ? null
        : 'Opt in with --dart-define=VOICE_RUN_LIVE_INTEGRATION=true',
  );
}
