package voice.backend.auth.events;

import java.util.UUID;

/** Non-authoritative observations; implementations must not fail the Auth operation. */
public interface AuthObservations {
  default void registered(UUID accountId, String type, String method) {}
  default void loggedIn(UUID accountId, String deviceInfo) {}
  default void loggedOut(UUID accountId, String deviceInfo) {}
  default void twoFaEnabled(UUID accountId) {}
}
