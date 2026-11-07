import 'dart:convert';

import '../messages_client.dart';
import 'message_cache_store.dart';
import 'message_cache_utils.dart';

class InMemoryMessageCacheStore implements HistoryCursorMessageCacheStore {
  final Map<String, Map<String, VoiceMessage>> _byProfileChat = {};
  final Map<String, MessageHistoryCursor> _historyCursors = {};

  String _key(String profileId, String chatId) => '$profileId::$chatId';

  @override
  Future<void> clearAll() async {
    _byProfileChat.clear();
    _historyCursors.clear();
  }

  @override
  Future<void> clearProfile(String profileId) async {
    _byProfileChat.removeWhere((key, _) => key.startsWith('$profileId::'));
    _historyCursors.removeWhere((key, _) => key.startsWith('$profileId::'));
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
    if (trimmed.isEmpty) _historyCursors.remove(_key(profileId, chatId));
  }

  @override
  Future<MessageHistoryCursor?> getHistoryCursor({
    required String profileId,
    required String chatId,
  }) async => _historyCursors[_key(profileId, chatId)];

  @override
  Future<void> replaceChatHistory({
    required String profileId,
    required String chatId,
    required List<VoiceMessage> messages,
    required MessageHistoryCursor? historyCursor,
  }) async {
    // Controller cache reads/writes share MessageCacheMutationQueue, including
    // subclass gates used by the social-block ordering regressions.
    await replaceChatMessages(
      profileId: profileId,
      chatId: chatId,
      messages: messages,
    );
    final key = _key(profileId, chatId);
    if (historyCursor == null) {
      _historyCursors.remove(key);
    } else {
      _historyCursors[key] = historyCursor;
    }
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
    return VoiceMessage.fromJson(jsonDecode(payload) as Map<String, dynamic>);
  }
}
