import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../backend/message_cache/drift_message_cache_store.dart';
import '../backend/message_cache/in_memory_message_cache_store.dart';
import '../backend/message_cache/message_cache_database.dart';
import '../backend/message_cache/message_cache_store.dart';
import 'auth_providers.dart';

final messageCacheStoreProvider = Provider<MessageCacheStore>((ref) {
  return InMemoryMessageCacheStore();
});

/// Clears message cache when the user logs out.
final messageCacheLifecycleProvider = Provider<void>((ref) {
  ref.listen<AuthState>(authControllerProvider, (previous, next) {
    if (previous?.isAuthenticated == true && !next.isAuthenticated) {
      unawaited(ref.read(messageCacheStoreProvider).clearAll());
    }
  });
});

Future<MessageCacheStore> openDefaultMessageCacheStore() async {
  final db = MessageCacheDatabase.defaults();
  return DriftMessageCacheStore(db);
}

MessageCacheStore inMemoryMessageCacheStoreForTests() {
  return InMemoryMessageCacheStore();
}
