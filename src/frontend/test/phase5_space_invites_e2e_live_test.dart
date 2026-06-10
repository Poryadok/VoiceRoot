import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/spaces_client.dart';

import 'support/live_gateway_harness.dart';

/// Phase-5 space invites E2E (API-level): create → join → list spaces.
void main() {
  test('space invites: create, join, both users see space', () async {
    final probe = await probeLiveGateway();
    expect(
      probe,
      isA<LiveGatewayReady>(),
      reason: probe is LiveGatewayUnavailable ? probe.reason : null,
    );
    final ctx = (probe as LiveGatewayReady).context;

    final owner = await ctx.registerUser('space-invite-owner');
    final joiner = await ctx.registerUser('space-invite-joiner');
    final spaces = ctx.spacesClient();

    final created = await spaces.createSpace(
      authorization: owner.authorizationHeader,
      name: 'Invite E2E',
    );
    expect(created, isA<SpacesApiOk<VoiceSpace>>());
    final spaceId = (created as SpacesApiOk<VoiceSpace>).data.id;

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
    expect(
      (joined as SpacesApiOk<SpaceMembershipData>).data.spaceId,
      spaceId,
    );

    final joinerSpaces = await spaces.listMySpaces(
      authorization: joiner.authorizationHeader,
    );
    expect(joinerSpaces, isA<SpacesApiOk<SpaceListData>>());
    final list = (joinerSpaces as SpacesApiOk<SpaceListData>).data.spaces;
    expect(list.any((s) => s.id == spaceId), isTrue);
  });
}
