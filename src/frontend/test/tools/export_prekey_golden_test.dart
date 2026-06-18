import 'package:flutter_test/flutter_test.dart';

import '../support/prekey_golden_helper.dart';

/// Writes libsignal pre-key golden when explicitly requested.
///
/// ```text
/// cd src/frontend
/// flutter test test/tools/export_prekey_golden_test.dart --dart-define=VOICE_EXPORT_PREKEY_GOLDEN=true
/// ```
void main() {
  const exportGolden = bool.fromEnvironment('VOICE_EXPORT_PREKEY_GOLDEN');

  test('export prekey libsignal golden', () async {
    final wire = await wireFromDeterministicStore();
    expect(wire, isNotEmpty);

    if (!exportGolden) {
      // Default CI path: no file mutation (drift checked by prekey_golden_drift_test.dart).
      return;
    }

    final outFile = prekeyGoldenFile();
    expect(outFile, isNotNull, reason: 'repo root not found');
    await outFile!.parent.create(recursive: true);
    await outFile.writeAsString(wire);

    final peerWire = await wireFromDeterministicSenderStore();
    final peerFile = prekeyPeerGoldenFile();
    final peerComposeFile = prekeyPeerGoldenComposeFile();
    expect(peerFile, isNotNull);
    expect(peerComposeFile, isNotNull);
    await peerFile!.writeAsString(peerWire);
    await peerComposeFile!.writeAsString(peerWire);

    // ignore: avoid_print
    print('Wrote libsignal pre-key golden to ${outFile.path}');
    // ignore: avoid_print
    print('Wrote libsignal peer pre-key golden to ${peerFile.path}');
  });
}
