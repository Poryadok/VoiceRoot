import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/roles_client.dart';
import 'package:voice_frontend/backend/spaces_client.dart';

import 'support/live_gateway_harness.dart';

/// Phase-5 space moderation E2E (API-level): kick, ban/unban, timeout.
void main() {
  test('space moderation: mod kick, ban list, unban rejoin', () async {
    final probe = await probeLiveGateway();
    expect(
      probe,
      isA<LiveGatewayReady>(),
      reason: probe is LiveGatewayUnavailable ? probe.reason : null,
    );
    final ctx = (probe as LiveGatewayReady).context;

    final owner = await ctx.registerUser('space-mod-owner');
    final moderator = await ctx.registerUser('space-mod-moderator');
    final member = await ctx.registerUser('space-mod-member');
    final spaces = ctx.spacesClient();
    final roles = ctx.rolesClient();

    final created = await spaces.createSpace(
      authorization: owner.authorizationHeader,
      name: 'Moderation E2E',
    );
    expect(created, isA<SpacesApiOk<VoiceSpace>>());
    final spaceId = (created as SpacesApiOk<VoiceSpace>).data.id;

    final modRoleListed = await roles.listRoles(
      authorization: owner.authorizationHeader,
      spaceId: spaceId,
    );
    expect(modRoleListed, isA<RolesApiOk<List<SpaceRole>>>());
    final modRole = (modRoleListed as RolesApiOk<List<SpaceRole>>)
        .data
        .firstWhere((r) => r.name == 'Moderator');

    final inviteMod = await spaces.createInvite(
      authorization: owner.authorizationHeader,
      spaceId: spaceId,
    );
    expect(inviteMod, isA<SpacesApiOk<SpaceInvite>>());
    await spaces.joinByInvite(
      authorization: moderator.authorizationHeader,
      code: (inviteMod as SpacesApiOk<SpaceInvite>).data.code,
    );
    await roles.assignRole(
      authorization: owner.authorizationHeader,
      spaceId: spaceId,
      profileId: moderator.activeProfileId,
      roleId: modRole.id,
    );

    final inviteMember = await spaces.createInvite(
      authorization: owner.authorizationHeader,
      spaceId: spaceId,
    );
    expect(inviteMember, isA<SpacesApiOk<SpaceInvite>>());
    await spaces.joinByInvite(
      authorization: member.authorizationHeader,
      code: (inviteMember as SpacesApiOk<SpaceInvite>).data.code,
    );

    final kick = await spaces.kickMember(
      authorization: moderator.authorizationHeader,
      spaceId: spaceId,
      profileId: member.activeProfileId,
    );
    expect(kick, isA<SpacesApiOk<void>>());

    final reinvite = await spaces.createInvite(
      authorization: owner.authorizationHeader,
      spaceId: spaceId,
    );
    expect(reinvite, isA<SpacesApiOk<SpaceInvite>>());
    await spaces.joinByInvite(
      authorization: member.authorizationHeader,
      code: (reinvite as SpacesApiOk<SpaceInvite>).data.code,
    );

    final ban = await spaces.banMember(
      authorization: owner.authorizationHeader,
      spaceId: spaceId,
      accountId: member.accountId,
      reason: 'e2e',
    );
    expect(ban, isA<SpacesApiOk<void>>());

    final bannedJoin = await spaces.joinByInvite(
      authorization: member.authorizationHeader,
      code: (reinvite as SpacesApiOk<SpaceInvite>).data.code,
    );
    expect(bannedJoin, isA<SpacesApiFailure>());

    final unban = await spaces.unbanMember(
      authorization: owner.authorizationHeader,
      spaceId: spaceId,
      accountId: member.accountId,
    );
    expect(unban, isA<SpacesApiOk<void>>());

    final timeout = await spaces.timeoutMember(
      authorization: moderator.authorizationHeader,
      spaceId: spaceId,
      profileId: member.activeProfileId,
      durationSeconds: 300,
    );
    expect(timeout, isA<SpacesApiOk<void>>());

    final clearTimeout = await spaces.removeMemberTimeout(
      authorization: moderator.authorizationHeader,
      spaceId: spaceId,
      profileId: member.activeProfileId,
    );
    expect(clearTimeout, isA<SpacesApiOk<void>>());
  }, skip: !runLiveIntegration);
}
