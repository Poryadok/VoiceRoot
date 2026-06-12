import 'package:drift/drift.dart';
import 'package:drift_flutter/drift_flutter.dart';

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

  factory MessageCacheDatabase.defaults() {
    return MessageCacheDatabase(
      driftDatabase(name: 'voice_message_cache'),
    );
  }

  @override
  int get schemaVersion => 1;
}
