import 'package:shared_preferences/shared_preferences.dart';

/// Persists whether the post-login social discover snackbar was shown.
abstract class DiscoverHintStorage {
  Future<bool> wasShown();
  Future<void> markShown();
}

class InMemoryDiscoverHintStorage implements DiscoverHintStorage {
  bool _shown = false;

  @override
  Future<void> markShown() async {
    _shown = true;
  }

  @override
  Future<bool> wasShown() async => _shown;
}

const _prefsKey = 'voice.discover.hint_shown';

class SharedPreferencesDiscoverHintStorage implements DiscoverHintStorage {
  SharedPreferencesDiscoverHintStorage(this._prefs);

  final SharedPreferences _prefs;

  @override
  Future<bool> wasShown() async => _prefs.getBool(_prefsKey) ?? false;

  @override
  Future<void> markShown() async {
    await _prefs.setBool(_prefsKey, true);
  }
}
