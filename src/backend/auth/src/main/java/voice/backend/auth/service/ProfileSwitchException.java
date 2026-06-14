package voice.backend.auth.service;

public class ProfileSwitchException extends AuthException {
  public enum Kind {
    NOT_FOUND,
    FORBIDDEN,
    PRECONDITION
  }

  private final Kind kind;

  public ProfileSwitchException(String message, Kind kind) {
    super(message);
    this.kind = kind;
  }

  public Kind kind() {
    return kind;
  }
}
