import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/voice_client.dart';

import 'support/live_gateway_harness.dart';

void main() {
  group('voice signaling', () {
    test(
      'start, incoming WS, accept, join token',
      () async {
        final probe = await probeLiveGateway();
        expect(
          probe,
          isA<LiveGatewayReady>(),
          reason: probe is LiveGatewayUnavailable ? probe.reason : null,
        );
        final ctx = (probe as LiveGatewayReady).context;

        final sessionA = await ctx.registerUser('call-a');
        final sessionB = await ctx.registerUser('call-b');

        final dm = await ctx.chatsClient().createDm(
          authorization: sessionA.authorizationHeader,
          otherProfileId: sessionB.activeProfileId,
        );
        expect(dm, isA<ChatsApiOk<VoiceChat>>());
        final chatId = (dm as ChatsApiOk<VoiceChat>).data.id;

        final voice = VoiceCallsClient(gateway: ctx.gatewayHttp());
        final realtimeB = await ctx.connectSubscribed(sessionB, chatId);
        addTearDown(realtimeB.dispose);

        final incomingFuture = waitForOp(realtimeB.events, 'call_incoming');
        final startFuture = voice.startCall(
          authorization: sessionA.authorizationHeader,
          chatId: chatId,
          calleeProfileId: sessionB.activeProfileId,
        );

        final start = await startFuture;
        expect(start, isA<VoiceApiOk<VoiceCallSession>>());
        final call = (start as VoiceApiOk<VoiceCallSession>).data;
        expect(call.roomId, isNotEmpty);

        final incoming = await incomingFuture;
        expect(incoming.data?['room_id'], call.roomId);

        final accept = await voice.acceptCall(
          authorization: sessionB.authorizationHeader,
          roomId: call.roomId,
        );
        expect(accept, isA<VoiceApiOk<VoiceCallSession>>());

        final tokenA = await voice.getJoinToken(
          authorization: sessionA.authorizationHeader,
          roomId: call.roomId,
        );
        expect(tokenA, isA<VoiceApiOk<VoiceJoinToken>>());
        expect((tokenA as VoiceApiOk<VoiceJoinToken>).data.jwt, isNotEmpty);

        await voice.endCall(
          authorization: sessionA.authorizationHeader,
          roomId: call.roomId,
        );
      },
      skip: runLiveIntegration
          ? null
          : 'Opt in with --dart-define=VOICE_RUN_LIVE_INTEGRATION=true',
    );

    test(
      'decline emits call_declined on caller WS',
      () async {
        final probe = await probeLiveGateway();
        expect(
          probe,
          isA<LiveGatewayReady>(),
          reason: probe is LiveGatewayUnavailable ? probe.reason : null,
        );
        final ctx = (probe as LiveGatewayReady).context;

        final sessionA = await ctx.registerUser('call-decline-a');
        final sessionB = await ctx.registerUser('call-decline-b');

        final dm = await ctx.chatsClient().createDm(
          authorization: sessionA.authorizationHeader,
          otherProfileId: sessionB.activeProfileId,
        );
        final chatId = (dm as ChatsApiOk<VoiceChat>).data.id;

        final voice = VoiceCallsClient(gateway: ctx.gatewayHttp());
        final realtimeA = await ctx.connectSubscribed(sessionA, chatId);
        addTearDown(realtimeA.dispose);
        final realtimeB = await ctx.connectSubscribed(sessionB, chatId);
        addTearDown(realtimeB.dispose);

        final declinedFuture = waitForOp(realtimeA.events, 'call_declined');
        final start = await voice.startCall(
          authorization: sessionA.authorizationHeader,
          chatId: chatId,
          calleeProfileId: sessionB.activeProfileId,
        );
        final call = (start as VoiceApiOk<VoiceCallSession>).data;

        await waitForOp(realtimeB.events, 'call_incoming');
        await voice.declineCall(
          authorization: sessionB.authorizationHeader,
          roomId: call.roomId,
        );

        final declined = await declinedFuture;
        expect(declined.data?['room_id'], call.roomId);
      },
      skip: runLiveIntegration
          ? null
          : 'Opt in with --dart-define=VOICE_RUN_LIVE_INTEGRATION=true',
    );
  });
}
