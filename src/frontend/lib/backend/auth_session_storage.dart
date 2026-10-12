import 'dart:convert';

import 'package:shared_preferences/shared_preferences.dart';

import 'auth_session.dart';

/// Persists [AuthSession] locally (tokens + active [AuthSession.activeProfileId]).
abstract class AuthSessionStorage {
  Future<AuthSession?> read();
  Future<void> write(AuthSession session);
  Future<void> clear();
}

/// Optional compare-and-clear capability for a logout tied to one exact session.
/// Implementations must order this operation with writes and unconditional clears.
abstract interface class ConditionalAuthSessionStorage {
  Future<bool> clearIfUnchanged(AuthSession expected);
}

class InMemoryAuthSessionStorage
    implements AuthSessionStorage, ConditionalAuthSessionStorage {
  AuthSession? _session;

  @override
  Future<void> clear() async {
    _session = null;
  }

  @override
  Future<AuthSession?> read() async => _session;

  @override
  Future<void> write(AuthSession session) async {
    _session = session;
  }

  @override
  Future<bool> clearIfUnchanged(AuthSession expected) async {
    if (_session != expected) return false;
    _session = null;
    return true;
  }
}

const _prefsKey = 'voice.auth.session';

class SharedPreferencesAuthSessionStorage
    implements AuthSessionStorage, ConditionalAuthSessionStorage {
  SharedPreferencesAuthSessionStorage(this._prefs);

  final SharedPreferences _prefs;
  Future<void> _operations = Future<void>.value();

  Future<T> _serialize<T>(Future<T> Function() operation) {
    final result = _operations.then((_) => operation());
    _operations = result.then<void>((_) {}, onError: (_, _) {});
    return result;
  }

  @override
  Future<void> clear() => _serialize(() async {
    await _prefs.remove(_prefsKey);
  });

  @override
  Future<AuthSession?> read() => _serialize(() async {
    final raw = _prefs.getString(_prefsKey);
    if (raw == null || raw.isEmpty) return null;
    try {
      final json = jsonDecode(raw) as Map<String, dynamic>;
      return AuthSession.fromJson(json);
    } catch (_) {
      await _prefs.remove(_prefsKey);
      return null;
    }
  });

  @override
  Future<void> write(AuthSession session) => _serialize(() async {
    await _prefs.setString(_prefsKey, jsonEncode(session.toJson()));
  });

  @override
  Future<bool> clearIfUnchanged(AuthSession expected) => _serialize(() async {
    final raw = _prefs.getString(_prefsKey);
    if (raw == null || raw.isEmpty) return false;
    AuthSession current;
    try {
      current = AuthSession.fromJson(jsonDecode(raw) as Map<String, dynamic>);
    } on Object {
      return false;
    }
    if (current != expected) return false;
    await _prefs.remove(_prefsKey);
    return true;
  });
}
