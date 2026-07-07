import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/messages_client.dart';

import 'support/live_gateway_harness.dart';

/// Phase-6 markdown E2E (API-level): source preserved, preview stripped.
///
/// ```text
/// flutter test test/markdown_e2e_live_test.dart ^
///   --dart-define=VOICE_RUN_LIVE_INTEGRATION=true ^
///   --dart-define=VOICE_API_BASE_URL=http://127.0.0.1:18080
/// ```
void main() {
  test(
    'markdown message keeps source and strips chat list preview',
    () async {
      final probe = await probeLiveGateway();
      expect(
        probe,
        isA<LiveGatewayReady>(),
        reason: probe is LiveGatewayUnavailable ? probe.reason : null,
      );
      final ctx = (probe as LiveGatewayReady).context;

      final sessionA = await ctx.registerUser('md-a');
      final sessionB = await ctx.registerUser('md-b');

      final dm = await ctx.chatsClient().createDm(
        authorization: sessionA.authorizationHeader,
        otherProfileId: sessionB.activeProfileId,
      );
      final chatId = (dm as ChatsApiOk<VoiceChat>).data.id;

      const body = '**bold** and [Voice](https://voice.app)';
      final messages = ctx.messagesClient();
      final sent = await messages.sendMessage(
        authorization: sessionA.authorizationHeader,
        chatId: chatId,
        content: body,
        clientMessageId: qaClientMessageId(),
      );
      expect(sent, isA<MessagesApiOk<VoiceMessage>>());
      final msg = (sent as MessagesApiOk<VoiceMessage>).data;
      expect(msg.content, body);

      final history = await messages.getMessages(
        authorization: sessionB.authorizationHeader,
        chatId: chatId,
      );
      final listed =
          (history as MessagesApiOk<MessageListData>).data.messages;
      expect(
        listed.any((m) => m.id == msg.id && m.content == body),
        isTrue,
      );

      final chats = await ctx.chatsClient().listChats(
        authorization: sessionB.authorizationHeader,
      );
      final items = (chats as ChatsApiOk<ChatListData>).data.items;
      final row = items.firstWhere((i) => i.chat.id == chatId);
      expect(row.lastMessagePreview, 'bold and Voice');
    },
    skip: runLiveIntegration
        ? null
        : 'Opt in with --dart-define=VOICE_RUN_LIVE_INTEGRATION=true',
  );
}
