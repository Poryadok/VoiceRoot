import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/chats_client.dart';

import 'support/live_gateway_harness.dart';

void main() {
  test(
    'typing_start fans out typing event to subscribed peer',
    () async {
      final probe = await probeLiveGateway();
      expect(
        probe,
        isA<LiveGatewayReady>(),
        reason: probe is LiveGatewayUnavailable ? probe.reason : null,
      );
      final ctx = (probe as LiveGatewayReady).context;

      final sessionA = await ctx.registerUser('typing-a');
      final sessionB = await ctx.registerUser('typing-b');

      final dm = await ctx.chatsClient().createDm(
        authorization: sessionA.authorizationHeader,
        otherProfileId: sessionB.activeProfileId,
      );
      final chatId = (dm as ChatsApiOk<VoiceChat>).data.id;

      final wsA = await ctx.connectSubscribed(sessionA, chatId);
      addTearDown(wsA.dispose);
      final wsB = await ctx.connectSubscribed(sessionB, chatId);
      addTearDown(wsB.dispose);

      final typingFuture = waitForOp(
        wsB.events,
        'typing',
        where: (f) =>
            f.data?['chat_id'] == chatId &&
            f.data?['profile_id'] == sessionA.activeProfileId,
      );
      wsA.sendTypingStart(chatId);
      final typing = await typingFuture;
      expect(typing.data?['kind'], 'start');
    },
    skip: runLiveIntegration
        ? null
        : 'Opt in with --dart-define=VOICE_RUN_LIVE_INTEGRATION=true',
  );
}
