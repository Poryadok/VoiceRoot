import 'dart:convert';
import 'dart:typed_data';

import 'package:crypto/crypto.dart';

/// Crockford Base32 alphabet (docs/features/encryption.md).
const _crockford = 'ABCDEFGHJKMNPQRSTUVWXYZ23456789';

/// Symmetric 6-character verification fingerprint for DM E2E trust UX.
///
/// Derived from sorted [localProfileId]/[remoteProfileId] and matching identity
/// key bytes (docs/features/encryption.md § Доверие к ключам).
String computeVerificationCode({
  required Uint8List localIdentityKey,
  required Uint8List remoteIdentityKey,
  required String localProfileId,
  required String remoteProfileId,
}) {
  final ids = [localProfileId, remoteProfileId]..sort();
  final firstKey =
      localProfileId == ids[0] ? localIdentityKey : remoteIdentityKey;
  final secondKey =
      localProfileId == ids[0] ? remoteIdentityKey : localIdentityKey;
  final digest = sha256.convert([
    ...utf8.encode(ids[0]),
    ...utf8.encode(ids[1]),
    ...firstKey,
    ...secondKey,
  ]).bytes;

  // 30 bits → 6 Crockford symbols (5 bits each).
  var value = 0;
  for (var i = 0; i < 4; i++) {
    value = (value << 8) | digest[i];
  }
  value &= 0x3fffffff;

  final chars = <String>[];
  for (var i = 0; i < 6; i++) {
    final shift = 5 * (5 - i);
    final index = (value >> shift) & 0x1f;
    chars.add(_crockford[index % _crockford.length]);
  }

  return '${chars[0]}${chars[1]}-${chars[2]}${chars[3]}-${chars[4]}${chars[5]}';
}

/// Extracts raw identity public key bytes from a libsignal identity serialization.
Uint8List identityKeyBytesFromSerialized(Uint8List serialized) {
  if (serialized.length >= 33) {
    return Uint8List.fromList(serialized.sublist(serialized.length - 33));
  }
  return serialized;
}
