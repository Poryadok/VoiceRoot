package voice.backend.auth.sdkidentity;

/** Uniform denial across SDK identity proofs and lifecycle checks. */
public final class SdkIdentityDeniedException extends RuntimeException {
  public SdkIdentityDeniedException() {
    super("invalid_sdk_identity");
  }
}
