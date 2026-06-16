package voice.backend.auth.oauth;

public class OAuthException extends RuntimeException {
  private final String error;
  private final int httpStatus;

  public OAuthException(String error, int httpStatus) {
    super(error);
    this.error = error;
    this.httpStatus = httpStatus;
  }

  public String error() {
    return error;
  }

  public int httpStatus() {
    return httpStatus;
  }
}
