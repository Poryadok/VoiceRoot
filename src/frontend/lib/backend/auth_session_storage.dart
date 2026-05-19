import 'dart:convert';

import 'package:shared_preferences/shared_preferences.dart';

import 'auth_session.dart';

/// Persists [AuthSession] locally (tokens + active [AuthSession.activeProfileId]).
abstract class AuthSessionStorage {
  Future<AuthSession?> read();
  Future<void> write(AuthSession session);
  Future<void> clear();
}

class InMemoryAuthSessionStorage implements AuthSessionStorage {
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
}

const _prefsKey = 'voice.auth.session';

class SharedPreferencesAuthSessionStorage implements AuthSessionStorage {
  SharedPreferencesAuthSessionStorage(this._prefs);

  final SharedPreferences _prefs;

  @override
  Future<void> clear() async {
    await _prefs.remove(_prefsKey);
  }

  @override
  Future<AuthSession?> read() async {
    final raw = _prefs.getString(_prefsKey);
    if (raw == null || raw.isEmpty) return null;
    try {
      final json = jsonDecode(raw) as Map<String, dynamic>;
      return AuthSession.fromJson(json);
    } catch (_) {
      await clear();
      return null;
    }
  }

  @override
  Future<void> write(AuthSession session) async {
    await _prefs.setString(_prefsKey, jsonEncode(session.toJson()));
  }
}
