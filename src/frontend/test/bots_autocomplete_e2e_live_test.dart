import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/bots_client.dart';

import 'support/bot_live_harness.dart';
import 'support/live_gateway_harness.dart';

/// BT-07: autocomplete live + owner command catalog (mirrors compose_bots_autocomplete_live_test.go).
void main() {
  test(
    'bots: autocomplete choices + command catalog with subcommands',
    () async {
      final probe = await probeLiveGateway();
      expect(
        probe,
        isA<LiveGatewayReady>(),
        reason: probe is LiveGatewayUnavailable ? probe.reason : null,
      );
      final ctx = (probe as LiveGatewayReady).context;

      final harness = await BotLiveHarness.setup(ctx: ctx, prefix: 'bt07-ac');
      final bot = await harness.registerAutocompleteBot(
        'StatsBot-${DateTime.now().microsecondsSinceEpoch}',
      );
      await harness.installBot(bot.botId);

      final catalog = await harness.fetchOwnerCommandCatalog(bot.botId);
      expect(catalog.any((c) => c['name'] == 'stats'), isTrue);
      expect(catalog.any((c) => c['name'] == 'queue join'), isTrue);
      final stats = catalog.firstWhere((c) => c['name'] == 'stats');
      final options = (stats['options'] as List<dynamic>).cast<Map<String, dynamic>>();
      expect(options.any((o) => o['name'] == 'game' && o['autocomplete'] == true), isTrue);

      final poller = harness.startAutocompletePollingBot(bot.botToken);
      addTearDown(poller.stop);
      await harness.waitUntilBotOnline();

      final result = await harness.waitAutocompleteChoices(
        botId: bot.botId,
        commandName: 'stats',
        optionName: 'game',
        focusedValue: 'cs',
      );
      expect(result, isA<BotAutocompleteResult>());
      expect(result.choices, isNotEmpty);
      expect(result.choices.first.name, 'CS2');
      expect(result.choices.first.value, 'cs2');
    },
    skip: runLiveIntegration
        ? null
        : 'Opt in with --dart-define=VOICE_RUN_LIVE_INTEGRATION=true',
  );
}
