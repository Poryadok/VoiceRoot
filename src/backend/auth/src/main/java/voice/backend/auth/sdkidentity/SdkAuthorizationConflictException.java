package voice.backend.auth.sdkidentity;

/** A client idempotency key cannot authorize another request body. */
public final class SdkAuthorizationConflictException extends RuntimeException {
  public SdkAuthorizationConflictException() { super("sdk_authorization_conflict"); }
}
