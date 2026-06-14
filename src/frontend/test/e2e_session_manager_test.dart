import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/e2e/e2e_exceptions.dart';
import 'package:voice_frontend/e2e/e2e_key_backup.dart';
import 'package:voice_frontend/e2e/e2e_session_manager.dart';
import 'package:voice_frontend/e2e/e2e_store_factory.dart';

void main() {
  test('E2eDecryptException formats with cause', () {
    final err = E2eDecryptException(StateError('bad'));
    expect('$err', contains('bad'));
  });

  group('E2eSessionManager', () {
    test('ensureSession creates bilateral stores per profile pair', () async {
      final manager = E2eSessionManager();
      final session = await manager.ensureSession(
        localProfileId: 'alpha',
        remoteProfileId: 'beta',
      );

      expect(session.localProfileId, 'alpha');
      expect(session.remoteProfileId, 'beta');
      expect(await session.localStore.containsSession(session.remoteAddress), isTrue);
      expect(await session.remoteStore.containsSession(session.localAddress), isTrue);
    });

    test('serialize and parse pre-key bundle roundtrip', () async {
      final store = await createInitializedSignalStore();
      final wire = await serializePreKeyBundle(store);
      final parsed = parseSerializedPreKeyBundle(wire);
      expect(parsed, isNotNull);
    });
  });

  group('E2eKeyBackupCodec', () {
    const codec = E2eKeyBackupCodec();

    test('encrypt then decrypt restores payload', () {
      const payload = {'profile_id': 'p1', 'version': 1};
      final blob = codec.encryptPayload(password: 'secret-pass', payload: payload);
      final restored = codec.decryptPayload(password: 'secret-pass', encryptedBlob: blob);
      expect(restored['profile_id'], 'p1');
    });

    test('wrong password fails decrypt', () {
      final blob = codec.encryptPayload(
        password: 'right',
        payload: const {'k': 'v'},
      );
      expect(
        () => codec.decryptPayload(password: 'wrong', encryptedBlob: blob),
        throwsA(isA<FormatException>()),
      );
    });
  });
}
