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

/// Live bot test helpers mirroring compose_bots_slash_live_test.go.
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
    final harness = BotLiveHarness(
      ctx: ctx,
      owner: owner,
      spaceId: spaceId,
      chatId: chatId,
      bots: bots,
      httpClient: ctx.httpClient,
    );
    await harness.ensureGroupReadyForBot();
    return harness;
  }

  Future<({String botId, String botToken})> registerBotWithScopes({
    required String name,
    required List<String> scopes,
  }) async {
    final base = ctx.config.baseUrl;
    final auth = owner.authorizationHeader;
    final scopesJSON = jsonEncode(scopes);

    final regResp = await httpClient.post(
      Uri.parse('$base/api/v1/bots'),
      headers: {
        'Authorization': auth,
        'Content-Type': 'application/json',
      },
      body: jsonEncode({
        'name': name,
        'description': 'BOT-C bot',
        'scopes_json': scopesJSON,
      }),
    );
    expect(regResp.statusCode, 200, reason: regResp.body);
    final regJson = jsonDecode(regResp.body) as Map<String, dynamic>;
    final botId = (regJson['bot'] as Map<String, dynamic>)['id'] as String;

    final scopesYAML = scopes.join(', ');
    final manifest = '''
name: $name
description: BOT-C
scopes: [$scopesYAML]
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

  Future<void> ensureGroupReadyForBot() async {
    final memberB = await ctx.registerUser('bot-member-b');
    final memberC = await ctx.registerUser('bot-member-c');
    await ctx.allowOpenGamingPrivacyMany([memberB, memberC]);
    final resp = await httpClient.post(
      Uri.parse('${ctx.config.baseUrl}/api/v1/chats/$chatId/members'),
      headers: {
        'Authorization': owner.authorizationHeader,
        'Content-Type': 'application/json',
      },
      body: jsonEncode({
        'profile_ids': [memberB.activeProfileId, memberC.activeProfileId],
      }),
    );
    expect(resp.statusCode, 204, reason: resp.body);
  }

  Future<void> _installBotRaw(String botId) async {
    final resp = await httpClient.post(
      Uri.parse('${ctx.config.baseUrl}/api/v1/bots/$botId/spaces/$spaceId/install'),
      headers: {
        'Authorization': owner.authorizationHeader,
        'Content-Type': 'application/json',
      },
      body: jsonEncode({
        'allowed_chats': [
          {'id': chatId, 'type': 'CHAT_TYPE_GROUP'},
        ],
      }),
    );
    expect(resp.statusCode, 200, reason: resp.body);
  }

  Future<void> installBotWithScopes(String botId) async {
    await _installBotRaw(botId);
  }

  Future<void> touchPresence(String botToken) async {
    final resp = await httpClient.post(
      Uri.parse('${ctx.config.baseUrl}/api/v1/bots/me/presence'),
      headers: {'Authorization': 'Bot $botToken'},
    );
    expect(resp.statusCode, 204, reason: resp.body);
  }

  Future<List<String>> listSpaceMembersForBot(String botToken) async {
    final resp = await httpClient.get(
      Uri.parse('${ctx.config.baseUrl}/api/v1/bots/me/spaces/$spaceId/members'),
      headers: {'Authorization': 'Bot $botToken'},
    );
    expect(resp.statusCode, 200, reason: resp.body);
    final parsed = jsonDecode(resp.body) as Map<String, dynamic>;
    return (parsed['profile_ids'] as List<dynamic>).cast<String>();
  }

  Future<String> createBotChat({
    required String botToken,
    required String name,
    String chatType = 'channel',
  }) async {
    final resp = await httpClient.post(
      Uri.parse('${ctx.config.baseUrl}/api/v1/bots/me/chats'),
      headers: {
        'Authorization': 'Bot $botToken',
        'Content-Type': 'application/json',
      },
      body: jsonEncode({
        'space_id': spaceId,
        'name': name,
        'chat_type': chatType,
      }),
    );
    expect(resp.statusCode, 200, reason: resp.body);
    final parsed = jsonDecode(resp.body) as Map<String, dynamic>;
    final chat = parsed['chat'] as Map<String, dynamic>;
    final id = chat['id'] as String;
    expect(id, isNotEmpty);
    return id;
  }

  Future<void> assertBotOnlineInSlashMenu() async {
    final listed = await bots.listSlashCommandsForChat(
      authorization: owner.authorizationHeader,
      chatId: chatId,
      chatType: 'CHAT_TYPE_GROUP',
    );
    expect(listed, isA<BotsApiOk<List<BotSlashCommand>>>(), reason: '$listed');
    final commands = (listed as BotsApiOk<List<BotSlashCommand>>).data;
    expect(commands, isNotEmpty);
    expect(commands.first.online, isTrue, reason: 'slash menu must show bot online after heartbeat');
  }

  Future<void> assertBotOnlineInInstalledList(String botId) async {
    final listed = await bots.listInstalledBots(
      authorization: owner.authorizationHeader,
      spaceId: spaceId,
    );
    expect(listed, isA<BotsApiOk<List<InstalledBotInfo>>>(), reason: '$listed');
    final botsList = (listed as BotsApiOk<List<InstalledBotInfo>>).data;
    final match = botsList.where((b) => b.bot.id == botId).toList();
    expect(match, isNotEmpty);
    expect(match.first.online, isTrue, reason: 'installed list must show bot online after heartbeat');
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

  Future<({String botId, String botToken})> registerAutocompleteBot(String name) async {
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
        'description': 'autocomplete bot',
        'scopes_json': '["TEXT_CHAT_SEND_MESSAGES"]',
      }),
    );
    expect(regResp.statusCode, 200, reason: regResp.body);
    final regJson = jsonDecode(regResp.body) as Map<String, dynamic>;
    final botId = (regJson['bot'] as Map<String, dynamic>)['id'] as String;

    const manifest = '''
name: StatsBot
description: autocomplete
scopes: [TEXT_CHAT_SEND_MESSAGES]
commands:
  - name: stats
    description: Show player stats
    options:
      - name: game
        type: string
        required: true
        autocomplete: true
  - name: queue
    description: Queue
    subcommands:
      - name: join
        description: Join
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

  Future<List<Map<String, dynamic>>> fetchOwnerCommandCatalog(String botId) async {
    final resp = await httpClient.get(
      Uri.parse('${ctx.config.baseUrl}/api/v1/bots/$botId/commands'),
      headers: {'Authorization': owner.authorizationHeader},
    );
    expect(resp.statusCode, 200, reason: resp.body);
    final parsed = jsonDecode(resp.body) as Map<String, dynamic>;
    final commandList = parsed['command_list'] as Map<String, dynamic>?;
    final commandsJson = commandList?['commands_json'] as String? ?? '[]';
    return (jsonDecode(commandsJson) as List<dynamic>).cast<Map<String, dynamic>>();
  }

  Future<BotAutocompleteResult> waitAutocompleteChoices({
    required String botId,
    required String commandName,
    required String optionName,
    required String focusedValue,
    Duration timeout = const Duration(seconds: 10),
  }) async {
    final deadline = DateTime.now().add(timeout);
    BotAutocompleteResult? last;
    while (DateTime.now().isBefore(deadline)) {
      final result = await bots.autocompleteOption(
        authorization: owner.authorizationHeader,
        chatId: chatId,
        chatType: 'CHAT_TYPE_GROUP',
        botId: botId,
        commandName: commandName,
        optionName: optionName,
        focusedValue: focusedValue,
      );
      expect(result, isA<BotsApiOk<BotAutocompleteResult>>(), reason: '$result');
      last = (result as BotsApiOk<BotAutocompleteResult>).data;
      if (last.choices.isNotEmpty) {
        return last;
      }
      if (!last.pending) {
        fail('autocomplete returned no choices and pending=false');
      }
      await Future<void>.delayed(const Duration(milliseconds: 200));
    }
    fail('timed out waiting for autocomplete choices (last=$last)');
  }

  PollingBotSession startAutocompletePollingBot(String botToken) {
    return PollingBotSession(
      httpClient: httpClient,
      baseUrl: ctx.config.baseUrl,
      botToken: botToken,
      handleAutocomplete: true,
    )..start();
  }

  Future<void> installBot(String botId) async {
    await _installBotRaw(botId);
  }

  Future<void> waitUntilBotOnline({Duration timeout = const Duration(seconds: 15)}) async {
    final deadline = DateTime.now().add(timeout);
    while (DateTime.now().isBefore(deadline)) {
      final listed = await bots.listSlashCommandsForChat(
        authorization: owner.authorizationHeader,
        chatId: chatId,
        chatType: 'CHAT_TYPE_GROUP',
      );
      if (listed is BotsApiOk<List<BotSlashCommand>> && listed.data.isNotEmpty) {
        if (listed.data.first.online) {
          return;
        }
      }
      await Future<void>.delayed(const Duration(milliseconds: 200));
    }
    fail('timed out waiting for bot online presence');
  }

  PollingBotSession startPollingBot(String botToken) {
    return PollingBotSession(
      httpClient: httpClient,
      baseUrl: ctx.config.baseUrl,
      botToken: botToken,
    )..start();
  }

  PollingBotSession startEphemeralPollingBot(String botToken) {
    return PollingBotSession(
      httpClient: httpClient,
      baseUrl: ctx.config.baseUrl,
      botToken: botToken,
      ephemeral: true,
    )..start();
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

  Future<void> assertMessageNotInHistory(String content) async {
    await assertMessageNotInHistoryFor(
      authorization: owner.authorizationHeader,
      content: content,
    );
  }

  Future<void> assertMessageNotInHistoryFor({
    required String authorization,
    required String content,
  }) async {
    final messages = ctx.messagesClient();
    final listed = await messages.getMessages(
      authorization: authorization,
      chatId: chatId,
    );
    expect(listed, isA<MessagesApiOk<MessageListData>>(), reason: '$listed');
    for (final msg in (listed as MessagesApiOk<MessageListData>).data.messages) {
      expect(
        msg.content.trim(),
        isNot(content),
        reason: 'ephemeral "$content" must not appear in chat $chatId history',
      );
    }
  }
}

class PollingBotSession {
  PollingBotSession({
    required this.httpClient,
    required this.baseUrl,
    required this.botToken,
    this.ephemeral = false,
    this.handleAutocomplete = false,
  });

  final http.Client httpClient;
  final String baseUrl;
  final String botToken;
  final bool ephemeral;
  final bool handleAutocomplete;

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
        final eventType = evt['event_type'] as String? ?? '';
        final payload = jsonDecode(evt['payload_json'] as String) as Map<String, dynamic>;
        final type = payload['type'] as String? ?? '';
        if (handleAutocomplete && (type == 'autocomplete' || eventType == 'autocomplete')) {
          final requestId = payload['request_id'] as String?;
          if (requestId == null || requestId.isEmpty) continue;
          await httpClient.post(
            Uri.parse('$baseUrl/api/v1/bots/me/autocomplete/complete'),
            headers: {
              'Authorization': auth,
              'Content-Type': 'application/json',
            },
            body: jsonEncode({
              'request_id': requestId,
              'choices': [
                {'name': 'CS2', 'value': 'cs2'},
                {'name': 'CS:GO', 'value': 'csgo'},
              ],
            }),
          );
          continue;
        }
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
            'content': ephemeral ? 'pong-ephemeral' : 'pong',
            'is_ephemeral': ephemeral,
          }),
        );
      }
    } catch (_) {
      // Best-effort polling loop for live tests.
    }
  }
}