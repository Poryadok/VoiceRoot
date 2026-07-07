import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/messages_client.dart';

import 'support/live_gateway_harness.dart';

void main() {
  test(
    'edit and delete message with WS message_update',
    () async {
      final probe = await probeLiveGateway();
      expect(
        probe,
        isA<LiveGatewayReady>(),
        reason: probe is LiveGatewayUnavailable ? probe.reason : null,
      );
      final ctx = (probe as LiveGatewayReady).context;

      final sessionA = await ctx.registerUser('edit-a');
      final sessionB = await ctx.registerUser('edit-b');

      final dm = await ctx.chatsClient().createDm(
        authorization: sessionA.authorizationHeader,
        otherProfileId: sessionB.activeProfileId,
      );
      final chatId = (dm as ChatsApiOk<VoiceChat>).data.id;

      final messages = ctx.messagesClient();
      final wsB = await ctx.connectSubscribed(sessionB, chatId);
      addTearDown(wsB.dispose);

      final createFuture = waitForOp(
        wsB.events,
        'message_create',
        where: (f) => f.data?['chat_id'] == chatId,
      );
      final send = await messages.sendMessage(
        authorization: sessionA.authorizationHeader,
        chatId: chatId,
        content: 'before-edit',
        clientMessageId: qaClientMessageId(),
      );
      final msgId = (send as MessagesApiOk<VoiceMessage>).data.id;
      final created = await createFuture;
      expect(created.data?['message_id'], msgId);

      const edited = 'after-edit';
      final updateFuture = waitForOp(
        wsB.events,
        'message_update',
        where: (f) => f.data?['message_id'] == msgId,
      );
      final edit = await messages.editMessage(
        authorization: sessionA.authorizationHeader,
        messageId: msgId,
        content: edited,
      );
      expect(edit, isA<MessagesApiOk<VoiceMessage>>());
      await updateFuture;

      final history = await messages.getMessages(
        authorization: sessionB.authorizationHeader,
        chatId: chatId,
      );
      final listed = (history as MessagesApiOk<MessageListData>).data.messages;
      expect(listed.any((m) => m.id == msgId && m.content == edited), isTrue);

      final deleted = await messages.deleteMessage(
        authorization: sessionA.authorizationHeader,
        messageId: msgId,
        scope: 'everyone',
      );
      expect(deleted, isA<MessagesApiOk<void>>());

      final afterDelete = await messages.getMessages(
        authorization: sessionB.authorizationHeader,
        chatId: chatId,
      );
      final afterList =
          (afterDelete as MessagesApiOk<MessageListData>).data.messages;
      expect(afterList.any((m) => m.id == msgId), isFalse);
    },
    skip: runLiveIntegration
        ? null
        : 'Opt in with --dart-define=VOICE_RUN_LIVE_INTEGRATION=true',
  );
}
