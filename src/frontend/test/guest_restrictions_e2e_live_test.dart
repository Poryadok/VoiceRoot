import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/auth_client.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/friends_client.dart';
import 'package:voice_frontend/backend/spaces_client.dart';

import 'support/live_gateway_harness.dart';

void main() {
  test('guest restrictions: DM, friends, space denied via gateway', () async {
    final probe = await probeLiveGateway();
    expect(
      probe,
      isA<LiveGatewayReady>(),
      reason: probe is LiveGatewayUnavailable ? probe.reason : null,
    );
    final ctx = (probe as LiveGatewayReady).context;

    final guestResult = await ctx.authClient().registerGuest(password: qaPassword);
    expect(guestResult, isA<AuthSessionOk>());
    final guest = (guestResult as AuthSessionOk).session;

    final regular = await ctx.registerUser('guest-restrict-regular');

    final dm = await ctx.chatsClient().createDm(
      authorization: guest.authorizationHeader,
      otherProfileId: regular.activeProfileId,
    );
    expect(dm, isA<ChatsApiFailure>());
    expect((dm as ChatsApiFailure).statusCode, 403);

    final invite = await ctx.friendsClient().sendFriendInvitation(
      authorization: guest.authorizationHeader,
      targetProfileId: regular.activeProfileId,
    );
    expect(invite, isA<FriendsApiFailure>());
    expect((invite as FriendsApiFailure).statusCode, 403);

    final space = await ctx.spacesClient().createSpace(
      authorization: guest.authorizationHeader,
      name: 'guest-space',
      description: 'denied',
    );
    expect(space, isA<SpacesApiFailure>());
    expect((space as SpacesApiFailure).statusCode, 403);
  }, skip: runLiveIntegration
      ? null
      : 'Opt in with --dart-define=VOICE_RUN_LIVE_INTEGRATION=true');
}
