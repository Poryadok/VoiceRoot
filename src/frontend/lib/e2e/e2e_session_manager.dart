import 'package:libsignal_protocol_dart/libsignal_protocol_dart.dart';

import 'e2e_session.dart';
import 'e2e_store_factory.dart';

/// Per-profile Signal stores and pairwise session establishment.
class E2eSessionManager {
  E2eSessionManager();

  final Map<String, InMemorySignalProtocolStore> _storesByProfile = {};

  Future<InMemorySignalProtocolStore> storeForProfile(String profileId) async {
    final existing = _storesByProfile[profileId];
    if (existing != null) return existing;
    final created = await createInitializedSignalStore();
    _storesByProfile[profileId] = created;
    return created;
  }

  Future<E2eSession> ensureSession({
    required String localProfileId,
    required String remoteProfileId,
    PreKeyBundle? remoteBundle,
  }) async {
    final localStore = await storeForProfile(localProfileId);
    final remoteStore = await storeForProfile(remoteProfileId);
    final localAddress = signalAddressForProfile(localProfileId);
    final remoteAddress = signalAddressForProfile(remoteProfileId);

    if (remoteBundle != null &&
        !await localStore.containsSession(remoteAddress)) {
      await SessionBuilder.fromSignalStore(localStore, remoteAddress)
          .processPreKeyBundle(remoteBundle);
    }

    await establishBilateralSession(
      localStore: localStore,
      remoteStore: remoteStore,
      localAddress: localAddress,
      remoteAddress: remoteAddress,
    );

    return E2eSession(
      localProfileId: localProfileId,
      remoteProfileId: remoteProfileId,
      localStore: localStore,
      remoteStore: remoteStore,
      localAddress: localAddress,
      remoteAddress: remoteAddress,
    );
  }

  InMemorySignalProtocolStore? peekStore(String profileId) =>
      _storesByProfile[profileId];
}
