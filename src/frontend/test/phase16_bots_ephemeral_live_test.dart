import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/bots_client.dart';

import 'support/bot_live_harness.dart';
import 'support/live_gateway_harness.dart';

/// Ephemeral slash: invoker sees content, no persisted history; member B sees nothing.
void main() {
  test(
    'phase 16 bots: ephemeral slash visible only to invoker',
    () async {
      final probe = await probeLiveGateway();
      expect(
        probe,
        isA<LiveGatewayReady>(),
        reason: probe is LiveGatewayUnavailable ? probe.reason : null,
      );
      final ctx = (probe as LiveGatewayReady).context;

      final member = await ctx.registerUser('p16-eph-member');
      final harness = await BotLiveHarness.setup(ctx: ctx, prefix: 'p16-eph');
      await ctx.chatsClient().addGroupMembers(
        authorization: harness.owner.authorizationHeader,
        chatId: harness.chatId,
        profileIds: [member.activeProfileId],
      );

      final bot = await harness.registerPollingBot(
        'EphBot-${DateTime.now().microsecondsSinceEpoch}',
      );
      await harness.installBot(bot.botId);

      final poller = harness.startEphemeralPollingBot(bot.botToken);
      addTearDown(poller.stop);

      final executed = await harness.executePing(bot.botId);
      expect(executed, isA<BotsApiOk<SlashInteractionOutcome>>());
      final outcome = (executed as BotsApiOk<SlashInteractionOutcome>).data;
      expect(outcome.isEphemeral, isTrue, reason: 'invoker must receive is_ephemeral');
      expect(outcome.content, 'pong-ephemeral');
      expect(outcome.message, isNull, reason: 'ephemeral slash must not return persisted message');

      await harness.assertMessageNotInHistory('pong-ephemeral');
      await harness.assertMessageNotInHistoryFor(
        authorization: member.authorizationHeader,
        content: 'pong-ephemeral',
      );
    },
    skip: runLiveIntegration
        ? null
        : 'Opt in with --dart-define=VOICE_RUN_LIVE_INTEGRATION=true',
  );
}
