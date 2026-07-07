import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/messages_client.dart';
import 'package:voice_frontend/backend/spaces_client.dart';
import 'package:voice_frontend/gen/voice/chat/v1/chat.pbenum.dart';

import 'support/live_gateway_harness.dart';

/// Phase-6 space channel mentions E2E: @user in space channel inherits space_members.
///
/// ```text
/// flutter test test/spaces_channel_mentions_e2e_live_test.dart ^
///   --dart-define=VOICE_RUN_LIVE_INTEGRATION=true ^
///   --dart-define=VOICE_API_BASE_URL=http://127.0.0.1:18080
/// ```
void main() {
  test(
    'space channel @user mention delivers WS mention to space member',
    () async {
      final probe = await probeLiveGateway();
      expect(
        probe,
        isA<LiveGatewayReady>(),
        reason: probe is LiveGatewayUnavailable ? probe.reason : null,
      );
      final ctx = (probe as LiveGatewayReady).context;

      final owner = await ctx.registerUser('space-mention-owner');
      final target = await ctx.registerUser('space-mention-target');
      final spaces = ctx.spacesClient();

      final created = await spaces.createSpace(
        authorization: owner.authorizationHeader,
        name: 'Mentions Space',
      );
      expect(created, isA<SpacesApiOk<VoiceSpace>>());
      final spaceId = (created as SpacesApiOk<VoiceSpace>).data.id;

      final channel = await spaces.createSpaceChat(
        authorization: owner.authorizationHeader,
        spaceId: spaceId,
        name: 'general',
        chatType: ChatType.CHAT_TYPE_CHANNEL,
      );
      expect(channel, isA<SpacesApiOk<SpaceTreeNodeData>>());
      final chatId = (channel as SpacesApiOk<SpaceTreeNodeData>).data.linkedChatId;
      expect(chatId, isNotNull);

      final invite = await spaces.createInvite(
        authorization: owner.authorizationHeader,
        spaceId: spaceId,
      );
      expect(invite, isA<SpacesApiOk<SpaceInvite>>());
      await spaces.joinByInvite(
        authorization: target.authorizationHeader,
        code: (invite as SpacesApiOk<SpaceInvite>).data.code,
      );

      final messages = ctx.messagesClient();
      final wsTarget = await ctx.connectSubscribed(target, chatId!);
      addTearDown(wsTarget.dispose);

      final mentionFuture = waitForOp(wsTarget.events, 'mention');

      const body = 'space channel ping';
      final sent = await messages.sendMessage(
        authorization: owner.authorizationHeader,
        chatId: chatId,
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
    },
    skip: runLiveIntegration
        ? null
        : 'Opt in with --dart-define=VOICE_RUN_LIVE_INTEGRATION=true',
  );
}
