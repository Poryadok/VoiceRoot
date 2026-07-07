import 'dart:convert';
import 'dart:typed_data';

import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/e2e/e2e_crypto_adapter.dart';
import 'package:voice_frontend/e2e/e2e_store_factory.dart';

/// app stack5 red tests: client-side Signal encrypt/decrypt adapter (lib/e2e).
void main() {
  group('E2eCryptoAdapter', () {
    late E2eCryptoAdapter adapter;

    setUp(() {
      adapter = E2eCryptoAdapter.inMemoryForTest();
    });

    Future<void> establishPair({
      required String localProfileId,
      required String remoteProfileId,
    }) async {
      final remoteStore = await adapter.sessionManager.storeForProfile(
        remoteProfileId,
      );
      final remoteBundle = await exportPreKeyBundle(remoteStore);
      await adapter.ensureSession(
        localProfileId: localProfileId,
        remoteProfileId: remoteProfileId,
        remoteBundle: remoteBundle,
      );
    }

    test('encrypt then decrypt roundtrips plaintext', () async {
      const plaintext = 'phase15-dm-secret';
      await establishPair(localProfileId: 'profile-a', remoteProfileId: 'profile-b');

      final session = await adapter.ensureSession(
        localProfileId: 'profile-a',
        remoteProfileId: 'profile-b',
      );
      final wire = await adapter.encryptToWire(
        session: session,
        plaintext: plaintext,
      );
      expect(wire, isNot(equals(plaintext)));

      await establishPair(localProfileId: 'profile-b', remoteProfileId: 'profile-a');
      final decrypted = await adapter.decryptFromWire(
        receiverProfileId: 'profile-b',
        senderProfileId: 'profile-a',
        wire: wire,
      );
      expect(decrypted, plaintext);
    });

    test('encrypt produces opaque bytes distinct from plaintext', () async {
      const plaintext = 'not-in-ciphertext';
      await establishPair(localProfileId: 'local', remoteProfileId: 'remote');
      final session = await adapter.ensureSession(
        localProfileId: 'local',
        remoteProfileId: 'remote',
      );

      final wire = await adapter.encryptToWire(
        session: session,
        plaintext: plaintext,
      );
      expect(wire.contains(plaintext), isFalse);
      expect(wire, isNotEmpty);
      expect(utf8.encode(wire), isA<Uint8List>());
    });

    test('decrypt rejects tampered ciphertext', () async {
      await establishPair(localProfileId: 'local', remoteProfileId: 'remote');
      final session = await adapter.ensureSession(
        localProfileId: 'local',
        remoteProfileId: 'remote',
      );
      final wire = await adapter.encryptToWire(
        session: session,
        plaintext: 'tamper-me',
      );
      final tampered = '${wire}x';

      await establishPair(localProfileId: 'remote', remoteProfileId: 'local');
      expect(
        () => adapter.decryptFromWire(
          receiverProfileId: 'remote',
          senderProfileId: 'local',
          wire: tampered,
        ),
        throwsA(isA<E2eDecryptException>()),
      );
    });
  });
}
