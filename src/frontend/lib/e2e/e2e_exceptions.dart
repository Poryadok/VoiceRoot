/// Thrown when ciphertext cannot be decrypted (tampered or missing session keys).
class E2eDecryptException implements Exception {
  E2eDecryptException([this.cause]);

  final Object? cause;

  @override
  String toString() =>
      cause == null ? 'E2eDecryptException' : 'E2eDecryptException: $cause';
}
