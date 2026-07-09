import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/roles_client.dart';
import 'package:voice_frontend/backend/spaces_client.dart';
import 'package:voice_frontend/gen/voice/chat/v1/chat.pbenum.dart';

import 'support/live_gateway_harness.dart';

/// roles/threads (docs/features/roles.md): custom role lifecycle via gateway (create → assign → chat override deny).
void main() {
  test('custom role: create, assign, chat override deny blocks send permission', () async {
    final probe = await probeLiveGateway();
    expect(
      probe,
      isA<LiveGatewayReady>(),
      reason: probe is LiveGatewayUnavailable ? probe.reason : null,
    );
    final ctx = (probe as LiveGatewayReady).context;

    final owner = await ctx.registerUser('phase10-owner');
    final member = await ctx.registerUser('phase10-member');
    final spaces = ctx.spacesClient();
    final roles = ctx.rolesClient();

    final created = await spaces.createSpace(
      authorization: owner.authorizationHeader,
      name: 'Custom Roles E2E',
    );
    expect(created, isA<SpacesApiOk<VoiceSpace>>());
    final spaceId = (created as SpacesApiOk<VoiceSpace>).data.id;

    final invite = await spaces.createInvite(
      authorization: owner.authorizationHeader,
      spaceId: spaceId,
    );
    expect(invite, isA<SpacesApiOk<SpaceInvite>>());
    await spaces.joinByInvite(
      authorization: member.authorizationHeader,
      code: (invite as SpacesApiOk<SpaceInvite>).data.code,
    );

    final channel = await spaces.createSpaceChat(
      authorization: owner.authorizationHeader,
      spaceId: spaceId,
      name: 'perms-channel',
      chatType: ChatType.CHAT_TYPE_CHANNEL,
    );
    expect(channel, isA<SpacesApiOk<SpaceTreeNodeData>>());
    final chatId = (channel as SpacesApiOk<SpaceTreeNodeData>).data.linkedChatId!;

    final create = await roles.createRole(
      authorization: owner.authorizationHeader,
      spaceId: spaceId,
      name: 'Muted',
      position: 2,
    );
    expect(create, isA<RolesApiOk<SpaceRole>>());
    final customRole = (create as RolesApiOk<SpaceRole>).data;

    final assign = await roles.assignRole(
      authorization: owner.authorizationHeader,
      spaceId: spaceId,
      profileId: member.activeProfileId,
      roleId: customRole.id,
    );
    expect(assign, isA<RolesApiOk<void>>());

    const sendMask = 1 << 15; // TEXT_CHAT_SEND_MESSAGES
    final override = await roles.setChatOverride(
      authorization: owner.authorizationHeader,
      spaceId: spaceId,
      chatId: chatId,
      roleId: customRole.id,
      denyMask: sendMask,
    );
    expect(override, isA<RolesApiOk<void>>());

    final check = await roles.checkPermission(
      authorization: owner.authorizationHeader,
      spaceId: spaceId,
      profileId: member.activeProfileId,
      permissionName: 'TEXT_CHAT_SEND_MESSAGES',
      chatId: chatId,
    );
    expect(check, isA<RolesApiOk<bool>>());
    expect((check as RolesApiOk<bool>).data, isFalse);
  }, skip: runLiveIntegration ? false : 'Set VOICE_RUN_LIVE_INTEGRATION=true');
}
