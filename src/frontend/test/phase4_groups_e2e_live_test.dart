import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/chats_client.dart';

import 'support/live_gateway_harness.dart';

/// Phase-4 groups E2E (API-level): create, invite, avatar, kick.
///
/// ```text
/// flutter test test/phase4_groups_e2e_live_test.dart ^
///   --dart-define=VOICE_RUN_LIVE_INTEGRATION=true ^
///   --dart-define=VOICE_API_BASE_URL=http://127.0.0.1:18080
/// ```
void main() {
  test(
    'standalone group: create, invite, avatar, kick',
    () async {
      final probe = await probeLiveGateway();
      expect(
        probe,
        isA<LiveGatewayReady>(),
        reason: probe is LiveGatewayUnavailable ? probe.reason : null,
      );
      final ctx = (probe as LiveGatewayReady).context;

      final owner = await ctx.registerUser('group-owner');
      final memberB = await ctx.registerUser('group-b');
      final memberC = await ctx.registerUser('group-c');

      final chats = ctx.chatsClient();
      final created = await chats.createGroup(
        authorization: owner.authorizationHeader,
        name: 'Friday squad',
      );
      expect(created, isA<ChatsApiOk<VoiceChat>>());
      final group = (created as ChatsApiOk<VoiceChat>).data;
      expect(group.isGroup, isTrue);
      expect(group.name, 'Friday squad');

      final invite = await chats.addGroupMembers(
        authorization: owner.authorizationHeader,
        chatId: group.id,
        profileIds: [memberB.activeProfileId, memberC.activeProfileId],
      );
      expect(invite, isA<ChatsApiOk<void>>());

      final listB = await chats.listChats(
        authorization: memberB.authorizationHeader,
      );
      final itemsB = (listB as ChatsApiOk<ChatListData>).data.items;
      expect(itemsB.any((i) => i.chatId == group.id), isTrue);

      const avatar = 'https://cdn.voice.gg/groups/flutter-e2e.webp';
      final updated = await chats.updateGroup(
        authorization: owner.authorizationHeader,
        chatId: group.id,
        avatarUrl: avatar,
      );
      expect(updated, isA<ChatsApiOk<VoiceChat>>());
      expect((updated as ChatsApiOk<VoiceChat>).data.avatarUrl, avatar);

      final kick = await chats.removeGroupMember(
        authorization: owner.authorizationHeader,
        chatId: group.id,
        profileId: memberC.activeProfileId,
      );
      expect(kick, isA<ChatsApiOk<void>>());

      final listC = await chats.listChats(
        authorization: memberC.authorizationHeader,
      );
      final itemsC = (listC as ChatsApiOk<ChatListData>).data.items;
      expect(itemsC.any((i) => i.chatId == group.id), isFalse);
    },
    skip: runLiveIntegration
        ? null
        : 'Opt in with --dart-define=VOICE_RUN_LIVE_INTEGRATION=true',
  );
}
