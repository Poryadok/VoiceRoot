import 'dart:convert';
import 'dart:typed_data';

import 'package:crypto/crypto.dart';

/// Password-wrapped export of local Signal store state (opaque to server).
class E2eKeyBackupCodec {
  const E2eKeyBackupCodec();

  static const _magic = 'voice-e2e-backup-v1';

  String encryptPayload({
    required String password,
    required Map<String, dynamic> payload,
    String? passwordHint,
  }) {
    final salt = _randomBytes(16);
    final key = _deriveKey(password, salt);
    final plaintext = utf8.encode(jsonEncode(payload));
    final encrypted = _xorBytes(Uint8List.fromList(plaintext), key);
    return base64Encode(
      utf8.encode(
        jsonEncode({
          'magic': _magic,
          'salt': base64Encode(salt),
          if (passwordHint != null && passwordHint.isNotEmpty)
            'password_hint': passwordHint,
          'blob': base64Encode(encrypted),
        }),
      ),
    );
  }

  Map<String, dynamic> decryptPayload({
    required String password,
    required String encryptedBlob,
  }) {
    final outer = jsonDecode(utf8.decode(base64Decode(encryptedBlob)))
        as Map<String, dynamic>;
    if (outer['magic'] != _magic) {
      throw const FormatException('invalid backup format');
    }
    final salt = base64Decode(outer['salt'] as String);
    final key = _deriveKey(password, salt);
    final encrypted = base64Decode(outer['blob'] as String);
    final plaintext = _xorBytes(encrypted, key);
    return jsonDecode(utf8.decode(plaintext)) as Map<String, dynamic>;
  }

  Uint8List _deriveKey(String password, Uint8List salt) {
    var material = sha256.convert([...utf8.encode(password), ...salt]).bytes;
    for (var i = 0; i < 10000; i++) {
      material = sha256.convert([...material, ...salt]).bytes;
    }
    return Uint8List.fromList(material);
  }

  Uint8List _xorBytes(Uint8List input, Uint8List key) {
    final out = Uint8List(input.length);
    for (var i = 0; i < input.length; i++) {
      out[i] = input[i] ^ key[i % key.length];
    }
    return out;
  }

  Uint8List _randomBytes(int length) {
    final seed = sha256.convert(utf8.encode('${DateTime.now().microsecondsSinceEpoch}')).bytes;
    final bytes = List<int>.generate(length, (i) => seed[i % seed.length] ^ (i * 17));
    return Uint8List.fromList(bytes);
  }
}
