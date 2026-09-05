package voice.backend.auth.sessionepoch;

import java.util.UUID;
import voice.backend.auth.repository.AccountRepository;

/** Prepares one durable session epoch for a later JWT issuance without reading an Account snapshot. */
public final class SessionEpochIssuanceGate {
  private final AccountRepository accounts;
  private final SessionEpochFloorStore floors;

  public SessionEpochIssuanceGate(AccountRepository accounts, SessionEpochFloorStore floors) {
    if (accounts == null || floors == null) {
      throw new IllegalArgumentException("account repository and floor store are required");
    }
    this.accounts = accounts;
    this.floors = floors;
  }

  public PreparedSessionEpoch prepare(UUID accountId, long durableEpoch) {
    if (accountId == null || durableEpoch <= 0) {
      throw new IllegalArgumentException("account id and positive session epoch are required");
    }

    long floor;
    try {
      floor = floors.recordAtLeast(accountId, durableEpoch);
    } catch (RuntimeException ex) {
      throw new SessionEpochFloorUnavailableException("session epoch floor unavailable", ex);
    }
    if (floor <= 0 || floor < durableEpoch) {
      throw new SessionEpochFloorUnavailableException("invalid session epoch floor");
    }
    if (floor == durableEpoch) {
      return new PreparedSessionEpoch(accountId, durableEpoch);
    }

    long advancedEpoch;
    try {
      advancedEpoch = accounts.advanceSessionEpochAtLeast(accountId, floor);
    } catch (RuntimeException ex) {
      throw new SessionEpochFloorUnavailableException("durable session epoch unavailable", ex);
    }
    if (advancedEpoch <= 0 || advancedEpoch < floor) {
      throw new SessionEpochFloorUnavailableException("invalid durable session epoch");
    }
    return new PreparedSessionEpoch(accountId, advancedEpoch);
  }
}
