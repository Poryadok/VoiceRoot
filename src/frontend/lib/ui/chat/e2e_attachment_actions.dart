import 'dart:typed_data';

import 'e2e_attachment_actions_io.dart'
    if (dart.library.html) 'e2e_attachment_actions_web.dart' as impl;

/// Saves decrypted E2E attachment bytes on web (blob download) or desktop/mobile
/// (downloads/documents directory via path_provider).
Future<bool> saveDecryptedE2eAttachment({
  required Uint8List bytes,
  required String fileName,
}) {
  return impl.saveDecryptedE2eAttachment(bytes: bytes, fileName: fileName);
}
