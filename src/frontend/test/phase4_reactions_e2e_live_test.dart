import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/messages_client.dart';

import 'support/live_gateway_harness.dart';

/// Phase-4 reactions E2E (API-level): emoji counters and toggle in a group chat.
///
/// ```text
/// flutter test test/phase4_reactions_e2e_live_test.dart ^
///   --dart-define=VOICE_RUN_LIVE_INTEGRATION=true ^
///   --dart-define=VOICE_API_BASE_URL=http://127.0.0.1:18080
/// ```
void main() {
  test(
    'group message reactions aggregate and remove',
    () async {
      final probe = await probeLiveGateway();
      expect(
        probe,
        isA<LiveGatewayReady>(),
        reason: probe is LiveGatewayUnavailable ? probe.reason : null,
      );
      final ctx = (probe as LiveGatewayReady).context;

      final owner = await ctx.registerUser('react-owner');
      final member = await ctx.registerUser('react-member');
      final filler = await ctx.registerUser('react-filler');

      final chats = ctx.chatsClient();
      final groupCreated = await chats.createGroup(
        authorization: owner.authorizationHeader,
        name: 'Reactions target',
      );
      expect(groupCreated, isA<ChatsApiOk<VoiceChat>>());
      final group = (groupCreated as ChatsApiOk<VoiceChat>).data;

      final invite = await chats.addGroupMembers(
        authorization: owner.authorizationHeader,
        chatId: group.id,
        profileIds: [member.activeProfileId, filler.activeProfileId],
      );
      expect(invite, isA<ChatsApiOk<void>>());

      final messages = ctx.messagesClient();
      const body = 'react-me-phase4';
      final sent = await messages.sendMessage(
        authorization: owner.authorizationHeader,
        chatId: group.id,
        content: body,
        clientMessageId: qaClientMessageId(),
      );
      expect(sent, isA<MessagesApiOk<VoiceMessage>>());
      final msgId = (sent as MessagesApiOk<VoiceMessage>).data.id;

      final wsMember = await ctx.connectSubscribed(member, group.id);
      addTearDown(wsMember.dispose);
      final addFuture = waitForOp(
        wsMember.events,
        'reaction_add',
        where: (f) =>
            f.data?['message_id'] == msgId && f.data?['emoji'] == '👍',
      );

      final added = await messages.addReaction(
        authorization: member.authorizationHeader,
        messageId: msgId,
        emoji: '👍',
      );
      expect(added, isA<MessagesApiOk<void>>());

      final wsAdd = await addFuture;
      expect(wsAdd.data?['chat_id'], group.id);
      expect(wsAdd.data?['profile_id'], member.activeProfileId);

      final afterOne = await messages.getMessages(
        authorization: owner.authorizationHeader,
        chatId: group.id,
      );
      final listedOne =
          (afterOne as MessagesApiOk<MessageListData>).data.messages;
      final withOne = listedOne.firstWhere((m) => m.id == msgId);
      expect(withOne.reactions, hasLength(1));
      expect(withOne.reactions.single.emoji, '👍');
      expect(withOne.reactions.single.count, 1);
      expect(withOne.reactions.single.reactedByMe, isFalse);

      final ownerReact = await messages.addReaction(
        authorization: owner.authorizationHeader,
        messageId: msgId,
        emoji: '👍',
      );
      expect(ownerReact, isA<MessagesApiOk<void>>());

      final afterTwo = await messages.getMessages(
        authorization: member.authorizationHeader,
        chatId: group.id,
      );
      final withTwo =
          (afterTwo as MessagesApiOk<MessageListData>).data.messages
              .firstWhere((m) => m.id == msgId);
      expect(withTwo.reactions.single.count, 2);
      expect(withTwo.reactions.single.reactedByMe, isTrue);

      final removeFuture = waitForOp(
        wsMember.events,
        'reaction_remove',
        where: (f) =>
            f.data?['message_id'] == msgId && f.data?['emoji'] == '👍',
      );

      final removed = await messages.removeReaction(
        authorization: member.authorizationHeader,
        messageId: msgId,
        emoji: '👍',
      );
      expect(removed, isA<MessagesApiOk<void>>());

      final wsRemove = await removeFuture;
      expect(wsRemove.data?['profile_id'], member.activeProfileId);

      final afterRemove = await messages.getMessages(
        authorization: owner.authorizationHeader,
        chatId: group.id,
      );
      final finalMsg =
          (afterRemove as MessagesApiOk<MessageListData>).data.messages
              .firstWhere((m) => m.id == msgId);
      expect(finalMsg.reactions.single.count, 1);
      expect(finalMsg.reactions.single.reactedByMe, isTrue);
    },
    skip: runLiveIntegration
        ? null
        : 'Opt in with --dart-define=VOICE_RUN_LIVE_INTEGRATION=true',
  );
}
