import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/messages_client.dart';
import 'package:voice_frontend/backend/spaces_client.dart';

import 'support/live_gateway_harness.dart';

/// Phase-5 space shell E2E (API-level): list spaces, tree, open text chat.
///
/// ```text
/// flutter test test/spaces_shell_e2e_live_test.dart ^
///   --dart-define=VOICE_RUN_LIVE_INTEGRATION=true ^
///   --dart-define=VOICE_API_BASE_URL=http://127.0.0.1:18080
/// ```
void main() {
  test(
    'space shell: list spaces, tree, open text chat',
    () async {
      final probe = await probeLiveGateway();
      expect(
        probe,
        isA<LiveGatewayReady>(),
        reason: probe is LiveGatewayUnavailable ? probe.reason : null,
      );
      final ctx = (probe as LiveGatewayReady).context;

      final owner = await ctx.registerUser('space-shell-owner');
      final spaces = ctx.spacesClient();
      final chats = ctx.chatsClient();
      final messages = ctx.messagesClient();

      const spaceName = 'Shell E2E';
      const chatName = 'general';

      final created = await spaces.createSpace(
        authorization: owner.authorizationHeader,
        name: spaceName,
      );
      expect(created, isA<SpacesApiOk<VoiceSpace>>());
      final space = (created as SpacesApiOk<VoiceSpace>).data;

      final chatNode = await spaces.createSpaceChat(
        authorization: owner.authorizationHeader,
        spaceId: space.id,
        name: chatName,
      );
      expect(chatNode, isA<SpacesApiOk<SpaceTreeNodeData>>());
      final linkedChatId =
          (chatNode as SpacesApiOk<SpaceTreeNodeData>).data.linkedChatId;
      expect(linkedChatId, isNotNull);

      final list = await spaces.listMySpaces(
        authorization: owner.authorizationHeader,
      );
      expect(list, isA<SpacesApiOk<SpaceListData>>());
      final mySpaces = (list as SpacesApiOk<SpaceListData>).data.spaces;
      expect(mySpaces.any((s) => s.id == space.id && s.name == spaceName),
          isTrue);

      final tree = await spaces.listSpaceTree(
        authorization: owner.authorizationHeader,
        spaceId: space.id,
      );
      expect(tree, isA<SpacesApiOk<SpaceTreeData>>());
      final treeData = (tree as SpacesApiOk<SpaceTreeData>).data;
      expect(
        treeData.nodes.any(
          (n) => n.isTextChat && n.displayName == chatName && !n.isChannelChat,
        ),
        isTrue,
      );

      final chatList = await chats.listChats(
        authorization: owner.authorizationHeader,
      );
      expect(chatList, isA<ChatsApiOk<ChatListData>>());
      final items = (chatList as ChatsApiOk<ChatListData>).data.items;
      final opened = items.where((item) => item.chatId == linkedChatId);
      expect(opened, isNotEmpty);
      expect(opened.first.chat.spaceId, space.id);

      final history = await messages.getMessages(
        authorization: owner.authorizationHeader,
        chatId: linkedChatId!,
      );
      expect(history, isA<MessagesApiOk<MessageListData>>());
    },
    skip: runLiveIntegration
        ? null
        : 'Opt in with --dart-define=VOICE_RUN_LIVE_INTEGRATION=true',
  );
}
