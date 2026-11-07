import '../messages_client.dart';

/// Server-order progress, independent of body timestamps and display sorting.
class MessageHistoryCursor {
  const MessageHistoryCursor({this.messageId, this.pageCursor});
  final String? messageId;
  final String? pageCursor;
}

abstract interface class HistoryCursorMessageCacheStore
    implements MessageCacheStore {
  Future<MessageHistoryCursor?> getHistoryCursor({
    required String profileId,
    required String chatId,
  });
  Future<void> replaceChatHistory({
    required String profileId,
    required String chatId,
    required List<VoiceMessage> messages,
    required MessageHistoryCursor? historyCursor,
  });
}

/// Local read-only message cache (offline v1).
abstract class MessageCacheStore {
  Future<List<VoiceMessage>> getMessages({
    required String profileId,
    required String chatId,
  });

  Future<void> replaceChatMessages({
    required String profileId,
    required String chatId,
    required List<VoiceMessage> messages,
  });

  Future<void> upsertMessages({
    required String profileId,
    required String chatId,
    required List<VoiceMessage> messages,
  });

  Future<void> clearProfile(String profileId);

  Future<void> clearAll();
}
