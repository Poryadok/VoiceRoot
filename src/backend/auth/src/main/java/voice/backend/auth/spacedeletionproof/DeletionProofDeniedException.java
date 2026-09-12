package voice.backend.auth.spacedeletionproof;

/** Deliberately coarse denial for every absent, expired, revoked or mismatched proof. */
public final class DeletionProofDeniedException extends RuntimeException {
  public DeletionProofDeniedException() {
    super("space deletion proof denied");
  }
}
