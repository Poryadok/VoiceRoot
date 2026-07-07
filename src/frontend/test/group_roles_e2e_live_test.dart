import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/chats_client.dart';

import 'support/live_gateway_harness.dart';

/// Phase-4 simple roles E2E (API-level): owner/member roles, leave, owner protections.
///
/// ```text
/// flutter test test/group_roles_e2e_live_test.dart ^
///   --dart-define=VOICE_RUN_LIVE_INTEGRATION=true ^
///   --dart-define=VOICE_API_BASE_URL=http://127.0.0.1:18080
/// ```
void main() {
  test(
    'standalone group: roles, leave, owner cannot leave or be kicked',
    () async {
      final probe = await probeLiveGateway();
      expect(
        probe,
        isA<LiveGatewayReady>(),
        reason: probe is LiveGatewayUnavailable ? probe.reason : null,
      );
      final ctx = (probe as LiveGatewayReady).context;

      final owner = await ctx.registerUser('roles-owner');
      final memberB = await ctx.registerUser('roles-b');
      final memberC = await ctx.registerUser('roles-c');
      final leaver = await ctx.registerUser('roles-leaver');

      final chats = ctx.chatsClient();
      final created = await chats.createGroup(
        authorization: owner.authorizationHeader,
        name: 'Roles squad',
      );
      expect(created, isA<ChatsApiOk<VoiceChat>>());
      final group = (created as ChatsApiOk<VoiceChat>).data;

      final invite = await chats.addGroupMembers(
        authorization: owner.authorizationHeader,
        chatId: group.id,
        profileIds: [
          memberB.activeProfileId,
          memberC.activeProfileId,
          leaver.activeProfileId,
        ],
      );
      expect(invite, isA<ChatsApiOk<void>>());

      final members = await chats.listGroupMembers(
        authorization: owner.authorizationHeader,
        chatId: group.id,
      );
      expect(members, isA<ChatsApiOk<MemberListData>>());
      final roles = {
        for (final m in (members as ChatsApiOk<MemberListData>).data.members)
          m.profileId: m.role,
      };
      expect(roles[owner.activeProfileId], 'owner');
      expect(roles[memberB.activeProfileId], 'member');
      expect(roles[memberC.activeProfileId], 'member');
      expect(roles[leaver.activeProfileId], 'member');

      final kickOwner = await chats.removeGroupMember(
        authorization: owner.authorizationHeader,
        chatId: group.id,
        profileId: owner.activeProfileId,
      );
      expect(kickOwner, isA<ChatsApiFailure>());
      expect((kickOwner as ChatsApiFailure).statusCode, 412);

      final ownerLeave = await chats.leaveGroup(
        authorization: owner.authorizationHeader,
        chatId: group.id,
      );
      expect(ownerLeave, isA<ChatsApiFailure>());
      expect((ownerLeave as ChatsApiFailure).statusCode, 412);

      final memberLeave = await chats.leaveGroup(
        authorization: leaver.authorizationHeader,
        chatId: group.id,
      );
      expect(memberLeave, isA<ChatsApiOk<void>>());

      final listLeaver = await chats.listChats(
        authorization: leaver.authorizationHeader,
      );
      final itemsLeaver = (listLeaver as ChatsApiOk<ChatListData>).data.items;
      expect(itemsLeaver.any((i) => i.chatId == group.id), isFalse);

      final listB = await chats.listChats(
        authorization: memberB.authorizationHeader,
      );
      final itemsB = (listB as ChatsApiOk<ChatListData>).data.items;
      expect(itemsB.any((i) => i.chatId == group.id), isTrue);
    },
    skip: runLiveIntegration
        ? null
        : 'Opt in with --dart-define=VOICE_RUN_LIVE_INTEGRATION=true',
  );
}
