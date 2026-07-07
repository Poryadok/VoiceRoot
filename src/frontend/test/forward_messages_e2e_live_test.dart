import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/messages_client.dart';

import 'support/live_gateway_harness.dart';

/// Phase-4 forward E2E (API-level): forward with attribution into a group chat.
///
/// ```text
/// flutter test test/forward_messages_e2e_live_test.dart ^
///   --dart-define=VOICE_RUN_LIVE_INTEGRATION=true ^
///   --dart-define=VOICE_API_BASE_URL=http://127.0.0.1:18080
/// ```
void main() {
  test(
    'forward DM message to group preserves attribution',
    () async {
      final probe = await probeLiveGateway();
      expect(
        probe,
        isA<LiveGatewayReady>(),
        reason: probe is LiveGatewayUnavailable ? probe.reason : null,
      );
      final ctx = (probe as LiveGatewayReady).context;

      final sender = await ctx.registerUser('fwd-sender');
      final peer = await ctx.registerUser('fwd-peer');
      final member = await ctx.registerUser('fwd-member');

      final chats = ctx.chatsClient();
      final dm = await chats.createDm(
        authorization: sender.authorizationHeader,
        otherProfileId: peer.activeProfileId,
      );
      final dmChatId = (dm as ChatsApiOk<VoiceChat>).data.id;

      final groupCreated = await chats.createGroup(
        authorization: sender.authorizationHeader,
        name: 'Forward target',
      );
      expect(groupCreated, isA<ChatsApiOk<VoiceChat>>());
      final group = (groupCreated as ChatsApiOk<VoiceChat>).data;

      final invite = await chats.addGroupMembers(
        authorization: sender.authorizationHeader,
        chatId: group.id,
        profileIds: [peer.activeProfileId, member.activeProfileId],
      );
      expect(invite, isA<ChatsApiOk<void>>());

      final messages = ctx.messagesClient();
      const originalText = 'forward-me-phase4';
      final sent = await messages.sendMessage(
        authorization: peer.authorizationHeader,
        chatId: dmChatId,
        content: originalText,
        clientMessageId: qaClientMessageId(),
      );
      expect(sent, isA<MessagesApiOk<VoiceMessage>>());
      final sourceId = (sent as MessagesApiOk<VoiceMessage>).data.id;

      final wsGroup = await ctx.connectSubscribed(member, group.id);
      addTearDown(wsGroup.dispose);
      final createFuture = waitForOp(
        wsGroup.events,
        'message_create',
        where: (f) => f.data?['chat_id'] == group.id,
      );

      final fwd = await messages.forwardMessage(
        authorization: sender.authorizationHeader,
        sourceMessageId: sourceId,
        targetChatId: group.id,
      );
      expect(fwd, isA<MessagesApiOk<VoiceMessage>>());
      final forwarded = (fwd as MessagesApiOk<VoiceMessage>).data;
      expect(forwarded.messageKind, VoiceMessageKind.forward);
      expect(forwarded.forwardFromId, sourceId);
      expect(forwarded.forwardFromSender, isNotEmpty);
      expect(forwarded.content, originalText);
      expect(forwarded.chatId, group.id);

      final created = await createFuture;
      expect(created.data?['message_id'], forwarded.id);

      final history = await messages.getMessages(
        authorization: member.authorizationHeader,
        chatId: group.id,
      );
      final listed = (history as MessagesApiOk<MessageListData>).data.messages;
      final stored = listed.firstWhere((m) => m.id == forwarded.id);
      expect(stored.messageKind, VoiceMessageKind.forward);
      expect(stored.forwardFromId, sourceId);
      expect(stored.forwardFromSender, forwarded.forwardFromSender);
      expect(stored.content, originalText);
    },
    skip: runLiveIntegration
        ? null
        : 'Opt in with --dart-define=VOICE_RUN_LIVE_INTEGRATION=true',
  );
}
