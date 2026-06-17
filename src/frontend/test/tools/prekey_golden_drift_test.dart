import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';

import '../support/prekey_golden_helper.dart';

Map<String, dynamic> canonicalPreKeyPayload(String wireB64) {
  final decoded = utf8.decode(base64Decode(wireB64.trim()));
  final map = Map<String, dynamic>.from(jsonDecode(decoded) as Map);
  map.remove('signed_pre_key_signature');
  return map;
}

/// CI drift guard: committed golden must match deterministic libsignal export.
///
/// libsignal may emit a different valid signature nonce per run; structural fields
/// must stay stable and the committed signature must remain verifiable by Go tests.
void main() {
  test('prekey libsignal golden matches deterministic export', () async {
    final goldenFile = prekeyGoldenFile();
    expect(goldenFile, isNotNull, reason: 'repo root not found');
    expect(
      goldenFile!.existsSync(),
      isTrue,
      reason: 'run export with VOICE_EXPORT_PREKEY_GOLDEN=true',
    );

    final committed = (await goldenFile.readAsString()).trim();
    final generated = (await wireFromDeterministicStore()).trim();
    expect(
      canonicalPreKeyPayload(generated),
      equals(canonicalPreKeyPayload(committed)),
    );

    final committedJson =
        jsonDecode(utf8.decode(base64Decode(committed))) as Map<String, dynamic>;
    final signature = committedJson['signed_pre_key_signature'] as String?;
    expect(signature, isNotNull);
    expect(base64Decode(signature!), hasLength(64));
  });
}
