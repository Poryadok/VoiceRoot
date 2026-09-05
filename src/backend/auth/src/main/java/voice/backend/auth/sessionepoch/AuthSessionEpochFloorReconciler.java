package voice.backend.auth.sessionepoch;

import java.util.List;
import java.util.UUID;
import voice.backend.auth.repository.AccountSessionEpoch;

/** Seeds and reconciles Redis floors upward from Auth's durable source. */
public final class AuthSessionEpochFloorReconciler {
  private final DurableAccountEpochSource durableEpochs;
  private final SessionEpochFloorStore floors;
  private final int pageSize;

  public AuthSessionEpochFloorReconciler(
      DurableAccountEpochSource durableEpochs, SessionEpochFloorStore floors, int pageSize) {
    if (durableEpochs == null || floors == null || pageSize <= 0) {
      throw new IllegalArgumentException("durable epochs and floor store are required");
    }
    this.durableEpochs = durableEpochs;
    this.floors = floors;
    this.pageSize = pageSize;
  }

  public void seedAndReconcile() {
    UUID cursor = null;
    while (true) {
      List<AccountSessionEpoch> page = pageAfter(cursor);
      if (page == null) {
        throw new SessionEpochFloorUnavailableException("durable session epochs unavailable");
      }
      if (page.isEmpty()) {
        return;
      }
      for (AccountSessionEpoch durableEpoch : page) {
        validateDurableEpoch(durableEpoch);
        if (cursor != null && comparePostgresUuid(durableEpoch.accountId(), cursor) <= 0) {
          throw new SessionEpochFloorUnavailableException("durable session epoch cursor did not advance");
        }
        long recordedFloor = recordFloor(durableEpoch);
        if (recordedFloor <= 0 || recordedFloor < durableEpoch.sessionEpoch()) {
          throw new SessionEpochFloorUnavailableException("invalid session epoch floor");
        }
        if (recordedFloor > durableEpoch.sessionEpoch()) {
          long advancedEpoch = advanceDurableEpoch(durableEpoch.accountId(), recordedFloor);
          if (advancedEpoch < recordedFloor) {
            throw new SessionEpochFloorUnavailableException("durable session epoch did not advance");
          }
        }
        cursor = durableEpoch.accountId();
      }
    }
  }

  private List<AccountSessionEpoch> pageAfter(UUID cursor) {
    try {
      return durableEpochs.pageSessionEpochsAfter(cursor, pageSize);
    } catch (RuntimeException ex) {
      throw new SessionEpochFloorUnavailableException("durable session epochs unavailable", ex);
    }
  }

  private static void validateDurableEpoch(AccountSessionEpoch durableEpoch) {
    if (durableEpoch == null || durableEpoch.accountId() == null || durableEpoch.sessionEpoch() <= 0) {
      throw new SessionEpochFloorUnavailableException("invalid durable session epoch");
    }
  }

  private static int comparePostgresUuid(UUID left, UUID right) {
    int mostSignificant =
        Long.compareUnsigned(left.getMostSignificantBits(), right.getMostSignificantBits());
    if (mostSignificant != 0) {
      return mostSignificant;
    }
    return Long.compareUnsigned(left.getLeastSignificantBits(), right.getLeastSignificantBits());
  }

  private long recordFloor(AccountSessionEpoch durableEpoch) {
    try {
      return floors.recordAtLeast(durableEpoch.accountId(), durableEpoch.sessionEpoch());
    } catch (RuntimeException ex) {
      throw new SessionEpochFloorUnavailableException("session epoch floor unavailable", ex);
    }
  }

  private long advanceDurableEpoch(UUID accountId, long recordedFloor) {
    try {
      return durableEpochs.advanceSessionEpochAtLeast(accountId, recordedFloor);
    } catch (RuntimeException ex) {
      throw new SessionEpochFloorUnavailableException("durable session epochs unavailable", ex);
    }
  }
}
