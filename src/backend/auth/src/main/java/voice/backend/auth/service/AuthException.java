package voice.backend.auth.service;

public class AuthException extends RuntimeException {
  public AuthException(String code) {
    super(code);
  }
}
