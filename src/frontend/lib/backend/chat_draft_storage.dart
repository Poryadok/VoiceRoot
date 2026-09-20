import 'package:shared_preferences/shared_preferences.dart';

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
