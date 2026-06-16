import '../gen/voice/bot/v1/bot.pb.dart' as bot_pb;
import '../gen/voice/chat/v1/chat.pb.dart' as chat_pb;
import '../gen/voice/chat/v1/chat.pbenum.dart';
import '../gen/voice/messaging/v1/messaging.pb.dart' as messaging_pb;
import 'api_result.dart';
import 'gateway_http.dart';
import 'proto_mappers.dart';

const String kBotsMissingBaseUrlDetail = 'missing base URL';
const String kBotTimeoutErrorCode = 'bot_timeout';

sealed class BotsApiResult<T> {
  const BotsApiResult();
}

final class BotsApiOk<T> extends BotsApiResult<T> {
  const BotsApiOk(this.data);
  final T data;
}

final class BotsApiFailure extends BotsApiResult<Never> {
  const BotsApiFailure({
    required this.message,
    this.errorCode,
    this.statusCode,
  });

  final String message;
  final String? errorCode;
  final int? statusCode;
}

class BotSlashCommandOption {
  const BotSlashCommandOption({
    required this.name,
    required this.type,
    this.required = false,
    this.autocomplete = false,
  });

  final String name;
  final String type;
  final bool required;
  final bool autocomplete;

  factory BotSlashCommandOption.fromProto(bot_pb.SlashCommandOption opt) {
    return BotSlashCommandOption(
      name: opt.name,
      type: opt.type,
      required: opt.required,
      autocomplete: opt.autocomplete,
    );
  }
}

class BotAutocompleteChoice {
  const BotAutocompleteChoice({required this.name, required this.value});

  final String name;
  final String value;
}

class BotSlashCommand {
  const BotSlashCommand({
    required this.botId,
    required this.botName,
    required this.name,
    required this.description,
    this.groupName,
    this.options = const [],
    this.online = true,
  });

  final String botId;
  final String botName;
  final String name;
  final String description;
  final String? groupName;
  final List<BotSlashCommandOption> options;
  final bool online;

  String get fullCommandName =>
      groupName != null && groupName!.isNotEmpty ? '$groupName $name' : name;

  String get displayName => '/$fullCommandName';

  String get menuGroupKey => '$botName::${groupName ?? ''}';

  factory BotSlashCommand.fromProto(bot_pb.SlashCommand cmd) {
    return BotSlashCommand(
      botId: cmd.botId,
      botName: cmd.botName,
      name: cmd.name,
      description: cmd.description,
      groupName: cmd.hasGroupName() ? cmd.groupName : null,
      options: cmd.options.map(BotSlashCommandOption.fromProto).toList(),
      online: cmd.online,
    );
  }
}

class VoiceBotSummary {
  const VoiceBotSummary({
    required this.id,
    required this.name,
    required this.description,
    required this.scopesJson,
    this.actorProfileId,
    this.slug,
    this.avatarUrl,
  });

  final String id;
  final String name;
  final String description;
  final String scopesJson;
  final String? actorProfileId;
  final String? slug;
  final String? avatarUrl;

  factory VoiceBotSummary.fromProto(bot_pb.Bot bot) {
    return VoiceBotSummary(
      id: bot.id,
      name: bot.name,
      description: bot.description,
      scopesJson: bot.scopesJson,
      actorProfileId: bot.hasActorProfileId() ? bot.actorProfileId : null,
      slug: bot.hasSlug() ? bot.slug : null,
      avatarUrl: bot.hasAvatarUrl() ? bot.avatarUrl : null,
    );
  }
}

class InstalledBotInfo {
  const InstalledBotInfo({
    required this.bot,
    required this.installationId,
    required this.allowedChatIds,
    this.online = false,
  });

  final VoiceBotSummary bot;
  final String installationId;
  final List<String> allowedChatIds;
  final bool online;

  factory InstalledBotInfo.fromProto(bot_pb.InstalledBot installed) {
    return InstalledBotInfo(
      bot: VoiceBotSummary.fromProto(installed.bot),
      installationId: installed.installationId,
      allowedChatIds: installed.allowedChats.map((c) => c.id).toList(),
      online: installed.online,
    );
  }
}

class ChatBotSettings {
  const ChatBotSettings({
    required this.bot,
    required this.enabled,
    required this.whitelisted,
  });

  final VoiceBotSummary bot;
  final bool enabled;
  final bool whitelisted;

  factory ChatBotSettings.fromProto(bot_pb.ChatBotEntry entry) {
    return ChatBotSettings(
      bot: VoiceBotSummary.fromProto(entry.bot),
      enabled: entry.enabled,
      whitelisted: entry.whitelisted,
    );
  }
}

class SlashInteractionOutcome {
  const SlashInteractionOutcome({
    required this.interactionToken,
    this.content,
    this.isEphemeral = false,
    this.deferred = false,
    this.errorCode,
    this.errorMessage,
    this.message,
  });

  final String interactionToken;
  final String? content;
  final bool isEphemeral;
  final bool deferred;
  final String? errorCode;
  final String? errorMessage;
  final messaging_pb.Message? message;

