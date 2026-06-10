import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/messages_client.dart';

import 'support/live_gateway_harness.dart';

/// Phase-6 pins E2E (API-level): pin, list, unpin, WS fan-out in a group chat.
///
/// ```text
/// flutter test test/phase6_pins_e2e_live_test.dart ^
///   --dart-define=VOICE_RUN_LIVE_INTEGRATION=true ^
///   --dart-define=VOICE_API_BASE_URL=http://127.0.0.1:18080
/// ```
void main() {
  test('group message pin list and unpin with WS event', () async {
    final probe = await probeLiveGateway();
    expect(
      probe,
      isA<LiveGatewayReady>(),
      reason: probe is LiveGatewayUnavailable ? probe.reason : null,
    );
    final ctx = (probe as LiveGatewayReady).context;

    final owner = await ctx.registerUser('pin-owner');
    final member = await ctx.registerUser('pin-member');

    final chats = ctx.chatsClient();
    final groupCreated = await chats.createGroup(
      authorization: owner.authorizationHeader,
      name: 'Pins target',
    );
    expect(groupCreated, isA<ChatsApiOk>());
    final group = (groupCreated as ChatsApiOk).data;

    final invite = await chats.addGroupMembers(
      authorization: owner.authorizationHeader,
      chatId: group.id,
      profileIds: [member.activeProfileId],
    );
    expect(invite, isA<ChatsApiOk<void>>());

    final messages = ctx.messagesClient();
    const body = 'pin-me-phase6';
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
    final pinFuture = waitForOp(
      wsMember.events,
      'message_pinned',
      where: (f) => f.data?['message_id'] == msgId,
    );

    final pinned = await messages.pinMessage(
      authorization: member.authorizationHeader,
      messageId: msgId,
      chatId: group.id,
    );
    expect(pinned, isA<MessagesApiOk<void>>());

    final wsPin = await pinFuture;
    expect(wsPin.data?['chat_id'], group.id);

    final listed = await messages.getPinnedMessages(
      authorization: owner.authorizationHeader,
      chatId: group.id,
    );
    expect(listed, isA<MessagesApiOk<MessageListData>>());
    final pins = (listed as MessagesApiOk<MessageListData>).data.messages;
    expect(pins, hasLength(1));
    expect(pins.first.id, msgId);
    expect(pins.first.isPinned, isTrue);

    final unpinned = await messages.unpinMessage(
      authorization: member.authorizationHeader,
      messageId: msgId,
      chatId: group.id,
    );
    expect(unpinned, isA<MessagesApiOk<void>>());

    final afterUnpin = await messages.getPinnedMessages(
      authorization: owner.authorizationHeader,
      chatId: group.id,
    );
    expect((afterUnpin as MessagesApiOk<MessageListData>).data.messages, isEmpty);
  },
    skip: runLiveIntegration
        ? null
        : 'Opt in with --dart-define=VOICE_RUN_LIVE_INTEGRATION=true',
  );
}
