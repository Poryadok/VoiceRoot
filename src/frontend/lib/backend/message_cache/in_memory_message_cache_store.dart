import 'dart:convert';

import '../messages_client.dart';
import 'message_cache_store.dart';
import 'message_cache_utils.dart';

class InMemoryMessageCacheStore implements MessageCacheStore {
  final Map<String, Map<String, VoiceMessage>> _byProfileChat = {};

  String _key(String profileId, String chatId) => '$profileId::$chatId';

  @override
  Future<void> clearAll() async {
    _byProfileChat.clear();
  }

  @override
  Future<void> clearProfile(String profileId) async {
    _byProfileChat.removeWhere((key, _) => key.startsWith('$profileId::'));
  }

  @override
  Future<List<VoiceMessage>> getMessages({
    required String profileId,
    required String chatId,
  }) async {
    final bucket = _byProfileChat[_key(profileId, chatId)];
    if (bucket == null || bucket.isEmpty) {
      return const [];
    }
    return sortMessagesForCache(bucket.values);
  }

  @override
  Future<void> replaceChatMessages({
    required String profileId,
    required String chatId,
    required List<VoiceMessage> messages,
  }) async {
    final trimmed = trimMessagesToCacheLimit(messages);
    final bucket = <String, VoiceMessage>{};
    for (final message in trimmed) {
      bucket[message.id] = message;
    }
    _byProfileChat[_key(profileId, chatId)] = bucket;
  }

  @override
  Future<void> upsertMessages({
    required String profileId,
    required String chatId,
    required List<VoiceMessage> messages,
  }) async {
    final key = _key(profileId, chatId);
    final bucket = Map<String, VoiceMessage>.from(
      _byProfileChat[key] ?? const {},
    );
    for (final message in messages) {
      bucket[message.id] = message;
    }
    _byProfileChat[key] = {
      for (final message in trimMessagesToCacheLimit(bucket.values))
        message.id: message,
    };
  }

  /// Round-trip helper for drift parity tests.
  static String encodePayload(VoiceMessage message) {
    return jsonEncode(message.toJson());
  }

  static VoiceMessage decodePayload(String payload) {
    return VoiceMessage.fromJson(
      jsonDecode(payload) as Map<String, dynamic>,
    );
  }
}
