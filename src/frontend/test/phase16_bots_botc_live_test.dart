import 'package:flutter_test/flutter_test.dart';

import 'support/bot_live_harness.dart';
import 'support/live_gateway_harness.dart';

/// BOT-C bot-token routes: presence, member list, create chat (mirrors Go TestComposePhase16BotsBotCRoutes_live).
void main() {
  test(
    'phase 16 bots BOT-C: presence, members, create chat',
    () async {
      final probe = await probeLiveGateway();
      expect(
        probe,
        isA<LiveGatewayReady>(),
        reason: probe is LiveGatewayUnavailable ? probe.reason : null,
      );
      final ctx = (probe as LiveGatewayReady).context;

      final harness = await BotLiveHarness.setup(ctx: ctx, prefix: 'p16-botc');
      final bot = await harness.registerBotWithScopes(
        name: 'BotCRoutes-${DateTime.now().microsecondsSinceEpoch}',
        scopes: const [
          'TEXT_CHAT_SEND_MESSAGES',
          'SPACE_VIEW_MEMBER_LIST',
          'TEXT_CHAT_CREATE_IN_SPACE',
        ],
      );
      await harness.installBotWithScopes(bot.botId);

      await harness.touchPresence(bot.botToken);
      await harness.assertBotOnlineInSlashMenu();
      await harness.assertBotOnlineInInstalledList(bot.botId);

      final members = await harness.listSpaceMembersForBot(bot.botToken);
      expect(members, contains(harness.owner.activeProfileId));

      final chatId = await harness.createBotChat(
        botToken: bot.botToken,
        name: 'bot-created-${DateTime.now().microsecondsSinceEpoch}',
      );
      expect(chatId, isNotEmpty);
    },
    skip: runLiveIntegration
        ? null
        : 'Opt in with --dart-define=VOICE_RUN_LIVE_INTEGRATION=true',
  );
}
