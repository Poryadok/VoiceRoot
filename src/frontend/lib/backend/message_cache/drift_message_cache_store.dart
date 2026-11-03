import 'package:drift/drift.dart';

import '../messages_client.dart';
import 'in_memory_message_cache_store.dart';
import 'message_cache_database.dart';
import 'message_cache_store.dart';
import 'message_cache_utils.dart';

class DriftMessageCacheStore implements HistoryCursorMessageCacheStore {
  DriftMessageCacheStore(this._db);

  final MessageCacheDatabase _db;

  @override
  Future<void> clearAll() async {
    await _db.transaction(() async {
      await _db.delete(_db.cachedMessages).go();
      await _db.customStatement('DELETE FROM cached_message_history');
    });
  }

  @override
  Future<void> clearProfile(String profileId) async {
    await _db.transaction(() async {
      await (_db.delete(
        _db.cachedMessages,
      )..where((row) => row.profileId.equals(profileId))).go();
      await _db.customStatement(
        'DELETE FROM cached_message_history WHERE profile_id=?',
        [profileId],
      );
    });
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
                    row.profileId.equals(profileId) & row.chatId.equals(chatId),
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
        .map((row) => InMemoryMessageCacheStore.decodePayload(row.payloadJson))
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
      await (_db.delete(_db.cachedMessages)..where(
            (row) =>
                row.profileId.equals(profileId) & row.chatId.equals(chatId),
          ))
          .go();
      if (trimmed.isEmpty) {
        await _db.customStatement(
          'DELETE FROM cached_message_history WHERE profile_id=? AND chat_id=?',
          [profileId, chatId],
        );
        return;
      }
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
  Future<MessageHistoryCursor?> getHistoryCursor({
    required String profileId,
    required String chatId,
  }) async {
    final row = await _db
        .customSelect(
          'SELECT after_message_id,page_cursor FROM cached_message_history WHERE profile_id=? AND chat_id=?',
          variables: [
            Variable.withString(profileId),
            Variable.withString(chatId),
          ],
        )
        .getSingleOrNull();
    if (row == null) return null;
    return MessageHistoryCursor(
      messageId: row.readNullable<String>('after_message_id'),
      pageCursor: row.readNullable<String>('page_cursor'),
    );
  }

  @override
  Future<void> replaceChatHistory({
    required String profileId,
    required String chatId,
    required List<VoiceMessage> messages,
    required MessageHistoryCursor? historyCursor,
  }) async {
    await _db.transaction(() async {
      await replaceChatMessages(
        profileId: profileId,
        chatId: chatId,
        messages: messages,
      );
      if (historyCursor == null) {
        await _db.customStatement(
          'DELETE FROM cached_message_history WHERE profile_id=? AND chat_id=?',
          [profileId, chatId],
        );
      } else {
        await _db.customStatement(
          '''INSERT INTO cached_message_history(profile_id,chat_id,after_message_id,page_cursor)
VALUES(?,?,?,?) ON CONFLICT(profile_id,chat_id) DO UPDATE SET after_message_id=excluded.after_message_id,page_cursor=excluded.page_cursor''',
          [
            profileId,
            chatId,
            historyCursor.messageId,
            historyCursor.pageCursor,
          ],
        );
      }
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
