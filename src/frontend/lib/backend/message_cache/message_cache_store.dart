import '../messages_client.dart';

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