  bool get isBotTimeout => errorCode == kBotTimeoutErrorCode;
}

/// HTTP client for Bot routes via API Gateway (`/api/v1/bots/**`).
class VoiceBotsClient {
  VoiceBotsClient({required GatewayHttpClient gateway}) : _gateway = gateway;

  final GatewayHttpClient _gateway;

  Future<BotsApiResult<List<BotSlashCommand>>> listSlashCommandsForChat({
    required String authorization,
    required String chatId,
    required String chatType,
  }) async {
    final result = await _gateway.getProto(
      _gateway.replace(
        path: '/api/v1/bots/commands',
        queryParameters: {
          'chat_id': chatId,
          'chat_type': chatType,
        },
      ),
      authorization: authorization,
      createEmpty: bot_pb.ListSlashCommandsForChatResponse.create,
    );
    return _map(
      result,
      (data) {
        final response = data as bot_pb.ListSlashCommandsForChatResponse;
        return response.commands
            .map(BotSlashCommand.fromProto)
            .toList(growable: false);
      },
    );
  }

  Future<BotsApiResult<SlashInteractionOutcome>> executeSlashInteraction({
    required String authorization,
    required String chatId,
    required String chatType,
    required String botId,
    required String commandName,
    String optionsJson = '{}',
  }) async {
    final result = await _gateway.postProto(
      uri: _gateway.resolve('/api/v1/bots/interactions'),
      authorization: authorization,
      body: bot_pb.ExecuteSlashInteractionRequest(
        chat: chat_pb.ChatRef(id: chatId, type: _chatTypeFromWire(chatType)),
        botId: botId,
        commandName: commandName,
        optionsJson: optionsJson,
      ),
      createEmpty: bot_pb.ExecuteSlashInteractionResponse.create,
    );
    return _map(
      result,
      (data) => _slashOutcomeFromProto(
        data as bot_pb.ExecuteSlashInteractionResponse,
      ),
    );
  }

  Future<BotsApiResult<List<BotAutocompleteChoice>>> autocompleteOption({
    required String authorization,
    required String chatId,
    required String chatType,
    required String botId,
    required String commandName,
    required String optionName,
    required String focusedValue,
    String optionsJson = '{}',
  }) async {
    final result = await _gateway.postProto(
      uri: _gateway.resolve('/api/v1/bots/autocomplete'),
      authorization: authorization,
      body: bot_pb.AutocompleteSlashOptionRequest(
        chat: chat_pb.ChatRef(id: chatId, type: _chatTypeFromWire(chatType)),
        botId: botId,
        commandName: commandName,
        optionName: optionName,
        focusedValue: focusedValue,
        optionsJson: optionsJson,
      ),
      createEmpty: bot_pb.AutocompleteSlashOptionResponse.create,
    );
    return _map(
      result,
      (data) {
        final response = data as bot_pb.AutocompleteSlashOptionResponse;
        return response.choices
            .map(
              (c) => BotAutocompleteChoice(name: c.name, value: c.value),
            )
            .toList(growable: false);
      },
    );
  }

  Future<BotsApiResult<List<VoiceBotSummary>>> listBots({
    required String authorization,
  }) async {
    final result = await _gateway.getProto(
      _gateway.resolve('/api/v1/bots'),
      authorization: authorization,
      createEmpty: bot_pb.ListBotsResponse.create,
    );
    return _map(
      result,
      (data) {
        final response = data as bot_pb.ListBotsResponse;
        return response.botList.bots
            .map(VoiceBotSummary.fromProto)
            .toList(growable: false);
      },
    );
  }

  Future<BotsApiResult<VoiceBotSummary>> getBot({
    required String authorization,
    required String botId,
  }) async {
    final result = await _gateway.getProto(
      _gateway.resolve('/api/v1/bots/$botId'),
      authorization: authorization,
      createEmpty: bot_pb.GetBotResponse.create,
    );
    return _map(
      result,
      (data) => VoiceBotSummary.fromProto(
        (data as bot_pb.GetBotResponse).bot,
      ),
    );
  }

  Future<BotsApiResult<VoiceBotSummary>> getBotBySlug({
    required String authorization,
    required String slug,
  }) async {
    final result = await _gateway.getProto(
      _gateway.resolve('/api/v1/bots/slug/$slug'),
      authorization: authorization,
      createEmpty: bot_pb.GetBotResponse.create,
    );
    return _map(
      result,
      (data) => VoiceBotSummary.fromProto(
        (data as bot_pb.GetBotResponse).bot,
      ),
    );
  }

  Future<BotsApiResult<List<InstalledBotInfo>>> listInstalledBots({
    required String authorization,
    required String spaceId,
  }) async {
    final result = await _gateway.getProto(
      _gateway.resolve('/api/v1/bots/spaces/$spaceId/installed'),
      authorization: authorization,
      createEmpty: bot_pb.ListInstalledBotsResponse.create,
    );
    return _map(
      result,
      (data) {
        final response = data as bot_pb.ListInstalledBotsResponse;
        return response.installedBots
            .map(InstalledBotInfo.fromProto)
            .toList(growable: false);
      },
    );
  }

