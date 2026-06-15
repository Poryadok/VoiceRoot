import 'dart:convert';

import 'package:flutter/foundation.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:libsignal_protocol_dart/libsignal_protocol_dart.dart';

import 'e2e_store_factory.dart';

/// Key-value persistence for Signal cryptographic state.
abstract class SecureSignalStorage {
  Future<String?> read({required String key});

  Future<void> write({required String key, required String value});

  Future<void> delete({required String key});
}

/// Production storage backed by OS secure storage (mobile/desktop/web).
class FlutterSecureSignalStorage implements SecureSignalStorage {
  FlutterSecureSignalStorage({required String profileId})
      : _prefix = 'voice_e2e_signal_${profileId}_',
        _storage = const FlutterSecureStorage(
          aOptions: AndroidOptions(encryptedSharedPreferences: true),
          webOptions: WebOptions(dbName: 'VoiceE2eSignal', publicKey: 'VoiceE2eSignal'),
        );

  final String _prefix;
  final FlutterSecureStorage _storage;

  @override
  Future<void> delete({required String key}) =>
      _storage.delete(key: '$_prefix$key');

  @override
  Future<String?> read({required String key}) =>
      _storage.read(key: '$_prefix$key');

  @override
  Future<void> write({required String key, required String value}) =>
      _storage.write(key: '$_prefix$key', value: value);
}

/// Persistent [SignalProtocolStore] for one profile (Batch E2E-A).
class SecureSignalStore implements SignalProtocolStore {
  SecureSignalStore._(this._inner, this._storage);

  final InMemorySignalProtocolStore _inner;
  final SecureSignalStorage _storage;
  var _dirty = false;

  static const _stateKey = 'state_v1';

  static Future<SecureSignalStore> open({
    required String profileId,
    SecureSignalStorage? storage,
  }) async {
    final backing = storage ?? FlutterSecureSignalStorage(profileId: profileId);
    final raw = await backing.read(key: _stateKey);
    if (raw != null && raw.isNotEmpty) {
      final inner = await _deserializeState(raw);
      return SecureSignalStore._(inner, backing);
    }
    final inner = await createInitializedSignalStore();
    final store = SecureSignalStore._(inner, backing).._dirty = true;
    await store.close();
    return store;
  }

  Future<void> close() async {
    if (!_dirty) return;
    await _storage.write(key: _stateKey, value: await _serializeState(_inner));
    _dirty = false;
  }

  static Future<InMemorySignalProtocolStore> _deserializeState(String raw) async {
    final json = jsonDecode(raw) as Map<String, dynamic>;
    final identity = IdentityKeyPair.fromSerialized(
      base64Decode(json['identity_key_pair'] as String),
    );
    final registrationId = json['registration_id'] as int;
    final store = InMemorySignalProtocolStore(identity, registrationId);

    final signedKeys = json['signed_pre_keys'] as Map<String, dynamic>? ?? {};
    for (final entry in signedKeys.entries) {
      await store.storeSignedPreKey(
        int.parse(entry.key),
        SignedPreKeyRecord.fromSerialized(base64Decode(entry.value as String)),
      );
    }

    final preKeys = json['pre_keys'] as Map<String, dynamic>? ?? {};
    for (final entry in preKeys.entries) {
      await store.storePreKey(
        int.parse(entry.key),
        PreKeyRecord.fromBuffer(base64Decode(entry.value as String)),
      );
    }
    return store;
  }

  static Future<String> _serializeState(InMemorySignalProtocolStore inner) async {
    final identity = await inner.getIdentityKeyPair();
    final registrationId = await inner.getLocalRegistrationId();
    final signedPreKeys = <String, String>{};
    for (final record in await inner.loadSignedPreKeys()) {
      signedPreKeys['${record.id}'] = base64Encode(record.serialize());
    }
    final preKeys = <String, String>{};
    for (var id = 1; id <= 110; id++) {
      if (await inner.containsPreKey(id)) {
        final record = await inner.loadPreKey(id);
        preKeys['$id'] = base64Encode(record.serialize());
      }
    }
    return jsonEncode({
      'identity_key_pair': base64Encode(identity.serialize()),
      'registration_id': registrationId,
      'signed_pre_keys': signedPreKeys,
      'pre_keys': preKeys,
    });
  }

  void _markDirty() {
    _dirty = true;
  }

  @override
  Future<IdentityKeyPair> getIdentityKeyPair() => _inner.getIdentityKeyPair();

  @override
  Future<int> getLocalRegistrationId() => _inner.getLocalRegistrationId();

  @override
  Future<bool> saveIdentity(
    SignalProtocolAddress address,
    IdentityKey? identityKey,
  ) async {
    _markDirty();
    return _inner.saveIdentity(address, identityKey);
  }

  @override
  Future<bool> isTrustedIdentity(
    SignalProtocolAddress address,
    IdentityKey? identityKey,
    Direction direction,
  ) => _inner.isTrustedIdentity(address, identityKey, direction);

  @override
  Future<IdentityKey?> getIdentity(SignalProtocolAddress address) =>
      _inner.getIdentity(address);

  @override
  Future<PreKeyRecord> loadPreKey(int preKeyId) => _inner.loadPreKey(preKeyId);

  @override
  Future<void> storePreKey(int preKeyId, PreKeyRecord record) async {
    _markDirty();
    await _inner.storePreKey(preKeyId, record);
  }

  @override
  Future<bool> containsPreKey(int preKeyId) => _inner.containsPreKey(preKeyId);

  @override
  Future<void> removePreKey(int preKeyId) async {
    _markDirty();
    await _inner.removePreKey(preKeyId);
  }

  @override
  Future<SessionRecord> loadSession(SignalProtocolAddress address) =>
      _inner.loadSession(address);

  @override
  Future<List<int>> getSubDeviceSessions(String name) =>
      _inner.getSubDeviceSessions(name);

  @override
  Future<void> storeSession(
    SignalProtocolAddress address,
    SessionRecord record,
  ) async {
    _markDirty();
    await _inner.storeSession(address, record);
  }

  @override
  Future<bool> containsSession(SignalProtocolAddress address) =>
      _inner.containsSession(address);

  @override
  Future<void> deleteSession(SignalProtocolAddress address) async {
    _markDirty();
    await _inner.deleteSession(address);
  }

  @override
  Future<void> deleteAllSessions(String name) async {
    _markDirty();
    await _inner.deleteAllSessions(name);
  }

  @override
  Future<SignedPreKeyRecord> loadSignedPreKey(int signedPreKeyId) =>
      _inner.loadSignedPreKey(signedPreKeyId);

  @override
  Future<List<SignedPreKeyRecord>> loadSignedPreKeys() =>
      _inner.loadSignedPreKeys();

  @override
  Future<void> storeSignedPreKey(
    int signedPreKeyId,
    SignedPreKeyRecord record,
  ) async {
    _markDirty();
    await _inner.storeSignedPreKey(signedPreKeyId, record);
  }

  @override
  Future<bool> containsSignedPreKey(int signedPreKeyId) =>
      _inner.containsSignedPreKey(signedPreKeyId);

  @override
  Future<void> removeSignedPreKey(int signedPreKeyId) async {
    _markDirty();
    await _inner.removeSignedPreKey(signedPreKeyId);
  }
}

/// Opens persistent stores on native; in-memory on web unit tests unless storage injected.
Future<SignalProtocolStore> openDefaultSignalStore(String profileId) async {
  if (kIsWeb) {
    return SecureSignalStore.open(profileId: profileId);
  }
  return SecureSignalStore.open(profileId: profileId);
}
