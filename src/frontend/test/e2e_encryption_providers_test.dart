import 'dart:convert';
import 'dart:typed_data';

import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/e2e/e2e_crypto_adapter.dart';

/// Phase 15 red tests: client-side Signal encrypt/decrypt adapter (lib/e2e).
void main() {
  group('E2eCryptoAdapter', () {
    late E2eCryptoAdapter adapter;

    setUp(() {
      adapter = E2eCryptoAdapter.inMemoryForTest();
    });

    test('encrypt then decrypt roundtrips plaintext', () async {
      const plaintext = 'phase15-dm-secret';
      final session = await adapter.ensureSession(
        localProfileId: 'profile-a',
        remoteProfileId: 'profile-b',
      );

      final ciphertext = await adapter.encrypt(
        session: session,
        plaintext: plaintext,
      );
      expect(ciphertext, isNot(equals(plaintext)));

      final decrypted = await adapter.decrypt(
        session: session,
        ciphertext: ciphertext,
      );
      expect(decrypted, plaintext);
    });

    test('encrypt produces opaque bytes distinct from plaintext', () async {
      const plaintext = 'not-in-ciphertext';
      final session = await adapter.ensureSession(
        localProfileId: 'local',
        remoteProfileId: 'remote',
      );

      final ciphertext = await adapter.encrypt(
        session: session,
        plaintext: plaintext,
      );
      final decoded = utf8.decode(ciphertext);
      expect(decoded.contains(plaintext), isFalse);
      expect(ciphertext, isA<Uint8List>());
      expect(ciphertext, isNotEmpty);
    });

    test('decrypt rejects tampered ciphertext', () async {
      final session = await adapter.ensureSession(
        localProfileId: 'local',
        remoteProfileId: 'remote',
      );
      final ciphertext = await adapter.encrypt(
        session: session,
        plaintext: 'tamper-me',
      );
      final tampered = Uint8List.fromList(ciphertext)..[0] ^= 0xff;

      expect(
        () => adapter.decrypt(session: session, ciphertext: tampered),
        throwsA(isA<E2eDecryptException>()),
      );
    });
  });
}
