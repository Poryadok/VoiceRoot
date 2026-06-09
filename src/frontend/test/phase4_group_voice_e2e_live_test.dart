import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/voice_client.dart';

import 'support/live_gateway_harness.dart';

/// Phase-4 group voice E2E (API-level): start active call, member join, join tokens.
///
/// ```text
/// flutter test test/phase4_group_voice_e2e_live_test.dart ^
///   --dart-define=VOICE_RUN_LIVE_INTEGRATION=true ^
///   --dart-define=VOICE_API_BASE_URL=http://127.0.0.1:18080
/// ```
void main() {
  test(
    'group voice: start active call, member joins, both get join tokens',
    () async {
      final probe = await probeLiveGateway();
      expect(
        probe,
        isA<LiveGatewayReady>(),
        reason: probe is LiveGatewayUnavailable ? probe.reason : null,
      );
      final ctx = (probe as LiveGatewayReady).context;

      final owner = await ctx.registerUser('gv-owner');
      final member = await ctx.registerUser('gv-member');

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
        profileIds: [member.activeProfileId],
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

      final join = await voice.joinCall(
        authorization: member.authorizationHeader,
        roomId: session.roomId,
      );
      expect(join, isA<VoiceApiOk<VoiceCallSession>>());
      expect((join as VoiceApiOk<VoiceCallSession>).data.status,
          VoiceCallStatus.active);

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

      final end = await voice.endCall(
        authorization: owner.authorizationHeader,
        roomId: session.roomId,
      );
      expect(end, isA<VoiceApiOk<void>>());
    },
    skip: runLiveIntegration
        ? null
        : 'Opt in with --dart-define=VOICE_RUN_LIVE_INTEGRATION=true',
  );
}
