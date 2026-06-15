import '../backend/message_cache/drift_message_cache_store.dart';
import '../backend/message_cache/message_cache_database.dart';
import '../backend/message_cache/message_cache_store.dart';

Future<MessageCacheStore> openDefaultMessageCacheStoreImpl() async {
  final db = await MessageCacheDatabase.openEncrypted();
  return DriftMessageCacheStore(db);
}
