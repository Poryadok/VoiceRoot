package voice.backend.auth.sessionepoch;

/** The floor key is absent, so a failed deletion may safely resume sealing its durable epoch. */
public final class SessionEpochFloorMissingException extends SessionEpochFloorUnavailableException {
  public SessionEpochFloorMissingException(String message) {
    super(message);
  }
}
