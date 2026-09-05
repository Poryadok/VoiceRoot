package voice.backend.auth.sessionepoch;

/** A floor cannot safely be read or advanced. Strict consumers must deny instead of defaulting. */
public class SessionEpochFloorUnavailableException extends RuntimeException {
  public SessionEpochFloorUnavailableException(String message) {
    super(message);
  }

  public SessionEpochFloorUnavailableException(String message, Throwable cause) {
    super(message, cause);
  }
}
