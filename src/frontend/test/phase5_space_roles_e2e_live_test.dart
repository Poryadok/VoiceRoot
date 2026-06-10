import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/roles_client.dart';
import 'package:voice_frontend/backend/spaces_client.dart';

import 'support/live_gateway_harness.dart';

/// Phase-5 space roles E2E (API-level): bootstrap, join assigns Member, invite permission gate.
void main() {
  test('space roles: hierarchy, join member role, delegate invite permission', () async {
    final probe = await probeLiveGateway();
    expect(
      probe,
      isA<LiveGatewayReady>(),
      reason: probe is LiveGatewayUnavailable ? probe.reason : null,
    );
    final ctx = (probe as LiveGatewayReady).context;

    final owner = await ctx.registerUser('space-roles-owner');
    final delegate = await ctx.registerUser('space-roles-delegate');
    final joiner = await ctx.registerUser('space-roles-joiner');
    final spaces = ctx.spacesClient();
    final roles = ctx.rolesClient();

    final created = await spaces.createSpace(
      authorization: owner.authorizationHeader,
      name: 'Roles E2E',
    );
    expect(created, isA<SpacesApiOk<VoiceSpace>>());
    final spaceId = (created as SpacesApiOk<VoiceSpace>).data.id;

    final listed = await roles.listRoles(
      authorization: owner.authorizationHeader,
      spaceId: spaceId,
    );
    expect(listed, isA<RolesApiOk<List<SpaceRole>>>());
    final spaceRoles = (listed as RolesApiOk<List<SpaceRole>>).data;
    final roleNames = spaceRoles.map((r) => r.name).toSet();
    expect(roleNames, containsAll(['Owner', 'Admin', 'Moderator', 'Member', 'Guest']));

    final delegateJoin = await spaces.createInvite(
      authorization: owner.authorizationHeader,
      spaceId: spaceId,
    );
    expect(delegateJoin, isA<SpacesApiOk<SpaceInvite>>());
    await spaces.joinByInvite(
      authorization: delegate.authorizationHeader,
      code: (delegateJoin as SpacesApiOk<SpaceInvite>).data.code,
    );

    final invite = await spaces.createInvite(
      authorization: owner.authorizationHeader,
      spaceId: spaceId,
    );
    expect(invite, isA<SpacesApiOk<SpaceInvite>>());
    final code = (invite as SpacesApiOk<SpaceInvite>).data.code;

    final joined = await spaces.joinByInvite(
      authorization: joiner.authorizationHeader,
      code: code,
    );
    expect(joined, isA<SpacesApiOk<SpaceMembershipData>>());

    final joinerRoles = await roles.getMemberRoles(
      authorization: owner.authorizationHeader,
      spaceId: spaceId,
      profileId: joiner.activeProfileId,
    );
    expect(joinerRoles, isA<RolesApiOk<List<SpaceRole>>>());
    expect(
      (joinerRoles as RolesApiOk<List<SpaceRole>>).data.any((r) => r.name == 'Member'),
      isTrue,
    );

    final delegateInvite = await spaces.createInvite(
      authorization: delegate.authorizationHeader,
      spaceId: spaceId,
    );
    expect(delegateInvite, isA<SpacesApiFailure>());

    final adminRole = spaceRoles.firstWhere((r) => r.name == 'Admin');
    final assigned = await roles.assignRole(
      authorization: owner.authorizationHeader,
      spaceId: spaceId,
      profileId: delegate.activeProfileId,
      roleId: adminRole.id,
    );
    expect(assigned, isA<RolesApiOk<void>>());

    final delegateInviteAfter = await spaces.createInvite(
      authorization: delegate.authorizationHeader,
      spaceId: spaceId,
    );
    expect(delegateInviteAfter, isA<SpacesApiOk<SpaceInvite>>());
  });
}
