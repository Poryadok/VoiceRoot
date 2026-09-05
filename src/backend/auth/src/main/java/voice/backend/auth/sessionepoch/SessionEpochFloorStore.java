package voice.backend.auth.sessionepoch;

import java.util.UUID;

/** Durable minimum session-epoch floor used by later strict consumers. */
public interface SessionEpochFloorStore {
  long recordAtLeast(UUID accountId, long epoch);

  long requireFloor(UUID accountId);
}
