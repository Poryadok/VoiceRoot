import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/messages_client.dart';

import 'support/live_gateway_harness.dart';

void main() {
  test(
    'ws resume: missed messages via REST after reconnect',
    () async {
      final probe = await probeLiveGateway();
      expect(
        probe,
        isA<LiveGatewayReady>(),
        reason: probe is LiveGatewayUnavailable ? probe.reason : null,
      );
      final ctx = (probe as LiveGatewayReady).context;

      final sessionA = await ctx.registerUser('resume-a');
      final sessionB = await ctx.registerUser('resume-b');

      final chats = ctx.chatsClient();
      final dm = await chats.createDm(
        authorization: sessionA.authorizationHeader,
        otherProfileId: sessionB.activeProfileId,
      );
      expect(dm, isA<ChatsApiOk<VoiceChat>>());
      final chatId = (dm as ChatsApiOk<VoiceChat>).data.id;

      var realtimeB = await ctx.connectSubscribed(sessionB, chatId);
      addTearDown(realtimeB.dispose);

      const baseline = 'resume-baseline';
      final messages = ctx.messagesClient();
      final baselineFuture = waitForOp(
        realtimeB.events,
        'message_create',
        where: (f) => f.data?['chat_id'] == chatId,
      );
      final baselineSend = await messages.sendMessage(
        authorization: sessionA.authorizationHeader,
        chatId: chatId,
        content: baseline,
        clientMessageId: qaClientMessageId(),
      );
      expect(baselineSend, isA<MessagesApiOk<VoiceMessage>>());
      await baselineFuture;

      final lastS = realtimeB.lastSequence;
      expect(lastS, isNotNull);

      await realtimeB.dispose();

      const missed = 'resume-missed-while-offline';
      final missedSend = await messages.sendMessage(
        authorization: sessionA.authorizationHeader,
        chatId: chatId,
        content: missed,
        clientMessageId: qaClientMessageId(),
      );
      expect(missedSend, isA<MessagesApiOk<VoiceMessage>>());
      final missedId = (missedSend as MessagesApiOk<VoiceMessage>).data.id;

      realtimeB = await ctx.connectSubscribed(sessionB, chatId);
      addTearDown(realtimeB.dispose);
      realtimeB.sendResume();

      final history = await messages.getMessages(
        authorization: sessionB.authorizationHeader,
        chatId: chatId,
      );
      expect(history, isA<MessagesApiOk<MessageListData>>());
      final listed = (history as MessagesApiOk<MessageListData>).data.messages;
      expect(
        listed.any((m) => m.id == missedId && m.content == missed),
        isTrue,
        reason: 'REST catch-up after WS gap',
      );
    },
    skip: runLiveIntegration
        ? null
        : 'Opt in with --dart-define=VOICE_RUN_LIVE_INTEGRATION=true',
  );
}
