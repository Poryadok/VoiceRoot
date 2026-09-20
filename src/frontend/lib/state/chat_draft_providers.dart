import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../backend/chat_draft_storage.dart';

final chatDraftStorageProvider = Provider<ChatDraftStorage>((ref) {
  return InMemoryChatDraftStorage();
});

final chatDraftProvider = StateNotifierProvider.autoDispose
    .family<ChatDraftController, AsyncValue<String>, ChatDraftKey>((ref, key) {
      return ChatDraftController(ref.read(chatDraftStorageProvider), key);
    });

class ChatDraftController extends StateNotifier<AsyncValue<String>> {
  ChatDraftController(this._storage, this.key)
    : super(const AsyncValue.loading()) {
    unawaited(_restore());
  }

  final ChatDraftStorage _storage;
  final ChatDraftKey key;
  Future<void> _pendingWrite = Future.value();
  var _revision = 0;

  Future<void> _restore() async {
    final revision = _revision;
    try {
      final restored = await _storage.read(key) ?? '';
      if (mounted && revision == _revision) {
        state = AsyncValue.data(restored);
      }
    } on Object catch (error, stackTrace) {
      state = AsyncValue.error(error, stackTrace);
    }
  }

  void update(String value) {
    _revision++;
    state = AsyncValue.data(value);
    _pendingWrite = _pendingWrite.then((_) async {
      if (value.isEmpty) {
        await _storage.clear(key);
      } else {
        await _storage.write(key, value);
      }
    });
  }

  Future<void> clear() async {
    update('');
    await _pendingWrite;
  }
}
