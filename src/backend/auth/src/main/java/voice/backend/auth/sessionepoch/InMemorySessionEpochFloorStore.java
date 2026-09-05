package voice.backend.auth.sessionepoch;

import java.util.UUID;
import java.util.concurrent.ConcurrentHashMap;

/** In-memory max-only floor used by the memory persistence mode and direct service tests. */
public final class InMemorySessionEpochFloorStore implements SessionEpochFloorStore {
  private final ConcurrentHashMap<UUID, Long> floors = new ConcurrentHashMap<>();

  @Override
  public long recordAtLeast(UUID accountId, long epoch) {
    if (accountId == null || epoch <= 0) {
      throw new IllegalArgumentException("account id and positive session epoch are required");
    }
    return floors.merge(accountId, epoch, Math::max);
  }

  @Override
  public long requireFloor(UUID accountId) {
    Long floor = floors.get(accountId);
    if (floor == null || floor <= 0) {
      throw new SessionEpochFloorUnavailableException("session epoch floor missing");
    }
    return floor;
  }
}
