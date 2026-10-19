import 'dart:convert';

import 'package:hive/hive.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'chat_draft_storage_open_web.dart'
    if (dart.library.io) 'chat_draft_storage_open_native.dart';

/// Identifies a local draft for exactly one active profile and chat.
class ChatDraftKey {
  const ChatDraftKey({required this.profileId, required this.chatId});

  final String profileId;
  final String chatId;

  @override
  bool operator ==(Object other) =>
      other is ChatDraftKey &&
      other.profileId == profileId &&
      other.chatId == chatId;

  @override
  int get hashCode => Object.hash(profileId, chatId);
}

/// Device-local persistence for unfinished text-chat messages.
abstract class ChatDraftStorage {
  Future<String?> read(ChatDraftKey key);
  Future<void> write(ChatDraftKey key, String draft);
  Future<void> clear(ChatDraftKey key);
}

class InMemoryChatDraftStorage implements ChatDraftStorage {
  final Map<ChatDraftKey, String> _drafts = {};

  @override
  Future<void> clear(ChatDraftKey key) async {
    _drafts.remove(key);
  }

  @override
  Future<String?> read(ChatDraftKey key) async => _drafts[key];

  @override
  Future<void> write(ChatDraftKey key, String draft) async {
    _drafts[key] = draft;
  }
}

class SharedPreferencesChatDraftStorage implements ChatDraftStorage {
  SharedPreferencesChatDraftStorage(this._prefs);

  static const _prefix = 'voice.chat_draft.';
  final SharedPreferences _prefs;

  String _storageKey(ChatDraftKey key) =>
      '$_prefix${Uri.encodeComponent(key.profileId)}.${Uri.encodeComponent(key.chatId)}';

  @override
  Future<void> clear(ChatDraftKey key) async {
    await _prefs.remove(_storageKey(key));
  }

  @override
  Future<String?> read(ChatDraftKey key) async =>
      _prefs.getString(_storageKey(key));

  @override
  Future<void> write(ChatDraftKey key, String draft) async {
    await _prefs.setString(_storageKey(key), draft);
  }
}

/// One durable local draft per profile/chat. Profile IDs are account-owned UUIDs.
class HiveChatDraftStorage implements ChatDraftStorage {
  HiveChatDraftStorage(this._box, {SharedPreferences? legacyPreferences})
    : _legacy = legacyPreferences == null
          ? null
          : SharedPreferencesChatDraftStorage(legacyPreferences);

  final Box<String> _box;
  final ChatDraftStorage? _legacy;
  Future<void> _pending = Future.value();

  Future<T> _ordered<T>(Future<T> Function() operation) {
    final result = _pending.then((_) => operation());
    // A failed operation is reported to its caller without poisoning later edits.
    _pending = result.then<void>((_) {}, onError: (Object _, StackTrace _) {});
    return result;
  }

  static Future<HiveChatDraftStorage> open(
    SharedPreferences preferences,
  ) async {
    await initializeChatDraftStorage();
    return HiveChatDraftStorage(
      await Hive.openBox<String>('voice_chat_drafts'),
      legacyPreferences: preferences,
    );
  }

  String _storageKey(ChatDraftKey key) =>
      jsonEncode([key.profileId, key.chatId]);

  @override
  Future<String?> read(ChatDraftKey key) => _ordered(() async {
    final stored = _box.get(_storageKey(key));
    if (stored != null) return stored;
    final legacy = await _legacy?.read(key);
    if (legacy != null) {
      // Keep the old value until the Hive write has completed successfully.
      await _box.put(_storageKey(key), legacy);
      await _legacy?.clear(key);
    }
    return legacy;
  });

  @override
  Future<void> write(ChatDraftKey key, String draft) => _ordered(() async {
    await _box.put(_storageKey(key), draft);
    await _legacy?.clear(key);
  });

  @override
  Future<void> clear(ChatDraftKey key) => _ordered(() async {
    // Delete the legacy value first so it cannot resurrect a cleared draft.
    await _legacy?.clear(key);
    await _box.delete(_storageKey(key));
  });
}
