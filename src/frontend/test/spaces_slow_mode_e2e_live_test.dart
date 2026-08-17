import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/messages_client.dart';
import 'package:voice_frontend/backend/spaces_client.dart';

import 'support/live_gateway_harness.dart';

void main() {
  test('slow mode blocks rapid sends in space channel', () async {
    final probe = await probeLiveGateway();
    expect(probe, isA<LiveGatewayReady>());
    final ctx = (probe as LiveGatewayReady).context;
    final owner = await ctx.registerUser('slow-owner');
    final spaces = ctx.spacesClient();
    final chats = ctx.chatsClient();
    final messages = ctx.messagesClient();

    final created = await spaces.createSpace(
      authorization: owner.authorizationHeader,
      name: 'Slow mode E2E',
    );
    final spaceId = (created as SpacesApiOk<VoiceSpace>).data.id;
    final chatNode = await spaces.createSpaceChat(
      authorization: owner.authorizationHeader,
      spaceId: spaceId,
      name: 'slow-general',
    );
    expect(chatNode, isA<SpacesApiOk<SpaceTreeNodeData>>());
    final chatId = (chatNode as SpacesApiOk<SpaceTreeNodeData>).data.linkedChatId!;
    expect(chatId, isNotEmpty);

    await chats.updateGroup(
      authorization: owner.authorizationHeader,
      chatId: chatId,
      slowModeSeconds: 10,
    );

    final first = await messages.sendMessage(
      authorization: owner.authorizationHeader,
      chatId: chatId,
      content: 'slow-1',
      clientMessageId: qaClientMessageId(),
    );
    expect(first, isA<MessagesApiOk<VoiceMessage>>());

    final second = await messages.sendMessage(
      authorization: owner.authorizationHeader,
      chatId: chatId,
      content: 'slow-2',
      clientMessageId: qaClientMessageId(),
    );
    expect(second, isA<MessagesApiFailure>());
  }, skip: runLiveIntegration ? null : 'opt-in live');
}
