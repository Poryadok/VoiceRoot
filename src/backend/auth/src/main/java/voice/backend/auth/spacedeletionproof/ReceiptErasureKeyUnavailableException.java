package voice.backend.auth.spacedeletionproof;

/** Lookup cannot verify pseudonymous evidence because its retained key is unavailable. */
public final class ReceiptErasureKeyUnavailableException extends RuntimeException {
  public ReceiptErasureKeyUnavailableException(int version) {
    super("space deletion receipt erasure key unavailable: version " + version);
  }
}
