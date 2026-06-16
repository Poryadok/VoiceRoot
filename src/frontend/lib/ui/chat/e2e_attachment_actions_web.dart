// ignore: avoid_web_libraries_in_flutter, deprecated_member_use
import 'dart:html' as html;
import 'dart:typed_data';

Future<bool> saveDecryptedE2eAttachment({
  required Uint8List bytes,
  required String fileName,
}) async {
  try {
    final blob = html.Blob([bytes]);
    final url = html.Url.createObjectUrlFromBlob(blob);
    final anchor = html.AnchorElement(href: url)
      ..download = _safeFileName(fileName)
      ..style.display = 'none';
    html.document.body?.children.add(anchor);
    anchor.click();
    anchor.remove();
    html.Url.revokeObjectUrl(url);
    return true;
  } on Object {
    return false;
  }
}

String _safeFileName(String raw) {
  final trimmed = raw.trim();
  if (trimmed.isEmpty) return 'download';
  final parts = trimmed.split(RegExp(r'[\\/]'));
  final base = parts.isEmpty ? trimmed : parts.last;
  return base.isEmpty ? 'download' : base;
}