  Future<BotsApiResult<List<ChatBotSettings>>> listBotsInChat({
    required String authorization,
    required String chatId,
    required String chatType,
    required String spaceId,
  }) async {
    final result = await _gateway.getProto(
      _gateway.replace(
        path: '/api/v1/bots/chats/$chatId',
        queryParameters: {
          'chat_type': chatType,
          'space_id': spaceId,
        },
      ),
      authorization: authorization,
      createEmpty: bot_pb.ListBotsInChatResponse.create,
    );
    return _map(
      result,
      (data) {
        final response = data as bot_pb.ListBotsInChatResponse;
        return response.bots
            .map(ChatBotSettings.fromProto)
            .toList(growable: false);
      },
    );
  }

  Future<BotsApiResult<void>> setBotChatEnabled({
    required String authorization,
    required String botId,
    required String chatId,
    required String chatType,
    required String spaceId,
    required bool enabled,
  }) async {
    final result = await _gateway.patchProto(
      uri: _gateway.resolve(
        '/api/v1/bots/$botId/chats/$chatId/enabled',
      ),
      authorization: authorization,
      body: bot_pb.SetBotChatEnabledRequest(
        botId: botId,
        chat: chat_pb.ChatRef(id: chatId, type: _chatTypeFromWire(chatType)),
        enabled: enabled,
        spaceId: spaceId,
      ),
      createEmpty: bot_pb.SetBotChatEnabledResponse.create,
      allowNoContent: true,
    );
    return switch (result) {
      GatewayHttpOk() => const BotsApiOk(null),
      GatewayHttpFailure(:final error) => BotsApiFailure(
        message: GatewayApiResultMapper.failureMessage(error),
        errorCode: GatewayApiResultMapper.failureCode(error),
        statusCode: GatewayApiResultMapper.failureStatus(error),
      ),
    };
  }

  Future<BotsApiResult<String>> installBotInSpace({
    required String authorization,
    required String botId,
    required String spaceId,
    required List<({String id, String type})> allowedChats,
    bool acknowledgePrivilegedScopes = false,
  }) async {
    final result = await _gateway.postProto(
      uri: _gateway.resolve(
        '/api/v1/bots/$botId/spaces/$spaceId/install',
      ),
      authorization: authorization,
      body: bot_pb.InstallBotInSpaceRequest(
        botId: botId,
        spaceId: spaceId,
        allowedChats: allowedChats
            .map(
              (c) => chat_pb.ChatRef(
                id: c.id,
                type: _chatTypeFromWire(c.type),
              ),
            )
            .toList(),
        acknowledgePrivilegedScopes: acknowledgePrivilegedScopes,
      ),
      createEmpty: bot_pb.InstallBotInSpaceResponse.create,
    );
    return _map(
      result,
      (data) => (data as bot_pb.InstallBotInSpaceResponse).installationId,
    );
  }

  Future<BotsApiResult<void>> uninstallBotFromSpace({
    required String authorization,
    required String botId,
    required String spaceId,
  }) async {
    final result = await _gateway.deleteEmpty(
      uri: _gateway.resolve('/api/v1/bots/$botId/spaces/$spaceId'),
      authorization: authorization,
    );
    return switch (result) {
      GatewayHttpOk() => const BotsApiOk(null),
      GatewayHttpFailure(:final error) => BotsApiFailure(
        message: GatewayApiResultMapper.failureMessage(error),
        errorCode: GatewayApiResultMapper.failureCode(error),
        statusCode: GatewayApiResultMapper.failureStatus(error),
      ),
    };
  }

  BotsApiResult<T> _map<T>(
    GatewayHttpResult<dynamic> result,
    T Function(dynamic data) parse,
  ) {
    return switch (result) {
      GatewayHttpOk(:final data) => BotsApiOk(parse(data)),
      GatewayHttpFailure(:final error) => BotsApiFailure(
        message: GatewayApiResultMapper.failureMessage(error),
        errorCode: GatewayApiResultMapper.failureCode(error),
        statusCode: GatewayApiResultMapper.failureStatus(error),
      ),
    };
  }
}

ChatType _chatTypeFromWire(String raw) {
  return switch (raw) {
    'CHAT_TYPE_DM' => ChatType.CHAT_TYPE_DM,
    'CHAT_TYPE_GROUP' => ChatType.CHAT_TYPE_GROUP,
    'CHAT_TYPE_CHANNEL' => ChatType.CHAT_TYPE_CHANNEL,
    _ => ChatType.CHAT_TYPE_CHANNEL,
  };
}

SlashInteractionOutcome _slashOutcomeFromProto(
  bot_pb.ExecuteSlashInteractionResponse data,
) {
  return SlashInteractionOutcome(
    interactionToken: data.interactionToken,
    content: emptyToNull(data.content),
    isEphemeral: data.isEphemeral,
    deferred: data.deferred,
    errorCode: emptyToNull(data.errorCode),
    errorMessage: emptyToNull(data.errorMessage),
    message: data.hasMessage() ? data.message : null,
  );
}
