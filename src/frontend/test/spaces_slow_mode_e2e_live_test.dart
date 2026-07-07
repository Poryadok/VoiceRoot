import 'package:flutter_test/flutter_test.dart';
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
    final tree = await spaces.listSpaceTree(
      authorization: owner.authorizationHeader,
      spaceId: spaceId,
    );
    final channel = (tree as SpacesApiOk<SpaceTreeData>).data.nodes
        .firstWhere((n) => n.isTextChat);
    final chatId = channel.linkedChatId!;
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
