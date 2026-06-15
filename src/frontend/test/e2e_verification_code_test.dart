import 'dart:typed_data';

import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/e2e/e2e_verification_code.dart';

/// Batch E2E-A audit: Crockford XX-XX-XX verification fingerprint (docs/features/encryption.md).
void main() {
  final crockfordPattern = RegExp(r'^[A-Z2-9]{2}-[A-Z2-9]{2}-[A-Z2-9]{2}$');

  Uint8List fakeIdentityKey(int seed) {
    return Uint8List.fromList(List<int>.generate(33, (i) => (seed + i) & 0xff));
  }

  test('computeVerificationCode returns XX-XX-XX Crockford format', () {
    final code = computeVerificationCode(
      localIdentityKey: fakeIdentityKey(0x11),
      remoteIdentityKey: fakeIdentityKey(0x22),
      localProfileId: 'profile-alpha',
      remoteProfileId: 'profile-beta',
    );

    expect(code, matches(crockfordPattern));
  });

  test('computeVerificationCode is symmetric for both participants', () {
    final localKey = fakeIdentityKey(0x33);
    final remoteKey = fakeIdentityKey(0x44);

    final ab = computeVerificationCode(
      localIdentityKey: localKey,
      remoteIdentityKey: remoteKey,
      localProfileId: 'user-a',
      remoteProfileId: 'user-b',
    );
    final ba = computeVerificationCode(
      localIdentityKey: remoteKey,
      remoteIdentityKey: localKey,
      localProfileId: 'user-b',
      remoteProfileId: 'user-a',
    );

    expect(ab, equals(ba));
  });

  test('computeVerificationCode is stable for the same inputs', () {
    final localKey = fakeIdentityKey(0x55);
    final remoteKey = fakeIdentityKey(0x66);

    final first = computeVerificationCode(
      localIdentityKey: localKey,
      remoteIdentityKey: remoteKey,
      localProfileId: 'stable-a',
      remoteProfileId: 'stable-b',
    );
    final second = computeVerificationCode(
      localIdentityKey: localKey,
      remoteIdentityKey: remoteKey,
      localProfileId: 'stable-a',
      remoteProfileId: 'stable-b',
    );

    expect(second, equals(first));
  });
}
