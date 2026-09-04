package voice.backend.auth.sessionepoch;

import java.util.Map;
import java.util.UUID;

/** Seeds and reconciles Redis floors upward from Auth's durable source. */
public final class AuthSessionEpochFloorReconciler {
  private final DurableAccountEpochSource durableEpochs;
  private final SessionEpochFloorStore floors;

  public AuthSessionEpochFloorReconciler(
      DurableAccountEpochSource durableEpochs, SessionEpochFloorStore floors) {
    if (durableEpochs == null || floors == null) {
      throw new IllegalArgumentException("durable epochs and floor store are required");
    }
    this.durableEpochs = durableEpochs;
    this.floors = floors;
  }

  public void seedAndReconcile() {
    Map<UUID, Long> epochs;
    try {
      epochs = durableEpochs.currentAccountEpochs();
    } catch (RuntimeException ex) {
      throw new SessionEpochFloorUnavailableException("durable session epochs unavailable", ex);
    }
    if (epochs == null) {
      throw new SessionEpochFloorUnavailableException("durable session epochs unavailable");
    }
    for (Map.Entry<UUID, Long> entry : epochs.entrySet()) {
      if (entry.getKey() == null || entry.getValue() == null || entry.getValue() <= 0) {
        throw new SessionEpochFloorUnavailableException("invalid durable session epoch");
      }
      floors.recordAtLeast(entry.getKey(), entry.getValue());
    }
  }
}
