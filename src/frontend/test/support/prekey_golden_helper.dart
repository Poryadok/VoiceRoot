import 'dart:io';
import 'dart:typed_data';

import 'package:fixnum/fixnum.dart';
import 'package:libsignal_protocol_dart/libsignal_protocol_dart.dart';
import 'package:voice_frontend/e2e/e2e_crypto_adapter.dart';
import 'package:voice_frontend/e2e/e2e_session_manager.dart';
import 'package:voice_frontend/e2e/e2e_store_factory.dart';

import 'live_gateway_harness.dart';

/// Plaintext encrypted into [e2eCiphertextGoldenFile] for Go compose live tests.
const kE2eCiphertextGoldenPlaintext = 'phase15-golden-plaintext';

const _goldenReceiverProfileId = 'golden-receiver';
const _goldenSenderProfileId = 'golden-sender';

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

File? prekeyPeerGoldenFile() {
  final root = liveRepoRoot();
  if (root == null) return null;
  return File(
    '$root${Platform.pathSeparator}src${Platform.pathSeparator}backend'
    '${Platform.pathSeparator}messaging${Platform.pathSeparator}testfixture'
    '${Platform.pathSeparator}prekey_libsignal_golden_peer.b64',
  );
}

File? prekeyPeerGoldenComposeFile() {
  final root = liveRepoRoot();
  if (root == null) return null;
  return File(
    '$root${Platform.pathSeparator}src${Platform.pathSeparator}backend'
    '${Platform.pathSeparator}pkg${Platform.pathSeparator}composefixture'
    '${Platform.pathSeparator}prekey_libsignal_golden_peer.b64',
  );
}

Future<String> wireFromDeterministicSenderStore() async {
  final store = await createDeterministicSenderStore();
  return serializePreKeyBundle(store);
}

Future<String> wireFromDeterministicStore() async {
  final store = await createDeterministicSignalStore();
  return serializePreKeyBundle(store);
}

/// Receiver store paired with [createDeterministicSenderStore] for ciphertext golden.
Future<InMemorySignalProtocolStore> createDeterministicSenderStore() async {
  final identityPrivate = Uint8List.fromList(
    List<int>.generate(32, (i) => 0xc5 ^ i),
  );
  final identityKeyPair = generateIdentityKeyPairFromPrivate(identityPrivate);
  const registrationId = 42_002;
  final store = InMemorySignalProtocolStore(identityKeyPair, registrationId);

  final signedPreKeyPrivate = Uint8List.fromList(
    List<int>.generate(32, (i) => 0xd6 ^ i),
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
      List<int>.generate(32, (i) => (id * 23) ^ i),
    );
    final keyPair = Curve.generateKeyPairFromPrivate(priv);
    await store.storePreKey(id, PreKeyRecord(id, keyPair));
  }

  return store;
}

File? e2eCiphertextGoldenFile() {
  final root = liveRepoRoot();
  if (root == null) return null;
  return File(
    '$root${Platform.pathSeparator}src${Platform.pathSeparator}backend'
    '${Platform.pathSeparator}messaging${Platform.pathSeparator}testfixture'
    '${Platform.pathSeparator}e2e_ciphertext_libsignal_golden.b64',
  );
}

File? e2eCiphertextGoldenComposeFile() {
  final root = liveRepoRoot();
  if (root == null) return null;
  return File(
    '$root${Platform.pathSeparator}src${Platform.pathSeparator}backend'
    '${Platform.pathSeparator}pkg${Platform.pathSeparator}composefixture'
    '${Platform.pathSeparator}e2e_ciphertext_libsignal_golden.b64',
  );
}

/// Libsignal ciphertext wire from deterministic sender → receiver (pre-key golden peer).
Future<String> ciphertextFromDeterministicStores() async {
  final receiverStore = await createDeterministicSignalStore();
  final senderStore = await createDeterministicSenderStore();
  final adapter = _goldenCryptoAdapter(receiverStore, senderStore);

  final remoteBundle = await exportPreKeyBundle(receiverStore);
  await adapter.ensureSession(
    localProfileId: _goldenSenderProfileId,
    remoteProfileId: _goldenReceiverProfileId,
    remoteBundle: remoteBundle,
  );
  final session = await adapter.ensureSession(
    localProfileId: _goldenSenderProfileId,
    remoteProfileId: _goldenReceiverProfileId,
  );
  return adapter.encryptToWire(
    session: session,
    plaintext: kE2eCiphertextGoldenPlaintext,
  );
}

/// Decrypts [wire] on the deterministic receiver store (golden round-trip check).
Future<String> decryptCiphertextOnDeterministicReceiver(String wire) async {
  final receiverStore = await createDeterministicSignalStore();
  final senderStore = await createDeterministicSenderStore();
  final adapter = _goldenCryptoAdapter(receiverStore, senderStore);
  return adapter.decryptFromWire(
    receiverProfileId: _goldenReceiverProfileId,
    senderProfileId: _goldenSenderProfileId,
    wire: wire,
  );
}

E2eCryptoAdapter _goldenCryptoAdapter(
  InMemorySignalProtocolStore receiverStore,
  InMemorySignalProtocolStore senderStore,
) {
  Future<SignalProtocolStore> factory(String profileId) async {
    if (profileId == _goldenReceiverProfileId) return receiverStore;
    if (profileId == _goldenSenderProfileId) return senderStore;
    throw ArgumentError.value(profileId, 'profileId', 'unknown golden profile');
  }

  return E2eCryptoAdapter(
    sessionManager: E2eSessionManager(storeFactory: factory),
  );
}
