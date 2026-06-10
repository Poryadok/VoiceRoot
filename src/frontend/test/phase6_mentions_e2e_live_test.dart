import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/messages_client.dart';

import 'support/live_gateway_harness.dart';

/// Phase-6 mentions E2E (API + WS): @user mention notifies target in group chat.
///
/// ```text
/// flutter test test/phase6_mentions_e2e_live_test.dart ^
///   --dart-define=VOICE_RUN_LIVE_INTEGRATION=true ^
///   --dart-define=VOICE_API_BASE_URL=http://127.0.0.1:18080
/// ```
void main() {
  test(
    'group @user mention delivers mention WS and notification',
    () async {
    final probe = await probeLiveGateway();
    expect(
      probe,
      isA<LiveGatewayReady>(),
      reason: probe is LiveGatewayUnavailable ? probe.reason : null,
    );
    final ctx = (probe as LiveGatewayReady).context;

    final owner = await ctx.registerUser('mention-owner');
    final target = await ctx.registerUser('mention-target');

    final chats = ctx.chatsClient();
    final groupCreated = await chats.createGroup(
      authorization: owner.authorizationHeader,
      name: 'Mentions target',
    );
    expect(groupCreated, isA<ChatsApiOk<VoiceChat>>());
    final group = (groupCreated as ChatsApiOk<VoiceChat>).data;

    final invite = await chats.addGroupMembers(
      authorization: owner.authorizationHeader,
      chatId: group.id,
      profileIds: [target.activeProfileId],
    );
    expect(invite, isA<ChatsApiOk<void>>());

    final messages = ctx.messagesClient();
    final wsTarget = await ctx.connectSubscribed(target, group.id);
    addTearDown(wsTarget.dispose);

    final mentionFuture = waitForOp(wsTarget.events, 'mention');
    final notificationFuture = waitForOp(wsTarget.events, 'notification');

    const body = 'please see this';
    final sent = await messages.sendMessage(
      authorization: owner.authorizationHeader,
      chatId: group.id,
      content: '$body @${target.activeProfileId}',
      mentions: [
        MessageMention(type: 'user', targetId: target.activeProfileId),
      ],
      clientMessageId: qaClientMessageId(),
    );
    expect(sent, isA<MessagesApiOk<VoiceMessage>>());

    final mentionFrame = await mentionFuture;
    expect(mentionFrame.data?['message_id'], isNotEmpty);
    expect(mentionFrame.data?['user_id'], target.activeProfileId);

    final notificationFrame = await notificationFuture;
    expect(notificationFrame.data?['type'], 'mention');
    },
    skip: runLiveIntegration
        ? null
        : 'Opt in with --dart-define=VOICE_RUN_LIVE_INTEGRATION=true',
  );
}
