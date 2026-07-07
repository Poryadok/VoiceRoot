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
}
