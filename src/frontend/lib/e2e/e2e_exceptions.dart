/// Thrown when outgoing plaintext cannot be encrypted for an E2E chat.
class E2eEncryptException implements Exception {
  E2eEncryptException(this.message, {this.cause});

  final String message;
  final Object? cause;

  @override
  String toString() => message;
}

/// Thrown when ciphertext cannot be decrypted (tampered or missing session keys).
class E2eDecryptException implements Exception {
  E2eDecryptException([this.cause]);

  final Object? cause;

  @override
  String toString() =>
      cause == null ? 'E2eDecryptException' : 'E2eDecryptException: $cause';
}
