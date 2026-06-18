import 'package:flutter_test/flutter_test.dart';

import '../support/prekey_golden_helper.dart';

/// Writes libsignal E2E ciphertext golden when explicitly requested.
///
/// ```text
/// cd src/frontend
/// flutter test test/tools/export_e2e_ciphertext_golden_test.dart --dart-define=VOICE_EXPORT_E2E_CIPHERTEXT_GOLDEN=true
/// ```
void main() {
  const exportGolden = bool.fromEnvironment('VOICE_EXPORT_E2E_CIPHERTEXT_GOLDEN');

  test('export e2e ciphertext libsignal golden', () async {
    final wire = await ciphertextFromDeterministicStores();
    expect(wire, isNotEmpty);
    expect(wire, isNot(contains(kE2eCiphertextGoldenPlaintext)));

    final decrypted = await decryptCiphertextOnDeterministicReceiver(wire);
    expect(decrypted, equals(kE2eCiphertextGoldenPlaintext));

    if (!exportGolden) {
      return;
    }

    final messagingFile = e2eCiphertextGoldenFile();
    final composeFile = e2eCiphertextGoldenComposeFile();
    expect(messagingFile, isNotNull, reason: 'repo root not found');
    expect(composeFile, isNotNull);
    await messagingFile!.parent.create(recursive: true);
    await messagingFile.writeAsString(wire);
    await composeFile!.writeAsString(wire);

    // ignore: avoid_print
    print('Wrote libsignal E2E ciphertext golden to ${messagingFile.path}');
  });
}
