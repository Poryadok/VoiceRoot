import '../backend/message_cache/in_memory_message_cache_store.dart';
import '../backend/message_cache/message_cache_store.dart';

Future<MessageCacheStore> openDefaultMessageCacheStoreImpl() async {
  return InMemoryMessageCacheStore();
}
