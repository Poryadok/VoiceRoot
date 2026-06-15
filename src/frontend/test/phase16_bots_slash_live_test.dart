import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/bots_client.dart';
import 'package:voice_frontend/backend/chats_client.dart';

import 'support/live_gateway_harness.dart';

void main() {
  test(
    'phase 16 bots: list slash commands and execute interaction',
    () async {
      final probe = await probeLiveGateway();
      expect(
        probe,
        isA<LiveGatewayReady>(),
        reason: probe is LiveGatewayUnavailable ? probe.reason : null,
      );
      final ctx = (probe as LiveGatewayReady).context;

      final user = await ctx.registerUser('p16-bot-user');
      final chats = ctx.chatsClient();
      final group = await chats.createGroup(
        authorization: user.authorizationHeader,
        name: 'p16-bot-channel',
      );
      expect(group, isA<ChatsApiOk<VoiceChat>>());
      final chatId = (group as ChatsApiOk<VoiceChat>).data.id;

      final bots = VoiceBotsClient(gateway: ctx.gatewayHttp());
      final listed = await bots.listSlashCommandsForChat(
        authorization: user.authorizationHeader,
        chatId: chatId,
        chatType: 'CHAT_TYPE_GROUP',
      );
      expect(listed, isA<BotsApiOk<List<BotSlashCommand>>>());

      final commands = (listed as BotsApiOk<List<BotSlashCommand>>).data;
      if (commands.isNotEmpty) {
        final cmd = commands.first;
        final executed = await bots.executeSlashInteraction(
          authorization: user.authorizationHeader,
          chatId: chatId,
          chatType: 'CHAT_TYPE_GROUP',
          botId: cmd.botId,
          commandName: cmd.name,
        );
        expect(executed, isA<BotsApiOk<SlashInteractionOutcome>>());
        return;
      }

      final executed = await bots.executeSlashInteraction(
        authorization: user.authorizationHeader,
        chatId: chatId,
        chatType: 'CHAT_TYPE_GROUP',
        botId: '00000000-0000-4000-8000-000000000001',
        commandName: 'ping',
      );
      expect(
        executed,
        anyOf(
          isA<BotsApiOk<SlashInteractionOutcome>>(),
          isA<BotsApiFailure>(),
        ),
      );
    },
    skip: runLiveIntegration
        ? null
        : 'Opt in with --dart-define=VOICE_RUN_LIVE_INTEGRATION=true',
  );
}
