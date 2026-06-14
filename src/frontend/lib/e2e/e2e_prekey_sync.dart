import 'package:libsignal_protocol_dart/libsignal_protocol_dart.dart';

import 'e2e_session_manager.dart';
import 'e2e_store_factory.dart';

/// Upload/download Signal pre-key bundles via Messaging API.
class E2ePreKeySync {
  const E2ePreKeySync({required E2eSessionManager sessionManager})
      : _sessions = sessionManager;

  final E2eSessionManager _sessions;

  Future<String> bundleForProfile(String profileId) async {
    final store = await _sessions.storeForProfile(profileId);
    return serializePreKeyBundle(store);
  }

  PreKeyBundle? bundleFromWire(String wire) => parseSerializedPreKeyBundle(wire);
}
