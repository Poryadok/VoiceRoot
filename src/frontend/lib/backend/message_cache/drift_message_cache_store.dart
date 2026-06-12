import 'package:drift/drift.dart';

import '../messages_client.dart';
import 'in_memory_message_cache_store.dart';
import 'message_cache_database.dart';
import 'message_cache_store.dart';
import 'message_cache_utils.dart';

class DriftMessageCacheStore implements MessageCacheStore {
  DriftMessageCacheStore(this._db);

  final MessageCacheDatabase _db;

  @override
  Future<void> clearAll() async {
    await _db.delete(_db.cachedMessages).go();
  }

  @override
  Future<void> clearProfile(String profileId) async {
    await (_db.delete(_db.cachedMessages)
          ..where((row) => row.profileId.equals(profileId)))
        .go();
  }

  @override
  Future<List<VoiceMessage>> getMessages({
    required String profileId,
    required String chatId,
  }) async {
    final rows =
        await (_db.select(_db.cachedMessages)
              ..where(
                (row) =>
                    row.profileId.equals(profileId) &
                    row.chatId.equals(chatId),
              )
              ..orderBy([
                (row) => OrderingTerm(
                  expression: row.createdAt,
                  mode: OrderingMode.asc,
                ),
                (row) => OrderingTerm(expression: row.messageId),
              ]))
            .get();
    return rows
        .map(
          (row) => InMemoryMessageCacheStore.decodePayload(row.payloadJson),
        )
        .toList(growable: false);
  }

  @override
  Future<void> replaceChatMessages({
    required String profileId,
    required String chatId,
    required List<VoiceMessage> messages,
  }) async {
    final trimmed = trimMessagesToCacheLimit(messages);
    await _db.transaction(() async {
      await (_db.delete(_db.cachedMessages)
            ..where(
              (row) =>
                  row.profileId.equals(profileId) & row.chatId.equals(chatId),
            ))
          .go();
      if (trimmed.isEmpty) return;
      await _db.batch((batch) {
        batch.insertAll(
          _db.cachedMessages,
          trimmed.map(
            (message) => CachedMessagesCompanion.insert(
              profileId: profileId,
              chatId: chatId,
              messageId: message.id,
              payloadJson: InMemoryMessageCacheStore.encodePayload(message),
              createdAt: Value(message.createdAt),
            ),
          ),
        );
      });
    });
  }

  @override
  Future<void> upsertMessages({
    required String profileId,
    required String chatId,
    required List<VoiceMessage> messages,
  }) async {
    if (messages.isEmpty) return;
    final existing = await getMessages(profileId: profileId, chatId: chatId);
    final merged = <String, VoiceMessage>{
      for (final message in existing) message.id: message,
      for (final message in messages) message.id: message,
    };
    await replaceChatMessages(
      profileId: profileId,
      chatId: chatId,
      messages: merged.values.toList(),
    );
  }
}
