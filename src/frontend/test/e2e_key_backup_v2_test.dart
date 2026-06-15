import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/e2e/e2e_key_backup_v2.dart';

/// Batch E2E-A audit: AEAD key backup codec v2 (docs/features/encryption.md, docs/TODO.md E2E-B).
void main() {
  group('E2eKeyBackupCodecV2', () {
    final codec = E2eKeyBackupCodecV2();

    test('encrypt uses voice-e2e-backup-v2 magic', () async {
      final blob = await codec.encryptPayload(
        password: 'phase15-backup-pass',
        payload: const {'profile_id': 'p1', 'version': 2},
      );

      final outer = jsonDecode(utf8.decode(base64Decode(blob))) as Map<String, dynamic>;
      expect(outer['magic'], E2eKeyBackupCodecV2.magic);
      expect(outer['magic'], 'voice-e2e-backup-v2');
      expect(outer['blob'], isNotEmpty);
      expect(outer['salt'], isNotEmpty);
      expect(outer['nonce'], isNotEmpty);
    });

    test('encrypt then decrypt restores payload', () async {
      const payload = {'profile_id': 'p2', 'version': 2, 'sessions': <dynamic>[]};
      final blob = await codec.encryptPayload(password: 'correct-horse', payload: payload);
      final restored = await codec.decryptPayload(password: 'correct-horse', encryptedBlob: blob);
      expect(restored['profile_id'], 'p2');
      expect(restored['version'], 2);
    });

    test('wrong password fails decrypt', () async {
      final blob = await codec.encryptPayload(
        password: 'right-password',
        payload: const {'k': 'v'},
      );
      await expectLater(
        codec.decryptPayload(password: 'wrong-password', encryptedBlob: blob),
        throwsA(isA<Exception>()),
      );
    });

    test('rejects v1 backup blobs', () async {
      const v1Outer = {
        'magic': 'voice-e2e-backup-v1',
        'salt': 'c2FsdA==',
        'blob': 'Ym9sYg==',
      };
      final v1Blob = base64Encode(utf8.encode(jsonEncode(v1Outer)));
      expect(
        () => codec.decryptPayload(password: 'any', encryptedBlob: v1Blob),
        throwsA(isA<FormatException>()),
      );
    });
  });
}
