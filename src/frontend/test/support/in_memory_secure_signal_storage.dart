import 'package:voice_frontend/e2e/secure_signal_store.dart';

/// In-memory [SecureSignalStorage] for live/unit tests without platform bindings.
class InMemorySecureSignalStorage implements SecureSignalStorage {
  final Map<String, String> _values = {};

  @override
  Future<void> delete({required String key}) async {
    _values.remove(key);
  }

  @override
  Future<String?> read({required String key}) async => _values[key];

  @override
  Future<void> write({required String key, required String value}) async {
    _values[key] = value;
  }
}
