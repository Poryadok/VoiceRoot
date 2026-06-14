import 'package:libsignal_protocol_dart/libsignal_protocol_dart.dart';

/// Handle to a bilateral Signal session between two profile ids.
class E2eSession {
  const E2eSession({
    required this.localProfileId,
    required this.remoteProfileId,
    required this.localStore,
    required this.remoteStore,
    required this.localAddress,
    required this.remoteAddress,
  });

  final String localProfileId;
  final String remoteProfileId;
  final SignalProtocolStore localStore;
  final SignalProtocolStore remoteStore;
  final SignalProtocolAddress localAddress;
  final SignalProtocolAddress remoteAddress;
}
