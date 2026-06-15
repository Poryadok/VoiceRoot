import 'dart:convert';
import 'dart:math';
import 'dart:typed_data';

import 'package:pointycastle/export.dart' as pc;

import 'e2e_message_service.dart';

/// Client-side AES-GCM encryption for E2E chat file uploads (docs/features/encryption.md).
class E2eFileCrypto {
  const E2eFileCrypto();

  static const _keyWirePrefix = 'voice-e2e-file-key-v1:';

  Future<E2eEncryptedFilePayload> encryptBytes({
    required Uint8List plaintext,
    required E2eMessageService messageService,
    required String localProfileId,
    required String peerProfileId,
    required String authorization,
    required String chatId,
  }) async {
    final key = _randomBytes(32);
    final nonce = _randomBytes(12);
    final ciphertext = _aesGcmEncrypt(key: key, nonce: nonce, plaintext: plaintext);
    final keyJson = jsonEncode({
      'k': base64Encode(key),
      'n': base64Encode(nonce),
    });
    final keyWire = await messageService.encryptOutgoing(
      localProfileId: localProfileId,
      peerProfileId: peerProfileId,
      plaintext: '$_keyWirePrefix$keyJson',
      authorization: authorization,
      chatId: chatId,
    );
    return E2eEncryptedFilePayload(
      ciphertext: ciphertext,
      keyWire: keyWire,
    );
  }

  Future<Uint8List> decryptBytes({
    required Uint8List ciphertext,
    required String keyWire,
    required E2eMessageService messageService,
    required String localProfileId,
    required String peerProfileId,
    String? authorization,
  }) async {
    final decrypted = await messageService.adapter.decryptFromWire(
      receiverProfileId: localProfileId,
      senderProfileId: peerProfileId,
      wire: keyWire,
    );
    if (!decrypted.startsWith(_keyWirePrefix)) {
      throw const FormatException('invalid e2e file key wire');
    }
    final payload = jsonDecode(decrypted.substring(_keyWirePrefix.length))
        as Map<String, dynamic>;
    final key = base64Decode(payload['k'] as String);
    final nonce = base64Decode(payload['n'] as String);
    return _aesGcmDecrypt(
      key: Uint8List.fromList(key),
      nonce: Uint8List.fromList(nonce),
      ciphertext: ciphertext,
    );
  }

  Uint8List _aesGcmEncrypt({
    required Uint8List key,
    required Uint8List nonce,
    required Uint8List plaintext,
  }) {
    final cipher = pc.GCMBlockCipher(pc.AESEngine())
      ..init(
        true,
        pc.AEADParameters(pc.KeyParameter(key), 128, nonce, Uint8List(0)),
      );
    return Uint8List.fromList(cipher.process(plaintext));
  }

  Uint8List _aesGcmDecrypt({
    required Uint8List key,
    required Uint8List nonce,
    required Uint8List ciphertext,
  }) {
    final cipher = pc.GCMBlockCipher(pc.AESEngine())
      ..init(
        false,
        pc.AEADParameters(pc.KeyParameter(key), 128, nonce, Uint8List(0)),
      );
    return Uint8List.fromList(cipher.process(ciphertext));
  }

  Uint8List _randomBytes(int length) {
    final random = Random.secure();
    return Uint8List.fromList(
      List<int>.generate(length, (_) => random.nextInt(256)),
    );
  }
}

class E2eEncryptedFilePayload {
  const E2eEncryptedFilePayload({
    required this.ciphertext,
    required this.keyWire,
  });

  final Uint8List ciphertext;
  final String keyWire;
}
