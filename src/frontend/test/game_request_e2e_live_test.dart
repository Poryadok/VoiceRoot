import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/matchmaking_client.dart';

import 'support/live_gateway_harness.dart';

/// GC-02: SubmitGameRequest → pending_moderation (not in public catalog).
void main() {
  test(
    'submit game request pending moderation (GC-02)',
    () async {
      final probe = await probeLiveGateway();
      expect(
        probe,
        isA<LiveGatewayReady>(),
        reason: probe is LiveGatewayUnavailable ? probe.reason : null,
      );
      final ctx = (probe as LiveGatewayReady).context;
      final user = await ctx.registerUser('gc02-user');

      final mm = VoiceMatchmakingClient(gateway: ctx.gatewayHttp());
      final name = 'Flutter Game ${DateTime.now().microsecondsSinceEpoch}';
      const configJson =
          '{"regions":["eu"],"modes":[{"name":"5v5","slots":10,"party_size_min":1,"party_size_max":5,"roles":[{"name":"Carry","required":true}],"ranks":[{"name":"Bronze","value":0}]}]}';

      final submitted = await mm.submitGameRequest(
        authorization: user.authorizationHeader,
        name: name,
        configJson: configJson,
      );
      expect(submitted, isA<MatchmakingApiOk<CatalogGame>>());
      final game = (submitted as MatchmakingApiOk<CatalogGame>).data;
      expect(game.status, 'pending_moderation');
      expect(game.name, name);

      final catalog = await mm.listGames(authorization: user.authorizationHeader);
      expect(catalog, isA<MatchmakingApiOk<GameListData>>());
      final names = (catalog as MatchmakingApiOk<GameListData>)
          .data
          .games
          .map((g) => g.name)
          .toList();
      expect(names, isNot(contains(name)));
    },
    skip: runLiveIntegration
        ? null
        : 'Opt in with --dart-define=VOICE_RUN_LIVE_INTEGRATION=true',
  );
}
