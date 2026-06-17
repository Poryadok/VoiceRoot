import 'package:flutter_secure_storage/flutter_secure_storage.dart';

/// Persists the auto-generated password for guest accounts (convert-guest later).
abstract class GuestCredentialsStorage {
  Future<String?> readPassword();
  Future<void> writePassword(String password);
  Future<void> clear();
}

class FlutterGuestCredentialsStorage implements GuestCredentialsStorage {
  FlutterGuestCredentialsStorage({FlutterSecureStorage? storage})
      : _storage = storage ?? const FlutterSecureStorage();

  static const _key = 'voice.auth.guest_password';

  final FlutterSecureStorage _storage;

  @override
  Future<void> clear() => _storage.delete(key: _key);

  @override
  Future<String?> readPassword() => _storage.read(key: _key);

  @override
  Future<void> writePassword(String password) =>
      _storage.write(key: _key, value: password);
}

class InMemoryGuestCredentialsStorage implements GuestCredentialsStorage {
  String? _password;

  @override
  Future<void> clear() async {
    _password = null;
  }

  @override
  Future<String?> readPassword() async => _password;

  @override
  Future<void> writePassword(String password) async {
    _password = password;
  }
}
