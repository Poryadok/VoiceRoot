import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:shared_preferences/shared_preferences.dart';

/// Persists the auto-generated password for guest accounts (convert-guest later)
/// and whether the guest has chosen a display nickname.
abstract class GuestCredentialsStorage {
  Future<String?> readPassword();
  Future<void> writePassword(String password);
  Future<String?> readPendingConversionEmail();
  Future<void> writePendingConversionEmail(String email);
  Future<void> clearPendingConversionEmail();
  Future<bool> isNicknameCompleted(String accountId);
  Future<void> markNicknameCompleted(String accountId);
  Future<void> clear();
}

/// True when the profile still has the server-assigned placeholder display name.
bool isPlaceholderGuestDisplayName({
  required String accountId,
  required String displayName,
}) {
  String norm(String s) => s.replaceAll('-', '').toLowerCase();
  return norm(displayName) == norm(accountId);
}

class FlutterGuestCredentialsStorage implements GuestCredentialsStorage {
  FlutterGuestCredentialsStorage({
    FlutterSecureStorage? storage,
    SharedPreferences? prefs,
  }) : _storage = storage ?? const FlutterSecureStorage(),
       _prefs = prefs;

  static const _passwordKey = 'voice.auth.guest_password';
  static const _pendingConversionEmailKey =
      'voice.auth.guest_pending_conversion_email';
  static const _nicknameKeyPrefix = 'voice.auth.guest_nickname_done.';

  final FlutterSecureStorage _storage;
  final SharedPreferences? _prefs;

  String _nicknameKey(String accountId) => '$_nicknameKeyPrefix$accountId';

  @override
  Future<void> clear() async {
    await _storage.delete(key: _passwordKey);
    await _storage.delete(key: _pendingConversionEmailKey);
    final prefs = _prefs;
    if (prefs == null) return;
    final keys = prefs
        .getKeys()
        .where((key) => key.startsWith(_nicknameKeyPrefix))
        .toList();
    for (final key in keys) {
      await prefs.remove(key);
    }
  }

  @override
  Future<bool> isNicknameCompleted(String accountId) async {
    return _prefs?.getBool(_nicknameKey(accountId)) ?? false;
  }

  @override
  Future<void> markNicknameCompleted(String accountId) async {
    await _prefs?.setBool(_nicknameKey(accountId), true);
  }

  @override
  Future<String?> readPassword() => _storage.read(key: _passwordKey);

  @override
  Future<String?> readPendingConversionEmail() =>
      _storage.read(key: _pendingConversionEmailKey);

  @override
  Future<void> writePendingConversionEmail(String email) =>
      _storage.write(key: _pendingConversionEmailKey, value: email);

  @override
  Future<void> clearPendingConversionEmail() =>
      _storage.delete(key: _pendingConversionEmailKey);

  @override
  Future<void> writePassword(String password) =>
      _storage.write(key: _passwordKey, value: password);
}

class InMemoryGuestCredentialsStorage implements GuestCredentialsStorage {
  String? _password;
  String? _pendingConversionEmail;
  final Map<String, bool> _nicknameCompleted = {};

  @override
  Future<void> clear() async {
    _password = null;
    _pendingConversionEmail = null;
    _nicknameCompleted.clear();
  }

  @override
  Future<bool> isNicknameCompleted(String accountId) async {
    return _nicknameCompleted[accountId] ?? false;
  }

  @override
  Future<void> markNicknameCompleted(String accountId) async {
    _nicknameCompleted[accountId] = true;
  }

  @override
  Future<String?> readPassword() async => _password;

  @override
  Future<String?> readPendingConversionEmail() async => _pendingConversionEmail;

  @override
  Future<void> writePendingConversionEmail(String email) async {
    _pendingConversionEmail = email;
  }

  @override
  Future<void> clearPendingConversionEmail() async {
    _pendingConversionEmail = null;
  }

  @override
  Future<void> writePassword(String password) async {
    _password = password;
  }
}
