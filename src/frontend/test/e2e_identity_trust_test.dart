import 'dart:typed_data';

import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/e2e/e2e_identity_trust.dart';

void main() {
  test('notePeerIdentityKey stores first key without pending change', () {
    final notifier = E2eIdentityTrustNotifier();
    final key = Uint8List.fromList([1, 2, 3]);
    notifier.notePeerIdentityKey(peerProfileId: 'peer-a', identityKeyBytes: key);
    expect(notifier.state.knownIdentityKeys['peer-a'], key);
    expect(notifier.hasPendingKeyChange('peer-a'), isFalse);
  });

  test('notePeerIdentityKey rotation marks pending key change', () {
    final notifier = E2eIdentityTrustNotifier();
    notifier.notePeerIdentityKey(
      peerProfileId: 'peer-a',
      identityKeyBytes: Uint8List.fromList([1]),
    );
    notifier.notePeerIdentityKey(
      peerProfileId: 'peer-a',
      identityKeyBytes: Uint8List.fromList([2]),
    );
    expect(notifier.hasPendingKeyChange('peer-a'), isTrue);
  });

  test('acceptKeyChange clears pending and updates known key', () {
    final notifier = E2eIdentityTrustNotifier();
    notifier.notePeerIdentityKey(
      peerProfileId: 'peer-a',
      identityKeyBytes: Uint8List.fromList([1]),
    );
    final newKey = Uint8List.fromList([9]);
    notifier.notePeerIdentityKey(peerProfileId: 'peer-a', identityKeyBytes: newKey);
    notifier.acceptKeyChange('peer-a', newKey);
    expect(notifier.hasPendingKeyChange('peer-a'), isFalse);
    expect(notifier.state.knownIdentityKeys['peer-a'], newKey);
  });

  test('distrustPeer marks peer distrusted', () {
    final notifier = E2eIdentityTrustNotifier();
    notifier.distrustPeer('peer-a');
    expect(notifier.isDistrusted('peer-a'), isTrue);
  });
}
