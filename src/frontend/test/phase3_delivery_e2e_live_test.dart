import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/messages_client.dart';

import 'support/live_gateway_harness.dart';

void main() {
  test(
    'delivery_ack notifies sender via message_delivered WS',
    () async {
      final probe = await probeLiveGateway();
      expect(
        probe,
        isA<LiveGatewayReady>(),
        reason: probe is LiveGatewayUnavailable ? probe.reason : null,
      );
      final ctx = (probe as LiveGatewayReady).context;

      final sessionA = await ctx.registerUser('delivery-a');
      final sessionB = await ctx.registerUser('delivery-b');

      final dm = await ctx.chatsClient().createDm(
        authorization: sessionA.authorizationHeader,
        otherProfileId: sessionB.activeProfileId,
      );
      final chatId = (dm as ChatsApiOk<VoiceChat>).data.id;

      final messages = ctx.messagesClient();
      final wsA = await ctx.connectSubscribed(sessionA, chatId);
      addTearDown(wsA.dispose);
      final wsB = await ctx.connectSubscribed(sessionB, chatId);
      addTearDown(wsB.dispose);

      final deliveredFuture = waitForOp(wsA.events, 'message_delivered');
      final createFuture = waitForOp(
        wsB.events,
        'message_create',
        where: (f) => f.data?['chat_id'] == chatId,
      );
      final send = await messages.sendMessage(
        authorization: sessionA.authorizationHeader,
        chatId: chatId,
        content: 'delivery-e2e',
        clientMessageId: qaClientMessageId(),
      );
      final msgId = (send as MessagesApiOk<VoiceMessage>).data.id;
      final created = await createFuture;
      expect(created.data?['message_id'], msgId);

      wsB.sendDeliveryAck(
        chatId: chatId,
        messageId: msgId,
        senderProfileId: sessionA.activeProfileId,
      );

      final delivered = await deliveredFuture;
      expect(delivered.data?['message_id'], msgId);
      expect(delivered.data?['chat_id'], chatId);
    },
    skip: runLiveIntegration
        ? null
        : 'Opt in with --dart-define=VOICE_RUN_LIVE_INTEGRATION=true',
  );
}
