import 'dart:io';
import 'dart:typed_data';

import 'package:path/path.dart' as p;
import 'package:path_provider/path_provider.dart';

Future<bool> saveDecryptedE2eAttachment({
  required Uint8List bytes,
  required String fileName,
}) async {
  try {
    final dir =
        await getDownloadsDirectory() ?? await getApplicationDocumentsDirectory();
    final safeName = _safeFileName(fileName);
    final file = File(p.join(dir.path, safeName));
    await file.writeAsBytes(bytes, flush: true);
    return true;
  } on Object {
    return false;
  }
}

String _safeFileName(String raw) {
  final trimmed = raw.trim();
  if (trimmed.isEmpty) return 'download';
  final base = p.basename(trimmed);
  return base.isEmpty ? 'download' : base;
}
