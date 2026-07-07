import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/bots_client.dart';

import 'support/bot_live_harness.dart';
import 'support/live_gateway_harness.dart';

/// Full install + polling + /ping → pong flow (mirrors compose_bots_slash_live_test.go).
void main() {
  test(
    'botss: install polling bot and slash ping returns pong',
    () async {
      final probe = await probeLiveGateway();
      expect(
        probe,
        isA<LiveGatewayReady>(),
        reason: probe is LiveGatewayUnavailable ? probe.reason : null,
      );
      final ctx = (probe as LiveGatewayReady).context;

      final harness = await BotLiveHarness.setup(ctx: ctx, prefix: 'p16-slash');
      final bot = await harness.registerPollingBot('PingBot-${DateTime.now().microsecondsSinceEpoch}');
      await harness.installBot(bot.botId);

      final poller = harness.startPollingBot(bot.botToken);
      addTearDown(poller.stop);
      await harness.waitUntilBotOnline();

      final executed = await harness.executePing(bot.botId);
      expect(executed, isA<BotsApiOk<SlashInteractionOutcome>>());
      final outcome = (executed as BotsApiOk<SlashInteractionOutcome>).data;
      expect(outcome.content, 'pong');

      await harness.assertMessageInHistory('pong');
    },
    skip: runLiveIntegration
        ? null
        : 'Opt in with --dart-define=VOICE_RUN_LIVE_INTEGRATION=true',
  );
}
