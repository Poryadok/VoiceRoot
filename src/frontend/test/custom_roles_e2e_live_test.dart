import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/messages_client.dart';
import 'package:voice_frontend/backend/roles_client.dart';
import 'package:voice_frontend/backend/spaces_client.dart';
import 'package:voice_frontend/backend/voice_client.dart';
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

    final roleList = await roles.listRoles(
      authorization: owner.authorizationHeader,
      spaceId: spaceId,
    );
    expect(roleList, isA<RolesApiOk<List<SpaceRole>>>());
    final memberRole = (roleList as RolesApiOk<List<SpaceRole>>).data
        .firstWhere((r) => r.name == 'Member');

    const sendMask = 1 << 15; // TEXT_CHAT_SEND_MESSAGES
    final override = await roles.setChatOverride(
      authorization: owner.authorizationHeader,
      spaceId: spaceId,
      chatId: chatId,
      roleId: memberRole.id,
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

    // RL-02: Messaging must enforce the deny on SendMessage.
    final messages = ctx.messagesClient();
    final sent = await messages.sendMessage(
      authorization: member.authorizationHeader,
      chatId: chatId,
      content: 'should be blocked',
      clientMessageId: qaClientMessageId(),
    );
    expect(sent, isA<MessagesApiFailure>());
    expect((sent as MessagesApiFailure).statusCode, 403);
  }, skip: runLiveIntegration ? false : 'Set VOICE_RUN_LIVE_INTEGRATION=true', timeout: const Timeout(Duration(minutes: 2)));

  test('voice room VOICE_JOIN deny blocks join (RL-03)', () async {
    final probe = await probeLiveGateway();
    expect(
      probe,
      isA<LiveGatewayReady>(),
      reason: probe is LiveGatewayUnavailable ? probe.reason : null,
    );
    final ctx = (probe as LiveGatewayReady).context;

    final owner = await ctx.registerUser('vj-deny-owner');
    final member = await ctx.registerUser('vj-deny-member');
    final spaces = ctx.spacesClient();
    final roles = ctx.rolesClient();
    final voice = VoiceCallsClient(gateway: ctx.gatewayHttp());

    final created = await spaces.createSpace(
      authorization: owner.authorizationHeader,
      name: 'Voice Join Deny',
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

    final vr = await spaces.createVoiceRoom(
      authorization: owner.authorizationHeader,
      spaceId: spaceId,
      name: 'Locked Lobby',
    );
    expect(vr, isA<SpacesApiOk<VoiceRoomData>>());
    final voiceRoomId = (vr as SpacesApiOk<VoiceRoomData>).data.id;

    final roleList = await roles.listRoles(
      authorization: owner.authorizationHeader,
      spaceId: spaceId,
    );
    expect(roleList, isA<RolesApiOk<List<SpaceRole>>>());
    final memberRole = (roleList as RolesApiOk<List<SpaceRole>>).data
        .firstWhere((r) => r.name == 'Member');

    const voiceJoinMask = 1 << 17; // VOICE_JOIN
    final override = await roles.setVoiceRoomOverride(
      authorization: owner.authorizationHeader,
      spaceId: spaceId,
      voiceRoomId: voiceRoomId,
      roleId: memberRole.id,
      denyMask: voiceJoinMask,
    );
    expect(override, isA<RolesApiOk<void>>());

    final join = await voice.joinVoiceRoom(
      authorization: member.authorizationHeader,
      voiceRoomId: voiceRoomId,
      spaceId: spaceId,
    );
    expect(join, isA<VoiceApiFailure>());
    expect((join as VoiceApiFailure).statusCode, 403);
  }, skip: runLiveIntegration ? false : 'Set VOICE_RUN_LIVE_INTEGRATION=true', timeout: const Timeout(Duration(minutes: 2)));
}
