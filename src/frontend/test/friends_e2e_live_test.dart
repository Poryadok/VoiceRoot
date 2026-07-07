import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/friends_client.dart';

import 'support/live_gateway_harness.dart';

void main() {
  test(
    'friends: invite, accept, both list each other',
    () async {
      final probe = await probeLiveGateway();
      expect(
        probe,
        isA<LiveGatewayReady>(),
        reason: probe is LiveGatewayUnavailable ? probe.reason : null,
      );
      final ctx = (probe as LiveGatewayReady).context;

      final sessionA = await ctx.registerUser('friends-a');
      final sessionB = await ctx.registerUser('friends-b');
      await ctx.inviteAndAcceptFriends(sessionA, sessionB);

      final friends = ctx.friendsClient();
      final listA = await friends.listFriends(
        authorization: sessionA.authorizationHeader,
      );
      expect(listA, isA<FriendsApiOk<FriendsListData>>());
      final idsA = (listA as FriendsApiOk<FriendsListData>).data.friends;
      expect(idsA, contains(sessionB.activeProfileId));

      final listB = await friends.listFriends(
        authorization: sessionB.authorizationHeader,
      );
      expect(listB, isA<FriendsApiOk<FriendsListData>>());
      final idsB = (listB as FriendsApiOk<FriendsListData>).data.friends;
      expect(idsB, contains(sessionA.activeProfileId));
    },
    skip: runLiveIntegration
        ? null
        : 'Opt in with --dart-define=VOICE_RUN_LIVE_INTEGRATION=true',
  );
}
