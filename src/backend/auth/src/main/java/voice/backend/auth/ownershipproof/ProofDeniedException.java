package voice.backend.auth.ownershipproof;

public final class ProofDeniedException extends RuntimeException {
  public ProofDeniedException() { super("ownership proof denied"); }
}
