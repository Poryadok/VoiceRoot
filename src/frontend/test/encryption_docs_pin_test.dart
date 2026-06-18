import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

/// Docs pin: encryption.md documents libsignal version and golden fixtures.
void main() {
  test('encryption.md documents libsignal_protocol_dart pin and golden paths', () {
    final root = Directory.current;
    var dir = root;
    File? docsFile;
    for (var i = 0; i < 8; i++) {
      final candidate = File(
        '${dir.path}${Platform.pathSeparator}docs${Platform.pathSeparator}features${Platform.pathSeparator}encryption.md',
      );
      if (candidate.existsSync()) {
        docsFile = candidate;
        break;
      }
      final parent = dir.parent;
      if (parent.path == dir.path) break;
      dir = parent;
    }
    expect(docsFile, isNotNull, reason: 'encryption.md not found from ${root.path}');
    final body = docsFile!.readAsStringSync();
    expect(body, contains('libsignal_protocol_dart'));
    expect(body, contains('^0.8.0'));
    expect(body, contains('prekey_libsignal_golden.b64'));
    expect(body, contains('e2e_ciphertext_libsignal_golden.b64'));
  });
}
