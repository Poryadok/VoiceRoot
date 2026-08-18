import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/matchmaking_client.dart';
import 'package:voice_frontend/backend/spaces_client.dart';

import 'support/live_gateway_harness.dart';

/// MM-08: space-scoped StartSpaceQueue + non-member deny.
void main() {
  test(
    'space matchmaking queue isolation (MM-08)',
    () async {
      final probe = await probeLiveGateway();
      expect(
        probe,
        isA<LiveGatewayReady>(),
        reason: probe is LiveGatewayUnavailable ? probe.reason : null,
      );
      final ctx = (probe as LiveGatewayReady).context;

      final owner = await ctx.registerUser('mm08-owner');
      final member = await ctx.registerUser('mm08-member');
      final outsider = await ctx.registerUser('mm08-out');

      final spaces = ctx.spacesClient();
      final space = await spaces.createSpace(
        authorization: owner.authorizationHeader,
        name: 'Space MM Flutter',
      );
      expect(space, isA<SpacesApiOk<VoiceSpace>>());
      final spaceId = (space as SpacesApiOk<VoiceSpace>).data.id;

      final invite = await spaces.createInvite(
        authorization: owner.authorizationHeader,
        spaceId: spaceId,
      );
      expect(invite, isA<SpacesApiOk<SpaceInvite>>());
      final code = (invite as SpacesApiOk<SpaceInvite>).data.code;

      final joined = await spaces.joinByInvite(
        authorization: member.authorizationHeader,
        code: code,
      );
      expect(joined, isA<SpacesApiOk<SpaceMembershipData>>());

      final mmCfg = await ctx.httpClient.patch(
        ctx.gatewayHttp().resolve('/api/v1/spaces/$spaceId/matchmaking/config'),
        headers: {
          'Authorization': owner.authorizationHeader,
          'Content-Type': 'application/json',
        },
        body: jsonEncode({'mm_config_json': '{"enabled":true}'}),
      );
      expect(mmCfg.statusCode, 200, reason: mmCfg.body);

      final mm = VoiceMatchmakingClient(gateway: ctx.gatewayHttp());
      final games = await mm.listGames(authorization: owner.authorizationHeader);
      expect(games, isA<MatchmakingApiOk<GameListData>>());
      final duo = (games as MatchmakingApiOk<GameListData>).data.games
          .firstWhere((g) => g.name == 'MM Duo Live');

      final ownerQ = await mm.startSpaceQueue(
        authorization: owner.authorizationHeader,
        spaceId: spaceId,
        gameId: duo.id,
        mode: 'Duo',
        criteria: const {'region': 'eu'},
      );
      expect(ownerQ, isA<MatchmakingApiOk<SearchSessionData>>(), reason: ownerQ is MatchmakingApiFailure ? '${ownerQ.message} ${ownerQ.statusCode}' : '$ownerQ');
      final ownerSession = (ownerQ as MatchmakingApiOk<SearchSessionData>).data;
      expect(ownerSession.status, 'searching');
      expect(ownerSession.spaceId, spaceId);

      final memberQ = await mm.startSpaceQueue(
        authorization: member.authorizationHeader,
        spaceId: spaceId,
        gameId: duo.id,
        mode: 'Duo',
        criteria: const {'region': 'eu'},
      );
      expect(memberQ, isA<MatchmakingApiOk<SearchSessionData>>());
      expect(
        (memberQ as MatchmakingApiOk<SearchSessionData>).data.spaceId,
        spaceId,
      );

      final denied = await mm.startSpaceQueue(
        authorization: outsider.authorizationHeader,
        spaceId: spaceId,
        gameId: duo.id,
        mode: 'Duo',
        criteria: const {'region': 'eu'},
      );
      expect(denied, isA<MatchmakingApiFailure>());
      expect((denied as MatchmakingApiFailure).statusCode, 403);
    },
    skip: runLiveIntegration
        ? null
        : 'Opt in with --dart-define=VOICE_RUN_LIVE_INTEGRATION=true',
  );
}
