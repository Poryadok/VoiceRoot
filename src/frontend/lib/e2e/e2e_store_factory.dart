import 'dart:convert';

import 'package:libsignal_protocol_dart/libsignal_protocol_dart.dart';

const int _defaultDeviceId = 1;
const int _defaultSignedPreKeyId = 1;
const int _otpkPoolStartId = 1;
const int otpkPoolSize = 10;
const int otpkReplenishThreshold = 3;

/// Creates an in-memory Signal store with identity + pre-keys loaded.
Future<InMemorySignalProtocolStore> createInitializedSignalStore() async {
  final identityKeyPair = generateIdentityKeyPair();
  final registrationId = generateRegistrationId(false);
  final store = InMemorySignalProtocolStore(identityKeyPair, registrationId);

  final signedPreKey = generateSignedPreKey(identityKeyPair, _defaultSignedPreKeyId);
  await store.storeSignedPreKey(_defaultSignedPreKeyId, signedPreKey);

  final preKeys = generatePreKeys(_otpkPoolStartId, otpkPoolSize);
  for (final preKey in preKeys) {
    await store.storePreKey(preKey.id, preKey);
  }

  return store;
}

SignalProtocolAddress signalAddressForProfile(String profileId) {
  return SignalProtocolAddress(profileId, _defaultDeviceId);
}

/// Ensures at least [otpkPoolSize] one-time pre-keys exist in [store].
Future<void> replenishOneTimePreKeysIfNeeded(SignalProtocolStore store) async {
  var available = 0;
  for (var id = _otpkPoolStartId; id < _otpkPoolStartId + otpkPoolSize; id++) {
    try {
      await store.loadPreKey(id);
      available++;
    } catch (_) {
      break;
    }
  }
  if (available >= otpkReplenishThreshold) return;

  final startId = _otpkPoolStartId + otpkPoolSize;
  final preKeys = generatePreKeys(startId, otpkPoolSize - available);
  for (final preKey in preKeys) {
    await store.storePreKey(preKey.id, preKey);
  }
}

/// Builds a [PreKeyBundle] from keys currently in [store] (first available OTPK).
Future<PreKeyBundle> exportPreKeyBundle(SignalProtocolStore store) async {
  final registrationId = await store.getLocalRegistrationId();
  final identityKeyPair = await store.getIdentityKeyPair();
  final signedPreKey = await store.loadSignedPreKey(_defaultSignedPreKeyId);
  final preKey = await store.loadPreKey(_otpkPoolStartId);

  return PreKeyBundle(
    registrationId,
    _defaultDeviceId,
    _otpkPoolStartId,
    preKey.getKeyPair().publicKey,
    _defaultSignedPreKeyId,
    signedPreKey.getKeyPair().publicKey,
    signedPreKey.signature,
    identityKeyPair.getPublicKey(),
  );
}

Future<List<Map<String, dynamic>>> _collectStoredPreKeys(
  SignalProtocolStore store,
) async {
  final out = <Map<String, dynamic>>[];
  for (var id = _otpkPoolStartId; id < _otpkPoolStartId + otpkPoolSize * 2; id++) {
    try {
      final preKey = await store.loadPreKey(id);
      out.add({
        'pre_key_id': id,
        'pre_key_public': base64Encode(preKey.getKeyPair().publicKey.serialize()),
      });
    } catch (_) {
      if (out.isNotEmpty) break;
    }
  }
  return out;
}

/// Opaque base64 blob uploaded to Messaging pre-key directory.
Future<String> serializePreKeyBundle(SignalProtocolStore store) async {
  final bundle = await exportPreKeyBundle(store);
  final pool = await _collectStoredPreKeys(store);
  final payload = <String, dynamic>{
    'registration_id': bundle.getRegistrationId(),
    'device_id': bundle.getDeviceId(),
    'pre_key_id': bundle.getPreKeyId(),
    'pre_key_public': base64Encode(bundle.getPreKey()!.serialize()),
    'signed_pre_key_id': bundle.getSignedPreKeyId(),
    'signed_pre_key_public': base64Encode(bundle.getSignedPreKey()!.serialize()),
    'signed_pre_key_signature': base64Encode(bundle.getSignedPreKeySignature()!),
    'identity_key': base64Encode(bundle.getIdentityKey().serialize()),
    if (pool.isNotEmpty) 'pre_keys': pool,
  };
  return base64Encode(utf8.encode(jsonEncode(payload)));
}

/// Restores a bundle produced by [serializePreKeyBundle].
PreKeyBundle? parseSerializedPreKeyBundle(String wire) {
  try {
    final decoded = utf8.decode(base64Decode(wire));
    final json = jsonDecode(decoded) as Map<String, dynamic>;
    return PreKeyBundle(
      json['registration_id'] as int,
      json['device_id'] as int,
      json['pre_key_id'] as int,
      _publicKeyFromBase64(json['pre_key_public'] as String),
      json['signed_pre_key_id'] as int,
      _publicKeyFromBase64(json['signed_pre_key_public'] as String),
      base64Decode(json['signed_pre_key_signature'] as String),
      IdentityKey.fromBytes(
        base64Decode(json['identity_key'] as String),
        0,
      ),
    );
  } catch (_) {
    return null;
  }
}

ECPublicKey _publicKeyFromBase64(String encoded) {
  return Curve.decodePoint(base64Decode(encoded), 0);
}

/// Ensures bilateral sessions exist between [localStore] and [remoteStore].
Future<void> establishBilateralSession({
  required InMemorySignalProtocolStore localStore,
  required InMemorySignalProtocolStore remoteStore,
  required SignalProtocolAddress localAddress,
  required SignalProtocolAddress remoteAddress,
}) async {
  if (!await localStore.containsSession(remoteAddress)) {
    final remoteBundle = await exportPreKeyBundle(remoteStore);
    await SessionBuilder.fromSignalStore(localStore, remoteAddress)
        .processPreKeyBundle(remoteBundle);
  }
  if (!await remoteStore.containsSession(localAddress)) {
    final localBundle = await exportPreKeyBundle(localStore);
    await SessionBuilder.fromSignalStore(remoteStore, localAddress)
        .processPreKeyBundle(localBundle);
  }
}
