import 'dart:io';
import 'dart:typed_data';

import 'package:fixnum/fixnum.dart';
import 'package:libsignal_protocol_dart/libsignal_protocol_dart.dart';
import 'package:voice_frontend/e2e/e2e_store_factory.dart';

import 'live_gateway_harness.dart';

/// Deterministic libsignal store for cross-language pre-key golden export/check.
Future<InMemorySignalProtocolStore> createDeterministicSignalStore() async {
  final identityPrivate = Uint8List.fromList(
    List<int>.generate(32, (i) => 0xa3 ^ i),
  );
  final identityKeyPair = generateIdentityKeyPairFromPrivate(identityPrivate);
  const registrationId = 42_001;
  final store = InMemorySignalProtocolStore(identityKeyPair, registrationId);

  final signedPreKeyPrivate = Uint8List.fromList(
    List<int>.generate(32, (i) => 0xb4 ^ i),
  );
  final signedPreKeyPair = Curve.generateKeyPairFromPrivate(signedPreKeyPrivate);
  final signature = Curve.calculateSignature(
    identityKeyPair.getPrivateKey(),
    signedPreKeyPair.publicKey.serialize(),
  );
  await store.storeSignedPreKey(
    1,
    SignedPreKeyRecord(1, Int64(1), signedPreKeyPair, signature),
  );

  for (var id = 1; id <= otpkPoolSize; id++) {
    final priv = Uint8List.fromList(
      List<int>.generate(32, (i) => (id * 17) ^ i),
    );
    final keyPair = Curve.generateKeyPairFromPrivate(priv);
    await store.storePreKey(id, PreKeyRecord(id, keyPair));
  }

  return store;
}

File? prekeyGoldenFile() {
  final root = liveRepoRoot();
  if (root == null) return null;
  return File(
    '$root${Platform.pathSeparator}src${Platform.pathSeparator}backend'
    '${Platform.pathSeparator}messaging${Platform.pathSeparator}testfixture'
    '${Platform.pathSeparator}prekey_libsignal_golden.b64',
  );
}

Future<String> wireFromDeterministicStore() async {
  final store = await createDeterministicSignalStore();
  return serializePreKeyBundle(store);
}
