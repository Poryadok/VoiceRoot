package voice.backend.auth.sdkidentity;

/** Terminal result without retained account, device, proof or response material. */
public final class SdkOperationErasedException extends RuntimeException {
  public SdkOperationErasedException() { super("operation_erased"); }
}
