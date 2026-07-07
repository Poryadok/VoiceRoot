import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/spaces_client.dart';
import 'package:voice_frontend/gen/voice/chat/v1/chat.pbenum.dart';

import 'support/live_gateway_harness.dart';

/// Phase-5 space channel E2E (API-level): create channel node, appears in tree.
///
/// ```text
/// flutter test test/spaces_channel_e2e_live_test.dart ^
///   --dart-define=VOICE_RUN_LIVE_INTEGRATION=true ^
///   --dart-define=VOICE_API_BASE_URL=http://127.0.0.1:18080
/// ```
void main() {
  test(
    'space channel: create and list in tree by name',
    () async {
      final probe = await probeLiveGateway();
      expect(
        probe,
        isA<LiveGatewayReady>(),
        reason: probe is LiveGatewayUnavailable ? probe.reason : null,
      );
      final ctx = (probe as LiveGatewayReady).context;

      final owner = await ctx.registerUser('space-channel-owner');
      final spaces = ctx.spacesClient();

      final created = await spaces.createSpace(
        authorization: owner.authorizationHeader,
        name: 'Channel E2E',
      );
      expect(created, isA<SpacesApiOk<VoiceSpace>>());
      final spaceId = (created as SpacesApiOk<VoiceSpace>).data.id;

      const channelName = 'announcements';
      final channel = await spaces.createSpaceChat(
        authorization: owner.authorizationHeader,
        spaceId: spaceId,
        name: channelName,
        chatType: ChatType.CHAT_TYPE_CHANNEL,
      );
      expect(channel, isA<SpacesApiOk<SpaceTreeNodeData>>());
      final node = (channel as SpacesApiOk<SpaceTreeNodeData>).data;
      expect(node.displayName, channelName);
      expect(node.isTextChat, isTrue);
      expect(node.isChannelChat, isTrue);
      expect(node.linkedChatId, isNotNull);

      final tree = await spaces.listSpaceTree(
        authorization: owner.authorizationHeader,
        spaceId: spaceId,
      );
      expect(tree, isA<SpacesApiOk<SpaceTreeData>>());
      final nodes = (tree as SpacesApiOk<SpaceTreeData>).data.nodes;
      final inTree = nodes.where(
        (n) => n.isTextChat && n.displayName == channelName,
      );
      expect(inTree, isNotEmpty);
      final treeNode = inTree.first;
      expect(treeNode.isChannelChat, isTrue);
      expect(treeNode.linkedChatId, node.linkedChatId);
    },
    skip: runLiveIntegration
        ? null
        : 'Opt in with --dart-define=VOICE_RUN_LIVE_INTEGRATION=true',
  );
}
