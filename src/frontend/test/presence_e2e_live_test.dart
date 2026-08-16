import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/users_client.dart';

import 'support/live_gateway_harness.dart';

void main() {
  test('presence API returns status for peer profile', () async {
    final probe = await probeLiveGateway();
    expect(probe, isA<LiveGatewayReady>());
    final ctx = (probe as LiveGatewayReady).context;
    final a = await ctx.registerUser('presence-a');
    final b = await ctx.registerUser('presence-b');
    final users = VoiceUsersClient(gateway: ctx.gatewayHttp());
    final result = await users.getPresence(
      authorization: a.authorizationHeader,
      profileId: b.activeProfileId,
    );
    expect(result, isA<UsersApiOk<VoicePresence>>());
  }, skip: runLiveIntegration ? null : 'opt-in live');

  test('PR-02 DND custom status and invisible look offline to peer', () async {
    final probe = await probeLiveGateway();
    expect(probe, isA<LiveGatewayReady>());
    final ctx = (probe as LiveGatewayReady).context;
    final a = await ctx.registerUser('presence-dnd-a');
    final b = await ctx.registerUser('presence-dnd-b');
    final users = VoiceUsersClient(gateway: ctx.gatewayHttp());

    await ctx.inviteAndAcceptFriends(a, b);

    final dnd = await users.updatePresence(
      authorization: b.authorizationHeader,
      status: 'dnd',
      customStatus: 'focus',
    );
    expect(dnd, isA<UsersApiOk<void>>());

    UsersApiOk<VoicePresence>? peerDnd;
    for (var i = 0; i < 40; i++) {
      final got = await users.getPresence(
        authorization: a.authorizationHeader,
        profileId: b.activeProfileId,
      );
      if (got is UsersApiOk<VoicePresence> &&
          got.data.status == 'dnd' &&
          got.data.customStatus == 'focus') {
        peerDnd = got;
        break;
      }
      await Future<void>.delayed(const Duration(milliseconds: 400));
    }
    expect(peerDnd, isNotNull, reason: 'friend should see DND + custom');

    final invis = await users.updatePresence(
      authorization: b.authorizationHeader,
      status: 'invisible',
    );
    expect(invis, isA<UsersApiOk<void>>());

    UsersApiOk<VoicePresence>? peerOffline;
    for (var i = 0; i < 40; i++) {
      final got = await users.getPresence(
        authorization: a.authorizationHeader,
        profileId: b.activeProfileId,
      );
      if (got is UsersApiOk<VoicePresence> && got.data.appearsOffline) {
        peerOffline = got;
        break;
      }
      await Future<void>.delayed(const Duration(milliseconds: 400));
    }
    expect(peerOffline, isNotNull, reason: 'invisible must look offline to peer');

    final self = await users.getPresence(
      authorization: b.authorizationHeader,
      profileId: b.activeProfileId,
    );
    expect(self, isA<UsersApiOk<VoicePresence>>());
    expect((self as UsersApiOk<VoicePresence>).data.status, 'invisible');
  }, skip: runLiveIntegration ? null : 'opt-in live');
}
