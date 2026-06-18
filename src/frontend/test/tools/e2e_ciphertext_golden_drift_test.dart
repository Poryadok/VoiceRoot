import 'package:flutter_test/flutter_test.dart';

import '../support/prekey_golden_helper.dart';

/// CI drift guard: committed ciphertext golden decrypts to [kE2eCiphertextGoldenPlaintext].
void main() {
  test('e2e ciphertext libsignal golden decrypts on deterministic receiver', () async {
    final goldenFile = e2eCiphertextGoldenFile();
    expect(goldenFile, isNotNull, reason: 'repo root not found');
    expect(
      goldenFile!.existsSync(),
      isTrue,
      reason: 'run export with VOICE_EXPORT_E2E_CIPHERTEXT_GOLDEN=true',
    );

    final committed = (await goldenFile.readAsString()).trim();
    expect(committed, isNotEmpty);
    expect(committed, isNot(contains(kE2eCiphertextGoldenPlaintext)));

    final decrypted = await decryptCiphertextOnDeterministicReceiver(committed);
    expect(decrypted, equals(kE2eCiphertextGoldenPlaintext));

    final generated = await ciphertextFromDeterministicStores();
    final generatedPlain = await decryptCiphertextOnDeterministicReceiver(generated);
    expect(generatedPlain, equals(kE2eCiphertextGoldenPlaintext));
  });
}
