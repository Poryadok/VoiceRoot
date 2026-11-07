package voice.backend.auth.ownershipproof;

/** A permanent operation fence replaces a receipt whose personal data was erased. */
public final class ProofErasedException extends RuntimeException {
  public ProofErasedException() { super("operation_erased"); }
}
