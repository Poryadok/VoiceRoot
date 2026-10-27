package voice.backend.auth.linkoauth;

/** Fixed public errors only; never attach provider responses or credentials. */
public final class LinkedOAuthException extends RuntimeException {
  public LinkedOAuthException(String error) { super(error); }
}
