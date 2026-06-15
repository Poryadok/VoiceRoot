import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../backend/bots_client.dart';
import '../backend/proto_mappers.dart';
import 'auth_providers.dart';
import 'chat_providers.dart';

final voiceBotsClientProvider = Provider<VoiceBotsClient>((ref) {
  return VoiceBotsClient(gateway: ref.watch(gatewayHttpClientProvider));
});

final chatTypeForChatProvider = Provider.autoDispose.family<String?, String>((
  ref,
  chatId,
) {
  final items = ref.watch(chatListProvider).valueOrNull?.items;
  if (items == null) return null;
  for (final item in items) {
    if (item.chatId == chatId) {
      return item.chat.type;
    }
  }
  return null;
});

final slashCommandsForChatProvider = FutureProvider.autoDispose
    .family<List<BotSlashCommand>, String>((ref, chatId) async {
      final auth = ref.watch(authorizationHeaderProvider);
      if (auth == null) return const [];

      final chatType =
          ref.watch(chatTypeForChatProvider(chatId)) ?? 'CHAT_TYPE_CHANNEL';
      final result = await ref
          .read(voiceBotsClientProvider)
          .listSlashCommandsForChat(
            authorization: auth,
            chatId: chatId,
            chatType: chatType,
          );
      return switch (result) {
        BotsApiOk(:final data) => data,
        BotsApiFailure() => const [],
      };
    });

class EphemeralBotMessage {
  EphemeralBotMessage({
    required this.id,
    required this.content,
    this.botName,
    DateTime? createdAt,
  }) : createdAt = createdAt ?? DateTime.now();

  final String id;
  final String content;
  final String? botName;
  final DateTime createdAt;
}

class EphemeralMessagesNotifier extends StateNotifier<List<EphemeralBotMessage>> {
  EphemeralMessagesNotifier() : super(const []);

  void add(EphemeralBotMessage message) {
    state = [...state, message];
  }

  void clear() => state = const [];
}

final ephemeralMessagesProvider = StateNotifierProvider.autoDispose
    .family<EphemeralMessagesNotifier, List<EphemeralBotMessage>, String>(
      (ref, chatId) => EphemeralMessagesNotifier(),
    );

class DeferredBotInteraction {
  const DeferredBotInteraction({
    required this.botName,
    required this.interactionToken,
  });

  final String botName;
  final String interactionToken;
}

class DeferredInteractionNotifier extends StateNotifier<DeferredBotInteraction?> {
  DeferredInteractionNotifier() : super(null);

  void setDeferred({required String botName, required String interactionToken}) {
    state = DeferredBotInteraction(
      botName: botName,
      interactionToken: interactionToken,
    );
  }

  void clear() => state = null;
}

final deferredBotInteractionProvider = StateNotifierProvider.autoDispose
    .family<DeferredInteractionNotifier, DeferredBotInteraction?, String>(
      (ref, chatId) => DeferredInteractionNotifier(),
    );

enum SlashInteractionFailure { botTimeout, requestFailed }

final slashInteractionExecutorProvider = Provider<SlashInteractionExecutor>((
  ref,
) {
  return SlashInteractionExecutor(ref);
});

class SlashInteractionExecutor {
  SlashInteractionExecutor(this._ref);

  final Ref _ref;

  Future<SlashInteractionFailure?> execute({
    required String chatId,
    required BotSlashCommand command,
    String optionsJson = '{}',
  }) async {
    final auth = _ref.read(authorizationHeaderProvider);
    if (auth == null) return SlashInteractionFailure.requestFailed;

    final chatType =
        _ref.read(chatTypeForChatProvider(chatId)) ?? 'CHAT_TYPE_CHANNEL';
    final result = await _ref.read(voiceBotsClientProvider).executeSlashInteraction(
      authorization: auth,
      chatId: chatId,
      chatType: chatType,
      botId: command.botId,
      commandName: command.name,
      optionsJson: optionsJson,
    );

    return switch (result) {
      BotsApiOk(:final data) => _handleOutcome(chatId, command, data),
      BotsApiFailure(:final errorCode) when errorCode == kBotTimeoutErrorCode =>
        SlashInteractionFailure.botTimeout,
      BotsApiFailure() => SlashInteractionFailure.requestFailed,
    };
  }

  SlashInteractionFailure? _handleOutcome(
    String chatId,
    BotSlashCommand command,
    SlashInteractionOutcome data,
  ) {
    if (data.isBotTimeout) {
      return SlashInteractionFailure.botTimeout;
    }
    if (data.deferred) {
      _ref.read(deferredBotInteractionProvider(chatId).notifier).setDeferred(
        botName: command.botName,
        interactionToken: data.interactionToken,
      );
      return null;
    }
    if (data.isEphemeral) {
      final content = data.content?.trim();
      if (content != null && content.isNotEmpty) {
        _ref.read(ephemeralMessagesProvider(chatId).notifier).add(
          EphemeralBotMessage(
            id: data.interactionToken,
            content: content,
            botName: command.botName,
          ),
        );
      }
      return null;
    }
    if (data.message != null) {
      _ref.read(deferredBotInteractionProvider(chatId).notifier).clear();
      final msg = voiceMessageFromProto(data.message!);
      final room = _ref.read(chatRoomControllerProvider(chatId));
      if (!room.messages.any((m) => m.id == msg.id)) {
        _ref
            .read(chatRoomControllerProvider(chatId).notifier)
            .ingestOutboundMessage(msg);
      }
    }
    return null;
  }
}
