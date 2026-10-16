import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/e2e/secure_signal_store.dart';

/// Batch E2E-A red test: persistent Signal store survives reopen (docs/TODO.md).
void main() {
  group('SecureSignalStore', () {
    test('persists identity across store reopen', () async {
      const profileId = 'profile-persist-1';
      final storage = InMemorySecureSignalStorage();

      final first = await SecureSignalStore.open(
        profileId: profileId,
        storage: storage,
      );
      final firstIdentity = await first.getIdentityKeyPair();
      final firstRegistration = await first.getLocalRegistrationId();
      await first.close();

      final reopened = await SecureSignalStore.open(
        profileId: profileId,
        storage: storage,
      );
      final reopenedIdentity = await reopened.getIdentityKeyPair();
      final reopenedRegistration = await reopened.getLocalRegistrationId();
      await reopened.close();

      expect(
        base64Encode(reopenedIdentity.getPublicKey().serialize()),
        base64Encode(firstIdentity.getPublicKey().serialize()),
      );
      expect(reopenedRegistration, firstRegistration);
      expect(await reopened.containsSignedPreKey(1), isTrue);
    });

    test(
      'imports a valid backup payload through the same persisted state format',
      () async {
        const profileId = 'profile-backup-import';
        final source = await SecureSignalStore.exportForBackup(
          profileId,
          storage: InMemorySecureSignalStorage(),
        );
        final storage = InMemorySecureSignalStorage();

        await SecureSignalStore.importFromBackup(
          profileId,
          source,
          storage: storage,
        );

        final encoded = await storage.read(key: 'state_v1');
        expect(encoded, isNotNull);
        expect(jsonDecode(encoded!), source);
        final restored = await SecureSignalStore.open(
          profileId: profileId,
          storage: storage,
        );
        expect(
          await restored.getLocalRegistrationId(),
          source['registration_id'],
        );
        await restored.close();
      },
    );
  });
}

/// Test double for flutter_secure_storage until platform storage is wired in tests.
class InMemorySecureSignalStorage implements SecureSignalStorage {
  final Map<String, String> _values = {};

  @override
  Future<void> delete({required String key}) async {
    _values.remove(key);
  }

  @override
  Future<String?> read({required String key}) async => _values[key];

  @override
  Future<void> write({required String key, required String value}) async {
    _values[key] = value;
  }
}
