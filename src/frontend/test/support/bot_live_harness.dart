import 'dart:async';
import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/bots_client.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/messages_client.dart';
import 'package:voice_frontend/backend/spaces_client.dart';

import 'live_gateway_harness.dart';

/// Live bot test helpers mirroring compose_phase16_bots_slash_live_test.go.
class BotLiveHarness {
  BotLiveHarness({
    required this.ctx,
    required this.owner,
    required this.spaceId,
    required this.chatId,
    required this.bots,
    required this.httpClient,
  });

  final LiveGatewayContext ctx;
  final AuthSession owner;
  final String spaceId;
  final String chatId;
  final VoiceBotsClient bots;
  final http.Client httpClient;

  static Future<BotLiveHarness> setup({
    required LiveGatewayContext ctx,
    required String prefix,
  }) async {
    final owner = await ctx.registerUser('$prefix-owner');
    final spaces = ctx.spacesClient();
    final chats = ctx.chatsClient();

    final spaceResult = await spaces.createSpace(
      authorization: owner.authorizationHeader,
      name: '$prefix space',
    );
    expect(spaceResult, isA<SpacesApiOk<VoiceSpace>>());
    final spaceId = (spaceResult as SpacesApiOk<VoiceSpace>).data.id;

    final groupResult = await chats.createGroup(
      authorization: owner.authorizationHeader,
      name: '$prefix-channel',
    );
    expect(groupResult, isA<ChatsApiOk<VoiceChat>>());
    final chatId = (groupResult as ChatsApiOk<VoiceChat>).data.id;

    final bots = VoiceBotsClient(gateway: ctx.gatewayHttp());
    return BotLiveHarness(
      ctx: ctx,
      owner: owner,
      spaceId: spaceId,
      chatId: chatId,
      bots: bots,
      httpClient: ctx.httpClient,
    );
  }

  Future<({String botId, String botToken})> registerPollingBot(String name) async {
    final base = ctx.config.baseUrl;
    final auth = owner.authorizationHeader;

    final regResp = await httpClient.post(
      Uri.parse('$base/api/v1/bots'),
      headers: {
        'Authorization': auth,
        'Content-Type': 'application/json',
      },
      body: jsonEncode({
        'name': name,
        'description': 'pong bot',
        'scopes_json': '["TEXT_CHAT_SEND_MESSAGES"]',
      }),
    );
    expect(regResp.statusCode, 200, reason: regResp.body);
    final regJson = jsonDecode(regResp.body) as Map<String, dynamic>;
    final botId = (regJson['bot'] as Map<String, dynamic>)['id'] as String;

    final manifest = '''
name: $name
description: pong
scopes: [TEXT_CHAT_SEND_MESSAGES]
commands:
  - name: ping
    description: ping
''';
    final manifestResp = await httpClient.post(
      Uri.parse('$base/api/v1/bots/$botId/manifest'),
      headers: {
        'Authorization': auth,
        'Content-Type': 'application/json',
      },
      body: jsonEncode({'manifest_yaml': manifest}),
    );
    expect(manifestResp.statusCode, 200, reason: manifestResp.body);

    var botToken = (regJson['tokenResponse'] as Map<String, dynamic>?)?['token'] as String?;
    if (botToken == null || botToken.trim().isEmpty) {
      final tokenResp = await httpClient.post(
        Uri.parse('$base/api/v1/bots/$botId/token/regenerate'),
        headers: {'Authorization': auth},
      );
      expect(tokenResp.statusCode, 200, reason: tokenResp.body);
      final tokenJson = jsonDecode(tokenResp.body) as Map<String, dynamic>;
      botToken = (tokenJson['token_response'] as Map<String, dynamic>)['token'] as String;
    }

    return (botId: botId, botToken: botToken);
  }

  Future<void> installBot(String botId) async {
    final result = await bots.installBotInSpace(
      authorization: owner.authorizationHeader,
      botId: botId,
      spaceId: spaceId,
      allowedChats: [(id: chatId, type: 'CHAT_TYPE_GROUP')],
    );
    expect(result, isA<BotsApiOk<String>>(), reason: '$result');
  }

  PollingBotSession startPollingBot(String botToken) {
    final session = PollingBotSession(
      httpClient: httpClient,
      baseUrl: ctx.config.baseUrl,
      botToken: botToken,
    );
    session.start();
    return session;
  }

  Future<BotsApiResult<SlashInteractionOutcome>> executePing(String botId) {
    return bots.executeSlashInteraction(
      authorization: owner.authorizationHeader,
      chatId: chatId,
      chatType: 'CHAT_TYPE_GROUP',
      botId: botId,
      commandName: 'ping',
    );
  }

  Future<void> assertMessageInHistory(String content) async {
    final messages = ctx.messagesClient();
    final deadline = DateTime.now().add(const Duration(seconds: 15));
    while (DateTime.now().isBefore(deadline)) {
      final listed = await messages.getMessages(
        authorization: owner.authorizationHeader,
        chatId: chatId,
      );
      if (listed is MessagesApiOk<MessageListData>) {
        for (final msg in listed.data.messages) {
          if (msg.content.trim() == content) {
            return;
          }
        }
      }
      await Future<void>.delayed(const Duration(milliseconds: 200));
    }
    fail('message "$content" not found in chat $chatId history');
  }
}

class PollingBotSession {
  PollingBotSession({
    required this.httpClient,
    required this.baseUrl,
    required this.botToken,
  });

  final http.Client httpClient;
  final String baseUrl;
  final String botToken;

  Timer? _timer;
  bool _running = false;

  void start() {
    _running = true;
    _timer = Timer.periodic(const Duration(milliseconds: 100), (_) => _pollOnce());
  }

  void stop() {
    _running = false;
    _timer?.cancel();
    _timer = null;
  }

  Future<void> _pollOnce() async {
    if (!_running) return;
    final auth = 'Bot $botToken';
    try {
      final resp = await httpClient.get(
        Uri.parse('$baseUrl/api/v1/bots/me/interactions/poll'),
        headers: {'Authorization': auth},
      );
      if (resp.statusCode != 200) return;
      final parsed = jsonDecode(resp.body) as Map<String, dynamic>;
      final events = (parsed['events'] as List<dynamic>?) ?? const [];
      for (final raw in events) {
        final evt = raw as Map<String, dynamic>;
        final payload = jsonDecode(evt['payload_json'] as String) as Map<String, dynamic>;
        final token = payload['interaction_token'] as String?;
        if (token == null || token.isEmpty) continue;
        await httpClient.post(
          Uri.parse('$baseUrl/api/v1/bots/me/interactions/complete'),
          headers: {
            'Authorization': auth,
            'Content-Type': 'application/json',
          },
          body: jsonEncode({
            'interaction_token': token,
            'content': 'pong',
            'is_ephemeral': false,
          }),
        );
      }
    } catch (_) {
      // Best-effort polling loop for live tests.
    }
  }
}