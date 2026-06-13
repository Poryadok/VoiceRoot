import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/voice_client.dart';

import 'support/live_gateway_harness.dart';

/// Phase 10 screen share signaling E2E (REST + optional WS).
void main() {
  test(
    'active DM call: start and stop screen share',
    () async {
      final probe = await probeLiveGateway();
      expect(
        probe,
        isA<LiveGatewayReady>(),
        reason: probe is LiveGatewayUnavailable ? probe.reason : null,
      );
      final ctx = (probe as LiveGatewayReady).context;

      final caller = await ctx.registerUser('ss-caller');
      final callee = await ctx.registerUser('ss-callee');

      final dm = await ctx.chatsClient().createDm(
        authorization: caller.authorizationHeader,
        otherProfileId: callee.activeProfileId,
      );
      expect(dm, isA<ChatsApiOk<VoiceChat>>());
      final chatId = (dm as ChatsApiOk<VoiceChat>).data.id;

      final voice = VoiceCallsClient(gateway: ctx.gatewayHttp());
      final realtimeCallee = await ctx.connectSubscribed(callee, chatId);
      addTearDown(realtimeCallee.dispose);

      final started = await voice.startCall(
        authorization: caller.authorizationHeader,
        chatId: chatId,
        calleeProfileId: callee.activeProfileId,
      );
      expect(started, isA<VoiceApiOk<VoiceCallSession>>());
      final roomId = (started as VoiceApiOk<VoiceCallSession>).data.roomId;

      final accepted = await voice.acceptCall(
        authorization: callee.authorizationHeader,
        roomId: roomId,
      );
      expect(accepted, isA<VoiceApiOk<VoiceCallSession>>());

      final shareStartedFuture = waitForOp(
        realtimeCallee.events,
        'screen_share_started',
      );
      final shareStarted = await voice.startScreenShare(
        authorization: caller.authorizationHeader,
        roomId: roomId,
      );
      expect(shareStarted, isA<VoiceApiOk<String>>());
      final streamId = (shareStarted as VoiceApiOk<String>).data;
      expect(streamId, isNotEmpty);

      final wsStarted = await shareStartedFuture;
      expect(wsStarted.data?['stream_id'], streamId);

      final shareStopped = await voice.stopScreenShare(
        authorization: caller.authorizationHeader,
        roomId: roomId,
        streamId: streamId,
      );
      expect(shareStopped, isA<VoiceApiOk<void>>());

      await voice.endCall(
        authorization: caller.authorizationHeader,
        roomId: roomId,
      );
    },
    skip: runLiveIntegration
        ? null
        : 'Opt in with --dart-define=VOICE_RUN_LIVE_INTEGRATION=true',
  );
}
