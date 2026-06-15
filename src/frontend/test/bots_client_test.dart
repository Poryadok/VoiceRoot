import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/bots_client.dart';
import 'package:voice_frontend/backend/gateway_config.dart';

import 'support/gateway_test_client.dart';

void main() {
  const config = GatewayConfig(baseUrl: 'http://api.test');
  const auth = 'Bearer access-token';

  group('VoiceBotsClient.listSlashCommandsForChat', () {
    test('GET /api/v1/bots/commands parses slash commands', () async {
      final mock = MockClient((req) async {
        expect(req.method, 'GET');
        expect(req.url.path, '/api/v1/bots/commands');
        expect(req.url.queryParameters['chat_id'], 'chat-1');
        expect(req.url.queryParameters['chat_type'], 'CHAT_TYPE_CHANNEL');
        expect(req.headers['Authorization'], auth);
        return http.Response(
          jsonEncode({
            'commands': [
              {
                'bot_id': 'bot-1',
                'bot_name': 'PingBot',
                'name': 'ping',
                'description': 'Replies with pong',
              },
            ],
          }),
          200,
        );
      });
      final client = VoiceBotsClient(
        gateway: gatewayHttpForTest(mock, config: config),
      );
      final result = await client.listSlashCommandsForChat(
        authorization: auth,
        chatId: 'chat-1',
        chatType: 'CHAT_TYPE_CHANNEL',
      );
      expect(result, isA<BotsApiOk<List<BotSlashCommand>>>());
      final commands = (result as BotsApiOk<List<BotSlashCommand>>).data;
      expect(commands, hasLength(1));
      expect(commands.first.botId, 'bot-1');
      expect(commands.first.name, 'ping');
      expect(commands.first.displayName, '/ping');
    });
  });

  group('VoiceBotsClient.executeSlashInteraction', () {
    test('POST /api/v1/bots/interactions parses ephemeral reply', () async {
      String? body;
      final mock = MockClient((req) async {
        expect(req.method, 'POST');
        expect(req.url.path, '/api/v1/bots/interactions');
        body = req.body;
        return http.Response(
          jsonEncode({
            'interaction_token': 'token-1',
            'content': 'pong',
            'is_ephemeral': true,
          }),
          200,
        );
      });
      final client = VoiceBotsClient(
        gateway: gatewayHttpForTest(mock, config: config),
      );
      final result = await client.executeSlashInteraction(
        authorization: auth,
        chatId: 'chat-1',
        chatType: 'CHAT_TYPE_CHANNEL',
        botId: 'bot-1',
        commandName: 'ping',
      );
      expect(result, isA<BotsApiOk<SlashInteractionOutcome>>());
      final data = (result as BotsApiOk<SlashInteractionOutcome>).data;
      expect(data.content, 'pong');
      expect(data.isEphemeral, isTrue);
      expect(data.isBotTimeout, isFalse);
      final decoded = jsonDecode(body!) as Map<String, dynamic>;
      expect(decoded['bot_id'], 'bot-1');
      expect(decoded['command_name'], 'ping');
      expect(decoded['chat']['id'], 'chat-1');
    });

    test('maps bot_timeout in response body', () async {
      final mock = MockClient((req) async {
        return http.Response(
          jsonEncode({
            'interaction_token': 'token-2',
            'error_code': 'bot_timeout',
            'error_message': 'Bot did not respond in time. Try again later.',
          }),
          200,
        );
      });
      final client = VoiceBotsClient(
        gateway: gatewayHttpForTest(mock, config: config),
      );
      final result = await client.executeSlashInteraction(
        authorization: auth,
        chatId: 'chat-1',
        chatType: 'CHAT_TYPE_GROUP',
        botId: 'bot-1',
        commandName: 'slow',
      );
      expect(result, isA<BotsApiOk<SlashInteractionOutcome>>());
      final data = (result as BotsApiOk<SlashInteractionOutcome>).data;
      expect(data.isBotTimeout, isTrue);
      expect(data.errorCode, kBotTimeoutErrorCode);
    });
  });

  group('VoiceBotsClient.listBotsInChat', () {
    test('GET /api/v1/bots/chats/{id} parses chat bot settings', () async {
      final mock = MockClient((req) async {
        expect(req.method, 'GET');
        expect(req.url.path, '/api/v1/bots/chats/chat-1');
        expect(req.url.queryParameters['space_id'], 'space-1');
        return http.Response(
          jsonEncode({
            'bots': [
              {
                'bot': {
                  'id': 'bot-1',
                  'name': 'PingBot',
                  'description': '',
                  'scopes_json': '[]',
                  'status': 'live',
                },
                'enabled': true,
                'whitelisted': true,
              },
            ],
          }),
          200,
        );
      });
      final client = VoiceBotsClient(
        gateway: gatewayHttpForTest(mock, config: config),
      );
      final result = await client.listBotsInChat(
        authorization: auth,
        chatId: 'chat-1',
        chatType: 'CHAT_TYPE_CHANNEL',
        spaceId: 'space-1',
      );
      expect(result, isA<BotsApiOk<List<ChatBotSettings>>>());
      final bots = (result as BotsApiOk<List<ChatBotSettings>>).data;
      expect(bots.first.bot.name, 'PingBot');
      expect(bots.first.enabled, isTrue);
    });
  });

  group('VoiceBotsClient.listSlashCommandsForChat online', () {
    test('parses online flag on slash commands', () async {
      final mock = MockClient((req) async {
        return http.Response(
          jsonEncode({
            'commands': [
              {
                'bot_id': 'bot-1',
                'bot_name': 'DownBot',
                'name': 'slow',
                'description': 'Slow',
                'online': false,
              },
            ],
          }),
          200,
        );
      });
      final client = VoiceBotsClient(
        gateway: gatewayHttpForTest(mock, config: config),
      );
      final result = await client.listSlashCommandsForChat(
        authorization: auth,
        chatId: 'chat-1',
        chatType: 'CHAT_TYPE_CHANNEL',
      );
      final commands = (result as BotsApiOk<List<BotSlashCommand>>).data;
      expect(commands.first.online, isFalse);
    });
  });
}
