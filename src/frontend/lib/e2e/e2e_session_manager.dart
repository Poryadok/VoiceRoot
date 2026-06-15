import 'package:libsignal_protocol_dart/libsignal_protocol_dart.dart';

import 'e2e_session.dart';
import 'e2e_store_factory.dart';
import 'secure_signal_store.dart';

typedef SignalStoreFactory = Future<SignalProtocolStore> Function(
  String profileId,
);

/// Per-profile Signal stores and pairwise session establishment.
class E2eSessionManager {
  E2eSessionManager({SignalStoreFactory? storeFactory})
      : _storeFactory = storeFactory ?? _defaultStoreFactory;

  factory E2eSessionManager.inMemory() =>
      E2eSessionManager(storeFactory: _inMemoryStoreFactory);

  final SignalStoreFactory _storeFactory;
  final Map<String, SignalProtocolStore> _storesByProfile = {};

  static final Map<String, InMemorySignalProtocolStore> _inMemoryOnly = {};

  static Future<SignalProtocolStore> _defaultStoreFactory(
    String profileId,
  ) async {
    return SecureSignalStore.open(profileId: profileId);
  }

  static Future<SignalProtocolStore> _inMemoryStoreFactory(
    String profileId,
  ) async {
    final existing = _inMemoryOnly[profileId];
    if (existing != null) return existing;
    final created = await createInitializedSignalStore();
    _inMemoryOnly[profileId] = created;
    return created;
  }

  Future<SignalProtocolStore> storeForProfile(String profileId) async {
    final existing = _storesByProfile[profileId];
    if (existing != null) return existing;
    final created = await _storeFactory(profileId);
    _storesByProfile[profileId] = created;
    return created;
  }

  Future<E2eSession> ensureSession({
    required String localProfileId,
    required String remoteProfileId,
    PreKeyBundle? remoteBundle,
  }) async {
    final localStore = await storeForProfile(localProfileId);
    final localAddress = signalAddressForProfile(localProfileId);
    final remoteAddress = signalAddressForProfile(remoteProfileId);

    if (remoteBundle != null &&
        !await localStore.containsSession(remoteAddress)) {
      await SessionBuilder.fromSignalStore(localStore, remoteAddress)
          .processPreKeyBundle(remoteBundle);
      if (localStore is SecureSignalStore) {
        await localStore.close();
      }
    }

    return E2eSession(
      localProfileId: localProfileId,
      remoteProfileId: remoteProfileId,
      localStore: localStore,
      remoteStore: localStore,
      localAddress: localAddress,
      remoteAddress: remoteAddress,
    );
  }

  SignalProtocolStore? peekStore(String profileId) =>
      _storesByProfile[profileId];
}
