import 'package:shared_preferences/shared_preferences.dart';

/// Per-profile accent override until User Service exposes accent_color.
abstract class ProfileAccentStorage {
  Future<String?> readOverride(String profileId);
  Future<void> writeOverride(String profileId, String hex);
  Future<void> clearOverride(String profileId);
  Future<int?> readProfileIndex(String profileId);
  Future<void> writeProfileIndex(String profileId, int index);
}

class InMemoryProfileAccentStorage implements ProfileAccentStorage {
  final Map<String, String> _accents = {};
  final Map<String, int> _indices = {};

  @override
  Future<String?> readOverride(String profileId) async =>
      _accents[profileId];

  @override
  Future<void> writeOverride(String profileId, String hex) async {
    _accents[profileId] = hex;
  }

  @override
  Future<void> clearOverride(String profileId) async {
    _accents.remove(profileId);
  }

  @override
  Future<int?> readProfileIndex(String profileId) async => _indices[profileId];

  @override
  Future<void> writeProfileIndex(String profileId, int index) async {
    _indices[profileId] = index;
  }
}

class SharedPreferencesProfileAccentStorage implements ProfileAccentStorage {
  SharedPreferencesProfileAccentStorage(this._prefs);

  final SharedPreferences _prefs;

  static String _accentKey(String profileId) => 'profile_accent_$profileId';
  static String _indexKey(String profileId) => 'profile_index_$profileId';

  @override
  Future<String?> readOverride(String profileId) async {
    return _prefs.getString(_accentKey(profileId));
  }

  @override
  Future<void> writeOverride(String profileId, String hex) async {
    await _prefs.setString(_accentKey(profileId), hex);
  }

  @override
  Future<void> clearOverride(String profileId) async {
    await _prefs.remove(_accentKey(profileId));
  }

  @override
  Future<int?> readProfileIndex(String profileId) async {
    return _prefs.getInt(_indexKey(profileId));
  }

  @override
  Future<void> writeProfileIndex(String profileId, int index) async {
    await _prefs.setInt(_indexKey(profileId), index);
  }
}
