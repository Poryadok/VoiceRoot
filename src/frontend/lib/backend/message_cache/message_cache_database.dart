import 'package:drift/drift.dart';
import 'package:drift/native.dart';

import 'message_cache_database_open.dart';

part 'message_cache_database.g.dart';

class CachedMessages extends Table {
  TextColumn get profileId => text()();
  TextColumn get chatId => text()();
  TextColumn get messageId => text()();
  TextColumn get payloadJson => text()();
  DateTimeColumn get createdAt => dateTime().nullable()();

  @override
  Set<Column<Object>> get primaryKey => {profileId, chatId, messageId};
}

@DriftDatabase(tables: [CachedMessages])
class MessageCacheDatabase extends _$MessageCacheDatabase {
  MessageCacheDatabase(super.e);

  /// Encrypted persistent cache for mobile/desktop (docs/features/encryption.md).
  static Future<MessageCacheDatabase> openEncrypted() async {
    final executor = await openEncryptedMessageCacheExecutor();
    return MessageCacheDatabase(executor);
  }

  /// Unencrypted in-memory database for unit tests.
  factory MessageCacheDatabase.forTesting() {
    return MessageCacheDatabase(NativeDatabase.memory());
  }

  @override
  int get schemaVersion => 1;
}
