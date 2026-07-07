import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/bots_client.dart';

import 'support/bot_live_harness.dart';
import 'support/live_gateway_harness.dart';

/// Ephemeral slash: invoker sees content, no persisted history; member B sees nothing.
void main() {
  test(
    'botss: ephemeral slash visible only to invoker',
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
      final addMemberResp = await harness.httpClient.post(
        Uri.parse('${ctx.config.baseUrl}/api/v1/chats/${harness.chatId}/members'),
        headers: {
          'Authorization': harness.owner.authorizationHeader,
          'Content-Type': 'application/json',
        },
        body: '{"profile_ids":["${member.activeProfileId}"]}',
      );
      expect(addMemberResp.statusCode, 204, reason: addMemberResp.body);

      final bot = await harness.registerPollingBot(
        'EphBot-${DateTime.now().microsecondsSinceEpoch}',
      );
      await harness.installBot(bot.botId);

      final poller = harness.startEphemeralPollingBot(bot.botToken);
      addTearDown(poller.stop);
      await harness.waitUntilBotOnline();

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
