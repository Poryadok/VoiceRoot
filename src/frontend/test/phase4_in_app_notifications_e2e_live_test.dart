import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/messages_client.dart';
import 'package:voice_frontend/backend/realtime_client.dart';

import 'support/live_gateway_harness.dart';

/// Phase-4 in-app notifications E2E (API + WS): unread badge when member is not viewing chat.
///
/// ```text
/// flutter test test/phase4_in_app_notifications_e2e_live_test.dart ^
///   --dart-define=VOICE_RUN_LIVE_INTEGRATION=true ^
///   --dart-define=VOICE_API_BASE_URL=http://127.0.0.1:18080
/// ```
void main() {
  test(
    'group message raises unread and delivers notification WS for offline viewer',
    () async {
      final probe = await probeLiveGateway();
      expect(
        probe,
        isA<LiveGatewayReady>(),
        reason: probe is LiveGatewayUnavailable ? probe.reason : null,
      );
      final ctx = (probe as LiveGatewayReady).context;

      final owner = await ctx.registerUser('notify-owner');
      final member = await ctx.registerUser('notify-member');
      final filler = await ctx.registerUser('notify-filler');

      final chats = ctx.chatsClient();
      final groupCreated = await chats.createGroup(
        authorization: owner.authorizationHeader,
        name: 'Notify target',
      );
      expect(groupCreated, isA<ChatsApiOk<VoiceChat>>());
      final group = (groupCreated as ChatsApiOk<VoiceChat>).data;

      final invite = await chats.addGroupMembers(
        authorization: owner.authorizationHeader,
        chatId: group.id,
        profileIds: [member.activeProfileId, filler.activeProfileId],
      );
      expect(invite, isA<ChatsApiOk<void>>());

      final beforeList = await chats.listChats(
        authorization: member.authorizationHeader,
      );
      final beforeItems =
          (beforeList as ChatsApiOk<ChatListData>).data.items;
      final beforeGroup = beforeItems.firstWhere((i) => i.chatId == group.id);
      expect(beforeGroup.unreadCount, 0);

      // Member online and subscribed to the group but not viewing it in UI (no selected chat).
      final wsMember = await ctx.connectSubscribed(member, group.id);
      addTearDown(wsMember.dispose);
      final notifyFuture = waitForOp(
        wsMember.events,
        'notification',
        where: (f) =>
            f.data?['type'] == 'new_message' && f.data?['chat_id'] == group.id,
      );

      final messages = ctx.messagesClient();
      const body = 'notify-me-phase4';
      final sent = await messages.sendMessage(
        authorization: owner.authorizationHeader,
        chatId: group.id,
        content: body,
        clientMessageId: qaClientMessageId(),
      );
      expect(sent, isA<MessagesApiOk<VoiceMessage>>());
      final msgId = (sent as MessagesApiOk<VoiceMessage>).data.id;

      final wsNotify = await notifyFuture;
      expect(wsNotify.data?['message_id'], msgId);
      expect(wsNotify.data?['sender_profile_id'], owner.activeProfileId);

      final afterList = await chats.listChats(
        authorization: member.authorizationHeader,
      );
      final afterGroup = (afterList as ChatsApiOk<ChatListData>)
          .data
          .items
          .firstWhere((i) => i.chatId == group.id);
      expect(afterGroup.unreadCount, greaterThan(0));
    },
    skip: runLiveIntegration
        ? null
        : 'Opt in with --dart-define=VOICE_RUN_LIVE_INTEGRATION=true',
  );

  test(
    'reaction on member message delivers notification WS to author',
    () async {
      final probe = await probeLiveGateway();
      expect(
        probe,
        isA<LiveGatewayReady>(),
        reason: probe is LiveGatewayUnavailable ? probe.reason : null,
      );
      final ctx = (probe as LiveGatewayReady).context;

      final owner = await ctx.registerUser('notify-react-owner');
      final member = await ctx.registerUser('notify-react-member');
      final filler = await ctx.registerUser('notify-react-filler');

      final chats = ctx.chatsClient();
      final groupCreated = await chats.createGroup(
        authorization: owner.authorizationHeader,
        name: 'Notify reactions',
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
      const body = 'react-notify-phase4';
      final sent = await messages.sendMessage(
        authorization: member.authorizationHeader,
        chatId: group.id,
        content: body,
        clientMessageId: qaClientMessageId(),
      );
      expect(sent, isA<MessagesApiOk<VoiceMessage>>());
      final msgId = (sent as MessagesApiOk<VoiceMessage>).data.id;

      final wsMember = await ctx.connectSubscribed(member, group.id);
      addTearDown(wsMember.dispose);
      final notifyFuture = waitForOp(
        wsMember.events,
        'notification',
        where: (f) =>
            f.data?['type'] == 'reaction' &&
            f.data?['chat_id'] == group.id &&
            f.data?['message_id'] == msgId,
      );

      final reacted = await messages.addReaction(
        authorization: owner.authorizationHeader,
        messageId: msgId,
        emoji: '👍',
      );
      expect(reacted, isA<MessagesApiOk<void>>());

      final wsNotify = await notifyFuture;
      expect(wsNotify.data?['reactor_profile_id'], owner.activeProfileId);
      expect(wsNotify.data?['emoji'], '👍');
    },
    skip: runLiveIntegration
        ? null
        : 'Opt in with --dart-define=VOICE_RUN_LIVE_INTEGRATION=true',
  );
}
