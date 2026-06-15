import 'dart:convert';
import 'dart:typed_data';

import 'package:libsignal_protocol_dart/libsignal_protocol_dart.dart';

export 'e2e_exceptions.dart';
import 'e2e_exceptions.dart';
import 'e2e_session.dart';
import 'e2e_session_manager.dart';

/// Client-side Signal encrypt/decrypt adapter (libsignal_protocol_dart).
class E2eCryptoAdapter {
  E2eCryptoAdapter({E2eSessionManager? sessionManager})
      : _sessions = sessionManager ?? E2eSessionManager();

  factory E2eCryptoAdapter.inMemoryForTest() =>
      E2eCryptoAdapter(sessionManager: E2eSessionManager.inMemory());

  final E2eSessionManager _sessions;

  E2eSessionManager get sessionManager => _sessions;

  Future<E2eSession> ensureSession({
    required String localProfileId,
    required String remoteProfileId,
    PreKeyBundle? remoteBundle,
  }) {
    return _sessions.ensureSession(
      localProfileId: localProfileId,
      remoteProfileId: remoteProfileId,
      remoteBundle: remoteBundle,
    );
  }

  Future<Uint8List> encrypt({
    required E2eSession session,
    required String plaintext,
  }) async {
    final cipher = SessionCipher.fromStore(
      session.localStore,
      session.remoteAddress,
    );
    final message = await cipher.encrypt(
      Uint8List.fromList(utf8.encode(plaintext)),
    );
    final wire = base64Encode(message.serialize());
    return Uint8List.fromList(utf8.encode(wire));
  }

  /// Decrypts a message sent on [session] from local → remote (receiver read path).
  Future<String> decrypt({
    required E2eSession session,
    required Uint8List ciphertext,
  }) async {
    final cipher = SessionCipher.fromStore(
      session.remoteStore,
      session.localAddress,
    );
    return _decryptWithCipher(cipher, ciphertext);
  }

  Future<String> decryptFromWire({
    required String receiverProfileId,
    required String senderProfileId,
    required String wire,
    PreKeyBundle? remoteBundle,
  }) async {
    final session = await ensureSession(
      localProfileId: receiverProfileId,
      remoteProfileId: senderProfileId,
      remoteBundle: remoteBundle,
    );
    final cipher = SessionCipher.fromStore(
      session.localStore,
      session.remoteAddress,
    );
    return _decryptWithCipher(cipher, Uint8List.fromList(utf8.encode(wire)));
  }

  Future<String> encryptToWire({
    required E2eSession session,
    required String plaintext,
  }) async {
    final bytes = await encrypt(session: session, plaintext: plaintext);
    return encodeWireCiphertext(bytes);
  }

  String encodeWireCiphertext(Uint8List bytes) => utf8.decode(bytes);

  Uint8List decodeWireCiphertext(String wire) =>
      Uint8List.fromList(utf8.encode(wire));

  Future<String> _decryptWithCipher(
    SessionCipher cipher,
    Uint8List ciphertext,
  ) async {
    try {
      final raw = _signalBytesFromWire(ciphertext);
      late final Uint8List plaintext;
      if (raw.isNotEmpty && raw[0] == CiphertextMessage.prekeyType) {
        plaintext = await cipher.decrypt(PreKeySignalMessage(raw));
      } else {
        try {
          plaintext = await cipher.decryptFromSignal(
            SignalMessage.fromSerialized(raw),
          );
        } on Exception {
          plaintext = await cipher.decrypt(PreKeySignalMessage(raw));
        }
      }
      return utf8.decode(plaintext);
    } catch (e) {
      throw E2eDecryptException(e);
    }
  }

  Uint8List _signalBytesFromWire(Uint8List ciphertext) {
    try {
      final asText = utf8.decode(ciphertext);
      return base64Decode(asText);
    } catch (_) {
      return ciphertext;
    }
  }
}
