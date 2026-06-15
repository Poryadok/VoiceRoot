import 'dart:convert';
import 'dart:math';
import 'dart:typed_data';

import 'package:pointycastle/export.dart' as pc;

/// Password-wrapped AEAD export of local Signal store state (opaque to server).
class E2eKeyBackupCodecV2 {
  E2eKeyBackupCodecV2();

  static const magic = 'voice-e2e-backup-v2';
  static const _pbkdf2Iterations = 100000;

  Future<String> encryptPayload({
    required String password,
    required Map<String, dynamic> payload,
    String? passwordHint,
  }) async {
    final salt = _randomBytes(16);
    final nonce = _randomBytes(12);
    final keyBytes = _deriveKey(password, salt);
    final plaintext = Uint8List.fromList(utf8.encode(jsonEncode(payload)));
    final cipher = pc.GCMBlockCipher(pc.AESEngine())
      ..init(
        true,
        pc.AEADParameters(
          pc.KeyParameter(keyBytes),
          128,
          nonce,
          Uint8List(0),
        ),
      );
    final encrypted = cipher.process(plaintext);
    return base64Encode(
      utf8.encode(
        jsonEncode({
          'magic': magic,
          'salt': base64Encode(salt),
          'nonce': base64Encode(nonce),
          if (passwordHint != null && passwordHint.isNotEmpty)
            'password_hint': passwordHint,
          'blob': base64Encode(encrypted),
        }),
      ),
    );
  }

  Future<Map<String, dynamic>> decryptPayload({
    required String password,
    required String encryptedBlob,
  }) async {
    final outer = jsonDecode(utf8.decode(base64Decode(encryptedBlob)))
        as Map<String, dynamic>;
    if (outer['magic'] != magic) {
      throw const FormatException('invalid backup format');
    }
    final salt = base64Decode(outer['salt'] as String);
    final nonce = base64Decode(outer['nonce'] as String);
    final ciphertext = base64Decode(outer['blob'] as String);
    final keyBytes = _deriveKey(password, salt);
    final cipher = pc.GCMBlockCipher(pc.AESEngine())
      ..init(
        false,
        pc.AEADParameters(
          pc.KeyParameter(keyBytes),
          128,
          nonce,
          Uint8List(0),
        ),
      );
    final clearBytes = cipher.process(ciphertext);
    return jsonDecode(utf8.decode(clearBytes)) as Map<String, dynamic>;
  }

  Uint8List _deriveKey(String password, Uint8List salt) {
    final derivator = pc.PBKDF2KeyDerivator(pc.HMac(pc.SHA256Digest(), 64))
      ..init(pc.Pbkdf2Parameters(salt, _pbkdf2Iterations, 32));
    return Uint8List.fromList(
      derivator.process(Uint8List.fromList(utf8.encode(password))),
    );
  }

  Uint8List _randomBytes(int length) {
    final random = Random.secure();
    return Uint8List.fromList(
      List<int>.generate(length, (_) => random.nextInt(256)),
    );
  }
}
