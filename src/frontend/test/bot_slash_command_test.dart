import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/bots_client.dart';

void main() {
  test('BotSlashCommand uses group name in display and API command', () {
    const cmd = BotSlashCommand(
      botId: 'bot-1',
      botName: 'QueueBot',
      name: 'join',
      description: 'Join queue',
      groupName: 'queue',
    );
    expect(cmd.fullCommandName, 'queue join');
    expect(cmd.displayName, '/queue join');
    expect(cmd.menuGroupKey, 'QueueBot::queue');
  });

  test('BotSlashCommand includes online flag', () {
    const cmd = BotSlashCommand(
      botId: 'b',
      botName: 'Bot',
      name: 'ping',
      description: 'd',
      online: false,
    );
    expect(cmd.online, isFalse);
  });
}
